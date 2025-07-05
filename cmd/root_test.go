package cmd_test

import (
	"bytes"
	"context"
	"drift-watcher/cmd"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		err = os.WriteFile(defaultCredsFilePath, []byte("[default]\naws_access_key_id = default_key"), 0644)
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
		if customCredsTempFile != nil && customCredsTempFile.Name() != "" {
			os.Remove(customCredsTempFile.Name())
			os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
		}
		if customConfigTempFile != nil && customConfigTempFile.Name() != "" {
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
		tempHomeDir, cleanup := setupTestEnv(t, true, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tempHomeDir)
		assert.NoError(t, err)
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "credentials"), cfg.CredentialPath[0])
		assert.Len(t, cfg.ConfigPath, 1)
		assert.Equal(t, filepath.Join(tempHomeDir, ".aws", "config"), cfg.ConfigPath[0])
	})

	// Test Case 3: AWS_SHARED_CREDENTIALS_FILE environment variable set
	t.Run("CustomCredsEnvVar", func(t *testing.T) {
		tempHomeDir, cleanup := setupTestEnv(t, false, true, "[custom]\naws_access_key_id = custom_key", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tempHomeDir)
		assert.NoError(t, err)
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Contains(t, cfg.CredentialPath[0], "custom_creds")
		assert.Len(t, cfg.ConfigPath, 1)
		assert.Contains(t, cfg.ConfigPath[0], filepath.Join(".aws", "config"))
	})

	// Test Case 4: AWS_CONFIG_FILE environment variable set
	t.Run("CustomConfigEnvVar", func(t *testing.T) {
		tempHomeDir, cleanup := setupTestEnv(t, true, false, "", "[custom]\nregion = eu-west-1")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tempHomeDir)
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

		cfg, err := cmd.CheckAWSConfig(tempHomeDir)
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
		tempHomeDir, cleanup := setupTestEnv(t, false, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tempHomeDir)
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
		tmpHomeDir, cleanup := setupTestEnv(t, true, false, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tmpHomeDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Len(t, cfg.CredentialPath, 1) // Creds path should still be found
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 9: Only credentials found (expect error)
	t.Run("OnlyCredsFound", func(t *testing.T) {
		tmpHomeDir, cleanup := setupTestEnv(t, true, false, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tmpHomeDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Len(t, cfg.CredentialPath, 1)
		assert.Empty(t, cfg.ConfigPath)
	})

	// Test Case 10: Only config found (expect error)
	t.Run("OnlyConfigFound", func(t *testing.T) {
		tmpHomeDir, cleanup := setupTestEnv(t, false, true, "", "")
		defer cleanup()

		cfg, err := cmd.CheckAWSConfig(tmpHomeDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
		assert.Empty(t, cfg.CredentialPath)
		assert.Len(t, cfg.ConfigPath, 1)
	})
}

func TestExecute(t *testing.T) {
	t.Run("CobraCommandError", func(t *testing.T) {
		// Check if we're in the subprocess
		if os.Getenv("BE_CRASHER") == "1" {
			// This runs in the subprocess - simulate the actual execution
			os.Args = []string{"driftwatcher", "nonexistent-command"}
			cmd.Execute(context.Background())
			return
		}

		// Run the test in a subprocess using 'go test'
		testCmd := exec.Command("go", "test", "-run=^TestExecute/CobraCommandError$", "-v")
		testCmd.Env = append(os.Environ(), "BE_CRASHER=1")

		// Set the working directory to current directory
		if wd, err := os.Getwd(); err == nil {
			testCmd.Dir = wd
		}

		// Capture both stdout and stderr
		var stdout, stderr bytes.Buffer
		testCmd.Stdout = &stdout
		testCmd.Stderr = &stderr

		// Run the command
		err := testCmd.Run()

		// Get combined output
		output := stdout.String() + stderr.String()
		t.Logf("Subprocess output: %q", output)

		// Check exit code - go test returns 1 for test failures
		if exitError, ok := err.(*exec.ExitError); ok {
			// We expect the subprocess to exit with code 1 (test failure due to os.Exit)
			if exitError.ExitCode() != 1 {
				t.Errorf("Expected exit code 1, got %d", exitError.ExitCode())
			}
		} else if err != nil {
			t.Fatalf("Unexpected error running subprocess: %v", err)
		} else {
			t.Fatal("Expected subprocess to exit with error, but it succeeded")
		}

		// The actual command output might be in the test output, so check for the error message
		if !strings.Contains(output, "unknown command") || !strings.Contains(output, "nonexistent-command") {
			t.Errorf("Expected error message not found in output: %q", output)
		}
	})
}

// Helper to clean up global state (cobra/viper) between tests if necessary.
// In a real scenario, you might have a TestMain function for this.
func TestMain(m *testing.M) {
	// Store original functions/variables to restore after tests
	originalArgs := os.Args

	// Run tests
	code := m.Run()

	// Restore original state
	os.Args = originalArgs
	viper.Reset()
	cobra.OnInitialize()

	os.Exit(code)
}
