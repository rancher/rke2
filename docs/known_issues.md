# Known Issues and Limitations

This section contains current known issues and limitations with rke2. If you come across issues with rke2 not documented here, please open a new issue [here](https://github.com/rancher/rke2/issues).

## Firewalld conflicts with default networking

Firewalld conflicts with RKE2's default Canal (Calico + Flannel) networking stack. To avoid unexpected behavior, firewalld should be disabled on systems running RKE2.

## NetworkManager

NetworkManager manipulates the routing table for interfaces in the default network namespace where many CNIs, including RKE2's default, create veth pairs for connections to containers. This can interfere with the CNI’s ability to route correctly. As such, if installing RKE2 on a NetworkManager enabled system, it is highly recommended to configure NetworkManager to ignore calico/flannel related network interfaces. In order to do this, create a configuration file called `rke2-canal.conf` in `/etc/NetworkManager/conf.d` with the contents:
```bash
[keyfile]
unmanaged-devices=interface-name:cali*;interface-name:flannel*
```

If you have not yet installed RKE2, a simple `systemctl reload NetworkManager` will suffice to install the configuration. If performing this configuration change on a system that already has RKE2 installed, a reboot of the node is necessary to effectively apply the changes.

## Istio in Selinux Enforcing System Fails by Default

This is due to just-in-time kernel module loading of rke2, which is disallowed under Selinux unless the container is privileged.
To allow Istio to run under these conditions, it requires two steps:
1. [Enable CNI](https://istio.io/latest/docs/setup/additional-setup/cni/) as part of the Istio install. Please note that this [feature](https://istio.io/latest/about/feature-stages/) is still in Alpha state at the time of this writing.
Ensure `values.cni.cniBinDir=/opt/cni/bin` and `values.cni.cniConfDir=/etc/cni/net.d`
2. After the install is complete, there should be `cni-node` pods in a CrashLoopBackoff. Manually edit their daemonset to include `securityContext.privileged: true` on the `install-cni` container.

This is also possible to do directly [through Rancher](https://github.com/rancher/rancher/issues/27377#issuecomment-739075400), if desired.
For more information regarding exact failures with detailed logs when not following these steps, please see [Issue 504](https://github.com/rancher/rke2/issues/504).

## Control Groups V2

Linux distributions, more and more, are shipping with kernels and userspaces that support cgroups v2,
e.g. since Fedora 31. However, at the time of this writing, the `containerd` that is built and shipped
with RKE2 is a 1.3.x fork (with back-ported SELinux commits from 1.4.x) which does not support cgroups v2.
Until RKE2 ships with `containerd` v1.4.x running it on cgroups v2 capable systems requires a little up-front
configuration:

Assuming a `systemd`-based system, setting the [systemd.unified_cgroup_hierarchy=0](https://www.freedesktop.org/software/systemd/man/systemd.html#systemd.unified_cgroup_hierarchy)
kernel parameter will indicate to systemd that it should run with hybrid (cgroups v1 + v2) support.
Combined with the above, setting the [systemd.legacy_systemd_cgroup_controller](https://www.freedesktop.org/software/systemd/man/systemd.html#systemd.legacy_systemd_cgroup_controller)
kernel parameter will indicate to systemd that it should run with legacy (cgroups v1) support.
As these are kernel command-line arguments they must be set in the system bootloader so that they will be
passed to `systemd` as PID 1 at `/sbin/init`.

See:

- [grub2 manual](https://www.gnu.org/software/grub/manual/grub/grub.html#linux)
- [systemd manual](https://www.freedesktop.org/software/systemd/man/systemd.html#Kernel%20Command%20Line)
- [cgroups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
