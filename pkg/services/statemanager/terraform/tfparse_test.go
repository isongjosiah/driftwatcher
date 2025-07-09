package terraform_test

import (
	"drift-watcher/pkg/services/statemanager/terraform"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// Helper to create a temporary HCL config file
func createTempHCLFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.tf")
	require.NoError(t, err)
	defer tmpFile.Close()
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	return tmpFile.Name()
}

func TestStateFileFromConfig_LocalBackend(t *testing.T) {
	configContent := `
	terraform {
		backend "local" {
			path = "path/to/my/terraform.tfstate"
		}
	}`
	configFilePath := createTempHCLFile(t, configContent)
	defer os.Remove(configFilePath)

	statePath, err := terraform.StateFileFromConfig(configFilePath)
	require.NoError(t, err)
	assert.Equal(t, "path/to/my/terraform.tfstate", statePath)
}

func TestStateFileFromConfig_NoLocalBackend(t *testing.T) {
	configContent := `
	terraform {
		backend "s3" {
			bucket = "my-s3-bucket"
		}
	}`
	configFilePath := createTempHCLFile(t, configContent)
	defer os.Remove(configFilePath)

	expectedDefaultPath := filepath.Dir(configFilePath) + "/terraform.tfstate"
	statePath, err := terraform.StateFileFromConfig(configFilePath)
	require.NoError(t, err)
	assert.Equal(t, expectedDefaultPath, statePath)
}

func TestStateFileFromConfig_NoBackendBlock(t *testing.T) {
	configContent := `
	terraform {
		required_version = ">= 1.0.0"
	}`
	configFilePath := createTempHCLFile(t, configContent)
	defer os.Remove(configFilePath)

	expectedDefaultPath := filepath.Dir(configFilePath) + "/terraform.tfstate"
	statePath, err := terraform.StateFileFromConfig(configFilePath)
	require.NoError(t, err)
	assert.Equal(t, expectedDefaultPath, statePath)
}

func TestStateFileFromConfig_InvalidHCL(t *testing.T) {
	configContent := `
	terraform {
		backend "local" {
			path = 
		}
	}` // Malformed HCL
	configFilePath := createTempHCLFile(t, configContent)
	defer os.Remove(configFilePath)

	_, err := terraform.StateFileFromConfig(configFilePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to parse terraform hcl file")
}

func TestStateFileFromConfig_NonExistentFile(t *testing.T) {
	path := "/path/to/nonexistent/file.tf"
	_, err := terraform.StateFileFromConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to read file; The configuration file \"/path/to/nonexistent/file.tf\" could not be read.")
}

func TestParseBackendBlock_Success(t *testing.T) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(`
		backend "s3" {
			bucket = "my-bucket"
			region = "us-east-1"
			encrypt = true
			key = "path/to/state"
			dynamodb_table = "my-lock-table"
			workspace_key_prefix = "env:"
		}
	`), "test.hcl")
	require.False(t, diags.HasErrors())

	backendBlockSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "backend",
				LabelNames: []string{"type"},
			},
		},
	}
	content, _, diags := file.Body.PartialContent(backendBlockSchema)
	require.False(t, diags.HasErrors())
	require.Len(t, content.Blocks, 1)

	backendConfig, err := terraform.ParseBackendBlock(content.Blocks[0])
	require.NoError(t, err)
	assert.NotNil(t, backendConfig)
	assert.Equal(t, "s3", backendConfig.Type)
	assert.Equal(t, "my-bucket", backendConfig.Config.Bucket)
	assert.Equal(t, "us-east-1", backendConfig.Config.Region)
	assert.Equal(t, true, backendConfig.Config.Encrypt)
	assert.Equal(t, "path/to/state", backendConfig.Config.Key)
	assert.Equal(t, "my-lock-table", backendConfig.Config.DynamoDBTable)
}

func TestParseBackendBlock_MissingTypeLabel(t *testing.T) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(`
		backend { # Missing "type" label
			bucket = "my-bucket"
		}
	`), "test.hcl")
	require.False(t, diags.HasErrors())

	backendBlockSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "backend",
			},
		},
	}
	content, _, diags := file.Body.PartialContent(backendBlockSchema)
	require.False(t, diags.HasErrors())
	require.Len(t, content.Blocks, 1)

	_, err := terraform.ParseBackendBlock(content.Blocks[0])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backend block missing type label")
}

func TestParseBackendBlock_InvalidAttribute(t *testing.T) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(`
		backend "s3" {
			bucket = var.non_existent_variable
		}
	`), "test.hcl")
	require.False(t, diags.HasErrors())

	backendBlockSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "backend",
				LabelNames: []string{"type"},
			},
		},
	}
	content, _, diags := file.Body.PartialContent(backendBlockSchema)
	require.False(t, diags.HasErrors())
	require.Len(t, content.Blocks, 1)

	backendConfig, err := terraform.ParseBackendBlock(content.Blocks[0])
	require.NoError(t, err) // Error is handled by storing a string representation
	assert.Contains(t, backendConfig.Config.Bucket, "Unknown variable")
}

func TestCtyValueToGo_Null(t *testing.T) {
	val := cty.NullVal(cty.String)
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Nil(t, goVal)
}

func TestCtyValueToGo_String(t *testing.T) {
	val := cty.StringVal("hello")
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, "hello", goVal)
}

func TestCtyValueToGo_Number_Int(t *testing.T) {
	val := cty.NumberIntVal(123)
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, int64(123), goVal)
}

func TestCtyValueToGo_Number_Float(t *testing.T) {
	val := cty.NumberFloatVal(123.45)
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, 123.45, goVal)
}

func TestCtyValueToGo_Bool(t *testing.T) {
	val := cty.BoolVal(true)
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, true, goVal)
}

func TestCtyValueToGo_List(t *testing.T) {
	val := cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("1")})
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "1"}, goVal)
}

func TestCtyValueToGo_Set(t *testing.T) {
	val := cty.SetVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	// Sets are unordered, so convert to map for comparison
	resultSlice := goVal.([]any)
	resultMap := make(map[any]bool)
	for _, v := range resultSlice {
		resultMap[v] = true
	}
	assert.True(t, resultMap["a"])
	assert.True(t, resultMap["b"])
	assert.Len(t, resultMap, 2)
}

func TestCtyValueToGo_Tuple(t *testing.T) {
	val := cty.TupleVal([]cty.Value{cty.StringVal("x"), cty.BoolVal(false)})
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, []any{"x", false}, goVal)
}

func TestCtyValueToGo_Map(t *testing.T) {
	val := cty.MapVal(map[string]cty.Value{
		"key1": cty.StringVal("value1"),
		"key2": cty.StringVal("10"),
	})
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key1": "value1", "key2": "10"}, goVal)
}

func TestCtyValueToGo_Object(t *testing.T) {
	val := cty.ObjectVal(map[string]cty.Value{
		"attr1": cty.StringVal("obj_val"),
		"attr2": cty.BoolVal(true),
	})
	goVal, err := terraform.CtyValueToGo(val)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"attr1": "obj_val", "attr2": true}, goVal)
}

func TestStateFileFromConfig_SlogWarning(t *testing.T) {
	configContent := `
	terraform {
		backend "s3" {
			bucket = "my-s3-bucket"
		}
	}`
	configFilePath := createTempHCLFile(t, configContent)
	defer os.Remove(configFilePath)

	// Capture slog output
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	_, err := terraform.StateFileFromConfig(configFilePath)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "no local backend found in terraform configuration file.")
}
