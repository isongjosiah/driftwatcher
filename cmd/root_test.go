package cmd_test

import (
	"context"
	"drift-watcher/cmd"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEnv creates temporary directories and files to simulate AWS config.
// It returns the path to the temporary home directory and a cleanup function.
func setupTestEnv(t *testing.T, createDefaultCreds, createDefaultConfig bool, customCredsContent, customConfigContent string) (string, func()) {
	// Create a temporary home directory
	tempHomeDir := os.TempDir()

	// Create a .aws directory within the temporary home
	tempAWSDir := filepath.Join(tempHomeDir, ".aws")
	err := os.MkdirAll(tempAWSDir, 0755)
	require.NoError(t, err)

	// Create default credentials file if requested
	var defaultCredsFilePath string
	if createDefaultCreds {
		defaultCredsFilePath = filepath.Join(tempAWSDir, "credentials")
		err = ioutil.WriteFile(defaultCredsFilePath, []byte("[default]\naws_access_key_id = default_key"), 0644)
		require.NoError(t, err)
	}

	// Create default config file if requested
	var defaultConfigFilePath string
	if createDefaultConfig {
		defaultConfigFilePath = filepath.Join(tempAWSDir, "config")
		err = os.WriteFile(defaultConfigFilePath, []byte("[default]\nregion = us-east-1"), 0644)
		require.NoError(t, err)
	}

	// Set custom environment variables if content is provided
	var customCredsTempFile *os.File
	if customCredsContent != "" {
		customCredsTempFile, err = os.CreateTemp("", "custom_creds")
		require.NoError(t, err)
		defer customCredsTempFile.Close()
		_, err = customCredsTempFile.WriteString(customCredsContent)
		require.NoError(t, err)
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", customCredsTempFile.Name())
	}

	var customConfigTempFile *os.File
	if customConfigContent != "" {
		customConfigTempFile, err = os.CreateTemp("", "custom_config")
		require.NoError(t, err)
		defer customConfigTempFile.Close()
		_, err = customConfigTempFile.WriteString(customConfigContent)
		require.NoError(t, err)
		os.Setenv("AWS_CONFIG_FILE", customConfigTempFile.Name())
	}

	// Override os.UserHomeDir to point to our temporary home directory
	originalUserHomeDir := os.UserHomeDir
	osUserHomeDir = func() (string, error) {
		return tempHomeDir, nil
	}

	cleanup := func() {
		os.RemoveAll(tempHomeDir)
		if customCredsTempFile.Name() != "" {
			os.Remove(customCredsTempFile.Name())
			os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
		}
		if customConfigTempFile.Name() != "" {
			os.Remove(customConfigTempFile.Name())
			os.Unsetenv("AWS_CONFIG_FILE")
		}
		osUserHomeDir = originalUserHomeDir
	}

	return tempHomeDir, cleanup
}

// A mock for os.UserHomeDir to control the home directory for tests.
// This is initialized with the real os.UserHomeDir, but can be overridden
// by test functions using setupTestEnv.
var osUserHomeDir = os.UserHomeDir

func TestCheckAWSConfig(t *testing.T) {
	// Test Case 1: No AWS config files found
	t.Run("NoConfigFound", func(t *testing.T) {
		_, cleanup := setupTestEnv(t, false, false, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Empty(t, cfg.CredentialPath)
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 2: Default credentials and config files found
	t.Run("DefaultConfigFound", func(t *testing.T) {
		tempHomeDir, cleanup := setupTestEnv(t, true, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.NoError(t, err)
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "credentials"), cfg.CredentialPath[0])
		assert.Len(t, cfg.ConfigPath, 1)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "config"), cfg.ConfigPath[0])
	})

	// Test Case 3: AWS_SHARED_CREDENTIALS_FILE environment variable set
	t.Run("CustomCredsEnvVar", func(t *testing.T) {
		_, cleanup := setupTestEnv(t, false, true, "[custom]\naws_access_key_id = custom_key", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.NoError(t, err)
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Contains(t, cfg.CredentialPath[0], "custom_creds") // Check for temp file name pattern
		assert.Len(t, cfg.ConfigPath, 1)
		assert.Contains(t, cfg.ConfigPath[0], filepath.Join(".aws", "config")) // Default config should still be found
	})

	// Test Case 4: AWS_CONFIG_FILE environment variable set
	t.Run("CustomConfigEnvVar", func(t *testing.T) {
		_, cleanup := setupTestEnv(t, true, false, "", "[custom]\nregion = eu-west-1")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.NoError(t, err)
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Contains(t, cfg.CredentialPath[0], filepath.Join(".aws", "credentials")) // Default creds should still be found
		assert.Len(t, cfg.ConfigPath, 1)
		assert.Contains(t, cfg.ConfigPath[0], "custom_config") // Check for temp file name pattern
	})

	// Test Case 5: Both default and environment variables set (env variables should be appended)
	t.Run("DefaultAndEnvVars", func(t *testing.T) {
		tempHomeDir, cleanup := setupTestEnv(t, true, true, "[custom]\naws_access_key_id = custom_key", "[custom]\nregion = eu-west-1")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.NoError(t, err)

		// Expect default path first, then custom path
		assert.Len(t, cfg.CredentialPath, 2)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "credentials"), cfg.CredentialPath[0])
		assert.Contains(t, cfg.CredentialPath[1], "custom_creds")

		assert.Len(t, cfg.ConfigPath, 2)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "config"), cfg.ConfigPath[0])
		assert.Contains(t, cfg.ConfigPath[1], "custom_config")
	})

	// Test Case 6: AWS_SHARED_CREDENTIALS_FILE points to a non-existent file
	t.Run("EnvVarNonExistentCreds", func(t *testing.T) {
		// Set a non-existent path for the environment variable
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/non/existent/path/creds")
		defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

		// Create only default config, no default creds
		_, cleanup := setupTestEnv(t, false, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing") // Expect error because creds path is missing
		assert.Empty(t, cfg.CredentialPath)
		assert.Len(t, cfg.ConfigPath, 1) // Config path should still be found
	})

	// Test Case 7: AWS_CONFIG_FILE points to a non-existent file
	t.Run("EnvVarNonExistentConfig", func(t *testing.T) {
		// Set a non-existent path for the environment variable
		os.Setenv("AWS_CONFIG_FILE", "/non/existent/path/config")
		defer os.Unsetenv("AWS_CONFIG_FILE")

		// Create only default creds, no default config
		_, cleanup := setupTestEnv(t, true, false, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing") // Expect error because config path is missing
		assert.Len(t, cfg.CredentialPath, 1)                                                  // Creds path should still be found
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 8: Error getting user home directory
	t.Run("UserHomeDirError", func(t *testing.T) {
		originalUserHomeDir := osUserHomeDir
		osUserHomeDir = func() (string, error) {
			return "", errors.New("permission denied")
		}
		defer func() { osUserHomeDir = originalUserHomeDir }()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
		assert.Empty(t, cfg.CredentialPath)
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 9: Only credentials found (expect error)
	t.Run("OnlyCredsFound", func(t *testing.T) {
		_, cleanup := setupTestEnv(t, true, false, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 10: Only config found (expect error)
	t.Run("OnlyConfigFound", func(t *testing.T) {
		_, cleanup := setupTestEnv(t, false, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Empty(t, cfg.CredentialPath)
		assert.Len(t, cfg.ConfigPath, 1)
	})
}

func TestExecute(t *testing.T) {
	// Since Execute interacts with os.Exit and os.Args, testing it directly
	// can be complex. For a robust test, you'd typically want to:
	// 1. Mock os.Exit to prevent the test from exiting the process.
	// 2. Control os.Args for different command line scenarios.
	// 3. Capture fmt.Println output to verify messages.

	// This is a basic example showing how you might mock os.Exit and capture output.
	// For full coverage, you'd need to test different command inputs and expected outputs.

	originalExit := osExit
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		osExit = originalExit
		os.Stdout = originalStdout
	}()

	var exitCode int
	osExit = func(code int) {
		exitCode = code
	}

	// Test case: No command, just --color flag
	t.Run("ColorFlagNoCommand", func(t *testing.T) {
		os.Args = []string{"driftwatcher", "--color", "auto"} // Simulate CLI input
		defer func() { os.Args = []string{"driftwatcher"} }() // Reset for other tests

		cmd.Execute(context.Background())
		w.Close()
		out, _ := ioutil.ReadAll(r)

		assert.Contains(t, string(out), "You provided the \"--color\" flag but did not specify any command. The \"--color\" flag configures the color output of a specified command.")
		assert.Equal(t, 0, exitCode) // Expected exit code 0 for this specific case
	})

	// Test case: Error in cobra command (e.g., unknown command)
	t.Run("CobraCommandError", func(t *testing.T) {
		os.Args = []string{"driftwatcher", "nonexistent-command"}
		defer func() { os.Args = []string{"driftwatcher"} }()

		cmd.Execute(context.Background())
		w.Close()
		out, _ := ioutil.ReadAll(r)

		assert.Contains(t, string(out), "unknown command \"nonexistent-command\" for \"driftwatcher\"")
		assert.Equal(t, 1, exitCode) // Expected exit code 1 for command error
	})

	// Reset os.Args to default for other tests if not done by defer in sub-tests
	os.Args = []string{"driftwatcher"}
}

// Mock os.Exit for testing purposes
var osExit = os.Exit

// Helper to clean up global state (cobra/viper) between tests if necessary.
// In a real scenario, you might have a TestMain function for this.
func TestMain(m *testing.M) {
	// Store original functions/variables to restore after tests
	originalUserHomeDir := osUserHomeDir
	originalExit := osExit
	originalArgs := os.Args

	// Run tests
	code := m.Run()

	// Restore original state
	osUserHomeDir = originalUserHomeDir
	osExit = originalExit
	os.Args = originalArgs
	viper.Reset()
	cobra.OnInitialize()

	os.Exit(code)
}
