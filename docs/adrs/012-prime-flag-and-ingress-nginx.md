# Introduction of "prime" configuration flag

Date: 2026-04-21

## Status

Proposed

## Context

On the 12th of November, a [blog post](https://www.kubernetes.dev/blog/2025/11/12/ingress-nginx-retirement/) announced that ingress-nginx will be discontinued after March 2026. This happened during Kubecon EU and ingress-nginx will not publish any extra release.

SUSE announced that RKE2 will still patch ingress-nginx for 8+ CVEs and provide support for its customers during an extended period for v1.32, v1.33, v1.34, v1.35 and v1.36 minor releases, so that they have plenty of time to migrate to Traefik. This includes the v1.35 LTS, whose support ends in December 2027.

Community users will get the latest ingress-nginx image published upstream but it will never be updated.

We expect both customers and community users to migrate to Traefik during the v1.36 lifecycle. Customers may get an extended opportunity in v1.37 but this is not guaranteed at the moment. 

## Problem

RKE2 currently lacks a mechanism to differentiate between community and customer distributions at the configuration level. There is no automated way to toggle between upstream images and SUSE-maintained images within the internal Helm charts.

## Proposal

We will introduce a new boolean configuration flag, prime, within the RKE2 configuration. The default value will be false. While the current focus is on ingress-nginx extended life for customers, the flag establishes a pattern for future functional deviations between community and customer distributions. As of today the following changes would occur when the flag is enabled (prime: true): 

1 - Registry Redirection: The global.systemDefaultRegistry is automatically pointed to the official SUSE registry for customers
2 - Image Tag Logic: The ingress-nginx Helm chart will switch from the standard hardened tag to the prime tag.
3 - Ingress-nginx support in RKE2 v1.37: Ingress-nginx helm chart will remain present

### ingress-nginx helm chart changes

The Github Project where ingress-nginx is built for RKE2 will have two types of tags:
* v1.14.5-hardenedN => for community, it will never be updated *
* v1.14.5-primeN => for customers. N is a natural number and it will be incremented when new patches are added.

The ingress-nginx helm chart includes two fields for the image tag:
* [tag](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-ingress-nginx/rke2-ingress-nginx/4.14.504/values.yaml#L859) is for community and hence will always be v1.14.5-hardenedN
* [primeTag](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-ingress-nginx/rke2-ingress-nginx/4.14.504/values.yaml#L860) is for customers and hence will be v1.14.5-primeN

When enabling the "prime" flag, primeTag will be set following this [logic](https://github.com/rancher/rke2-charts/blob/main/charts/rke2-ingress-nginx/rke2-ingress-nginx/4.14.504/templates/_helpers.tpl#L273-L275)


### Default ingress controller

Default ingress controller will be Traefik regardless of the "prime" flag value.

Clusters upgrading to RKE2 v1.36 minor will retain their existing ingress controller, even if one was not explicitly selected. This ensures community users do not experience an unexpected swap from ingress-nginx to traefik during the minor version upgrade.

### Deprecation schedule for ingress-nginx

v1.36: Deprecation announcement, default ingress controller changed to traefik

v1.37: The ingress-nginx helm chart will be removed for community users; customers will have very limited support

v1.38: The ingress-nginx helm chart will be fully removed for all users, including customers.


### ingress-nginx and traefik in airgap 

Starting with May releases, the image tarball rke2-images-core.linux-amd64 will include both ingress-nginx and traefik images. In Github, the image tarball will include ingress-nginx with the hardened tag. In `prime.ribs`, the image tarball will include ingress-nginx with the prime tag. Ingress-nginx images will be part of both tarballs until 1.36 is EOL (April 2027 for community and October 2027 for Prime customers)


## Use cases

### RKE2 Standalone

Community users can easily become Prime customers and start benefiting from the advantages of it, like getting an extended support period for ingress-nginx images.

Current customers should be already setting the suse image registry in their configuration. They'd need to swap that configuration with "prime: true" and restart the rke2-server in the control-plane nodes. That will trigger a reinstallation of the system charts via a "helm upgrade job" but as there will be no change, nothing will happen.

### RKE2 Downstream (Rancher Manager)

[to be discussed with RM engineers]

Starting with 2.15, when Rancher Prime customers deploy RKE2, the "prime: true" configuration will be set automatically. 

For Rancher Community users, the "prime" flag would not be set, hence the default value will apply (false)
