#!/bin/bash
mkdir -p /etc/rancher/rke2
cat <<EOF >>/etc/rancher/rke2/config.yaml
server: https://${3}:9345
token:  "${4}"
node-external-ip: "${3}"
EOF
if [ ! -z "${8}" ] && [[ "${8}" == *":"* ]]
then
   echo "${8}"
   echo -e "${8}" >> /etc/rancher/rke2/config.yaml
   cat /etc/rancher/rke2/config.yaml
fi

if [[ ${1} == *"rhel"* ]]
then
   subscription-manager register --auto-attach --username=${9} --password=${10}
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [ ${1} = "centos8" ] || [ ${1} = "rhel8" ]
then
  yum install tar -y
  yum install iptables -y
  workaround="[keyfile]\nunmanaged-devices=interface-name:cali*;interface-name:tunl*;interface-name:vxlan.calico;interface-name:flannel*"
  if [ ! -e /etc/NetworkManager/conf.d/canal.conf ]; then
    echo -e $workaround > /etc/NetworkManager/conf.d/canal.conf
  else
    echo -e $workaround >> /etc/NetworkManager/conf.d/canal.conf
  fi
  sudo systemctl reload NetworkManager
fi

export "${7}"="${5}"

if [ ${6} != "null" ]
   then
       curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=${6} INSTALL_RKE2_TYPE='agent' sh -
   else
       curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE='agent' sh -
fi

if [ ! -z "${8}" ] && [[ "${8}" == *"cis"* ]]
then
   if [[ ${1} == *"rhel"* ]] || [[ ${1} == *"centos"* ]]
   then
        cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
   else
        cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
    fi
    systemctl restart systemd-sysctl
    useradd -r -c "etcd user" -s /sbin/nologin -M etcd
fi
sudo systemctl enable rke2-agent
sudo systemctl start rke2-agent
