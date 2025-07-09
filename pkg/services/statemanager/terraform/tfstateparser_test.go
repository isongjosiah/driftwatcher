package terraform_test

import (
	"drift-watcher/pkg/services/statemanager/terraform"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a dummy tfstate file for testing
func createDummyTFStateFileForParser(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.tfstate")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	return tmpFile.Name()
}

// Helper function to create a dummy .tf config file for testing stateFileFromConfig
func createDummyTFConfigFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.tf")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	return tmpFile.Name()
}

func TestNewStateParser(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.NotNil(t, parser)
	assert.Nil(t, parser.State)
}

func TestParseFile_Success_TFState(t *testing.T) {
	dummyStateContent := `{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {},
		"resources": []
	}`
	stateFilePath := createDummyTFStateFileForParser(t, dummyStateContent)
	defer os.Remove(stateFilePath)

	parser := terraform.NewStateParser()
	err := parser.ParseFile(stateFilePath)
	require.NoError(t, err)
	assert.NotNil(t, parser.State)
	assert.Equal(t, 4, parser.State.Version)
	assert.Equal(t, "1.0.0", parser.State.TerraformVersion)
}

func TestParseFile_Success_TFConfigWithLocalBackend(t *testing.T) {
	// Create a dummy .tfstate file that the .tf config will point to
	dummyStateContent := `{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {},
		"resources": []
	}`
	tempDir := t.TempDir()
	localStateFilePath := filepath.Join(tempDir, "my-local.tfstate")
	err := os.WriteFile(localStateFilePath, []byte(dummyStateContent), 0644)
	require.NoError(t, err)

	// Create a dummy .tf config file pointing to the local state file
	configContent := `
	terraform {
		backend "local" {
			path = "` + localStateFilePath + `"
		}
	}`
	configFilePath := createDummyTFConfigFile(t, configContent)
	defer os.Remove(configFilePath)

	parser := terraform.NewStateParser()
	err = parser.ParseFile(configFilePath)
	require.NoError(t, err)
	assert.NotNil(t, parser.State)
	assert.Equal(t, 4, parser.State.Version)
	assert.Equal(t, "1.0.0", parser.State.TerraformVersion)
}

func TestParseFile_NotExist(t *testing.T) {
	parser := terraform.NewStateParser()
	err := parser.ParseFile("/path/to/nonexistent/file.tfstate")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Terraform state file does not exist")
}

func TestParseFile_IsDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	parser := terraform.NewStateParser()
	err = parser.ParseFile(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Terraform state directories are not currently supported")
}

func TestParseFile_UnsupportedExtension(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	parser := terraform.NewStateParser()
	err = parser.ParseFile(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".txt file is not currently supported")
}

func TestParseFile_ReadError(t *testing.T) {
	// Create a file that we can't read (e.g., by changing permissions)
	tmpFile, err := os.CreateTemp("", "test-*.tfstate")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// On Unix-like systems, set permissions to 000 (no read)
	os.Chmod(tmpFile.Name(), 0000)
	defer os.Chmod(tmpFile.Name(), 0644) // Restore permissions for cleanup

	parser := terraform.NewStateParser()
	err = parser.ParseFile(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestParseFile_InvalidJSON(t *testing.T) {
	invalidJsonPath := createDummyTFStateFileForParser(t, `{"version": "invalid"}`)
	defer os.Remove(invalidJsonPath)

	parser := terraform.NewStateParser()
	err := parser.ParseFile(invalidJsonPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON")
}

func TestParseBytes_Success(t *testing.T) {
	dummyStateContent := []byte(`{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {},
		"resources": []
	}`)
	parser := terraform.NewStateParser()
	err := parser.ParseBytes(dummyStateContent)
	require.NoError(t, err)
	assert.NotNil(t, parser.State)
	assert.Equal(t, 4, parser.State.Version)
}

func TestParseBytes_InvalidJSON(t *testing.T) {
	invalidJson := []byte(`{"version": "invalid"}`)
	parser := terraform.NewStateParser()
	err := parser.ParseBytes(invalidJson)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON")
}

func TestGetVersion(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Empty(t, parser.GetVersion()) // No state loaded

	parser.State = &terraform.TerraformState{TerraformVersion: "1.2.3"}
	assert.Equal(t, "1.2.3", parser.GetVersion())
}

func TestGetStateVersion(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Equal(t, 0, parser.GetStateVersion()) // No state loaded

	parser.State = &terraform.TerraformState{Version: 4}
	assert.Equal(t, 4, parser.GetStateVersion())
}

func TestGetResources(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.GetResources()) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{Type: "aws_s3_bucket", Name: "my_bucket"},
		},
	}
	resources := parser.GetResources()
	assert.Len(t, resources, 1)
	assert.Equal(t, "aws_s3_bucket", resources[0].Type)
}

