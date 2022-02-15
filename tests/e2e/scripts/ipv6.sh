#!/bin/bash
ip6_prefix=$1
ip6_addr=$2

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.default.disable_ipv6=0
ip -6 addr add "$ip6_addr"/64 dev eth1
# ip -6 route add default via "$ip6_prefix"::1
# Wait for dhcp6 link to come up and remove it
iterations=0
numlinks="$(ip -6 addr show to "$ip6_prefix"::/64 | grep -oP 'inet6 \K(\S+)' | wc -l)"
while [ "$numlinks" -ne 2 ]; do
    ((iterations++))
    if [ "$iterations" -ge 60 ]; then
        echo "dhcp6 address never came up"
        exit 1
    fi
    sleep 1
    numlinks="$(ip -6 addr show to "$ip6_prefix"::/64 | grep -oP 'inet6 \K(\S+)' | wc -l)"
done
dhcplink=$(ip -6 addr show to "$ip6_prefix"::/64 | grep -oP 'inet6 \K(\S+)' | grep "$ip6_prefix:[0-9].*")
ip -6 addr del "$dhcplink" dev eth1

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
      iface: \"eth1\"" >> /var/lib/rancher/rke2/server/manifests/canal.yaml