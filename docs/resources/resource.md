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

  # Try to complete in at most 15 minutes and wait 5 seconds between retries
  timeout     = "15m"
  retry_delay = "5s"
  
  file {
    content     = "echo Hello world"
    destination = "/tmp/hello.sh"
    permissions = "0700"
  }
  
  commands = [
    "/tmp/hello.sh"
  ]
}
```

This next example will force the command to run on each `terraform apply` using the triggers feature:

```hcl
resource "ssh_resource" "always_run" {
  triggers = {
    always_run = "${timestamp()}"
  }

  host         = "some.private-instance.io"
  user         = var.user
  agent        = true
  
  commands = [
     "touch /tmp/terraform_triggered"
  ]
}
```

## Argument Reference

The following arguments are supported:

Connection arguments can also be configured as defaults on the provider. Values set on the resource take precedence.

* `host` - (Optional) The IP address or DNS hostname of the target server, without a port. Defaults to the provider `host` value. Use the `port` argument for a non-default SSH port
* `user` - (Optional) The username to use for provision activities using SSH. Defaults to the provider `user` value
* `password` - (Optional) The SSH password to use for the host. Defaults to the provider `password` value
* `when` - (Optional) Determines when the file blocks and commands are executed. Options are `create` or `destroy`. Default: `"create"`
* `bastion_user` - (Optional) A distinct username to use for bastion user. Defaults to the provider `bastion_user` value, or the effective `user` when unset
* `private_key` - (Optional) The SSH private key to use for provision activities. Recommend to use ssh-agent. Defaults to the provider `private_key` value
* `bastion_private_key` - (Optional) A distinct SSH private key to use for the bastion host. Recommend to use ssh-agent. Defaults to the provider `bastion_private_key` value
* `port` - (Optional) The SSH port to use on the target server. Defaults to the provider `port` value, or `"22"` when unset
* `agent` - (Optional) Enforce the use of an SSH-agent. When set, will error in case a private key is provided. Defaults to the provider `agent` value, or `false` when unset
* `file` - (Optional) Block specifying content to be written to the container host after creation
* `commands` - (Required, list(string)) List of commands to execute after creation of container host
* `pre_commands` - (Optional) Similar to `commands` but these are run before files are copied. This is useful if you need to prepare
  the host in some way before you can copy content over e.g. create some directories
* `bastion_host` - (Optional) The IP address or DNS hostname of the bastion host, without a port. Defaults to the provider `bastion_host` value
* `bastion_port` - (Optional) The SSH port to use on the bastion host. Defaults to the provider `bastion_port` value, or `"22"` when unset
* `bastion_password` - (Optional) The SSH password to use for the bastion host. Defaults to the provider `bastion_password` value
* `triggers` - (Optional, map(string)) A map of values which, when changed, forces recreation of the resource and
  re-runs all file copies and commands. Trigger values may be derived from computed attributes of other resources and
  can be resolved during apply.
* `commands_after_file_changes` - (Optional, bool) Re-run all commands after file changes. Default is `true`.
* `timeout` - (Optional) Time to wait before considering provisioning as unsuccessful. This spans the copy and command phase. Default is `5m`. Accept seconds (e.g. `300s`) or minutes (e.g. `30m`)
* `ignore_no_supported_methods_remain` - (Optional, bool) Retry file uploads and commands when authentication fails with `no supported methods remain`. This is useful when SSH keys are provisioned asynchronously. Default is `false`, which returns the authentication error immediately
* `retry_delay` - (Optional) Time to wait before retrying an SSH operation (copy or command execution). Default is `10s`.

Each `file` block can contain the following fields. Use either `content` or `source`:

* `source` - (Optional, file path) Path to a local file to copy. Conflicts with `content`
* `content` - (Optional, string) Content of the file. Conflicts with `source`. Set to an empty string to create an empty file
* `destination` - (Required, string) Remote filename to store the content in
* `permissions` - (Optional, string) The file permissions. Default permissions are "0644"
* `owner` - (Optional, string) The file owner. Default owner the SSH user
* `group` - (Optional, string) The file group. Default group is the SSH user's group

### Passphrases on SSH private keys

The provider supports using private keys with a passphrases. However, to prevent passphrases from being stored
in Terraform state they can only be provided through the environment variables:

| Environment                        | Description                                |
|------------------------------------|--------------------------------------------|
| SSH_PRIVATE_KEY_PASSPHRASE         | Passphrase for the host target private key |
| SSH_BASTION_PRIVATE_KEY_PASSPHRASE | Passphrase for the bastion private key     |

## Attributes Reference

The following attributes are exported:

* `id` - The resource ID
* `result` - The stdout of the last executed command
