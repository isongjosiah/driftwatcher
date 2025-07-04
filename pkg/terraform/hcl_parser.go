package terraform

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect" // For type checking in printing

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl" // Import gohcl
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// --- Define Go Structs mirroring the HCL structure ---

// TerraformConfig represents the entire HCL file structure
type TerraformConfig struct {
	Terraform *TerraformBlock `hcl:"terraform,block"`
	Providers []ProviderBlock `hcl:"provider,block"`
	Variables []VariableBlock `hcl:"variable,block"`
	Data      []DataBlock     `hcl:"data,block"`
	Resources []ResourceBlock `hcl:"resource,block"`
	Outputs   []OutputBlock   `hcl:"output,block"`
	// Use hcl.Body to capture any unparsed content if needed, though for a full schema, it might be empty
	// UnparsedBody hcl.Body `hcl:",remain"`
}

// TerraformBlock represents the top-level 'terraform' block
type TerraformBlock struct {
	RequiredVersion   cty.Value               `hcl:"required_version,attr"` // Can be an expression or string
	RequiredProviders *RequiredProvidersBlock `hcl:"required_providers,block"`
}

// RequiredProvidersBlock represents the nested 'required_providers' block
type RequiredProvidersBlock struct {
	AWS *ProviderRequirement `hcl:"aws,block"` // Assumes 'aws' is a direct block within required_providers
}

// ProviderRequirement represents the details for a required provider (e.g., 'aws = {...}')
type ProviderRequirement struct {
	Source  cty.Value `hcl:"source,attr"`
	Version cty.Value `hcl:"version,attr"`
}

// ProviderBlock represents a 'provider' block
type ProviderBlock struct {
	Name   string    `hcl:",label"` // "aws"
	Region cty.Value `hcl:"region,attr"`
	// Add other provider-specific attributes as cty.Value if present
}

// VariableBlock represents a 'variable' block
type VariableBlock struct {
	Name        string    `hcl:",label"` // "aws_region", "instance_type" etc.
	Description cty.Value `hcl:"description,attr"`
	Type        cty.Value `hcl:"type,attr"`
	Default     cty.Value `hcl:"default,attr"` // Can be any type
}

// DataBlock represents a 'data' block
type DataBlock struct {
	Type        string        `hcl:",label"` // "aws_ami", "aws_vpc", etc.
	Name        string        `hcl:",label"` // "amazon_linux", "default"
	MostRecent  cty.Value     `hcl:"most_recent,attr"`
	Owners      cty.Value     `hcl:"owners,attr"`  // Should be []string in HCL, use cty.Value
	Filters     []FilterBlock `hcl:"filter,block"` // Nested block
	DefaultBool cty.Value     `hcl:"default,attr"` // For data "aws_vpc" "default" { default = true }
}

// FilterBlock represents a nested 'filter' block within a data source
type FilterBlock struct {
	Name   cty.Value `hcl:"name,attr"`
	Values cty.Value `hcl:"values,attr"` // Should be []string in HCL, use cty.Value
}

// ResourceBlock represents a 'resource' block. This struct will be dynamically mapped.
// We'll capture its raw body and then parse it based on its type.
type ResourceBlock struct {
	Type         string   `hcl:",label"`  // e.g., "aws_security_group"
	Name         string   `hcl:",label"`  // e.g., "web_sg"
	ResourceBody hcl.Body `hcl:",remain"` // Captures the entire body of the resource block
}

// Define specific structs for each resource type's internal body structure
// (These are "inner" structs that will be populated after we identify the resource type)

// AWSSecurityGroupBody represents the attributes and nested blocks of an aws_security_group
type AWSSecurityGroupBody struct {
	Name        cty.Value      `hcl:"name,attr"`
	Description cty.Value      `hcl:"description,attr"`
	VPCID       cty.Value      `hcl:"vpc_id,attr"`
	Ingress     []IngressBlock `hcl:"ingress,block"`
	Egress      []EgressBlock  `hcl:"egress,block"`
	Tags        cty.Value      `hcl:"tags,attr"`
}

// IngressBlock represents an 'ingress' block within a security group
type IngressBlock struct {
	Description cty.Value `hcl:"description,attr"`
	FromPort    cty.Value `hcl:"from_port,attr"`
	ToPort      cty.Value `hcl:"to_port,attr"`
	Protocol    cty.Value `hcl:"protocol,attr"`
	CIDRBlocks  cty.Value `hcl:"cidr_blocks,attr"`
}

