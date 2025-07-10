// Package terraform provides Terraform-specific implementation of the state manager interface.
// It handles parsing and processing of Terraform state files to extract resource information
// for drift detection and comparison with live infrastructure.
package terraform

import (
	"context"
	"drift-watcher/pkg/services/statemanager"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
)

// TerraformStateManager implements the StateManagerI interface for Terraform state files.
// It provides functionality to parse Terraform state files and extract resource information
// in a standardized format for drift detection.
type TerraformStateManager struct {
	parser *StateParser
}

func NewTerraformManager() *TerraformStateManager {
	return &TerraformStateManager{
		parser: NewStateParser(),
	}
}

// ParseStateFile parses a Terraform state file from the specified path and converts it
// to a standardized StateContent format. This method handles file validation, parsing,
// and conversion to the internal representation used by the drift detection system.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - statePath: File system path to the Terraform state file (.tfstate)
//
// Returns:
//   - statemanager.StateContent: Parsed and standardized state content
//   - error: Any error encountered during file reading, parsing, or conversion
func (t *TerraformStateManager) ParseStateFile(ctx context.Context, statePath string) (statemanager.StateContent, error) {
	var out statemanager.StateContent
	_, err := os.Stat(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, errors.Wrap(err, "state file does not exist")
		}
		return out, errors.Wrap(err, "Failed to retrieve file info for tfstate file")
	}

	if err := t.parser.ParseFile(statePath); err != nil {
		return out, err
	}

	statecontent, err := ConvertTerraformStateToStateContent(*t.parser.State)
	if err != nil {
		return out, err
	}

	return statecontent, nil
}

// ConvertTerraformStateToStateContent converts a TerraformState object to a StateContent object.
// This function maps Terraform-specific state structure to the standardized StateContent format
// used throughout the drift detection system. It handles version conversion, resource mapping,
// and metadata extraction.
//
// Parameters:
//   - tfState: The parsed Terraform state object to convert
//
// Returns:
//   - statemanager.StateContent: Standardized state content with mapped fields
//   - error: Any error encountered during conversion or JSON marshaling
func ConvertTerraformStateToStateContent(tfState TerraformState) (statemanager.StateContent, error) {
	newState := statemanager.StateContent{
		StateVersion:  strconv.Itoa(tfState.Version), // Convert int to string
		Tool:          statemanager.TerraformTool,
		ToolVersion:   tfState.TerraformVersion,
		ToolMetadata:  make(map[string]any),
		SchemaVersion: strconv.Itoa(tfState.Version), // Using TerraformState.Version as SchemaVersion
		StateId:       tfState.Lineage,
		BackendConfig: statemanager.BackendConfig{}, // No direct mapping, initialize empty
	}

	// Populate ToolMetadata
	newState.ToolMetadata["serial"] = tfState.Serial

	// Convert Resources
	for _, res := range tfState.Resources {
		stateRes := statemanager.StateResource{
			Mode:     res.Mode,
			Module:   res.Module,
			Name:     res.Name,
			Type:     res.Type,
			Provider: statemanager.ProviderType(res.Provider), // Direct string conversion
			ToolData: make(map[string]any),
		}

		// Add EachMode to ToolData if present
		if res.EachMode != "" {
			stateRes.ToolData["each_mode"] = res.EachMode
		}

		// Convert Instances
		for _, inst := range res.Instances {
			stateInst := statemanager.ResourceInstance{
				ScheamVersion: inst.SchemaVersion,
				Attributes:    inst.Attributes,
				Dependencies:  inst.Dependencies,
			}
			stateRes.Instances = append(stateRes.Instances, stateInst)
		}
		newState.Resource = append(newState.Resource, stateRes)
	}

	// Marshal the original state (or parts of it) into RawState
	// For simplicity, we'll marshal the entire original state into RawState.
	// You might want to selectively include parts like Outputs, CheckResults, Modules here.
	rawStateBytes, err := json.Marshal(tfState)
	if err != nil {
		return statemanager.StateContent{}, fmt.Errorf("failed to marshal raw state: %w", err)
	}
	newState.RawState = json.RawMessage(rawStateBytes)

	return newState, nil
}

// RetrieveResources retrieves all resources of a specific type from the parsed state content.
// This method filters the parsed state to return only resources matching the specified type,
// which is useful for targeted drift detection on specific resource types.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - content: The parsed state content (currently unused but maintained for interface compatibility)
//   - resourceType: The type of resources to retrieve (e.g., "aws_instance", "azurerm_virtual_machine")
//
// Returns:
//   - []statemanager.StateResource: List of resources matching the specified type
//   - error: Any error encountered during resource retrieval
func (t *TerraformStateManager) RetrieveResources(ctx context.Context, content statemanager.StateContent, resourceType string) ([]statemanager.StateResource, error) {
	if t.parser == nil {
		return nil, fmt.Errorf("")
	}
	resources := t.parser.GetResourcesByType(resourceType)
	return resources, nil
}
