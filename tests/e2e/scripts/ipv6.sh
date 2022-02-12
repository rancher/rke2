#!/bin/bash
ip6_prefix=$1
ip6_addr=$2

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.default.disable_ipv6=0
ip -6 addr add "$ip6_addr"/64 dev eth1
sleep 5
dhcplink=$(ip -6 addr show to "$ip6_prefix"::/64 | grep -oP 'inet6 \K(\S+)' | grep "$ip6_prefix:[0-9].*")
ip -6 addr del "$dhcplink" dev eth1
