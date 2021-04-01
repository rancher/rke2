# Air-Gap Install

**Important:** If your node has NetworkManager installed and enabled, [ensure that it is configured to ignore CNI-managed interfaces.](https://docs.rke2.io/known_issues/#networkmanager)

RKE2 can be installed in an air-gapped environment with two different methods.
You can either deploy via the `rke2-airgap-images` tarball release artifact, or by using a private registry.

All files mentioned in the steps can be obtained from the assets of the desired released rke2 version [here](https://github.com/rancher/rke2/releases).

If running on an SELinux enforcing air-gapped node, you must first install the necessary SELinux policy RPM before performing these steps. See our [RPM Documentation](https://github.com/rancher/rke2#rpm-repositories) to determine what you need.

## Tarball Method
1. Download the airgap images tarball from the RKE release artifacts list for the version of RKE2 you are using.
    Use `rke2-airgap-images-amd64.tar.zstd`, or `rke2-airgap-images-amd64.tar.gz` for releases prior to v1.20. Zstandard offers better compression ratios and faster decompression speeds compared to gzip.
2. Ensure that the `/var/lib/rancher/rke2/agent/images/` directory exists on the node.
3. Copy the compressed archive to `/var/lib/rancher/rke2/agent/images/` on the node, ensuring that the file extension is retained.
4. [Install RKE2](#install-rke2)

## Private Registry Method
As of RKE2 v1.20, private registry support honors all settings from the [containerd registry configuration](containerd_registry_configuration.md). This includes endpoint override and transport protocol (HTTP/HTTPS), authentication, certificate verification, etc.

Prior to RKE2 v1.20, private registries must use TLS, with a cert trusted by the host CA bundle. If the registry is using a self-signed cert, you can add the cert to the host CA bundle with `update-ca-certificates`. The registry must also allow anonymous (unauthenticated) access.

1. Add all the required system images to your private registry. A list of images can be obtained from the `rke2-images.linux-amd64.txt` file, or you may `docker load` the airgap image tarball referenced above, then tag and push the loaded images.
2. Add the registry's CA cert to the containerd registry configuration, or operating system's trusted certs for releases prior to v1.20.
3. [Install RKE2](#install-rke2) using the `system-default-registry` parameter.

## Install RKE2
These steps should only be performed after completing one of either the [Tarball Method](#tarball-method) or [Private Registry Method](#private-registry-method).

1. Obtain the rke2 binary file `rke2.linux-amd64`.
2. Ensure the binary is named `rke2` and place it in `/usr/local/bin`. Ensure it is executable.
3. Run the binary with the desired parameters. For example, if using the Private Registry Method, your config file would have the following:
```yaml
system-default-registry: "registry.example.com:5000"
```

**Note:** The `system-default-registry` parameter must specify only valid RFC 3986 URI authorities, i.e. a host and optional port.
