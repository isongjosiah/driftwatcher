package driftchecker

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/statemanager"
	"time"
)

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

//counterfeiter:generate . DriftChecker
type DriftChecker interface {
	CompareStates(ctx context.Context, liveData provider.InfrastructureResourceI, desiredState statemanager.StateResource, attributesToTrack []string) (*DriftReport, error)
}
