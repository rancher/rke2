# Advanced Options and Configuration

This section contains advanced information describing the different ways you can run and manage RKE2.

## Certificate Rotation

By default, certificates in RKE2 expire in 12 months.

If the certificates are expired or have fewer than 90 days remaining before they expire, the certificates are rotated when RKE2 is restarted.

## Auto-Deploying Manifests

Any file found in `/var/lib/rancher/rke2/server/manifests` will automatically be deployed to Kubernetes in a manner similar to `kubectl apply`.

For information about deploying Helm charts using the manifests directory, refer to the section about [Helm.](helm.md)

## Configuring containerd

RKE2 will generate config.toml for containerd in `/var/lib/rancher/rke2/agent/etc/containerd/config.toml`.

For advanced customization for this file you can create another file called `config.toml.tmpl` in the same directory and it will be used instead.

The `config.toml.tmpl` will be treated as a Go template file, and the `config.Node` structure is being passed to the template. See [this template](https://github.com/rancher/k3s/blob/master/pkg/agent/templates/templates.go#L16-L32) for an example of how to use the structure to customize the configuration file.

## Secrets Encryption Config
RKE2 supports encrypting Secrets at rest, and will do the following automatically:

- Generate an AES-CBC key
- Generate an encryption config file with the generated key:

```yaml
{
  "kind": "EncryptionConfiguration",
  "apiVersion": "apiserver.config.k8s.io/v1",
  "resources": [
    {
      "resources": [
        "secrets"
      ],
      "providers": [
        {
          "aescbc": {
            "keys": [
              {
                "name": "aescbckey",
                "secret": "xxxxxxxxxxxxxxxxxxx"
              }
            ]
          }
        },
        {
          "identity": {}
        }
      ]
    }
  ]
}
```

- Pass the config to the Kubernetes APIServer as encryption-provider-config

Once enabled any created secret will be encrypted with this key. Note that if you disable encryption then any encrypted secrets will not be readable until you enable encryption again using the same key.

## Node Labels and Taints

RKE2 agents can be configured with the options `--node-label` and `--node-taint` which adds a label and taint to the kubelet. The two options only add labels and/or taints [at registration time [FIXME]](#FIXME), so they can only be added once and not changed after that again by running RKE2 commands.

If you want to change node labels and taints after node registration you should use `kubectl`. Refer to the official Kubernetes documentation for details on how to add [taints](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [node labels.](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/#add-a-label-to-a-node)

## Starting the Server with the Installation Script

The installation script provides units for systemd, but does not enable or start the service by default.

When running with systemd, logs will be created in `/var/log/syslog` and viewed using `journalctl -u rke2-server` or `journalctl -u rke2-agent`.

An example of installing with the install script:

```bash
curl -sfL https://get.rke2.io | sh -
systemctl enable rke2-server
systemctl start rke2-server
```

## Disabling Server Charts

The server charts bundled with `rke2` deployed during cluster bootstrapping can be disabled and replaced with alternatives.  A common use case is replacing the bundled `rke2-ingress-nginx` chart with an alternative.

To disable any of the bundled system charts, pass the `--disable` flag to `rke2 server` during bootstrapping.  The full list of system charts to disable is below:

* `rke2-canal`
* `rke2-coredns`
* `rke2-ingress`
* `rke2-kube-proxy`
* `rke2-metrics-server`

Note that it is the cluster operator's responsibility to ensure that components are disabled or replaced with care, as the server charts play important roles in cluster operability.  Refer to the [architecture overview](architecture/architecture.md#server-charts) for more information on the individual system charts role within the cluster.

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
