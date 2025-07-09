package reporter_test

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/reporter"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a dummy DriftReport for testing
func createDummyDriftReportForFile(hasDrift bool) *driftchecker.DriftReport {
	report := &driftchecker.DriftReport{
		GeneratedAt:  time.Date(2023, time.February, 20, 14, 30, 0, 0, time.UTC),
		ResourceId:   "file-res-456",
		ResourceType: "aws_ec2_instance",
		ResourceName: "my-ec2-instance",
		HasDrift:     hasDrift,
		Status:       "MATCH",
	}

	if hasDrift {
		report.Status = "DRIFT"
		report.DriftDetails = []driftchecker.DriftItem{
			{
				Field:          "instance_type",
				TerraformValue: "t2.micro",
				ActualValue:    "t2.medium",
				DriftType:      driftchecker.AttributeValueChanged,
			},
			{
				Field:          "security_groups",
				TerraformValue: []string{"sg-123"},
				ActualValue:    []string{"sg-123", "sg-456"},
				DriftType:      driftchecker.AttributeValueChanged,
			},
		}
	}
	return report
}

func TestNewFileReporter(t *testing.T) {
	fileReporter := reporter.NewFileReporter("report.json")
	assert.NotNil(t, fileReporter)
	assert.Equal(t, "report.json", fileReporter.OutputFile)
}

func TestFileReporter_WriteReport_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.json")
	require.NoError(t, err)
	tmpFile.Close() // Close immediately as WriteReport will open/create it
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewFileReporter(tmpFile.Name())
	report := createDummyDriftReportForFile(true)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	// Read the content and verify
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.True(t, len(data) > 0)

	var writtenReport driftchecker.DriftReport
	err = json.Unmarshal(data, &writtenReport)
	require.NoError(t, err)

	assert.Equal(t, report.ResourceId, writtenReport.ResourceId)
	assert.Equal(t, report.HasDrift, writtenReport.HasDrift)
	assert.Len(t, writtenReport.DriftDetails, 2)
	assert.Equal(t, "instance_type", writtenReport.DriftDetails[0].Field)
}

func TestFileReporter_WriteReport_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	nonWritableFile := filepath.Join(tmpDir, "non_writable.json")

	// Create the file first, then make it non-writable
	_, err := os.Create(nonWritableFile)
	require.NoError(t, err)
	err = os.Chmod(nonWritableFile, 0000) // No permissions
	require.NoError(t, err)
	defer os.Chmod(nonWritableFile, 0644) // Restore permissions for cleanup
	defer os.Remove(nonWritableFile)

	reporter := reporter.NewFileReporter(nonWritableFile)
	report := createDummyDriftReportForFile(false)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write drift report to file")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestFileReporter_WriteReport_MarshalError(t *testing.T) {
	// To simulate a marshal error, we need a struct that json.Marshal cannot handle.
	// A channel is a good example.
	type Unmarshalable struct {
		Ch chan int
	}

	reporter := reporter.NewFileReporter("dummy.json")
	report := &driftchecker.DriftReport{
		GeneratedAt:  time.Now(),
		ResourceId:   "bad-report",
		ResourceType: "bad-type",
		ResourceName: "bad-name",
		HasDrift:     true,
		Status:       "ERROR",
		// Inject a value that cannot be marshaled
		DriftDetails: []driftchecker.DriftItem{
			{
				Field:          "problem_field",
				TerraformValue: Unmarshalable{Ch: make(chan int)}, // This will cause marshal error
				ActualValue:    "some_value",
				DriftType:      driftchecker.AttributeValueChanged,
			},
		},
	}
	ctx := context.Background()

	err := reporter.WriteReport(ctx, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal drift report to JSON")
	assert.Contains(t, err.Error(), "unsupported type: chan int")
}

func TestFileReporter_WriteReport_NoOutputDir(t *testing.T) {
	// Test case where outputFile is just a filename, no directory specified
	tmpFile, err := os.CreateTemp("", "report_no_dir.json")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewFileReporter(tmpFile.Name())
	report := createDummyDriftReportForFile(false)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	// Verify file exists and has content
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.True(t, len(data) > 0)
	assert.Contains(t, string(data), report.ResourceId)
}
