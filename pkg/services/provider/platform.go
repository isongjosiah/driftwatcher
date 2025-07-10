// Package provider defines interfaces for interacting with cloud infrastructure providers
// and their resources. It provides an abstraction layer that allows the drift detection
// system to work with different cloud providers (AWS, Azure, GCP, etc.) through a
// common interface, enabling consistent resource metadata retrieval and attribute access.
package provider

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"drift-watcher/pkg/services/statemanager"
)

// InfrastructureResourceI defines the interface for accessing live infrastructure resource data
// from cloud providers. This interface provides a consistent way to interact with any type
// of infrastructure resource, regardless of the underlying provider or resource type.
//
// Implementations of this interface represent actual live resources (e.g., EC2 instances,
// S3 buckets, etc.) and provide methods to retrieve their current state and attributes
// for comparison with the desired state defined in IaC configurations.
//
//counterfeiter:generate . InfrastructureResourceI
type InfrastructureResourceI interface {
	ResourceType() string
	AttributeValue(attribute string) (string, error)
}

// ProviderI defines the interface for cloud infrastructure providers.
// This interface abstracts the process of connecting to different cloud providers
// and retrieving live resource metadata. It enables the drift detection system
// to work with multiple cloud providers through a unified interface.
//
// Implementations of this interface handle provider-specific authentication,
// API interactions, and resource metadata retrieval logic.
//
//counterfeiter:generate . ProviderI
type ProviderI interface {
	InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (InfrastructureResourceI, error)
}
