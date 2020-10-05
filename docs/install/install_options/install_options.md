---
title: Options
---

This page focuses on the options that can be used when you set up RKE2 for the first time:

- [Options for installation with script](#options-for-installation-with-script)
- [Options for running from binary](#running-rke2-from-the-binary)
- [Registration options for the RKE2 server](#registration-options-for-the-rke2-server)
- [Registration options for the RKE2 agent](#registration-options-for-the-rke2-agent)
- [Configuration File](#configuration-file)

In addition to configuring RKE2 with environment variables and CLI arguments, RKE2 can also use a [config file.](#configuration-file)

> Throughout the RKE2 documentation, you will see some options that can be passed in as both command flags and environment variables. For help with passing in options, refer to [How to Use Flags and Environment Variables.](./how_to_flags/how_to_flags.md)

### Options for Installation with Script

As mentioned in the [Quick-Start Guide](../../install/quickstart.md), you can use the installation script available at https://get.rke2.io to install RKE2 as a service.

The simplest form of this command is as follows:
```sh
curl -sfL https://get.rke2.io | sh -
```

When using this method to install RKE2, the following environment variables can be used to configure the installation:

| Environment Variable | Description |
|-----------------------------|---------------------------------------------|
| `INSTALL_RKE2_VERSION` | Version of RKE2 to download from GitHub. Will attempt to download from the latest channel if not specified. |
| `INSTALL_RKE2_TYPE` | Type of systemd service to create, can be either "server" or "agent" Default is "server". |
| `INSTALL_RKE2_CHANNEL_URL` | Channel URL for fetching RKE2 download URL. Defaults to https://update.rke2.io/v1-release/channels. |
| `INSTALL_RKE2_CHANNEL` | Channel to use for fetching RKE2 download URL. Defaults to "latest". Options include: `stable`, `latest`, `testing`. |
| `INSTALL_RKE2_METHOD` | Method of installation to use. Default is on RPM-based systems "rpm", all else "tar". |


Environment variables which begin with `RKE2_` will be preserved for the services to use.

When running the agent `RKE2_TOKEN` must also be set.

### Running RKE2 from the Binary

As stated, the installation script is primarily concerned with configuring RKE2 to run as a service. If you choose to not use the script, you can run RKE2 simply by downloading the binary from our [release page](https://github.com/rancher/rke2/releases/latest), placing it on your path, and executing it. The RKE2 binary supports the following commands:

Command | Description
--------|------------------
<span class='nowrap'>`rke2 server`</span> | Run the RKE2 management server, which will also launch the Kubernetes control plane components such as the API server, controller-manager, and scheduler.
<span class='nowrap'>`rke2 agent`</span> |  Run the RKE2 node agent. This will cause RKE2 to run as a worker node, launching the Kubernetes node services `kubelet` and `kube-proxy`.
<span class='nowrap'>`rke2 help`</span> | Shows a list of commands or help for one command

The `rke2 server` and `rke2 agent` commands have additional configuration options that can be viewed with <span class='nowrap'>`rke2 server --help`</span> or <span class='nowrap'>`rke2 agent --help`</span>.

### Registration Options for the RKE2 Server

For details on configuring the RKE2 server, refer to the [server configuration reference.](./server_config/server_config.md)


### Registration Options for the RKE2 Agent

For details on configuring the RKE2 agent, refer to the [agent configuration reference.](./agent_config/agent_config.md)

### Configuration File

In addition to configuring RKE2 with environment variables and CLI arguments, RKE2 can also use a config file.

By default, values present in a YAML file located at `/etc/rancher/rke2/config.yaml` will be used on install.

An example of a basic `server` config file is below:

```yaml
write-kubeconfig-mode: "0644"
tls-san:
  - "foo.local"
node-label:
  - "foo=bar"
  - "something=amazing"
```

In general, CLI arguments map to their respective YAML key, with repeatable CLI arguments being represented as YAML lists.

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
