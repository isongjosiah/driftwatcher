package terraform

// TerraformState represents a Terraform state file structure
type TerraformState struct {
	Version          int            `json:"version"`
	TerraformVersion string         `json:"terraform_version"`
	Serial           int            `json:"serial"`
	Lineage          string         `json:"lineage"`
	Outputs          map[string]any `json:"outputs"`
	Resources        []Resource     `json:"resources"`
}

// Resource represents a resource in the Terraform state
type Resource struct {
	Mode         string `json:"mode"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Attributes   map[string]any
	Provider     string     `json:"provider"`
	Instances    []Instance `json:"instances"`
	Dependencies []string   `json:"depends_on,omitempty"`
}

// Instance represents an instance of a resource
type Instance struct {
	SchemaVersion int                `json:"schema_version"`
	Attributes    InstanceAttributes `json:"attributes"`
	Dependencies  []string           `json:"dependencies,omitempty"`
}

type InstanceAttributes struct {
	AMI                       string                   `json:"ami"`
	ARN                       string                   `json:"arn"`
	AssociatePublicIPAddress  bool                     `json:"associate_public_ip_address"`
	AvailabilityZone          string                   `json:"availability_zone"`
	CPUCoreCount              int                      `json:"cpu_core_count"`
	CPUThreadsPerCore         int                      `json:"cpu_threads_per_core"`
	CreditSpecification       []map[string]interface{} `json:"credit_specification"`
	DisableAPITermination     bool                     `json:"disable_api_termination"`
	EBSBlockDevice            []map[string]interface{} `json:"ebs_block_device"`
	EBSOptimized              bool                     `json:"ebs_optimized"`
	EphemeralBlockDevice      []map[string]interface{} `json:"ephemeral_block_device"`
	GetPasswordData           bool                     `json:"get_password_data"`
	Hibernation               bool                     `json:"hibernation"`
	HostID                    interface{}              `json:"host_id"`
	IAMInstanceProfile        string                   `json:"iam_instance_profile"`
	ID                        string                   `json:"id"`
	InstanceInitiatedShutdown string                   `json:"instance_initiated_shutdown_behavior"`
	InstanceState             string                   `json:"instance_state"`
	InstanceType              string                   `json:"instance_type"`
	IPv6AddressCount          int                      `json:"ipv6_address_count"`
	IPv6Addresses             []interface{}            `json:"ipv6_addresses"`
	KeyName                   string                   `json:"key_name"`
	MetadataOptions           []map[string]interface{} `json:"metadata_options"`
	Monitoring                bool                     `json:"monitoring"`
	NetworkInterface          []map[string]interface{} `json:"network_interface"`
	OutpostARN                string                   `json:"outpost_arn"`
	PasswordData              string                   `json:"password_data"`
	PlacementGroup            string                   `json:"placement_group"`
	PrimaryNetworkInterfaceID string                   `json:"primary_network_interface_id"`
	PrivateDNS                string                   `json:"private_dns"`
	PrivateIP                 string                   `json:"private_ip"`
	PublicDNS                 string                   `json:"public_dns"`
	PublicIP                  string                   `json:"public_ip"`
	RootBlockDevice           []map[string]interface{} `json:"root_block_device"`
	SecondaryPrivateIPs       []interface{}            `json:"secondary_private_ips"`
	SecurityGroups            []interface{}            `json:"security_groups"`
	SourceDestCheck           bool                     `json:"source_dest_check"`
	SubnetID                  string                   `json:"subnet_id"`
	Tags                      map[string]string        `json:"tags"`
	TagsAll                   map[string]string        `json:"tags_all"`
	Tenancy                   string                   `json:"tenancy"`
	Timeouts                  interface{}              `json:"timeouts"`
	UserData                  interface{}              `json:"user_data"`
	UserDataBase64            interface{}              `json:"user_data_base64"`
	VolumeTags                interface{}              `json:"volume_tags"`
	VPCSecurityGroupIDs       []string                 `json:"vpc_security_group_ids"`
}

// ParsedTerraform represents the result of parsing either format
type ParsedTerraform struct {
	Type      string // "state" or "hcl"
	FilePath  string // path to the parsed file
	State     *TerraformState
	HCLConfig TerraformConfig
}
