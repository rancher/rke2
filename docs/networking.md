# Networking

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

# Nodes Without a Hostname

Some cloud providers, such as Linode, will create machines with "localhost" as the hostname and others may not have a hostname set at all. This can cause problems with domain name resolution. You can run RKE2 with the `--node-name` flag or `RKE2_NODE_NAME` environment variable and this will pass the node name to resolve this issue.
