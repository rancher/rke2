#!/bin/bash

# Set Calico parameters to use the eBPF dataplane instead of iptables.
# Optional first arg overrides kubernetesServiceEndpoint.host (defaults to localhost);
# optional second arg overrides kubernetesServiceEndpoint.port (defaults to 6443).
# optional third arg controls IPv6 node autodetection (defaults to true).
# When kube-proxy is disabled behind an external load balancer, these must point at the
# load balancer VIP/API port so pods can reach the API server. See
# https://docs.rke2.io/networking/cluster-loadbalancer
SERVICE_ENDPOINT_HOST=${1:-localhost}
SERVICE_ENDPOINT_PORT=${2:-6443}
ENABLE_IPV6_AUTODETECTION=${3:-true}
mkdir -p /var/lib/rancher/rke2/server/manifests

echo "Creating calico chart"
cat > /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml <<EOF
apiVersion: helm.cattle.io/v1
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
EOF

if [ "$ENABLE_IPV6_AUTODETECTION" != "false" ]; then
cat >> /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml <<EOF
        nodeAddressAutodetectionV6:
          interface: eth1.*
EOF
fi

cat >> /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml <<EOF
        kubeProxyManagement: Enabled
        linuxDataplane: BPF
    kubernetesServiceEndpoint:
      host: "${SERVICE_ENDPOINT_HOST}"
      port: "${SERVICE_ENDPOINT_PORT}"
EOF
