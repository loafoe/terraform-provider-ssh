# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## v2.6.0

- Fix regression in duration parsing (#65)

## v2.5.0

- Support for private key passphrases (#61)

## v2.4.0

- Support for password authentication (#59)

## v2.3.0

- Retry SSH operations on error (#31)
- Bump SDK

## v2.2.0

- Add `ssh_sensitive_resource` (#45)

## v2.1.0

- Add `pre_commands` argument (#43)

## v2.0.1

- Always validate on create (#40)

## v2.0.0

- Add 'when' argument (#37). Thanks @arbourd

## v1.2.0

- Option to configure SSH ports to use for bastion and host server (#26)

## v1.1.0

- Add configurable command timeout (#24)

## v1.0.1

- Small refactor for better code use

## v1.0.0

- Capture result from command
- Feature complete for version 1

## v0.4.0

- Improve debug logging
- Don't ignore file copy errors

## v0.3.0

- Support for using SSH-agent (#14)
- Provider can be debugged using '--debug' flag

## v0.2.2

- Add optional `host_user` option to override host username

## v0.2.1

- Optional support different keys between bastion and host (@vikramraodp)

## v.2.0

- Update dependencies

## v0.1.1

- Documentation fix (@jeffbski-rga)

## v0.1.0

- Initial release
