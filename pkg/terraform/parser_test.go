package terraform_test

import (
	"drift-watcher/pkg/terraform" // Assuming your package is named 'terraform'
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseTerraformFile tests the ParseTerraformFile function
func TestParseTerraformFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := ioutil.TempDir("", "terraform_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the temporary directory

	tfStatePath := "../../assets/terraform_ec2_state.tfstate"

	// Test Case 1: Valid .tfstate file
	t.Run("ValidTFStateFile", func(t *testing.T) {
		parsed, err := terraform.ParseTerraformFile(tfStatePath)
		if err != nil {
			t.Errorf("ParseTerraformFile failed for valid .tfstate: %v", err)
		}

		if parsed.Type != "state" {
			t.Errorf("Expected type 'state', got '%s'", parsed.Type)
		}
		if parsed.FilePath != tfStatePath {
			t.Errorf("Expected FilePath '%s', got '%s'", tfStatePath, parsed.FilePath)
		}
		if parsed.State == nil {
			t.Errorf("Expected State to be non-nil")
		}
		if len(parsed.State.Resources) != 2 || parsed.State.Resources[0].Name != "web_server" {
			t.Errorf("Parsed state resources mismatch")
		}
		if parsed.State.Resources[0].Instances[0].Attributes.ID != "i-08af3a1b1a9500f2d" {
			t.Errorf("Expected instance ID 'i-08af3a1b1a9500f2d', got '%s'", parsed.State.Resources[0].Instances[0].Attributes.ID)
		}
	})

	// Test Case 2: Non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "non_existent.tfstate")
		_, err := terraform.ParseTerraformFile(nonExistentPath)
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
		if !strings.Contains(err.Error(), "Failed to open terraform state file") {
			t.Errorf("Expected 'Failed to open' error, got: %v", err)
		}
	})

	// Test Case 3: Invalid file extension
	t.Run("InvalidFileExtension", func(t *testing.T) {
		invalidFilePath := filepath.Join(tempDir, "invalid.txt")
		err := ioutil.WriteFile(invalidFilePath, []byte("some content"), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid file: %v", err)
		}

		_, err = terraform.ParseTerraformFile(invalidFilePath)
		if err == nil {
			t.Error("Expected error for invalid file extension, got nil")
		}
		expectedErr := "invalid file format. Only .tfstate files are supported at the moment"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})

	// Test Case 4: Malformed .tfstate JSON
	t.Run("MalformedTFStateJSON", func(t *testing.T) {
		malformedContent := `{ "version": 4, "resources": [` // Missing closing brace and array
		malformedPath := filepath.Join(tempDir, "malformed.tfstate")
		err := ioutil.WriteFile(malformedPath, []byte(malformedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write malformed .tfstate file: %v", err)
		}

		_, err = terraform.ParseTerraformFile(malformedPath)
		if err == nil {
			t.Error("Expected error for malformed JSON, got nil")
		}
		if !strings.Contains(err.Error(), "Failed to unmarshal terraform state file") {
			t.Errorf("Expected 'Failed to unmarshal' error, got: %v", err)
		}
	})

	// Test Case 5: Valid .hcl file (if parseHCLFile is uncommented and implemented)
	// This test case is commented out because parseHCLFile is commented out in the original.
	// Uncomment and implement once parseHCLFile and related HCL parsing logic is active.
	/*
		t.Run("ValidHCLFile", func(t *testing.T) {
			hclContent := `
			resource "aws_vpc" "main" {
				cidr_block = "10.0.0.0/16"
			}
			`
			hclPath := filepath.Join(tempDir, "main.tf") // Typical HCL file extension
			err := ioutil.WriteFile(hclPath, []byte(hclContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test .tf file: %v", err)
			}

			parsed, err := terraform.ParseTerraformFile(hclPath)
			if err != nil {
				t.Errorf("ParseTerraformFile failed for valid .tf file: %v", err)
			}
			if parsed.Type != "hcl" {
				t.Errorf("Expected type 'hcl', got '%s'", parsed.Type)
			}
			if parsed.FilePath != hclPath {
				t.Errorf("Expected FilePath '%s', got '%s'", hclPath, parsed.FilePath)
			}
			if len(parsed.HCLConfig.ResourceBlocks) != 1 || parsed.HCLConfig.ResourceBlocks[0].Name != "main" {
				t.Errorf("Parsed HCL resources mismatch")
			}
		})
	*/
}

