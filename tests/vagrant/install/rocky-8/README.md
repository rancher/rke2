RKE2 Install on CentOS 8
---

Asserting correctness of the RKE2 installer script using [Rocky Linux](https://wiki.rockylinux.org/)
as a stand-in for [RHEL 8](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8).

### Testing With Vagrant

The [Vagrant box](https://app.vagrantup.com/generic/boxes/rocky8) used for this test supports these providers:
- `libvirt`
- `virtualbox` (the default for most installations, including `macos-10.15` github actions runners)
- `vmware_desktop`
- `hyperv`
- `parallels`

To spin up a VM to test a locally modified `install.sh`:
```shell
vagrant up
```

See also:
- [developer-docs/testing.md](../../../../developer-docs/testing.md#environment-variables)
