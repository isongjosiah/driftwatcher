package cmd

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/provider/aws"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

type detectCmd struct {
	tfConfigPath      string
	AttributesToTrack []string
	Provider          string
	Resource          string
	awsConfig         config.AWSConfig
	cmd               *cobra.Command
	cfg               *config.Config
}

func newDetectCmd(cfg *config.Config) *detectCmd {
	dc := &detectCmd{
		tfConfigPath:      "",
		AttributesToTrack: []string{},
		Provider:          "",
		Resource:          "",
		awsConfig:         config.AWSConfig{},
		cmd:               &cobra.Command{},
		cfg:               cfg,
	}
	dc.cmd = &cobra.Command{
		Use:     "detect",
		Aliases: []string{"d"},
		Short:   "Detect drift between your configurationa file and EC2 metadata instance from AWS",
		Long:    "Hello world",
		RunE:    dc.Run,
	}

	var profileName string
	dc.cmd.Flags().StringVar(&dc.tfConfigPath, "configfile", "", "Path to the terraform configuration file")
	dc.cmd.Flags().StringArrayVar(&dc.AttributesToTrack, "attributes", []string{"instance_type"}, "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&profileName, "aws-profile", "default", "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&dc.Provider, "provider", "aws", "Name of provider")
	dc.cmd.Flags().StringVar(&dc.Resource, "resource", "ec2", "Resource to check for drift")

	config, err := CheckAWSConfig()
	if err != nil {
		slog.Error("Failed to parse aws credentials")
		os.Exit(1)
	}

	config.ProfileName = profileName
	dc.cfg.Profile.AWSConfig = &config

	return dc
}

func (d *detectCmd) Run(cmd *cobra.Command, args []string) error {
	if d.tfConfigPath == "" {
		slog.Error("Invalid configuration file path provided")
		os.Exit(1)
	}

	if _, err := os.Stat(d.tfConfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Error(fmt.Sprintf("file %s does not exist", d.tfConfigPath))
		} else {
			slog.Error(fmt.Sprintf("failed to read file %s", d.tfConfigPath))
		}
		os.Exit(1)
	}

	switch d.Provider {
	case "aws":
		awsProvider, err := aws.NewAWSProvider(d.cfg)
		if err != nil {
			slog.Error("Failed to setup aws provide", "error", err.Error())
			os.Exit(1)
		}
		instance, err := awsProvider.ResourceMetadata(context.Background(), d.Resource, d.AttributesToTrack, map[string]string{})
		if err != nil {
			slog.Error("Failed to setup aws provider", "error", err.Error())
		}
		print(instance)
	default:
		slog.Error(d.Provider + " provider is not supported")
		os.Exit(1)
	}
	return nil
}
