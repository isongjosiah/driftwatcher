package statemanager

type IaCTool string

const (
	TerraformTool IaCTool = "terraform"
)

type ProviderType string

const (
	AwsProvider ProviderType = "aws"
)
