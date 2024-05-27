#!/bin/bash
ip4_addr=$1

# Set Cilium parameters to get as much BPF as possible and as a consequence
# as less iptables rules as possible
mkdir -p /var/lib/rancher/rke2/server/manifests

echo "Creating cilium chart"
echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-cilium
  namespace: kube-system
spec:
  valuesContent: |-
    ipv6:
      enabled: true
    devices: eth1
    kubeProxyReplacement: true
    k8sServiceHost: $ip4_addr
    k8sServicePort: 6443
    cni:
      chainingMode: none
    bpf:
      masquerade: true" > /var/lib/rancher/rke2/server/manifests/e2e-cilium.yaml