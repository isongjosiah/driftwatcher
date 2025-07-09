package aws

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/statemanager"
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

func NewAWSProvider(cfg *config.AWSConfig) (provider.ProviderI, error) {
	provider := AWSProvider{}

	awsConfig, err := aConfig.LoadDefaultConfig(context.Background(),
		aConfig.WithSharedCredentialsFiles(cfg.CredentialPath),
		aConfig.WithSharedConfigFiles(cfg.ConfigPath),
		aConfig.WithSharedConfigProfile(cfg.ProfileName))
	if err != nil {
		return nil, err
	}
	provider.config = awsConfig

	return &provider, nil
}

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

		instance, err := a.handleEC2Metadata(ctx, resourceId)
		if err != nil {
			return instance, err
		}

		return instance, nil

	default:
		return nil, fmt.Errorf("%s resource not yet supported for AWS provider", resourceType)
	}
}

func (a *AWSProvider) handleEC2Metadata(ctx context.Context, resourceId string) (*EC2InfraInstance, error) {
	ec2Filters := []types.Filter{
		{
			Name:   aws.String("instance-id"),
			Values: []string{resourceId},
		},
	}

	ec2Client := ec2.NewFromConfig(a.config)
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
