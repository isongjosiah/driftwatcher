package aws_test

import (
	awsProvider "drift-watcher/pkg/services/provider/aws"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEC2InfraInstance_ResourceType(t *testing.T) {
	e := awsProvider.EC2InfraInstance{}
	assert.Equal(t, "aws_instance", e.ResourceType())
}

func TestEC2InfraInstance_AttributeValue_CoreConfiguration(t *testing.T) {
	instance := types.Instance{
		ImageId:      aws.String("ami-12345"),
		InstanceType: types.InstanceTypeT2Micro,
		InstanceId:   aws.String("i-abcdef123"),
		KeyName:      aws.String("my-key-pair"),
		Placement: &types.Placement{
			AvailabilityZone: aws.String("us-east-1a"),
			Tenancy:          types.TenancyDedicated,
		},
		CpuOptions: &types.CpuOptions{
			CoreCount:      aws.Int32(2),
			ThreadsPerCore: aws.Int32(1),
		},
		EbsOptimized: aws.Bool(true),
	}
	e := awsProvider.EC2InfraInstance{Instance: instance}

	tests := []struct {
		attribute string
		expected  string
		hasError  bool
	}{
		{"ami", "ami-12345", false},
		{"instance_type", "t2.micro", false},
		{"instance_id", "i-abcdef123", false},
		{"key_name", "my-key-pair", false},
		{"availability_zone", "us-east-1a", false},
		{"tenancy", "dedicated", false},
		{"cpu_core_count", "2", false},
		{"cpu_thread_per_core", "1", false},
		{"ebs_optimized", "true", false},
	}

	for _, tt := range tests {
		t.Run(tt.attribute, func(t *testing.T) {
			val, err := e.AttributeValue(tt.attribute)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, val)
			}
		})
	}

	// Test nil pointers for core configuration
	nilInstance := types.Instance{}
	eNil := awsProvider.EC2InfraInstance{Instance: nilInstance}
	val, err := eNil.AttributeValue("ami")
	assert.NoError(t, err) // Should return "0" if CpuOptions or CoreCount is nil
	assert.Empty(t, val)

	val, err = eNil.AttributeValue("availability_zone")
	assert.NoError(t, err) // Should return empty string, not error, if Placement is nil
	assert.Empty(t, val)

	val, err = eNil.AttributeValue("cpu_core_count")
	assert.NoError(t, err) // Should return "0" if CpuOptions or CoreCount is nil
	assert.Equal(t, "0", val)
}

func TestEC2InfraInstance_AttributeValue_NetworkingSecurity(t *testing.T) {
	instance := types.Instance{
		SecurityGroups: []types.GroupIdentifier{
			{GroupId: aws.String("sg-1"), GroupName: aws.String("web")},
			{GroupId: aws.String("sg-2"), GroupName: aws.String("db")},
		},
		SubnetId:         aws.String("subnet-abc"),
		PrivateIpAddress: aws.String("10.0.0.10"),
		PrivateDnsName:   aws.String("ip-10-0-0-10.ec2.internal"),
		PublicIpAddress:  aws.String("54.1.2.3"),
		PublicDnsName:    aws.String("ec2-54-1-2-3.compute-1.amazonaws.com"),
		NetworkInterfaces: []types.InstanceNetworkInterface{
			{
				Association: &types.InstanceNetworkInterfaceAssociation{
					PublicIp: aws.String("54.1.2.3"), // Indicates public IP association
				},
				SourceDestCheck: aws.Bool(false),
			},
		},
	}
	e := awsProvider.EC2InfraInstance{Instance: instance}

	tests := []struct {
		attribute string
		expected  string
		hasError  bool
	}{
		{"security_group_ids", "sg-1,sg-2", false},
		{"subnet_id", "subnet-abc", false},
		{"associate_public_ip_address", "true", false},
		{"private_ip", "10.0.0.10", false},
		{"private_dns_name", "ip-10-0-0-10.ec2.internal", false},
		{"public_ip", "54.1.2.3", false},
		{"public_dns_name", "ec2-54-1-2-3.compute-1.amazonaws.com", false},
		{"source_dest_check", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.attribute, func(t *testing.T) {
			val, err := e.AttributeValue(tt.attribute)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, val)
			}
		})
	}

	// Test nil NetworkInterfaces and Association
	nilNetInstance := types.Instance{}
	eNilNet := awsProvider.EC2InfraInstance{Instance: nilNetInstance}
	val, err := eNilNet.AttributeValue("associate_public_ip_address")
	assert.NoError(t, err)
	assert.Equal(t, "false", val) // Default to false

	val, err = eNilNet.AttributeValue("source_dest_check")
	assert.NoError(t, err)
	assert.Equal(t, "true", val) // Default to true
}

