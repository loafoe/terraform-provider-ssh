package ssh_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/loafoe/terraform-provider-ssh/internal/acc"
)

func TestAccResourceResource_basic(t *testing.T) {
	t.Parallel()

	resourceName := "ssh_resource.test"
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
				Config:       testAccResourceResource(randomName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "user", username)),
			},
		},
	})
}

func testAccResourceResource(random string) string {
	username := acc.AccUsername()
	privateKey := acc.AccPrivateKey()
	hostname := acc.AccHostname()

	return fmt.Sprintf(`

resource "ssh_resource" "test" {
	host        = "%s"
    user        = "%s"
    agent       = false
    private_key = "%s"

	timeout = "5m"

	retry_delay = "2s"

    commands = [
       "date > /tmp/terraform-provider-ssh-test-%s"
    ]
}

resource "ssh_resource" "destroy" {
	host        = "%s"
    user        = "%s"
    agent       = false
    private_key = "%s"
 
    when        = "destroy"

    commands = [
        "rm /tmp/terraform-provider-ssh-test-%s"
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
