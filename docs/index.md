# ssh provider

Provides a resource which allows you to copy, run commands and capture output
via SSH. It supports Bastion host connection and HTTP proxies,
so it can work from behind corporate networks as well.

Typically, this resource is used in place of a `null_provider` or instead
of the Terraform `remote-exec` provisioner where you are in a firewalled
environment and need to use a HTTP proxy to punch through.

## Example usage

```hcl
resource "ssh_resource" "example" {
  host         = "remote-server.test"
  bastion_host = "jumpgate.remote-host.com"
  user         = "alpine"
  agent        = true

  file {
    content     = "echo '{\"hello\":\"world\"}' && exit 0"
    destination = "/home/alpine/test.sh"
    permissions = "0700"
  }

  commands = [
    "/home/alpine/test.sh",
  ]
}

output "result" {
  value = try(jsondecode(ssh_resource.example.result), {})
}
```

The above example snippet uploads a generated shell script, executes it remotely and captures the
output for further use in Terraform.

## Argument Reference

The following arguments are supported:

* `debug_log` - (Optional, filename) Write debugging info to this file
