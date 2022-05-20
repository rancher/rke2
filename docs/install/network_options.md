# Network Options

RKE2 requires a CNI plugin to connect pods and services. The Canal CNI plugin is the default and has been supported since the beginning. Starting with RKE2 v1.21, there are two extra supported CNI plugins: Calico and Cilium. All CNI
plugins get installed via a helm chart after the main components are up and running and can be customized by modifying the helm chart options.

This page focuses on the network options available when setting up RKE2:

- [Install a CNI plugin](#install-a-cni-plugin)
- [Dual-stack configuration](#dual-stack-configuration)
- [Using Multus](#using-multus)


## Install a CNI plugin

The next tabs inform how to deploy each CNI plugin and override the default options:

=== "Canal CNI plugin"
    Canal means using Flannel for inter-node traffic and Calico for intra-node traffic and network policies. By default, it will use vxlan encapsulation to create an overlay network among nodes. Canal is deployed by default in RKE2 and thus nothing must be configured to activate it. To override the default Canal options you should create a HelmChartConfig resource. The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart. For example to override the flannel interface, you can apply the following config:

    ```yaml
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

    Starting with RKE2 v1.23 it is possible to use flannels [wireguard backend](https://github.com/flannel-io/flannel/blob/master/Documentation/backends.md#wireguard) for in-kernel WireGuard encapsulation and encryption ([Users of kernels < 5.6 need to install a module](https://www.wireguard.com/install/)). This can be achieved using the following config:
    
    ```yaml
    apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-canal
      namespace: kube-system
    spec:
      valuesContent: |-
        flannel:
          backend: "wireguard"
    ```
    
    For more information about the full options of the Canal config please refer to the [rke2-charts](https://github.com/rancher/rke2-charts/blob/main-source/packages/rke2-canal/charts/values.yaml).

    Canal is currently no supported in the windows installation of RKE2

=== "Cilium CNI plugin"
    Starting with RKE2 v1.21, Cilium can be deployed as the CNI plugin. To do so, pass `cilium` as the value of the `--cni` flag. To override the default options, please use a HelmChartConfig resource. The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart. For example, to enable eni:

    ```yaml
    apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-cilium
      namespace: kube-system
    spec:
      valuesContent: |-
        eni:
          enabled: true
    ```

    For more information about values available in the Cilium chart, please refer to the [rke2-charts repository](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-cilium/rke2-cilium/1.11.201/values.yaml)

    Cilium includes advanced features to fully replace kube-proxy and implement the routing of services using eBPF instead of iptables. It is not recommended to replace kube-proxy by Cilium if your kernel is not v5.8 or newer, as important bug fixes and features will be missing. To activate this mode, deploy rke2 with the flag `--disable-kube-proxy` and the following cilium configuration:

    ```yaml
    apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-cilium
      namespace: kube-system
    spec:
      valuesContent: |-
        kubeProxyReplacement: strict
        k8sServiceHost: REPLACE_WITH_API_SERVER_IP
        k8sServicePort: REPLACE_WITH_API_SERVER_PORT  
    ```

    For more information, please check the [upstream docs](https://docs.cilium.io/en/v1.11/gettingstarted/kubeproxy-free/)

    Cilium is currently not supported in the Windows installation of RKE2

=== "Calico CNI plugin"
    Starting with RKE2 v1.21, Calico can be deployed as the CNI plugin. To do so, pass `calico` as the value of the `--cni` flag. To override the default options, please use a HelmChartConfig resource. The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart. For example, to change the mtu:

    ```yaml
    apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-calico
      namespace: kube-system
    spec:
      valuesContent: |-
        installation:
          calicoNetwork:
            mtu: 9000
    ```

    For more information about values available for the Calico chart, please refer to the [rke2-charts repository](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-calico/rke2-calico/v3.19.2-204/values.yaml)


## Dual-stack configuration

IPv4/IPv6 dual-stack networking enables the allocation of both IPv4 and IPv6 addresses to Pods and Services. It is supported in RKE2 since v1.21 but not activated by default. To activate it correctly, both RKE2 and the chosen CNI plugin must be configured accordingly. To configure RKE2 in dual-stack mode, it is enough to set a valid IPv4/IPv6 dual-stack cidr for pods and services. To do so, use the flags `--cluster-cidr` and `--service-cidr`, for example:

    ```bash
    --cluster-cidr 10.42.0.0/16,2001:cafe:42:0::/56
    --service-cidr 10.43.0.0/16,2001:cafe:42:1::/112
    ```

Each CNI plugin requires a different configuration for dual-stack:

=== "Canal CNI plugin"

    Canal automatically detects the RKE2 configuration for dual-stack and does not need any extra configuration. Dual-stack is currently not supported in the windows installations of RKE2.

=== "Cilium CNI plugin"

    Enable the ipv6 parameter using a HelmChartConfig:

    ```yaml
    apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-cilium
      namespace: kube-system
    spec:
      valuesContent: |-
        ipv6:
          enabled: true
    ```

=== "Calico CNI plugin"

    Calico automatically detects the RKE2 configuration for dual-stack and does not need any extra configuration. When deployed in dual-stack mode, it creates two different ippool resources. Note that when using dual-stack, calico leverages BGP instead of VXLAN encapsulation. Dual-stack and BGP are currently not supported in the windows installations of RKE2.


## IPv6 setup

In case of IPv6 only configuration RKE2 needs to use `localhost` to access the liveness URL of the ETCD pod; check that your operating system configures `/etc/hosts` file correctly:
```
::1       localhost
```


## Using Multus

Starting with RKE2 v1.21 it is possible to deploy the Multus CNI meta-plugin. Note that this is for advanced users.

[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) is a CNI plugin that enables attaching multiple network interfaces to pods. Multus does not replace CNI plugins, instead it acts as a CNI plugin multiplexer. Multus is useful in certain use cases, especially when pods are network intensive and require extra network interfaces that support dataplane acceleration techniques such as SR-IOV.

Multus can not be deployed standalone. It always requires at least one conventional CNI plugin that fulfills the Kubernetes cluster network requirements. That CNI plugin becomes the default for Multus, and will be used to provide the primary interface for all pods.

To enable Multus, pass `multus` as the first value to the `--cni` flag, followed by the name of the plugin you want to use alongside Multus (or `none` if you will provide your own default plugin). Note that multus must always be in the
first position of the list. For example, to use Multus with `canal` as the default plugin you could specify `--cni=multus,canal` or `--cni=multus --cni=canal`.

For more information about Multus, refer to the [multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/docs) documentation.


## Using Multus with the containernetworking plugins

Any CNI plugin can be used as secondary CNI plugin for Multus to provide additional network interfaces attached to a pod. However, it is most common to use the CNI plugins maintained by the containernetworking team (bridge, host-device,
macvlan, etc) as secondary CNI plugins for Multus. These containernetworking plugins are automatically deployed when installing Multus. For more information about these plugins, refer to the [containernetworking plugins](https://www.cni.dev/plugins/current) documentation.

To use any of these plugins, a proper NetworkAttachmentDefinition object will need to be created to define the configuration of the secondary network. The definition is then referenced by pod annotations, which Multus will use to provide extra interfaces to that pod. An example using the macvlan cni plugin with Mu is available [in the multus-cni repo](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md#storing-a-configuration-as-a-custom-resource).

## Using Multus with SR-IOV (experimental)

**Please note this is an experimental feature introduced with v1.21.2+rke2r1.**

Using the SR-IOV CNI with Multus can help with data-plane acceleration use cases, providing an extra interface in the pod that can achieve very high throughput. SR-IOV will not work in all environments, and there are several requirements
that must be fulfilled to consider the node as SR-IOV capable:

* Physical NIC must support SR-IOV (e.g. by checking /sys/class/net/$NIC/device/sriov_totalvfs)
* The host operating system must activate IOMMU virtualization
* The host operating system includes drivers capable of doing sriov (e.g. i40e, vfio-pci, etc)

The SR-IOV CNI plugin cannot be used as the default CNI plugin for Multus; it must be deployed alongside both Multus and a traditional CNI plugin. The SR-IOV CNI helm chart can be found in the `rancher-charts` Helm repo. For more information see [Rancher Helm Charts documentation](https://rancher.com/docs/rancher/v2.x/en/helm-charts/).

After installing the SR-IOV CNI chart, the SR-IOV operator will be deployed. Then, the user must specify what nodes in the cluster are SR-IOV capable by labeling them with `feature.node.kubernetes.io/network-sriov.capable=true`:

```
kubectl label node $NODE-NAME feature.node.kubernetes.io/network-sriov.capable=true
```

Once labeled, the sriov-network-config Daemonset will deploy a pod to the node to collect information about the network interfaces. That information is available through the `sriovnetworknodestates` Custom Resource Definition. A couple of
minutes after the deployment, there will be one `sriovnetworknodestates` resource per node, with the name of the node as the resource name.

For more information about how to use the SR-IOV operator, please refer to [sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator/blob/master/doc/quickstart.md#configuration)
