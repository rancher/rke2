---
title: FIPS 140-2 Enablement
---

FIPS 140-2 is a U.S. Federal Government security standard used to approve cryptographic modules. This document explains how RKE2 is built with FIPS validated cryptographic libraries.

## Use of FIPS Compatible Go compiler.

The Go compiler in use can be found [here](https://go.googlesource.com/go/+/dev.boringcrypto). Each component of the system is built with the version of this compiler that matches the same standard Go compiler version that would be used otherwise.

This version of Go replaces the standard Go crypto libraries with the FIPS validated BoringCrypto module. See GoBoring's [readme](https://github.com/golang/go/blob/dev.boringcrypto/README.boringcrypto.md) for more details. This module has been revalidated as the [Rancher Kubernetes Cryptographic Library](https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/3836) in order to ensure support on a wider range of systems.

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

## CNI

As of v1.21.2, RKE2 supports selecting a different CNI via the `--cni` flag and comes bundled with several CNIs including Canal (default), Calico, Cilium, and Multus. Of these, only Canal (the default) is rebuilt for FIPS compliance.

## Ingress

RKE2 ships with NGNIX as its default ingress provider. As of v1.21+, this component is FIPS compliant. There are two primary sub-components for NGINX ingress:

- controller - responsible for monitoring/updating Kubernetes resources and configuring the server accordingly
- server - responsible for accepting and routing traffic

The controller is written in Go and as such is compiled using our [FIPS compatible Go compiler](./fips_support.md#use-of-fips-compatible-go-compiler).

The server is written in C and also requires OpenSSL to function properly. As such, it leverages a FIPS-validated version of OpenSSL to achieve FIPS compliance.