// TestParseStateFile tests the ParseStateFile function directly
func TestParseStateFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := ioutil.TempDir("", "state_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the temporary directory

	// Test Case 1: Valid .tfstate file
	t.Run("ValidTFStateFile", func(t *testing.T) {
		tfStatePath := "../../assets/terraform_ec2_state.tfstate"
		parsed, err := terraform.ParseStateFile(tfStatePath)
		if err != nil {
			t.Errorf("parseStateFile failed for valid .tfstate: %v", err)
		}
		if parsed.Type != "state" {
			t.Errorf("Expected type 'state', got '%s'", parsed.Type)
		}
		if parsed.FilePath != tfStatePath {
			t.Errorf("Expected FilePath '%s', got '%s'", tfStatePath, parsed.FilePath)
		}
		if parsed.State == nil {
			t.Errorf("Expected State to be non-nil")
		}

		if len(parsed.State.Resources) != 2 || parsed.State.Resources[0].Name != "web_server" {
			t.Errorf("Parsed state resources mismatch: expected 1 resource named 'my_bucket', got %d", len(parsed.State.Resources))
		}
		if parsed.State.Resources[0].Instances[0].Attributes.ID != "i-08af3a1b1a9500f2d" {
			t.Errorf("Parsed resource instance ID mismatch: expected 'my-unique-bucket', got '%s'", parsed.State.Resources[0].Instances[0].Attributes.ID)
		}
	})

	// Test Case 2: Non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "non_existent_state.tfstate")
		_, err := terraform.ParseStateFile(nonExistentPath)
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
		if !strings.Contains(err.Error(), "Failed to open terraform state file") {
			t.Errorf("Expected 'Failed to open' error, got: %v", err)
		}
	})

	// Test Case 3: Malformed JSON content
	t.Run("MalformedJSON", func(t *testing.T) {
		malformedContent := `{ "version": 4, "resources": [` // Incomplete JSON
		malformedPath := filepath.Join(tempDir, "malformed_state.tfstate")
		err := ioutil.WriteFile(malformedPath, []byte(malformedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write malformed test .tfstate file: %v", err)
		}

		_, err = terraform.ParseStateFile(malformedPath)
		if err == nil {
			t.Error("Expected error for malformed JSON, got nil")
		}
		if !strings.Contains(err.Error(), "Failed to unmarshal terraform state file") {
			t.Errorf("Expected 'Failed to unmarshal' error, got: %v", err)
		}
	})

	// Test Case 4: Empty file
	t.Run("EmptyFile", func(t *testing.T) {
		emptyPath := filepath.Join(tempDir, "empty.tfstate")
		err := ioutil.WriteFile(emptyPath, []byte(""), 0644)
		if err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		_, err = terraform.ParseStateFile(emptyPath)
		if err == nil {
			t.Error("Expected error for empty file, got nil")
		}
		if !strings.Contains(err.Error(), "Failed to unmarshal terraform state file") {
			t.Errorf("Expected 'Failed to unmarshal' error for empty file, got: %v", err)
		}
	})

	// Test Case 5: Real-world state file example
	t.Run("RealWorldStateFile", func(t *testing.T) {
		statePath := "../../assets/terraform_ec2_state.tfstate"
		parsed, err := terraform.ParseStateFile(statePath)
		if err != nil {
			t.Errorf("parseStateFile failed for real .tfstate: %v", err)
		}
		if parsed.Type != "state" {
			t.Errorf("Expected type 'state', got '%s'", parsed.Type)
		}
		if parsed.FilePath != statePath {
			t.Errorf("Expected FilePath '%s', got '%s'", statePath, parsed.FilePath)
		}
		if parsed.State == nil {
			t.Errorf("Expected State to be non-nil")
		}

		if len(parsed.State.Resources) != 2 {
			t.Errorf("Expected 2 resources, got %d", len(parsed.State.Resources))
		}

		// Check specific resource details
		instanceResource := parsed.State.Resources[0]
		if instanceResource.Type != "aws_instance" || instanceResource.Name != "web_server" {
			t.Errorf("Expected aws_instance web_server, got %s %s", instanceResource.Type, instanceResource.Name)
		}
		if len(instanceResource.Instances) != 1 {
			t.Errorf("Expected 1 instance for aws_instance, got %d", len(instanceResource.Instances))
		}
		if instanceResource.Instances[0].Attributes.ID != "i-08af3a1b1a9500f2d" {
			t.Errorf("Expected instance ID i-08af3a1b1a9500f2d, got %s", instanceResource.Instances[0].Attributes.ID)
		}
		if !reflect.DeepEqual(instanceResource.Instances[0].Dependencies, []string{"aws_security_group.web_sg", "aws_subnet.public_subnet"}) {
			t.Errorf("Instance dependencies mismatch: expected %v, got %v", []string{"aws_security_group.web_sg", "aws_subnet.public_subnet"}, instanceResource.Dependencies)
		}

		sgResource := parsed.State.Resources[1]
		if sgResource.Type != "aws_security_group" || sgResource.Name != "web_sg" {
			t.Errorf("Expected aws_security_group web_sg, got %s %s", sgResource.Type, sgResource.Name)
		}
	})
}

