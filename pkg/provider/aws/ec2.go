package aws

import (
	"context"
	"drift-watcher/pkg/provider"
	"drift-watcher/pkg/terraform"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
)

// GetLiveEC2Attributes extracts relevant attributes from an *ec2.Instance
// into a map[string]any, structured similarly to Terraform state attributes,
// using the defined EC2Attributes constants as keys.
func GetLiveEC2Attributes(instance *types.Instance) map[string]any {
	if instance == nil {
		return nil
	}

	attrs := make(map[string]any)

	// --- Core Instance Configuration ---
	if instance.ImageId != nil {
		attrs[string(EC2AMIID)] = *instance.ImageId
	}
	attrs[string(EC2INSTANCETYPE)] = string(instance.InstanceType)

	if instance.KeyName != nil {
		attrs[string(EC2KEYNAME)] = *instance.KeyName
	}

	if instance.Placement != nil {
		if instance.Placement.AvailabilityZone != nil {
			attrs[string(EC2AvailabilityZone)] = *instance.Placement.AvailabilityZone
		}

		attrs[string(EC2TENANCY)] = instance.Placement.Tenancy
	}
	if instance.Monitoring != nil {
		attrs[string(EC2MONITORING)] = instance.Monitoring.State
	}

	if instance.InstanceId != nil {
		attrs[string(EC2INSTANCEID)] = *instance.InstanceId
	}

	if instance.IamInstanceProfile != nil {
		attrs[string(EC2IAMInstanceID)] = *instance.IamInstanceProfile.Id
		attrs[string(EC2IAMInstanceARN)] = *instance.IamInstanceProfile.Arn
	}

	if instance.State != nil {
		attrs[string(EC2InstanceState)] = instance.State.Name
	}

	if instance.CpuOptions != nil {
		if instance.CpuOptions.CoreCount != nil {
			attrs[string(EC2CPUCORECOUNT)] = float64(*instance.CpuOptions.CoreCount)
		}
		if instance.CpuOptions.ThreadsPerCore != nil {
			attrs[string(EC2CPUTHREADPERCORE)] = float64(*instance.CpuOptions.ThreadsPerCore)
		}
	}

	attrs[string(EC2EbsOptimzied)] = false
	if instance.EbsOptimized != nil {
		attrs[string(EC2EbsOptimzied)] = *instance.EbsOptimized
	}

	// --- Networking & Security ---
	sgIDs := []string{}
	for _, sg := range instance.SecurityGroups {
		if sg.GroupId != nil {
			sgIDs = append(sgIDs, *sg.GroupId)
		}
	}
	attrs[string(EC2SecurityGroupIDs)] = sgIDs
	if instance.SubnetId != nil {
		attrs[string(EC2SUBNETID)] = *instance.SubnetId
	}
	attrs[string(EC2AssociatePublicIPAddress)] = (instance.PublicIpAddress != nil)
	if instance.PrivateIpAddress != nil {
		attrs[string(EC2PrivateIP)] = *instance.PrivateIpAddress
	}
	if instance.PublicIpAddress != nil {
		attrs[string(EC2PublicIP)] = *instance.PublicIpAddress
	}
	if instance.SourceDestCheck != nil {
		attrs[string(EC2SourceDestCheck)] = *instance.SourceDestCheck
	}

	if instance.PrivateDnsName != nil {
		attrs[string(EC2PrivateDnsName)] = *instance.PrivateDnsName
	}
	if instance.PublicDnsName != nil {
		attrs[string(EC2PublicDnsName)] = *instance.PublicDnsName
	}

	// --- Storage (EBS Volumes) ---
	rootBlockDeviceFound := false
	var rootDeviceMap []map[string]any
	ebsBlockDevices := []map[string]any{}

	for _, bdm := range instance.BlockDeviceMappings {

		blockDeviceAttrs := make(map[string]any)
		if bdm.DeviceName != nil {
			blockDeviceAttrs[string(EC2BlockDeviceName)] = *bdm.DeviceName
		}
		if bdm.Ebs.VolumeId != nil {
			blockDeviceAttrs[string(EC2VolumeID)] = *bdm.Ebs.VolumeId
		}
		if bdm.Ebs.DeleteOnTermination != nil {
			blockDeviceAttrs[string(EC2DeleteOnTermination)] = *bdm.Ebs.DeleteOnTermination
		} else {
			blockDeviceAttrs[string(EC2DeleteOnTermination)] = false
		}

		if bdm.DeviceName != nil && (*bdm.DeviceName == *instance.RootDeviceName) { // Use RootDeviceName for reliability
			rootDeviceMap = []map[string]any{blockDeviceAttrs}
			rootBlockDeviceFound = true
		} else {
			ebsBlockDevices = append(ebsBlockDevices, blockDeviceAttrs)
		}
	}

	attrs[string(EC2RootBlockDevice)] = rootDeviceMap
	attrs[string(EC2EBSBlockDevice)] = ebsBlockDevices
	if !rootBlockDeviceFound {
		attrs[string(EC2RootBlockDevice)] = []map[string]any{} // Ensure it's an empty slice if no root device found
	}

	return attrs
}

