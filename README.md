# ssh Terraform provider

- Documentation: https://registry.terraform.io/providers/loafoe/ssh/latest/docs

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

## Overview

This is a terraform provider to copy files and run commands remotely over SSH. Unlike the Terraform provisioners which are described as
 a "last resort" this provider embraces the concept of pushing and executing content to compute instances over SSH. Apart from bastion
 hosts it also supports tunneling over HTTP proxies. This is very useful if you are running Terraform from inside a corporate
 network and need to reach out to your instances.

# Using the provider

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

## Development requirements

-	[Terraform](https://www.terraform.io/downloads.html) 0.14.x
-	[Go](https://golang.org/doc/install) 1.15 or newer (to build the provider plugin)

## Issues

- If you have an issue: report it on the [issue tracker](https://github.com/loafoe/terraform-provider-ssh/issues)

## LICENSE

License is MIT
