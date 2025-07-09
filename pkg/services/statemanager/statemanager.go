package statemanager

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"encoding/json"
	"fmt"
)

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

type ConfigDetails struct {
	Path          string `json:"path,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	Encrypt       bool   `json:"encrypt,omitempty"`
	Key           string `json:"key,omitempty"`
	DynamoDBTable string `json:"dynamodb_table,omitempty"`
}

type BackendConfig struct {
	Type   string        `json:"type,omitempty"`
	Config ConfigDetails `json:"config"`
}

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

func (s StateResource) ResourceType() string {
	return s.Type
}

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

type ResourceInstance struct {
	ScheamVersion int            `json:"scheam_version,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
	Dependencies  []string       `json:"dependencies,omitempty"`
}

//counterfeiter:generate . StateManagerI
type StateManagerI interface {
	ParseStateFile(ctx context.Context, statePath string) (StateContent, error)
	RetrieveResources(ctx context.Context, content StateContent, resourceType string) ([]StateResource, error)
}
