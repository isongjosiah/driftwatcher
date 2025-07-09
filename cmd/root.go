package cmd

import (
	"context"
	"drift-watcher/config"
	"log/slog"

	"github.com/spf13/cobra"
)

var Config config.Config

var rootCmd = &cobra.Command{
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
	rootCmd.SetUsageTemplate("hello world")
	rootCmd.SetVersionTemplate("1.0")
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error("Failed to execute command", "error", err)
	}
}

func init() {
	ctx := context.Background()
	cobra.OnInitialize(Config.Init)
	rootCmd.PersistentFlags().StringVar(&Config.LogLevel, "log-level", "info", "log level (debug, info, trace, warn, error)")
	rootCmd.Flags().BoolP("version", "v", false, "Get the version of the DriftWatcher CLI")

	rootCmd.AddCommand(newDetectCmd(ctx, &Config).cmd)
	rootCmd.AddCommand(newConfigCmd().cmd)
}
