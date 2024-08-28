This document lists all of the images used by RKE2 and any of its packaged components (Helm charts), along with info on where they are built and what other images they copy content from. If you find an image not on this list, please open a PR.

The **Chart** column indicates the Helm chart that uses this image. `CORE` indicates that this is not used by a Helm chart, but by the supervisor itself to host a control-plane component. `CORE FROM` indicates that one of the `CORE` images has a `COPY --from` line in the Dockerfile that loads content from this image.

| Chart | Image | Build Repo | Source Images | Hardened |
| ----- | ----- | ---------- | ------------- | -------- |
| CORE | rancher/hardened-kubernetes | rancher/image-build-kubernetes | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE | rancher/rke2-runtime | rancher/rke2 | ‣ rancher/k3s<br>‣ rancher/hardened-kubernetes<br>‣ rancher/hardened-containerd<br>‣ rancher/hardened-crictl<br>‣ rancher/hardened-runc | TRUE |
| CORE FROM | rancher/hardened-containerd | rancher/image-build-containerd | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE FROM | rancher/hardened-crictl | rancher/image-build-crictl | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE FROM | rancher/hardened-runc | rancher/image-build-runc | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE | rancher/rke2-cloud-provider  | rancher/image-build-rke2-cloud-provider | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE | rancher/hardened-etcd | rancher/image-build-etcd | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| CORE | docker.io/rancher/klipper-helm | k3s-io/klipper-helm | ‣ alpine | FALSE |
|  |  |  |  |  |
| rke2-coredns | rancher/hardened-cluster-autoscaler | rancher/image-build-coredns | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rke2-coredns | rancher/hardened-coredns | rancher/image-build-coredns | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rke2-coredns | rancher/hardened-dns-node-cache | rancher/image-build-dns-nodecache | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base<br>‣ rancher/hardened-kube-proxy | TRUE |
|  |  |  |  |  |
| rke2-metrics-server | rancher/hardened-k8s-metrics-server | rancher/image-build-k8s-metrics-server | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
|  |  |  |  |  |
| rke2-ingress-nginx | rancher/nginx-ingress-controller | rancher/ingress-nginx | ‣ ubi8/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rke2-ingress-nginx | rancher/mirrored-jettech-kube-webhook-certgen | rancher/image-mirror |  | FALSE |
|  |  |  |  |  |
| rke2-canal | rancher/hardened-calico | rancher/image-build-calico | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base<br>‣ rancher/hardened-cni-plugins<br>‣ calico/bpftool<br>‣ calico/bird<br>centos | TRUE |
| rke2-canal FROM | rancher/hardened-cni-plugins | rancher/image-build-cni-plugins | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rke2-canal | rancher/hardened-flannel | rancher/image-build-flannel | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
|  |  |  |  |  |
| rke2-multus | rancher/hardened-multus-cni | rancher/image-build-multus | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rke2-multus | rancher/hardened-cni-plugins | rancher/image-build-cni-plugins | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
|  |  |  |  |  |
| rancher-sriov | rancher/hardened-sriov-cni  | rancher/image-build-sriov-cni | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rancher-sriov | rancher/hardened-ib-sriov-cni | rancher/image-build-ib-sriov-cni | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rancher-sriov | rancher/hardened-sriov-network-config-daemon  | rancher/image-build-sriov-operator | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base<br>goboring/golang | TRUE |
| rancher-sriov | rancher/hardened-sriov-network-device-plugin | rancher/image-build-sriov-network-device-plugin | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rancher-sriov | rancher/hardened-sriov-network-operator | rancher/image-build-sriov-operator | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rancher-sriov | rancher/hardened-sriov-network-resources-injector | rancher/image-build-sriov-network-resources-injector | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
| rancher-sriov | rancher/hardened-sriov-network-webhook | rancher/image-build-sriov-operator | ‣ ubi7/ubi-minimal<br>‣ rancher/hardened-build-base | TRUE |
|  |  |  |  |  |
| rke2-calico | rancher/mirrored-calico-operator | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-ctl | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-kube-controllers | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-typha | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-node | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-pod2daemon-flexvol | rancher/image-mirror |  | FALSE |
| rke2-calico | rancher/mirrored-calico-cni | rancher/image-mirror |  | FALSE |
|  |  |  |  |  |
| rke2-cilium | rancher/mirrored-cilium-cilium | rancher/image-mirror |  | FALSE |
| rke2-cilium | rancher/mirrored-cilium-operator-aws | rancher/image-mirror |  | FALSE |
| rke2-cilium | rancher/mirrored-cilium-operator-azure | rancher/image-mirror |  | FALSE |
| rke2-cilium | rancher/mirrored-cilium-operator-generic | rancher/image-mirror |  | FALSE |
| rke2-cilium | rancher/mirrored-cilium-startup-script | rancher/image-mirror |  | FALSE |
|  |  |  |  |  |
| rancher-vsphere-cpi | rancher/mirrored-cloud-provider-vsphere | rancher/image-mirror |  | FALSE |
| rancher-vsphere-cpi | rancher/mirrored-cloud-provider-vsphere-csi-release-driver | rancher/image-mirror |  | FALSE |
| rancher-vsphere-cpi | rancher/mirrored-cloud-provider-vsphere-csi-release-syncer | rancher/image-mirror |  | FALSE |
|  |  |  |  |  |
| rancher-vsphere-csi | rancher/mirrored-k8scsi-csi-attacher | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | rancher/mirrored-k8scsi-csi-node-driver-registrar | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | rancher/mirrored-k8scsi-csi-provisioner | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | rancher/mirrored-k8scsi-csi-resizer | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | rancher/mirrored-k8scsi-livenessprobe | rancher/image-mirror |  | FALSE |
|  |  |  |  |  |
| harvester-cloud-provider | docker.io/rancher/harvester-cloud-provider | Harvester team |  | FALSE |
|  |  |  |  |  |
| harvester-csi-driver | docker.io/rancher/harvester-csi-driver | Harvester team |  | FALSE |
| harvester-csi-driver | docker.io/rancher/longhornio-csi-attacher | Longhorn team |  | FALSE |
| harvester-csi-driver | docker.io/rancher/longhornio-csi-node-driver-registrar | Longhorn team |  | FALSE |
| harvester-csi-driver | docker.io/rancher/longhornio-csi-provisioner | Longhorn team |  | FALSE |
| harvester-csi-driver | docker.io/rancher/longhornio-csi-resizer | Longhorn team |  | FALSE |
|  |  |  |  |  |
| rke2-upgrade | rancher/rke2-upgrade | rancher/rke2-upgrade | alpine | FALSE |
