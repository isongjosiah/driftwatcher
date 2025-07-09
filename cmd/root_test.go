package cmd_test

import (
	"bytes"
	"context"
	"drift-watcher/config" // Assuming config package exists
	"log/slog"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Helper to capture slog output
func captureSlogOutput() *bytes.Buffer {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))
	return &buf
}

func TestExecute_Success(t *testing.T) {
	// Reset Cobra commands to ensure clean state for testing init()
	// This is a common pattern for testing Cobra apps.
	cobra.ResetCommands()

	// Capture stdout to prevent actual help output during test
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Initialize Config (mimic cobra.OnInitialize)
	// This is usually done by the main package, but for isolated testing, we do it here.
	Config = config.Config{}
	Config.Init()

	// Re-add commands after resetting Cobra, so init() runs again
	init()

	ctx := context.Background()
	buf := captureSlogOutput() // Capture slog output after init

	// Simulate running with a known command that doesn't error
	rootCmd.SetArgs([]string{"detect", "--configfile", "/tmp/dummy.tfstate"})
	Execute(ctx)

	w.Close()
	os.Stdout = oldStdout
	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(r) // Read captured stdout

	// No error expected, but slog might log info/debug from command setup
	assert.Empty(t, buf.String(), "Expected no slog errors for successful execution")
	// No specific stdout output expected for this simple detect command mock
	// assert.Empty(t, stdoutBuf.String())
}

func TestExecute_CommandError(t *testing.T) {
	// Reset Cobra commands to ensure clean state for testing init()
	cobra.ResetCommands()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Initialize Config (mimic cobra.OnInitialize)
	Config = config.Config{}
	Config.Init()

	// Re-add commands after resetting Cobra, so init() runs again
	init()

	ctx := context.Background()
	buf := captureSlogOutput()

	// Simulate running a command that will cause an error (e.g., missing required flag)
	rootCmd.SetArgs([]string{"detect"}) // Missing --configfile, which is required
	Execute(ctx)

	w.Close()
	os.Stdout = oldStdout
	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(r)

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "Failed to execute command")
	assert.Contains(t, buf.String(), "A state file is required") // Error from detectCmd.Run

	// Cobra's default behavior for missing required flags prints usage to stderr (which slog captures)
	// and returns an error.
}

func TestRootCmd_FlagsAndCommands(t *testing.T) {
	// This test relies on the `init()` function being called, which Cobra handles.
	// We'll just check if the commands and flags are registered correctly.
	cobra.ResetCommands() // Ensure clean state
	init()                // Manually call init to ensure commands are added

	assert.Equal(t, "driftwatcher", rootCmd.Use)
	assert.Contains(t, rootCmd.Aliases, "dw")
	assert.Equal(t, "1.0", rootCmd.Version) // Check static version

	// Check persistent flags
	logLevelFlag := rootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, logLevelFlag)
	assert.Equal(t, "log-level", logLevelFlag.Name)
	assert.Equal(t, "info", logLevelFlag.DefValue)

	versionFlag := rootCmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag)
	assert.Equal(t, "version", versionFlag.Name)
	assert.Equal(t, "v", versionFlag.Shorthand)

	// Check subcommands
	detectCmdFound := false
	configCmdFound := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "detect" {
			detectCmdFound = true
		}
		if cmd.Use == "config" {
			configCmdFound = true
		}
	}
	assert.True(t, detectCmdFound, "detect command should be added")
	assert.True(t, configCmdFound, "config command should be added")
}
