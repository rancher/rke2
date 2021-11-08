RKE2 Install on MicroOS
---

Asserting correctness of the RKE2 installer script using [openSUSE MicroOS](https://microos.opensuse.org/)
as a stand-in for [SUSE Linux Enterprise Micro](https://www.suse.com/products/micro/).

### Testing With Vagrant

The [Vagrant box](https://app.vagrantup.com/dweomer/boxes/microos.amd64) used for this test supports these providers:
- `libvirt`
- `virtualbox` (the default for most installations, including `macos-10.15` github actions runners)
- `vmware_desktop`

To spin up a VM to test a locally modified `install.sh`:
```shell
# make sure the vagrant-reload plugin is installed, one-time only
vagrant plugin install vagrant-reload
```
```shell
vagrant up
```

See also:
- [developer-docs/testing.md](../../../../developer-docs/testing.md#environment-variables)

### Vagrant Reload Plugin

The MicroOS guest leverages [transactional updates](https://documentation.suse.com/sles/15-SP1/html/SLES-all/cha-transactional-updates.html)
for most persistent mutations of the installation (typically involving the `/usr` partition) which requires a reboot to
take effect. The `vagrant-reload` provisioner plugin is used for this because the implementation of the [`reboot` option](https://www.vagrantup.com/docs/provisioning/shell#reboot)
of the built-in [Vagrant Shell Provisioner](https://www.vagrantup.com/docs/provisioning/shell) is unreliable
(especially if used more than once per provisioning run).
