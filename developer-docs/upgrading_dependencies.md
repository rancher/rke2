# Upgrade Image Process

From time to time we need to update the images that RKE2 depends on. This document serves as a how-to for that process. The following steps are laid out in order.

##  Update Image

Create a new release in the image repository (eg [image-build-etcd](github.com/rancher/image-build-etcd)). This is done by specifying the tag version you want built from whatever upstream this repo is using. An image will be built and pushed to Docker Hub.

## Update RKE2

The following example files have references that will need to be updated in the respective locations for etcd. Replace the found version with the desired version.

* build-images: `${REGISTRY}/rancher/hardened-etcd:${ETCD_VERSION}-build20220413`
* scripts/version.sh:    `ETCD_VERSION=${ETCD_VERSION:-v3.4.13-k3s1}`

Some images may include a build date as part of the tag in format `-buildYYYYmmdd`. Trivy image scans may periodically fail as vulnerabilities are found in the base operating system. Re-tagging an image with the current build date should force an update of the base operating system and may help to resolve vulnerabilities found in image scans.
