package cmd_test

import (
	"bytes"
	"context"
	"drift-watcher/cmd"
	"drift-watcher/config"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/driftchecker/driftcheckerfakes"
	"drift-watcher/pkg/services/provider/providerfakes"
	"drift-watcher/pkg/services/reporter"
	"drift-watcher/pkg/services/reporter/reporterfakes"
	"drift-watcher/pkg/services/statemanager" // Import for NewTerraformManager
	"drift-watcher/pkg/services/statemanager/statemanagerfakes"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	dc.TfConfigPath = "../assets/terraform_ec2_state.tfstate"

	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported-manager statemanager not currently supported")
}

func TestDetectCmd_Run_UnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.Provider = "unsupported-provider"
	dc.TfConfigPath = "../assets/terraform_ec2_state.tfstate"

	err := dc.Run(dc.Cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported-provider platform not currently supported")
}

func TestDetectCmd_Run_Success_StdoutReporter(t *testing.T) {
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}

	resource := []statemanager.StateResource{
		{
			Instances: []statemanager.ResourceInstance{
				{
					Attributes: map[string]any{
						"id": "fake-id",
					},
				},
			},
		},
	}

	driftReport := reporter.CreateDummyDriftReport(true)

	mockStateManager.ParseStateFileReturnsOnCall(0, statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturnsOnCall(0, resource, nil)
	mockPlatformProvider.InfrastructreMetadataReturns(mockInfraResource, nil)
	mockDriftChecker.CompareStatesReturns(driftReport, nil)
	mockReporter.WriteReportReturnsOnCall(0, nil)

	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.TfConfigPath = "../assets/terraform_ec2_state.tfstate"
	dc.StateManagerType = "terraform"
	dc.Provider = "aws"
	dc.OutputPath = "" // Ensure stdout reporter is used
	dc.PlatformProvider = mockPlatformProvider
	dc.StateManager = mockStateManager
	dc.DriftChecker = mockDriftChecker
	dc.Reporter = mockReporter

	err := dc.Run(dc.Cmd, []string{})
	require.NoError(t, err)
	assert.Equal(t, mockStateManager.ParseStateFileCallCount(), 1, fmt.Sprintf("Expect ParseFile to be called %v time(s) it was called %v time(s)", 1, mockStateManager.ParseStateFileCallCount()))
	_, file := mockStateManager.ParseStateFileArgsForCall(0)
	assert.Equal(t, dc.TfConfigPath, file, fmt.Sprintf("Expected file parsed to be %s, got %s instead", dc.TfConfigPath, file))
	assert.Equal(t, mockPlatformProvider.InfrastructreMetadataCallCount(), 1, fmt.Sprintf("Expect infrastructure metadata to be called %v time(s) it was called %v time(s)", 1, mockPlatformProvider.InfrastructreMetadataCallCount()))
	assert.Equal(t, mockDriftChecker.CompareStatesCallCount(), 1)
	assert.Equal(t, mockReporter.WriteReportCallCount(), 1)
}

func TestDetectCmd_Run_Success_FileReporter(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}
	_ = mockInfraResource

	resource := []statemanager.StateResource{
		{
			Instances: []statemanager.ResourceInstance{
				{
					Attributes: map[string]any{
						"id": "fake-id",
					},
				},
			},
		},
	}

	driftReport := reporter.CreateDummyDriftReport(true)

	mockStateManager.ParseStateFileReturnsOnCall(0, statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturnsOnCall(0, resource, nil)
	mockPlatformProvider.InfrastructreMetadataReturns(mockInfraResource, nil)
	mockDriftChecker.CompareStatesReturns(driftReport, nil)
	mockReporter.WriteReportReturnsOnCall(0, nil)

	ctx := context.Background()
	cfg := &config.Config{}
	dc := cmd.NewDetectCmd(ctx, cfg)
	dc.TfConfigPath = "../assets/terraform_ec2_state.tfstate"
	dc.StateManagerType = "terraform"
	dc.Provider = "aws"
	dc.Resource = "aws_instance"
	dc.OutputPath = "../assets/output.json" // Ensure file reporter is used

	err := dc.Run(dc.Cmd, []string{})
	require.NoError(t, err)
	//assert.Equal(t, mockStateManager.ParseStateFileCallCount(), 1, fmt.Sprintf("Expect ParseFile to be called %v time(s) it was called %v time(s)", 1, mockStateManager.ParseStateFileCallCount()))
	//_, file := mockStateManager.ParseStateFileArgsForCall(0)
	//assert.Equal(t, dc.TfConfigPath, file, fmt.Sprintf("Expected file parsed to be %s, got %s instead", dc.TfConfigPath, file))
	//assert.Equal(t, mockPlatformProvider.InfrastructreMetadataCallCount(), 1, fmt.Sprintf("Expect infrastructure metadata to be called %v time(s) it was called %v time(s)", 1, mockPlatformProvider.InfrastructreMetadataCallCount()))
	//assert.Equal(t, mockDriftChecker.CompareStatesCallCount(), 1)
	//assert.Equal(t, mockReporter.WriteReportCallCount(), 1)
}

