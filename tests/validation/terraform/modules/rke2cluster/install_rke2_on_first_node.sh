#!/bin/bash
echo $@

mkdir -p /etc/rancher/rke2
cat << EOF >/etc/rancher/rke2/flags.conf
write-kubeconfig-mode: "0644"
tls-san:
  - ${4}
EOF

if [ ${1} = "ubuntu" ]
then
  wget https://raw.githubusercontent.com/rancher/rke2/master/install.sh
  chmod u+x install.sh
  INSTALL_RKE2_VERSION=${5} ./install.sh
fi

if [ ${1} = "rhel" ]
then
    subscription-manager register --auto-attach --username=${2} --password=${3}
    sleep 30
    subscription-manager repos --enable=rhel-7-server-extras-rpms
    sleep 20
fi

if [ ${1} = "rhel" ] || [ ${1} = "centos" ]
then
cat <<EOF >>/etc/yum.repos.d/rpm-rancher-io.repo
[rancher-rke2-common-testing]
name=Rancher RKE2 Common Testing
baseurl=https://rpm-testing.rancher.io/rke2/testing/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm-testing.rancher.io/public.key
[rancher-rke2-1-18-testing]
name=Rancher RKE2 1.18 Testing
baseurl=https://rpm-testing.rancher.io/rke2/testing/1.18/centos/7/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm-testing.rancher.io/public.key
EOF
echo "ARGUMENTS 5 ${5} 7 ${7}"

if [ "${5}" == "null" ]
then
  echo "DDDDDD"
  echo "null" >/tmp/nullmainmaster1
  yum -y install rke2-server
else
  echo "not null" >/tmp/notnullmainmaster1
  yum -y install rke2-server${5}
fi
sleep 30
systemctl start rke2-server
fi
sleep 20


cat /etc/rancher/rke2/flags.conf > /tmp/joinflags
cat /var/lib/rancher/rke2/server/node-token >/tmp/nodetoken
cat /etc/rancher/rke2/rke2.yaml >/tmp/config
