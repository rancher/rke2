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
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w 'rke2/data/[^/]*/bin/containerd-shim' | cut -f1
}

do_unmount() {
    { set +x; } 2>/dev/null
    MOUNTS=
    while read ignore mount ignore; do
        MOUNTS="${mount}\n${MOUNTS}"
    done </proc/self/mounts
    MOUNTS=$(printf ${MOUNTS} | grep "^$1" | sort -r)
    if [ -n "${MOUNTS}" ]; then
        set -x
        umount ${MOUNTS}
    else
        set -x
    fi
}

export PATH=$PATH:/var/lib/rancher/rke2/bin

set -x

systemctl stop rke2-server.service || true
systemctl stop rke2-agent.service || true

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount '/run/k3s'
do_unmount '/var/lib/rancher/rke2'
do_unmount '/var/lib/kubelet/pods'
do_unmount '/run/netns/cni-'

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
    iface=${iface%%@*}
    [ -z "$iface" ] || ip link delete $iface
done
ip link delete cni0
ip link delete flannel.1
ip link delete vxlan.calico
ip link delete cilium_vxlan
ip link delete cilium_net

#Delete the nodeLocal created objects
if [ -d /sys/class/net/nodelocaldns ]; then
  for i in $(ip address show nodelocaldns | grep inet | awk '{print $2}');
  do
    iptables-save | grep -v $i | iptables-restore
  done
  ip link delete nodelocaldns
fi

rm -rf /var/lib/cni/
iptables-save | grep -v KUBE- | grep -v CNI- | grep -v cali- | grep -v cali: | grep -v CILIUM_ | iptables-restore
