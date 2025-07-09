package aws_test

import (
	"bytes"
	awsProvider "drift-watcher/pkg/services/provider/aws"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create dummy AWS config files
func createAwsConfigFiles(t *testing.T, dir string, credsContent, configContent string) (string, string) {
	credsPath := filepath.Join(dir, "credentials")
	configPath := filepath.Join(dir, "config")

	if credsContent != "" {
		err := os.WriteFile(credsPath, []byte(credsContent), 0644)
		require.NoError(t, err)
	}
	if configContent != "" {
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
	}
	return credsPath, configPath
}

// Helper to capture slog output
func captureSlogOutput() *bytes.Buffer {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))
	return &buf
}

func TestCheckAWSConfig_DefaultPathsFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := tmpDir // Use temp dir as home dir for this test

	credsPath := filepath.Join(homeDir, ".aws", "credentials")
	configPath := filepath.Join(homeDir, ".aws", "config")

	err := os.MkdirAll(filepath.Join(homeDir, ".aws"), 0755)
	require.NoError(t, err)

	createAwsConfigFiles(t, filepath.Join(homeDir, ".aws"), "[default]\naws_access_key_id = test", "[profile default]\nregion = us-east-1")

	cfg, err := awsProvider.CheckAWSConfig(homeDir, "")
	require.NoError(t, err)

	assert.Len(t, cfg.CredentialPath, 1)
	assert.Equal(t, credsPath, cfg.CredentialPath[0])
	assert.Len(t, cfg.ConfigPath, 1)
	assert.Equal(t, configPath, cfg.ConfigPath[0])
	assert.Equal(t, "default", cfg.ProfileName)
}

func TestCheckAWSConfig_EnvVarsOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom env files
	customCredsFile := filepath.Join(tmpDir, "custom_creds")
	customConfigFile := filepath.Join(tmpDir, "custom_config")
	createAwsConfigFiles(t, tmpDir, "[default]\naws_access_key_id = env_test", "[profile default]\nregion = env_us-east-1")
	os.Rename(filepath.Join(tmpDir, "credentials"), customCredsFile)
	os.Rename(filepath.Join(tmpDir, "config"), customConfigFile)

	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", customCredsFile)
	os.Setenv("AWS_CONFIG_FILE", customConfigFile)
	defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
	defer os.Unsetenv("AWS_CONFIG_FILE")

	buf := captureSlogOutput()
	cfg, err := awsProvider.CheckAWSConfig("/nonexistent/home", "my-profile") // Use non-existent home to ensure env vars are picked
	require.NoError(t, err)

	assert.Len(t, cfg.CredentialPath, 1)
	assert.Equal(t, customCredsFile, cfg.CredentialPath[0])
	assert.Len(t, cfg.ConfigPath, 1)
	assert.Equal(t, customConfigFile, cfg.ConfigPath[0])
	assert.Equal(t, "my-profile", cfg.ProfileName)

	assert.Contains(t, buf.String(), "AWS credentials file found via AWS_SHARED_CREDENTIALS_FILE")
	assert.Contains(t, buf.String(), "AWS config file found via AWS_CONFIG_FILE")
}

func TestCheckAWSConfig_HomeDirError(t *testing.T) {
	dir := os.TempDir()

	buf := captureSlogOutput()
	cfg, err := awsProvider.CheckAWSConfig(dir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
	assert.Empty(t, cfg.CredentialPath)
	assert.Empty(t, cfg.ConfigPath)
	assert.Contains(t, buf.String(), "Default AWS config file not found")
}

func TestCheckAWSConfig_DefaultCredsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := tmpDir
	os.MkdirAll(filepath.Join(homeDir, ".aws"), 0755)

	// Only create config file, not creds
	createAwsConfigFiles(t, filepath.Join(homeDir, ".aws"), "", "[profile default]\nregion = us-east-1")

	buf := captureSlogOutput()
	_, err := awsProvider.CheckAWSConfig(homeDir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
	assert.Contains(t, buf.String(), "Default AWS credentials file not found")
}

func TestCheckAWSConfig_DefaultConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := tmpDir
	os.MkdirAll(filepath.Join(homeDir, ".aws"), 0755)

	// Only create creds file, not config
	createAwsConfigFiles(t, filepath.Join(homeDir, ".aws"), "[default]\naws_access_key_id = test", "")

	buf := captureSlogOutput()
	_, err := awsProvider.CheckAWSConfig(homeDir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
	assert.Contains(t, buf.String(), "Default AWS config file not found")
}

func TestCheckAWSConfig_EnvCredsFileNotExist(t *testing.T) {
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/env/creds")
	defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

	tmpDir := t.TempDir()
	homeDir := tmpDir
	awsDir := filepath.Join(homeDir, ".aws")
	os.MkdirAll(awsDir, 0755)
	createAwsConfigFiles(t, awsDir, "[default]\naws_access_key_id = test", "[profile default]\nregion = us-east-1")

	buf := captureSlogOutput()
	cfg, err := awsProvider.CheckAWSConfig(homeDir, "")
	require.NoError(t, err) // Should still succeed if default files exist
	assert.Contains(t, buf.String(), "AWS_SHARED_CREDENTIALS_FILE environment variable points to a non-existent file")
	assert.Len(t, cfg.CredentialPath, 1) // Should still have the default path
}

func TestCheckAWSConfig_EnvConfigFileNotExist(t *testing.T) {
	os.Setenv("AWS_CONFIG_FILE", "/nonexistent/env/config")
	defer os.Unsetenv("AWS_CONFIG_FILE")

	tmpDir := t.TempDir()
	homeDir := tmpDir
	awsDir := filepath.Join(homeDir, ".aws")
	os.MkdirAll(awsDir, 0755)
	createAwsConfigFiles(t, awsDir, "[default]\naws_access_key_id = test", "[profile default]\nregion = us-east-1")

	buf := captureSlogOutput()
	cfg, err := awsProvider.CheckAWSConfig(homeDir, "")
	require.NoError(t, err) // Should still succeed if default files exist
	assert.Contains(t, buf.String(), "AWS_CONFIG_FILE environment variable points to a non-existent file")
	assert.Len(t, cfg.ConfigPath, 1) // Should still have the default path
}

func TestCheckAWSConfig_NoPathsFound(t *testing.T) {
	// Ensure no default files and no env vars
	os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
	os.Unsetenv("AWS_CONFIG_FILE")

	tmpDir := t.TempDir()
	// Do not create .aws directory or any files

	buf := captureSlogOutput()
	cfg, err := awsProvider.CheckAWSConfig(tmpDir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Either configuration or credential path is missing")
	assert.Contains(t, buf.String(), "Default AWS credentials file not found")
	assert.Contains(t, buf.String(), "Default AWS config file not found")
	assert.Empty(t, cfg.CredentialPath)
	assert.Empty(t, cfg.ConfigPath)
}

func TestCheckAWSConfig_CustomProfileName(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := tmpDir
	awsDir := filepath.Join(homeDir, ".aws")
	os.MkdirAll(awsDir, 0755)
	createAwsConfigFiles(t, awsDir, "[default]\naws_access_key_id = test", "[profile default]\nregion = us-east-1")

	cfg, err := awsProvider.CheckAWSConfig(homeDir, "my-custom-profile")
	require.NoError(t, err)
	assert.Equal(t, "my-custom-profile", cfg.ProfileName)
}
