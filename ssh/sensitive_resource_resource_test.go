package ssh_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/loafoe/terraform-provider-ssh/internal/acc"
)

func TestAccSensitiveResourceResource_basic(t *testing.T) {
	t.Parallel()

	resourceName := "ssh_sensitive_resource.test"
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	username := acc.AccUsername()

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acc.PreCheck(t)
		},
		ProviderFactories: acc.ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: resourceName,
				Config:       testAccSensitiveResourceResource(randomName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "user", username)),
			},
		},
	})
}

func testAccSensitiveResourceResource(random string) string {
	username := acc.AccUsername()
	privateKey := acc.AccPrivateKey()
	hostname := acc.AccHostname()

	return fmt.Sprintf(`

resource "ssh_sensitive_resource" "test" {
	host        = "%s"
    user        = "%s"
    agent       = false
    private_key = "%s"

    commands = [
       "date > /tmp/%s"
    ]
}

resource "ssh_sensitive_resource" "destroy" {
	host        = "%s"
    user        = "%s"
    agent       = false
    private_key = "%s"
 
    when        = "destroy"

    commands = [
        "rm /tmp/%s"
    ]
}
`,
		// SSH Resource test
		hostname,
		username,
		privateKey,
		random,

		// SSH Resource destroy
		hostname,
		username,
		privateKey,
		random,
	)
}
