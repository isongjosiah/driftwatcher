# DriftWatcher CLI Documentation

## 1. Project Overview

This section provides a high-level description of the project, its purpose, and its key features.

### What is this project?

This is a command-line interface (CLI) tool written in Go that parses Terraform state or HCL configurations and compares them with their corresponding AWS EC2 instance configurations. It detects and reports drifts across a specified list of attributes. The tool handles various field types and nested structures, returning whether a drift is detected for any attribute and specifying which attributes have changed, the desired value and observed value, all presented in a structured, easy-to-understand format.

### Key Features

**EC2 Drift Detection**: Compares live AWS EC2 instance configurations against Terraform state or HCL.

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

- `description`
- `egress`
- `ingress`
- `name`
- `vpc_id`

> **Note**: This list can be extended to other attributes, resources, and platforms in future versions.

**Structured Reporting**: Presents detected drifts in an easy-to-understand format, detailing attribute changes, including desired and observed values.

**Flexible Configuration Input**: This tool supports parsing configuration from both Terraform state files (`.tfstate`) and HCL configuration files (`.tf`).

**State File Preference**: It is highly recommended to use Terraform state files (`.tfstate`) for configuration input, as parsing directly from HCL files (`.tf`) is not yet stable and may not capture all nuances of your infrastructure's desired state.

**State File Lookup Logic** (when HCL is provided): When an HCL configuration file is provided, the tool first attempts to locate a corresponding Terraform state file. It looks for a state file explicitly specified, then a default state file (e.g., `terraform.tfstate`) in the HCL configuration's path. If no state file is found, the tool will attempt to parse configuration information directly from the HCL file.

> **Note**: Extensive parsing directly from HCL files was initially explored but has been temporarily abandoned in favour of fetching state files based on the HCL configuration, as described, until HCL parsing capabilities are stabilised.

**Local File Support Only**: Currently, configuration files can only be fetched locally. For instructions on how to fetch remote state locally, please refer to the "Fetching Terraform State Locally (Recommended)" section below.

**Exit Statuses**: Provides clear exit codes for scripting (0 for no drift/success, 1 for general errors, 2 for drift detected).

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

### Basic Usage

To run the CLI tool, open your terminal and use the main command, followed by its subcommands and arguments.

```bash
driftwatcher --help
```

This command will display the main help message, listing available subcommands.

### Common Commands

**Detect configuration drift**:

```bash
driftwatcher detect \
  --configfile "path/to/your/terraform.tfstate" \
  --attributes "instance_type,ami,tags" \
  --awsprofile "my-dev-profile" \
  --provider "aws" \
  --resource "ec2" \
  --output-file "drift_report.json"
```

This command compares the live EC2 instance configuration with the specified Terraform state file, checking for drifts in `instance_type`, `ami`, and `tags`. It uses the `my-dev-profile` AWS credentials and outputs the report to `drift_report.json`.

Output example:

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

### Advanced Usage

**Piping output**:

```bash
driftwatcher detect > file.json
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

**Programming Language: Go**

- **Reasoning**: Chosen for its excellent performance, strong concurrency primitives (goroutines), static typing, and the ability to compile into single, self-contained binaries, making distribution very simple. It's well-suited for building efficient and reliable CLI tools.
- **Trade-offs**: Can have a steeper learning curve for developers new to compiled languages or Go's specific paradigms (e.g., error handling, interfaces).

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

- **Performance vs. Ease of Use**: In some cases, a more user-friendly interactive prompt or a simpler command structure was prioritised over raw execution speed, aiming for a better developer experience.
- **Feature Scope vs. Initial Release**: The initial release focuses on a core set of functionalities to deliver immediate value, deferring more advanced or niche features to future iterations to avoid scope creep.
- **Cross-platform Compatibility**: Go's excellent cross-compilation capabilities minimise this trade-off, but platform-specific interactions (e.g., file paths, permissions) still require careful handling.

## 6. Ideas for Future Improvements

This section outlines potential enhancements and new features that could be added in the future.

### Feature Enhancements

- Add more subcommands for managing configuration
- Implement tab-completion for commands and arguments in various shells (Bash, Zsh, PowerShell)
- Support for different output formats (e.g., CSV, XML) in addition to JSON
- Add a daemon mode for continuous operations

### Performance Optimisations

- Optimise startup time for complex commands
- Implement caching for frequently accessed remote data

### User Experience

- Introduce a more advanced interactive mode with richer prompts (e.g., using Go libraries like survey or go-prompt)
- Implement a mechanism within the interactive mode to take actions that fix identified drifts, providing guided remediation options
- Add progress indicators for long-running operations
- Provide clearer and more context-sensitive help messages

### Testing

- Expand unit and integration test coverage for all commands and their logic using Go's built-in testing framework
- Implement end-to-end tests to simulate real user interactions
- Set up a CI/CD pipeline for automated testing and deployment

### Distribution

- Provide pre-built binaries for common operating systems directly from Go's cross-compilation capabilities
- Publish to package managers (e.g., Homebrew for macOS, Scoop for Windows)

### Security

- Implement secure handling of sensitive credentials (e.g., using OS keychains or secure environment variable practices)
-
