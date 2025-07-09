package reporter

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
)

//counterfeiter:generate . OutputWriter
type OutputWriter interface {
	WriteReport(ctx context.Context, report *driftchecker.DriftReport) error
}
