package cmd

import (
	"bytes"
	"context"
	"drift-watcher/cmd/mocks" // Import the mocks package
	"drift-watcher/config"
	"drift-watcher/pkg/provider"
	"drift-watcher/pkg/provider/aws"
	"drift-watcher/pkg/terraform"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Helper to capture slog output
func captureSlogOutput() (*bytes.Buffer, func()) {
	var buf bytes.Buffer
	// Set the default handler to write to our buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf, func() {
		// Restore the original handler after the test
		slog.SetDefault(slog.New(originalHandler))
	}
}

// Helper to create a dummy Cobra command for testing purposes
func newTestCobraCommand() *cobra.Command {
	return &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}

// Helper to create a temporary HCL file for testing
func createTempHCLFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.tf")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp HCL file: %v", err)
	}
	return filePath
}

// Helper to create a temporary state file for testing
func createTempStateFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "terraform.tfstate")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}
	return filePath
}

func TestDetectCmd_Run_MissingConfigFile(t *testing.T) {
	// Capture slog output
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	// Capture os.Exit
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected os.Exit(1), got %d", code)
		}
		panic("os.Exit called") // Panics to stop test execution but allows recovery in deferred func
	}

	cfg := &config.Config{} // Dummy config
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand() // Use a dummy cobra command

	// Simulate no configfile flag set
	dc.tfConfigPath = ""

	// Run the command, expecting a panic from os.Exit
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected os.Exit to be called, but it wasn't")
		}
	}()

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}
	if !exitCalled {
		t.Error("Expected os.Exit to be called, but it wasn't")
	}
	if !strings.Contains(slogOutput.String(), "Invalid configuration file path provided") {
		t.Errorf("Expected 'Invalid configuration file path provided' log, got: %s", slogOutput.String())
	}
}

func TestDetectCmd_Run_ConfigFileDoesNotExist(t *testing.T) {
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected os.Exit(1), got %d", code)
		}
		panic("os.Exit called")
	}

	cfg := &config.Config{}
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand()

	// Point to a non-existent file
	dc.tfConfigPath = "/path/to/non/existent/file.tf"

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected os.Exit to be called, but it wasn't")
		}
	}()

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}
	if !exitCalled {
		t.Error("Expected os.Exit to be called, but it wasn't")
	}
	if !strings.Contains(slogOutput.String(), "file /path/to/non/existent/file.tf does not exist") {
		t.Errorf("Expected 'file does not exist' log, got: %s", slogOutput.String())
	}
}

func TestDetectCmd_Run_InvalidTerraformFile(t *testing.T) {
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected os.Exit(1), got %d", code)
		}
		panic("os.Exit called")
	}

	cfg := &config.Config{}
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand()

	// Create a temporary file with invalid HCL
	invalidHCL := `resource "aws_instance" "test" { instance_type = ` // Missing closing quote
	filePath := createTempHCLFile(t, invalidHCL)
	dc.tfConfigPath = filePath

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected os.Exit to be called, but it wasn't")
		}
	}()

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}
	if !exitCalled {
		t.Error("Expected os.Exit to be called, but it wasn't")
	}
	if !strings.Contains(slogOutput.String(), "Failed to parse terraform configuratio file") {
		t.Errorf("Expected 'Failed to parse terraform configuratio file' log, got: %s", slogOutput.String())
	}
}

func TestDetectCmd_Run_UnsupportedProvider(t *testing.T) {
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected os.Exit(1), got %d", code)
		}
		panic("os.Exit called")
	}

	cfg := &config.Config{}
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand()

	filePath := createTempHCLFile(t, `resource "aws_instance" "test" { instance_type = "t2.micro" }`)
	dc.tfConfigPath = filePath
	dc.Provider = "unsupported" // Set an unsupported provider

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected os.Exit to be called, but it wasn't")
		}
	}()

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}
	if !exitCalled {
		t.Error("Expected os.Exit to be called, but it wasn't")
	}
	if !strings.Contains(slogOutput.String(), "unsupported provider is not supported") {
		t.Errorf("Expected 'unsupported provider is not supported' log, got: %s", slogOutput.String())
	}
}

