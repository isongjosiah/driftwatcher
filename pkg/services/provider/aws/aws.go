// Package aws provides an AWS-specific implementation of the infrastructure provider interface.
// It handles communication with AWS services to retrieve live infrastructure data for drift detection.
package aws

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/statemanager"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	aConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
)

// AWSProvider implements the ProviderI interface for AWS infrastructure.
// It encapsulates AWS SDK configuration and provides methods to retrieve
// live infrastructure data from AWS services.
type AWSProvider struct {
	Config aws.Config
}

// NewAWSProvider creates a new AWSProvider instance with the given configuration.
// It initializes the AWS SDK config with credentials, region, and optional LocalStack settings
// for local development and testing.
//
// Parameters:
//   - cfg: AWS configuration containing credential paths, config paths, and profile information
//
// Returns:
//   - provider.ProviderI: A configured AWS provider instance
//   - error: Any error encountered during AWS SDK configuration
func NewAWSProvider(cfg *config.AWSConfig) (provider.ProviderI, error) {
	provider := AWSProvider{}

	localStack := os.Getenv("DRIFT_LOCALSTACK_URL")
	localStackRegion := os.Getenv("DRIFT_LOCALSTACK_REGION")

	awsConfig, err := aConfig.LoadDefaultConfig(context.Background(),
		aConfig.WithSharedCredentialsFiles(cfg.CredentialPath),
		aConfig.WithSharedConfigFiles(cfg.ConfigPath),
		aConfig.WithSharedConfigProfile(cfg.ProfileName),
		aConfig.WithBaseEndpoint(localStack),
		aConfig.WithRegion(localStackRegion))
	if err != nil {
		return nil, err
	}
	provider.Config = awsConfig

	return &provider, nil
}

// InfrastructreMetadata retrieves live infrastructure metadata for a given resource
// from AWS services. It acts as a dispatcher, routing requests to appropriate
// service-specific handlers based on the resource type.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - resourceType: The type of AWS resource (e.g., "aws_instance", "aws_s3_bucket")
//   - resource: The Terraform state resource containing resource configuration
//
// Returns:
//   - provider.InfrastructureResourceI: Live infrastructure data for the resource
//   - error: Any error encountered during metadata retrieval
func (a *AWSProvider) InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (provider.InfrastructureResourceI, error) {
	switch resourceType {
	case "aws_instance":
		resourceId, err := resource.AttributeValue("id")
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse resource identifier from parsed state object")
		}
		if resourceId == "" {
			return nil, fmt.Errorf("resource Id not parsed from state file")
		}

		instance, err := a.HandleEC2Metadata(ctx, resourceId)
		if err != nil {
			return instance, err
		}

		return instance, nil

	default:
		return nil, fmt.Errorf("%s resource not yet supported for AWS provider", resourceType)
	}
}

// HandleEC2Metadata retrieves metadata for a specific EC2 instance from AWS.
// It uses the AWS EC2 API to describe the instance and returns the live infrastructure data.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - resourceId: The AWS instance ID to retrieve metadata for
//
// Returns:
//   - *EC2InfraInstance: The live EC2 instance data wrapped in our internal structure
//   - error: Any error encountered during the AWS API call or data processing
func (a *AWSProvider) HandleEC2Metadata(ctx context.Context, resourceId string) (*EC2InfraInstance, error) {
	ec2Filters := []types.Filter{
		{
			Name:   aws.String("instance-id"),
			Values: []string{resourceId},
		},
	}

	ec2Client := ec2.NewFromConfig(a.Config)
	input := ec2.DescribeInstancesInput{
		Filters: ec2Filters,
	}
	output, err := ec2Client.DescribeInstances(ctx, &input)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to describe ec2 instance")
	}
	if len(output.Reservations) == 0 {
		return nil, fmt.Errorf("%s resource with filters is not running", "EC2")
	}
	// TODO: this should ideally never happen, but find a sensible way to handle this
	if len(output.Reservations) != 1 {
		return nil, fmt.Errorf("%s resource with id %s returns duplicate result", "EC2", "")
	}
	out := &EC2InfraInstance{
		Instance: output.Reservations[0].Instances[0],
	}

	return out, nil
}
