#!/bin/bash
# Configure an HAProxy + Keepalived load balancer that fronts the RKE2 control plane,
# reproducing https://docs.rke2.io/networking/cluster-loadbalancer
#
# Usage: loadbalancer_setup.sh <vip> <interface> <server_ip> [<server_ip> ...]
set -e

VIP=$1
INTERFACE=$2
shift 2
SERVER_IPS=("$@")

if [ -z "$VIP" ] || [ -z "$INTERFACE" ] || [ ${#SERVER_IPS[@]} -eq 0 ]; then
  echo "Usage: $0 <vip> <interface> <server_ip> [<server_ip> ...]"
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y haproxy keepalived psmisc

# Allow HAProxy to bind the VIP even before keepalived assigns it locally.
echo 'net.ipv4.ip_nonlocal_bind=1' > /etc/sysctl.d/99-nonlocal-bind.conf
sysctl -p /etc/sysctl.d/99-nonlocal-bind.conf

# Build the HAProxy backend server lines for both the supervisor (9345) and API (6443).
rke2_backend=""
k8s_backend=""
i=1
for ip in "${SERVER_IPS[@]}"; do
  rke2_backend+="    server server-$i ${ip}:9345 check\n"
  k8s_backend+="    server server-$i ${ip}:6443 check\n"
  i=$((i + 1))
done

cat > /etc/haproxy/haproxy.cfg <<EOF
global
    log /dev/log local0
    maxconn 4096

defaults
    log     global
    mode    tcp
    option  tcplog
    timeout connect 10s
    timeout client  30s
    timeout server  30s

frontend rke2-frontend
    bind *:9345
    mode tcp
    default_backend rke2-backend

frontend k8s-api-frontend
    bind *:6443
    mode tcp
    default_backend k8s-api-backend

backend rke2-backend
    mode tcp
    option tcp-check
    balance roundrobin
    default-server inter 10s downinter 5s
$(printf "%b" "$rke2_backend")

backend k8s-api-backend
    mode tcp
    option tcp-check
    balance roundrobin
    default-server inter 10s downinter 5s
$(printf "%b" "$k8s_backend")
EOF

cat > /etc/keepalived/keepalived.conf <<EOF
global_defs {
    enable_script_security
    script_user root
}

vrrp_script chk_haproxy {
    script 'killall -0 haproxy'
    interval 2
}

vrrp_instance haproxy-vip {
    interface ${INTERFACE}
    state MASTER
    priority 200
    virtual_router_id 51

    virtual_ipaddress {
        ${VIP}/24
    }

    track_script {
        chk_haproxy
    }
}
EOF

systemctl enable keepalived haproxy
systemctl restart keepalived
systemctl restart haproxy

echo "HAProxy + Keepalived configured with VIP ${VIP} on ${INTERFACE}"
