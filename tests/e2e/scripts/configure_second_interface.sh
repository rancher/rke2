#!/bin/bash

ip4_addr=$1
ip6_addr=$2
ip6_addr_gw=$3
os=$4


# Dual-stack case
if [ -n "$ip6_addr" ]; then
    echo "Configuring Dual-Stack"
    
    # Enable IPv6 at the system level
    sysctl -w net.ipv6.conf.all.disable_ipv6=0
    sysctl -w net.ipv6.conf.eth1.accept_dad=0
    echo "net.ipv6.conf.all.disable_ipv6=0
net.ipv6.conf.eth1.accept_dad=0" > /etc/sysctl.conf

    if [ -z "${os##*ubuntu*}" ]; then
        # Add IPv6 to the existing Netplan config
        netplan set ethernets.eth1.accept-ra=false
        netplan set ethernets.eth1.addresses=["$ip4_addr"/24,"$ip6_addr"/64]
        netplan set ethernets.eth1.gateway6="$ip6_addr_gw"
        netplan apply
    elif [ -z "${os##*alpine*}" ]; then
        ip link set eth1 down
        ip link set eth1 up
        ip -6 addr add "$ip6_addr"/64 dev eth1
        ip -6 r add default via "$ip6_addr_gw"
    else
        ip -6 addr add "$ip6_addr"/64 dev eth1
        ip -6 r add default via "$ip6_addr_gw"
    fi
else
    # ipv4-only
    echo "IPv6 address not detected. Proceeding with IPv4-only configuration."
    if [ -z "${os##*ubuntu*}" ]; then
        netplan set ethernets.eth1.addresses=["$ip4_addr"/24]
        netplan apply
    fi
fi