func TestDetectCmd_Run_HCL_Success_ConsoleOutput(t *testing.T) {
	// Capture slog output and fmt.Println output
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	cfg := &config.Config{}
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand()

	hclContent := `
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  tags = {
    Name = "test-instance"
  }
}
`
	filePath := createTempHCLFile(t, hclContent)
	dc.tfConfigPath = filePath
	dc.Provider = "aws"
	dc.Resource = "ec2"
	dc.AttributesToTrack = []string{"instance_type"}
	dc.OutputPath = "" // Ensure console output

	// Mock the AWS provider methods
	mockProvider := &mocks.MockProvider{}
	mockProvider.InfrastructreMetadataFunc = func(ctx context.Context, resourceType string, filter map[string]string) (map[string]interface{}, error) {
		// Simulate EC2 metadata
		if filter["tag:Name"] == "test-instance" {
			return map[string]interface{}{"InstanceType": "t2.large"}, nil
		}
		return nil, nil
	}
	mockProvider.CompareActiveAndDesiredStateFunc = func(ctx context.Context, resourceType string, infrastructureData map[string]interface{}, resource terraform.Resource, attributesToTrack []string) (map[string]interface{}, error) {
		// Simulate a drift report
		return map[string]interface{}{
			"resource_name":  "test-instance",
			"desired_state":  map[string]interface{}{"instance_type": "t2.micro"},
			"active_state":   map[string]interface{}{"instance_type": "t2.large"},
			"drift_detected": true,
			"differences": []map[string]interface{}{
				{"attribute": "instance_type", "desired": "t2.micro", "active": "t2.large"},
			},
		}, nil
	}

	// Override the NewAWSProvider to return our mock
	originalNewAWSProvider := newAWSProviderFunc // Store original
	newAWSProviderFunc = func(cfg *config.Config) (provider.ProviderI, error) {
		return mockProvider, nil
	}
	defer func() { newAWSProviderFunc = originalNewAWSProvider }() // Restore original

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Close the writer and read the stdout output
	w.Close()
	stdoutOutput, _ := os.ReadFile("/dev/fd/" + strings.Split(r.Name(), "/")[len(strings.Split(r.Name(), "/"))-1]) // More portable way
	r.Close()

	// Parse the captured JSON output
	var outputReport map[string]interface{}
	err = json.Unmarshal(stdoutOutput, &outputReport)
	if err != nil {
		t.Fatalf("Failed to unmarshal stdout JSON: %v", err)
	}

	expectedReport := map[string]interface{}{
		"resource_name":  "test-instance",
		"desired_state":  map[string]interface{}{"instance_type": "t2.micro"},
		"active_state":   map[string]interface{}{"instance_type": "t2.large"},
		"drift_detected": true,
		"differences": []interface{}{
			map[string]interface{}{"attribute": "instance_type", "desired": "t2.micro", "active": "t2.large"},
		},
	}

	// Compare the unmarshaled output with the expected report
	// Using reflect.DeepEqual for map comparison, handle float64 nuances if any
	// JSON unmarshals numbers to float64 by default.
	if !reflect.DeepEqual(outputReport, expectedReport) {
		t.Errorf("Mismatch in console output.\nExpected: %s\nGot: %s",
			prettyPrintMap(t, expectedReport), prettyPrintMap(t, outputReport))
	}

	if slogOutput.Len() > 0 {
		t.Errorf("Expected no slog errors, but got: %s", slogOutput.String())
	}
}

