package provider

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

type ProviderI interface {
	InfrastructreMetadata(resourceType string, filters map[string]string) ([]InfrastructureResource, error)
	CompareWithTerraformConfig() error // TODO: come back here after setting up terraform service
}
