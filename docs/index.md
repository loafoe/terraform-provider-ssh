# ssh provider
Provides a resource which allows you to copy and run commands
via SSH. It supports Bastion host connection and HTTP proxyies 
so it can work from behind corporate networks as well.

Typically, this resource is used in place of a `null_provider` or instead
of the Terraform `remote-exec` provisioner where you are in a firewalled
environment and need to use a HTTP proxy to punch through.

## Argument Reference

The following arguments are supported:

* `debug_log` - (Optional, filename) Write debugging info to this file
