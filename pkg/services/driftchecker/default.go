package driftchecker

import (
	"context"
	"drift-watcher/pkg/services/provider"
	"drift-watcher/pkg/services/statemanager"
	"fmt"
	"log/slog"
	"time"
)

type DefaultDriftChecker struct{}

// NewDefaultDriftChecker creates a new instance of AWSDriftChecker.
func NewDefaultDriftChecker() *DefaultDriftChecker {
	return &DefaultDriftChecker{}
}

// CompareStates compares the attributes of a live AWS resource with its desired state.
// It iterates through the specified attributesToTrack and identifies any discrepancies.
//
// Parameters:
//
//	ctx: The context for the operation, allowing for cancellation or timeouts.
//	liveState: An interface representing the live resource's state fetched from AWS.
//	           It must implement methods to get resource type and attribute values.
//	desiredState: An interface representing the desired state of the resource from
//	              the configuration (e.g., Terraform state). It must also implement
//	              methods to get resource type and attribute values.
//	attributesToTrack: A slice of attribute keys (e.g., "instance_type", "tags.Name")
//	                   to compare.
//
// Returns:
//
//	A *DriftReport detailing any drift found for the tracked attributes, or nil if
//	the initial setup of the report fails.
//	An error if the resource types do not match or other critical issues occur.
func (d *DefaultDriftChecker) CompareStates(ctx context.Context, liveState provider.InfrastructureResourceI, desiredState statemanager.StateResource, attributesToTrack []string) (*DriftReport, error) {
	out := &DriftReport{
		GeneratedAt: time.Now(),
	}
	if liveState == nil {
		out.Status = ResourceMissingInInfrastructure
		out.HasDrift = true
		return out, nil
	}

	if liveState.ResourceType() != desiredState.ResourceType() {
		return out, fmt.Errorf("resource type mismatch: live resource %s does not match desired type %s", liveState.ResourceType(), desiredState.ResourceType())
	}

	out.ResourceType = liveState.ResourceType()

	overallDrift := Match
	for _, attribute := range attributesToTrack {
		driftItem := DriftItem{
			Field: attribute,
		}

		// TODO: add drift Item to show that drift check for this attribute failed
		liveVal, err := liveState.AttributeValue(attribute)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to retrieve value of %s attribute for live state", attribute))
			continue
		}
		desiredVal, err := desiredState.AttributeValue(attribute)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to retrieve value of %s attribute for desired state", attribute))
			continue
		}

		driftItem.TerraformValue = desiredVal
		driftItem.ActualValue = liveVal
		driftItem.DriftType = Match // default value

		switch {
		case driftItem.TerraformValue == "" && driftItem.ActualValue != "":
			driftItem.DriftType = AttributeMissingInTerraform
			if overallDrift == Match {
				overallDrift = Drift
			}
		case driftItem.ActualValue == "" && driftItem.TerraformValue != "":
			driftItem.DriftType = AttributeMissingInInfrastructure
			if overallDrift == Match {
				overallDrift = Drift
			}
		case driftItem.TerraformValue != driftItem.ActualValue:
			driftItem.DriftType = AttributeValueChanged
			if overallDrift == Match {
				overallDrift = Drift
			}
		}

		out.DriftDetails = append(out.DriftDetails, driftItem)

	}

	out.Status = overallDrift
	out.HasDrift = overallDrift != Match

	return out, nil
}
