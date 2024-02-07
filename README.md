# SSH Terraform provider

- Documentation: https://registry.terraform.io/providers/loafoe/ssh/latest/docs

## Overview

This is a Terraform provider to copy files, run commands remotely and capture results via SSH. Unlike the Terraform provisioners which are described as
 a "last resort" this provider embraces the concept of pushing and executing content to compute instances over SSH. Apart from bastion
 hosts it also supports tunneling over HTTP proxies. This is very useful if you are running Terraform from inside a corporate
 network and need to reach out to your instances.

## Using the provider

To install this provider, copy and paste this code into your Terraform configuration. Then, run terraform init.

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

  timeout = "3m"

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

### With triggers

```hcl
resource "ssh_resource" "deploy-kubevip" {
  count = local.deploy_kubevip == true ? 1 : 0

  triggers = {
    template = data.template_file.kubevip_config[count.index].rendered
  }

  file {
    content     = data.template_file.kubevip_config[count.index].rendered
    destination = "/tmp/deploy-kubevip.sh"
    permissions = "0644"
  }

  commands = [
    "chmod +x /tmp/deploy-kubevip.sh",
    "sudo /tmp/deploy-kubevip.sh"
  ]

  user        = local.ssh_user_server
  private_key = local.ssh_key_server
  host        = local.servernodes.0
}
```

## Development requirements

-	[Terraform](https://www.terraform.io/downloads.html) 1.3.0 or newer
-	[Go](https://golang.org/doc/install) 1.21 or newer (to build the provider plugin)

## Issues

- If you have an issue: report it on the [issue tracker](https://github.com/loafoe/terraform-provider-ssh/issues)

## LICENSE

License is MIT
