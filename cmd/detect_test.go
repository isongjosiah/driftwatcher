package cmd_test

import (
	"bytes"
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/provider/aws"
	"drift-watcher/pkg/services/reporter"
	"drift-watcher/pkg/services/statemanager" // Import for NewTerraformManager
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mocks for interfaces
type MockStateManager struct {
	mock.Mock
}

func (m *MockStateManager) ParseStateFile(ctx context.Context, statePath string) (statemanager.StateContent, error) {
	args := m.Called(ctx, statePath)
	return args.Get(0).(statemanager.StateContent), args.Error(1)
}

func (m *MockStateManager) RetrieveResources(ctx context.Context, content statemanager.StateContent, resourceType string) ([]statemanager.StateResource, error) {
	args := m.Called(ctx, content, resourceType)
	return args.Get(0).([]statemanager.StateResource), args.Error(1)
}

type MockPlatformProvider struct {
	mock.Mock
}

func (m *MockPlatformProvider) InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (provider.InfrastructureResourceI, error) {
	args := m.Called(ctx, resourceType, resource)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(provider.InfrastructureResourceI), args.Error(1)
}

type MockDriftChecker struct {
	mock.Mock
}

func (m *MockDriftChecker) CompareStates(ctx context.Context, liveState provider.InfrastructureResourceI, desiredState statemanager.StateResource, attributesToTrack []string) (*driftchecker.DriftReport, error) {
	args := m.Called(ctx, liveState, desiredState, attributesToTrack)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driftchecker.DriftReport), args.Error(1)
}

type MockOutputWriter struct {
	mock.Mock
}

func (m *MockOutputWriter) WriteReport(ctx context.Context, report *driftchecker.DriftReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

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
	dc := newDetectCmd(ctx, cfg)

	assert.NotNil(t, dc)
	assert.NotNil(t, dc.cmd)
	assert.Equal(t, "detect", dc.cmd.Use)
	assert.Contains(t, dc.cmd.Aliases, "d")

	// Check flags
	assert.True(t, dc.cmd.Flags().HasPersistentFlags()) // Should have flags
	assert.NotNil(t, dc.cmd.Flags().Lookup("configfile"))
	assert.NotNil(t, dc.cmd.Flags().Lookup("attributes"))
	assert.NotNil(t, dc.cmd.Flags().Lookup("provider"))
	assert.NotNil(t, dc.cmd.Flags().Lookup("resource"))
	assert.NotNil(t, dc.cmd.Flags().Lookup("output-file"))
	assert.NotNil(t, dc.cmd.Flags().Lookup("state-manager"))

	// Check default values
	assert.Equal(t, "", dc.tfConfigPath)
	assert.Equal(t, []string{"instance_type"}, dc.AttributesToTrack)
	assert.Equal(t, "aws", dc.Provider)
	assert.Equal(t, "aws_instance", dc.Resource)
	assert.Equal(t, "", dc.OutputPath)
	assert.Equal(t, "terraform", dc.stateManagerType)
}

func TestDetectCmd_Run_MissingConfigFile(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	// dc.tfConfigPath is empty by default

	buf := captureSlogOutput()
	err := dc.Run(dc.cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "A state file is required")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Invalid state file path provided")
}

