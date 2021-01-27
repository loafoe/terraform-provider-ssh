# ssh_resource
Supports copying and running commands over an
SSH connection.

The following example uses the internal provisioning support for bootstrapping an instance

```hcl
resource "ssh_resource" "init" {
  host = "private-ec2.instance.com"
  bastion_host = "bastion.host.com"
  user = var.user
  private_key = var.private_key

  file {
    content = "echo Hello world"
    destination = "/tmp/hello.sh"
  }
  
  commands = [
    "chmod +x /tmp/hello.sh",
    "/tmp/hello.sh"
  ]
}
```

## Argument Reference

The following arguments are supported:

* `user` - (Required) The username to use for provision activities using SSH
* `private_key` - (Required) The SSH private key to use for provision activities
* `file` - (Optional) Block specifying content to be written to the container host after creation
* `commands` - (Required, list(string)) List of commands to execute after creation of container host
* `bastion_host` - (Optional) The bastion host to use.  When not set, this will be deduced from the container host location
* `triggers` - (Optional, list(string)) An list of strings which when changes will trigger recreation of the resource triggering
  all create files and commands executions.

Each `file` block can contain the following fields. Use either `content` or `source`:

* `source` - (Optional, file path) Content of the file. Conflicts with `content`
* `content` - (Optional, string) Content of the file. Conflicts with `source`
* `destination` - (Required, string) Remote filename to store the content in

## Attributes Reference

The following attributes are exported:

* `id` - The resource ID
