#!/bin/bash
cni=$1
filter=$2

# Override default CNI and specify the interface since we don't have a default IPv6 route
mkdir -p /var/lib/rancher/rke2/server/manifests

case "$cni" in
  *canal*)
    enable_nftables=false
    [ "$filter" = "nftables" ] && enable_nftables=true
    felix_iptables_backend=auto
    [ "$filter" = "nftables" ] && felix_iptables_backend=nft
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
      enableNFTables: ${enable_nftables}
    calico:
      ipAutoDetectionMethod: \"interface=eth1.*\"
      ip6AutoDetectionMethod: \"interface=eth1.*\"
      felixIptablesBackend: ${felix_iptables_backend}" >> /var/lib/rancher/rke2/server/manifests/rke2-canal-config.yaml
  ;;
  
  *flannel*)
    enable_nftables=false
    [ "$filter" = "nftables" ] && enable_nftables=true
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
      - \"--iface=eth1\"
      enableNFTables: ${enable_nftables}" >> /var/lib/rancher/rke2/server/manifests/rke2-flannel-config.yaml
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
      enabled: true">> /var/lib/rancher/rke2/server/manifests/rke2-cilium-config.yaml
  ;;
  
  *calico*)
    linux_dataplane=Iptables
    [ "$filter" = "nftables" ] && linux_dataplane=Nftables
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
        linuxDataplane: ${linux_dataplane}
        nodeAddressAutodetectionV4:
          interface: eth1.* 
        nodeAddressAutodetectionV6:
          interface: eth1.* " >> /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml
  ;;
esac