func TestDetectCmd_Run_UnsupportedStateManager(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "unsupported-manager"

	err := dc.Run(dc.cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported-manager statemanager not currently supported")
}

func TestDetectCmd_Run_UnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "terraform" // Supported
	dc.Provider = "unsupported-provider"

	err := dc.Run(dc.cmd, []string{})
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

func TestDetectCmd_Run_AWSConfigError(t *testing.T) {
	// Temporarily replace the actual aws.CheckAWSConfig with our mock
	originalCheckAWSConfig := aws.CheckAWSConfig
	aws.CheckAWSConfig = mockCheckAWSConfig
	defer func() { aws.CheckAWSConfig = originalCheckAWSConfig }()

	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "terraform"
	dc.Provider = "aws"
	dc.cfg.Profile.AWSConfig.ProfileName = "error-profile" // Trigger mock error

	err := dc.Run(dc.cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock AWS config error")
}

func TestDetectCmd_Run_NewAWSProviderError(t *testing.T) {
	// Temporarily replace the actual aws.CheckAWSConfig with our mock
	originalCheckAWSConfig := aws.CheckAWSConfig
	aws.CheckAWSConfig = mockCheckAWSConfig
	defer func() { aws.CheckAWSConfig = originalCheckAWSConfig }()

	// Temporarily replace aws.NewAWSProvider to simulate an error
	originalNewAWSProvider := aws.NewAWSProvider
	aws.NewAWSProvider = func(cfg *config.AWSConfig) (provider.ProviderI, error) {
		return nil, fmt.Errorf("mock NewAWSProvider error")
	}
	defer func() { aws.NewAWSProvider = originalNewAWSProvider }()

	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "terraform"
	dc.Provider = "aws"
	dc.cfg.Profile.AWSConfig.ProfileName = "default" // Don't trigger CheckAWSConfig error

	err := dc.Run(dc.cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock NewAWSProvider error")
}

func TestDetectCmd_Run_Success_StdoutReporter(t *testing.T) {
	// Setup mocks
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter) // This will be replaced by NewStdoutReporter

	// Mock dependencies for RunDriftDetection
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, mock.Anything).Return([]statemanager.StateResource{}, nil) // No resources for simplicity

	// Temporarily replace the actual aws.CheckAWSConfig with our mock
	originalCheckAWSConfig := aws.CheckAWSConfig
	aws.CheckAWSConfig = mockCheckAWSConfig
	defer func() { aws.CheckAWSConfig = originalCheckAWSConfig }()

	// Temporarily replace aws.NewAWSProvider with a mock
	originalNewAWSProvider := aws.NewAWSProvider
	aws.NewAWSProvider = func(cfg *config.AWSConfig) (provider.ProviderI, error) {
		return mockPlatformProvider, nil
	}
	defer func() { aws.NewAWSProvider = originalNewAWSProvider }()

	// Temporarily replace reporter.NewStdoutReporter to capture it
	originalNewStdoutReporter := reporter.NewStdoutReporter
	reporter.NewStdoutReporter = func() reporter.OutputWriter {
		return mockReporter
	}
	defer func() { reporter.NewStdoutReporter = originalNewStdoutReporter }()

	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "terraform"
	dc.Provider = "aws"
	dc.OutputPath = "" // Ensure stdout reporter is used

	// Manually set internal mocks for RunDriftDetection
	dc.stateManager = mockStateManager
	dc.platformProvider = mockPlatformProvider
	dc.driftChecker = mockDriftChecker

	err := dc.Run(dc.cmd, []string{})
	require.NoError(t, err)

	mockStateManager.AssertCalled(t, "ParseStateFile", mock.Anything, "/tmp/test.tfstate")
	mockStateManager.AssertCalled(t, "RetrieveResources", mock.Anything, mock.Anything, "aws_instance")
	mockPlatformProvider.AssertNotCalled(t, "InfrastructreMetadata", mock.Anything, mock.Anything, mock.Anything) // No resources to process
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)
}

