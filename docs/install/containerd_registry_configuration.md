# Containerd Registry Configuration

Containerd can be configured to connect to private registries and use them to pull private images on each node.

Upon startup, RKE2 will check to see if a `registries.yaml` file exists at `/etc/rancher/rke2/` and instruct containerd to use any registries defined in the file. If you wish to use a private registry, then you will need to create this file as root on each node that will be using the registry.

Note that server nodes are schedulable by default. If you have not tainted the server nodes and will be running workloads on them, please ensure you also create the `registries.yaml` file on each server as well.

**Note:** Containerd is not used for the initial RKE2 node bootstrapping. It is only used for Kubernetes workloads that are launched after the node is joined to the cluster. Therefore, in an airgap setup you must follow one of the methods outlined in the [airgap installation documentation](airgap.md), even if you use this containerd registry feature.

Configuration in containerd can be used to connect to a private registry with a TLS connection and with registries that enable authentication as well. The following section will explain the `registries.yaml` file and give different examples of using private registry configuration in RKE2.

## Registries Configuration File

The file consists of two main sections:

- mirrors
- configs

### Mirrors

Mirrors is a directive that defines the names and endpoints of the private registries, for example:

```yaml
mirrors:
  mycustomreg.com:
    endpoint:
      - "https://mycustomreg.com:5000"
```

Each mirror must have a name and set of endpoints. When pulling an image from a registry, containerd will try these endpoint URLs one by one, and use the first working one.

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
      - "https://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
    tls:
      cert_file: # path to the cert file used in the registry
      key_file:  # path to the key file used in the registry
      ca_file:   # path to the ca file used in the registry
```

*Without Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "https://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    tls:
      cert_file: # path to the cert file used in the registry
      key_file:  # path to the key file used in the registry
      ca_file:   # path to the ca file used in the registry
```

### Without TLS

Below are examples showing how you may configure `/etc/rancher/rke2/registries.yaml` on each node when _not_ using TLS.

*With Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "http://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
```

*Without Authentication:*

```yaml
mirrors:
  docker.io:
    endpoint:
      - "http://mycustomreg.com:5000"
```

> In case of no TLS communication, you need to specify `http://` for the endpoints, otherwise it will default to https.

In order for the registry changes to take effect, you need to either configure this file before starting RKE2 on the node, or restart RKE2 on each configured node.
