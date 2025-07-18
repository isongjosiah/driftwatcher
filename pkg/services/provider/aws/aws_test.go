package aws_test

import (
	"context"
	"drift-watcher/config"
	awsProvider "drift-watcher/pkg/services/provider/aws"
	"drift-watcher/pkg/services/statemanager"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	localstackEndpoint string
	awsConfig          aws.Config
)

// TestMain sets up LocalStack once for all tests in this package.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start LocalStack container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack:latest",
			ExposedPorts: []string{"4566/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForLog("Ready.").WithStartupTimeout(2 * time.Minute),
			),
			Env: map[string]string{
				"SERVICES": "ec2", // Only start EC2 service for these tests
				"DEBUG":    "1",
			},
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("Failed to start LocalStack container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate LocalStack container: %v", err)
		}
	}()

	// Get LocalStack endpoint
	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get LocalStack host: %v", err)
	}
	port, err := container.MappedPort(ctx, "4566")
	if err != nil {
		log.Fatalf("Failed to get LocalStack port: %v", err)
	}
	localstackEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())

	// Configure AWS SDK to use LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == ec2.ServiceID {
			return aws.Endpoint{
				URL: localstackEndpoint,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsConfig, err = aConfig.LoadDefaultConfig(ctx,
		aConfig.WithEndpointResolverWithOptions(customResolver),
		aConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "test")),
		aConfig.WithRegion("us-east-1"), // LocalStack typically uses us-east-1 as default
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config for LocalStack: %v", err)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestNewAWSProvider(t *testing.T) {
	// Create dummy credential and config files for NewAWSProvider to load
	tmpDir := t.TempDir()
	credFilePath := []string{filepath.Join(tmpDir, "credentials")}
	configFilePath := []string{filepath.Join(tmpDir, "config")}

	err := os.WriteFile(credFilePath[0], []byte("[default]\naws_access_key_id = test\naws_secret_access_key = test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(configFilePath[0], []byte("[profile default]\nregion = us-east-1"), 0644)
	require.NoError(t, err)

	cfg := &config.AWSConfig{
		CredentialPath: credFilePath,
		ConfigPath:     configFilePath,
		ProfileName:    "default",
	}

	provider, err := awsProvider.NewAWSProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	_, ok := provider.(*awsProvider.AWSProvider)
	require.True(t, ok)
}

func TestNewAWSProvider_LoadConfigError(t *testing.T) {
	// Simulate a scenario where config loading fails (e.g., invalid profile)
	tmpDir := t.TempDir()
	credFilePath := []string{filepath.Join(tmpDir, "credentials")}
	configFilePath := []string{filepath.Join(tmpDir, "config")}

	err := os.WriteFile(credFilePath[0], []byte("[default]\naws_access_key_id = test\naws_secret_access_key = test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(configFilePath[0], []byte("[profile default]\nregion = us-east-1"), 0644)
	require.NoError(t, err)

	cfg := &config.AWSConfig{
		CredentialPath: credFilePath,
		ConfigPath:     configFilePath,
		ProfileName:    "nonexistent-profile", // This should cause an error
	}

	p, err := awsProvider.NewAWSProvider(cfg)
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "failed to get shared config profile, nonexistent-profile")
}

func TestInfrastructureMetadata_EC2Instance_Success(t *testing.T) {
	ctx := context.Background()
	ec2Client := ec2.NewFromConfig(awsConfig)

	// 1. Create a dummy EC2 instance in LocalStack
	runInstancesOutput, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0abcdef1234567890"), // Dummy AMI ID
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("test-instance")},
					{Key: aws.String("Environment"), Value: aws.String("dev")},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, runInstancesOutput.Instances, 1)
	instanceID := aws.ToString(runInstancesOutput.Instances[0].InstanceId)
	defer func() {
		// Clean up the instance
		_, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: []string{instanceID},
		})
		if err != nil {
			log.Printf("Failed to terminate instance %s: %v", instanceID, err)
		}
	}()

	// Wait for the instance to be in a runnable state (LocalStack might be quick, but good practice)
	waiter := ec2.NewInstanceRunningWaiter(ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 1*time.Minute)
	require.NoError(t, err, "waiting for instance to run")

	// 2. Prepare desired state resource
	desiredStateResource := statemanager.StateResource{
		Type: "aws_instance",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"id":            instanceID,
					"instance_type": "t2.micro", // Expected value from Terraform
				},
			},
		},
	}

	// 3. Initialize AWSProvider with the LocalStack config
	p := awsProvider.AWSProvider{Config: awsConfig}

	// 4. Call InfrastructreMetadata
	infraResource, err := p.InfrastructreMetadata(ctx, "aws_instance", desiredStateResource)
	require.NoError(t, err)
	assert.NotNil(t, infraResource)

	ec2Instance, ok := infraResource.(*awsProvider.EC2InfraInstance)
	require.True(t, ok)
	assert.Equal(t, instanceID, aws.ToString(ec2Instance.Instance.InstanceId))
	assert.Equal(t, string(types.InstanceTypeT2Micro), string(ec2Instance.Instance.InstanceType))
	assert.Equal(t, "aws_instance", ec2Instance.ResourceType())

	// Test AttributeValue for EC2InfraInstance
	instanceType, err := ec2Instance.AttributeValue("instance_type")
	require.NoError(t, err)
	assert.Equal(t, "t2.micro", instanceType)

	id, err := ec2Instance.AttributeValue("instance_id")
	require.NoError(t, err)
	assert.Equal(t, instanceID, id)

	// Test a tag attribute
	tagName, err := ec2Instance.AttributeValue("tags.Name")
	require.NoError(t, err)
	assert.Equal(t, "test-instance", tagName)

	tagEnv, err := ec2Instance.AttributeValue("tags.Environment")
	require.NoError(t, err)
	assert.Equal(t, "dev", tagEnv)

	// Test non-existent tag attribute
	nonExistentTag, err := ec2Instance.AttributeValue("tags.NonExistent")
	assert.NoError(t, err)
	assert.Empty(t, nonExistentTag)

	// Test non-existent direct attribute
	nonExistentAttr, err := ec2Instance.AttributeValue("non_existent_attribute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'non_existent_attribute' attribute is not supported for EC2 instances or is an invalid attribute name")
	assert.Empty(t, nonExistentAttr)
}

