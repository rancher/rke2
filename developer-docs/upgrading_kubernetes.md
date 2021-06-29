# Upgrade Kubernetes Process

From time to time we need to update the version of Kubernetes used by RKE2. This document serves as a how-to for that process. The following steps are laid out in order.

##  Kube-proxy

### Container Image

Create a new release tag at the [image-build-kube-proxy](https://github.com/rancher/image-build-kube-proxy) repo.

* Click "Releases"
* Click "Draft a new release"
* Enter the new release version (the new k8s version) into the "Tag version" box
* Click the "Publish release" button. 

This will take a few minutes for CI to run but upon completion, a new image will be available in [Dockerhub](https://hub.docker.com/r/rancher/hardened-kubernetes).

### Helm Chart

RKE2 depends on it's [Helm Charts](https://github.com/rancher/rke2-charts) being up-to-date with the expected versions for the Kubernetes components. The build process downloads these charts and bundles them into the runtime image.

Create a PR in [rke2-charts](https://github.com/rancher/rke2-charts) that updates the version of the `kube-proxy` image in both the `image.tag` field of `packages/rke2-kube-proxy/charts/values.yaml`, and also in `packages/rke2-kube-proxy/charts/Chart.yaml`. Upon getting 2 approvals and merging, CI will create the needed build artifact that RKE2 will use.

## Update RKE2

The following files have references that will need to be updated in the respective locations. Replace the found version with the desired version. There are also references in documentation that should be updated and kept in sync. 

* Dockerfile: `RUN CHART_VERSION="v1.19.8"     CHART_FILE=/charts/rke2-kube-proxy.yaml`
* Dockerfile: `FROM rancher/k3s:v1.19.8-k3s1 AS k3s`
* images.go:  `KubernetesVersion = "v1.19.8"`
* version.sh: `KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.19.8}`

Once these changes are made, submit a PR for review and let CI complete. When CI is finished and 2 approvals are had, merge the PR. CI will run for the master merge. 

## RKE2 Release RC

Next, we need to create a release candidate (RC). 

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.19.8-rc1+rke2r1`

CI will run and build the release assets as well as kick off an image build for [RKE2 Upgrade images](https://hub.docker.com/r/rancher/rke2-upgrade/tags?page=1&ordering=last_updated).

### RKE2 Packaging

Along with creating a new RKE2 release, we need to trigger a new build of the associated RPM. These are found in the [rke2-packaging](https://github.com/rancher/rke2-packaging) repository. We need to create a new release here and the process is nearly identical to the above steps.

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.19.8-rc1+rke2r1.testing.0`
    * The first part of the tag here must match the tag created in the RKE2 repo.

When CI completes, let QA know so they can perform testing.

### Primary Release

Once QA signs off on the RC, it's time to cut the primary release. Go to the [rke2](https://github.com/rancher/rke2) repository.

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.19.8+rke2r1`

Leave the release as "prerelease". This will be unchecked as soon as CI completes successfully.

Once complete, the process is repeated in the [rke2-packaging](https://github.com/rancher/rke2-packaging) repository.

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.19.8+rke2r1.testing.0`
    * The first part of the tag here must match the tag created in the RKE2 repo.

Make sure that CI passes. This is for RPM availability in the testing channel.

Once complete, perform the steps above again however this time, use the tag "latest" tag. E.g. `v1.19.8+rke2r1.latest.0`.

We choose "latest" here since we want to wait at least 24 hours in case the community finds an issue. Patches will need at least 24 hours. We'll then wait up to 7 days until marking the release as "stable".

### Updating Channel Server

After all of the builds are complete and QA has signed off on the release, we need to update the channel server. This is done by editing the `channels.yaml` file at the root of the [rke2](https://github.com/rancher/rke2) repository.

* Update the line: `latest: <release>` to be the recent release. e.g. `v1.19.8+rke2r1`.
* Verify updated in the JSON output from a call [here](https://update.rke2.io/).

### Promoting to Stable

After 24 hours, we'll promote the release to stable by updating the channel server's config as we did at above, however this time changing "latest" to "stable". We need to do the same thing for RPM's too. This involves the same steps for RPM releases but changing "latest" to "stable" in the release name. E.g. `v1.19.8+rke2r1.stable.0`.
