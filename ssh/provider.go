package ssh

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"debug_log": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "File to write debugging info to",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"ssh_resource": resourceResource(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	config := &Config{}

	var diags diag.Diagnostics

	config.DebugLog = d.Get("debug_log").(string)

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
