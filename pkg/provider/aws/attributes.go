package aws

import "strings"

// EC2Attributes defines string constants for various EC2 instance attributes
// that can be tracked for drift detection.
type EC2Attributes string

const (
	// Core Instance Configuration
	EC2AMIID            EC2Attributes = "ami"
	EC2INSTANCETYPE     EC2Attributes = "instance_type"
	EC2INSTANCEID       EC2Attributes = "instance_id"
	EC2KEYNAME          EC2Attributes = "key_name"
	EC2AvailabilityZone EC2Attributes = "availability_zone"
	EC2TENANCY          EC2Attributes = "tenancy"
	EC2MONITORING       EC2Attributes = "monitoring"
	EC2CPUCORECOUNT     EC2Attributes = "cpu_core_count"
	EC2CPUTHREADPERCORE EC2Attributes = "cpu_thread_per_core"
	EC2EbsOptimzied     EC2Attributes = "ebs_optimized"

	// Networking & Security
	EC2SecurityGroupIDs         EC2Attributes = "security_group_ids"
	EC2SUBNETID                 EC2Attributes = "subnet_id"
	EC2AssociatePublicIPAddress EC2Attributes = "associate_public_ip_address"
	EC2PrivateIP                EC2Attributes = "private_ip"
	EC2PrivateDnsName           EC2Attributes = "private_dns_name"
	EC2PublicIP                 EC2Attributes = "public_ip"
	EC2PublicDnsName            EC2Attributes = "public_dns_name"
	EC2SourceDestCheck          EC2Attributes = "source_dest_check"
	EC2IAMInstanceID            EC2Attributes = "iam_instance_id"
	EC2IAMInstanceARN           EC2Attributes = "iam_instance_arn"

	// Storage (EBS Volumes)
	// For these, we typically look at sub-attributes within "root_block_device"
	// or "ebs_block_device". These constants represent the top-level
	// attribute for the block device configurations.
	EC2RootBlockDevice EC2Attributes = "root_block_device"
	EC2EBSBlockDevice  EC2Attributes = "ebs_block_device"
	// Specific sub-attributes for block devices (could be used as keys in a nested map if needed)
	EC2BlockDeviceName     EC2Attributes = "block_device_name"
	EC2VolumeID            EC2Attributes = "volume_id"
	EC2VolumeSize          EC2Attributes = "volume_size"
	EC2VolumeType          EC2Attributes = "volume_type"
	EC2VolumeEncrypted     EC2Attributes = "encrypted"
	EC2DeleteOnTermination EC2Attributes = "delete_on_termination"

	// Metadata & User Data
	EC2MetadataOptions EC2Attributes = "metadata_options"
	EC2UserData        EC2Attributes = "user_data"
	EC2UserDataBase64  EC2Attributes = "user_data_base64"

	// State
	EC2InstanceState EC2Attributes = "instance_state"

	// AWS Security Group Specific Attributes (for completeness, though distinct resource type)
	SGDescription EC2Attributes = "description" // Using EC2Attributes type for consistency for related resources
	SGEgress      EC2Attributes = "egress"
	SGIngress     EC2Attributes = "ingress"
	SGName        EC2Attributes = "name"
	SGVPCID       EC2Attributes = "vpc_id"
)

// IsValidEC2Attribute checks if the given string corresponds to a valid EC2Attributes constant
// using a single switch case with multiple values. It returns true if the string is a valid attribute, false otherwise.
// NOTE: maps might be more performant when this list grows
func IsValidEC2Attribute(attr string) bool {
	switch EC2Attributes(strings.ToLower(attr)) {
	case
		EC2AMIID,
		EC2INSTANCETYPE,
		EC2INSTANCEID,
		EC2KEYNAME,
		EC2AvailabilityZone,
		EC2TENANCY,
		EC2MONITORING,

		EC2SUBNETID,
		EC2AssociatePublicIPAddress,
		EC2PrivateIP,
		EC2PublicIP,
		EC2SourceDestCheck,

		EC2RootBlockDevice,
		EC2EBSBlockDevice,
		EC2VolumeSize,
		EC2VolumeType,
		EC2VolumeEncrypted,
		EC2DeleteOnTermination,

		EC2MetadataOptions,
		EC2UserData,
		EC2UserDataBase64,

		EC2InstanceState,

		SGDescription,
		SGEgress,
		SGIngress,
		SGName,
		SGVPCID:
		return true
	default:
		return false
	}
}
