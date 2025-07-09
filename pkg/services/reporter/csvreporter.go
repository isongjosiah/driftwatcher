package reporter

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CsvReporter implements OutputWriter to write reports to a CSV file.
type CsvReporter struct {
	outputFile string
}

// NewCsvReporter creates a new CsvReporter instance.
// outputFile: The path to the CSV file where the report will be written.
func NewCsvReporter(outputFile string) *CsvReporter {
	return &CsvReporter{
		outputFile: outputFile,
	}
}

// WriteReport converts the DriftReport into CSV format and writes it to the configured file.
// Each row in the CSV represents a single DriftItem, or a summary row if no drift.
func (c *CsvReporter) WriteReport(ctx context.Context, report *driftchecker.DriftReport) error {
	// Ensure the output directory exists
	outputDir := filepath.Dir(c.outputFile)
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s for CSV report: %w", outputDir, err)
		}
	}

	file, err := os.Create(c.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create CSV output file %s: %w", c.outputFile, err)
	}
	defer file.Close() // Ensure the file is closed after function returns

	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush() // Ensure all buffered data is written to the file

	// Define CSV header
	header := []string{
		"GeneratedAt",
		"ResourceId",
		"ResourceType",
		"ResourceName", // Corrected typo in comments, assuming 'resource_name'
		"HasDrift",
		"ReportStatus", // Overall report status (MATCH/DRIFT)
		"DriftField",
		"TerraformValue",
		"ActualValue",
		"DriftType", // Specific drift item type
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Handle the case where there is no specific drift details but we still want a record
	if !report.HasDrift || len(report.DriftDetails) == 0 {
		row := []string{
			report.GeneratedAt.Format(time.RFC3339),
			report.ResourceId,
			report.ResourceType,
			report.ResourceName,                // Using the field name from your struct. If this is `resource_nae`, it might still be a typo.
			fmt.Sprintf("%t", report.HasDrift), // Convert bool to string
			report.Status,
			"", // DriftField (empty for no drift)
			"", // TerraformValue (empty for no drift)
			"", // ActualValue (empty for no drift)
			"", // DriftType (empty for no drift)
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write no-drift summary row to CSV: %w", err)
		}
	} else {
		// Iterate over each DriftItem and write a row for each
		for _, item := range report.DriftDetails {
			row := []string{
				report.GeneratedAt.Format(time.RFC3339),
				report.ResourceId,
				report.ResourceType,
				report.ResourceName, // Using the field name from your struct. If this is `resource_nae`, it might still be a typo.
				fmt.Sprintf("%t", report.HasDrift),
				report.Status,
				item.Field,
				fmt.Sprintf("%v", item.TerraformValue), // Convert any to string
				fmt.Sprintf("%v", item.ActualValue),    // Convert any to string
				string(item.DriftType),                 // Convert custom type to string
			}
			if err := csvWriter.Write(row); err != nil {
				return fmt.Errorf("failed to write drift item row to CSV: %w", err)
			}
		}
	}

	fmt.Printf("Drift report successfully written to: %s (CSV format)\n", c.outputFile)
	return nil
}
