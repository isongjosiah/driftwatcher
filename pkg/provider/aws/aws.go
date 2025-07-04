package aws

import (
	"context"
	"drift-watcher/config"
	"drift-watcher/pkg/provider"
	"drift-watcher/pkg/terraform"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aConfig "github.com/aws/aws-sdk-go-v2/config"
)

type AWSProvider struct {
	config aws.Config
}

func NewAWSProvider(cfg *config.Config) (provider.ProviderI, error) {
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

func (a *AWSProvider) InfrastructreMetadata(ctx context.Context, resourceType string, filter map[string]string) (*provider.InfrastructureResource, error) {
	var resourceDetail *provider.InfrastructureResource

	switch resourceType {
	case "ec2":
		fmt.Println("filter inside handler", filter)
		instance, err := a.handleEC2Metadata(ctx, filter)
		if err != nil {
			return resourceDetail, err
		}

		tags := map[string]string{}
		for _, tag := range instance.Tags {
			tags[*tag.Key] = *tag.Value
		}

		attributes := GetLiveEC2Attributes(&instance)
		resourceDetail = &provider.InfrastructureResource{
			ID:         *instance.InstanceId,
			Type:       "ec2",
			Tags:       tags,
			Attributes: attributes,
		}
		return resourceDetail, nil

	default:
		return resourceDetail, fmt.Errorf("%s resource not yet supported for AWS provider", resourceType)
	}
}

func (a *AWSProvider) CompareActiveAndDesiredState(ctx context.Context, resourceType string, liveState *provider.InfrastructureResource, desiredState terraform.Resource, attributesToTrack []string) (provider.DriftReport, error) {
	var report provider.DriftReport
	switch resourceType {
	case "ec2":
		if liveState == nil { // INFRASTRUCTURE_MISSING_IN_LIVE
			report = provider.DriftReport{
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
		report = provider.DriftReport{
			ResourceId:   desiredState.Instances[0].Attributes.ID,
			ResourceType: resourceType,
			ResourceName: desiredState.Name,
			HasDrift:     driftReportStatus != provider.Match,
			DriftDetails: driftItems,
			GeneratedAt:  time.Now(),
			Status:       driftReportStatus,
		}
	}
	return report, fmt.Errorf("infrastructure type not supported")
}

func generateDesiredStateMapper(desiredInstanceState terraform.Instance) map[string]string {
	attr := desiredInstanceState.Attributes
	result := make(map[string]string, 0)
	// Core Instance Configuration
	result[string(EC2AMIID)] = attr.AMI
	result[string(EC2INSTANCETYPE)] = attr.InstanceType
	result[string(EC2INSTANCEID)] = attr.ID
	result[string(EC2KEYNAME)] = attr.KeyName
	result[string(EC2AvailabilityZone)] = attr.AvailabilityZone
	result[string(EC2TENANCY)] = attr.Tenancy
	result[string(EC2MONITORING)] = strconv.FormatBool(attr.Monitoring)
	result[string(EC2CPUCORECOUNT)] = strconv.Itoa(attr.CPUCoreCount)
	result[string(EC2CPUTHREADPERCORE)] = strconv.Itoa(attr.CPUThreadsPerCore)
	result[string(EC2EbsOptimzied)] = strconv.FormatBool(attr.EBSOptimized)

	// Networking & Security
	result[string(EC2SecurityGroupIDs)] = strings.Join(attr.VPCSecurityGroupIDs, ",")
	result[string(EC2SUBNETID)] = attr.SubnetID
	result[string(EC2AssociatePublicIPAddress)] = strconv.FormatBool(attr.AssociatePublicIPAddress)
	result[string(EC2PrivateIP)] = attr.PrivateIP
	result[string(EC2PrivateDnsName)] = attr.PrivateDNS
	result[string(EC2PublicIP)] = attr.PublicIP
	result[string(EC2PublicDnsName)] = attr.PublicDNS
	result[string(EC2SourceDestCheck)] = strconv.FormatBool(attr.SourceDestCheck)
	result[string(EC2IAMInstanceID)] = attr.IAMInstanceProfile
	result[string(EC2IAMInstanceARN)] = attr.ARN

	// Storage (EBS Volumes) - serialize as JSON for complex structures
	if len(attr.RootBlockDevice) > 0 {
		rootBlockJSON, _ := json.Marshal(attr.RootBlockDevice)
		result[string(EC2RootBlockDevice)] = string(rootBlockJSON)

		// Extract specific attributes from root block device
		if rootBlock := attr.RootBlockDevice[0]; rootBlock != nil {
			if deviceName, ok := rootBlock["device_name"].(string); ok {
				result[string(EC2BlockDeviceName)] = deviceName
			}
			if volumeID, ok := rootBlock["volume_id"].(string); ok {
				result[string(EC2VolumeID)] = volumeID
			}
			if volumeSize, ok := rootBlock["volume_size"].(float64); ok {
				result[string(EC2VolumeSize)] = strconv.FormatFloat(volumeSize, 'f', 0, 64)
			}
			if volumeType, ok := rootBlock["volume_type"].(string); ok {
				result[string(EC2VolumeType)] = volumeType
			}
			if encrypted, ok := rootBlock["encrypted"].(bool); ok {
				result[string(EC2VolumeEncrypted)] = strconv.FormatBool(encrypted)
			}
			if deleteOnTermination, ok := rootBlock["delete_on_termination"].(bool); ok {
				result[string(EC2DeleteOnTermination)] = strconv.FormatBool(deleteOnTermination)
			}
		}
	}

	if len(attr.EBSBlockDevice) > 0 {
		ebsBlockJSON, _ := json.Marshal(attr.EBSBlockDevice)
		result[string(EC2EBSBlockDevice)] = string(ebsBlockJSON)
	}

	// Metadata & User Data
	if len(attr.MetadataOptions) > 0 {
		metadataJSON, _ := json.Marshal(attr.MetadataOptions)
		result[string(EC2MetadataOptions)] = string(metadataJSON)
	}

	if attr.UserData != nil {
		if userData, ok := attr.UserData.(string); ok {
			result[string(EC2UserData)] = userData
		}
	}

	if attr.UserDataBase64 != nil {
		if userDataBase64, ok := attr.UserDataBase64.(string); ok {
			result[string(EC2UserDataBase64)] = userDataBase64
		}
	}

	// State
	result[string(EC2InstanceState)] = attr.InstanceState

	// Remove empty values
	for k, v := range result {
		if v == "" || v == "[]" || v == "null" {
			delete(result, k)
		}
	}

	return result
}
