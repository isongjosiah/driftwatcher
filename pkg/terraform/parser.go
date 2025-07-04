package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/pkg/errors"
)

func ParseTerraformFile(filePath string) (ParsedTerraform, error) {
	var parsedInfo ParsedTerraform
	ext := filepath.Ext(filePath)
	switch strings.ToLower(ext) {
	case ".tfstate":
		return ParseStateFile(filePath)
	// case ".hcl":
	//	return parseHCLFile(filePath)
	default:
		return parsedInfo, fmt.Errorf("invalid file format. Only .tfstate files are supported at the moment")
	}
}

func ParseStateFile(filePath string) (ParsedTerraform, error) {
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

// GetResources extracts resources from parsed Terraform data
func (pt *ParsedTerraform) GetResources() ([]Resource, error) {
	switch pt.Type {
	case "state":
		if pt.State == nil {
			return []Resource{}, nil
		}
		return pt.State.Resources, nil
	// case "hcl":
	//	return processExtractedResources(&pt.HCLConfig)
	default:
		return nil, fmt.Errorf("no valid data to extract resources from")
	}
}

func parseHCLFile(filePath string) (ParsedTerraform, error) {
	parser := hclparse.NewParser()
	var out ParsedTerraform

	file, diags := parser.ParseHCLFile(filePath)
	//if diags.HasErrors() {
	//	return out, errors.Wrap(diags, fmt.Sprintf("Failed to parse terraform hcl file %s", filePath))
	//}

	config, err := extractTerraformConfigFromHCL(file.Body)
	fmt.Printf("%#v", config)
	if err != nil {
		return out, errors.Wrap(diags, fmt.Sprintf("Failed to parse terraform hcl file %s", filePath))
	}

	out = ParsedTerraform{
		Type:      "hcl",
		FilePath:  filePath,
		HCLConfig: *config,
	}
	return out, nil
}
