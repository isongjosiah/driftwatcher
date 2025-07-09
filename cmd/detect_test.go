package cmd_test

import (
	"bytes"
	"context"
	"drift-watcher/cmd"
	"drift-watcher/config"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/driftchecker/driftcheckerfakes"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/provider/providerfakes"
	"drift-watcher/pkg/services/reporter/reporterfakes"
	"drift-watcher/pkg/services/statemanager" // Import for NewTerraformManager
	"drift-watcher/pkg/services/statemanager/statemanagerfakes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	fakeStateManager           statemanagerfakes.FakeStateManagerI
	fakeProvider               providerfakes.FakeProviderI
	fakeDriftChecker           driftcheckerfakes.FakeDriftChecker
	fakeReporter               reporterfakes.FakeOutputWriter
	fakeInfrastructureResource providerfakes.FakeInfrastructureResourceI
)

// Helper to capture slog output
func captureSlogOutput() *bytes.Buffer {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))
	return &buf
}

func TestNewDetectCmd(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)

	assert.NotNil(t, dc)
	assert.NotNil(t, dc.Cmd)
	assert.Equal(t, "detect", dc.Cmd.Use)
	assert.Contains(t, dc.Cmd.Aliases, "d")

	// Check flags
	assert.NotNil(t, dc.Cmd.Flags().Lookup("configfile"))
	assert.NotNil(t, dc.Cmd.Flags().Lookup("attributes"))
	assert.NotNil(t, dc.Cmd.Flags().Lookup("provider"))
	assert.NotNil(t, dc.Cmd.Flags().Lookup("resource"))
	assert.NotNil(t, dc.Cmd.Flags().Lookup("output-file"))
	assert.NotNil(t, dc.Cmd.Flags().Lookup("state-manager"))

	// Check default values
	assert.Equal(t, []string{"instance_type"}, dc.AttributesToTrack)
	assert.Equal(t, "aws", dc.Provider)
	assert.Equal(t, "aws_instance", dc.Resource)
	assert.Equal(t, "", dc.OutputPath)
	assert.Equal(t, "terraform", dc.StateManagerType)
}

func TestDetectCmd_Run_MissingConfigFile(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	// dc.tfConfigPath is empty by default

	buf := captureSlogOutput()
	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "A state file is required")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Invalid state file path provided")
}

func TestDetectCmd_Run_UnsupportedStateManager(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.StateManagerType = "unsupported-manager"

	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported-manager statemanager not currently supported")
}

func TestDetectCmd_Run_UnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.Provider = "unsupported-provider"

	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported-provider platform not currently supported")
}

// Mock for aws.CheckAWSConfig to avoid actual file system interaction
func mockCheckAWSConfig(homeDir string, profile string) (config.AWSConfig, error) {
	if profile == "error-profile" {
		return config.AWSConfig{}, fmt.Errorf("mock AWS config error")
	}
	return config.AWSConfig{
		CredentialPath: []string{"/mock/creds"},
		ConfigPath:     []string{"/mock/config"},
		ProfileName:    profile,
	}, nil
}

// Mock for aws.NewAWSProvider to avoid actual AWS SDK initialization
type MockAWSProvider struct {
	mock.Mock
}

func (m *MockAWSProvider) InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (provider.InfrastructureResourceI, error) {
	args := m.Called(ctx, resourceType, resource)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(provider.InfrastructureResourceI), args.Error(1)
}

func TestDetectCmd_Run_NewAWSProviderError(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.TfConfigPath = "/tmp/test.tfstate"
	dc.StateManagerType = "terraform"
	dc.Provider = "aws"
	dc.PlatformProvider = &fakeProvider
	dc.StateManager = &fakeStateManager
	dc.DriftChecker = &fakeDriftChecker
	dc.Reporter = &fakeReporter

	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock NewAWSProvider error")
}

func TestDetectCmd_Run_Success_StdoutReporter(t *testing.T) {
	// Mock dependencies for RunDriftDetection
	fakeStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	fakeStateManager.RetrieveResourcesReturns([]statemanager.StateResource{}, nil)

	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.TfConfigPath = "/tmp/test.tfstate"
	dc.StateManagerType = "terraform"
	dc.Provider = "aws"
	dc.OutputPath = "" // Ensure stdout reporter is used
	dc.PlatformProvider = &fakeProvider
	dc.StateManager = &fakeStateManager
	dc.DriftChecker = &fakeDriftChecker
	dc.Reporter = &fakeReporter

	err := dc.Run(dc.Cmd, []string{})
	require.NoError(t, err)

	assert.Equal(t, fakeStateManager.RetrieveResourcesCallCount(), 1)
}

func TestDetectCmd_Run_Success_FileReporter(t *testing.T) {
	// Setup mocks

	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.TfConfigPath = "/tmp/test.tfstate"
	dc.StateManagerType = "terraform"
	dc.Provider = "aws"
	dc.OutputPath = "/tmp/report.json" // Ensure file reporter is used

	// Manually set internal mocks for RunDriftDetection
	dc.StateManager = &fakeStateManager
	dc.PlatformProvider = &fakeProvider
	dc.DriftChecker = &fakeDriftChecker
	dc.Reporter = &fakeReporter

	err := dc.Run(dc.Cmd, []string{})
	require.NoError(t, err)
}

