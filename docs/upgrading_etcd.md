# Upgrade Etcd Process

From time to time we need to update the version of Etcd used by RKE2. This document serves as a how-to for that process. The following steps are laid out in order.

##  Etcd Image

Create a new release in the [image-build-etcd](github.com/rancher/image-build-etcd) repository. This is done by specifying the tag version you want built from whatever upstream this repo is using. An image will be built and pushed to Docker Hub.

## Update RKE2

The following files have references that will need to be updated in the respective locations. Replace the found version with the desired version.

* build-images: `docker.io/rancher/hardened-etcd:v3.4.13-k3s1`
* images.go:    `EtcdVersion       = "v3.4.13-k3s1"`