func TestGetResourcesByType(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.GetResourcesByType("any")) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{Type: "aws_s3_bucket", Name: "bucket1", Instances: []terraform.Instance{{SchemaVersion: 1}}},
			{Type: "aws_s3_bucket", Name: "bucket2", Instances: []terraform.Instance{{SchemaVersion: 1}}},
			{Type: "aws_instance", Name: "instance1", Instances: []terraform.Instance{{SchemaVersion: 2}}},
		},
	}

	s3Resources := parser.GetResourcesByType("aws_s3_bucket")
	assert.Len(t, s3Resources, 2)
	assert.Equal(t, "bucket1", s3Resources[0].Name)
	assert.Equal(t, "bucket2", s3Resources[1].Name)
	assert.Equal(t, 1, s3Resources[0].Instances[0].ScheamVersion) // Check schema version mapping

	instanceResources := parser.GetResourcesByType("aws_instance")
	assert.Len(t, instanceResources, 1)
	assert.Equal(t, "instance1", instanceResources[0].Name)
	assert.Equal(t, 2, instanceResources[0].Instances[0].ScheamVersion)

	nonExistentResources := parser.GetResourcesByType("non_existent")
	assert.Empty(t, nonExistentResources)
}

func TestGetResourceByName(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.GetResourceByName("any", "any")) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{Type: "aws_s3_bucket", Name: "my_bucket"},
			{Type: "aws_instance", Name: "my_instance"},
		},
	}

	resource := parser.GetResourceByName("aws_s3_bucket", "my_bucket")
	assert.NotNil(t, resource)
	assert.Equal(t, "my_bucket", resource.Name)

	resource = parser.GetResourceByName("aws_s3_bucket", "non_existent")
	assert.Nil(t, resource)

	resource = parser.GetResourceByName("non_existent_type", "my_bucket")
	assert.Nil(t, resource)
}

func TestGetOutputs(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.GetOutputs()) // No state loaded

	parser.State = &terraform.TerraformState{
		Outputs: map[string]terraform.Output{
			"output1": {Value: "value1"},
			"output2": {Value: 123},
		},
	}
	outputs := parser.GetOutputs()
	assert.Len(t, outputs, 2)
	assert.Equal(t, "value1", outputs["output1"].Value)
}

func TestGetOutput(t *testing.T) {
	parser := terraform.NewStateParser()
	output, exists := parser.GetOutput("any")
	assert.Nil(t, output)
	assert.False(t, exists) // No state loaded

	parser.State = &terraform.TerraformState{
		Outputs: map[string]terraform.Output{
			"my_output": {Value: "test_value", Type: "string"},
		},
	}

	output, exists = parser.GetOutput("my_output")
	assert.NotNil(t, output)
	assert.True(t, exists)
	assert.Equal(t, "test_value", output.Value)

	output, exists = parser.GetOutput("non_existent_output")
	assert.Nil(t, output.Type)
	assert.Nil(t, output.Value)
	assert.False(t, output.Sensitive)
	assert.False(t, exists)
}

func TestGetResourceAttributes(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.GetResourceAttributes("any", "any", 0)) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{
				Type: "aws_s3_bucket",
				Name: "my_bucket",
				Instances: []terraform.Instance{
					{
						Attributes: map[string]any{"bucket_name": "test-bucket", "region": "us-east-1"},
					},
					{
						Attributes: map[string]any{"bucket_name": "another-bucket", "region": "eu-west-1"},
					},
				},
			},
		},
	}

	attrs := parser.GetResourceAttributes("aws_s3_bucket", "my_bucket", 0)
	assert.NotNil(t, attrs)
	assert.Equal(t, "test-bucket", attrs["bucket_name"])
	assert.Equal(t, "us-east-1", attrs["region"])

	attrs = parser.GetResourceAttributes("aws_s3_bucket", "my_bucket", 1)
	assert.NotNil(t, attrs)
	assert.Equal(t, "another-bucket", attrs["bucket_name"])
	assert.Equal(t, "eu-west-1", attrs["region"])

	attrs = parser.GetResourceAttributes("aws_s3_bucket", "non_existent_name", 0)
	assert.Nil(t, attrs)

	attrs = parser.GetResourceAttributes("aws_s3_bucket", "my_bucket", 2) // Index out of bounds
	assert.Nil(t, attrs)
}

func TestListResourceTypes(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Nil(t, parser.ListResourceTypes()) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{Type: "aws_s3_bucket"},
			{Type: "aws_instance"},
			{Type: "aws_s3_bucket"}, // Duplicate
			{Type: "aws_vpc"},
		},
	}

	types := parser.ListResourceTypes()
	assert.Len(t, types, 3)
	assert.Contains(t, types, "aws_s3_bucket")
	assert.Contains(t, types, "aws_instance")
	assert.Contains(t, types, "aws_vpc")
}

func TestGetResourceCount(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Equal(t, 0, parser.GetResourceCount()) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{}, {}, {},
		},
	}
	assert.Equal(t, 3, parser.GetResourceCount())
}

func TestGetResourceInstanceCount(t *testing.T) {
	parser := terraform.NewStateParser()
	assert.Equal(t, 0, parser.GetResourceInstanceCount()) // No state loaded

	parser.State = &terraform.TerraformState{
		Resources: []terraform.Resource{
			{Instances: []terraform.Instance{{}, {}}}, // 2 instances
			{Instances: []terraform.Instance{{}}},     // 1 instance
			{Instances: []terraform.Instance{}},       // 0 instances
		},
	}
	assert.Equal(t, 3, parser.GetResourceInstanceCount())
}