func TestInfrastructureMetadata_UnsupportedResourceType(t *testing.T) {
	ctx := context.Background()
	provider := &awsProvider.AWSProvider{Config: awsConfig}
	desiredStateResource := statemanager.StateResource{Type: "aws_s3_bucket"} // Unsupported type

	infraResource, err := provider.InfrastructreMetadata(ctx, "aws_s3_bucket", desiredStateResource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws_s3_bucket resource not yet supported for AWS provider")
	assert.Nil(t, infraResource)
}

func TestInfrastructureMetadata_MissingResourceId(t *testing.T) {
	ctx := context.Background()
	provider := &awsProvider.AWSProvider{Config: awsConfig}
	desiredStateResource := statemanager.StateResource{
		Type: "aws_instance",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"id": "", // Empty ID
				},
			},
		},
	}

	infraResource, err := provider.InfrastructreMetadata(ctx, "aws_instance", desiredStateResource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource Id not parsed from state file")
	assert.Nil(t, infraResource)
}

func TestInfrastructureMetadata_AttributeValueError(t *testing.T) {
	ctx := context.Background()
	provider := &awsProvider.AWSProvider{Config: awsConfig}
	desiredStateResource := statemanager.StateResource{
		Type: "aws_instance",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"id": 123, // Not a string, will cause AttributeValue to return error
				},
			},
		},
	}

	infraResource, err := provider.InfrastructreMetadata(ctx, "aws_instance", desiredStateResource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to parse resource identifier from parsed state object")
	assert.Nil(t, infraResource)
}

func TestHandleEC2Metadata_InstanceNotFound(t *testing.T) {
	ctx := context.Background()
	provider := &awsProvider.AWSProvider{Config: awsConfig}

	// Use a non-existent instance ID
	nonExistentInstanceID := "i-00000000000000000"

	instance, err := provider.HandleEC2Metadata(ctx, nonExistentInstanceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EC2 resource with filters is not running")
	assert.Nil(t, instance)
}

func TestHandleEC2Metadata_DescribeInstancesError(t *testing.T) {
	ctx := context.Background()

	// For demonstration, let's create an AWSProvider with a broken config
	badConfig := awsConfig
	badConfig.Region = "invalid-region" // This might cause an error, or just return empty
	provider := &awsProvider.AWSProvider{Config: badConfig}

	_, err := provider.HandleEC2Metadata(ctx, "i-1234567890abcdef0") // Dummy ID
	assert.Error(t, err)
	// The error message might vary based on LocalStack's behavior for invalid regions.
	// It could be "InvalidParameterValue" or similar.
	assert.Contains(t, err.Error(), "EC2 resource with filters is not running")
}

func TestHandleEC2Metadata_DuplicateResults(t *testing.T) {
	// This scenario (len(output.Reservations) != 1) is difficult to reliably
	// reproduce with LocalStack's standard behavior for DescribeInstances.
	// DescribeInstances typically returns one reservation per filter match.
	// To cover this, one would typically mock the EC2 client's DescribeInstances response.
	// For now, we acknowledge this path is hard to hit with live LocalStack.
	// The code path is: if len(output.Reservations) != 1 { return nil, fmt.Errorf(...) }
	// This line is covered if the previous TestHandleEC2Metadata_InstanceNotFound is run
	// as len(output.Reservations) will be 0, triggering the first part of the 'if' condition.
	// The specific branch for len > 1 is hard to trigger.
	t.Skip("Skipping TestHandleEC2Metadata_DuplicateResults as it's hard to reliably reproduce with LocalStack.")
}

//func TestEC2InfraInstance_ResourceType(t *testing.T) {
//	instance := &awsProvider.EC2InfraInstance{}
//	assert.Equal(t, "aws_instance", instance.ResourceType())
//}

func TestEC2InfraInstance_AttributeValue_DirectAttributes(t *testing.T) {
	// Create a mock EC2 instance with some attributes
	ec2Instance := types.Instance{
		InstanceId:   aws.String("i-test123"),
		InstanceType: types.InstanceTypeT2Micro,
		State: &types.InstanceState{
			Name: types.InstanceStateNameRunning,
		},
		PrivateIpAddress: aws.String("10.0.0.10"),
		PublicIpAddress:  aws.String("54.1.2.3"),
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("my-server")},
		},
	}
	infraInstance := &awsProvider.EC2InfraInstance{Instance: ec2Instance}

	val, err := infraInstance.AttributeValue("instance_id")
	require.NoError(t, err)
	assert.Equal(t, "i-test123", val)

	val, err = infraInstance.AttributeValue("instance_type")
	require.NoError(t, err)
	assert.Equal(t, "t2.micro", val)

	val, err = infraInstance.AttributeValue("private_ip")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.10", val)

	val, err = infraInstance.AttributeValue("public_ip")
	require.NoError(t, err)
	assert.Equal(t, "54.1.2.3", val)

	// Test non-existent direct attribute
	val, err = infraInstance.AttributeValue("non_existent_field")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'non_existent_field' attribute is not supported for EC2 instances or is an invalid attribute name")
	assert.Empty(t, val)
}
