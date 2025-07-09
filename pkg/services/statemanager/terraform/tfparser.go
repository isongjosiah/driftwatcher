package terraform

import (
	"drift-watcher/pkg/services/statemanager"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func StateFileFromConfig(configFilePath string) (string, error) {
	defaultStatePath := ""
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(configFilePath)
	if diags.HasErrors() {
		return defaultStatePath, errors.Wrap(diags, fmt.Sprintf("Failed to parse terraform hcl file %s", configFilePath))
	}

	terraformBlockSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "terraform",
			},
		},
	}
	partialContent, _, diags := file.Body.PartialContent(terraformBlockSchema)
	if diags.HasErrors() {
		return "", errors.Wrap(diags, "Failed to retrieve backend from terraform configuration file")
	}
	terraformSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "backend",
				LabelNames: []string{"type"}, // backend "s3" { ... }
			},
		},
		Attributes: []hcl.AttributeSchema{
			{Name: "required_version"},
			{Name: "required_providers"},
		},
	}
	for _, block := range partialContent.Blocks {
		if block.Type == "terraform" {
			content, _, diags := block.Body.PartialContent(terraformSchema)
			if diags.HasErrors() {
				return "", errors.Wrap(diags, "Failed to retrieve backend from terraform configuration file")
			}
			for _, backendBlock := range content.Blocks {
				if backendBlock.Type == "backend" {
					config, err := ParseBackendBlock(backendBlock)
					if err != nil {
						return "", err
					}
					if config.Type == "local" {
						defaultStatePath = config.Config.Path
						break
					}
				}
			}
		}
	}

	if defaultStatePath == "" {
		configDir := filepath.Dir(configFilePath)
		defaultStatePath = configDir + "/terraform.tfstate"
		slog.Warn("no local backend found in terraform configuration file. Checking or default state file in configuration path " + defaultStatePath)
	}

	return defaultStatePath, nil
}

// BackendConfig represents the parsed backend configuration
type BackendConfig struct {
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	Workspace string         `json:"workspace,omitempty"`
}

// ParseBackendBlock parses a specific backend block
func ParseBackendBlock(backendBlock *hcl.Block) (*statemanager.BackendConfig, error) {
	if len(backendBlock.Labels) == 0 {
		return nil, fmt.Errorf("backend block missing type label")
	}

	backendType := backendBlock.Labels[0]
	config := make(map[string]any)

	// Get all attributes from the backend block
	attributes, diags := backendBlock.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to get backend attributes: %s", diags.Error())
	}

	// Evaluate each attribute
	evalCtx := &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
		Functions: make(map[string]function.Function),
	}

	for name, attr := range attributes {
		value, diags := attr.Expr.Value(evalCtx)
		if diags.HasErrors() {
			// Store as string if we can't evaluate
			config[name] = fmt.Sprintf("<%s>", diags.Error())
			continue
		}

		// Convert cty.Value to Go any
		goValue, err := CtyValueToGo(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for %s: %w", name, err)
		}
		config[name] = goValue
	}

	var details statemanager.ConfigDetails
	configByte, err := json.Marshal(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal backend config information")
	}
	if err := json.Unmarshal(configByte, &details); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal backend config information")
	}

	return &statemanager.BackendConfig{
		Type:   backendType,
		Config: details,
	}, nil
}

// CtyValueToGo converts cty.Value to Go any
func CtyValueToGo(val cty.Value) (any, error) {
	if val.IsNull() {
		return nil, nil
	}

	switch val.Type() {
	case cty.String:
		return val.AsString(), nil
	case cty.Number:
		bf := val.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return i, nil
		}
		f, _ := bf.Float64()
		return f, nil
	case cty.Bool:
		return val.True(), nil
	}

	if val.Type().IsListType() || val.Type().IsSetType() || val.Type().IsTupleType() {
		var result []any
		for it := val.ElementIterator(); it.Next(); {
			_, elemVal := it.Element()
			goVal, err := CtyValueToGo(elemVal)
			if err != nil {
				return nil, err
			}
			result = append(result, goVal)
		}
		return result, nil
	}

	if val.Type().IsMapType() || val.Type().IsObjectType() {
		result := make(map[string]any)
		for it := val.ElementIterator(); it.Next(); {
			key, elemVal := it.Element()
			goVal, err := CtyValueToGo(elemVal)
			if err != nil {
				return nil, err
			}
			result[key.AsString()] = goVal
		}
		return result, nil
	}

	return fmt.Sprintf("<%s>", val.Type().FriendlyName()), nil
}