func (a AWSProvider) handleEC2Metadata(ctx context.Context, filters map[string]string) (types.Instance, error) {
	if len(filters) == 0 {
		return types.Instance{}, fmt.Errorf("At least an instance id must be specified")
	}
	ec2Filters := []types.Filter{}
	for key, value := range filters {
		ec2Filters = append(ec2Filters, types.Filter{
			Name:   aws.String(key),
			Values: []string{value},
		})
	}

	ec2Client := ec2.NewFromConfig(a.config)
	input := ec2.DescribeInstancesInput{
		Filters: ec2Filters,
	}
	output, err := ec2Client.DescribeInstances(ctx, &input)
	if err != nil {
		return types.Instance{}, errors.Wrap(err, "Failed to describe ec2 instance")
	}
	if len(output.Reservations) == 0 {
		return types.Instance{}, fmt.Errorf("%s resource with filters is not running", "EC2")
	}
	// TODO: this should ideally never happen, but find a sensible way to handle this
	if len(output.Reservations) != 1 {
		return types.Instance{}, fmt.Errorf("%s resource with id %s returns duplicate result", "EC2", "")
	}

	return output.Reservations[0].Instances[0], nil
}

func compareEC2DesiredAndActiveState(ctx context.Context, resourceType string, liveState *provider.InfrastructureResource, desiredState terraform.Resource, attributesToTrack []string) (provider.DriftReport, error) {
	if liveState == nil { // INFRASTRUCTURE_MISSING_IN_LIVE
		report := provider.DriftReport{
			ResourceId:   desiredState.Instances[0].Attributes.ID,
			ResourceType: resourceType,
			ResourceName: desiredState.Name,
			HasDrift:     true,
			DriftDetails: []provider.DriftItem{
				{
					Field:          "existence",
					TerraformValue: "exists",
					ActualValue:    "missing",
					DriftType:      provider.AttributeMissingInInfrastructure,
				},
			},
			GeneratedAt: time.Now(),
			Status:      provider.ResourceMissingInInfrastructure,
		}
		return report, nil
	}

	desiredStateMap := generateDesiredStateMapper(desiredState.Instances[0])

	driftReportStatus := provider.Match
	var driftItems []provider.DriftItem
	for _, attribute := range attributesToTrack {
		if !IsValidEC2Attribute(attribute) {
			slog.Warn(fmt.Sprintf("%s attribute is currently not supported for the %s resource", attribute, resourceType))
			continue
		}
		liveVal := liveState.Attributes[attribute]
		desiredVal := desiredStateMap[attribute]

		driftItem := provider.DriftItem{
			Field:          attribute,
			TerraformValue: desiredVal,
			ActualValue:    liveVal,
			DriftType:      provider.Match,
		}
		switch {
		case liveVal == nil && desiredVal != "":
			driftItem.DriftType = provider.AttributeMissingInInfrastructure
			driftReportStatus = provider.Drift
		case desiredVal == "" && liveVal != nil:
			driftItem.DriftType = provider.AttributeMissingInTerraform
			driftReportStatus = provider.Drift
		case desiredVal != liveVal:
			driftItem.DriftType = provider.AttributeValueChanged
			driftReportStatus = provider.Drift
		}
		driftItems = append(driftItems, driftItem)
	}
	report := provider.DriftReport{
		ResourceId:   desiredState.Instances[0].Attributes.ID,
		ResourceType: resourceType,
		ResourceName: desiredState.Name,
		HasDrift:     driftReportStatus != provider.Match,
		DriftDetails: driftItems,
		GeneratedAt:  time.Now(),
		Status:       driftReportStatus,
	}
	return report, nil
}
