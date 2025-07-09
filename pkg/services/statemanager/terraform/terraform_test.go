package terraform_test

import (
	"context"
	"drift-watcher/pkg/services/statemanager"
	"drift-watcher/pkg/services/statemanager/terraform"
	"encoding/json"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a dummy tfstate file for testing
func createDummyTFStateFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.tfstate")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	return tmpFile.Name()
}

func TestNewTerraformManager(t *testing.T) {
	manager := terraform.NewTerraformManager()
	assert.NotNil(t, manager)
	// assert.NotNil(t, manager.parser)
}

func TestParseStateFile_Success(t *testing.T) {
	// Create a dummy tfstate file
	dummyStateContent := `{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {
			"test_output": {
				"value": "hello",
				"type": "string",
				"sensitive": false
			}
		},
		"resources": [
			{
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "my_bucket",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {
							"bucket": "my-test-bucket",
							"acl": "private"
						},
						"dependencies": []
					}
				]
			},
			{
				"mode": "managed",
				"type": "null_resource",
				"name": "each_resource",
				"each": "map",
				"provider": "provider[\"registry.terraform.io/hashicorp/null\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {
							"id": "instance1"
						},
						"dependencies": []
					}
				]
			}
		]
	}`
	stateFilePath := createDummyTFStateFile(t, dummyStateContent)
	defer os.Remove(stateFilePath)

	manager := terraform.NewTerraformManager()
	ctx := context.Background()

	stateContent, err := manager.ParseStateFile(ctx, stateFilePath)
	require.NoError(t, err)
	assert.NotNil(t, stateContent)

	assert.Equal(t, "4", stateContent.StateVersion)
	assert.Equal(t, statemanager.TerraformTool, stateContent.Tool)
	assert.Equal(t, "1.0.0", stateContent.ToolVersion)
	assert.Equal(t, "some-lineage", stateContent.StateId)
	assert.Equal(t, "4", stateContent.SchemaVersion)
	assert.Equal(t, int(1), stateContent.ToolMetadata["serial"])

	assert.Len(t, stateContent.Resource, 2)

	// Check the first resource
	res1 := stateContent.Resource[0]
	assert.Equal(t, "managed", res1.Mode)
	assert.Equal(t, "aws_s3_bucket", res1.Type)
	assert.Equal(t, "my_bucket", res1.Name)
	assert.Equal(t, statemanager.ProviderType("provider[\"registry.terraform.io/hashicorp/aws\"]"), res1.Provider)
	assert.Len(t, res1.Instances, 1)
	assert.Equal(t, 0, res1.Instances[0].ScheamVersion)
	assert.Equal(t, "my-test-bucket", res1.Instances[0].Attributes["bucket"])
	assert.Empty(t, res1.ToolData)

	// Check the second resource with EachMode
	res2 := stateContent.Resource[1]
	assert.Equal(t, "map", res2.ToolData["each_mode"])

	// Check RawState
	var rawState terraform.TerraformState
	err = json.Unmarshal(stateContent.RawState, &rawState)
	require.NoError(t, err)
	assert.Equal(t, 4, rawState.Version)
	assert.Equal(t, "1.0.0", rawState.TerraformVersion)
}

func TestParseStateFile_NotExist(t *testing.T) {
	manager := terraform.NewTerraformManager()
	ctx := context.Background()
	_, err := manager.ParseStateFile(ctx, "/path/to/nonexistent/file.tfstate")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
	assert.Contains(t, err.Error(), "state file does not exist")
}

func TestParseStateFile_InvalidFile(t *testing.T) {
	// Create a dummy file with invalid JSON
	invalidFilePath := createDummyTFStateFile(t, `{"version": "invalid"}`) // Malformed JSON
	defer os.Remove(invalidFilePath)

	manager := terraform.NewTerraformManager()
	ctx := context.Background()
	_, err := manager.ParseStateFile(ctx, invalidFilePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON")
}

func TestParseStateFile_DirectoryPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := terraform.NewTerraformManager()
	ctx := context.Background()
	_, err = manager.ParseStateFile(ctx, tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Terraform state directories are not currently supported")
}

func TestConvertTerraformStateToStateContent_MarshalError(t *testing.T) {
	// Create a TerraformState that will cause json.Marshal to fail (e.g., a channel)
	type BadStruct struct {
		Ch chan int
	}
	tfState := terraform.TerraformState{
		Version: 1,
		Outputs: map[string]terraform.Output{"bad": {Value: make(chan int)}}, // Channel cannot be marshaled
	}

	_, err := terraform.ConvertTerraformStateToStateContent(tfState)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal raw state")
}

func TestRetrieveResources_Success(t *testing.T) {
	// Setup a dummy state file with resources
	dummyStateContent := `{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {},
		"resources": [
			{
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "my_bucket_1",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {"bucket": "bucket1"}
					}
				]
			},
			{
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "my_bucket_2",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {"bucket": "bucket2"}
					}
				]
			},
			{
				"mode": "managed",
				"type": "aws_instance",
				"name": "my_instance",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {"instance_type": "t2.micro"}
					}
				]
			}
		]
	}`
	stateFilePath := createDummyTFStateFile(t, dummyStateContent)
	defer os.Remove(stateFilePath)

	manager := terraform.NewTerraformManager()
	ctx := context.Background()

	// Parse the state file first to populate the parser's state
	_, err := manager.ParseStateFile(ctx, stateFilePath)
	require.NoError(t, err)

	// Retrieve resources of type "aws_s3_bucket"
	s3Resources, err := manager.RetrieveResources(ctx, statemanager.StateContent{}, "aws_s3_bucket")
	require.NoError(t, err)
	assert.Len(t, s3Resources, 2)
	assert.Equal(t, "my_bucket_1", s3Resources[0].Name)
	assert.Equal(t, "my_bucket_2", s3Resources[1].Name)

	// Retrieve resources of type "aws_instance"
	instanceResources, err := manager.RetrieveResources(ctx, statemanager.StateContent{}, "aws_instance")
	require.NoError(t, err)
	assert.Len(t, instanceResources, 1)
	assert.Equal(t, "my_instance", instanceResources[0].Name)

	// Retrieve resources of a non-existent type
	nonExistentResources, err := manager.RetrieveResources(ctx, statemanager.StateContent{}, "non_existent_type")
	require.NoError(t, err)
	assert.Empty(t, nonExistentResources)
}

func TestRetrieveResources_NilParser(t *testing.T) {
	manager := &terraform.TerraformStateManager{} // Initialize with a nil parser
	ctx := context.Background()
	_, err := manager.RetrieveResources(ctx, statemanager.StateContent{}, "any_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "") // Expecting an empty string error from the function
}

func TestRetrieveResources_EmptyState(t *testing.T) {
	// Create an empty state file
	emptyStateContent := `{
		"version": 4,
		"terraform_version": "1.0.0",
		"serial": 1,
		"lineage": "some-lineage",
		"outputs": {},
		"resources": []
	}`
	stateFilePath := createDummyTFStateFile(t, emptyStateContent)
	defer os.Remove(stateFilePath)

	manager := terraform.NewTerraformManager()
	ctx := context.Background()

	_, err := manager.ParseStateFile(ctx, stateFilePath)
	require.NoError(t, err)

	resources, err := manager.RetrieveResources(ctx, statemanager.StateContent{}, "any_type")
	require.NoError(t, err)
	assert.Empty(t, resources)
}