func TestRunDriftDetection_ParseStateFileError(t *testing.T) {
	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/nonexistent.tfstate", "aws_instance", []string{}, &fakeStateManager, &fakeProvider, &fakeDriftChecker, &fakeReporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse state file: parse error")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to parse desired state information from the state file")
}

func TestRunDriftDetection_RetrieveResourcesError(t *testing.T) {
	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, &fakeStateManager, &fakeProvider, &fakeDriftChecker, &fakeReporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve resources: retrieve error")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to retrieve resources from state")
}

func TestRunDriftDetection_NoResourcesFound(t *testing.T) {
	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, &fakeStateManager, &fakeProvider, &fakeDriftChecker, &fakeReporter)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "No resources found to check for drift.")
}

func TestRunDriftDetection_SuccessWithDrift(t *testing.T) {
	// Prepare dummy resources
	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resource2 := statemanager.StateResource{Name: "res2", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1, resource2}

	// Mock behaviors
	fakeStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	fakeStateManager.RetrieveResourcesReturns(resources, nil)

	fakeInfrastructureResource.ResourceTypeReturnsOnCall(1, "aws_instance")
	fakeInfrastructureResource.AttributeValueReturnsOnCall(1, "t2.medium", nil)

	fakeInfrastructureResource.ResourceTypeReturnsOnCall(2, "aws_instance")
	fakeInfrastructureResource.AttributeValueReturnsOnCall(2, "t2.micro", nil)

	fakeProvider.InfrastructreMetadataReturns(&fakeInfrastructureResource, nil)
	fakeProvider.InfrastructreMetadataReturns(&fakeInfrastructureResource, nil)

	driftReport1 := &driftchecker.DriftReport{
		HasDrift: true,
		Status:   driftchecker.Drift,
		DriftDetails: []driftchecker.DriftItem{
			{Field: "instance_type", TerraformValue: "t2.micro", ActualValue: "t2.medium", DriftType: driftchecker.AttributeValueChanged},
		},
	}
	driftReport2 := &driftchecker.DriftReport{
		HasDrift: false,
		Status:   driftchecker.Match,
	}

	fakeDriftChecker.CompareStatesReturns(driftReport1, nil)
	fakeDriftChecker.CompareStatesReturns(driftReport2, nil)

	fakeReporter.WriteReportReturnsOnCall(0, nil)
	fakeReporter.WriteReportReturnsOnCall(1, nil)

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, &fakeStateManager, &fakeProvider, &fakeDriftChecker, &fakeReporter)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "Drift detection completed.")
}

func TestRunDriftDetection_InfrastructureMetadataError(t *testing.T) {
	mockStateManager := statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := providerfakes.FakeProviderI{}
	mockDriftChecker := driftcheckerfakes.FakeDriftChecker{}
	mockReporter := reporterfakes.FakeOutputWriter{}

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturns(resources, nil)
	mockPlatformProvider.InfrastructreMetadataReturns(nil, fmt.Errorf("infra metadata error"))

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, &mockStateManager, &mockPlatformProvider, &mockDriftChecker, &mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to retrieve infrastructure metadata")
	assert.Contains(t, buf.String(), "resource_id=res1")
}

func TestRunDriftDetection_CompareStatesError(t *testing.T) {
	mockStateManager := statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := providerfakes.FakeProviderI{}
	mockDriftChecker := driftcheckerfakes.FakeDriftChecker{}
	mockReporter := reporterfakes.FakeOutputWriter{}
	mockInfraResource := providerfakes.FakeInfrastructureResourceI{}

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}
	mockStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturns(resources, nil)

	mockInfraResource.ResourceTypeReturns("aws_instance")
	mockPlatformProvider.InfrastructreMetadataReturns(&mockInfraResource, nil)

	mockDriftChecker.CompareStatesReturns(nil, fmt.Errorf("compare states error"))

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, &mockStateManager, &mockPlatformProvider, &mockDriftChecker, &mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to compare states for resource")
	assert.Contains(t, buf.String(), "resource_id=res1")
}

func TestRunDriftDetection_WriteReportError(t *testing.T) {
	mockStateManager := statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := providerfakes.FakeProviderI{}
	mockDriftChecker := driftcheckerfakes.FakeDriftChecker{}
	mockReporter := reporterfakes.FakeOutputWriter{}
	mockInfraResource := providerfakes.FakeInfrastructureResourceI{}

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturns(resources, nil)

	mockInfraResource.ResourceTypeReturns("aws")
	mockPlatformProvider.InfrastructreMetadataReturns(&mockInfraResource, nil)

	driftReport1 := &driftchecker.DriftReport{HasDrift: true}
	mockDriftChecker.CompareStatesReturns(driftReport1, nil)
	mockReporter.WriteReportReturns(fmt.Errorf("write report error"))

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, &mockStateManager, &mockPlatformProvider, &mockDriftChecker, &mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to write report for resource")
	assert.Contains(t, buf.String(), "resource_id=res1")
}
