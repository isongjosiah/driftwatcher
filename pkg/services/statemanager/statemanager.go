// Package statemanager provides interfaces and types for parsing and managing
// Infrastructure as Code (IaC) state files, particularly Terraform state files.
// It abstracts the process of reading state configurations and extracting
// resource information for drift detection purposes.
package statemanager

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"encoding/json"
	"fmt"
)

// StateContent represents the parsed content of an Infrastructure as Code state file.
// It contains metadata about the IaC tool, schema version, and the actual resources
// defined in the state, along with backend configuration information.
type StateContent struct {
	StateVersion  string          `json:"state_version,omitempty"`
	Tool          IaCTool         `json:"tool,omitempty"`
	ToolVersion   string          `json:"tool_version,omitempty"`
	ToolMetadata  map[string]any  `json:"tool_metadata,omitempty"`
	SchemaVersion string          `json:"schema_version,omitempty"`
	StateId       string          `json:"state_id,omitempty"`
	Resource      []StateResource `json:"resource,omitempty"`
	RawState      json.RawMessage `json:"raw_state,omitempty"`
	BackendConfig BackendConfig   `json:"backend_config"`
}

// ConfigDetails contains the specific configuration parameters for a backend.
// These details vary depending on the backend type (e.g., S3, local, etc.).
// NOTE: only local backend is supported currently
type ConfigDetails struct {
	Path          string `json:"path,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	Encrypt       bool   `json:"encrypt,omitempty"`
	Key           string `json:"key,omitempty"`
	DynamoDBTable string `json:"dynamodb_table,omitempty"`
}

// BackendConfig describes the backend configuration for storing state files.
// It includes the backend type and its specific configuration parameters.
type BackendConfig struct {
	Type   string        `json:"type,omitempty"`
	Config ConfigDetails `json:"config"`
}

// StateResource represents a single resource defined in the state file.
// It contains metadata about the resource and its current state, including
// all configured attributes and their values.
type StateResource struct {
	Mode     string       `json:"mode,omitempty"`
	Module   string       `json:"module,omitempty"`
	Name     string       `json:"name,omitempty"`
	Type     string       `json:"type,omitempty"`
	Provider ProviderType `json:"provider,omitempty"`
	// NOTE: assuming only one instance exists for a resource
	Instances []ResourceInstance `json:"instances,omitempty"`
	ToolData  map[string]any     `json:"tool_data,omitempty"`
}

// ResourceType returns the type of the resource (e.g., "aws_instance").
// This method implements part of the resource identification interface
func (s StateResource) ResourceType() string {
	return s.Type
}

// AttributeValue retrieves the value of a specific attribute from the resource's
// first instance. It returns an error if no instances exist or if the attribute
// value cannot be converted to a string.
//
// Parameters:
//   - attribute: The name of the attribute to retrieve
//
// Returns:
//   - The string value of the attribute, or empty string if not found
//   - An error if no instances exist or if type conversion fails
func (s StateResource) AttributeValue(attribute string) (string, error) {
	if len(s.Instances) == 0 {
		return "", fmt.Errorf("No Instance for resource")
	}

	data, ok := s.Instances[0].Attributes[attribute]
	if !ok {
		return "", nil
	}
	value, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("attribute value cannot be parsed to string")
	}
	return value, nil
}

// ResourceInstance represents a single instance of a resource.
// Resources can have multiple instances when using count or for_each,
// but most resources have only one instance.
type ResourceInstance struct {
	ScheamVersion int            `json:"scheam_version,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
	Dependencies  []string       `json:"dependencies,omitempty"`
}

// StateManagerI defines the interface for parsing and managing IaC state files.
// Implementations of this interface handle the specifics of different IaC tools
// and state file formats while providing a consistent API for state operations.
//
//counterfeiter:generate . StateManagerI
type StateManagerI interface {
	ParseStateFile(ctx context.Context, statePath string) (StateContent, error)
	RetrieveResources(ctx context.Context, content StateContent, resourceType string) ([]StateResource, error)
}
