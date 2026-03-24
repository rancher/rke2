#!/bin/bash
# Override default flannel and specify the interface to use
mkdir -p /var/lib/rancher/rke2/server/manifests

echo "Creating flannel chart"
echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-flannel
  namespace: kube-system
spec:
  valuesContent: |-
    flannel:
      args:
      - \"--ip-masq\"
      - \"--kube-subnet-mgr\"
      - \"--iface=eth1\"" >> /var/lib/rancher/rke2/server/manifests/e2e-flannel.yaml
