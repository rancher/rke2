RKE2 Install on CentOS 8
---

Asserting correctness of the RKE2 installer script using [CentOS 7](https://docs.centos.org/en-US/centos/install-guide/)
as a stand-in for [RHEL 7](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/7/).

### Testing With Vagrant

The [Vagrant box](https://app.vagrantup.com/dweomer/boxes/centos-7.9-amd64) used for this test supports these providers:
- `libvirt`
- `virtualbox` (the default for most installations, including `macos-10.15` github actions runners)
- `vmware_desktop`

To spin up a VM to test a locally modified `install.sh`:
```shell
vagrant up
```

See also:
- [developer-docs/testing.md](../../../../developer-docs/testing.md#environment-variables)