// EgressBlock represents an 'egress' block within a security group (same structure as Ingress)
type EgressBlock IngressBlock

// AWSECSInstanceBody represents the attributes and nested blocks of an aws_instance
type AWSECSInstanceBody struct {
	AMI                      cty.Value             `hcl:"ami,attr"`
	InstanceType             cty.Value             `hcl:"instance_type,attr"`
	KeyName                  cty.Value             `hcl:"key_name,attr"`
	SubnetID                 cty.Value             `hcl:"subnet_id,attr"`
	VPCSecurityGroupIDs      cty.Value             `hcl:"vpc_security_group_ids,attr"`
	AssociatePublicIPAddress cty.Value             `hcl:"associate_public_ip_address,attr"`
	UserData                 cty.Value             `hcl:"user_data,attr"`
	RootBlockDevice          *RootBlockDeviceBlock `hcl:"root_block_device,block"` // Pointer because it's optional and singular
	MetadataOptions          *MetadataOptionsBlock `hcl:"metadata_options,block"`  // Pointer because it's optional and singular
	Tags                     cty.Value             `hcl:"tags,attr"`
}

// RootBlockDeviceBlock represents a 'root_block_device' block within an aws_instance
type RootBlockDeviceBlock struct {
	VolumeType          cty.Value `hcl:"volume_type,attr"`
	VolumeSize          cty.Value `hcl:"volume_size,attr"`
	DeleteOnTermination cty.Value `hcl:"delete_on_termination,attr"`
	Encrypted           cty.Value `hcl:"encrypted,attr"`
}

// MetadataOptionsBlock represents a 'metadata_options' block within an aws_instance
type MetadataOptionsBlock struct {
	HTTPEndpoint cty.Value `hcl:"http_endpoint,attr"`
	HTTPTokens   cty.Value `hcl:"http_tokens,attr"`
}

// AWSEIPBody represents the attributes and nested blocks of an aws_eip
type AWSEIPBody struct {
	Instance cty.Value `hcl:"instance,attr"`
	Domain   cty.Value `hcl:"domain,attr"`
	Tags     cty.Value `hcl:"tags,attr"`
}

// OutputBlock represents an 'output' block
type OutputBlock struct {
	Name        string    `hcl:",label"` // "instance_id", "instance_public_ip" etc.
	Description cty.Value `hcl:"description,attr"`
	Value       cty.Value `hcl:"value,attr"` // Can be any type
}

// --- The main parsing function ---

// extractTerraformConfigFromHCL parses the entire HCL body into structured Go types.
func extractTerraformConfigFromHCL(body hcl.Body) (*TerraformConfig, error) {
	var config TerraformConfig
	// nil context for now, means expressions won't be fully evaluated
	_ = gohcl.DecodeBody(body, nil, &config)
	fmt.Printf("%#v", config)
	//if diags.HasErrors() {
	//	return nil, fmt.Errorf("failed to decode HCL body: %s", diags.Error())
	//}
	return &config, nil
}

