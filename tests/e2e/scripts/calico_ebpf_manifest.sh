#!/bin/bash
ip4_addr=$1

# Set Calico parameters to use the eBPF dataplane instead of iptables
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
          canReach: $ip4_addr 
        bpfNetworkBootstrap: Enabled
        kubeProxyManagement: Enabled
        linuxDataplane: BPF
    kubernetesServiceEndpoint:
      host: $ip4_addr" >> /var/lib/rancher/rke2/server/manifests/e2e-calico.yaml
