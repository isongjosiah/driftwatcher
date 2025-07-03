package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/pkg/errors"
)

func ParseTerraformFile(filePath string) (ParsedTerraform, error) {
	var parsedInfo ParsedTerraform
	ext := filepath.Ext(filePath)
	switch strings.ToLower(ext) {
	case ".tfstate":
		return parseStateFile(filePath)
	case ".hcl":
		return parseHCLFile(filePath)
	default:
		return parsedInfo, fmt.Errorf("invalid file format. Only .tfstate and .hcl files are supported")
	}
}

func parseStateFile(filePath string) (ParsedTerraform, error) {
	var out ParsedTerraform
	file, err := os.Open(filePath)
	if err != nil {
		return out, errors.Wrap(err, fmt.Sprintf("Failed to open terraform state file %s for parsing", filePath))
	}
	defer file.Close()
	byteContent, err := io.ReadAll(file)
	if err != nil {
		return out, errors.Wrap(err, fmt.Sprintf("Failed to read terraform state file %s for parsing", filePath))
	}
	var state TerraformState
	if err := json.Unmarshal(byteContent, &state); err != nil {
		return out, errors.Wrap(err, fmt.Sprintf("Failed to unmarshal terraform state file %s byte content for parsing", filePath))
	}

	out = ParsedTerraform{
		Type:     "state",
		FilePath: filePath,
		State:    &state,
	}

	return out, nil
}

func parseHCLFile(filePath string) (ParsedTerraform, error) {
	parser := hclparse.NewParser()
	var out ParsedTerraform

	file, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return out, errors.Wrap(diags, fmt.Sprintf("Failed to parse terraform hcl file %s", filePath))
	}

	out = ParsedTerraform{
		Type:     "hcl",
		FilePath: filePath,
		HCL:      file,
		Body:     file.Body,
	}

	return out, nil
}

// GetResources extracts resources from parsed Terraform data
func (pt *ParsedTerraform) GetResources() ([]Resource, error) {
	if pt.Type == "state" && pt.State != nil {
		return pt.State.Resources, nil
	}

	if pt.Type == "hcl" && pt.Body != nil {
		return extractResourcesFromHCL(pt.Body)
	}

	return nil, fmt.Errorf("no valid data to extract resources from")
}

// extractResourcesFromHCL extracts resource information from HCL body
func extractResourcesFromHCL(body hcl.Body) ([]Resource, error) {
	var resources []Resource

	// Get the body content
	content, _, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "resource",
				LabelNames: []string{"type", "name"},
			},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to extract HCL content: %s", diags.Error())
	}

	// Process each resource block
	for _, block := range content.Blocks {
		if block.Type == "resource" {
			resource := Resource{
				Mode: "managed",
				Type: block.Labels[0],
				Name: block.Labels[1],
			}

			// Extract attributes (simplified)
			attrs, diags := block.Body.JustAttributes()
			if !diags.HasErrors() {
				instance := Instance{
					Attributes: make(map[string]interface{}),
				}

				for name, attr := range attrs {
					val, diags := attr.Expr.Value(nil)
					if !diags.HasErrors() {
						instance.Attributes[name] = val
					}
				}

				resource.Instances = []Instance{instance}
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}