func TestDetectCmd_Run_HCL_Success_FileOutput(t *testing.T) {
	slogOutput, restoreSlog := captureSlogOutput()
	defer restoreSlog()

	cfg := &config.Config{}
	dc := newDetectCmd(cfg)
	dc.cmd = newTestCobraCommand()

	hclContent := `
resource "aws_instance" "web" {
  instance_type = "t2.small"
  tags = {
    Name = "another-instance"
  }
}
`
	filePath := createTempHCLFile(t, hclContent)
	dc.tfConfigPath = filePath
	dc.Provider = "aws"
	dc.Resource = "ec2"
	dc.AttributesToTrack = []string{"instance_type"}

	// Create a temporary file for output
	outputFilePath := filepath.Join(t.TempDir(), "output.json")
	dc.OutputPath = outputFilePath // Ensure file output

	// Mock the AWS provider methods
	mockProvider := &mocks.MockProvider{}
	mockProvider.InfrastructreMetadataFunc = func(ctx context.Context, resourceType string, filter map[string]string) (map[string]interface{}, error) {
		if filter["tag:Name"] == "another-instance" {
			return map[string]interface{}{"InstanceType": "t2.medium"}, nil
		}
		return nil, nil
	}
	mockProvider.CompareActiveAndDesiredStateFunc = func(ctx context.Context, resourceType string, infrastructureData map[string]interface{}, resource terraform.Resource, attributesToTrack []string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_name":  "another-instance",
			"desired_state":  map[string]interface{}{"instance_type": "t2.small"},
			"active_state":   map[string]interface{}{"instance_type": "t2.medium"},
			"drift_detected": true,
			"differences": []map[string]interface{}{
				{"attribute": "instance_type", "desired": "t2.small", "active": "t2.medium"},
			},
		}, nil
	}

	originalNewAWSProvider := newAWSProviderFunc // Store original
	newAWSProviderFunc = func(cfg *config.Config) (provider.ProviderI, error) {
		return mockProvider, nil
	}
	defer func() { newAWSProviderFunc = originalNewAWSProvider }()

	err := dc.Run(dc.cmd, []string{})
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Read the content of the output file
	fileContent, err := os.ReadFile(outputFilePath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var outputFileReport map[string]interface{}
	err = json.Unmarshal(fileContent, &outputFileReport)
	if err != nil {
		t.Fatalf("Failed to unmarshal output file JSON: %v", err)
	}

	expectedReport := map[string]interface{}{
		"resource_name":  "another-instance",
		"desired_state":  map[string]interface{}{"instance_type": "t2.small"},
		"active_state":   map[string]interface{}{"instance_type": "t2.medium"},
		"drift_detected": true,
		"differences": []interface{}{
			map[string]interface{}{"attribute": "instance_type", "desired": "t2.small", "active": "t2.medium"},
		},
	}

	if !reflect.DeepEqual(outputFileReport, expectedReport) {
		t.Errorf("Mismatch in file output.\nExpected: %s\nGot: %s",
			prettyPrintMap(t, expectedReport), prettyPrintMap(t, outputFileReport))
	}

	if slogOutput.Len() > 0 {
		t.Errorf("Expected no slog errors, but got: %s", slogOutput.String())
	}
}

// Helper function to pretty print a map for error messages
func prettyPrintMap(t *testing.T, data map[string]interface{}) string {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal map for printing: %v", err)
	}
	return string(bytes)
}

// --- Test setup for os.Exit mocking ---
// A variable to hold the os.Exit function, so we can override it in tests.
var osExit = os.Exit

// Override os.Stat and os.Create for specific test cases if needed
// For these tests, we're relying on t.TempDir() and os.WriteFile/ReadFile
// which directly interact with the filesystem, making the tests more
// integration-like for file operations, but still unit-like for logic.

// --- Mocks for AWS Provider Instantiation ---
// To mock the actual creation of an AWS provider, we need to
// create a variable that holds the constructor function and
// then replace it in tests.
var newAWSProviderFunc = aws.NewAWSProvider

func init() {
	// Override CheckAWSConfig to avoid actual AWS config checks during tests
	// This function is called in newDetectCmd
	originalCheckAWSConfig := CheckAWSConfig
	CheckAWSConfig = func() (config.AWSConfig, error) {
		return config.AWSConfig{ProfileName: "mock_profile"}, nil
	}
	// Restore it after tests if this file is part of a larger test suite
	// For standalone tests, it's fine to leave it overridden.
	// However, for robust testing, it's good practice to restore.
	// In a `TestMain` function, you could do:
	/*
	   func TestMain(m *testing.M) {
	       // Setup global mocks
	       originalCheckAWSConfig := CheckAWSConfig
	       CheckAWSConfig = func() (config.AWSConfig, error) {
	           return config.AWSConfig{ProfileName: "mock_profile"}, nil
	       }
	       code := m.Run()
	       // Teardown global mocks
	       CheckAWSConfig = originalCheckAWSConfig
	       os.Exit(code)
	   }
	*/
	// For simplicity here, we'll just leave it overridden for this file's tests.
	// If other tests depend on the real CheckAWSConfig, this would need more careful handling.
}
