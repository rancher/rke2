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

# Using Cilium or Calico instead of Canal

Starting with RKE2 v1.21, different CNI Plugins can be deployed instead of Canal. To do so, pass `cilium` or `calico` as the value of the `--cni` flag. To override the default options, use a HelmChartConfig resource, as explained in the previous section. Note that the HelmChartConfig resource names must match the chart names for your selected CNI - `rke2-cilium`, `rke2-calico`, etc.

For more information about values available for the Cilium chart, please refer to the [rke2-charts repository](https://github.com/rancher/rke2-charts/blob/main-source/packages/rke2-cilium/charts/values.yaml)

For more information about values available for the Calico chart, please refer to the [rke2-charts repository](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-calico/rke2-calico/v3.18.1-103/values.yaml)

# Using Multus

Starting with RKE2 v1.21 it is possible to deploy the Multus CNI meta-plugin. Note that this is for advanced users.

[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) is a CNI plugin that enables attaching multiple network interfaces to pods. Multus does not replace CNI plugins, instead it acts as a CNI plugin multiplexer. Multus is useful in certain use cases, especially when pods are network intensive and require extra network interfaces that support dataplane acceleration techniques such as SR-IOV.

Multus can not be deployed standalone. It always requires at least one conventional CNI plugin that fulfills the Kubernetes cluster network requirements. That CNI plugin becomes the default for Multus, and will be used to provide the primary interface for all pods.

To enable Multus, pass `multus` as the first value to the `--cni` flag, followed by the name of the plugin you want to use alongside Multus (or `none` if you will provide your own default plugin). Note that multus must always be in the first position of the list. For example, to use Multus with `canal` as the default plugin you could specify `--cni=multus,canal` or `--cni=multus --cni=canal`.

For more information about Multus, refer to the [multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/docs) documentation.


## Using Multus with the containernetworking plugins

Any CNI plugin can be used as secondary CNI plugin for Multus to provide additional network interfaces attached to a pod. However, it is most common to use the CNI plugins maintained by the containernetworking team (bridge, host-device, macvlan, etc) as secondary CNI plugins for Multus. These containernetworking plugins are automatically deployed when installing Multus. For more information about these plugins, refer to the [containernetworking plugins](https://www.cni.dev/plugins/current) documentation.

To use any of these plugins, a proper NetworkAttachmentDefinition object will need to be created to define the configuration of the secondary network. The definition is then referenced by pod annotations, which Multus will use to provide extra interfaces to that pod. An example using the macvlan cni plugin with Mu is available [in the multus-cni repo](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md#storing-a-configuration-as-a-custom-resource).

## Using Multus with SR-IOV

Using the SR-IOV CNI with Multus can help with data-plane acceleration use cases, providing an extra interface in the pod that can achieve very high throughput. SR-IOV will not work in all environments, and there are several requirements that must be fulfilled to consider the node as SR-IOV capable:

* Physical NIC must support SR-IOV (e.g. by checking /sys/class/net/$NIC/device/sriov_totalvfs)
* The host operating system must activate IOMMU virtualization
* The host operating system includes drivers capable of doing sriov (e.g. i40e, vfio-pci, etc)

The SR-IOV CNI plugin cannot be used as the default CNI plugin for Multus; it must be deployed alongside both Multus and a traditional CNI plugin. The SR-IOV CNI helm chart can be found in the `rancher-charts` Helm repo. For more information see [Rancher Helm Charts documentation](https://rancher.com/docs/rancher/v2.x/en/helm-charts/).

After installing the SR-IOV CNI chart, the SR-IOV operator will be deployed. Then, the user must specify what nodes in the cluster are SR-IOV capable by labeling them with `feature.node.kubernetes.io/network-sriov.capable=true`:

```
kubectl label node $NODE-NAME feature.node.kubernetes.io/network-sriov.capable=true
```

Once labeled, the sriov-network-config Daemonset will deploy a pod to the node to collect information about the network interfaces. That information is available through the `sriovnetworknodestates` Custom Resource Definition. A couple of minutes after the deployment, there will be one `sriovnetworknodestates` resource per node, with the name of the node as the resource name.

For more information about how to use the SR-IOV operator, please refer to [sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator/blob/master/doc/quickstart.md#configuration)
