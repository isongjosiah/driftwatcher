package reporter_test

import (
	"bytes"
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/reporter"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a dummy DriftReport for testing
func createDummyDriftReport(hasDrift bool) *driftchecker.DriftReport {
	report := &driftchecker.DriftReport{
		GeneratedAt:  time.Date(2023, time.January, 15, 10, 0, 0, 0, time.UTC),
		ResourceId:   "res-123",
		ResourceType: "aws_s3_bucket",
		ResourceName: "my-bucket-name",
		HasDrift:     hasDrift,
		Status:       "MATCH",
		DriftDetails: []driftchecker.DriftItem{},
	}

	if hasDrift {
		report.Status = "DRIFT"
		report.DriftDetails = []driftchecker.DriftItem{
			{
				Field:          "bucket_acl",
				TerraformValue: "private",
				ActualValue:    "public-read",
				DriftType:      driftchecker.AttributeValueChanged,
			},
			{
				Field:          "tags.Environment",
				TerraformValue: "dev",
				ActualValue:    "prod",
				DriftType:      driftchecker.AttributeValueChanged,
			},
		}
	}
	return report
}

func TestNewCsvReporter(t *testing.T) {
	csvReporter := reporter.NewCsvReporter("test.csv")
	assert.NotNil(t, csvReporter)
	assert.Equal(t, "test.csv", csvReporter.OutputFile)
}

func TestCsvReporter_WriteReport_NoDrift(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.csv")
	require.NoError(t, err)
	tmpFile.Close() // Close immediately as WriteReport will open it
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewCsvReporter(tmpFile.Name())
	report := createDummyDriftReport(false)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	// Read the content and verify
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2) // Header + 1 data row
	assert.Equal(t, "GeneratedAt", records[0][0])
	assert.Equal(t, "ResourceId", records[0][1])
	assert.Equal(t, "HasDrift", records[0][4])
	assert.Equal(t, "ReportStatus", records[0][5])
	assert.Equal(t, "DriftField", records[0][6])

	// Verify data row
	assert.Equal(t, "2023-01-15T10:00:00Z", records[1][0])
	assert.Equal(t, "res-123", records[1][1])
	assert.Equal(t, "aws_s3_bucket", records[1][2])
	assert.Equal(t, "my-bucket-name", records[1][3])
	assert.Equal(t, "false", records[1][4])
	assert.Equal(t, "MATCH", records[1][5])
	assert.Empty(t, records[1][6]) // DriftField should be empty
	assert.Empty(t, records[1][7]) // TerraformValue should be empty
	assert.Empty(t, records[1][8]) // ActualValue should be empty
	assert.Empty(t, records[1][9]) // DriftType should be empty
}

func TestCsvReporter_WriteReport_WithDrift(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.csv")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewCsvReporter(tmpFile.Name())
	report := createDummyDriftReport(true)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	// Read the content and verify
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 3) // Header + 2 data rows (for 2 drift items)

	// Verify header
	assert.Equal(t, "GeneratedAt", records[0][0])
	assert.Equal(t, "DriftField", records[0][6])
	assert.Equal(t, "TerraformValue", records[0][7])
	assert.Equal(t, "ActualValue", records[0][8])
	assert.Equal(t, "DriftType", records[0][9])

	// Verify first drift item row
	assert.Equal(t, "2023-01-15T10:00:00Z", records[1][0])
	assert.Equal(t, "res-123", records[1][1])
	assert.Equal(t, "my-bucket-name", records[1][3])
	assert.Equal(t, "true", records[1][4])
	assert.Equal(t, "DRIFT", records[1][5])
	assert.Equal(t, "bucket_acl", records[1][6])
	assert.Equal(t, "private", records[1][7])
	assert.Equal(t, "public-read", records[1][8])
	assert.Equal(t, driftchecker.AttributeValueChanged, records[1][9])

	// Verify second drift item row
	assert.Equal(t, "tags.Environment", records[2][6])
	assert.Equal(t, "dev", records[2][7])
	assert.Equal(t, "prod", records[2][8])
	assert.Equal(t, driftchecker.AttributeValueChanged, records[2][9])
}

func TestCsvReporter_WriteReport_CreateFileError(t *testing.T) {
	// Create a directory that exists but is not writable
	tmpDir := t.TempDir()
	nonWritableFile := filepath.Join(tmpDir, "non_writable.csv")

	// Create the file first, then make it non-writable
	_, err := os.Create(nonWritableFile)
	require.NoError(t, err)
	err = os.Chmod(nonWritableFile, 0000) // No permissions
	require.NoError(t, err)
	defer os.Chmod(nonWritableFile, 0644) // Restore permissions for cleanup
	defer os.Remove(nonWritableFile)

	reporter := reporter.NewCsvReporter(nonWritableFile)
	report := createDummyDriftReport(false)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create CSV output file")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCsvReporter_WriteReport_EmptyDriftDetailsButHasDriftTrue(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.csv")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewCsvReporter(tmpFile.Name())
	report := &driftchecker.DriftReport{
		GeneratedAt:  time.Date(2023, time.January, 15, 10, 0, 0, 0, time.UTC),
		ResourceId:   "res-456",
		ResourceType: "aws_lambda_function",
		ResourceName: "my-lambda",
		HasDrift:     true, // HasDrift is true, but DriftDetails is empty
		Status:       "DRIFT",
		DriftDetails: []driftchecker.DriftItem{},
	}
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2)               // Header + 1 data row (summary row)
	assert.Equal(t, "true", records[1][4])  // HasDrift should be true
	assert.Equal(t, "DRIFT", records[1][5]) // Status should be DRIFT
	assert.Empty(t, records[1][6])          // DriftField should be empty
}

func TestCsvReporter_WriteReport_NoOutputDir(t *testing.T) {
	// Test case where outputFile is just a filename, no directory specified
	tmpFile, err := os.CreateTemp("", "report_no_dir.csv")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	reporter := reporter.NewCsvReporter(tmpFile.Name())
	report := createDummyDriftReport(false)
	ctx := context.Background()

	err = reporter.WriteReport(ctx, report)
	require.NoError(t, err)

	// Verify file exists and has content
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.True(t, len(data) > 0)
	assert.Contains(t, string(data), "GeneratedAt,ResourceId") // Check header
}

// Mocking os.Create to simulate an error after successful directory creation
type errorWriter struct{}

func (errorWriter) Write(p []byte) (n int, err error) {
	return 0, os.ErrPermission // Simulate a write error
}

func (errorWriter) Close() error {
	return nil
}