// processExtractedResources takes the raw ResourceBlocks and decodes their specific bodies
func processExtractedResources(config *TerraformConfig) ([]Resource, error) {
	var parsedResources []Resource

	for _, rBlock := range config.Resources {
		parsedRes := Resource{
			Mode:       "managed", // Default for resources
			Type:       rBlock.Type,
			Name:       rBlock.Name,
			Attributes: make(map[string]interface{}),
		}

		var diags hcl.Diagnostics
		switch rBlock.Type {
		case "aws_security_group":
			var sgBody AWSSecurityGroupBody
			diags = gohcl.DecodeBody(rBlock.ResourceBody, nil, &sgBody)
			if !diags.HasErrors() {
				// Convert cty.Value to Go's native types for storage in map[string]interface{}
				parsedRes.Attributes["name"] = getStringValue(sgBody.Name)
				parsedRes.Attributes["description"] = getStringValue(sgBody.Description)
				parsedRes.Attributes["vpc_id"] = getStringValue(sgBody.VPCID)
				parsedRes.Attributes["tags"] = getMapStringValue(sgBody.Tags)

				var ingressRules []map[string]interface{}
				for _, ing := range sgBody.Ingress {
					ingressRules = append(ingressRules, map[string]interface{}{
						"description": getStringValue(ing.Description),
						"from_port":   getIntValue(ing.FromPort),
						"to_port":     getIntValue(ing.ToPort),
						"protocol":    getStringValue(ing.Protocol),
						"cidr_blocks": getListStringValue(ing.CIDRBlocks),
					})
				}
				parsedRes.Attributes["ingress"] = ingressRules

				var egressRules []map[string]interface{}
				for _, eg := range sgBody.Egress {
					egressRules = append(egressRules, map[string]interface{}{
						"description": getStringValue(eg.Description),
						"from_port":   getIntValue(eg.FromPort),
						"to_port":     getIntValue(eg.ToPort),
						"protocol":    getStringValue(eg.Protocol),
						"cidr_blocks": getListStringValue(eg.CIDRBlocks),
					})
				}
				parsedRes.Attributes["egress"] = egressRules
			}
		case "aws_instance":
			var ec2Body AWSECSInstanceBody
			diags = gohcl.DecodeBody(rBlock.ResourceBody, nil, &ec2Body)
			if !diags.HasErrors() {
				parsedRes.Attributes["ami"] = getStringValue(ec2Body.AMI)
				parsedRes.Attributes["instance_type"] = getStringValue(ec2Body.InstanceType)
				parsedRes.Attributes["key_name"] = getStringValue(ec2Body.KeyName)
				parsedRes.Attributes["subnet_id"] = getStringValue(ec2Body.SubnetID)
				parsedRes.Attributes["vpc_security_group_ids"] = getListStringValue(ec2Body.VPCSecurityGroupIDs)
				parsedRes.Attributes["associate_public_ip_address"] = getBoolValue(ec2Body.AssociatePublicIPAddress)
				parsedRes.Attributes["user_data"] = getStringValue(ec2Body.UserData)
				parsedRes.Attributes["tags"] = getMapStringValue(ec2Body.Tags)

				if ec2Body.RootBlockDevice != nil {
					parsedRes.Attributes["root_block_device"] = map[string]interface{}{
						"volume_type":           getStringValue(ec2Body.RootBlockDevice.VolumeType),
						"volume_size":           getIntValue(ec2Body.RootBlockDevice.VolumeSize),
						"delete_on_termination": getBoolValue(ec2Body.RootBlockDevice.DeleteOnTermination),
						"encrypted":             getBoolValue(ec2Body.RootBlockDevice.Encrypted),
					}
				}
				if ec2Body.MetadataOptions != nil {
					parsedRes.Attributes["metadata_options"] = map[string]interface{}{
						"http_endpoint": getStringValue(ec2Body.MetadataOptions.HTTPEndpoint),
						"http_tokens":   getStringValue(ec2Body.MetadataOptions.HTTPTokens),
					}
				}
			}
		case "aws_eip":
			var eipBody AWSEIPBody
			diags = gohcl.DecodeBody(rBlock.ResourceBody, nil, &eipBody)
			if !diags.HasErrors() {
				parsedRes.Attributes["instance"] = getStringValue(eipBody.Instance)
				parsedRes.Attributes["domain"] = getStringValue(eipBody.Domain)
				parsedRes.Attributes["tags"] = getMapStringValue(eipBody.Tags)
			}
		default:
			fmt.Printf("Warning: No specific body schema defined for resource type '%s'. Capturing raw attributes.\n", rBlock.Type)
			// Fallback: get just attributes if no specific schema
			attrs, bodyDiags := rBlock.ResourceBody.JustAttributes()
			if bodyDiags.HasErrors() {
				fmt.Printf("  Error extracting direct attributes for %s.%s: %s\n", rBlock.Type, rBlock.Name, bodyDiags.Error())
			} else {
				for name, attr := range attrs {
					val, exprDiags := attr.Expr.Value(nil)
					if !exprDiags.HasErrors() {
						parsedRes.Attributes[name] = convertCtyValue(val)
					}
				}
			}
		}

		if diags.HasErrors() {
			fmt.Printf("  Error decoding body of resource %s.%s: %s\n", rBlock.Type, rBlock.Name, diags.Error())
			// Still add partially parsed resource
		}
		parsedResources = append(parsedResources, parsedRes)
	}
	fmt.Printf("%#v", parsedResources)
	return parsedResources, nil
}

