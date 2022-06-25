package ssh

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDestroyResource() *schema.Resource {
	return &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CreateContext: resourceDestroyResourceCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: resourceDestroyResourceDelete,
		Schema:        resourceSchema,
	}
}

func resourceDestroyResourceDelete(c context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	diags := resourceResourceCreate(c, d, m)
	d.SetId("")
	return diags
}

func resourceDestroyResourceCreate(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	d.SetId(fmt.Sprintf("%d", rand.Int()))
	return diags
}
