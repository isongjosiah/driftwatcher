package aws

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/provider"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	aConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
)

type AWSProvider struct {
	config aws.Config
}

func (aws *AWSProvider) InfrastructreMetadata(resourceType string, filter map[string]string) ([]provider.InfrastructureResource, error) {
	return nil, nil
}

func (aws *AWSProvider) CompareWithTerraformConfig() error {
	return nil
}

func NewAWSProvider(cfg *config.Config) (*AWSProvider, error) {
	provider := AWSProvider{}

	awsConfig, err := aConfig.LoadDefaultConfig(context.Background(),
		aConfig.WithSharedCredentialsFiles(cfg.Profile.AWSConfig.CredentialPath),
		aConfig.WithSharedConfigFiles(cfg.Profile.AWSConfig.ConfigPath),
		aConfig.WithSharedConfigProfile(cfg.Profile.AWSConfig.ProfileName))
	if err != nil {
		return nil, err
	}
	provider.config = awsConfig

	return &provider, nil
}

func (aws AWSProvider) ResourceMetadata(ctx context.Context, resource string, attributes []string, filters map[string]string) (map[string]string, error) {
	switch resource {
	case "ec2":
		instance, err := aws.handleEC2Metadata(ctx, filters)
		if err != nil {
			return nil, err
		}
		// TODO: parse and return map[string]string
		_ = instance
		return nil, nil
	default:
		return nil, fmt.Errorf("%s resource not yet supported for AWS provider", resource)
	}
}

// TODO: standardize filter structure for EC2
func (aws AWSProvider) handleEC2Metadata(ctx context.Context, filters map[string]string) (types.Instance, error) {
	if len(filters) == 0 {
		return types.Instance{}, fmt.Errorf("At least an instance id must be specified")
	}
	ec2Filters := []types.Filter{}
	for range filters {
	}

	ec2Client := ec2.NewFromConfig(aws.config)
	input := ec2.DescribeInstancesInput{
		Filters: ec2Filters,
	}
	output, err := ec2Client.DescribeInstances(ctx, &input)
	if err != nil {
		return types.Instance{}, errors.Wrap(err, "Failed to describe ec2 instance")
	}
	if len(output.Reservations) == 0 {
		return types.Instance{}, fmt.Errorf("%s resource with id %s is not running", "EC2", "")
	}
	// TODO: this should ideally never happen, but find a sensible way to handle this
	if len(output.Reservations) != 1 {
		return types.Instance{}, fmt.Errorf("%s resource with id %s returns duplicate result", "EC2", "")
	}

	return output.Reservations[0].Instances[0], nil
}
