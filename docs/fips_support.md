# Rancher Kubernetes Enginer 2

## Support for FIPS Cryptography

Starting with Rancher Kubernetes Engine version 2 (RKE2), you can install a cluster that uses FIPS validated libraries. 

## Use of FIPS Compatible Go compiler.

The Go compiler in use can be found [here](https://hub.docker.com/u/goboring). Each component of the system is built with the version of this compiler that matches the same standard Go compiler version that would be used otherwise. 


### FIPS Support in Cluster Components

Most of the components of the RKE2 system are statically compiled with the GoBoring Go compiler implementation that takes advantage of the BoringSSL library. RKE2, from a component perspective, is broken up in a number of sections. The list below contains the sections and associated components.

* etcd

* Kubernetes
  * API Server
  * Controller Manager
  * Scheduler
  * Kubelet
  * KubeProxy
  * MetricsServer
  * Kubectl

* Helm
  * Flannel
  * Calico
  * CoreDNS

## Runtime

To ensure that all aspects of the system architecture are using FIPS 140-2 compliant algorithm implemenations, the RKE2 runtime contains utilities statically compiled with the customized Go compiler for FIPS 140-2 compliance. This ensures that all levels of the stack are compliant from Kuberenetes daemons to container orchestration mechanics.

* containerd
  * containerd-shim
  * containerd-shim-runc-v1
  * containerd-shim-runc-v2
* ctr
* runc
* crictl

## Ingress

Ingress is not included in the RKE2 FIPS 140-2 compliance purview. This is the responsibility of the customers as ingress is ultimately their choice of implemenation.
