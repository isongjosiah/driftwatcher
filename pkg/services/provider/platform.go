package provider

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"drift-watcher/pkg/services/statemanager"
)

type InfrastructureResourceI interface {
	ResourceType() string
	AttributeValue(attribute string) (string, error)
}

//counterfeiter:generate . ProviderI
type ProviderI interface {
	InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (InfrastructureResourceI, error)
}
