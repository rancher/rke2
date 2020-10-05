---
title: SELinux
---

RKE2 can be run on SELinux-enabled systems which is the default when installed on CentOS/RHEL 7 &amp; 8.
The [policy](https://github.com/rancher/rke2-selinux) supporting this is a specialization of the 
[container-selinux](https://github.com/containers/container-selinux) policy for containerd. It accounts
for the non-standard location(s) which containerd is installed and places persistent and ephemeral state.

#### Custom Context Labels

RKE2 runs control-plane services as static pods which require access to multiple
[`container_var_lib_t`](https://github.com/containers/container-selinux/blob/RHEL7.5/container.te#L59)
locations. The `etcd` container must be able to read-write under `/var/lib/rancher/rke2/server/db` and read,
along with `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler`, from `/var/lib/rancher/rke2/server/tls`.
To make this work without over-privileging, e.g.,
[`spc_t`](https://github.com/containers/container-selinux/blob/RHEL7.5/container.te#L47-L49), the RKE2 SELinux policy
introduces the [`rke2_service_db_t`](https://github.com/rancher/rke2-selinux/blob/v0.3.latest.1/rke2.te#L15-L21) and 
[`rke2_service_t`](https://github.com/rancher/rke2-selinux/blob/v0.3.latest.1/rke2.te#L9-L13) context labels for
read-write and read-only access, respectively. These labels will only be applied to the RKE2 control-plane static pods.  

#### Configuration

RKE2 support for SELinux amounts to a single configuration item, the `--selinux` boolean flag. This is a pass-through
to the [`enable_selinux` boolean in the cri section of the containerd/cri toml](https://github.com/containerd/cri/blob/release/1.4/docs/config.md).
If RKE2 was installed via tarball then SELinux will not be enabled without additional configuration. The recommended
method to configure such is via an entry in the RKE2 `config.yaml`, e.g.:

```yaml
# /etc/rancher/rke2/config.yaml is the default location
selinux: true
```

This is equivalent to passing the `--selinux` flag to `rke2 server` or `rke2 agent` command-line or setting the
`RKE2_SELINUX=true` environment variable.
