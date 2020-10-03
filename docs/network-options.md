# Network Options

By default RKE2 runs Canal as the cni with VXLAN as the default backend, Canal is installed via a helm chart after the main components are up and running.

This page explains how to customize the Canal network plugin by modifying the helm chart options.

# Canal Options

To allow overriding Canal options you should be able to create HelmChartConfig resources, The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart, for example to override Canal Options, you can create the following config:


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

For more information about the full options of the Canal config please refer to the [rke2-charts](https://github.com/rancher/rke2-charts/blob/main-source/packages/rke2-canal/charts/values.yaml).

Note that you need to create this config before installing rke2, to be able to do that you need to run the following command:

```
mkdir -p /var/lib/rancher/rke2/server/manifests/
cp rke2-canal-config.yml /var/lib/rancher/rke2/server/manifests/
```
