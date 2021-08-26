At the heart of RKE2 is the embedded K3s engine which functions as a
supervisor for the kubelet and containerd processes. The K3s engine also
provides AddOn and Helm controllers that RKE2 leverages. So, RKE2 depends on K3s,
but what does that look like from version to version? It is not yet as simple
as 1.20.7+rke2r1 &rarr; 1.20.7+k3s1, but starting with the release-1.22 branch
it should be.

Until then, here is a handy table:

| RKE2 Branch | K3s Branch | Comments |
|-------------------------------------------------------------|-----------------------------------------------------------|---|
| [release-1.18](https://github.com/rancher/rke2/tree/release-1.18) | [release-1.19](https://github.com/k3s-io/k3s/tree/release-1.19) | Making k3s an embeddable engine required changes developed after release-1.18 was branched. |
| [release-1.19](https://github.com/rancher/rke2/tree/release-1.19) | [release-1.19](https://github.com/k3s-io/k3s/tree/release-1.19) | RKE2 development stayed on 1.18 for a long time, essentially jumping from 1.18 to 1.20 with both release-1.18 and release-1.19 forked off master close to each other. |
| [release-1.20](https://github.com/rancher/rke2/tree/release-1.20) | [engine-1.21](https://github.com/k3s-io/k3s/tree/engine-1.21) | The K3s engine-1.21 branch was forked from K3s master just before master was updated to Kubernetes 1.22, and contains critical changes necessary to support RKE2 on Windows. |
| [release-1.21](https://github.com/rancher/rke2/tree/release-1.21) | [engine-1.21](https://github.com/k3s-io/k3s/tree/engine-1.21) | Same K3s upstream as the RKE2 release-1.20 branch. |
| [release-1.22](https://github.com/rancher/rke2/tree/release-1.22) | [engine-1.21](https://github.com/k3s-io/k3s/tree/release-1.22) | We plan to better align the K3s and RKE2 release-1.22 branches, when they are forked. |
| [master](https://github.com/rancher/rke2/tree/master) | [master](https://github.com/k3s-io/k3s/tree/master) | Rolling commit from K3s master |
