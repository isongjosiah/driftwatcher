package config

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type IConfig any

// Config holds the configuration settings for the drift watcher.
// It encapsulates various parameters that control the tool's behavior,
// such as logging verbosity and AWS profile settings.
type Config struct {
	LogLevel    string
	ProfileFile string
	Profile     Profile
}

// GetConfigFolder retrieves the folder where the profiles file is stored
// It searches for the xdg environment path first and will secondarily
// place it in the home directory
func (c *Config) GetConfigFolder(xdgPath string) string {
	configPath := xdgPath

	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		configPath = filepath.Join(home, ".config")
	}

	driftWatcherConfigPath := filepath.Join(configPath, "driftwatcher")
	slog.Debug("Using profiles file", "prefix", "config.Config.GetProfilesFolder", "path", driftWatcherConfigPath)

	return driftWatcherConfigPath
}

func (c *Config) Init() {
	var level slog.Level
	var output io.Writer = os.Stderr

	switch strings.ToLower(c.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "trace": // TODO: implement tracing with open telementry
		level = slog.LevelDebug
	default:
		slog.Error("Unrecognized log level value. Defaulting to 'info'.", "provided_level", c.LogLevel)
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	if c.ProfileFile != "" {
		viper.SetConfigFile(c.ProfileFile)
	} else {
		configFolder := c.GetConfigFolder(os.Getenv("XDG_CONFIG_HOME"))
		configFile := filepath.Join(configFolder, "config.toml")
		c.ProfileFile = configFile
		viper.SetConfigType("toml")
		viper.SetConfigFile(configFile)
		viper.SetConfigPermissions(os.FileMode(0600))

		// Try to change permissions manually, because we used to create files
		// with default permissions (0644)
		err := os.Chmod(configFile, os.FileMode(0600))
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("%s", err)
		}
	}
}

func (c *Config) PrintConfig() error {
	return nil
}

func (c *Config) EditConfig() error {
	return nil
}
