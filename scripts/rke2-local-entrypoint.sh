#!/usr/bin/env bash
set -euo pipefail

mkdir -p /var/lib/kubelet /var/lib/rancher/rke2 /var/lib/cni /var/log

# RKE2-in-container needs shared mount propagation for kubelet paths.
mount --make-rshared /var/lib/kubelet 2>/dev/null || true
mount --make-rshared / 2>/dev/null || true

exec /usr/local/bin/rke2 "$@"
