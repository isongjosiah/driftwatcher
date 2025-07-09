package terraform

import (
	"drift-watcher/pkg/services/statemanager"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// TerraformState represents the root structure of a .tfstate file
type TerraformState struct {
	Version          int               `json:"version"`
	TerraformVersion string            `json:"terraform_version"`
	Serial           int               `json:"serial"`
	Lineage          string            `json:"lineage"`
	Outputs          map[string]Output `json:"outputs"`
	Resources        []Resource        `json:"resources"`
	CheckResults     any               `json:"check_results,omitempty"`
	Modules          []Module          `json:"modules,omitempty"` // For older versions
}

// Output represents a Terraform output value
type Output struct {
	Value     any  `json:"value"`
	Type      any  `json:"type"`
	Sensitive bool `json:"sensitive,omitempty"`
}

// Resource represents a Terraform resource in the state
type Resource struct {
	Mode      string                    `json:"mode"`
	Type      string                    `json:"type"`
	Name      string                    `json:"name"`
	Provider  statemanager.ProviderType `json:"provider"`
	Instances []Instance                `json:"instances"`
	EachMode  string                    `json:"each,omitempty"`
	Module    string                    `json:"module,omitempty"`
}

// Instance represents a resource instance
type Instance struct {
	SchemaVersion       int               `json:"schema_version"`
	Attributes          map[string]any    `json:"attributes"`
	AttributesFlat      map[string]string `json:"attributes_flat,omitempty"`
	Private             string            `json:"private,omitempty"`
	Dependencies        []string          `json:"dependencies,omitempty"`
	IndexKey            any               `json:"index_key,omitempty"`
	Status              string            `json:"status,omitempty"`
	Deposed             string            `json:"deposed,omitempty"`
	CreateBeforeDestroy bool              `json:"create_before_destroy,omitempty"`
}

// Module represents a Terraform module (for older state versions)
type Module struct {
	Path         []string                  `json:"path"`
	Outputs      map[string]Output         `json:"outputs"`
	Resources    map[string]LegacyResource `json:"resources"`
	Dependencies []string                  `json:"dependencies"`
}

// LegacyResource represents resource format in older state versions
type LegacyResource struct {
	Type         string      `json:"type"`
	Primary      *Instance   `json:"primary"`
	Deposed      []*Instance `json:"deposed"`
	Provider     string      `json:"provider"`
	Dependencies []string    `json:"depends_on"`
}

// StateParser provides methods to parse and analyze Terraform state files
type StateParser struct {
	State *TerraformState
}

// NewStateParser creates a new StateParser instance
func NewStateParser() *StateParser {
	return &StateParser{}
}

// ParseFile parses a .tfstate file from the given file path
func (p *StateParser) ParseFile(filePath string) error {
	fileHandler, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "Terraform state file does not exist")
		}
		return errors.Wrap(err, "Unable to retrieve file description")
	}
	if fileHandler.IsDir() {
		return fmt.Errorf("Terraform state directories are not currently supported")
	}
	ext := filepath.Ext(filePath)
	switch ext {
	case ".tf":
		filePath, err = StateFileFromConfig(filePath)
		if err != nil {
			return err
		}
	case ".tfstate":
		break
	default:
		return fmt.Errorf("%s file is not currently supported", ext)
	}

	fileHandler, err = os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "Terraform state file does not exist")
		}
		return errors.Wrap(err, "Unable to retrieve file description")
	}
	if fileHandler.IsDir() {
		return fmt.Errorf("Terraform state directories are not currently supported")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return p.ParseBytes(data)
}

// ParseBytes parses .tfstate data from a byte slice
func (p *StateParser) ParseBytes(data []byte) error {
	var state TerraformState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	p.State = &state
	return nil
}

// GetVersion returns the Terraform version used to create the state
func (p *StateParser) GetVersion() string {
	if p.State == nil {
		return ""
	}
	return p.State.TerraformVersion
}

// GetStateVersion returns the state format version
func (p *StateParser) GetStateVersion() int {
	if p.State == nil {
		return 0
	}
	return p.State.Version
}

// GetResources returns all resources in the state
func (p *StateParser) GetResources() []Resource {
	if p.State == nil {
		return nil
	}
	return p.State.Resources
}

// GetResourcesByType returns resources of a specific type
func (p *StateParser) GetResourcesByType(resourceType string) []statemanager.StateResource {
	if p.State == nil {
		return nil
	}

	var resources []statemanager.StateResource
	for _, resource := range p.State.Resources {
		if resource.Type != resourceType {
			continue
		}

		newStateResource := statemanager.StateResource{
			Mode:      resource.Mode,
			Module:    resource.Module,
			Name:      resource.Name,
			Type:      resource.Type,
			Provider:  resource.Provider,
			Instances: make([]statemanager.ResourceInstance, len(resource.Instances)),
			ToolData:  make(map[string]any),
		}

		for i, instance := range resource.Instances {
			newStateResource.Instances[i] = statemanager.ResourceInstance{
				// Note: There's a typo in the target struct's field name: 'ScheamVersion' instead of 'SchemaVersion'.
				// I'm mapping it as it is in your target struct. If it's a typo, correct the target struct.
				ScheamVersion: instance.SchemaVersion,
				Attributes:    instance.Attributes,
				Dependencies:  instance.Dependencies,
			}
		}
		resources = append(resources, newStateResource)
	}
	return resources
}

// GetResourceByName returns a specific resource by type and name
func (p *StateParser) GetResourceByName(resourceType, name string) *Resource {
	if p.State == nil {
		return nil
	}

	for _, resource := range p.State.Resources {
		if resource.Type == resourceType && resource.Name == name {
			return &resource
		}
	}
	return nil
}

// GetOutputs returns all outputs from the state
func (p *StateParser) GetOutputs() map[string]Output {
	if p.State == nil {
		return nil
	}
	return p.State.Outputs
}

// GetOutput returns a specific output by name
func (p *StateParser) GetOutput(name string) (*Output, bool) {
	if p.State == nil {
		return nil, false
	}

	output, exists := p.State.Outputs[name]
	return &output, exists
}

// GetResourceAttributes returns attributes for a specific resource instance
func (p *StateParser) GetResourceAttributes(resourceType, name string, instanceIndex int) map[string]any {
	resource := p.GetResourceByName(resourceType, name)
	if resource == nil || instanceIndex >= len(resource.Instances) {
		return nil
	}

	return resource.Instances[instanceIndex].Attributes
}

// ListResourceTypes returns all unique resource types in the state
func (p *StateParser) ListResourceTypes() []string {
	if p.State == nil {
		return nil
	}

	typeSet := make(map[string]bool)
	for _, resource := range p.State.Resources {
		typeSet[resource.Type] = true
	}

	var types []string
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// GetResourceCount returns the total number of resources
func (p *StateParser) GetResourceCount() int {
	if p.State == nil {
		return 0
	}
	return len(p.State.Resources)
}

// GetResourceInstanceCount returns the total number of resource instances
func (p *StateParser) GetResourceInstanceCount() int {
	if p.State == nil {
		return 0
	}

	count := 0
	for _, resource := range p.State.Resources {
		count += len(resource.Instances)
	}
	return count
}
