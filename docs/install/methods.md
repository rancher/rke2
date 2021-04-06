---
title: Installation Methods
---

**Important:** If your node has NetworkManager installed and enabled, [ensure that it is configured to ignore CNI-managed interfaces.](https://docs.rke2.io/known_issues/#networkmanager)

RKE2 can be installed to a system in a number of ways, two of which are the preferred and supported methods. Those methods are tarball and RPM. The install script referenced in the Quick Start is a wrapper around these two methods.

This document explains these installation methods in greater detail. 

### Tarball

To install RKE2 via install you first need to get the install script. This can be done in a number of ways.

This gets the script and immediately starts the install process.

```bash
curl -sfL https://get.rke2.io | sh -
```

This will download the install script and make it executable.

```bash
curl -sfL https://get.rke2.io --output install.sh
chmod +x install.sh
```

#### Installation

The install process defaults to the latest RKE2 version and no other qualifiers are necessary. However, if you want specify a version, you should set the `INSTALL_RKE2_CHANNEL` environment variable. An example below:

```bash
INSTALL_RKE2_CHANNEL=latest ./install.sh
```

When the install script is executed, it makes a determination of what type of system it is. If it's an OS that uses RPMs (such as CentOS or RHEL), it will perform an RPM based installation, otherwise the script defaults to tarball. RPM based installation is covered below.

Next, the installation script downloads the tarball, verifies it by comparing SHA256 hashes, and lastly, extracts the contents to `/usr/local`. An operator is free to move the files after installation if desired. This operation simply extracts the tarball and no other system modifications are made.

Tarball structure / contents

* bin - contains the RKE2 executable as well as the `rke2-killall.sh` and `rke2-uninstall.sh` scripts
* lib - contains server and agent systemd unit files
* share - contains the RKE2 license as well as a sysctl configuration file used for when RKE2 is ran in CIS mode

To configure the system any further, you'll want to reference the either the [server](install_options/server_config.md) or [agent](install_options/agent_config.md) documentation.


### RPM

To start the RPM install process, you need to get the installation script which is covered above. The script will check your system for `rpm`, `yum`, or `dnf` and if any of those exist, it determines that the system is Redhat based and starts the RPM install process.

Files are installed with the prefix of `/usr` rather than `/usr/local`.

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
