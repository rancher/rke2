# Air-Gap Install

RKE2 can be installed in an air-gapped environment with two different methods.
You can either deploy via the bundled `rke2-airgap-images` tarball, or by using a private registry.

All files mentioned in the steps can be obtained from the assets of the desired released rke2 version [here](https://github.com/rancher/rke2/releases).

If running on an SELinux enforcing air-gapped node, you must first install the necessary SELinux policy RPM before performing these steps. See our [RPM Documentation](https://github.com/rancher/rke2#rpm-repositories) to determine what you need.

## Tarball Method
1. Add the desired version of the `rke2-airgap-images-amd64.tar.gz` file to the air-gapped node.
2. Gunzip the tar.gz file so that it is only a tar, and move it to `/var/lib/rancher/rke2/agent/images/`.
3. [Install RKE2](#install-rke2)

## Private Registry Method
The private registry must be using TLS, with a cert trusted by the host CA bundle. If the registry is using a self-signed cert, you can add the cert to the host CA bundle with `update-ca-certificates`. The registry must also allow anonymous (unauthenticated) access.

1. Add all the required system images to your private registry. A simple list of these can be obtained from the `rke2-images.linux-amd64.txt` file.
2. Add the ca cert to the operating system's trusted certs
3. [Install RKE2](#install-rke2) using the `system-default-registry` parameter.

## Install RKE2
These steps should only be performed after completing one of either the [Tarball Method](#tarball-method) or [Private Registry Method](#private-registry-method).

1. Obtain the rke2 binary file `rke2.linux-amd64`
2. Ensure the binary is named `rke2` and place it in `/usr/local/bin`. Ensure it is executable.
3. Run the binary with the desired parameters. For example, if using the Private Registry Method, your config file would have the following:
```yaml
system-default-registry: "https://myprivreg.com:5000"
```
