# Updating RKE2 charts

In each release, it is very common to update the charts that rke2 is using. This document describes the process that mus
t be followed to succeed in updating and integrating it into rke2 and Rancher.

## What are these rke2-charts?

The following RKE2 components are consumed using Helm Charts:

- coredns
- ingress-nginx
- metrics-server
- kube-proxy (up to v1.21)
- all cni plugins

The charts of these components are in the [rke2-charts repo](https://github.com/rancher/rke2-charts/tree/main-source/packages)
and are installed in the cluster by the [helm-controller](https://github.com/k3s-io/helm-controller), which is one of
the controllers part of `rke2 server` binary.

## How to update a chart?

Before going into updating the chart, note a chart update normally means updating the images that this chart is consuming.
In general, rke2 is consuming hardened images that are built using a FIPS compliant process. In other words, **do not
 use the upstream images**. Instead, refer to the Github project building that image and use the code of the upstream
project to yield a hardened image. The Github projects building hardened images are under our [github rancher](https://github.com/rancher/)
and start with the name `image-build-`, for example: [image-build-coredns](https://github.com/rancher/image-build-coredns).
There are two exceptions to this rule: Calico and Cilium. For these components, we are mirroring the upstream images
into our rancher dockerhub and consuming those. To mirror new images, refer to [the README of the image-mirror repo](https://github.com/rancher/image-mirror/blob/master/README.md).

To apply changes to a chart, please follow the [README](https://github.com/rancher/rke2-charts/blob/main-source/README.md)
of the rke2-charts project. Once the PR has been merged, move to the `main` branch in the rke2-charts project and check
your chart under the directory `assets/`. A new tarball would be there with your changes with a name that follows the
pattern `$chartName-$version.tgz`. For example: `rke2-calico-v3.19.2-201.tgz`.

## My chart is ready, how to integrate it into rke2 and Rancher?

From the previous section, you know the name of the chart (e.g. `rke2-calico`) and its version (e.g. `v3.19.2-201`).
Using that information, we can easily update rke2 and Rancher following the next steps:

1. Open [Dockerfile](https://github.com/rancher/rke2/blob/master/Dockerfile) and go to the stage where the charts are
build (`FROM build AS charts`). There, modify the line where we are referring to the chart, e.g. for calico:
```
RUN CHART_VERSION="v3.19.1-105"               CHART_FILE=/charts/rke2-calico.yaml         CHART_BOOTSTRAP=true   /charts/build-chart.sh
```
If we want the new version to be consumed, we should update the value of `RUN CHART_VERSION`:
```
RUN CHART_VERSION="v3.19.2-201"               CHART_FILE=/charts/rke2-calico.yaml         CHART_BOOTSTRAP=true   /charts/build-chart.sh
```

2. For CNI plugins only, we need to do the same for [Dockerfile.windows](https://github.com/rancher/rke2/blob/master/Dockerfile.windows).
In this case, we are not using helm and we are downloading a released tarball from the upstream project. Therefore, we
must use the version that makes sense for the upstream project, and not the previous version which was pointing to the
rke2-charts tarball. The version in the Dockerfile is tracked by a variable, which for example for Calico is
`CALICO_VERSION`. If we move from v3.19.1 to v3.19.2, we should open the file and change:
```
ENV CALICO_VERSION="v3.19.2"
```

3. If the update of the chart means using different images, we must update the script for air-gap scenarios. Open the
file [scripts/build-images](https://github.com/rancher/rke2/blob/master/scripts/build-images) and update the version of
the images. Images used by CNI plugins have their own section, for example for Calico:
```
xargs -n1 -t docker image pull --quiet << EOF > build/images-calico.txt
    ${REGISTRY}/rancher/mirrored-calico-operator:v1.17.6
    ${REGISTRY}/rancher/mirrored-calico-ctl:v3.19.2
    ${REGISTRY}/rancher/mirrored-calico-kube-controllers:v3.19.2
    ${REGISTRY}/rancher/mirrored-calico-typha:v3.19.2
    ${REGISTRY}/rancher/mirrored-calico-node:v3.19.2
    ${REGISTRY}/rancher/mirrored-calico-pod2daemon-flexvol:v3.19.2
    ${REGISTRY}/rancher/mirrored-calico-cni:v3.19.2
EOF
```
whereas basic RKE2 images like coredns are all grouped together in the same section of the file:
```
xargs -n1 -t docker image pull --quiet << EOF >> build/images-core.txt
    ${REGISTRY}/rancher/hardened-kubernetes:${KUBERNETES_IMAGE_TAG}
    ${REGISTRY}/rancher/hardened-coredns:v1.8.3-build20210720
    ${REGISTRY}/rancher/hardened-cluster-autoscaler:v1.8.3-build20210729
    ${REGISTRY}/rancher/hardened-dns-node-cache:1.20.0-build20210803
    ${REGISTRY}/rancher/hardened-etcd:${ETCD_VERSION}-build20220413
    ${REGISTRY}/rancher/hardened-k8s-metrics-server:v0.5.0-build20210915
    ${REGISTRY}/rancher/klipper-helm:v0.6.1-build20210616
    ${REGISTRY}/rancher/mirrored-pause:${PAUSE_VERSION}
    ${REGISTRY}/rancher/mirrored-jettech-kube-webhook-certgen:v1.5.1
    ${REGISTRY}/rancher/nginx-ingress-controller:nginx-0.47.0-hardened1
    ${REGISTRY}/rancher/rke2-cloud-provider:${CCM_VERSION}
EOF
```
Update the image version so that it points to the correct one

4. Once the previous steps are done and the PR is merged in rke2, we can go ahead and, if needed, update Rancher with
the change. We must only update it in case the options available to the user changed, i.e. when a field in values.yaml
changed. If that is the case, we must create a PR in the `kontainer-driver-metadata` Github repo or [KDM](https://github.com/rancher/kontainer-driver-metadata).
KDM collects information so that the UI/API knows what options to display and validate for chart configuration.
It does not impact what is deployed in the cluster, it only informs the options that rancher exposes. Therefore, it is
crucial that the versions are the same as what rke2 is consuming. Open the file `channels-rke2.yaml` and under the
charts variable, modify the version of the chart. The version is the one we used in step 1, i.e. the version of the
tarball in rke2-charts. For example, for coredns:
```
      rke2-coredns:
        repo: rancher-rke2-charts
        version: 1.10.101-build2021022304
```

Note that there are some charts that separate their crds into a different helm chart. This affects this step because
there will be two charts to update. For example Calico:
```
      rke2-calico:
        repo: rancher-rke2-charts
        version: v3.19.2-201
      rke2-calico-crd:
        repo: rancher-rke2-charts
        version: v1.0.101
```
Once that file is updated, as explained in the [README](https://github.com/rancher/kontainer-driver-metadata/blob/dev-v2.6/README.md#run)
run `go generate` and create a different commit for its changes

5. Make sure all the issues in rke2 and rancher related to this update are updated and possibly moved "To test" so that
 QA can take over