// TestGetResources tests the GetResources method
func TestGetResources(t *testing.T) {
	// Test Case 1: ParsedTerraform with valid state data
	t.Run("ValidStateData", func(t *testing.T) {
		expectedResources := []terraform.Resource{
			{
				Mode: "managed",
				Type: "aws_s3_bucket",
				Name: "my_bucket",
				Instances: []terraform.Instance{
					{SchemaVersion: 0, Attributes: terraform.InstanceAttributes{ID: "bucket-1"}},
				},
			},
			{
				Mode: "managed",
				Type: "aws_instance",
				Name: "my_server",
				Instances: []terraform.Instance{
					{SchemaVersion: 0, Attributes: terraform.InstanceAttributes{InstanceType: "t2.micro", ID: "server-1"}},
				},
			},
		}
		pt := terraform.ParsedTerraform{
			Type: "state",
			State: &terraform.TerraformState{
				Resources: expectedResources,
			},
		}

		resources, err := pt.GetResources()
		if err != nil {
			t.Errorf("GetResources failed for valid state data: %v", err)
		}
		if !reflect.DeepEqual(resources, expectedResources) {
			t.Errorf("Returned resources mismatch expected resources.\nExpected: %+v\nGot: %+v", expectedResources, resources)
		}
	})

	// Test Case 2: ParsedTerraform with Type "state" but nil State
	t.Run("StateNil", func(t *testing.T) {
		pt := terraform.ParsedTerraform{
			Type:  "state",
			State: nil, // State is nil
		}

		resources, err := pt.GetResources()
		if err != nil {
			t.Errorf("GetResources failed when State is nil: %v", err)
		}
		if len(resources) != 0 {
			t.Errorf("Expected no resources, got %d", len(resources))
		}
	})

	// Test Case 3: ParsedTerraform with Type "hcl" (mocking processExtractedResources)
	// t.Run("HCLType", func(t *testing.T) {
	//	// Mock HCLConfig with some dummy resource blocks
	//	mockHCLConfig := terraform.TerraformConfig{
	//		ResourceBlocks: []terraform.ResourceBlock{ // Use the dummy ResourceBlock
	//			{Type: "aws_vpc", Name: "main"},
	//			{Type: "aws_subnet", Name: "public"},
	//		},
	//	}

	//	pt := terraform.ParsedTerraform{
	//		Type:      "hcl",
	//		HCLConfig: mockHCLConfig,
	//	}

	//	// Temporarily re-assign the unexported processExtractedResources for testing
	//	// This is a common pattern for testing unexported functions or dependencies
	//	// where direct mocking isn't easily achievable without modifying the original source.
	//	originalProcessExtractedResources := func(config *terraform.TerraformConfig) ([]terraform.Resource, error) {
	//		var resources []terraform.Resource
	//		for _, rb := range config.ResourceBlocks {
	//			resources = append(resources, terraform.Resource{
	//				Type: rb.Type,
	//				Name: rb.Name,
	//			})
	//		}
	//		return resources, nil
	//	}

	//	// Save the original GetResources to restore it later if needed (good practice for real projects)
	//	// For this simplified example, we'll just demonstrate the call.
	//	// In a real scenario, you would not typically reassign methods of structs from other packages.
	//	// Instead, you'd design your code so `processExtractedResources` is either exported,
	//	// or part of an interface that can be mocked.
	//	// Since GetResources directly calls an unexported function, this test is limited
	//	// without direct access to the unexported function or refactoring the main code.
	//	// The most accurate test would involve calling parseHCLFile and then GetResources on the result.
	//	// Given the `GetResources` method *directly* calls `processExtractedResources`,
	//	// and that function is internal, we have to simulate its behavior or refactor the original code.
	//	// For the purpose of this test, we are assuming `processExtractedResources` behaves as defined in this test.

	//	expectedResources, _ := originalProcessExtractedResources(&pt.HCLConfig)

	//	// Call the actual GetResources method
	//	resources, err := pt.GetResources() // This will call the actual (unmocked) processExtractedResources in the terraform package
	//	if err != nil {
	//		t.Errorf("GetResources failed for HCL type: %v", err)
	//	}
	//	if !reflect.DeepEqual(resources, expectedResources) {
	//		t.Errorf("Returned HCL resources mismatch expected resources.\nExpected: %+v\nGot: %+v", expectedResources, resources)
	//	}
	//})

	// Test Case 4: Unknown Type
	t.Run("UnknownType", func(t *testing.T) {
		pt := terraform.ParsedTerraform{
			Type: "unknown",
		}

		resources, err := pt.GetResources()
		if err == nil {
			t.Error("Expected error for unknown type, got nil")
		}
		expectedErr := "no valid data to extract resources from"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
		}
		if resources != nil {
			t.Errorf("Expected nil resources for unknown type, got %v", resources)
		}
	})
}

