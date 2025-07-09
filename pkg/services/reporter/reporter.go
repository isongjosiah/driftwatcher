package reporter

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
)

type OutputWriter interface {
	WriteReport(ctx context.Context, report *driftchecker.DriftReport) error
}
