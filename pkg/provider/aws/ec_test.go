package aws_test

import (
	awsProvider "drift-watcher/pkg/provider/aws"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestGetLiveEC2Attributes(t *testing.T) {
	tests := []struct {
		name     string
		instance *types.Instance
		expected map[string]any
	}{
		{
			name:     "Nil Instance",
			instance: nil,
			expected: nil,
		},
		{
			name: "Full Instance Attributes",
			instance: &types.Instance{
				ImageId:      aws.String("ami-12345"),
				InstanceType: types.InstanceTypeT2Micro,
				KeyName:      aws.String("my-key"),
				Placement: &types.Placement{
					AvailabilityZone: aws.String("us-east-1a"),
					Tenancy:          types.TenancyDefault,
				},
				Monitoring: &types.Monitoring{
					State: types.MonitoringStatePending,
				},
				InstanceId: aws.String("i-abcdef123456"),
				IamInstanceProfile: &types.IamInstanceProfile{
					Id:  aws.String("aid-profile1"),
					Arn: aws.String("arn:aws:iam::123456789012:instance-profile/my-profile"),
				},
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
				CpuOptions: &types.CpuOptions{
					CoreCount:      aws.Int32(1),
					ThreadsPerCore: aws.Int32(2),
				},
				EbsOptimized: aws.Bool(true),
				SecurityGroups: []types.GroupIdentifier{
					{GroupId: aws.String("sg-123"), GroupName: aws.String("web-sg")},
					{GroupId: aws.String("sg-456"), GroupName: aws.String("db-sg")},
				},
				SubnetId:         aws.String("subnet-abc"),
				PublicIpAddress:  aws.String("1.2.3.4"),
				PrivateIpAddress: aws.String("10.0.0.10"),
				SourceDestCheck:  aws.Bool(true),
				PrivateDnsName:   aws.String("ip-10-0-0-10.ec2.internal"),
				PublicDnsName:    aws.String("ec2-1-2-3-4.compute-1.amazonaws.com"),
				RootDeviceName:   aws.String("/dev/sda1"),
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:            aws.String("vol-root"),
							DeleteOnTermination: aws.Bool(true),
						},
					},
					{
						DeviceName: aws.String("/dev/sdb"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:            aws.String("vol-ebs1"),
							DeleteOnTermination: aws.Bool(false),
						},
					},
				},
			},
			expected: map[string]any{
				string(awsProvider.EC2AMIID):                    "ami-12345",
				string(awsProvider.EC2INSTANCETYPE):             "t2.micro",
				string(awsProvider.EC2KEYNAME):                  "my-key",
				string(awsProvider.EC2AvailabilityZone):         "us-east-1a",
				string(awsProvider.EC2TENANCY):                  types.TenancyDefault,
				string(awsProvider.EC2MONITORING):               types.MonitoringStatePending,
				string(awsProvider.EC2INSTANCEID):               "i-abcdef123456",
				string(awsProvider.EC2IAMInstanceID):            "aid-profile1",
				string(awsProvider.EC2IAMInstanceARN):           "arn:aws:iam::123456789012:instance-profile/my-profile",
				string(awsProvider.EC2InstanceState):            types.InstanceStateNameRunning,
				string(awsProvider.EC2CPUCORECOUNT):             float64(1),
				string(awsProvider.EC2CPUTHREADPERCORE):         float64(2),
				string(awsProvider.EC2EbsOptimzied):             true,
				string(awsProvider.EC2SecurityGroupIDs):         []string{"sg-123", "sg-456"},
				string(awsProvider.EC2SUBNETID):                 "subnet-abc",
				string(awsProvider.EC2AssociatePublicIPAddress): true,
				string(awsProvider.EC2PrivateIP):                "10.0.0.10",
				string(awsProvider.EC2PublicIP):                 "1.2.3.4",
				string(awsProvider.EC2SourceDestCheck):          true,
				string(awsProvider.EC2PrivateDnsName):           "ip-10-0-0-10.ec2.internal",
				string(awsProvider.EC2PublicDnsName):            "ec2-1-2-3-4.compute-1.amazonaws.com",
				string(awsProvider.EC2RootBlockDevice): []map[string]any{
					{string(awsProvider.EC2BlockDeviceName): "/dev/sda1", string(awsProvider.EC2VolumeID): "vol-root", string(awsProvider.EC2DeleteOnTermination): true},
				},
				string(awsProvider.EC2EBSBlockDevice): []map[string]any{
					{string(awsProvider.EC2BlockDeviceName): "/dev/sdb", string(awsProvider.EC2VolumeID): "vol-ebs1", string(awsProvider.EC2DeleteOnTermination): false},
				},
			},
		},
		{
			name: "Minimal Instance Attributes",
			instance: &types.Instance{
				InstanceType: types.InstanceTypeT2Nano,
				InstanceId:   aws.String("i-minimal"),
				State: &types.InstanceState{
					Name: types.InstanceStateNameStopped,
				},
				SecurityGroups: []types.GroupIdentifier{}, // Empty security groups
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-minimal-root"),
						},
					},
				},
				RootDeviceName: aws.String("/dev/xvda"),
			},
			expected: map[string]any{
				string(awsProvider.EC2INSTANCETYPE):             "t2.nano",
				string(awsProvider.EC2INSTANCEID):               "i-minimal",
				string(awsProvider.EC2InstanceState):            types.InstanceStateNameStopped,
				string(awsProvider.EC2EbsOptimzied):             false, // Default if not explicitly true
				string(awsProvider.EC2SecurityGroupIDs):         []string{},
				string(awsProvider.EC2AssociatePublicIPAddress): false,
				string(awsProvider.EC2RootBlockDevice): []map[string]any{
					{string(awsProvider.EC2BlockDeviceName): "/dev/xvda", string(awsProvider.EC2VolumeID): "vol-minimal-root", string(awsProvider.EC2DeleteOnTermination): false}, // Default false if not set
				},
				string(awsProvider.EC2EBSBlockDevice): []map[string]any{},
			},
		},
		{
			name: "Instance with nil pointers for optional fields",
			instance: &types.Instance{
				ImageId:            nil,
				InstanceType:       types.InstanceTypeT3Medium,
				KeyName:            nil,
				Placement:          nil, // Entire placement nil
				Monitoring:         nil,
				InstanceId:         aws.String("i-nilpointers"),
				IamInstanceProfile: nil,
				State: &types.InstanceState{
					Name: types.InstanceStateNameTerminated,
				},
				CpuOptions:          nil,
				EbsOptimized:        nil,
				SecurityGroups:      []types.GroupIdentifier{},
				SubnetId:            nil,
				PublicIpAddress:     nil,
				PrivateIpAddress:    nil,
				SourceDestCheck:     nil,
				PrivateDnsName:      nil,
				PublicDnsName:       nil,
				RootDeviceName:      aws.String("/dev/sda1"),              // Root device name is present
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{}, // No block device mappings
			},
			expected: map[string]any{
				string(awsProvider.EC2INSTANCETYPE):             "t3.medium",
				string(awsProvider.EC2INSTANCEID):               "i-nilpointers",
				string(awsProvider.EC2InstanceState):            types.InstanceStateNameTerminated,
				string(awsProvider.EC2EbsOptimzied):             false,
				string(awsProvider.EC2SecurityGroupIDs):         []string{},
				string(awsProvider.EC2AssociatePublicIPAddress): false,
				string(awsProvider.EC2RootBlockDevice):          []map[string]any{}, // Empty slice if no mappings
				string(awsProvider.EC2EBSBlockDevice):           []map[string]any{},
			},
		},
		{
			name: "Instance with only RootDeviceName but no matching BDM",
			instance: &types.Instance{
				InstanceType:   types.InstanceTypeT2Micro,
				InstanceId:     aws.String("i-root-no-bdm"),
				RootDeviceName: aws.String("/dev/sda1"),
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sdb"), // Mismatching device name
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-ebs1"),
						},
					},
				},
			},
			expected: map[string]any{
				string(awsProvider.EC2INSTANCETYPE):             "t2.micro",
				string(awsProvider.EC2INSTANCEID):               "i-root-no-bdm",
				string(awsProvider.EC2EbsOptimzied):             false,
				string(awsProvider.EC2SecurityGroupIDs):         []string{},
				string(awsProvider.EC2AssociatePublicIPAddress): false,
				string(awsProvider.EC2RootBlockDevice):          []map[string]any{}, // Should be empty if no matching root device BDM
				string(awsProvider.EC2EBSBlockDevice): []map[string]any{ // This one should still be included
					{string(awsProvider.EC2BlockDeviceName): "/dev/sdb", string(awsProvider.EC2VolumeID): "vol-ebs1", string(awsProvider.EC2DeleteOnTermination): false},
				},
			},
		},
	}

	// Assuming GetLiveEC2Attributes is in the `aws` package
	// Since it's in the same package as this test file (aws_test), it can be called directly.
	// If it were in `drift-watcher/pkg/aws`, you would call `aws.GetLiveEC2Attributes`.
	// For this test, I'll use a local copy of the function to ensure it runs within this test file.
	// In a real scenario, you would import and use `aws.GetLiveEC2Attributes`.

	// Helper function to replicate GetLiveEC2Attributes for testing
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := awsProvider.GetLiveEC2Attributes(tt.instance) // Call the local helper function
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetLiveEC2Attributes() got = %+v, want %+v", got, tt.expected)
			}
		})
	}
}
