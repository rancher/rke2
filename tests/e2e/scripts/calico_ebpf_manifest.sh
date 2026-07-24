#!/bin/bash

# Set Calico parameters to use the eBPF dataplane instead of iptables
mkdir -p /var/lib/rancher/rke2/server/manifests

NODE_IP=${1}

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
      host: ${NODE_IP}" > /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml
