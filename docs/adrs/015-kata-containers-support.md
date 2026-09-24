# Kata containers support

Date: 2026-09-17

## Status

Accepted

## Context

[Kata Containers](https://kata-containers.github.io/kata-containers/quick-start-guide/) is an open-source container runtime that builds lightweight virtual machines (VMs) to run container workloads.

This technology is under high demand lately because of the increasing security concerns in the software industry. Additionally, sandboxing workloads is becoming a basic requirement for running AI workloads, as explained in the [CNCF AI Conformance KAR](https://github.com/kubernetes-sigs/ai-conformance/tree/main/kars/0020-workload-sandboxing).

There is also a Rancher initiative to support [Confidential Containers (CoCo)](https://confidentialcontainers.org/). Kata containers is a key part in the CoCo architecture.

Moreover, k3k is supporting Kata containers as a replacement of sysbox: [PR](https://github.com/rancher/k3k/pull/814)

Kata containers include a few components and it support different options for each component (e.g. for hypervisor, it includes QEMU, Dragonball, Firecracker...). Initially at least, we should only scope the essential ones.


## Kata containers integration

The recommended installation method of Kata containers in any Kubernetes distribution is by using the [Kata Deploy Helm chart](https://github.com/kata-containers/kata-containers/blob/main/tools/packaging/kata-deploy/helm-chart/README.md). This chart deploys a DaemonSet which installs all the necessary binaries and sets the correct containerd configuration. It also installs the configured runtimesClasses. Fortunately for us, kata deploy chart supports rke2 specifics and the user only needs to set the value `rke2` in one of their [configuration fields](https://github.com/kata-containers/kata-containers/blob/main/tools/packaging/kata-deploy/helm-chart/kata-deploy/values.yaml#L293).


Given that the chart works well in RKE2 and it is supported by upstream, we should reuse it as much as possible.

We should take a two-phases approach in which we start with documentation, and then, if required, move to deeper integration with Kata.

### (Phase 1) Loosely Integration based on documentation

Describe in our rke2-product-docs documentation how to install Kata containers using kata-deploy and add some sections about prerequisites, how to verify it works and how to debug. In the documentation, we should show the HelmChart with the few options that we would support initially.

Initial PR: https://github.com/rancher/rke2-product-docs/pull/253

#### Prime differentiation

In this phase, the only approach we can take is adding the docs to rke2-product-docs only. Chart and images are consumed from upstream.


### (Phase 2) Integration using rke2-charts

Add kata-deploy to our rke2-charts as rke2-kata-deploy. We can integrate the chart in two different ways:

1 - (preferable) By using an rke2-chart patch to the upstream chart, we should add a switch to the rke2-kata-deploy chart that allows the chart to be dormant and perform no-op in the cluster. This is currently not possible in upstream kata-deploy.

Then, via a HelmChart API, we install Kata in all the RKE2 prime clusters with that switch turned off. If the customer wants to leverage Kata containers, then the customer must turn that switch on and set the correct configuration. The customer can do that using the HelmChartConfig API.

One challenge for this option is to support the use case in which the customer turns it off again. There is currently not a good clean-up mechanism in kata-deploy. We would need to create it.

2 - Include a new flag in RKE2 that would install the kata-deploy helm chart (e.g. --kata)


### Other options

There is a process that automatically searches for runtimes, that RKE2 inherits from K3s. We could install the Kata Container binaries and reuse that proecss to set the containerd configuration and the runtimeClass. Given that we have kata-deploy, this option seems like reinventing the wheel and more work for us.  


### Initial supported options

CPU architecture: amd64
Snapshotter: nydus (default)
shims: qemu-runtime-rs, qemu-runtime-rs-nvidia
hypervisor: qemu

Optionally, and as part of the Rancher CoCo initiative, we should allow these extra shims:
* qemu-nvidia-gpu-snp-runtime-rs (if we have hardware to test)
* qemu-nvidia-gpu-tdx-runtime-rs (if we have hardware to test)

To fully support these options, collaboration with the QA team is expected to develop test cases. It would be highly recommended to add a test case in the CI and also prepare our support friends with a session about Kata containers.


## Decision

Kata containers is a mature project but we don't have a lot of experience with it and its architecture has a few components which might be challenging to support/debug. Therefore, I'd suggest being conservative at the beginning and start with option 1 and consider it as tech preview for a few months. That should also be good enough for the different demands we are seeing.

After a few months (e.g. for 1.38 release), we can reconsider taking option 2 and pursue a more coupled integration.