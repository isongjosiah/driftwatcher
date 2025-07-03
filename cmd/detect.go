package cmd

import (
	"drift-watcher/config"
	"log/slog"

	"github.com/spf13/cobra"
)

type detectCmd struct {
	tfConfigPath      string
	AttributesToTrack []string
	cmd               *cobra.Command
	cfg               *config.Config
}

func newDetectCmd(cfg *config.Config) *detectCmd {
	dc := &detectCmd{
		cfg: cfg,
	}
	dc.cmd = &cobra.Command{
		Use:     "detect",
		Aliases: []string{"d"},
		Short:   "Detect drift between your configurationa file and EC2 metadata instance from AWS",
		RunE:    dc.Run,
	}

	dc.cmd.Flags().StringVar(&dc.tfConfigPath, "config-file", "", "Path to the terraform configuration file")
	dc.cmd.Flags().StringArrayVar(&dc.AttributesToTrack, "tracked-attributes", []string{"instance_type"}, "Attributes to check for drift")

	return dc
}

func (d *detectCmd) Run(cmd *cobra.Command, args []string) error {
	slog.Info("Running detection")
	return nil
}
