---
title: Installation Methods
---

RKE2 can be installed to a system in a number of ways, 2 of which being the preferred and supported methods. Those methods are tarball and RPM. Both of which are wrappers for the provided install script.

### Tarball

#### Structure 

* bin - contains the RKE2 executable as well as the `rke2-killall.sh` and `rke2-uninstall.sh` scripts
* lib - contains server and agent systemd unit files
* share - contains the RKE2 license as well as a sysctl configuration file used for when RKE2 is ran in CIS mode

The tarball extracts its contents to `/usr/local` by default.

#### Installation

```bash

```

### RPM

#### Repositories

Signed RPMs are published for RKE2 within the `rpm-testing.rancher.io` and `rpm.rancher.io` RPM repositories. If you run the https://get.rke2.io script on nodes supporting RPMs, it will use these RPM rpeos by default. But you can also install them yourself.

The RPMs provide `systemd` units for managing `rke2`, but will need to be configured via configuration file before starting the services for the first time.

#### Enterprise Linux 7

In order to use the RPM repository, on a CentOS 7 or RHEL 7 system, run the following bash snippet:

```bash
cat << EOF > /etc/yum.repos.d/rancher-rke2-1-18-latest.repo
[rancher-rke2-common-latest]
name=Rancher RKE2 Common Latest
baseurl=https://rpm.rancher.io/rke2/latest/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key

[rancher-rke2-1-18-latest]
name=Rancher RKE2 1.18 Latest
baseurl=https://rpm.rancher.io/rke2/latest/1.18/centos/7/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
```

#### Enterprise Linux 8

In order to use the RPM repository, on a CentOS 8 or RHEL 8 system, run the following bash snippet:

```bash
cat << EOF > /etc/yum.repos.d/rancher-rke2-1-18-latest.repo
[rancher-rke2-common-latest]
name=Rancher RKE2 Common Latest
baseurl=https://rpm.rancher.io/rke2/latest/common/centos/8/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key

[rancher-rke2-1-18-latest]
name=Rancher RKE2 1.18 Latest
baseurl=https://rpm.rancher.io/rke2/latest/1.18/centos/8/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
```

#### Installation

After the repository is configured, you can run either of the following commands:

```bash
yum -y install rke2-server
```
or

```bash
yum -y install rke2-agent
```

The RPM will install a corresponding `rke2-server.service` or `rke2-agent.service` systemd unit that can be invoked like: `systemctl start rke2-server`. Make sure that you configure `rke2` before you start it, by following the `Configuration File` instructions below.

### Manual

The RKE2 binary is statically compiled and linked which allows for the RKE2 binary to be portable across Linux distributions without the concern for dependency issues. The simplest installation is to download the binary, make sure it's executable, and copy it into the `${PATH}`, generally `/usr/local/bin`. After first execution, RKE2 will create all necessary directories and files. To configure the system any further, you'll want to reference the [config file](install_options/install_options.md) documentation.
