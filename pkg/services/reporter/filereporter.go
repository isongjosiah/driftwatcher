package reporter

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type FileReporter struct {
	OutputFile string
}

// NewFileReporter creates a new FileReporter instance.
// outputFile: The path to the file where the report will be written.
func NewFileReporter(outputFile string) *FileReporter {
	return &FileReporter{
		OutputFile: outputFile,
	}
}

// WriteReport marshals the DriftReport to JSON and writes it to the configured file.
// If the file does not exist, it will be created. If it exists, its content will be truncated.
func (f *FileReporter) WriteReport(ctx context.Context, report *driftchecker.DriftReport) error {
	// Ensure the output directory exists
	outputDir := filepath.Dir(f.OutputFile)
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
		}
	}

	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal drift report to JSON: %w", err)
	}

	err = os.WriteFile(f.OutputFile, reportBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write drift report to file %s: %w", f.OutputFile, err)
	}

	fmt.Printf("Drift report successfully written to: %s\n", f.OutputFile)
	return nil
}
