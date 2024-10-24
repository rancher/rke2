#!/bin/sh

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

pschildren() {
    ps -e -o ppid= -o pid= | \
    sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
    grep -w "^$1" | \
    cut -f2
}

pstree() {
    for pid in "$@"; do
        echo ${pid}
        for child in $(pschildren ${pid}); do
            pstree ${child}
        done
    done
}

killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        pstree "$@";
        set -x;
    ) 2>/dev/null
}

getshims() {
    COLUMNS=2147483647 ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w "${RKE2_DATA_DIR}"'/data/[^/]*/bin/containerd-shim' | cut -f1
}

do_unmount_and_remove() {
    { set +x; } 2>/dev/null
    MOUNTS=
    while read ignore mount ignore; do
        MOUNTS="${mount}\n${MOUNTS}"
    done </proc/self/mounts
    MOUNTS=$(printf ${MOUNTS} | grep "^$1" | sort -r)
    if [ -n "${MOUNTS}" ]; then
        set -x
        umount -- ${MOUNTS} && rm -rf --one-file-system -- ${MOUNTS}
    else
        set -x
    fi
}

RKE2_DATA_DIR=${RKE2_DATA_DIR:-/var/lib/rancher/rke2}

export PATH=$PATH:${RKE2_DATA_DIR}/bin

set -x

systemctl stop rke2-server.service || true
systemctl stop rke2-agent.service || true

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount_and_remove '/run/k3s'
do_unmount_and_remove '/var/lib/kubelet/pods'
do_unmount_and_remove '/run/netns/cni-'

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
    iface=${iface%%@*}
    [ -z "$iface" ] || ip link delete $iface
done
ip link delete cni0
ip link delete flannel.1
ip link delete flannel.4096
ip link delete flannel-v6.1
ip link delete flannel-v6.4096
ip link delete flannel-wg
ip link delete flannel-wg-v6
ip link delete vxlan.calico
ip link delete vxlan-v6.calico
ip link delete cilium_vxlan
ip link delete cilium_net
ip link delete cilium_wg0
ip link delete kube-ipvs0

#Delete the nodeLocal created objects
if [ -d /sys/class/net/nodelocaldns ]; then
  for i in $(ip address show nodelocaldns | grep inet | awk '{print $2}');
  do
    iptables-save | grep -v $i | iptables-restore
  done
  ip link delete nodelocaldns
fi

rm -rf /var/lib/cni/ /var/log/pods/ /var/log/containers

# Remove pod-manifests files for rke2 components
POD_MANIFESTS_DIR=${RKE2_DATA_DIR}/agent/pod-manifests

rm -f "${POD_MANIFESTS_DIR}/etcd.yaml" \
      "${POD_MANIFESTS_DIR}/kube-apiserver.yaml" \
      "${POD_MANIFESTS_DIR}/kube-controller-manager.yaml" \
      "${POD_MANIFESTS_DIR}/cloud-controller-manager.yaml" \
      "${POD_MANIFESTS_DIR}/kube-scheduler.yaml" \
      "${POD_MANIFESTS_DIR}/kube-proxy.yaml"

# Delete iptables created by CNI plugins or Kubernetes (kube-proxy)
iptables-save | grep -v KUBE- | grep -v CNI- | grep -v cali- | grep -v cali: | grep -v CILIUM_ | grep -v flannel | iptables-restore
ip6tables-save | grep -v KUBE- | grep -v CNI- | grep -v cali- | grep -v cali: | grep -v CILIUM_ | grep -v flannel | ip6tables-restore

set +x

echo 'If this cluster was upgraded from an older release of the Canal CNI, you may need to manually remove some flannel iptables rules:'
echo -e '\texport cluster_cidr=YOUR-CLUSTER-CIDR'
echo -e '\tiptables -D POSTROUTING -s $cluster_cidr -j MASQUERADE --random-fully'
echo -e '\tiptables -D POSTROUTING ! -s $cluster_cidr -d  -j MASQUERADE --random-fully'
