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

// GetResources extracts resources from parsed Terraform data
func (pt *ParsedTerraform) GetResources() ([]Resource, error) {
	if pt.Type == "state" && pt.State != nil {
		return pt.State.Resources, nil
	}

	if pt.Type == "hcl" {
		return processExtractedResources(&pt.HCLConfig)
	}

	return nil, fmt.Errorf("no valid data to extract resources from")
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