func TestRunDriftDetection_ParseStateFileError(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}
	_ = mockInfraResource

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/nonexistent.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	assert.NoError(t, err)
	assert.Equal(t, mockPlatformProvider.InfrastructreMetadataCallCount(), 0)
	assert.Equal(t, mockDriftChecker.CompareStatesCallCount(), 0)
	assert.Equal(t, mockReporter.WriteReportCallCount(), 0)
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "No resources found to check for drift")
}

func TestRunDriftDetection_RetrieveResourcesError(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}
	_ = mockInfraResource

	mockStateManager.RetrieveResourcesReturnsOnCall(0, []statemanager.StateResource{}, errors.New("retrieve error"))

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve resources: retrieve error")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to retrieve resources from state")
}

func TestRunDriftDetection_NoResourcesFound(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}
	_ = mockInfraResource

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "No resources found to check for drift.")
}

func TestRunDriftDetection_SuccessWithDrift(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource1 := &providerfakes.FakeInfrastructureResourceI{}
	mockInfraResource2 := &providerfakes.FakeInfrastructureResourceI{}

	// Prepare dummy resources
	resource1 := statemanager.StateResource{
		Name: "res1",
		Type: "aws_instance",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"instance_type": "t2.micro",
				},
			},
		},
	}
	resource2 := statemanager.StateResource{
		Name: "res2",
		Type: "aws_instance",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"instance_type": "t2.micro",
				},
			},
		},
	}
	resources := []statemanager.StateResource{resource1, resource2}
	_ = resources

	// Mock behaviors
	mockStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturns(resources, nil)

	mockInfraResource1.ResourceTypeReturnsOnCall(0, "aws_instance")
	mockInfraResource1.AttributeValueReturnsOnCall(0, "t2.medium", nil)

	mockInfraResource2.ResourceTypeReturnsOnCall(0, "aws_instance")
	mockInfraResource2.AttributeValueReturnsOnCall(0, "t2.micro", nil)

	mockPlatformProvider.InfrastructreMetadataReturnsOnCall(0, mockInfraResource1, nil)
	mockPlatformProvider.InfrastructreMetadataReturnsOnCall(1, mockInfraResource2, nil)

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
	_, _ = driftReport1, driftReport2

	mockDriftChecker.CompareStatesReturnsOnCall(0, driftReport1, nil)
	mockDriftChecker.CompareStatesReturnsOnCall(1, driftReport2, nil)

	mockReporter.WriteReportReturnsOnCall(0, nil)
	mockReporter.WriteReportReturnsOnCall(1, nil)

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "Drift detection completed.")
}

func TestRunDriftDetection_InfrastructureMetadataError(t *testing.T) {
	// Setup mocks
	mockStateManager := &statemanagerfakes.FakeStateManagerI{}
	mockPlatformProvider := &providerfakes.FakeProviderI{}
	mockDriftChecker := &driftcheckerfakes.FakeDriftChecker{}
	mockReporter := &reporterfakes.FakeOutputWriter{}
	mockInfraResource := &providerfakes.FakeInfrastructureResourceI{}
	_ = mockInfraResource

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.ParseStateFileReturns(statemanager.StateContent{}, nil)
	mockStateManager.RetrieveResourcesReturns(resources, nil)
	mockPlatformProvider.InfrastructreMetadataReturns(nil, fmt.Errorf("infra metadata error"))

	buf := captureSlogOutput()
	err := cmd.RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
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
