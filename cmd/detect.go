package cmd

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/provider"
	"drift-watcher/pkg/provider/aws"
	"drift-watcher/pkg/terraform"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

type detectCmd struct {
	tfConfigPath      string
	OutputPath        string
	ctx               context.Context
	AttributesToTrack []string
	Provider          string
	Resource          string
	cmd               *cobra.Command
	cfg               *config.Config
}

func newDetectCmd(cfg *config.Config) *detectCmd {
	config, err := CheckAWSConfig("")
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
		ctx:               context.Background(),
	}
	dc.cmd = &cobra.Command{
		Use:     "detect",
		Aliases: []string{"d"},
		Short:   "Detect drift between your configurationa file and EC2 metadata instance from AWS",
		Long:    "Hello world",
		RunE:    dc.Run,
	}

	dc.cmd.Flags().StringVar(&dc.tfConfigPath, "configfile", "", "Path to the terraform configuration file")
	dc.cmd.Flags().StringSliceVar(&dc.AttributesToTrack, "attributes", []string{"instance_type"}, "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&dc.cfg.Profile.AWSConfig.ProfileName, "awsprofile", "default", "Attributes to check for drift")
	dc.cmd.Flags().StringVar(&dc.Provider, "provider", "aws", "Name of provider")
	dc.cmd.Flags().StringVar(&dc.Resource, "resource", "ec2", "Resource to check for drift")
	dc.cmd.Flags().StringVar(&dc.OutputPath, "output-file", "", "Resource to check for drift")

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

	var providerImpl provider.ProviderI

	switch d.Provider {
	case "aws":
		providerImpl, err = aws.NewAWSProvider(d.cfg)
		if err != nil {
			slog.Error("Failed to setup aws provide", "error", err.Error())
			os.Exit(1)
		}
	default:
		slog.Error(d.Provider + " provider is not supported")
		os.Exit(1)
	}

	wg := &sync.WaitGroup{}
	maxWorker := 5
	channel := make(chan terraform.Resource, maxWorker)

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for resource := range channel {

				var filterKey, filterValue string
				switch parsedContent.Type {
				case "hcl":
					filterKey = "tag:Name"
					filterValue = resource.Name
				case "state":
					filterKey = "instance-id"
					filterValue = resource.Instances[0].Attributes.ID
				}

				filter := map[string]string{
					filterKey: filterValue,
				}

				infrastructureData, err := providerImpl.InfrastructreMetadata(d.ctx, d.Resource, filter)
				if err != nil {
					slog.Error("Failed to retrieve infrastructure information", "error", err.Error())
					os.Exit(1)
				}
				report, err := providerImpl.CompareActiveAndDesiredState(d.ctx, d.Resource, infrastructureData, resource, d.AttributesToTrack)
				if err != nil {
					slog.Error("Failed to compare infrastructure state", "error", err.Error())
				}
				if d.OutputPath != "" {
				} else {
					// Marshal with 4 spaces for indentation
					prettyJSON, err := json.MarshalIndent(report, "", "    ")
					if err != nil {
						log.Fatalf("Error marshaling JSON: %v", err)
					}

					fmt.Println(string(prettyJSON))
				}

			}
		}()
	}
	for _, resource := range resources {
		// NOTE: focus on aws instances for now.
		// we are assuming that aws library is concurrency
		// safe - confirm this
		if resource.Type == "aws_instance" {
			channel <- resource
		}
	}
	close(channel)

	wg.Wait()
	return nil
}
