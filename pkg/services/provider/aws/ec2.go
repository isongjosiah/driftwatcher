package aws

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2InfraInstance struct {
	Instance types.Instance
}

func (ec2 EC2InfraInstance) ResourceType() string {
	return "aws_instance"
}

// (EC2InfraInstance struct and GetID, GetName, ResourceType methods as previously defined)

// AttributeValue retrieves the string value of a specified EC2 instance attribute.
// It maps the attribute names (defined by EC2Attributes constants) to the corresponding
// fields within the AWS SDK's types.Instance struct.
//
// For complex attributes like block devices or metadata options, it marshals them to JSON string.
// For booleans and integers, it converts them to their string representations.
// For attributes that are slices (e.g., SecurityGroupIDs), it joins them into a comma-separated string.
//
// If an attribute is missing in the live data (e.g., a tag doesn't exist), it returns an empty string
// and nil error, allowing the drift checker to correctly identify it as "missing in infrastructure".
func (e *EC2InfraInstance) AttributeValue(attribute string) (string, error) {
	switch EC2Attributes(attribute) { // Cast attribute string to EC2Attributes type
	// Core Instance Configuration
	case EC2AMIID:
		return aws.ToString(e.Instance.ImageId), nil
	case EC2INSTANCETYPE:
		return string(e.Instance.InstanceType), nil
	case EC2INSTANCEID:
		return aws.ToString(e.Instance.InstanceId), nil
	case EC2KEYNAME:
		return aws.ToString(e.Instance.KeyName), nil
	case EC2AvailabilityZone:
		if e.Instance.Placement != nil {
			return aws.ToString(e.Instance.Placement.AvailabilityZone), nil
		}
		return "", nil
	case EC2TENANCY:
		if e.Instance.Placement != nil {
			return string(e.Instance.Placement.Tenancy), nil
		}
		return "", nil
	case EC2CPUCORECOUNT:
		if e.Instance.CpuOptions != nil && e.Instance.CpuOptions.CoreCount != nil {
			return strconv.Itoa(int(*e.Instance.CpuOptions.CoreCount)), nil
		}
		return "0", nil // Default value if CPU options are missing
	case EC2CPUTHREADPERCORE:
		if e.Instance.CpuOptions != nil && e.Instance.CpuOptions.ThreadsPerCore != nil {
			return strconv.Itoa(int(*e.Instance.CpuOptions.ThreadsPerCore)), nil
		}
		return "0", nil // Default value if CPU options are missing
	case EC2EbsOptimzied:
		// Convert pointer to bool, then to string
		return strconv.FormatBool(aws.ToBool(e.Instance.EbsOptimized)), nil

	// Networking & Security
	case EC2SecurityGroupIDs:
		var ids []string
		for _, sg := range e.Instance.SecurityGroups {
			ids = append(ids, aws.ToString(sg.GroupId))
		}
		// Join all security group IDs into a comma-separated string
		return strings.Join(ids, ","), nil
	case EC2SUBNETID:
		return aws.ToString(e.Instance.SubnetId), nil
	case EC2AssociatePublicIPAddress:
		// This attribute is typically on the primary network interface.
		// Assume the first network interface if available.
		if len(e.Instance.NetworkInterfaces) > 0 && e.Instance.NetworkInterfaces[0].Association != nil {
			return strconv.FormatBool(e.Instance.NetworkInterfaces[0].Association.PublicIp != nil), nil
		}
		return strconv.FormatBool(false), nil // Default to false if no association found
	case EC2PrivateIP:
		return aws.ToString(e.Instance.PrivateIpAddress), nil
	case EC2PrivateDnsName:
		return aws.ToString(e.Instance.PrivateDnsName), nil
	case EC2PublicIP:
		return aws.ToString(e.Instance.PublicIpAddress), nil
	case EC2PublicDnsName:
		return aws.ToString(e.Instance.PublicDnsName), nil
	case EC2SourceDestCheck:
		// This attribute is on the primary network interface.
		if len(e.Instance.NetworkInterfaces) > 0 && e.Instance.NetworkInterfaces[0].SourceDestCheck != nil {
			return strconv.FormatBool(aws.ToBool(e.Instance.NetworkInterfaces[0].SourceDestCheck)), nil
		}
		return strconv.FormatBool(true), nil // Default to true if not specified (AWS default)
	// Storage (EBS Volumes) - These are complex, so we'll JSON marshal them
	case EC2RootBlockDevice:
		for _, bdm := range e.Instance.BlockDeviceMappings {
			if aws.ToString(bdm.DeviceName) == "/dev/sda1" || aws.ToString(bdm.DeviceName) == "/dev/xvda" { // Common root device names
				if bdm.Ebs != nil {
					bytes, err := json.Marshal(bdm.Ebs)
					if err != nil {
						return "", fmt.Errorf("failed to marshal root_block_device: %w", err)
					}
					return string(bytes), nil
				}
				return "", nil
			}
		}
		return "", nil // No root block device found or EBS info missing

	// Metadata & User Data
	case EC2MetadataOptions:
		if e.Instance.MetadataOptions != nil {
			bytes, err := json.Marshal(e.Instance.MetadataOptions)
			if err != nil {
				return "", fmt.Errorf("failed to marshal metadata_options: %w", err)
			}
			return string(bytes), nil
		}
		return "", nil

	// State
	case EC2InstanceState:
		if e.Instance.State != nil {
			return string(e.Instance.State.Name), nil
		}
		return "", nil

	default:
		// Handle tags in the format "tags.KEY"
		if strings.HasPrefix(attribute, "tags.") {
			tagName := strings.TrimPrefix(attribute, "tags.")
			for _, tag := range e.Instance.Tags {
				if aws.ToString(tag.Key) == tagName {
					return aws.ToString(tag.Value), nil
				}
			}
			// If a tag is requested but not present, return empty string and nil error.
			// This indicates absence, allowing the drift checker to mark it as missing.
			return "", nil
		}

		// For security group specific attributes (SGDescription, SGEgress, etc.)
		// that are not part of the EC2 instance itself, return an unsupported error.
		// If these attributes were meant to be retrieved from a Security Group resource,
		// you'd need a separate SecurityGroupInfraResource struct and provider method.
		return "", fmt.Errorf("'%s' attribute is not supported for EC2 instances or is an invalid attribute name", attribute)
	}
}
