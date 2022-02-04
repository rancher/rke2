# Overview

This page focuses on the configuration options available when setting up RKE2:

- [Configuring the Linux installation script](#configuring-the-linux-installation-script)
- [Configuring the Windows installation script](#configuring-the-windows-installation-script)
- [Configuring RKE2 server nodes](#configuring-rke2-server-nodes)
- [Configuring Linux RKE2 agent nodes](#configuring-linux-rke2-agent-nodes)
- [Configuring Windows RKE2 agent nodes](#configuring-windows-rke2-agent-nodes)
- [Using the configuration file](#configuration-file)
- [Configuring when running the binary directly](#configuring-when-running-the-binary-directly)

The primary way to configure RKE2 is through its [config file](#configuration-file). Command line arguments and environment variables are also available, but RKE2 is installed as a systemd service and thus these are not as easy to leverage.

### Configuring the Linux Installation Script

As mentioned in the [Quick-Start Guide](../../install/quickstart.md), you can use the installation script available at <https://get.rke2.io> to install RKE2 as a service.

The simplest form of this command is running, as root user or through `sudo`, as follows:

```sh
# curl -sfL https://get.rke2.io | sudo sh -
curl -sfL https://get.rke2.io | sh -
```

When using this method to install RKE2, the following environment variables can be used to configure the installation:

| Environment Variable | Description |
|-----------------------------|---------------------------------------------|
| <span style="white-space: nowrap">`INSTALL_RKE2_VERSION`</span> | Version of RKE2 to download from GitHub. Will attempt to download the latest release from the `stable` channel if not specified. `INSTALL_RKE2_CHANNEL` should also be set if installing on an RPM-based system and the desired version does not exist in the `stable` channel. |
| <span style="white-space: nowrap">`INSTALL_RKE2_TYPE`</span> | Type of systemd service to create, can be either "server" or "agent" Default is "server". |
| <span style="white-space: nowrap">`INSTALL_RKE2_CHANNEL_URL`</span> | Channel URL for fetching RKE2 download URL. Defaults to `https://update.rke2.io/v1-release/channels`. |
| <span style="white-space: nowrap">`INSTALL_RKE2_CHANNEL`</span> | Channel to use for fetching RKE2 download URL. Defaults to `stable`. Options include: `stable`, `latest`, `testing`. |
| <span style="white-space: nowrap">`INSTALL_RKE2_METHOD`</span> | Method of installation to use. Default is on RPM-based systems `rpm`, all else `tar`. |

This installation script is straight-forward and will do the following:

1. Obtain the desired version to install based on the above parameters. If no parameters are supplied, the latest official release will be used.
2. Determine and execute the installation method. There are two methods: rpm and tar. If the `INSTALL_RKE2_METHOD` variable is set, that will be respected, Otherwise, `rpm` will be used on operating systems that use this package management system. On all other systems, tar will be used. In the case of the tar method, the script will simply unpack the tar archive associated with the desired release. In the case of rpm, a yum repository will be set up and the rpm will be installed using yum.

### Configuring the Windows Installation Script
**Windows Support is currently Experimental as of v1.21.3+rke2r1**
**Windows Support requires choosing Calico as the CNI for the RKE2 cluster**

As mentioned in the [Quick-Start Guide](../../install/quickstart.md), you can use the installation script available at https://github.com/rancher/rke2/blob/master/install.ps1 to install RKE2 on a Windows Agent Node.

The simplest form of this command is as follows:
```powershell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/rancher/rke2/master/install.ps1 -Outfile install.ps1
```
When using this method to install the Windows RKE2 agent, the following parameters can be passed to configure the installation script:

```console
SYNTAX

install.ps1 [[-Channel] <String>] [[-Method] <String>] [[-Type] <String>] [[-Version] <String>] [[-TarPrefix] <String>] [-Commit] [[-AgentImagesDir] <String>] [[-ArtifactPath] <String>] [[-ChannelUrl] <String>] [<CommonParameters>]

OPTIONS

-Channel           Channel to use for fetching RKE2 download URL (Default: "stable")
-Method            The installation method to use. Currently tar or choco installation supported. (Default: "tar")
-Type              Type of RKE2 service. Only the "agent" type is supported on Windows. (Default: "agent")
-Version           Version of rke2 to download from Github
-TarPrefix         Installation prefix when using the tar installation method. (Default: `C:/usr/local` unless `C:/usr/local` is read-only or has a dedicated mount point, in which case `C:/opt/rke2` is used instead)
-Commit            (experimental/agent) Commit of RKE2 to download from temporary cloud storage. If set, this forces `--Method=tar`. Intended for development purposes only.
-AgentImagesDir    Installation path for airgap images when installing from CI commit. (Default: `C:/var/lib/rancher/rke2/agent/images`)
-ArtifactPath      If set, the install script will use the local path for sourcing the `rke2.windows-$SUFFIX` and `sha256sum-$ARCH.txt` files rather than the downloading the files from GitHub. Disabled by default.
```

### Other Windows Installation Script Usage Examples
#### Install the Latest Version Instead of Stable
```powershell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/rancher/rke2/master/install.ps1 -Outfile install.ps1
./install.ps1 -Channel Latest
```

#### Install the Latest Version using Tar Installation Method
```powershell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/rancher/rke2/master/install.ps1 -Outfile install.ps1
./install.ps1 -Channel Latest -Method Tar
```
### Configuring RKE2 Server Nodes

For details on configuring the RKE2 server, refer to the [server configuration reference.](server_config.md)

### Configuring Linux RKE2 Agent Nodes

For details on configuring the RKE2 agent, refer to the [agent configuration reference.](linux_agent_config.md)

### Configuring Windows RKE2 Agent Nodes

For details on configuring the RKE2 Windows agent, refer to the [Windows agent configuration reference.](windows_agent_config.md)

### Configuration File

By default, RKE2 will launch with the values present in the YAML file located at `/etc/rancher/rke2/config.yaml`.

An example of a basic `server` config file is below:

```yaml
write-kubeconfig-mode: "0644"
tls-san:
  - "foo.local"
node-label:
  - "foo=bar"
  - "something=amazing"
```

The configuration file parameters map directly to CLI arguments, with repeatable CLI arguments being represented as YAML lists.

An identical configuration using solely CLI arguments is shown below to demonstrate this:

```bash
rke2 server \
  --write-kubeconfig-mode "0644"    \
  --tls-san "foo.local"             \
  --node-label "foo=bar"            \
  --node-label "something=amazing"
```

It is also possible to use both a configuration file and CLI arguments.  In these situations, values will be loaded from both sources, but CLI arguments will take precedence.  For repeatable arguments such as `--node-label`, the CLI arguments will overwrite all values in the list.

Finally, the location of the config file can be changed either through the cli argument `--config FILE, -c FILE`, or the environment variable `$RKE2_CONFIG_FILE`.

### Configuring when Running the Binary Directly

As stated, the installation script is primarily concerned with configuring RKE2 to run as a service. If you choose to not use the script, you can run RKE2 simply by downloading the binary from our [release page](https://github.com/rancher/rke2/releases/latest), placing it on your path, and executing it. The RKE2 binary supports the following commands:

Command | Description
--------|------------------
<span style="white-space: nowrap">`rke2 server`</span> | Run the RKE2 management server, which will also launch the Kubernetes control plane components such as the API server, controller-manager, and scheduler. Only Supported on Linux.
<span style="white-space: nowrap">`rke2 agent`</span> |  Run the RKE2 node agent. This will cause RKE2 to run as a worker node, launching the Kubernetes node services `kubelet` and `kube-proxy`. Supported on Linux and Windows.
<span style="white-space: nowrap">`rke2 help`</span> | Shows a list of commands or help for one command

The `rke2 server` and `rke2 agent` commands have additional configuration options that can be viewed with <span style="white-space: nowrap">`rke2 server --help`</span> or <span style="white-space: nowrap">`rke2 agent --help`</span>.