// TestParseHCLFile tests the parseHCLFile function
// This test is provided as a template. It requires a working
// extractTerraformConfigFromHCL and proper HCL content parsing.
// It is commented out because parseHCLFile is currently unexported
// and its dependencies (extractTerraformConfigFromHCL) are missing.
/*
func TestParseHCLFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := ioutil.TempDir("", "hcl_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the temporary directory

 // Test Case 1: Valid .tf file
	t.Run("ValidHCLFile", func(t *testing.T) {
		hclContent := `
			resource "aws_vpc" "main" {
				cidr_block = "10.0.0.0/16"
				tags = {
					Name = "my-vpc"
				}
			}
			resource "aws_subnet" "public" {
				vpc_id     = aws_vpc.main.id
				cidr_block = "10.0.1.0/24"
			}
		`
		hclPath := filepath.Join(tempDir, "main.tf")
		err := ioutil.WriteFile(hclPath, []byte(hclContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test .tf file: %v", err)
		}

		// Since parseHCLFile is unexported, you would need to either:
		// 1. Export parseHCLFile to test it directly.
		// 2. Test it indirectly via ParseTerraformFile (which is what TestParseTerraformFile's HCL case does).
		// For this template, we'll assume it's exported for direct testing.
		// parsed, err := terraform.ParseHCLFile(hclPath) // This line would be active if exported

		// For demonstration, manually create a ParsedTerraform with expected HCLConfig
		// This simulates the outcome of a successful parseHCLFile.
		expectedConfig := terraform.TerraformConfig{
			ResourceBlocks: []terraform.ResourceBlock{
				{
					Type: "aws_vpc",
					Name: "main",
					Arguments: map[string]interface{}{
						"cidr_block": "10.0.0.0/16",
						"tags": map[string]interface{}{
							"Name": "my-vpc",
						},
					},
				},
				{
					Type: "aws_subnet",
					Name: "public",
					Arguments: map[string]interface{}{
						"vpc_id":     "aws_vpc.main.id", // This might be an expression, not a direct string in real HCL parsing
						"cidr_block": "10.0.1.0/24",
					},
				},
			},
		}

		// Simulate the ParsedTerraform structure that parseHCLFile would return
		parsed := terraform.ParsedTerraform{
			Type:      "hcl",
			FilePath:  hclPath,
			HCLConfig: expectedConfig,
		}

		// Perform assertions as if parseHCLFile returned 'parsed'
		if parsed.Type != "hcl" {
			t.Errorf("Expected type 'hcl', got '%s'", parsed.Type)
		}
		if parsed.FilePath != hclPath {
			t.Errorf("Expected FilePath '%s', got '%s'", hclPath, parsed.FilePath)
		}
		if len(parsed.HCLConfig.ResourceBlocks) != 2 {
			t.Errorf("Expected 2 resource blocks, got %d", len(parsed.HCLConfig.ResourceBlocks))
		}
		// More detailed assertions on resource blocks, types, names, and arguments
		// require your actual extractTerraformConfigFromHCL implementation to be known.
		// For now, basic checks:
		if parsed.HCLConfig.ResourceBlocks[0].Type != "aws_vpc" || parsed.HCLConfig.ResourceBlocks[0].Name != "main" {
			t.Errorf("Expected first resource to be aws_vpc.main")
		}
		if parsed.HCLConfig.ResourceBlocks[1].Type != "aws_subnet" || parsed.HCLConfig.ResourceBlocks[1].Name != "public" {
			t.Errorf("Expected second resource to be aws_subnet.public")
		}
	})

	// Test Case 2: Malformed .tf file (syntax error)
	t.Run("MalformedHCLFile", func(t *testing.T) {
		malformedContent := `resource "aws_vpc" "main" { cidr_block = "10.0.0.0/16"` // Missing closing brace
		malformedPath := filepath.Join(tempDir, "malformed.tf")
		err := ioutil.WriteFile(malformedPath, []byte(malformedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write malformed .tf file: %v", err)
		}

		// Assuming parseHCLFile is exported for direct testing
		// _, err = terraform.ParseHCLFile(malformedPath)
		// if err == nil {
		// 	t.Error("Expected error for malformed HCL, got nil")
		// }
		// if !strings.Contains(err.Error(), "Failed to parse terraform hcl file") { // Or more specific error from hclparse
		// 	t.Errorf("Expected 'Failed to parse' error, got: %v", err)
		// }
	})

	// Test Case 3: Non-existent .tf file
	t.Run("NonExistentHCLFile", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "non_existent.tf")
		// Assuming parseHCLFile is exported for direct testing
		// _, err := terraform.ParseHCLFile(nonExistentPath)
		// if err == nil {
		// 	t.Error("Expected error for non-existent HCL file, got nil")
		// }
		// if !strings.Contains(err.Error(), "Failed to parse terraform hcl file") { // Or more specific error from hclparse
		// 	t.Errorf("Expected 'Failed to parse' error, got: %v", err)
		// }
	})
}
*/
