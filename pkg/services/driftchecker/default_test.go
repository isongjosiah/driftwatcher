package driftchecker_test

import (
	"context"
	"drift-watcher/pkg/services/driftchecker"
	"drift-watcher/pkg/services/statemanager"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockInfrastructureResourceI is a mock implementation of provider.InfrastructureResourceI
type MockInfrastructureResourceI struct {
	mock.Mock
}

func (m *MockInfrastructureResourceI) ResourceType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockInfrastructureResourceI) AttributeValue(attribute string) (string, error) {
	args := m.Called(attribute)
	return args.String(0), args.Error(1)
}

// MockStateResource is a mock implementation of statemanager.StateResource
// We'll use the actual statemanager.StateResource struct for desiredState,
// but for testing purposes, we can create a similar mock if needed for specific interface behavior.
// For now, we'll rely on the actual struct's methods for desiredState.

func TestNewDefaultDriftChecker(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	assert.NotNil(t, checker)
}

func TestCompareStates_LiveStateIsNil(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	// desiredState can be a dummy, as liveState being nil is checked first
	desiredState := statemanager.StateResource{Type: "aws_s3_bucket"}

	report, err := checker.CompareStates(ctx, nil, desiredState, []string{})
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.HasDrift)
	assert.Equal(t, driftchecker.ResourceMissingInInfrastructure, report.Status)
	assert.Empty(t, report.DriftDetails)
}

