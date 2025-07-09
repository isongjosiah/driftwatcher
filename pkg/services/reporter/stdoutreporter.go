package reporter

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"encoding/json"
	"fmt"
	"os"
)

// StdoutReporter implements OutputWriter to write reports to standard output.
type StdoutReporter struct{}

// NewStdoutReporter creates a new StdoutReporter instance.
func NewStdoutReporter() *StdoutReporter {
	return &StdoutReporter{}
}

// WriteReport marshals the DriftReport to JSON and prints it to os.Stdout.
func (s *StdoutReporter) WriteReport(ctx context.Context, report *driftchecker.DriftReport) error {
	// Marshal the report struct to JSON bytes
	// We use json.MarshalIndent for pretty-printed JSON, which is easier to read.
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal drift report to JSON for stdout: %w", err)
	}

	// Write the JSON bytes to standard output
	// fmt.Println adds a newline at the end.
	// os.Stdout.Write can also be used if you need more fine-grained control
	_, err = fmt.Fprintln(os.Stdout, string(reportBytes))
	if err != nil {
		return fmt.Errorf("failed to write drift report to stdout: %w", err)
	}

	return nil
}
