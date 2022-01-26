RKE2 Install on Windows Server 2019
---

Asserting correctness of the RKE2 installer script on [Windows Server 2019](https://docs.microsoft.com/en-us/windows-server/get-started/whats-new-in-windows-server-2019).

### Testing With Vagrant

The [Vagrant box](https://app.vagrantup.com/jborean93/boxes/WindowsServer2019) used for this test supports these providers:
- `hyperv`
- `libvirt`
- `virtualbox` (the default for most installations, including `macos-10.15` github actions runners)

To spin up a VM to test a locally modified `install.ps1`:
```shell
vagrant up
```

See also:
- [developer-docs/testing.md](../../../../developer-docs/testing.md#environment-variables)
