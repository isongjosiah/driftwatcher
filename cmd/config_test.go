package cmd_test

import (
	"bytes"
	"drift-watcher/config"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockConfig is a mock implementation of config.Config for testing
type MockConfig struct {
	mock.Mock
	// Embed a mock for Profile if needed, or mock its methods directly
	MockProfile *MockProfile
}

func (m *MockConfig) Init() {
	m.Called()
}

func (m *MockConfig) PrintConfig() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConfig) EditConfig() error {
	args := m.Called()
	return args.Error(0)
}

// MockProfile is a mock for config.Profile
type MockProfile struct {
	mock.Mock
}

func (m *MockProfile) WriteConfigField(field, value string) error {
	args := m.Called(field, value)
	return args.Error(0)
}

func (m *MockProfile) DeleteConfigField(field string) error {
	args := m.Called(field)
	return args.Error(0)
}

func TestNewConfigCmd(t *testing.T) {
	// Temporarily set the global Config variable to a mock
	originalConfig := Config
	mockGlobalConfig := &config.Config{} // Use a real config.Config for initialization
	Config = *mockGlobalConfig           // Assign by value or pointer based on how Config is used
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()

	assert.NotNil(t, cc)
	assert.NotNil(t, cc.cmd)
	assert.Equal(t, "config", cc.cmd.Use)
	assert.Contains(t, cc.cmd.Short, "Manually change the config values")

	// Check flags
	assert.NotNil(t, cc.cmd.Flags().Lookup("list"))
	assert.NotNil(t, cc.cmd.Flags().Lookup("edit"))
	assert.NotNil(t, cc.cmd.Flags().Lookup("unset"))
	assert.NotNil(t, cc.cmd.Flags().Lookup("set"))

	// Ensure SetInterspersed is called
	assert.False(t, cc.cmd.Flags().GetInterspersed())
}

func TestRunConfigCmd_SetSuccess(t *testing.T) {
	mockProfile := new(MockProfile)
	mockProfile.On("WriteConfigField", "color", "off").Return(nil)

	mockConfig := &MockConfig{MockProfile: mockProfile}
	mockConfig.On("Init").Return() // Mock Init if it's called by the global Config.Init()

	// Replace the global Config with our mock for the test duration
	originalConfig := Config
	Config = config.Config{
		Profile: mockProfile, // Inject the mock profile
	}
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.set = true // Simulate --set flag
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{"color", "off"})
	require.NoError(t, err)
	mockProfile.AssertExpectations(t)
}

func TestRunConfigCmd_SetError(t *testing.T) {
	mockProfile := new(MockProfile)
	mockProfile.On("WriteConfigField", "color", "on").Return(fmt.Errorf("write error"))

	mockConfig := &MockConfig{MockProfile: mockProfile}
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = config.Config{
		Profile: mockProfile,
	}
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.set = true
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{"color", "on"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write error")
	mockProfile.AssertExpectations(t)
}

func TestRunConfigCmd_UnsetSuccess(t *testing.T) {
	mockProfile := new(MockProfile)
	mockProfile.On("DeleteConfigField", "color").Return(nil)

	mockConfig := &MockConfig{MockProfile: mockProfile}
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = config.Config{
		Profile: mockProfile,
	}
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.unset = "color" // Simulate --unset flag
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	require.NoError(t, err)
	mockProfile.AssertExpectations(t)
}

func TestRunConfigCmd_UnsetError(t *testing.T) {
	mockProfile := new(MockProfile)
	mockProfile.On("DeleteConfigField", "color").Return(fmt.Errorf("delete error"))

	mockConfig := &MockConfig{MockProfile: mockProfile}
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = config.Config{
		Profile: mockProfile,
	}
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.unset = "color"
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete error")
	mockProfile.AssertExpectations(t)
}

func TestRunConfigCmd_ListSuccess(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("PrintConfig").Return(nil)
	mockConfig.On("Init").Return() // Mock Init if it's called by the global Config.Init()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.list = true // Simulate --list flag
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	require.NoError(t, err)
	mockConfig.AssertExpectations(t)
}

func TestRunConfigCmd_ListError(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("PrintConfig").Return(fmt.Errorf("print error"))
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.list = true
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "print error")
	mockConfig.AssertExpectations(t)
}

func TestRunConfigCmd_EditSuccess(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("EditConfig").Return(nil)
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.edit = true // Simulate --edit flag
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	require.NoError(t, err)
	mockConfig.AssertExpectations(t)
}

func TestRunConfigCmd_EditError(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("EditConfig").Return(fmt.Errorf("edit error"))
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.edit = true
	cmd := &cobra.Command{}

	err := cc.runConfigCmd(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "edit error")
	mockConfig.AssertExpectations(t)
}

func TestRunConfigCmd_DefaultCase(t *testing.T) {
	// Capture stdout to check help message
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mockConfig := new(MockConfig)
	mockConfig.On("Init").Return() // Mock Init if it's called by the global Config.Init()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	// No flags set, or invalid args for --set
	cmd := &cobra.Command{
		Use: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help() // This is what runConfigCmd calls in default
		},
	}
	cc.cmd = cmd // Replace the command with one that has a Help method

	err := cc.runConfigCmd(cmd, []string{}) // No flags, no args for --set
	require.NoError(t, err)                 // Help() returns nil error

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.Contains(t, buf.String(), "Usage:")
	assert.Contains(t, buf.String(), "Manually change the config values for the CLI")
	mockConfig.AssertNotCalled(t, "PrintConfig")
	mockConfig.AssertNotCalled(t, "EditConfig")
	mockConfig.AssertNotCalled(t, "WriteConfigField")
	mockConfig.AssertNotCalled(t, "DeleteConfigField")
}

func TestRunConfigCmd_SetInvalidArgs(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("Init").Return()

	originalConfig := Config
	Config = *mockConfig
	defer func() { Config = originalConfig }()

	cc := newConfigCmd()
	cc.set = true
	cmd := &cobra.Command{
		Use: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help() // This is what runConfigCmd calls in default
		},
	}
	cc.cmd = cmd

	// Simulate --set with only one argument
	err := cc.runConfigCmd(cmd, []string{"color"})
	require.NoError(t, err) // Help() returns nil error

	// Verify that WriteConfigField was NOT called
	mockConfig.AssertNotCalled(t, "WriteConfigField")
}
