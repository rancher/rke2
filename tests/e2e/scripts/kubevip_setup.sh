#!/bin/bash
# Deploy kube-vip as the control-plane load balancer for an HA RKE2 cluster, providing the
# fixed registration address (VIP) on the supervisor (9345) and API server (6443) ports.
#
# Unlike the HAProxy + Keepalived setup (see loadbalancer_setup.sh), kube-vip does NOT need a
# dedicated VM: it runs as a DaemonSet on the control-plane (server) nodes and, in ARP mode,
# floats the VIP onto whichever server node currently holds the leader lease. That node already
# serves 9345 and 6443 locally, so no separate load balancing is required.
#
# The manifest is written into RKE2's auto-deploy directory so it is applied automatically once
# the cluster-init server comes up. It must therefore run on every server node before RKE2 starts.
#
# Usage: kubevip_setup.sh <vip> <interface> [image]
set -e

VIP=$1
INTERFACE=$2
IMAGE=${3:-ghcr.io/kube-vip/kube-vip:v0.8.9}

if [ -z "$VIP" ] || [ -z "$INTERFACE" ]; then
  echo "Usage: $0 <vip> <interface> [image]"
  exit 1
fi

mkdir -p /var/lib/rancher/rke2/server/manifests

cat > /var/lib/rancher/rke2/server/manifests/kube-vip.yaml <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-vip
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  name: system:kube-vip-role
rules:
  - apiGroups: [""]
    resources: ["services", "services/status", "nodes", "endpoints"]
    verbs: ["list", "get", "watch", "update"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["list", "get", "watch", "update", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:kube-vip-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kube-vip-role
subjects:
  - kind: ServiceAccount
    name: kube-vip
    namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-vip-ds
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-vip-ds
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-vip-ds
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kube-vip-ds
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
              - matchExpressions:
                  - key: node-role.kubernetes.io/master
                    operator: Exists
      containers:
        - name: kube-vip
          image: ${IMAGE}
          imagePullPolicy: IfNotPresent
          args:
            - manager
          env:
            - name: vip_arp
              value: "true"
            - name: port
              value: "6443"
            - name: vip_nodename
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: vip_interface
              value: "${INTERFACE}"
            - name: vip_cidr
              value: "32"
            - name: dns_mode
              value: first
            - name: cp_enable
              value: "true"
            - name: cp_namespace
              value: kube-system
            - name: svc_enable
              value: "false"
            - name: vip_leaderelection
              value: "true"
            - name: vip_leasename
              value: plndr-cp-lock
            - name: vip_leaseduration
              value: "5"
            - name: vip_renewdeadline
              value: "3"
            - name: vip_retryperiod
              value: "1"
            - name: address
              value: "${VIP}"
          securityContext:
            capabilities:
              add:
                - NET_ADMIN
                - NET_RAW
      hostNetwork: true
      serviceAccountName: kube-vip
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - effect: NoExecute
          operator: Exists
EOF

echo "kube-vip manifest written with VIP ${VIP} on ${INTERFACE} using ${IMAGE}"
