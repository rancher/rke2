# Upgrade Kubernetes Process

From time to time we need to update the version of Kubernetes used by RKE2. This document serves as a how-to for that process and constitutes a "release". The following steps are laid out in order.

This process will need to be done any time a new release is needed.

## Hardened Kubernetes

The Hardened Kubernetes build process for RKE2 was once part of the RKE2 build process itself. It's been since split out and exists on its own in the [image-build-kubernetes](https://github.com/rancher/image-build-kubernetes) repository. Follow the steps below to create a new Hardened Kubernetes build.

Create a new release tag at the [image-build-kubernetes](https://github.com/rancher/image-build-kubernetes) repo.

* Click "Releases"
* Click "Draft a new release"
* Enter the new release version (the RKE2 Kubernetes version), appended with `-buildYYYYMMdd`, into the "Tag version" box.  **NOTE** The build system is in UTC.
    When converting the RKE2 version to the Kubernetes version, use dash instead of plus, and do not include any alpha/beta/rc components. For example, if preparing for RKE2 `v1.21.4+rke2r2` before 5 PM Pacific on Friday, August 27th 2021 you would tag `v1.21.4-rke2r2-build20210829`
* Click the "Publish release" button. 

This will take a few minutes for CI to run but upon completion, a new image will be available in [Dockerhub](https://hub.docker.com/r/rancher/hardened-kubernetes).


### Helm Chart

RKE2 depends on it's [Helm Charts](https://github.com/rancher/rke2-charts) being up-to-date with the expected versions for the Kubernetes components. The build process downloads these charts and bundles them into the runtime image.

Create a PR in [rke2-charts](https://github.com/rancher/rke2-charts) that updates the version of the `kube-proxy` image in both the `image.tag` field of `packages/rke2-kube-proxy/charts/values.yaml`, and also in `packages/rke2-kube-proxy/charts/Chart.yaml`. Upon getting 2 approvals and merging, CI will create the needed build artifact that RKE2 will use.

## Update RKE2

The following files have references that will need to be updated in the respective locations. Replace the found version with the desired version. There are also references in documentation that should be updated and kept in sync. 

* Dockerfile: `RUN CHART_VERSION="v1.21.4-build2021041301"     CHART_FILE=/charts/rke2-kube-proxy.yaml`
* Dockerfile: `FROM rancher/k3s:v1.21.4-k3s1 AS k3s`
* version.sh: `KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.21.4}`

Once these changes are made, submit a PR for review and let CI complete. When CI is finished and 2 approvals are had, merge the PR. CI will run for the master merge. 

## RKE2 Release RC

Next, we need to create a release candidate (RC). The Drone (CI) process that builds the release itself can be monitored [here](https://drone-publish.rancher.io/rancher/rke2/).

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.21.4+rke2r2`
    * **NOTE** Make sure to create the tag against the correct release branch. In the example above, that would map to release branch `release-1.21`.

CI will run and build the release assets as well as kick off an image build for [RKE2 Upgrade images](https://hub.docker.com/r/rancher/rke2-upgrade/tags?page=1&ordering=last_updated).

_**Note: Once an RC is released for QA, the release branch associated with the RC is now considered frozen until the final release is complete. If additional PRs need to get merged in after an RC, but before the final release, you should notify the RKE2 team of this immediately. After merging, an additional RC will need to be released for QA.**_

### RKE2 Packaging

Along with creating a new RKE2 release, we need to trigger a new build of the associated RPM. These are found in the [rke2-packaging](https://github.com/rancher/rke2-packaging) repository. We need to create a new release here and the process is nearly identical to the above steps. The Drone (CI) process that builds the release itself can be monitored [here](https://drone-publish.rancher.io/rancher/rke2-packaging/).

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.21.4-rc1+rke2r1.testing.0`
    * The first part of the tag here must match the tag created in the RKE2 repo.

When CI completes, let QA know so they can perform testing.

### Primary Release

Once QA signs off on the RC, it's time to cut the primary release. Go to the [rke2](https://github.com/rancher/rke2) repository.

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.21.4+rke2r1`

Leave the release as "prerelease". This will be unchecked as soon as CI completes successfully.

Once complete, the process is repeated in the [rke2-packaging](https://github.com/rancher/rke2-packaging) repository.

* Click "Releases"
* Click "Draft new release"
* Enter the desired version into the "Tag version" box. 
    * Example tag: `v1.21.4+rke2r1.testing.0`
    * The first part of the tag here must match the tag created in the RKE2 repo.

Make sure that CI passes. This is for RPM availability in the testing channel.

Once complete, perform the steps above again however this time, use the tag "latest" tag. E.g. `v1.21.4+rke2r1.latest.0`.

We choose "latest" here since we want to wait at least 24 hours in case the community finds an issue. Patches will need at least 24 hours. We'll then wait up to 7 days until marking the release as "stable".

### Updating Channel Server

After all of the builds are complete and QA has signed off on the release, we need to update the channel server. This is done by editing the `channels.yaml` file at the root of the [rke2](https://github.com/rancher/rke2) repository.

* Update the line: `latest: <release>` to be the recent release. e.g. `v1.21.4+rke2r1`.
* Verify updated in the JSON output from a call [here](https://update.rke2.io/).

## Update Rancher KDM

This step is specific to Rancher and serves to update Rancher's [Kontainer Driver Metadata](https://github.com/rancher/kontainer-driver-metadata/).

* Create a PR in the latest [KDM](https://github.com/rancher/kontainer-driver-metadata/) dev branch to update the kubernetes versions in channels.yaml. The PR should consist of two commits. The first being the change made to channels.yaml to update the kubernetes versions. The second being go generate. To do this, run `go generate` and commit the changes this caused to data/data.json. Title this second commit "go generate".
    * Please note if this is a new minor release of kubernetes, then a new entry will need to be created in channels.yaml. Ensure to set the min/max versions accordingly. If you are not certain what they should be, reach out to the team for input on this as it will depend on what Rancher will be supporting.
* Create a backport PR in any additional dev branches as necessary.
* The PRs should be merged in a timely manner, within about a day; however, they do not need to be merged before RC releases and they typically do not need to block the final release.

### Promoting to Stable

After 24 hours, we'll promote the release to stable by updating the channel server's config as we did at above, however this time changing "latest" to "stable". We need to do the same thing for RPM's too. This involves the same steps for RPM releases but changing "latest" to "stable" in the release name. E.g. `v1.21.4+rke2r1.stable.0`.
