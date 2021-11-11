RKE2 Install on Ubuntu Focal Fossa
---

Asserting correctness of the RKE2 installer script on [Ubuntu 20.04](https://releases.ubuntu.com/20.04/).

### Testing With Vagrant

The [Vagrant box](https://app.vagrantup.com/generic/boxes/ubuntu2004) used for this test supports these providers:
- `hyperv`
- `libvirt`
- `parallels`
- `virtualbox` (the default for most installations, including `macos-10.15` github actions runners)
- `vmware_desktop`

To spin up a VM to test a locally modified `install.sh`:
```shell
vagrant up
```

See also:
- [developer-docs/testing.md](../../../../developer-docs/testing.md#environment-variables)
