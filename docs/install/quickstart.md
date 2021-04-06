---
title: Quick Start
---

This guide will help you quickly launch a cluster with default options.

> New to Kubernetes? The official Kubernetes docs already have some great tutorials outlining the basics [here](https://kubernetes.io/docs/tutorials/kubernetes-basics/).

### Prerequisites

Make sure your environment fulfills the [requirements.](https://docs.rke2.io/install/requirements/)
If NetworkManager is installed and enabled on your hosts, [ensure that it is configured to ignore CNI-managed interfaces.](https://docs.rke2.io/known_issues/#networkmanager)

### Server Node Installation
--------------
RKE2 provides an installation script that is a convenient way to install it as a service on systemd based systems. This script is available at https://get.rke2.io. To install RKE2 using this method do the following:

#### 1. Run the installer
```
curl -sfL https://get.rke2.io | sh -
```
This will install the `rke2-server` service and the `rke2` binary onto your machine.

#### 2. Enable the rke2-server service
```
systemctl enable rke2-server.service
```

#### 3. Start the service
```
systemctl start rke2-server.service
```

#### 4. Follow the logs, if you like
```
journalctl -u rke2-server -f
```

After running this installation:

* The `rke2-server` service will be installed. The `rke2-server` service will be configured to automatically restart after node reboots or if the process crashes or is killed.
* Additional utilities will be installed at `/var/lib/rancher/rke2/bin/`. They include: `kubectl`, `crictl`, and `ctr`. Note that these are not on your path by default.
* Two cleanup scripts will be installed to the path at `/usr/local/bin/rke2`. They are: `rke2-killall.sh` and `rke2-uninstall.sh`.
* A [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file will be written to `/etc/rancher/rke2/rke2.yaml`.
* A token that can be used to register other server or agent nodes will be created at `/var/lib/rancher/rke2/server/node-token`

**Note:** If you are adding additional server nodes, you must have an odd number in total. An odd number is needed to maintain quorum. See the [High Availability documentation](ha.md) for more details.

### Agent (Worker) Node Installation
#### 1. Run the installer
```
curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sh -
```
This will install the `rke2-agent` service and the `rke2` binary onto your machine.

#### 2. Enable the rke2-agent service
```
systemctl enable rke2-agent.service
```

#### 3. Configure the rke2-agent service
```
mkdir -p /etc/rancher/rke2/
vim /etc/rancher/rke2/config.yaml
```
Content for config.yaml:
```bash
server: https://<server>:9345
token: <token from server node>
```
**Note:** The `rke2 server` process listens on port `9345` for new nodes to register. The Kubernetes API is still served on port `6443`, as normal.

#### 4. Start the service
```
systemctl start rke2-agent.service
```

**Follow the logs, if you like**
```
journalctl -u rke2-agent -f
```

**Note:** Each machine must have a unique hostname. If your machines do not have unique hostnames, set the `node-name` parameter in the `config.yaml` file and provide a value with a valid and unique hostname for each node.

To read more about the config.yaml file, see the [Install Options documentation.](./install_options/install_options.md#configuration-file)
