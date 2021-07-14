At the heart of RKE2 is the embedded K3s engine which functions as a
supervisor for the kubelet and containerd processes. The K3s engine also
provides an add-on controller that RKE2 leverages. So, RKE2 depends on K3s,
but what does that look like from version to version? It is not yet as simple
as 1.20.7+k3s1rke2r1 &rarr; 1.20.7+k3s1 but starting with 1.22.x it should be.

Until then, here is a handy table:

| RKE2 Release Branch | K3s Release Branch | Comments |
|-------------------------------------------------------------|-----------------------------------------------------------|---|
| [release-1.18](https://github.com/rancher/rke2/tree/release-1.18) | [release-1.19](https://github.com/k3s-io/k3s/tree/release-1.19) | Making k3s an embeddable engine required changes developed after 1.18.x was released. |
| [release-1.19](https://github.com/rancher/rke2/tree/release-1.19) | [release-1.19](https://github.com/k3s-io/k3s/tree/release-1.19) |   |
| [release-1.20](https://github.com/rancher/rke2/tree/release-1.20) | [master](https://github.com/k3s-io/k3s/tree/master) | Rolling commit from k3s master; should be moved to release-1.21 |
| [release-1.21](https://github.com/rancher/rke2/tree/release-1.21) | [master](https://github.com/k3s-io/k3s/tree/master) | Rolling commit from k3s master; should be moved to release-1.21 |
| [master](https://github.com/rancher/rke2/tree/master) | [master](https://github.com/k3s-io/k3s/tree/release-1.21) | Rolling commit from K3s master |
