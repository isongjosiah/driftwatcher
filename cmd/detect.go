package cmd

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/provider/aws"
	"drift-watcher/pkg/services/reporter"
	"drift-watcher/pkg/services/statemanager"
	"drift-watcher/pkg/services/statemanager/terraform"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

type detectCmd struct {
	StateManager      statemanager.StateManagerI
	PlatformProvider  provider.ProviderI
	DriftChecker      driftchecker.DriftChecker
	Reporter          reporter.OutputWriter
	Profile           string
	LocalStackRegion  string
	Provider          string
	Resource          string
	TfConfigPath      string
	OutputPath        string
	StateManagerType  string
	LocalStackUrl     string
	AttributesToTrack []string
	ctx               context.Context
	Cmd               *cobra.Command
	cfg               *config.Config
}

// newDetectCmd creates and configures the 'detect' Cobra command.
// This command is responsible for identifying configuration drift between
// a Terraform configuration file and the live state of resources in a cloud environment (e.g., AWS EC2).
//
// Parameters:
//
//	ctx: The context for the command's execution, allowing for cancellation or timeouts.
//	cfg: The application's global configuration, containing settings like AWS profile.
//	stateManager: An interface for managing and accessing state information (e.g., Terraform state).
//	platformProvider: An interface for interacting with the cloud platform (e.g., AWS API calls).
//
// Returns:
//
//	A pointer to a detectCmd struct, which encapsulates the Cobra command and its dependencies.
func NewDetectCmd(ctx context.Context, cfg *config.Config) *detectCmd {
	dc := &detectCmd{
		cfg: cfg,
		ctx: ctx,
	}
	dc.Cmd = &cobra.Command{
		Use:     "detect",
		Aliases: []string{"d"},
		Short:   "Detect drift between your configuration file and EC2 metadata instance from AWS",
		Long: `This command is designed to identify drift between your infrastructure-as-code (IaC) and your live AWS environment. Specifically, it compares the specified attributes within your Terraform configuration file (e.g., instance type, AMI ID) against the actual metadata of your running EC2 instances. By highlighting these differences, you can quickly spot unauthorized changes, misconfigurations, or manual modifications that deviate from your desired state, ensuring your infrastructure remains consistent and compliant.

For example:
  # Check for instance type drift using a specific config file
  yourcommand detect --configfile /path/to/your/main.tf --attributes instance_type

  # Check multiple attributes and specify an AWS profile
  yourcommand detect --configfile /path/to/your/main.tf --attributes instance_type,ami --awsprofile my-dev-profile

  # Output the drift report to a file
  yourcommand detect --configfile /path/to/your/main.tf --output-file drift_report.json
`,
		RunE: dc.Run,
	}

	dc.Cmd.Flags().StringVar(&dc.TfConfigPath, "configfile", "", "Path to the terraform configuration file")
	dc.Cmd.Flags().StringSliceVar(&dc.AttributesToTrack, "attributes", []string{"instance_type"}, "Attributes to check for drift")
	dc.Cmd.Flags().StringVar(&dc.Profile, "awsprofile", "default", "Attributes to check for drift")
	dc.Cmd.Flags().StringVar(&dc.LocalStackRegion, "localstackregion", "", "Attributes to check for drift")
	dc.Cmd.Flags().StringVar(&dc.Provider, "provider", "aws", "Name of provider")
	dc.Cmd.Flags().StringVar(&dc.Resource, "resource", "aws_instance", "Resource to check for drift")
	dc.Cmd.Flags().StringVar(&dc.OutputPath, "output-file", "", "Resource to check for drift")
	dc.Cmd.Flags().StringVar(&dc.StateManagerType, "state-manager", "terraform", "Resource to check for drift")
	dc.Cmd.Flags().StringVar(&dc.LocalStackUrl, "localstack-url", "", "Resource to check for drift")

	return dc
}

