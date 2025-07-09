package cmd_test

import (

	// Assuming config package exists

	"drift-watcher/cmd"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_FlagsAndCommands(t *testing.T) {
	// This test relies on the `init()` function being called, which Cobra handles.
	// We'll just check if the commands and flags are registered correctly.

	assert.Equal(t, "driftwatcher", cmd.RootCmd.Use)
	assert.Equal(t, "1.0", cmd.RootCmd.Version) // Check static version

	// Check persistent flags
	logLevelFlag := cmd.RootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, logLevelFlag)
	assert.Equal(t, "log-level", logLevelFlag.Name)
	assert.Equal(t, "info", logLevelFlag.DefValue)

	versionFlag := cmd.RootCmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag)
	assert.Equal(t, "version", versionFlag.Name)
	assert.Equal(t, "v", versionFlag.Shorthand)

	// Check subcommands
	detectCmdFound := false
	configCmdFound := false
	for _, cmd := range cmd.RootCmd.Commands() {
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
