package statemanager_test

import (
	"context"
	"drift-watcher/pkg/services/statemanager"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementation of StateManagerI for testing purposes if needed
type MockStateManager struct {
	ParseStateFileFunc    func(ctx context.Context, statePath string) (statemanager.StateContent, error)
	RetrieveResourcesFunc func(ctx context.Context, content statemanager.StateContent, resourceType string) ([]statemanager.StateResource, error)
}

func (m *MockStateManager) ParseStateFile(ctx context.Context, statePath string) (statemanager.StateContent, error) {
	if m.ParseStateFileFunc != nil {
		return m.ParseStateFileFunc(ctx, statePath)
	}
	return statemanager.StateContent{}, fmt.Errorf("ParseStateFile not implemented")
}

func (m *MockStateManager) RetrieveResources(ctx context.Context, content statemanager.StateContent, resourceType string) ([]statemanager.StateResource, error) {
	if m.RetrieveResourcesFunc != nil {
		return m.RetrieveResourcesFunc(ctx, content, resourceType)
	}
	return nil, fmt.Errorf("RetrieveResources not implemented")
}

func TestStateResource_ResourceType(t *testing.T) {
	s := statemanager.StateResource{Type: "aws_s3_bucket"}
	assert.Equal(t, "aws_s3_bucket", s.ResourceType())

	s2 := statemanager.StateResource{Type: "aws_instance"}
	assert.Equal(t, "aws_instance", s2.ResourceType())

	s3 := statemanager.StateResource{}
	assert.Empty(t, s3.ResourceType())
}

func TestStateResource_AttributeValue_Success(t *testing.T) {
	s := statemanager.StateResource{
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_name": "my-test-bucket",
					"region":      "us-east-1",
					"enabled":     "true", // String representation
				},
			},
		},
	}

	// Test existing string attribute
	val, err := s.AttributeValue("bucket_name")
	require.NoError(t, err)
	assert.Equal(t, "my-test-bucket", val)

	val, err = s.AttributeValue("region")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", val)

	val, err = s.AttributeValue("enabled")
	require.NoError(t, err)
	assert.Equal(t, "true", val)
}

func TestStateResource_AttributeValue_AttributeDoesNotExist(t *testing.T) {
	s := statemanager.StateResource{
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_name": "my-test-bucket",
				},
			},
		},
	}

	// Test non-existent attribute
	val, err := s.AttributeValue("non_existent_attribute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attribute does not exist")
	assert.Empty(t, val)
}

func TestStateResource_AttributeValue_AttributeNotString(t *testing.T) {
	s := statemanager.StateResource{
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"count": 123, // Integer attribute
				},
			},
		},
	}

	// Test attribute that is not a string
	val, err := s.AttributeValue("count")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attribute value cannot be parsed to string")
	assert.Empty(t, val)
}

func TestStateResource_AttributeValue_NoInstances(t *testing.T) {
	s := statemanager.StateResource{
		Instances: []statemanager.ResourceInstance{}, // No instances
	}

	// Test with no instances in the slice
	val, err := s.AttributeValue("any_attribute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No Instance for resource")
	assert.Empty(t, val)
}

func TestStateResource_AttributeValue_NilAttributesMap(t *testing.T) {
	s := statemanager.StateResource{
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: nil, // Nil attributes map
			},
		},
	}

	// Test with nil attributes map
	val, err := s.AttributeValue("any_attribute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attribute does not exist")
	assert.Empty(t, val)
}

