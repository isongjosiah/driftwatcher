# DriftWatcher CLI Documentation

## Table of Contents

1. [Project Overview](#1-project-overview)
   - [What is this project?](#what-is-this-project)
   - [Architectural Design](#architectural-design)
     - [Modular and Interface-Driven Design](#modular-and-interface-driven-design)
     - [Supported Attributes for Drift Detection](#supported-attributes-for-drift-detection)
       - [Core Instance Configuration](#core-instance-configuration)
       - [Networking & Security](#networking--security)
       - [Storage (EBS Volumes)](#storage-ebs-volumes)
       - [Metadata & User Data](#metadata--user-data)
       - [State](#state)
       - [AWS Security Group Specific Attributes](#aws-security-group-specific-attributes-for-related-resources)
2. [Setup and Installation Instructions](#2-setup-and-installation-instructions)
   - [Prerequisites](#prerequisites)
   - [Installation Steps](#installation-steps)
3. [Usage Examples](#3-usage-examples)
   - [Basic Usage](#basic-usage)
   - [Common Scenarios](#common-scenarios)
     - [Detect Drift for a Specific Terraform State File](#1-detect-drift-for-a-specific-terraform-state-file)
     - [Checking an HCL Configuration file](#2-checking-an-hcl-configuration-file)
     - [Using with LocalStack for Development/Testing](#using-with-localstack-for-developmenttesting)
       - [Testing with LocalStack (Detailed Walkthrough)](#testing-with-localstack-detailed-walkthrough)
4. [Running Tests](#4-running-tests)
5. [Design Decisions and Trade-offs](#5-design-decisions-and-trade-offs)
   - [Architectural Choices](#architectural-choices)
   - [Key Design Patterns / Principles](#key-design-patterns--principles)
   - [Trade-offs Made](#trade-offs-made)
6. [Ideas for Future Improvements](#7-ideas-for-future-improvements)
   - [Feature Enhancements](#feature-enhancements)
   - [User Experience](#user-experience)
   - [Testing](#testing)
   - [Distribution](#distribution)
   - [Security](#security)

---

## 1. Project Overview

This section provides a high-level description of the project, its purpose, and its key features.

### What is this project?

This is a command-line interface (CLI) tool written in Go that parses Terraform state or HCL configurations and compares them with their corresponding AWS EC2 instance configurations. It detects and reports drifts across a specified list of attributes. The tool handles various field types and nested structures, returning whether a drift is detected for any attribute and specifying which attributes have changed, the desired value and observed value, all presented in a structured, easy-to-understand format.

### Architectural Design

This section delves into the core architectural choices for the drfitwatcher CLI tool, highlighting how its modular and interface-driven design contributes to flexibility, testability, and maintainability

#### Modular and Interface-Driven Design

The DriftWatcher CLI is built with a strong emphasis on modularity and the use of Go interfaces. This approach separates concerns and defines clear contracts between different components of the application. The key interfaces are:

- `StateManagerI`: Defined in `pkg/services/statemanager/statemanager.go` this interface abstracts the process of parsing and retrieving resource information from configuration files (e.g., Terraform state files). This allows the application to support various Infrastructure as Code (IaC) tools and state formats without modifying the core drift detection logic.

```go
type StateManagerI interface {
 ParseStateFile(ctx context.Context, statePath string) (StateContent, error)
 RetrieveResources(ctx context.Context, content StateContent, resourceType string) ([]StateResource, error)
}
```

- `ProviderI`: Defined in `pkg/services/provider/platform.go`, this interface his interface represents a generic cloud or infrastructure provider. It's responsible for fetching live resource metadata from the actual infrastructure. This design allows for easy extension to other cloud providers (e.g., Azure, GCP) by simply implementing this interface.

```go
type ProviderI interface {
 InfrastructreMetadata(ctx context.Context, resourceType string, resource statemanager.StateResource) (InfrastructureResourceI, error)
}
```

-`InfrastructureResourceI`: Defined in `pkg/services/provider/platform.go` , this interface defines how to interact with an individual resource's live data from a provider. It ensures that any concrete implementation of a resource (e.g., an AWS EC2 instance) provides a consistent way to retrieve its type and attribute values.

```go
type InfrastructureResourceI interface {
 ResourceType() string
 AttributeValue(attribute string) (string, error)
}
```

- `DriftChecker`: Defined in `pkg/services/driftchecker/driftchecker.go`, this interface encapsulates the core logic for comparing a desired state (from the `StateManager`) with the actual live state (from the `Provider`). This separation ensures that the comparison algorithm can be independently tested and potentially swapped out for different comparison strategies.

```go
type DriftChecker interface {
 CompareStates(ctx context.Context, liveData provider.InfrastructureResourceI, desiredState statemanager.StateResource, attributesToTrack []string) (*DriftReport, error)
}
```

- `OutputWriter`: Defined in `reporter.go` this interface handles the reporting of drift detection results. This allows for various output formats (e.g., JSON to file, stdout) without affecting the drift detection process itself.

```go
type OutputWriter interface {
 WriteReport(ctx context.Context, report *driftchecker.DriftReport) error
}
```

The architectural design of the DriftWatcher CLI emphasizes high modularity and separation of concerns through the extensive use of Go interfaces. This approach makes the codebase easier to understand, manage, and debug.

Here are the key advantages:

Extensibility: The interface-driven design allows for straightforward expansion to support new Infrastructure as Code (IaC) tools (e.g., Pulumi, CloudFormation), new cloud providers (e.g., Azure, GCP), and different reporting formats simply by implementing the relevant interfaces.

Testability: Interfaces facilitate easy mocking and dependency injection, enabling isolated testing of individual components, leading to more robust and reliable code.

Maintainability: Changes or bug fixes in one module are less likely to affect others, as long as the interface contracts are preserved, simplifying long-term maintenance.

Flexibility: Components like StateManager, PlatformProvider, DriftChecker, and Reporter can be dynamically instantiated based on runtime configurations, allowing users to customize the tool's behavior.

Concurrency Management: The design incorporates Go's concurrency primitives to process resources in parallel, enhancing performance for large-scale drift detection by efficiently managing concurrent requests to cloud providers.

This modular and interface-driven architecture not only makes DriftWatcher a functional, scalable, and adaptable CLI tool but also inherently allows it to be used as a library. Because the core functionalities are abstracted behind interfaces, other applications can easily import and utilize these components (e.g., the StateManager, Provider, DriftChecker, and Reporter services) independently, integrating drift detection capabilities into larger systems without running the full CLI.

**Supported Attributes for Drift Detection**:

#### Core Instance Configuration

- `ami` (Amazon Machine Image ID)
- `instance_type` (e.g., t2.micro, m5.large)
- `instance_id`
- `key_name` (SSH key pair name)
- `availability_zone`
- `tenancy` (instance tenancy: default, dedicated, or host)
- `monitoring`
- `cpu_core_count`
- `cpu_thread_per_core`
- `ebs_optimized`

#### Networking & Security

- `security_group_ids` (list of security group IDs)
- `subnet_id` (ID of the subnet the instance is launched in)
- `associate_public_ip_address`
- `private_ip`
- `private_dns_name`
- `public_ip`
- `public_dns_name`
- `source_dest_check`
- `iam_instance_id`
- `iam_instance_arn`

#### Storage (EBS Volumes)

- `root_block_device` (Configuration of the root EBS volume)
- `ebs_block_device` (Configuration of additional EBS volumes attached)
- `block_device_name` (sub-attribute for block devices)
- `volume_id` (sub-attribute for block devices)
- `volume_size` (sub-attribute for block devices)
- `volume_type` (sub-attribute for block devices)
- `encrypted` (sub-attribute for block devices)
- `delete_on_termination` (sub-attribute for block devices)

#### Metadata & User Data

- `metadata_options`
- `user_data` (user data script attached to the instance)
- `user_data_base64`

#### State

- `instance_state`

#### AWS Security Group Specific Attributes (for related resources)

- `security_group_ids`

> **Note**: This list can be extended to other attributes, resources, and platforms in future versions.

**Structured Reporting**: Presents detected drifts in an easy-to-understand format, detailing attribute changes, including desired and observed values.

**Flexible Configuration Input**: This tool supports parsing configuration from both Terraform state files (`.tfstate`) and HCL configuration files (`.tf`). It is highly recommended to use Terraform state files (`.tfstate`) for configuration input, as parsing directly from HCL files (`.tf`) is not yet stable and may not capture all nuances of your infrastructure's desired state.

**State File Lookup Logic** (when HCL is provided): When an HCL configuration file is provided, the tool first attempts to locate a corresponding Terraform state file. It looks for a state file explicitly specified, then a default state file (e.g., `terraform.tfstate`) in the HCL configuration's path.

> **Note**: Extensive parsing directly from HCL files was initially explored but has been temporarily abandoned in favour of fetching state files based on the HCL configuration, as described, until HCL parsing capabilities are stabilised.

**Local File Support Only**: Currently, configuration files can only be fetched locally. For instructions on how to fetch remote state locally, please refer to the "Fetching Terraform State Locally (Recommended)" section below.

## 2. Setup and Installation Instructions

This section guides users on how to get the project up and running on their local machine.

### Prerequisites

Before you begin, ensure you have the following installed:

- **Go**: v1.24
- **Git**: v2.x
- **Terraform CLI**: Required if you plan to use `terraform pull` to fetch state files.

### Installation Steps

Follow these steps to set up the project locally:

1. **Clone the repository**:

   ```bash
   git clone https://github.com/isongjosiah/driftwatcher.git
   cd driftwatcher
   ```

2. **Build the CLI tool**:

   ```bash
   make build
   ```

   This command compiles the Go source code into an executable binary named `driftwatcher` (or `driftwatcher.exe` on Windows) in the current directory.

3. **AWS Credentials and Profile Setup**:
   This CLI tool interacts with AWS services and requires proper authentication. The tool will automatically look for credentials in the following order:
   - Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`)
   - The shared credentials file (`~/.aws/credentials` or `%USERPROFILE%\.aws\credentials`)
   - The shared config file (`~/.aws/config` or `%USERPROFILE%\.aws\config`)

   For detailed instructions on setting up your AWS credentials and profiles, please refer to the official AWS documentation:
   - [Configuring the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)
   - [Setting up AWS credentials (specifically for Go SDK)](https://docs.aws.amazon.com/sdk-for-go/api/aws/session/)

   You can configure a named profile using the AWS CLI:

   ```bash
   aws configure --profile your-profile-name
   ```

   Then, you can instruct the tool to use a specific profile by setting the `AWS_PROFILE` environment variable:

   ```bash
   export AWS_PROFILE="your-profile-name"
   ```

   When a custom profile is used, its credential path takes precedence over the default path for profiles that exist in both configurations.

4. **Fetching Terraform State Locally (Recommended)**:
   If your Terraform state is managed remotely (e.g., in S3, Terraform Cloud), you can use the `terraform pull` command to download the latest state file to your local machine before running this CLI tool.

   First, ensure you are in the directory containing your Terraform configuration (`.tf` files). Then, run:

   ```bash
   terraform pull > terraform.tfstate
   ```

   This command fetches the latest state from your configured backend and pipes it into a local file named `terraform.tfstate`. You can then point the CLI tool to this local state file.

5. **Make the CLI tool accessible (optional, but recommended)**:
   To run the tool from any directory, move the compiled binary to a directory included in your system's PATH.

   ```bash
   # For Linux/macOS
   sudo mv driftwatcher /usr/local/bin/

   # For Windows, add the directory containing driftwatcher.exe to your system's PATH
   # Or place driftwatcher.exe in a directory already in PATH (e.g., C:\Windows)
   ```

   After this step, you should be able to run `driftwatcher` from any terminal location.

## 3. Usage Examples

This section demonstrates how to use the CLI tool, providing clear examples for common functionalities.

**Command Flags Reference**

The `detect` command supports the following flags to customize its behavior:

- `--configfile` (string, required): Specifies the path to your Terraform configuration file. This can be a Terraform state file (`.tfstate``) or an HCL configuration file (`.tf`). It is highly recommended to use a`.tfstate` file for accurate drift detection.

- `--attributes` (string slice, default: `instance_type`): A comma-separated list of resource attributes to check for drift. For example:`instance_type,ami`.

- `--awsprofile` (string, default: `default`): The name of the AWS profile to use for authenticating with AWS services. This corresponds to profiles configured in your ~/.aws/credentials or ~/.aws/config files.

- `--provider` (string, default: `aws`): Specifies the cloud provider to interact with. Currently, only aws is supported.

- `--resource` (string, default: `aws_instance`): Defines the specific type of resource to check for drift. For AWS, only `aws_instance`
  is currently supported

- `--output-file (string)`: If provided, the drift report will be written to this file in JSON format. If omitted, the report will be printed to standard output (stdout).

- `--state-manager` (string, default: `terraform`): Specifies the state manager type to use for parsing your configuration. Currently, only terraform is supported.

- `--localstack-url` (string): If provided, the tool will connect to a LocalStack instance at this URL for AWS API calls, useful for local development and testing. When used, `DRIFT_LOCALSTACK_URL`` and`DRIFT_LOCALSTACK_REGION` environment variables are temporarily set.

- `--localstackregion` (string, default: `us-east-1``): Specifies the AWS region to use when connecting to LocalStack. Only relevant when`--localstack-url` is also provided.

### Basic Usage

To display the main help message, listing available subcommands and their flags

```bash
driftwatcher detect --help
```

### Common Scenarios

#### 1. **Detect Drift for a Specific Terraform State File**

This is the recommended way to use driftwatcher, ensuring the most accurate
comparison against your desired state.

```bash
bin/driftwatcher detect \
--configfile "path/to/your/terraform.tfstate" \
--attributes "instance_type,ami,tags" \
--awsprofile "my-dev-profile" \
--provider "aws" \
--resource "aws_instance" \
--output-file "drift_report.json"
```

This command will:

- Read the desired state from your `terraform.tfstate` file located at `path/to/your/terraform.tfstate`
- Connect to AWS using the `my-dev-profile` credentials.
- Focus on `aws_instance` resources.
- Check for differences in `instance_type`, `ami`, and `tags` attributes between
  your Terraform state and live AWS environment.
- Write the detailed drfit report to `drift_report.json`

  Output example(`drift_report.json` content):

```json
{
  "resource_type": "aws_instance",
  "has_drift": true,
  "drift_details": [
    {
      "field": "instance_type",
      "terraform_value": "t2.micro",
      "actual_value": "t2.micro",
      "drift_type": "MATCH"
    },
    {
      "field": "ami",
      "terraform_value": "ami-0c55b159cbfafe1d0",
      "actual_value": "ami-0b7ef3c7339f4970c",
      "drift_type": "VALUE_CHANGED"
    }
  ],
  "generated_at": "2025-07-10T11:17:15.659474+01:00",
  "status": "DRIFT"
}
```

#### 2. **Checking an HCL Configuration file**

While using `.tfstate` files is recommended, you can also point DriftWatcher
to an HCL configuration file (`.tf`). The tool will attempt to locate a corresponding
state file based on the configuration local `backend` or default to `terraform.tfstate`
in the same directory

```bash
bin/driftwatcher detect \
--configfile "./path/to/your/main.tf" \
--attributes "instance_type,key_name" \
--resource "aws_instance"
```

#### 3. **Using with LocalStack for Development/Testing**

For local development and testing purposes, you can configure DriftWatcher to
interact with a LocalStack instance instead of a real AWS environment.
First, ensure your LocalStack instance is running
(e.g., docker run -d -p 4566:4566 localstack/localstack). Then, if you have
deployed infrastructure using Terraform into
LocalStack (e.g., from ./assets/localstack as per setup), you can run:

```bash
bin/driftwatcher detect \
--configfile "./assets/localstack/terraform.tfstate" \
--provider aws \
--attributes instance_type,ami \
--localstack-url http://localhost:4566 \
--localstackregion us-east-1
```

##### **Testing with localstack(Detailed Walkthrough)**

Pull localstack with docker

```bash
docker pull localstack/localstack
```

Run localstack on port 4566

```bash
docker run -d -p 4566:4566 localstack/localstack
```

Deploy infrastrcuture with terraform

```bash
cd ./assets/localstack
terraform init
terrafrom plan
terraform apply
```

Build and Run the cli

```bash
cd ../..
make build
bin/driftwatcher detect --configfile ./assets/localstack/terraform.tfstate \
--provider aws --attributes instance_type,ami --localstack-url http://localhost:4566
```

Edit attributes outside of Terraform

```bash
# stop the instance
aws --endpoint-url=http://localhost:4566 ec2 stop-instances  --region us-east-1 --instance-ids {instance_id}
# modify the instance_type attribute
aws --endpoint-url=http://localhost:4566 ec2 modify-instance-attribute  --region us-east-1 --instance-id {instance_id} --instance-type "t2.nano"
# start the instance
aws --endpoint-url=http://localhost:4566 ec2 start-instances  --region us-east-1 --instance-ids {instance_id}
```

## 4. Running Tests

This section provides instructions on how to run the tests for the project.

To execute all unit and integration tests, navigate to the root directory of the project and run:

```bash
make test
```

To run tests with verbose output:

```bash
make testv
```

To run tests with coverage output:

```bash
make testcov
```

To run specific tests, you can use the `-run` flag with a regular expression:

```bash
make test-specific TEST_FUNCTION=TestAuthHandler PACKAGE_PATH=./server/handlers
```

For detailed test coverage information:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

This will generate a `coverage.out` file and then open an HTML report in your browser, showing test coverage.

## 5. Design Decisions and Trade-offs

This section explains key architectural and design choices made during development, along with the reasoning and any trade-offs involved.

### Architectural Choices

**CLI Framework/Library: (Cobra, spf13/Viper)**

- **Reasoning**: Selected for simplifying argument parsing, subcommand management, and generating helpful command-line interfaces. Provides a structured way to build robust CLIs with Go.
- **Trade-offs**: Introduces a dependency and might have a slight learning curve for its specific conventions.

**Configuration Management: [e.g., YAML files, environment variables, TOML]**

- **Reasoning**: Using [e.g., YAML] for complex configurations allows for human-readable and version-controllable settings, while environment variables provide flexibility for sensitive data or runtime overrides. Go's standard library and external packages (like spf13/Viper) make configuration parsing efficient.
- **Trade-offs**: Requires careful handling of configuration loading order and validation to prevent errors.

### Key Design Patterns / Principles

- **Command-Line Interface (CLI) Best Practices**: Adherence to common CLI patterns for arguments, flags, subcommands, and help messages to ensure a familiar and intuitive user experience.
- **Modularity**: Breaking down complex tasks into smaller, manageable functions and modules to improve code organisation, reusability, and testability.
- **Error Handling**: Robust error handling using Go's idiomatic error return values and informative error messages to guide users when issues arise.

### Trade-offs Made

- **Feature Scope vs. Initial Release**: The initial release focuses on a core set of functionalities to deliver immediate value, deferring more advanced or niche features to future iterations to avoid scope creep.
- **Cross-platform Compatibility**: Go's excellent cross-compilation capabilities minimise this trade-off, but platform-specific interactions (e.g., file paths, permissions) still require careful handling.

## 7. Ideas for Future Improvements

This section outlines potential enhancements and new features that could be added in the future.

### Feature Enhancements

- Add more subcommands for managing configuration
- Implement tab-completion for commands and arguments in various shells (Bash, Zsh, PowerShell)
- Support for different output formats (e.g., CSV, XML) in addition to JSON
- Add a daemon mode for continuous operations

### User Experience

- Introduce a more advanced interactive mode with richer prompts (e.g., using Go libraries like survey or go-prompt)
- Implement a mechanism within the interactive mode to take actions that fix identified drifts, providing guided remediation options
- Add progress indicators for long-running operations
- Provide clearer and more context-sensitive help messages

### Testing

- Implement end-to-end tests to simulate real user interactions
- Set up a CI/CD pipeline for automated testing and deployment

### Distribution

- Provide pre-built binaries for common operating systems directly from Go's cross-compilation capabilities
- Publish to package managers (e.g., Homebrew for macOS, Scoop for Windows)

### Security

- Implement secure handling of sensitive credentials (e.g., using OS keychains or secure environment variable practices)
