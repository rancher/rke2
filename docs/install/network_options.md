# Network Options

By default RKE2 runs Canal as the cni with VXLAN as the default backend, Canal is installed via a helm chart after the main components are up and running and can be customized by modifying the helm chart options.

Optionally, Cilium might be used as the cni instead of Canal.

# Canal Options

To override Canal options you should be able to create HelmChartConfig resources, The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart, for example to override Canal Options, you can create the following config:


```
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-canal
  namespace: kube-system
spec:
  valuesContent: |-
    flannel:
      iface: "eth1"
```

The config needs to be copied over to the manifests directory before installing rke2:

```
mkdir -p /var/lib/rancher/rke2/server/manifests/
cp rke2-canal-config.yml /var/lib/rancher/rke2/server/manifests/
```

For more information about the full options of the Canal config please refer to the [rke2-charts](https://github.com/rancher/rke2-charts/blob/main-source/packages/rke2-canal/charts/values.yaml).

# Using Cilium instead of Canal

To use Cilium, Canal needs to be disabled on deployment. Refer to [Advanced Options and Configuration](../advanced.md#Disabling Server Charts) for information on how to do this.

To deploy Cilium, you can use the following config:

```
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: rke2-cilium
  namespace: kube-system
spec:
  repo: https://rke2-charts.rancher.io
  chart: rke2-cilium
  bootstrap: true
  valuesContent: |-
    cilium: {}
```

The config needs to be copied over to the manifests directory:

```
mkdir -p /var/lib/rancher/rke2/server/manifests/
cp rke2-cilium.yml /var/lib/rancher/rke2/server/manifests/
```

For more information about the full options of the Cilium config please refer to the [rke2-charts](https://github.com/rancher/rke2-charts/blob/main-source/packages/rke2-cilium/charts/values.yaml).