func TestStateContent_JSONMarshalling(t *testing.T) {
	sc := statemanager.StateContent{
		StateVersion:  "1.0",
		Tool:          statemanager.TerraformTool,
		ToolVersion:   "1.2.3",
		ToolMetadata:  map[string]any{"serial": 123},
		SchemaVersion: "1",
		StateId:       "test-id",
		Resource: []statemanager.StateResource{
			{
				Mode:     "managed",
				Type:     "aws_s3_bucket",
				Name:     "my_bucket",
				Provider: statemanager.ProviderType("aws"),
				Instances: []statemanager.ResourceInstance{
					{
						ScheamVersion: 0,
						Attributes:    map[string]any{"bucket": "test-bucket"},
						Dependencies:  []string{"dep1"},
					},
				},
				ToolData: map[string]any{"each_mode": "map"},
			},
		},
		RawState: json.RawMessage(`{"original_key":"original_value"}`),
		BackendConfig: statemanager.BackendConfig{
			Type: "s3",
			Config: statemanager.ConfigDetails{
				Bucket: "my-s3-bucket",
				Region: "us-east-1",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(sc)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Unmarshal back to verify
	var unmarshaledSc statemanager.StateContent
	err = json.Unmarshal(data, &unmarshaledSc)
	require.NoError(t, err)

	assert.Equal(t, sc.StateVersion, unmarshaledSc.StateVersion)
	assert.Equal(t, sc.Tool, unmarshaledSc.Tool)
	assert.Equal(t, sc.ToolVersion, unmarshaledSc.ToolVersion)
	assert.Equal(t, sc.SchemaVersion, unmarshaledSc.SchemaVersion)
	assert.Equal(t, sc.StateId, unmarshaledSc.StateId)
	assert.Len(t, unmarshaledSc.Resource, 1)
	assert.Equal(t, sc.Resource[0].Name, unmarshaledSc.Resource[0].Name)
	assert.Equal(t, string(sc.RawState), string(unmarshaledSc.RawState))
	assert.Equal(t, sc.BackendConfig.Type, unmarshaledSc.BackendConfig.Type)
	assert.Equal(t, sc.BackendConfig.Config.Bucket, unmarshaledSc.BackendConfig.Config.Bucket)
}

func TestBackendConfig_JSONMarshalling(t *testing.T) {
	bc := statemanager.BackendConfig{
		Type: "azurerm",
		Config: statemanager.ConfigDetails{
			Key:    "tfstate",
			Bucket: "my-azure-container",
		},
	}

	data, err := json.Marshal(bc)
	require.NoError(t, err)

	var unmarshaledBc statemanager.BackendConfig
	err = json.Unmarshal(data, &unmarshaledBc)
	require.NoError(t, err)

	assert.Equal(t, bc.Type, unmarshaledBc.Type)
	assert.Equal(t, bc.Config.Key, unmarshaledBc.Config.Key)
	assert.Equal(t, bc.Config.Bucket, unmarshaledBc.Config.Bucket)
}

func TestConfigDetails_JSONMarshalling(t *testing.T) {
	cd := statemanager.ConfigDetails{
		Path:          "/tmp/state.tfstate",
		Bucket:        "my-bucket",
		Region:        "us-west-2",
		Encrypt:       "true",
		Key:           "path/to/key",
		DynamoDBTable: "my-ddb-table",
	}

	data, err := json.Marshal(cd)
	require.NoError(t, err)

	var unmarshaledCd statemanager.ConfigDetails
	err = json.Unmarshal(data, &unmarshaledCd)
	require.NoError(t, err)

	assert.Equal(t, cd.Path, unmarshaledCd.Path)
	assert.Equal(t, cd.Bucket, unmarshaledCd.Bucket)
	assert.Equal(t, cd.Region, unmarshaledCd.Region)
	assert.Equal(t, cd.Encrypt, unmarshaledCd.Encrypt)
	assert.Equal(t, cd.Key, unmarshaledCd.Key)
	assert.Equal(t, cd.DynamoDBTable, unmarshaledCd.DynamoDBTable)
}

func TestResourceInstance_JSONMarshalling(t *testing.T) {
	ri := statemanager.ResourceInstance{
		ScheamVersion: 1,
		Attributes: map[string]any{
			"id":   "resource-id-123",
			"name": "test-resource",
			"tags": []string{"env:dev", "project:xyz"},
		},
		Dependencies: []string{"module.vpc.aws_vpc.main"},
	}

	data, err := json.Marshal(ri)
	require.NoError(t, err)

	var unmarshaledRi statemanager.ResourceInstance
	err = json.Unmarshal(data, &unmarshaledRi)
	require.NoError(t, err)

	assert.Equal(t, ri.ScheamVersion, unmarshaledRi.ScheamVersion)
	assert.Equal(t, ri.Attributes["id"], unmarshaledRi.Attributes["id"])
	assert.Equal(t, ri.Attributes["name"], unmarshaledRi.Attributes["name"])
	// Note: JSON unmarshaling of interface{} can result in float64 for numbers
	// For slices, direct comparison works if elements are comparable.
	assert.ElementsMatch(t, ri.Attributes["tags"].([]string), unmarshaledRi.Attributes["tags"].([]any))
	assert.ElementsMatch(t, ri.Dependencies, unmarshaledRi.Dependencies)
}