// --- Helper functions for converting cty.Value to Go native types ---
// These helpers gracefully handle nil cty.Value and expressions by returning the raw string.

func getStringValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}
	if val.Type().IsPrimitiveType() && val.Type() == cty.String {
		return val.AsString()
	}
	// Fallback for expressions or complex types that aren't evaluated
	return ""
}

func getIntValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}
	if val.Type().IsPrimitiveType() && val.Type() == cty.Number {
		f := val.AsBigFloat()
		return f.String() // Return string if not exact int64
	}
	return ""
}

func getBoolValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}
	if val.Type().IsPrimitiveType() && val.Type() == cty.Bool {
		return val.True()
	}
	return ""
}

func getListStringValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}
	if (val.Type().IsListType() || val.Type().IsSetType() || val.Type().IsTupleType()) && val.Type().ElementType() == cty.String {
		var s []string
		for it := val.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			s = append(s, elem.AsString())
		}
		return s
	}
	return ""
}

func getMapStringValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}
	if (val.Type().IsMapType() || val.Type().IsObjectType()) && val.Type().ElementType() == cty.String {
		m := make(map[string]string)
		for it := val.ElementIterator(); it.Next(); {
			key, elem := it.Element()
			m[key.AsString()] = elem.AsString()
		}
		return m
	}
	// For maps containing expressions, or if not a map of strings, return raw string
	return ""
}

// A generic converter for cty.Value to interface{}, handling primitive and complex types
// If evaluation fails, it stores the raw HCL expression string.
func convertCtyValue(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}

	if val.Type().IsPrimitiveType() {
		switch val.Type() {
		case cty.String:
			return val.AsString()
		case cty.Number:
			f := val.AsBigFloat()
			return f.String() // Return string if not exact int64
		case cty.Bool:
			return val.True()
		}
	} else if val.Type().IsListType() || val.Type().IsSetType() || val.Type().IsTupleType() {
		var result []interface{}
		for it := val.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			result = append(result, convertCtyValue(elem)) // Recurse for nested structures
		}
		return result
	} else if val.Type().IsMapType() || val.Type().IsObjectType() {
		result := make(map[string]interface{})
		for it := val.ElementIterator(); it.Next(); {
			key, elem := it.Element()
			result[key.AsString()] = convertCtyValue(elem) // Recurse for nested structures
		}
		return result
	}

	// Fallback: If it's an expression or a complex type that couldn't be converted
	return ""
}

// --- Main function to demonstrate usage ---

