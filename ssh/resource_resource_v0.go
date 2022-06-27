package ssh

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Upgrades a SSH resource from v0 to v1
func patchResourceV0(_ context.Context, rawState map[string]interface{}, _ interface{}) (map[string]interface{}, error) {
	if rawState == nil {
		rawState = map[string]interface{}{}
	}
	rawState["when"] = "create"
	return rawState, nil
}

func resourceResourceV0() *schema.Resource {
	return &schema.Resource{
		// This is only used for state migration, so the CRUD
		// callbacks are no longer relevant
		Schema: map[string]*schema.Schema{
			"triggers": {
				Description: "A map of arbitrary strings that, when changed, will force the 'hsdp_container_host_exec' resource to be replaced, re-running any associated commands.",
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
			},
			"host": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"port": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "22",
			},
			"bastion_host": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"bastion_port": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "22",
			},
			"user": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"host_user": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"private_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"host_private_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"agent": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"commands": {
				Type:     schema.TypeList,
				MaxItems: 100,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"commands_after_file_changes": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"timeout": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "5m",
			},
			"result": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"file": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"source": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"content": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"destination": {
							Type:     schema.TypeString,
							Required: true,
						},
						"permissions": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"owner": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"group": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}
