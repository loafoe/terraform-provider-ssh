package ssh

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	DebugLog = "SSH_DEBUG_LOG"
)

// Provider
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"debug_log": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "File to write debugging info to",
				DefaultFunc: schema.EnvDefaultFunc(DebugLog, ""),
			},
			"host": {
				Description:  "Default IP address or DNS hostname of the target server, without a port.",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateOptionalHost,
			},
			"port": {
				Description: "Default SSH port to use on the target server.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "22",
			},
			"user": {
				Description: "Default username to use for SSH authentication.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"password": {
				Description: "Default SSH password to use for the target server.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"private_key": {
				Description: "Default SSH private key to use for authentication.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"bastion_host": {
				Description:  "Default IP address or DNS hostname of the bastion host, without a port.",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateOptionalHost,
			},
			"bastion_port": {
				Description: "Default SSH port to use on the bastion host.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "22",
			},
			"bastion_user": {
				Description: "Default username to use for the bastion host.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"bastion_password": {
				Description: "Default SSH password to use for the bastion host.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"bastion_private_key": {
				Description: "Default SSH private key to use for the bastion host.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"agent": {
				Description: "Default whether to use an SSH agent for authentication.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"ssh_resource":           resourceResource(),
			"ssh_sensitive_resource": sensitiveResourceResource(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	config := &Config{}

	var diags diag.Diagnostics

	config.DebugLog = d.Get("debug_log").(string)
	config.Connection = SSHConnectionDefaults{
		Host:              d.Get("host").(string),
		Port:              d.Get("port").(string),
		User:              d.Get("user").(string),
		Password:          d.Get("password").(string),
		PrivateKey:        d.Get("private_key").(string),
		BastionHost:       d.Get("bastion_host").(string),
		BastionPort:       d.Get("bastion_port").(string),
		BastionUser:       d.Get("bastion_user").(string),
		BastionPassword:   d.Get("bastion_password").(string),
		BastionPrivateKey: d.Get("bastion_private_key").(string),
		Agent:             d.Get("agent").(bool),
	}

	if config.DebugLog != "" {
		debugFile, err := os.OpenFile(config.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			config.debugFile = nil
		} else {
			config.debugFile = debugFile
		}
	}

	return config, diags
}
