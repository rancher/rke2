#!/bin/bash
ip4_addr=$1

# Override default calico and specify the interface for windows-agent
# by default, the windows-agent use a different interface name than the linux-agent
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
          canReach: $ip4_addr" >> /var/lib/rancher/rke2/server/manifests/e2e-calico.yaml
