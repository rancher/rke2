# RKE2

We're starting to experiment with our next-generation Kuberenets distribution. This is currently an alpha project that will evolve a lot. Stay tuned.

## What is this?

RKE2 is a fully compliant Kubernetes distribution, with the following changes:

- Packaged as a single binary.
- Kubelet running as an embedded process in the agent and server.
- Master components running as static pods.
- All components images are compiled using Goboring library.
- The following addons are installed as helm charts:
  - CoreDNS
  - Kube-proxy
  - Canal
  - Nginx Ingress Controller
  - Metrics Server

## Installation

RKE2 comes with a convenient and configurable installation script that allows for system configuration, binary release download, and openrc init script or systemd unit file. 

### Quick Start

The `install.sh` script provides a convenient way for installing rke2 server and agent, to install rke2 service:

```sh
INSTALL_RKE2_VERSION=v0.0.1-alpha.7 ./install.sh
```

### CIS Mode

If you plan to run your RKE2 cluster in CIS mode, the installation script offers the ability to install RKE2 in preparation for this. Use the `INSTALL_RKE2_CIS_MODE` environment variable. This will configure the system for RKE2 to run in CIS compliant mode. This includes the creation of a user "etcd", updates a number of kernel parameters, and installs a systemd or openrc unit file that passes the CIS mode flag to the binary (`--profile=cis-1.5`). Please reference the installation script (`install.sh`) for CIS specifics.

```sh
INSTALL_RKE2_CIS_MODE=true INSTALL_RKE2_VERSION=v0.0.1-alpha.7 ./install.sh
```

*NOTE*
RKE2 can be run in CIS mode without running the install script in CIS mode however the remediations will need to be done manually. 

### Post Installation

A kubeconfig is written to `/etc/rancher/rke2/rke2.yaml`, the install script will install rke2 and additional binaries such as `kubectl`, `ctr`, `rke2-uninstall.sh`.

## Operation

To start using the rke2 cluster:

```sh
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
kubectl get nodes
```

`RKE2_TOKEN` is created at `/var/lib/rancher/rke2/server/node-token` on your server. To install on worker nodes we should pass `RKE2_URL` along with `RKE2_TOKEN` or `RKE2_CLUSTER_SECRET` environment variables, for example:

```sh
RKE2_URL=https://myserver:6443 RKE2_TOKEN=XXX ./install.sh
```

## Automated deployment

We provide a simple automated way to install RKE2 on AWS via terraform scripts, this method requires terraform to be installed and access to AWS cloud, to get started please checkout the [rke2-build](https://github.com/rancher/rke2-build) repo.
