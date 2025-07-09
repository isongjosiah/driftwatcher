package reporter

import (
	"drift-watcher/pkg/services/driftchecker"
	"time"
)

// Helper function to create a dummy DriftReport for testing
func CreateDummyDriftReport(hasDrift bool) *driftchecker.DriftReport {
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
