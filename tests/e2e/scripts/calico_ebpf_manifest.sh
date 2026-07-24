#!/bin/bash

# Set Calico parameters to use the eBPF dataplane instead of iptables.
# Optional first arg overrides kubernetesServiceEndpoint.host (defaults to localhost).
# When kube-proxy is disabled behind an external load balancer, this must point at the
# load balancer VIP so pods can reach the API server. See
# https://docs.rke2.io/networking/cluster-loadbalancer
SERVICE_ENDPOINT_HOST=${1:-localhost}
mkdir -p /var/lib/rancher/rke2/server/manifests

echo "Creating calico chart"
echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-calico
  namespace: kube-system
spec:
  valuesContent: |-
    installation:
      calicoNetwork:
        nodeAddressAutodetectionV4:
          interface: eth1.* 
        nodeAddressAutodetectionV6:
          interface: eth1.* 
        kubeProxyManagement: Enabled
        linuxDataplane: BPF
    kubernetesServiceEndpoint:
      host: ${SERVICE_ENDPOINT_HOST}" > /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml
