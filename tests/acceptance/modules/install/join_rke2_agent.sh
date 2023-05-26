#!/bin/bash
# This script is used to join one or more nodes as agents
set -x
echo "$@"


node_os=$1
server_ip=$2
token=$3
rke2_version=$4
public_ip=$5
rke2_channel=$6
worker_flags=$7
install_mode=$8
rhel_username=$9
rhel_password=${10}
install_method=${11}

hostname=$(hostname -f)
mkdir -p /etc/rancher/rke2
cat <<EOF >>/etc/rancher/rke2/config.yaml
server: https://${server_ip}:9345
token:  "${token}"
node-name: "${hostname}"
EOF

if [ -n "$worker_flags" ] && [[ "$worker_flags" == *":"* ]]
then
   echo "$worker_flags"
   echo -e "$worker_flags" >> /etc/rancher/rke2/config.yaml
   if [[ "$worker_flags" != *"cloud-provider-name"* ]]
   then
     echo -e "node-external-ip: $public_ip" >> /etc/rancher/rke2/config.yaml
   fi
   cat /etc/rancher/rke2/config.yaml
else
  echo -e "node-external-ip: $public_ip" >> /etc/rancher/rke2/config.yaml
fi

if [[ "$node_os" = "rhel" ]]
then
   subscription-manager register --auto-attach --username="$rhel_username" --password="$rhel_password"
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [ "$node_os" = "centos8" ] || [ "$node_os" = "rhel8" ] || [ "$node_os" = "oracle8" ]
then
  yum install tar -y
  yum install iptables -y
  workaround="[keyfile]\nunmanaged-devices=interface-name:cali*;interface-name:tunl*;interface-name:vxlan.calico;interface-name:flannel*"
  if [ ! -e /etc/NetworkManager/conf.d/canal.conf ]; then
    echo -e "$workaround" > /etc/NetworkManager/conf.d/canal.conf
  else
    echo -e "$workaround" >> /etc/NetworkManager/conf.d/canal.conf
  fi
  sudo systemctl reload NetworkManager
fi

export "$install_mode"="$rke2_version"
if [ -n "$install_method" ]
then
  export INSTALL_RKE2_METHOD="$install_method"
fi

if [ "$rke2_channel" != "null" ]
then
    curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL="$rke2_channel" INSTALL_RKE2_TYPE='agent' sh -
else
    curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE='agent' sh -
fi
if [ -n "$worker_flags" ] && [[ "$worker_flags" == *"cis"* ]]
then
    if [[ "$node_os" == *"rhel"* ]] || [[ "$node_os" == *"centos"* ]] || [[ "$node_os" == *"oracle"* ]]
    then
        cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
    else
        cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
    fi
    systemctl restart systemd-sysctl
fi
sudo systemctl enable rke2-agent
sudo systemctl start rke2-agent