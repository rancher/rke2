# Upgrade Basics


You can upgrade rke2 by using the installation script, or by manually installing the binary of the desired version.

>**Note:** Upgrade the server nodes first, one at a time. Once all servers have been upgraded, you may then upgrade agent nodes.

### Release Channels

Upgrades performed via the installation script or using our [automated upgrades](automated_upgrade.md) feature can be tied to different release channels.

Currently, the `latest` channel is the only available channel. Once we have more releases and need to distinguish between the most recent release and the most stable release, we will add a stable channel and set it as the default.

For an exhaustive and up-to-date list of channels, you can visit the [rke2 channel service API](https://update.rke2.io/v1-release/channels). For more technical details on how channels work, you can see the [channelserver project](https://github.com/rancher/channelserver).

### Upgrade rke2 Using the Installation Script

To upgrade rke2 from an older version you can re-run the installation script using the same flags, for example:

```sh
curl -sfL https://get.rke2.io | sh -
```
This will upgrade to the most recent version in the stable channel by default.

If you want to upgrade to the most recent version in a specific channel (such as latest) you can specify the channel:
```sh
curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=latest sh -
```

If you want to upgrade to a specific version you can run the following command:

```sh
curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=vX.Y.Z+rke2rN sh -
```

Remember to restart the rke2 process after installing:

```sh
# Server nodes:
systemctl restart rke2-server

# Agent nodes:
systemctl restart rke2-agent
```

### Manually Upgrade rke2 Using the Binary

Or to manually upgrade rke2:

1. Download the desired version of the rke2 binary from [releases](https://github.com/rancher/rke2/releases)
2. Copy the downloaded binary to `/usr/local/bin/rke2` for tarball installed rke2, and `/usr/bin` for rpm installed rke2
3. Stop the old rke2 binary
4. Launch the new rke2 binary

### Restarting rke2

Restarting rke2 is supported by the installation script for systemd.

**systemd**

To restart servers manually:
```sh
sudo systemctl restart rke2-server
```

To restart agents manually:
```sh
sudo systemctl restart rke2-agent
```