func (d *detectCmd) Run(cmd *cobra.Command, args []string) error {
	if d.TfConfigPath == "" {
		slog.Error("Invalid state file path provided")
		return fmt.Errorf("A state file is required")
	}

	if d.StateManager == nil {
		switch d.StateManagerType {
		case "terraform":
			d.StateManager = terraform.NewTerraformManager()
		default:
			return fmt.Errorf("%s statemanager not currently supported", d.StateManagerType)
		}
	}

	if d.LocalStackUrl != "" {
		if d.LocalStackRegion == "" {
			// NOTE: we are not setting this as a default in the flag, so that if a user is interacting with AWS directly
			// we don't overwrite their region, since we are relying on aws ignoring empty region string to dynamically
			// load the region
			d.LocalStackRegion = "us-east-1"
		}
		os.Setenv("DRIFT_LOCALSTACK_URL", d.LocalStackUrl)
		os.Setenv("DRIFT_LOCALSTACK_REGION", d.LocalStackRegion)
		defer os.Unsetenv("DRIFT_LOCALSTACK_URL")
		defer os.Unsetenv("DRIFT_LOCALSTACK_REGION")
	}

	if d.PlatformProvider == nil {
		switch d.Provider {
		case "aws":
			config, err := aws.CheckAWSConfig("", d.Profile)
			if err != nil {
				return err
			}

			provider, err := aws.NewAWSProvider(&config)
			if err != nil {
				return err
			}
			d.PlatformProvider = provider
		default:
			return fmt.Errorf("%s platform not currently supported", d.Provider)
		}
	}

	if d.DriftChecker == nil {
		d.DriftChecker = driftchecker.NewDefaultDriftChecker()
	}

	if d.Reporter == nil {
		if d.OutputPath != "" {
			d.Reporter = reporter.NewFileReporter(d.OutputPath)
		} else {
			d.Reporter = reporter.NewStdoutReporter()
		}
	}

	return RunDriftDetection(d.ctx, d.TfConfigPath, d.Resource, d.AttributesToTrack, d.StateManager, d.PlatformProvider, d.DriftChecker, d.Reporter)
}

func RunDriftDetection(
	ctx context.Context,
	tfConfigPath string,
	resourceType string,
	attributesToTrack []string,
	stateManager statemanager.StateManagerI,
	platformProvider provider.ProviderI,
	driftChecker driftchecker.DriftChecker,
	reporter reporter.OutputWriter,
) error {
	stateContent, err := stateManager.ParseStateFile(ctx, tfConfigPath)
	if err != nil {
		slog.Error("Failed to parse desired state information from the state file", "error", err)
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	resources, err := stateManager.RetrieveResources(ctx, stateContent, resourceType)
	if err != nil {
		slog.Error("Failed to retrieve resources from state", "error", err)
		return fmt.Errorf("failed to retrieve resources: %w", err)
	}

	if len(resources) == 0 {
		slog.Error("No resources found to check for drift.")
		return nil
	}

	wg := &sync.WaitGroup{}
	maxWorker := 5
	channel := make(chan statemanager.StateResource, maxWorker)

	for range maxWorker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for resource := range channel {
				infrastructureResource, err := platformProvider.InfrastructreMetadata(ctx, resourceType, resource)
				if err != nil {
					slog.Error("Failed to retrieve infrastructure metadata", "resource_id", resource.Name, "error", err)
					continue
				}

				// Compare the desired state (from state file) with the actual infrastructure state.
				report, err := driftChecker.CompareStates(ctx, infrastructureResource, resource, attributesToTrack)
				if err != nil {
					slog.Error("Failed to compare states for resource", "resource_id", resource.Name, "error", err)
					continue
				}

				// Write the drift report.
				if err := reporter.WriteReport(ctx, report); err != nil {
					slog.Error("Failed to write report for resource", "resource_id", resource.Name, "error", err)
					continue
				}
			}
		}()
	}

	for _, resource := range resources {
		channel <- resource
	}

	close(channel)

	wg.Wait()

	slog.Info("Drift detection completed.")
	return nil
}
