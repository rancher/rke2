#!/bin/bash
ip6_addr=$1

netplan set ethernets.eth1.accept-ra=false
netplan apply

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.eth1.accept_dad=0
ip -6 addr add "$ip6_addr"/64 dev eth1
ip -6 addr show dev eth1
# Override default canal and specify the interface since we don't have a default IPv6 route
mkdir -p /var/lib/rancher/rke2/server/manifests
echo "apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-canal
  namespace: kube-system
spec:
  valuesContent: |-
    flannel:
      iface: \"eth1\"" >> /var/lib/rancher/rke2/server/manifests/e2e-canal.yaml