func TestEC2InfraInstance_AttributeValue_Storage(t *testing.T) {
	// Root block device present
	instanceWithRootBD := types.Instance{
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsInstanceBlockDevice{
					VolumeId:            aws.String("vol-root"),
					DeleteOnTermination: aws.Bool(true),
					Status:              types.AttachmentStatusAttached,
				},
			},
		},
	}
	eRoot := awsProvider.EC2InfraInstance{Instance: instanceWithRootBD}
	val, err := eRoot.AttributeValue("root_block_device")
	require.NoError(t, err)
	var ebsInfo types.EbsInstanceBlockDevice
	err = json.Unmarshal([]byte(val), &ebsInfo)
	require.NoError(t, err)
	assert.Equal(t, "vol-root", aws.ToString(ebsInfo.VolumeId))

	// Root block device with different common name
	instanceWithXvdaRootBD := types.Instance{
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &types.EbsInstanceBlockDevice{
					VolumeId: aws.String("vol-xvda"),
				},
			},
		},
	}
	eXvdaRoot := awsProvider.EC2InfraInstance{Instance: instanceWithXvdaRootBD}
	val, err = eXvdaRoot.AttributeValue("root_block_device")
	require.NoError(t, err)
	err = json.Unmarshal([]byte(val), &ebsInfo)
	require.NoError(t, err)
	assert.Equal(t, "vol-xvda", aws.ToString(ebsInfo.VolumeId))

	// No root block device
	instanceNoRootBD := types.Instance{
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sdb"),
			},
		},
	}
	eNoRoot := awsProvider.EC2InfraInstance{Instance: instanceNoRootBD}
	val, err = eNoRoot.AttributeValue("root_block_device")
	require.NoError(t, err)
	assert.Empty(t, val) // Should return empty string if no root device

	// Root block device exists but EBS is nil
	instanceRootBDEbsNil := types.Instance{
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs:        nil,
			},
		},
	}
	eRootEbsNil := awsProvider.EC2InfraInstance{Instance: instanceRootBDEbsNil}
	val, err = eRootEbsNil.AttributeValue("root_block_device")
	require.NoError(t, err)
	assert.Empty(t, val) // Should return empty string if EBS is nil

	// Test JSON marshal error for root_block_device (hard to simulate without custom types)
	// This path is difficult to hit with standard types.
}

func TestEC2InfraInstance_AttributeValue_State(t *testing.T) {
	instance := types.Instance{
		State: &types.InstanceState{
			Name: types.InstanceStateNameRunning,
			Code: aws.Int32(16),
		},
	}
	e := awsProvider.EC2InfraInstance{Instance: instance}

	val, err := e.AttributeValue("instance_state")
	require.NoError(t, err)
	assert.Equal(t, "running", val)

	// Test nil State
	nilStateInstance := types.Instance{}
	eNilState := awsProvider.EC2InfraInstance{Instance: nilStateInstance}
	val, err = eNilState.AttributeValue("instance_state")
	require.NoError(t, err)
	assert.Empty(t, val)
}

func TestEC2InfraInstance_AttributeValue_Tags(t *testing.T) {
	instance := types.Instance{
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("my-web-server")},
			{Key: aws.String("Environment"), Value: aws.String("production")},
		},
	}
	e := awsProvider.EC2InfraInstance{Instance: instance}

	val, err := e.AttributeValue("tags.Name")
	require.NoError(t, err)
	assert.Equal(t, "my-web-server", val)

	val, err = e.AttributeValue("tags.Environment")
	require.NoError(t, err)
	assert.Equal(t, "production", val)

	// Test non-existent tag
	val, err = e.AttributeValue("tags.Project")
	assert.NoError(t, err) // Should now return an error as per the change in the default case
	assert.Empty(t, val)

	val, err = e.AttributeValue("tags.name")
	assert.NoError(t, err)
	assert.Empty(t, val)
}

func TestEC2InfraInstance_AttributeValue_UnsupportedAttribute(t *testing.T) {
	e := awsProvider.EC2InfraInstance{}
	val, err := e.AttributeValue("some_unsupported_attribute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'some_unsupported_attribute' attribute is not supported for EC2 instances or is an invalid attribute name")
	assert.Empty(t, val)

	val, err = e.AttributeValue("SGDescription") // Security Group specific, not on EC2 instance
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'SGDescription' attribute is not supported for EC2 instances or is an invalid attribute name")
	assert.Empty(t, val)
}

func TestEC2InfraInstance_AttributeValue_EmptyInstance(t *testing.T) {
	e := awsProvider.EC2InfraInstance{Instance: types.Instance{}}

	// Test attributes that should return empty string for nil pointers
	val, err := e.AttributeValue("availability_zone")
	assert.NoError(t, err)
	assert.Empty(t, val)

	val, err = e.AttributeValue("tenancy")
	assert.NoError(t, err)
	assert.Empty(t, val)

	val, err = e.AttributeValue("cpu_core_count")
	assert.NoError(t, err)
	assert.Equal(t, "0", val)

	val, err = e.AttributeValue("cpu_thread_per_core")
	assert.NoError(t, err)
	assert.Equal(t, "0", val)

	val, err = e.AttributeValue("security_group_ids")
	assert.NoError(t, err)
	assert.Empty(t, val)

	val, err = e.AttributeValue("root_block_device")
	assert.NoError(t, err)
	assert.Empty(t, val)

	val, err = e.AttributeValue("metadata_options")
	assert.NoError(t, err)
	assert.Empty(t, val)

	val, err = e.AttributeValue("instance_state")
	assert.NoError(t, err)
	assert.Empty(t, val)
}
