# Containerd Registry Configuration

Containerd can be configured to connect to private registries and use them to pull private images on each node.

Upon startup, RKE2 will check to see if a `registries.yaml` file exists at `/etc/rancher/rke2/` and instruct containerd to use any registries defined in the file. If you wish to use a private registry, then you will need to create this file as root on each node that will be using the registry.

Note that server nodes are schedulable by default. If you have not tainted the server nodes and will be running workloads on them, please ensure you also create the `registries.yaml` file on each server as well.

**Note:** Prior to RKE2 v1.20, containerd registry configuration is not honored for the initial RKE2 node bootstrapping, only for Kubernetes workloads that are launched after the node is joined to the cluster. Consult the [airgap installation documentation](airgap.md) if you plan on using this containerd registry feature to bootstrap nodes.

Configuration in containerd can be used to connect to a private registry with a TLS connection and with registries that enable authentication as well. The following section will explain the `registries.yaml` file and give different examples of using private registry configuration in RKE2.

## Registries Configuration File

The file consists of two main sections:

- mirrors
- configs

### Mirrors

Mirrors is a directive that defines the names and endpoints of the private registries. Private registries can be used as a local mirror for the default docker.io registry, or for images where the registry is explicitly specified in the name.

For example, the following configuration would pull from the private registry at `https://registry.example.com:5000` for both `library/busybox:latest` and `registry.example.com/library/busybox:latest`:

```yaml
mirrors:
  docker.io:
    endpoint:
      - "https://registry.example.com:5000"
  registry.example.com:
    endpoint:
      - "https://registry.example.com:5000"
```

Each mirror must have a name and set of endpoints. When pulling an image from a registry, containerd will try these endpoint URLs one by one, and use the first working one.

**Note:** If no endpoint is configured, containerd assumes that the registry can be accessed anonymously via HTTPS on port 443, and is using a certificate trusted by the host operating system. For more information, you may [consult the containerd documentation](https://github.com/containerd/containerd/blob/master/docs/cri/registry.md#configure-registry-endpoint).

#### Rewrites

Each mirror can have a set of rewrites. Rewrites can change the tag of an image based on a regular expression. This is useful if the organization/project structure in the mirror registry is different to the upstream one.

For example, the following configuration would transparently pull the image `rancher/rke2-runtime:v1.23.5-rke2r1` from `registry.example.com:5000/mirrorproject/rancher-images/rke2-runtime:v1.23.5-rke2r1`:

```yaml
mirrors:
  docker.io:
    endpoint:
      - "https://registry.example.com:5000"
    rewrite:
      "^rancher/(.*)": "mirrorproject/rancher-images/$1"
```

### Configs

The configs section defines the TLS and credential configuration for each mirror. For each mirror you can define `auth` and/or `tls`. The TLS part consists of:

Directive | Description
----------|------------
`cert_file` | The client certificate path that will be used to authenticate with the registry
`key_file` | The client key path that will be used to authenticate with the registry
`ca_file` | Defines the CA certificate path to be used to verify the registry's server cert file
<span style="white-space: nowrap">`insecure_skip_verify`</span> | Boolean that defines if TLS verification should be skipped for the registry

The credentials consist of either username/password or authentication token:

- username: user name of the private registry basic auth
- password: user password of the private registry basic auth
- auth: authentication token of the private registry basic auth

Below are basic examples of using private registries in different modes:

### With TLS

Below are examples showing how you may configure `/etc/rancher/rke2/registries.yaml` on each node when using TLS.

*With Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "https://registry.example.com:5000"
configs:
  "registry.example.com:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
    tls:
      cert_file:            # path to the cert file used to authenticate to the registry
      key_file:             # path to the key file for the certificate used to authenticate to the registry
      ca_file:              # path to the ca file used to verify the registry's certificate
      insecure_skip_verify: # may be set to true to skip verifying the registry's certificate
```

*Without Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "https://registry.example.com:5000"
configs:
  "registry.example.com:5000":
    tls:
      cert_file:            # path to the cert file used to authenticate to the registry
      key_file:             # path to the key file for the certificate used to authenticate to the registry
      ca_file:              # path to the ca file used to verify the registry's certificate
      insecure_skip_verify: # may be set to true to skip verifying the registry's certificate
```

### Without TLS

Below are examples showing how you may configure `/etc/rancher/rke2/registries.yaml` on each node when _not_ using TLS.

*Plaintext HTTP With Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "http://registry.example.com:5000"
configs:
  "registry.example.com:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
```

*Plaintext HTTP Without Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "http://registry.example.com:5000"
```

> If using a registry using plaintext HTTP without TLS, you need to specify `http://` as the endpoint URI scheme, otherwise it will default to `https://`.

In order for the registry changes to take effect, you need to either configure this file before starting RKE2 on the node, or restart RKE2 on each configured node.
