package aws

import (
	"drift-watcher/config"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// CheckAWSConfig checks for the presence of AWS configuration files
// or environment variables that point to them.
// It returns true if a configuration file is found, along with the path to the first one found.
// It logs debug messages indicating where it's looking and what it finds.
func CheckAWSConfig(homeDir string, profile string) (config.AWSConfig, error) {
	configDetail := config.AWSConfig{
		CredentialPath: []string{},
		ConfigPath:     []string{},
	}

	var err error
	// attempt to load from the default location
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			slog.Error("Failed to get user home directory", "error", err)
			return configDetail, err
		}
	}

	defaultAWSPath := filepath.Join(homeDir, ".aws")
	slog.Debug("Checking default AWS configuration directory", "path", defaultAWSPath)

	// Check for default credentials file
	defaultCredsFile := filepath.Join(defaultAWSPath, "credentials")
	if _, err := os.Stat(defaultCredsFile); err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Default AWS credentials file not found", "path", defaultCredsFile)
		} else {
			slog.Error("Error checking default AWS credentials file", "path", defaultCredsFile, "error", err)
			return configDetail, err
		}
	} else {
		// default credential found
		configDetail.CredentialPath = append(configDetail.CredentialPath, defaultCredsFile)
	}

	// check for default profile file
	defaultConfigFiles := filepath.Join(defaultAWSPath, "config")
	if _, err := os.Stat(defaultConfigFiles); err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Default AWS config file not found", "path", defaultCredsFile)
		} else {
			slog.Error("Error checking default AWS config file", "path", defaultCredsFile, "error", err)
			return configDetail, err
		}
	} else {
		// default credential found
		configDetail.ConfigPath = append(configDetail.ConfigPath, defaultConfigFiles)
	}

	// NOTE: we want to handle for situations where the user has set a custom path in their environment variable
	// so if a custom path is found, it will overwrite the user's default path.
	// TODO: might make sense to allow the user define if they want to prioritize custom or default paths, but that
	// is a non-functional requirement, so we'll come back to this. For now we default to custom paths if they exist
	credsFileEnv := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credsFileEnv != "" {
		slog.Debug("Checking AWS_SHARED_CREDENTIALS_FILE environment variable", "path_env", credsFileEnv)

		if _, err := os.Stat(credsFileEnv); err != nil {
			if os.IsNotExist(err) {
				slog.Warn("AWS_SHARED_CREDENTIALS_FILE environment variable points to a non-existent file", "path", credsFileEnv)
			} else {
				slog.Error("Error checking file specified by AWS_SHARED_CREDENTIALS_FILE", "path", credsFileEnv, "error", err)
			}
		} else {
			configDetail.CredentialPath = append(configDetail.CredentialPath, credsFileEnv)
			slog.Info("AWS credentials file found via AWS_SHARED_CREDENTIALS_FILE", "path", credsFileEnv)
		}
	}

	if configFileEnv := os.Getenv("AWS_CONFIG_FILE"); configFileEnv != "" {
		slog.Debug("Checking AWS_CONFIG_FILE environment variable", "path_env", configFileEnv)
		if _, err := os.Stat(configFileEnv); err != nil {
			if os.IsNotExist(err) {
				slog.Warn("AWS_CONFIG_FILE environment variable points to a non-existent file", "path", credsFileEnv)
			} else {
				slog.Error("Error checking file specified by AWS_CONFG_FILE", "path", credsFileEnv, "error", err)
			}
		} else {
			configDetail.ConfigPath = append(configDetail.ConfigPath, configFileEnv)
			slog.Info("AWS config file found via AWS_CONFIG_FILE", "path", configFileEnv)
		}
	}

	if len(configDetail.ConfigPath) == 0 || len(configDetail.CredentialPath) == 0 {
		return configDetail, fmt.Errorf("Either configuration or credential path is missing")
	}

	configDetail.ProfileName = profile
	if configDetail.ProfileName == "" {
		configDetail.ProfileName = "default"
	}

	return configDetail, nil
}
