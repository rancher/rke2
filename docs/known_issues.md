# Known Issues and Limitations

This section contains advanced information describing the different ways you can run and manage RKE2.

## Firewalld conflicts with default networking
Firewalld conflicts with RKE2's default Canal (Calico + Flannel) networking stack. To avoid unexpected behavior, firewalld should be disabled on systems running RKE2.

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
