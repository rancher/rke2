# Charts

This document lists all of the charts used by RKE2.
The goal is to list the charts, their source, and the images they use.
If you find a chart that is not listed here, please open a PR to add it.
If you find an image from a chart that is not listed here, please open a PR to add it.

- The **Chart** column indicates the Helm chart that uses this image.
- The **Chart Repo** column indicates the repository that builds the chart.
- The **Upstream Chart** column indicates the upstream chart that the chart is based on.
- The **Image** column indicated the image used by the chart.
- The **Image Repo** column indicates the repository that builds the chart.
- The **Base** column indicates the image that the image is based on.
- The **Hardened** column indicates if the image is considered hardened by Rancher's standards.


| Chart | Chart Repo | Upstream Chart | Image | Image Repo | Base | Hardened |
| ----- | ---------- | -------------- | ----- | ---------- | ---- | -------- |
| rke2-coredns | | | rancher/hardened-cluster-autoscaler | rancher/image-build-coredns | bci | TRUE |
| rke2-coredns | | | rancher/hardened-coredns | rancher/image-build-coredns | bci | TRUE |
| rke2-coredns | | | rancher/hardened-dns-node-cache | rancher/image-build-dns-nodecache | bci | TRUE |
| rke2-metrics-server | | | rancher/hardened-k8s-metrics-server | rancher/image-build-k8s-metrics-server | bci | TRUE |
| rke2-ingress-nginx | | | rancher/nginx-ingress-controller | rancher/ingress-nginx | bci | TRUE |
| rke2-ingress-nginx | | | rancher/mirrored-jettech-kube-webhook-certgen | rancher/image-mirror |  | FALSE |
| rke2-canal | | | rancher/hardened-calico | rancher/image-build-calico | bci | TRUE |
| rke2-canal | | | rancher/hardened-cni-plugins | rancher/image-build-cni-plugins | bci | TRUE |
| rke2-canal | | | rancher/hardened-flannel | rancher/image-build-flannel | bci | TRUE |
| rke2-multus | | | rancher/hardened-multus-cni | rancher/image-build-multus | bci | TRUE |
| rke2-multus | | | rancher/hardened-cni-plugins | rancher/image-build-cni-plugins | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-cni  | rancher/image-build-sriov-cni | bci | TRUE |
| rancher-sriov | | | rancher/hardened-ib-sriov-cni | rancher/image-build-ib-sriov-cni | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-network-config-daemon  | rancher/image-build-sriov-operator | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-network-device-plugin | rancher/image-build-sriov-network-device-plugin | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-network-operator | rancher/image-build-sriov-operator | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-network-resources-injector | rancher/image-build-sriov-network-resources-injector | bci | TRUE |
| rancher-sriov | | | rancher/hardened-sriov-network-webhook | rancher/image-build-sriov-operator | bci | TRUE |
| rke2-calico | | | rancher/mirrored-calico-operator | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-ctl | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-kube-controllers | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-typha | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-node | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-pod2daemon-flexvol | rancher/image-mirror |  | FALSE |
| rke2-calico | | | rancher/mirrored-calico-cni | rancher/image-mirror |  | FALSE |
| rke2-cilium | | | rancher/mirrored-cilium-cilium | rancher/image-mirror |  | FALSE |
| rke2-cilium | | | rancher/mirrored-cilium-operator-aws | rancher/image-mirror |  | FALSE |
| rke2-cilium | | | rancher/mirrored-cilium-operator-azure | rancher/image-mirror |  | FALSE |
| rke2-cilium | | | rancher/mirrored-cilium-operator-generic | rancher/image-mirror |  | FALSE |
| rke2-cilium | | | rancher/mirrored-cilium-startup-script | rancher/image-mirror |  | FALSE |
| rancher-vsphere-cpi | | | rancher/mirrored-cloud-provider-vsphere-cpi-release-manager | rancher/image-mirror |  | FALSE |
| rancher-vsphere-cpi | | | rancher/mirrored-cloud-provider-vsphere-csi-release-driver | rancher/image-mirror |  | FALSE |
| rancher-vsphere-cpi | | | rancher/mirrored-cloud-provider-vsphere-csi-release-syncer | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | | | rancher/mirrored-k8scsi-csi-attacher | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | | | rancher/mirrored-k8scsi-csi-node-driver-registrar | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | | | rancher/mirrored-k8scsi-csi-provisioner | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | | | rancher/mirrored-k8scsi-csi-resizer | rancher/image-mirror |  | FALSE |
| rancher-vsphere-csi | | | rancher/mirrored-k8scsi-livenessprobe | rancher/image-mirror |  | FALSE |
| harvester-cloud-provider | | | docker.io/rancher/harvester-cloud-provider | Harvester team |  | FALSE |
| harvester-csi-driver | | | docker.io/rancher/harvester-csi-driver | Harvester team |  | FALSE |
| harvester-csi-driver | | | docker.io/rancher/longhornio-csi-attacher | Longhorn team |  | FALSE |
| harvester-csi-driver | | | docker.io/rancher/longhornio-csi-node-driver-registrar | Longhorn team |  | FALSE |
| harvester-csi-driver | | | docker.io/rancher/longhornio-csi-provisioner | Longhorn team |  | FALSE |
| harvester-csi-driver | | | docker.io/rancher/longhornio-csi-resizer | Longhorn team |  | FALSE |
| rke2-upgrade | | | rancher/rke2-upgrade | rancher/rke2-upgrade | alpine | FALSE |
