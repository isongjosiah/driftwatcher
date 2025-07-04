package provider

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"drift-watcher/pkg/terraform"
	"time"
)

// InfrastructureResource represents the metadata of an infrastructure resource.
// It includes common fields like ID, Type, Name, Region, and dynamic Attributes.
type InfrastructureResource struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Region     string            `json:"region"`
	Tags       map[string]string `json:"tags"`
	Attributes map[string]any    `json:"attributes"`
}

type DrfitItemValue = string

const (
	AttributeValueChanged            DrfitItemValue = "VALUE_CHANGED"
	AttributeMissingInTerraform      DrfitItemValue = "MISSING_IN_TERRAFORM"
	AttributeMissingInInfrastructure DrfitItemValue = "MISSING_IN_INFRASTRUCTURE"
)

// DriftItem represents a specific drift between expected and actual values
type DriftItem struct {
	Field          string         `json:"field"`
	TerraformValue any            `json:"terraform_value"`
	ActualValue    any            `json:"actual_value"`
	DriftType      DrfitItemValue `json:"drift_type"` // "VALUE_CHANGED", "MISSING_IN_TERRAFORM", "MISSING_IN_INFRASTRUCTURE"
}

type DriftReportStatus = string

const (
	Match                           DriftReportStatus = "MATCH"
	Drift                           DriftReportStatus = "DRIFT"
	ResourceMissingInTerraform      DriftReportStatus = "MISSING_IN_TERRAFORM"
	ResourceMissingInInfrastructure DriftReportStatus = "MISSING_IN_INFRASTRUCTURE"
)

// DriftReport represents the comparison result
type DriftReport struct {
	ResourceId   string      `json:"resource_id,omitempty"`
	ResourceType string      `json:"resource_type,omitempty"`
	ResourceName string      `json:"resource_nae,omitempty"`
	HasDrift     bool        `json:"has_drift,omitempty"`
	DriftDetails []DriftItem `json:"drift_details,omitempty"`
	GeneratedAt  time.Time   `json:"generated_at"`
	Status       string      `json:"status,omitempty"`
}

//counterfeiter:generate . ProviderI
type ProviderI interface {
	InfrastructreMetadata(ctx context.Context, resourceType string, filters map[string]string) (*InfrastructureResource, error)
	CompareActiveAndDesiredState(ctx context.Context, resourceType string, liveState *InfrastructureResource, desiredState terraform.Resource, attributesToTrack []string) (DriftReport, error)
}

type AttributeMapping = map[string]string
