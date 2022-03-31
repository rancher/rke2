#!/bin/bash
ip6_addr=$1
cni=$2
os=$3

if [ -z "${os##*ubuntu2004*}" ]; then
  netplan set ethernets.eth1.accept-ra=false
  netplan apply
fi

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.eth1.accept_dad=0
ip -6 addr add "$ip6_addr"/64 dev eth1
ip addr show dev eth1
# Override default canal and specify the interface since we don't have a default IPv6 route
mkdir -p /var/lib/rancher/rke2/server/manifests

case "$cni" in
  "canal")
    echo "Creating canal chart"
    echo "apiVersion: helm.cattle.io/v1
    kind: HelmChartConfig
    metadata:
      name: rke2-canal
      namespace: kube-system
    spec:
      valuesContent: |-
        flannel:
          iface: \"eth1\"" >> /var/lib/rancher/rke2/server/manifests/e2e-canal.yaml
  ;;
  "cilum")
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
  "calico")
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



