package cmd

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/provider/aws"
	"drift-watcher/pkg/terraform"
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
	cmd               *cobra.Command
	cfg               *config.Config
}

func newDetectCmd(cfg *config.Config) *detectCmd {
	config, err := CheckAWSConfig()
	if err != nil {
		slog.Error("Failed to parse aws credentials")
		os.Exit(1)
	}
	cfg.Profile.AWSConfig = &config

	dc := &detectCmd{
		tfConfigPath:      "",
		AttributesToTrack: []string{},
		Provider:          "",
		Resource:          "",
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

	dc.cmd.Flags().StringVar(&dc.tfConfigPath, "configfile", "", "Path to the terraform configuration file")
	dc.cmd.Flags().StringArrayVar(&dc.AttributesToTrack, "attributes", []string{"instance_type"}, "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&dc.cfg.Profile.AWSConfig.ProfileName, "awsprofile", "default", "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&dc.Provider, "provider", "aws", "Name of provider")
	dc.cmd.Flags().StringVar(&dc.Resource, "resource", "ec2", "Resource to check for drift")

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

	parsedContent, err := terraform.ParseTerraformFile(d.tfConfigPath)
	if err != nil {
		slog.Error("Failed to parse terraform configuratio file", "error", err.Error())
		os.Exit(1)
	}
	resources, err := parsedContent.GetResources()
	if err != nil {
		slog.Error("Failed to retrieve resources information from parsed configuration file", "error", err.Error())
		os.Exit(1)
	}

	fmt.Printf("parsed content resources is %#v", resources)

	switch d.Provider {
	case "aws":
		awsProvider, err := aws.NewAWSProvider(d.cfg)
		if err != nil {
			slog.Error("Failed to setup aws provide", "error", err.Error())
			os.Exit(1)
		}
		filter := map[string]string{
			"instance-id": "i-08af3a1b1a9500f2d",
		}
		instance, err := awsProvider.ResourceMetadata(context.Background(), d.Resource, d.AttributesToTrack, filter)
		if err != nil {
			slog.Error("Failed to setup aws provider", "error", err.Error())
		}
		fmt.Printf("%#v", instance)
	default:
		slog.Error(d.Provider + " provider is not supported")
		os.Exit(1)
	}
	return nil
}
