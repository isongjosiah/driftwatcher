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

type TerraformStateManager struct {
	parser *StateParser
}

func NewTerraformManager() *TerraformStateManager {
	return &TerraformStateManager{
		parser: NewStateParser(),
	}
}

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

func (t *TerraformStateManager) RetrieveResources(ctx context.Context, content statemanager.StateContent, resourceType string) ([]statemanager.StateResource, error) {
	if t.parser == nil {
		return nil, fmt.Errorf("")
	}
	resources := t.parser.GetResourcesByType(resourceType)
	return resources, nil
}