func TestDetectCmd_Run_Success_FileReporter(t *testing.T) {
	// Setup mocks
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	// Mock dependencies for RunDriftDetection
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, mock.Anything).Return([]statemanager.StateResource{}, nil) // No resources for simplicity

	// Temporarily replace the actual aws.CheckAWSConfig with our mock
	originalCheckAWSConfig := aws.CheckAWSConfig
	aws.CheckAWSConfig = mockCheckAWSConfig
	defer func() { aws.CheckAWSConfig = originalCheckAWSConfig }()

	// Temporarily replace aws.NewAWSProvider with a mock
	originalNewAWSProvider := aws.NewAWSProvider
	aws.NewAWSProvider = func(cfg *config.AWSConfig) (provider.ProviderI, error) {
		return mockPlatformProvider, nil
	}
	defer func() { aws.NewAWSProvider = originalNewAWSProvider }()

	// Temporarily replace reporter.NewFileReporter to capture it
	originalNewFileReporter := reporter.NewFileReporter
	reporter.NewFileReporter = func(outputFile string) *reporter.CsvReporter { // Assuming CsvReporter for simplicity of mock interface
		return &reporter.CsvReporter{} // Return a dummy, as we're mocking WriteReport
	}
	defer func() { reporter.NewFileReporter = originalNewFileReporter }()

	ctx := context.Background()
	cfg := &config.Config{}
	dc := newDetectCmd(ctx, cfg)
	dc.tfConfigPath = "/tmp/test.tfstate"
	dc.stateManagerType = "terraform"
	dc.Provider = "aws"
	dc.OutputPath = "/tmp/report.json" // Ensure file reporter is used

	// Manually set internal mocks for RunDriftDetection
	dc.stateManager = mockStateManager
	dc.platformProvider = mockPlatformProvider
	dc.driftChecker = mockDriftChecker
	dc.reporter = mockReporter // Set the mock reporter directly

	err := dc.Run(dc.cmd, []string{})
	require.NoError(t, err)

	mockStateManager.AssertCalled(t, "ParseStateFile", mock.Anything, "/tmp/test.tfstate")
	mockStateManager.AssertCalled(t, "RetrieveResources", mock.Anything, mock.Anything, "aws_instance")
	mockPlatformProvider.AssertNotCalled(t, "InfrastructreMetadata", mock.Anything, mock.Anything, mock.Anything) // No resources to process
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)
}

func TestRunDriftDetection_ParseStateFileError(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, fmt.Errorf("parse error"))

	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/nonexistent.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse state file: parse error")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to parse desired state information from the state file")

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertNotCalled(t, "InfrastructreMetadata", mock.Anything, mock.Anything, mock.Anything)
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)
}

func TestRunDriftDetection_RetrieveResourcesError(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, mock.Anything).Return([]statemanager.StateResource{}, fmt.Errorf("retrieve error"))

	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve resources: retrieve error")
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to retrieve resources from state")

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertNotCalled(t, "InfrastructreMetadata", mock.Anything, mock.Anything, mock.Anything)
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)
}

func TestRunDriftDetection_NoResourcesFound(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, mock.Anything).Return([]statemanager.StateResource{}, nil)

	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "No resources found to check for drift.")

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertNotCalled(t, "InfrastructreMetadata", mock.Anything, mock.Anything, mock.Anything)
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)
}

func TestRunDriftDetection_SuccessWithDrift(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	// Prepare dummy resources
	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resource2 := statemanager.StateResource{Name: "res2", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1, resource2}

	// Mock behaviors
	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, "aws_instance").Return(resources, nil)

	mockLiveResource1 := new(MockInfrastructureResourceI)
	mockLiveResource1.On("ResourceType").Return("aws_instance")
	mockLiveResource1.On("AttributeValue", "instance_type").Return("t2.medium", nil)

	mockLiveResource2 := new(MockInfrastructureResourceI)
	mockLiveResource2.On("ResourceType").Return("aws_instance")
	mockLiveResource2.On("AttributeValue", "instance_type").Return("t2.micro", nil) // No drift

	mockPlatformProvider.On("InfrastructreMetadata", mock.Anything, "aws_instance", resource1).Return(mockLiveResource1, nil)
	mockPlatformProvider.On("InfrastructreMetadata", mock.Anything, "aws_instance", resource2).Return(mockLiveResource2, nil)

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

	mockDriftChecker.On("CompareStates", mock.Anything, mockLiveResource1, resource1, []string{"instance_type"}).Return(driftReport1, nil)
	mockDriftChecker.On("CompareStates", mock.Anything, mockLiveResource2, resource2, []string{"instance_type"}).Return(driftReport2, nil)

	mockReporter.On("WriteReport", mock.Anything, driftReport1).Return(nil)
	mockReporter.On("WriteReport", mock.Anything, driftReport2).Return(nil)

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err)

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertExpectations(t)
	mockDriftChecker.AssertExpectations(t)
	mockReporter.AssertExpectations(t)

	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "Drift detection completed.")
}

