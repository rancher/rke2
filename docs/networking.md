title: "Networking"
---

This page explains how CoreDNS, the Nginx-Ingress controller, and Klipper service load balancer work within RKE2.

Refer to the [Installation Network Options [FIXME]](#FIXME) page for details on Canal configuration options, or how to set up your own CNI.

For information on which ports need to be opened for RKE2, refer to the [Installation Requirements [FIXME]](#FIXME).

- [CoreDNS](#coredns)
- [Nginx-Ingress Controller](#nginx-ingress-controller)
- [Service Load Balancer](#service-load-balancer)
  - [How the Service LB Works](#how-the-service-lb-works)
  - [Usage](#usage)
  - [Excluding the Service LB from Nodes](#excluding-the-service-lb-from-nodes)
  - [Disabling the Service LB](#disabling-the-service-lb)

# CoreDNS

CoreDNS is deployed by default when starting the server. To disable, run each server with the `--disable rke2-coredns` option.

If you don't install CoreDNS, you will need to install a cluster DNS provider yourself.

# Nginx Ingress Controller

[nginx-ingress](https://github.com/kubernetes/ingress-nginx) is an Ingress controller that uses ConfigMap to store the nginx configuration.

Nginx-ingress is deployed by default when starting the server. The ingress controller will use ports 80, and 443 on the host (i.e. these will not be usable for HostPort or NodePort).

Nginx-ingress can be configured by creating a [HelmChartConfig manifest](helm.md#customizing-packaged-components-with-helmchartconfig) to customize the `rke2-nignix-ingress` HelmChart values. For more information, refer to the official [Traefik for Helm Configuration Parameters.](https://github.com/helm/charts/tree/cfcf87ac254dcbb2d4aa1c866e20dd7e8e55b8e5/stable/nginx-ingress#configuration)

To disable it, start each server with the `--disable rke2-ingress-nginx` option.

# Service Load Balancer

Any service load balancer (LB) can be leveraged in your Kubernetes cluster. RKE2 provides a load balancer known as [Klipper Load Balancer](https://github.com/rancher/klipper-lb) that uses available host ports.

Upstream Kubernetes allows a Service of type LoadBalancer to be created, but doesn't include the implementation of the LB. Some LB services require a cloud provider such as Amazon EC2 or Microsoft Azure. By contrast, the RKE2 service LB makes it possible to use an LB service without a cloud provider.

### How the Service LB Works

RKE2 creates a controller that creates a Pod for the service load balancer, which is a Kubernetes object of kind [Service.](https://kubernetes.io/docs/concepts/services-networking/service/)

For each service load balancer, a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) is created. The DaemonSet creates a pod with the `svclb` prefix on each node.

The Service LB controller listens for other Kubernetes Services. After it finds a Service, it creates a proxy Pod for the service using a DaemonSet on all of the nodes. This Pod becomes a proxy to the other Service, so that for example, requests coming to port 8000 on a node could be routed to your workload on port 8888.

If the Service LB runs on a node that has an external IP, it uses the external IP.

If multiple Services are created, a separate DaemonSet is created for each Service.

It is possible to run multiple Services on the same node, as long as they use different ports.

If you try to create a Service LB that listens on port 80, the Service LB will try to find a free host in the cluster for port 80. If no host with that port is available, the LB will stay in Pending.

### Usage

Create a [Service of type LoadBalancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) in RKE2.

### Excluding the Service LB from Nodes

To exclude nodes from using the Service LB, add the following label to the nodes that should not be excluded:

```
svccontroller.rke2.cattle.io/enablelb
```

If the label is used, the service load balancer only runs on the labeled nodes.

### Disabling the Service LB

To disable the embedded LB, run the server with the `--disable servicelb` option.

This is necessary if you wish to run a different LB, such as MetalLB.

# Nodes Without a Hostname

Some cloud providers, such as Linode, will create machines with "localhost" as the hostname and others may not have a hostname set at all. This can cause problems with domain name resolution. You can run RKE2 with the `--node-name` flag or `RKE2_NODE_NAME` environment variable and this will pass the node name to resolve this issue.