func main() {
	hclFilePath := "main.tf" // Or your actual file path like "assets/terraform_ec2_config.hcl"
	hclFileContent, err := ioutil.ReadFile(hclFilePath)
	if err != nil {
		fmt.Printf("Error reading HCL file '%s': %s\n", hclFilePath, err)
		os.Exit(1)
	}

	parser := hclparse.NewParser()
	f, diags := parser.ParseHCL(hclFileContent, hclFilePath)
	if diags.HasErrors() {
		fmt.Printf("Error parsing HCL from '%s': %s\n", hclFilePath, diags.Error())
		os.Exit(1)
	}

	// Step 1: Extract the top-level configuration into the TerraformConfig struct
	config, err := extractTerraformConfigFromHCL(f.Body)
	if err != nil {
		fmt.Printf("Error extracting Terraform config: %s\n", err)
		os.Exit(1)
	}

	// Step 2: Process the extracted resource blocks to get their specific attributes
	parsedResources, err := processExtractedResources(config)
	if err != nil {
		fmt.Printf("Error processing resources: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Parsed Terraform Configuration ---")

	// Print Terraform Block
	if config.Terraform != nil {
		fmt.Println("Terraform Block:")
		fmt.Printf("  Required Version: %s\n", getStringValue(config.Terraform.RequiredVersion))
		if config.Terraform.RequiredProviders != nil && config.Terraform.RequiredProviders.AWS != nil {
			fmt.Printf("    AWS Provider: Source=%s, Version=%s\n",
				getStringValue(config.Terraform.RequiredProviders.AWS.Source),
				getStringValue(config.Terraform.RequiredProviders.AWS.Version))
		}
	}

	// Print Providers
	fmt.Println("\nProviders:")
	for _, p := range config.Providers {
		fmt.Printf("  Provider Name: %s, Region: %s\n", p.Name, getStringValue(p.Region))
	}

	// Print Variables
	fmt.Println("\nVariables:")
	for _, v := range config.Variables {
		fmt.Printf("  Variable Name: %s\n", v.Name)
		fmt.Printf("    Description: %s\n", getStringValue(v.Description))
		fmt.Printf("    Type: %s\n", getStringValue(v.Type))
		fmt.Printf("    Default: %v\n", convertCtyValue(v.Default)) // Use generic converter
	}

	// Print Data Sources
	fmt.Println("\nData Sources:")
	for _, d := range config.Data {
		fmt.Printf("  Data Source Type: %s, Name: %s\n", d.Type, d.Name)
		fmt.Printf("    Most Recent: %v\n", getBoolValue(d.MostRecent))
		fmt.Printf("    Owners: %v\n", getListStringValue(d.Owners))
		for _, f := range d.Filters {
			fmt.Printf("    Filter: Name=%s, Values=%v\n", getStringValue(f.Name), getListStringValue(f.Values))
		}
		if d.Type == "aws_vpc" && d.Name == "default" {
			fmt.Printf("    Default VPC: %v\n", getBoolValue(d.DefaultBool))
		}
	}

	// Print Resources (using the processedParsedResources)
	fmt.Println("\nResources:")
	for _, res := range parsedResources {
		fmt.Printf("  Resource: %s.%s (Mode: %s)\n", res.Type, res.Name, res.Mode)
		fmt.Println("    Attributes:")
		// Use the improved printInterfaceMap for cleaner output of attributes
		printInterfaceMap(res.Attributes, "      ")
	}

	// Print Outputs
	fmt.Println("\nOutputs:")
	for _, o := range config.Outputs {
		fmt.Printf("  Output Name: %s\n", o.Name)
		fmt.Printf("    Description: %s\n", getStringValue(o.Description))
		fmt.Printf("    Value: %v\n", convertCtyValue(o.Value)) // Use generic converter
	}
	fmt.Println("---------------------------------------")
}

// Helper function to print map[string]interface{} for nested structures
func printInterfaceMap(m map[string]interface{}, indent string) {
	fmt.Println("{")
	for k, v := range m {
		fmt.Printf("%s  %s: ", indent, k)
		switch val := v.(type) {
		case cty.Value:
			// This case should ideally not be hit if convertCtyValue works as expected,
			// but serves as a fallback.
			if val.IsKnown() {
				if val.Type().IsPrimitiveType() {
					fmt.Printf("%v (%s)\n", val.GoString(), val.Type().FriendlyName())
				} else {
					fmt.Printf("%v (cty complex type %s)\n", val.GoString(), val.Type().FriendlyName())
				}
			} else if val.IsNull() {
				fmt.Println("null")
			} else {
				fmt.Println("<unknown/unevaluated cty expression>")
			}
		case string:
			fmt.Printf("'%s' (raw HCL expression)\n", val)
		case []interface{}: // For lists of nested blocks or complex lists
			printInterfaceList(val, indent+"  ")
		case map[string]interface{}: // For nested blocks like root_block_device, metadata_options
			printInterfaceMap(val, indent+"  ")
		default:
			fmt.Printf("%v (Go type: %s)\n", val, reflect.TypeOf(val))
		}
	}
	fmt.Printf("%s}\n", indent)
}

// Helper function to print []interface{} for nested structures
func printInterfaceList(s []interface{}, indent string) {
	fmt.Println("[")
	for _, v := range s {
		fmt.Printf("%s  - ", indent)
		switch val := v.(type) {
		case cty.Value:
			if val.IsKnown() {
				if val.Type().IsPrimitiveType() {
					fmt.Printf("%v (%s)\n", val.GoString(), val.Type().FriendlyName())
				} else {
					fmt.Printf("%v (cty complex type %s)\n", val.GoString(), val.Type().FriendlyName())
				}
			} else if val.IsNull() {
				fmt.Println("null")
			} else {
				fmt.Println("<unknown/unevaluated cty expression>")
			}
		case string:
			fmt.Printf("'%s' (raw HCL expression)\n", val)
		case []interface{}:
			printInterfaceList(val, indent+"  ")
		case map[string]interface{}:
			printInterfaceMap(val, indent+"  ")
		default:
			fmt.Printf("%v (Go type: %s)\n", val, reflect.TypeOf(val))
		}
	}
	fmt.Printf("%s]\n", indent)
}
