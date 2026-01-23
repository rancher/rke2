#!/bin/bash
cni=$1

# Override default CNI and specify the interface since we don't have a default IPv6 route
mkdir -p /var/lib/rancher/rke2/server/manifests

case "$cni" in
  *canal*)
    echo "Creating canal chart"
    echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-canal
  namespace: kube-system
spec:
  valuesContent: |-
    flannel:
      iface: \"eth1\"
    calico:
      ipAutoDetectionMethod: \"interface=eth1.*\"
      ip6AutoDetectionMethod: \"interface=eth1.*\"" >> /var/lib/rancher/rke2/server/manifests/e2e-canal.yaml
  ;;
  
  *cilium*)
    echo "Creating cilium chart"
    echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-cilium
  namespace: kube-system
spec:
  valuesContent: |-
    devices: eth1
    ipv6:
      enabled: true">> /var/lib/rancher/rke2/server/manifests/e2e-cilium.yaml
  ;;
  
  *calico*)
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
          interface: eth1.* " >> /var/lib/rancher/rke2/server/manifests/e2e-calico.yaml
  ;;
esac
