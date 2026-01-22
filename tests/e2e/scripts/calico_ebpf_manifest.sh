#!/bin/bash
ip4_addr=$1
ip6_addr=$2
ip6_addr_gw=$3
os=$4

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.eth1.accept_dad=0

if [ -z "${os##*ubuntu*}" ]; then
  netplan set ethernets.eth1.accept-ra=false
  netplan set ethernets.eth1.addresses=["$ip4_addr"/24,"$ip6_addr"/64]
  netplan set ethernets.eth1.gateway6="$ip6_addr_gw"
  netplan apply
elif [ -z "${os##*alpine*}" ]; then
  iplink set eth1 down
  iplink set eth1 up
  ip -6 addr add "$ip6_addr"/64 dev eth1
  ip -6 r add default via "$ip6_addr_gw"
else
  ip -6 addr add "$ip6_addr"/64 dev eth1
  ip -6 r add default via "$ip6_addr_gw"
fi
ip addr show dev eth1
ip -6 r

echo "net.ipv6.conf.all.disable_ipv6=0
net.ipv6.conf.eth1.accept_dad=0" > /etc/sysctl.conf

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
          interface: eth1.* 
        nodeAddressAutodetectionV6:
          interface: eth1.* 
        kubeProxyManagement: Enabled
        linuxDataplane: BPF
    kubernetesServiceEndpoint:
      host: localhost" > /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml
