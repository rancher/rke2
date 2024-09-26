## Install Tests

These tests are used to validate the installation and operation of RKE2 on a variety of operating systems. The test themselves are Vagrantfiles describing single-node installations that are easily spun up with Vagrant for the `libvirt` and `virtualbox` providers:

- [Install Script](install) :arrow_right: scheduled nightly and on an install script change
  - [CentOS 9 Stream](install/centos-9)
  - [Rocky Linux 8](install/rocky-8) (stand-in for RHEL 8)
  - [Oracle 9](install/oracle-9)
  - [Leap 15.6](install/opensuse-leap) (stand-in for SLES)
  - [Ubuntu 24.04](install/ubuntu-2404)
  - [Windows Server 2019](install/windows-2019)
  - [Windows Server 2022](install/windows-2022)

## Format
When adding new installer test(s) please copy the prevalent style for the `Vagrantfile`.
Ideally, the boxes used for additional assertions will support the default `libvirt` provider which
enables them to be used by our GitHub Actions [Nightly Install Test Workflow](../../.github/workflows/nightly-install.yaml).

### Framework

If you are new to Vagrant, Hashicorp has written some pretty decent introductory tutorials and docs, see:
- https://learn.hashicorp.com/collections/vagrant/getting-started
- https://www.vagrantup.com/docs/installation

#### Plugins and Providers

The `libvirt`provider cannot be used without first [installing the `vagrant-libvirt` plugin](https://github.com/vagrant-libvirt/vagrant-libvirt). Libvirtd service must be installed and running on the host machine as well.

This can be installed with:
```shell
vagrant plugin install vagrant-libvirt
```

#### Environment Variables

These can be set on the CLI or exported before invoking Vagrant:
- `TEST_VM_CPUS` (default :arrow_right: 2)<br/>
  The number of vCPU for the guest to use.
- `TEST_VM_MEMORY` (default :arrow_right: 3072)<br/>
  The number of megabytes of memory for the guest to use.
- `TEST_VM_BOOT_TIMEOUT` (default :arrow_right: 600)<br/>
  The time in seconds that Vagrant will wait for the machine to boot and be accessible.

### Running

The **Install Script** tests can be run by changing to the fixture directory and invoking `vagrant up`, e.g.:
```shell
cd tests/install/rocky-8
vagrant up
# The following provisioners are optional. In GitHub Actions CI they are invoked
# explicitly to avoid certain timeout issues on slow runners
vagrant provision --provision-with=rke2-wait-for-node
vagrant provision --provision-with=rke2-wait-for-coredns
vagrant provision --provision-with=rke2-wait-for-local-storage
vagrant provision --provision-with=rke2-wait-for-metrics-server
vagrant provision --provision-with=rke2-wait-for-traefik
vagrant provision --provision-with=rke2-status
vagrant provision --provision-with=rke2-procps
```