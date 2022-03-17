# 3. RPM SLE support for RKE2

Date: 2022-01-27

## Status

Accepted

## Context

RKE2 publishes RPMs for SUSE OS distributions, the rpms will be installed via transactional updates if exists, this will enable two things, the installation of rke2-selinux and the extraction of the binaries in the right `/usr` paths instead of the alternative tarball installation which will extract the binaries in `/opt`.

## Decision

We will add support for RPM publishing for SUSE OS distributions in rke2-packaging repo, the `rke2-server` and `rke2-agent` packages will require installing `rke2-common` which will in turn install the `rke2-selinux` RPM package which is already supported for microos.

The decision will involve defaulting to the tarball installation for SUSE OS distribution in the installation script to prevent breaking current compatibility with users who currently installed via tarball installation, the RPM installation will be allowed via passing the environment variable `RKE2_INSTALL_METHOD=rpm` to the install script.

The installation script will also have measures to prevent installation switching from RPM to tarball installation and vice versa, and finally the installation via the tarball method will not allow SELINUX to be enabled unless manually.

## Consequences

The decision will result in some drawbacks:

- The decision will not enable RPM installation by default.
- The tarball installation will not enable SELINUX by default.