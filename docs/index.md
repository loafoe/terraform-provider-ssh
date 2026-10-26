# SSH provider

Provides a resource which allows you to copy, run commands and capture output
via SSH. It supports Bastion host connection and HTTP proxies,
so it can work from behind corporate networks as well.

Typically, this resource is used in place of a `null_provider` or instead
of the Terraform `remote-exec` provisioner where you are in a firewalled
environment and need to use a HTTP proxy to punch through.

Using the `when` attribute you can also choose to run the commands at destroy time.

## Example usage

```hcl
resource "ssh_resource" "example" {
  host         = "remote-server.test"
  bastion_host = "jumpgate.remote-host.com"
  user         = "alpine"
  agent        = true
  
  when         = "create" # Default

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

## Parallelism and dependencies

The provider does not serialize SSH operations. Terraform and OpenTofu schedule each resource
instance independently, so `depends_on` inside a module used with `for_each` is evaluated per
module instance rather than as a global barrier.

If all upstream resources take approximately the same time to finish, downstream resources may
appear to start as a group. This is normal scheduling behavior. On the first apply error,
Terraform and OpenTofu stop scheduling new work, so unrelated resource instances may not start.

Use the `-parallelism` command line option to adjust the number of concurrent operations.

## Argument Reference

The following arguments are supported:

* `debug_log` - (Optional, filename) Write debugging info to this file
