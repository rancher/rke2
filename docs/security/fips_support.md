---
title: FIPS 140-2 Enablement
---

FIPS 140-2 is a U.S. Federal Government security standard used to approve cryptographic modules. This document explains how RKE2 is built with FIPS validated cryptographic libraries.

## Use of FIPS Compatible Go compiler.

The Go compiler in use can be found [here](https://go.googlesource.com/go/+/dev.boringcrypto). Each component of the system is built with the version of this compiler that matches the same standard Go compiler version that would be used otherwise.

This version of Go replaces the standard Go crypto libraries with the FIPS validated BoringCrypto module. See the [readme](https://github.com/golang/go/blob/dev.boringcrypto/README.boringcrypto.md) for more details.

Moreover, this module is currently being [revalidated](../assets/fips_engagement.pdf) as the Rancher Kubernetes Cryptographic Library for the additional platforms and systems supported by RKE2.

### FIPS Support in Cluster Components

Most of the components of the RKE2 system are statically compiled with the GoBoring Go compiler implementation. RKE2, from a component perspective, is broken up in a number of sections. The list below contains the sections and associated components.

* Kubernetes
  * API Server
  * Controller Manager
  * Scheduler
  * Kubelet
  * Kube Proxy
  * Metric Server
  * Kubectl

* Helm Charts
  * Flannel
  * Calico
  * CoreDNS

## Runtime

To ensure that all aspects of the system architecture are using FIPS 140-2 compliant algorithm implementations, the RKE2 runtime contains utilities statically compiled with the FIPS enabled Go compiler for FIPS 140-2 compliance. This ensures that all levels of the stack are compliant from Kubernetes daemons to container orchestration mechanics.

* etcd
* containerd
  * containerd-shim
  * containerd-shim-runc-v1
  * containerd-shim-runc-v2
  * ctr
* crictl
* runc

## Ingress

The NGINX Ingress included with RKE2 is **not** currently FIPS enabled. It can, however, be [disabled and replaced](../advanced.md#disabling-server-charts) by the cluster operator/owner.
