![](./assets/logo-horizontal-rke.svg)

RKE2, also known as RKE Government, is Rancher's next-generation Kubernetes distribution.

It is a fully [conformant Kubernetes distribution](https://landscape.cncf.io/selected=rke-government) that focuses on security and compliance within the U.S. Federal Government sector.

To meet these goals, RKE2 does the following:

- Provides [defaults and configuration options](security/hardening_guide.md) that allow clusters to pass the [CIS Kubernetes Benchmark](security/cis_self_assessment.md) with minimal operator intervention
- Enables [FIPS 140-2 compliance](security/fips_support.md)
- Regularly scans components for CVEs using [trivy](https://github.com/aquasecurity/trivy) in our build pipeline

## How is this different from RKE or K3s?

RKE2 combines the best-of-both-worlds from the 1.x version of RKE (hereafter referred to as RKE1) and K3s.

From K3s, it inherits the usability, ease-of-operations, and deployment model.

From RKE1, it inherits close alignment with upstream Kubernetes. In places K3s has diverged from upstream Kubernetes in order to optimize for edge deployments, but RKE1 and RKE2 can stay closely aligned with upstream.

Importantly, RKE2 does not rely on Docker as RKE1 does. RKE1 leveraged Docker for deploying and managing the control plane components as well as the container runtime for Kubernetes. RKE2 launches control plane components as static pods, managed by the kubelet. The embedded container runtime is containerd.

## Why two names?

There are a few reasons why this distribution is known as both RKE2 and RKE Government.
It is known as RKE Government in order to convey the primary use cases and sector it currently targets.

It is known as RKE2 because it is the future of the RKE distribution. Right now, it is entirely independent from RKE1.

## Can I upgrade from RKE1 to RKE2?

We do not currently support upgrades from RKE1 or K3s to RKE2.

Our next phase of development will focus on a seamless upgrade path and feature parity with RKE1, when integrated with the Rancher multi-cluster management platform.

Once we've completed the upgrade path and Rancher-integration feature parity work, RKE1 and RKE Government will converge into a single distribution.

## Security

Rancher Labs supports responsible disclosure and endeavors to resolve security
issues in a reasonable timeframe. To report a security vulnerability, email
[security@rancher.com](mailto:security@rancher.com).
