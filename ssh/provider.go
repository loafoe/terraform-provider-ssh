package ssh

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
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
	return config, diags
}
