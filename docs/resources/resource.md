# ssh_resource

Supports copying and running commands over an
SSH connection.

The following example uses the internal provisioning support for bootstrapping an instance

```hcl
resource "ssh_resource" "init" {
  # The default behaviour is to run file blocks and commands at create time
  # You can also specify 'destroy' to run the commands at destroy time
  when = "create"
  
  host         = "private-ec2.instance.com"
  bastion_host = "bastion.host.com"
  user         = var.user
  host_user    = var.host_user
  agent        = true
  # An ssh-agent with your SSH private keys should be running
  # Use 'private_key' to set the SSH key otherwise
  
  file {
    content     = "echo Hello world"
    destination = "/tmp/hello.sh"
    permissions = "0700"
  }
  
  timeout = "15m"
  
  commands = [
    "/tmp/hello.sh"
  ]
}
```

## Argument Reference

The following arguments are supported:

* `host` - (Required) The IP address or DNS hostname of the target server
* `user` - (Required) The username to use for provision activities using SSH
* `when` - (Optional) Determines when the file blocks and commands are executed. Options are `create` or `destroy`. Default: `"create"`
* `host_user` - (Optional) A distinct username to use for provision activities when provided. When missing the provided `user` is used
* `private_key` - (Optional) The SSH private key to use for provision activities. Recommend to use ssh-agent
* `host_private_key` - (Optional) A distinct SSH private key to use for provision activities when provided. Recommend to use ssh-agent
* `port` - (Optional) The SSH port to use on the target server. Default: `"22"`
* `agent` - (Optional) Enforce the use of an SSH-agent. When set, will error in case a private key is provided. Default: 'false'
* `file` - (Optional) Block specifying content to be written to the container host after creation
* `commands` - (Required, list(string)) List of commands to execute after creation of container host
* `pre_commands` - (Optional) Similar to `commands` but these are run before files are copied. This is useful if you need to prepare
  the host in some way before you can copy content over e.g. create some directories
* `bastion_host` - (Optional) The bastion host to use.  When not set, this will be deduced from the container host location
* `bastion_port` - (Optional) The SSH port to use on the bastion host. Default: `"22"`
* `triggers` - (Optional, list(string)) An list of strings which when changes will trigger recreation of the resource triggering
  all create files and commands executions.
* `commands_after_file_changes` - (Optional, bool) Re-run all commands after file changes. Default is `true`.
* `timeout` - (Optional) Time to wait before considering a command to have timed out. Default is `5m`. Accept seconds (e.g. `180s`) or minutes (e.g. `30m`)

Each `file` block can contain the following fields. Use either `content` or `source`:

* `source` - (Optional, file path) Content of the file. Conflicts with `content`
* `content` - (Optional, string) Content of the file. Conflicts with `source`
* `destination` - (Required, string) Remote filename to store the content in
* `permissions` - (Optional, string) The file permissions. Default permissions are "0644"
* `owner` - (Optional, string) The file owner. Default owner the SSH user
* `group` - (Optional, string) The file group. Default group is the SSH user's group

## Attributes Reference

The following attributes are exported:

* `id` - The resource ID
* `result` - The stdout of the last executed command
