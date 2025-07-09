package cmd

import (
	"context"
	"drift-watcher/config"
	"log/slog"

	"github.com/spf13/cobra"
)

var Config config.Config

var RootCmd = &cobra.Command{
	Use:           "driftwatcher",
	Aliases:       []string{"dw"},
	Short:         "A CLI to help you compare two configurations and detect drift across a list of defined attributes",
	Long:          "CLI to interact with driftwatcher.",
	Version:       "1.0", // TODO: make dynamic
	SilenceErrors: true,
	SilenceUsage:  true,
	Run:           func(cmd *cobra.Command, args []string) {},
}

func Execute(ctx context.Context) {
	RootCmd.SetUsageTemplate("hello world")
	RootCmd.SetVersionTemplate("1.0")
	if err := RootCmd.ExecuteContext(ctx); err != nil {
		slog.Error("Failed to execute command", "error", err)
	}
}

func init() {
	ctx := context.Background()
	cobra.OnInitialize(Config.Init)
	RootCmd.PersistentFlags().StringVar(&Config.LogLevel, "log-level", "info", "log level (debug, info, trace, warn, error)")
	RootCmd.Flags().BoolP("version", "v", false, "Get the version of the DriftWatcher CLI")

	RootCmd.AddCommand(NewDetectCmd(ctx, &Config).Cmd)
	RootCmd.AddCommand(newConfigCmd().cmd)
}