func TestRunDriftDetection_InfrastructureMetadataError(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, "aws_instance").Return(resources, nil)
	mockPlatformProvider.On("InfrastructreMetadata", mock.Anything, "aws_instance", resource1).Return(nil, fmt.Errorf("infra metadata error"))

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertExpectations(t)
	mockDriftChecker.AssertNotCalled(t, "CompareStates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to retrieve infrastructure metadata")
	assert.Contains(t, buf.String(), "resource_id=res1")
}

func TestRunDriftDetection_CompareStatesError(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, "aws_instance").Return(resources, nil)

	mockLiveResource1 := new(MockInfrastructureResourceI)
	mockLiveResource1.On("ResourceType").Return("aws_instance") // Ensure this is called for resource type check
	mockPlatformProvider.On("InfrastructreMetadata", mock.Anything, "aws_instance", resource1).Return(mockLiveResource1, nil)

	mockDriftChecker.On("CompareStates", mock.Anything, mockLiveResource1, resource1, []string{"instance_type"}).Return(nil, fmt.Errorf("compare states error"))

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertExpectations(t)
	mockDriftChecker.AssertExpectations(t)
	mockReporter.AssertNotCalled(t, "WriteReport", mock.Anything, mock.Anything)

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to compare states for resource")
	assert.Contains(t, buf.String(), "resource_id=res1")
}

func TestRunDriftDetection_WriteReportError(t *testing.T) {
	mockStateManager := new(MockStateManager)
	mockPlatformProvider := new(MockPlatformProvider)
	mockDriftChecker := new(MockDriftChecker)
	mockReporter := new(MockOutputWriter)

	resource1 := statemanager.StateResource{Name: "res1", Type: "aws_instance"}
	resources := []statemanager.StateResource{resource1}

	mockStateManager.On("ParseStateFile", mock.Anything, mock.Anything).Return(statemanager.StateContent{}, nil)
	mockStateManager.On("RetrieveResources", mock.Anything, mock.Anything, "aws_instance").Return(resources, nil)

	mockLiveResource1 := new(MockInfrastructureResourceI)
	mockLiveResource1.On("ResourceType").Return("aws_instance") // Ensure this is called for resource type check
	mockPlatformProvider.On("InfrastructreMetadata", mock.Anything, "aws_instance", resource1).Return(mockLiveResource1, nil)

	driftReport1 := &driftchecker.DriftReport{HasDrift: true}
	mockDriftChecker.On("CompareStates", mock.Anything, mockLiveResource1, resource1, []string{"instance_type"}).Return(driftReport1, nil)

	mockReporter.On("WriteReport", mock.Anything, driftReport1).Return(fmt.Errorf("write report error"))

	buf := captureSlogOutput()
	err := RunDriftDetection(context.Background(), "/tmp/test.tfstate", "aws_instance", []string{"instance_type"}, mockStateManager, mockPlatformProvider, mockDriftChecker, mockReporter)
	require.NoError(t, err) // Function should continue despite worker error

	mockStateManager.AssertExpectations(t)
	mockPlatformProvider.AssertExpectations(t)
	mockDriftChecker.AssertExpectations(t)
	mockReporter.AssertExpectations(t)

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to write report for resource")
	assert.Contains(t, buf.String(), "resource_id=res1")
}
