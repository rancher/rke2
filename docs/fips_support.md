# Rancher Kubernetes Enginer 2

## Support for FIPS Cryptography

Starting with Rancher Kubernetes Enginer version 2 (RKE2), you can install a cluster that uses FIPS validated libraries. 

## Use of FIPS Compatible Go compiler.

The Go compiler in use can be found [here](https://hub.docker.com/u/goboring). Each component of the system is built with the version of this compiler that matches the same standard Go compiler version that would be used otherwise. 

This compiler utilizes the BoringSSL FIPS 140-2 compliant cryptographic library to perform crypto functions by creating a shim between the standard Go crypto API and allowing compatible passthrough to BoringSSL. This approach requires [CGO](https://golang.org/cmd/cgo/) so to stick with the normal Go use, we make sure all binaries are statically compiled. 

### FIPS Support in Cluster Components

Most of the components of the RKE2 system are statically compiled with the GoBoring Go compiler implementation that takes advantage of the BoringSSL library. RKE2, from a component perspective, is broken up in a number of sections. The list below contains the sections and associated components.

* etcd
* Kubernetes: API Server, Controller Manager, Scheduler, Kubelet, KubeProxy, MetricsServer, Kubectl 
* Helm: Flannel, Calico, CoreDNS
* Runtime: containerd, containerd-shim, containerd-shim-runc-v1, containerd-shim-runc-v2, ctr, runc
