# ssh Terraform provider

- Documentation: https://registry.terraform.io/providers/loafoe/ssh/latest/docs

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

## Overview

This is a terraform provider to copy files and run commands remotely over SSH. Unlike the Terraform provisioners which are described as
 a "last resort" this provider embraces the concept of pushing and executing content to compute instances over SSH. Apart from bastion
 hosts it also supports tunneling over HTTP proxies. This is very useful if you are running Terraform from inside a corporate
 network and need to reach out to your instances.

## Using the provider

**Terraform 0.14**: To install this provider, copy and paste this code into your Terraform configuration. Then, run terraform init.

```terraform
terraform {
  required_providers {
    ssh = {
      source = "loafoe/ssh"
    }
  }
}
```
## Example

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

## Development requirements

-	[Terraform](https://www.terraform.io/downloads.html) 0.15.0 or newer
-	[Go](https://golang.org/doc/install) 1.17 or newer (to build the provider plugin)

## Issues

- If you have an issue: report it on the [issue tracker](https://github.com/loafoe/terraform-provider-ssh/issues)

## LICENSE

License is MIT
