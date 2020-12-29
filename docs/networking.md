# Networking

This page explains how CoreDNS and the Nginx-Ingress controller work within RKE2.

Refer to the [Installation Network Options](install/network_options.md) page for details on Canal configuration options, or how to set up your own CNI.

For information on which ports need to be opened for RKE2, refer to the [Installation Requirements](install/requirements.md).

- [CoreDNS](#coredns)
- [Nginx Ingress Controller](#nginx-ingress-controller)
- [Nodes Without a Hostname](#nodes-without-a-hostname)

## CoreDNS

CoreDNS is deployed by default when starting the server. To disable, run each server with `disable: rke2-coredns` option in your configuration file.

If you don't install CoreDNS, you will need to install a cluster DNS provider yourself.

## Nginx Ingress Controller

[nginx-ingress](https://github.com/kubernetes/ingress-nginx) is an Ingress controller powered by NGINX that uses a ConfigMap to store the NGINX configuration.

`nginx-ingress` is deployed by default when starting the server. Ports 80 and 443 will be bound by the ingress controller in its default configuration, making these unusable for HostPort or NodePort services in the cluster.

Configuration options can be specified by creating a [HelmChartConfig manifest](helm.md#customizing-packaged-components-with-helmchartconfig) to customize the `rke2-ingress-nginx` HelmChart values. For example, a HelmChartConfig at `/var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx-config.yaml` with the following contents sets `use-forwarded-headers` to `"true"` in the ConfigMap storing the NGINX config:
```yaml
# /var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx-config.yaml
---
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-ingress-nginx
  namespace: kube-system
spec:
  valuesContent: |-
    controller:
      config:
        use-forwarded-headers: "true"
```
For more information, refer to the official [nginx-ingress Helm Configuration Parameters](https://github.com/kubernetes/ingress-nginx/tree/9c0a39636da11b7e262ddf0b4548c79ae9fa1667/charts/ingress-nginx#configuration).

To disable the NGINX ingress controller, start each server with the `disable: rke2-ingress-nginx` option in your configuration file.

## Nodes Without a Hostname

Some cloud providers, such as Linode, will create machines with "localhost" as the hostname and others may not have a hostname set at all. This can cause problems with domain name resolution. You can run RKE2 with the `node-name` parameter and this will pass the node name to resolve this issue.
