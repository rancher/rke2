---
title: Quick Start
---

This guide will help you quickly launch a cluster with default options.

> New to Kubernetes? The official Kubernetes docs already have some great tutorials outlining the basics [here](https://kubernetes.io/docs/tutorials/kubernetes-basics/).

### Prerequisites

- Make sure your environment fulfills the [requirements.](https://docs.rke2.io/install/requirements/)
If NetworkManager is installed and enabled on your hosts, [ensure that it is configured to ignore CNI-managed interfaces.](https://docs.rke2.io/known_issues/#networkmanager)

- For RKE2 versions 1.21 and higher, if the host kernel supports [AppArmor](https://apparmor.net/), the AppArmor tools (usually available via the `apparmor-parser` package) must also be present prior to installing RKE2.

- The RKE2 installation process must be run as the root user or through `sudo`.

### Server Node Installation
--------------
RKE2 provides an installation script that is a convenient way to install it as a service on systemd based systems. This script is available at https://get.rke2.io. To install RKE2 using this method do the following:

#### 1. Run the installer

```sh
curl -sfL https://get.rke2.io | sh -
```

This will install the `rke2-server` service and the `rke2` binary onto your machine. Due to its nature, It will fail unless it runs as the root user or through `sudo`.

#### 2. Enable the rke2-server service

```sh
systemctl enable rke2-server.service
```

#### 3. Start the service

```sh
systemctl start rke2-server.service
```

#### 4. Follow the logs, if you like

```sh
journalctl -u rke2-server -f
```

After running this installation:

* The `rke2-server` service will be installed. The `rke2-server` service will be configured to automatically restart after node reboots or if the process crashes or is killed.
* Additional utilities will be installed at `/var/lib/rancher/rke2/bin/`. They include: `kubectl`, `crictl`, and `ctr`. Note that these are not on your path by default.
* Two cleanup scripts will be installed to the path at `/usr/local/bin/rke2`. They are: `rke2-killall.sh` and `rke2-uninstall.sh`.
* A [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file will be written to `/etc/rancher/rke2/rke2.yaml`.
* A token that can be used to register other server or agent nodes will be created at `/var/lib/rancher/rke2/server/node-token`

**Note:** If you are adding additional server nodes, you must have an odd number in total. An odd number is needed to maintain quorum. See the [High Availability documentation](ha.md) for more details.

### Linux Agent (Worker) Node Installation

The steps on this section requires root level access or `sudo` to work.

#### 1. Run the installer

```sh
curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sh -
```

This will install the `rke2-agent` service and the `rke2` binary onto your machine. Due to its nature, It will fail unless it runs as the root user or through `sudo`.

#### 2. Enable the rke2-agent service

```sh
systemctl enable rke2-agent.service
```

#### 3. Configure the rke2-agent service

```sh
mkdir -p /etc/rancher/rke2/
vim /etc/rancher/rke2/config.yaml
```

Content for config.yaml:

```yaml
server: https://<server>:9345
token: <token from server node>
```

**Note:** The `rke2 server` process listens on port `9345` for new nodes to register. The Kubernetes API is still served on port `6443`, as normal.

#### 4. Start the service

```sh
systemctl start rke2-agent.service
```

**Follow the logs, if you like**

```sh
journalctl -u rke2-agent -f
```

**Note:** Each machine must have a unique hostname. If your machines do not have unique hostnames, set the `node-name` parameter in the `config.yaml` file and provide a value with a valid and unique hostname for each node.

To read more about the config.yaml file, see the [Install Options documentation.](./install_options/install_options.md#configuration-file)

### Windows Agent (Worker) Node Installation
**Windows Support is currently Experimental as of v1.21.3+rke2r1**
**Windows Support requires choosing Calico as the CNI for the RKE2 cluster**

#### 0. Prepare the Windows Agent Node
**Note** The Windows Server Containers feature needs to be enabled for the RKE2 agent to work.

Open a new Powershell window with Administrator privileges
```powershell
powershell -Command "Start-Process PowerShell -Verb RunAs"
```

In the new Powershell window, run the following command.
```powershell
Enable-WindowsOptionalFeature -Online -FeatureName containers –All
```
This will require a reboot for the `Containers` feature to properly function.

#### 1. Download the Install Script
```powershell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/rancher/rke2/master/install.ps1 -Outfile install.ps1
```
This script will download the `rke2.exe` Windows binary onto your machine.

#### 2. Configure the rke2-agent for Windows
```powershell
New-Item -Type Directory c:/etc/rancher/rke2 -Force
Set-Content -Path c:/etc/rancher/rke2/config.yaml -Value @"
server: https://<server>:9345
token: <token from server node>
"@
```

To read more about the config.yaml file, see the [Install Options documentation.](./install_options/install_options.md#configuration-file)


#### 3. Configure PATH 
```powershell
$env:PATH+=";c:\var\lib\rancher\rke2\bin;c:\usr\local\bin"

[Environment]::SetEnvironmentVariable(
    "Path",
    [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";c:\var\lib\rancher\rke2\bin;c:\usr\local\bin",
    [EnvironmentVariableTarget]::Machine)
```
#### 4. Run the Installer
```powershell
./install.ps1
```

#### 5. Start the Windows RKE2 Service
```powershell
rke2.exe agent service --add
```
**Note:** Each machine must have a unique hostname. 

If you would prefer to use CLI parameters only instead, run the binary with the desired parameters. 

```powershell
rke2.exe agent --token <> --server <>
```
