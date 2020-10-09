# Networking

This page explains how CoreDNS, the Nginx-Ingress controller, and Klipper service load balancer work within RKE2.

Refer to the [Installation Network Options](install/network_options.md) page for details on Canal configuration options, or how to set up your own CNI.

For information on which ports need to be opened for RKE2, refer to the [Installation Requirements](install/requirements.md).

- [CoreDNS](#coredns)
- [Nginx-Ingress Controller](#nginx-ingress-controller)
- [Service Load Balancer](#service-load-balancer)
  - [How the Service LB Works](#how-the-service-lb-works)
  - [Usage](#usage)
  - [Excluding the Service LB from Nodes](#excluding-the-service-lb-from-nodes)
  - [Disabling the Service LB](#disabling-the-service-lb)

# CoreDNS

CoreDNS is deployed by default when starting the server. To disable, run each server with `disable: rke2-coredns` option in your configuration file.

If you don't install CoreDNS, you will need to install a cluster DNS provider yourself.

# Nginx Ingress Controller

[nginx-ingress](https://github.com/kubernetes/ingress-nginx) is an Ingress controller that uses ConfigMap to store the nginx configuration.

Nginx-ingress is deployed by default when starting the server. The ingress controller will use ports 80, and 443 on the host (i.e. these will not be usable for HostPort or NodePort).

Nginx-ingress can be configured by creating a [HelmChartConfig manifest](helm.md#customizing-packaged-components-with-helmchartconfig) to customize the `rke2-nginx-ingress` HelmChart values. For more information, refer to the official [nginx-ingress Helm Configuration Parameters](https://github.com/helm/charts/tree/cfcf87ac254dcbb2d4aa1c866e20dd7e8e55b8e5/stable/nginx-ingress#configuration).

To disable it, start each server with the `disable: rke2-ingress-nginx` option in your configuration file.

# Nodes Without a Hostname

Some cloud providers, such as Linode, will create machines with "localhost" as the hostname and others may not have a hostname set at all. This can cause problems with domain name resolution. You can run RKE2 with the `node-name` parameter and this will pass the node name to resolve this issue.

# CNI options

### Replacing the default CNI

By default RKE2 deploys the Canal CNI which uses flannel for networking and provides calico-felix to enable networkpolicy support.  If you wish to use a CNI other than Canal, you can disable it by adding the `disable: rke2-canal` option in your configuration file.

### Running Calico as your CNI

In order to deploy Calico as your CNI you will need to set a few options in your RKE2 config file.

```
service-cidr: "10.41.0.0/16"
cluster-cidr: "10.42.0.0/16"
disable:
  - rke2-canal
```

You may need to modify your cluster-cidr and service-cidr depending on your specific network environment.

Once you have created or updated your config file with the options above, you will need to create the Calico manifests in the RKE2 server manifests directory at `/var/lib/rancher/rke2/server/manifests/`

Copy the tigera-operator manifest from https://docs.projectcalico.org/manifests/tigera-operator.yaml

Copy the Calico custom resources manifest from https://docs.projectcalico.org/manifests/custom-resources.yaml

You may wish to modify settings in these manifests, or you can use the defaults.  Once these manifests are copied over and the config file options are set, you can restart the RKE2 server service on your server nodes with `sudo systemctl restart rke2-server` for the changes to take effect and the manifests to deploy.

<b>Warning:</b> It is not recommended to switch CNI providers once you have already deployed one as it will cause downtime and issues.  It's best to deploy a fresh cluster with the default CNI disabled before installing another CNI by following the instructions above.
