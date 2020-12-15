# Upgrade Kubernetes Process

From time to time we need to update the version of Kubernetes used by RKE2. This document serves as a how-to for that process. The following steps are laid out in order.

##  Kube-proxy

### Container Image

Create a new release tag at https://github.com/rancher/image-build-kube-proxy

### Helm Chart

Create a new release asset for in the [rke2-charts](github.com/rancher/rke2-charts) repository. Instructions for doing so can be found in the repo. This is necessary as the RKE2 build process will check for that chart and source it into one of its build artifacts.

## Update RKE2

The following files have references that will need to be updated in the respective locations. Replace the found version with the desired version.

* Dockerfile: `RUN CHART_VERSION="v1.19.5"     CHART_FILE=/charts/rke2-kube-proxy.yaml`
* Dockerfile: `FROM rancher/k3s:v1.19.5-k3s1 AS k3s`
* images.go:  `KubernetesVersion = "v1.19.5"`
* version.sh: `KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.19.5}`
