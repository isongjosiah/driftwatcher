package terraform

import "github.com/hashicorp/hcl/v2"

// TerraformState represents a Terraform state file structure
type TerraformState struct {
	Version          int            `json:"version"`
	TerraformVersion string         `json:"terraform_version"`
	Serial           int            `json:"serial"`
	Lineage          string         `json:"lineage"`
	Outputs          map[string]any `json:"outputs"`
	Resources        []Resource     `json:"resources"`
}

// Resource represents a resource in the Terraform state
type Resource struct {
	Mode         string     `json:"mode"`
	Type         string     `json:"type"`
	Name         string     `json:"name"`
	Provider     string     `json:"provider"`
	Instances    []Instance `json:"instances"`
	Dependencies []string   `json:"depends_on,omitempty"`
}

// Instance represents an instance of a resource
type Instance struct {
	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
	Dependencies  []string       `json:"dependencies,omitempty"`
}

// ParsedTerraform represents the result of parsing either format
type ParsedTerraform struct {
	Type     string // "state" or "hcl"
	FilePath string // path to the parsed file
	State    *TerraformState
	HCL      *hcl.File
	Body     hcl.Body
}