func TestCompareStates_ResourceTypeMismatch(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_ec2_instance")

	desiredState := statemanager.StateResource{Type: "aws_s3_bucket"}

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource type mismatch")
	assert.Contains(t, err.Error(), "live resource aws_ec2_instance does not match desired type aws_s3_bucket")
	assert.NotNil(t, report)         // Report should still be initialized
	assert.False(t, report.HasDrift) // Default value, as error occurred before drift check
	assert.Empty(t, report.Status)   // Status not set on mismatch error
	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_NoDrift(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	mockLiveState.On("AttributeValue", "bucket_name").Return("my-test-bucket", nil)
	mockLiveState.On("AttributeValue", "acl").Return("private", nil)

	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_name": "my-test-bucket",
					"acl":         "private",
				},
			},
		},
	}
	attributesToTrack := []string{"bucket_name", "acl"}

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.HasDrift)
	assert.Equal(t, driftchecker.Match, report.Status)
	assert.Equal(t, "aws_s3_bucket", report.ResourceType)
	assert.Len(t, report.DriftDetails, 2)

	// Verify drift details for no drift
	assert.Equal(t, "bucket_name", report.DriftDetails[0].Field)
	assert.Equal(t, "my-test-bucket", report.DriftDetails[0].TerraformValue)
	assert.Equal(t, "my-test-bucket", report.DriftDetails[0].ActualValue)
	assert.Equal(t, driftchecker.Match, report.DriftDetails[0].DriftType)

	assert.Equal(t, "acl", report.DriftDetails[1].Field)
	assert.Equal(t, "private", report.DriftDetails[1].TerraformValue)
	assert.Equal(t, "private", report.DriftDetails[1].ActualValue)
	assert.Equal(t, driftchecker.Match, report.DriftDetails[1].DriftType)

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_AttributeValueChanged(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	mockLiveState.On("AttributeValue", "bucket_acl").Return("public-read", nil)
	mockLiveState.On("AttributeValue", "versioning").Return("Enabled", nil)

	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_acl": "private",
					"versioning": "Disabled",
				},
			},
		},
	}
	attributesToTrack := []string{"bucket_acl", "versioning"}

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.HasDrift)
	assert.Equal(t, driftchecker.Drift, report.Status)
	assert.Len(t, report.DriftDetails, 2)

	// Verify first drift item
	assert.Equal(t, "bucket_acl", report.DriftDetails[0].Field)
	assert.Equal(t, "private", report.DriftDetails[0].TerraformValue)
	assert.Equal(t, "public-read", report.DriftDetails[0].ActualValue)
	assert.Equal(t, driftchecker.AttributeValueChanged, report.DriftDetails[0].DriftType)

	// Verify second drift item
	assert.Equal(t, "versioning", report.DriftDetails[1].Field)
	assert.Equal(t, "Disabled", report.DriftDetails[1].TerraformValue)
	assert.Equal(t, "Enabled", report.DriftDetails[1].ActualValue)
	assert.Equal(t, driftchecker.AttributeValueChanged, report.DriftDetails[1].DriftType)

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_AttributeMissingInTerraform(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	mockLiveState.On("AttributeValue", "tags.Name").Return("my-app", nil)

	// Desired state does not have "tags.Name"
	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{}, // No "tags.Name" attribute
			},
		},
	}
	attributesToTrack := []string{"tags.Name"}

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.HasDrift)
	assert.Equal(t, driftchecker.Drift, report.Status)
	assert.Len(t, report.DriftDetails, 1)

	assert.Equal(t, "tags.Name", report.DriftDetails[0].Field)
	assert.Empty(t, report.DriftDetails[0].TerraformValue) // Expected empty from desiredState
	assert.Equal(t, "my-app", report.DriftDetails[0].ActualValue)
	assert.Equal(t, driftchecker.AttributeMissingInTerraform, report.DriftDetails[0].DriftType)

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_AttributeMissingInInfrastructure(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	// Simulate attribute missing in live infrastructure
	mockLiveState.On("AttributeValue", "website_endpoint").Return("", fmt.Errorf("attribute does not exist"))

	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"website_endpoint": "http://example.com",
				},
			},
		},
	}
	attributesToTrack := []string{"website_endpoint"}

	// Capture slog output to ensure warning is logged
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.HasDrift)
	assert.Equal(t, driftchecker.Drift, report.Status)
	assert.Len(t, report.DriftDetails, 1)

	assert.Equal(t, "website_endpoint", report.DriftDetails[0].Field)
	assert.Equal(t, "http://example.com", report.DriftDetails[0].TerraformValue)
	assert.Empty(t, report.DriftDetails[0].ActualValue) // Expected empty due to error
	assert.Equal(t, driftchecker.AttributeMissingInInfrastructure, report.DriftDetails[0].DriftType)

	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "Failed to retrieve value of website_endpoint attribute for live state")

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_DesiredStateAttributeError(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	mockLiveState.On("AttributeValue", "bucket_name").Return("my-live-bucket", nil)

	// Simulate desiredState.AttributeValue returning an error (e.g., attribute not a string)
	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_name": 123, // Not a string, will cause error in AttributeValue
				},
			},
		},
	}
	attributesToTrack := []string{"bucket_name"}

	// Capture slog output to ensure warning is logged
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err) // The function continues, logging a warning
	assert.NotNil(t, report)
	assert.False(t, report.HasDrift)                   // No drift recorded for this attribute as comparison failed
	assert.Equal(t, driftchecker.Match, report.Status) // Overall status remains Match if no other drift
	assert.Len(t, report.DriftDetails, 0)              // No drift item added if attribute value retrieval fails

	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "Failed to retrieve value of bucket_name attribute for desired state")

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_MixedDriftTypes(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")
	mockLiveState.On("AttributeValue", "bucket_name").Return("live-bucket", nil)             // Changed
	mockLiveState.On("AttributeValue", "acl").Return("private", nil)                         // Match
	mockLiveState.On("AttributeValue", "tags.Owner").Return("team-a", nil)                   // Missing in TF
	mockLiveState.On("AttributeValue", "website_config").Return("", fmt.Errorf("not found")) // Missing in Infra

	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{
					"bucket_name":    "desired-bucket",
					"acl":            "private",
					"website_config": "enabled", // Present in desired, missing in live
				},
			},
		},
	}
	attributesToTrack := []string{"bucket_name", "acl", "tags.Owner", "website_config"}

	// Capture slog output
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.HasDrift)
	assert.Equal(t, driftchecker.Drift, report.Status)
	assert.Len(t, report.DriftDetails, 4) // All attributes should be processed

	// Verify bucket_name (AttributeValueChanged)
	assert.Equal(t, "bucket_name", report.DriftDetails[0].Field)
	assert.Equal(t, "desired-bucket", report.DriftDetails[0].TerraformValue)
	assert.Equal(t, "live-bucket", report.DriftDetails[0].ActualValue)
	assert.Equal(t, driftchecker.AttributeValueChanged, report.DriftDetails[0].DriftType)

	// Verify acl (Match)
	assert.Equal(t, "acl", report.DriftDetails[1].Field)
	assert.Equal(t, "private", report.DriftDetails[1].TerraformValue)
	assert.Equal(t, "private", report.DriftDetails[1].ActualValue)
	assert.Equal(t, driftchecker.Match, report.DriftDetails[1].DriftType)

	// Verify tags.Owner (AttributeMissingInTerraform)
	assert.Equal(t, "tags.Owner", report.DriftDetails[2].Field)
	assert.Empty(t, report.DriftDetails[2].TerraformValue)
	assert.Equal(t, "team-a", report.DriftDetails[2].ActualValue)
	assert.Equal(t, driftchecker.AttributeMissingInTerraform, report.DriftDetails[2].DriftType)

	// Verify website_config (AttributeMissingInInfrastructure)
	assert.Equal(t, "website_config", report.DriftDetails[3].Field)
	assert.Equal(t, "enabled", report.DriftDetails[3].TerraformValue)
	assert.Empty(t, report.DriftDetails[3].ActualValue) // Empty because liveState.AttributeValue returned error
	assert.Equal(t, driftchecker.AttributeMissingInInfrastructure, report.DriftDetails[3].DriftType)

	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "Failed to retrieve value of website_config attribute for live state")

	mockLiveState.AssertExpectations(t)
}

func TestCompareStates_EmptyAttributesToTrack(t *testing.T) {
	checker := driftchecker.NewDefaultDriftChecker()
	ctx := context.Background()

	mockLiveState := new(MockInfrastructureResourceI)
	mockLiveState.On("ResourceType").Return("aws_s3_bucket")

	desiredState := statemanager.StateResource{
		Type: "aws_s3_bucket",
		Instances: []statemanager.ResourceInstance{
			{
				Attributes: map[string]any{"bucket_name": "test"},
			},
		},
	}
	attributesToTrack := []string{} // Empty list

	report, err := checker.CompareStates(ctx, mockLiveState, desiredState, attributesToTrack)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.HasDrift)
	assert.Equal(t, driftchecker.Match, report.Status)
	assert.Empty(t, report.DriftDetails)

	mockLiveState.AssertExpectations(t)
}
