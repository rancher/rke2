# Standalone Gateway API CRD Management

## Established
2026-04-06

## Subject
Given the need for cross-controller Gateway API support, when deploying RKE2, then we bundle a standalone rke2-gateway-api-crd Helm chart to manage CRD lifecycles independently of the ingress controller.

## Status
Approved

## Context

While ingress API is not going away any time soon, there is an evolution called Gateway API. The Kubernetes upstream community is recommending moving to Gateway API and the ingress-nginx project termination is going to accelerate that transition.

Gateway API groups CRDs in two buckets: stable and experimental. It typically releases a new minor version twice a year, which often moves experimental CRDs into stable. The velocity in which components using Gateway API consume the newest release can vary a lot, from a few weeks to a few months. 

Unfortunately, Gateway API is not bundled in the core Kubernetes API and it will remain like that until its stable API becomes more "stable". As a result, the components that use Gateway API either require the user to install the Gateway API CRDs manually (e.g. Cilium) or include them in their helm charts (e.g. Traefik). The latter generates a "dependency hell" that arises when infrastructure components try to be too helpful by bundling global resources.

RKE2 bundles Traefik (default in v1.36) and separates the [chart](https://github.com/rancher/rke2-charts/tree/main/charts/rke2-traefik/rke2-traefik) from the [CRDs](https://github.com/rancher/rke2-charts/tree/main/charts/rke2-traefik/rke2-traefik-crd). As a consequence, the Gateway API CRDs are always installed, even if the user does not want to leverage Gateway API. Moreover, the lifecycle of the Gateway API CRDs is always bound to Traefik's lifecycle.

Problematic use cases:
1 - Users that want Traefik for ingress api but prefer another ingress controller for Gateway API
2 - Users that start their Gateway API transition with Traefik and want to switch afterwards to another ingress controller
3 - Users that would like to, additionally to Traefik, use another component which can consume Gateway API (e.g. Istio)

## Proposal

Create a new rke2 helm chart called rke2-gateway-api-crd to manage the Gateway API CRDs lifecycle independently of Traefik (or any other). Requirements for this chart:
* Always up to date with the latest Gateway API version. Note that Gateway API versions are [backwards compatible](https://gateway-api.sigs.k8s.io/guides/implementers/#changes-to-the-standard-channel-crds-are-backwards-compatible)
* Stable CRDs are always installed by this chart.
* Experimental CRDs are optional and gated behind chart value `gatewayAPIExperimental=true`.
* The chart is installed with `takeOwnership` so it can adopt existing Gateway API CRDs during migration.
* It can be easily uninstalled using the known RKE2 configuration mechanisms (`disable:`) for advanced users that require the experimental CRDs or are using a very old RKE2 version that bundles an old Gateway API chart
* We include the upstream Gateway API `safe-upgrades` `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding` as chart templates.


## Pros
*Controller agnostic:* Allows components to coexist using the same stable API definitions (e.g. Cilium, Traefik and Istio )
*Always latest version alignment:* RKE2 can guarantee the latest stable Gateway API version in every monthly patch.
*Versioning safety:* Enables a mechanism to prevent accidental CRD downgrades by 3rd party charts.
*Flexibility:* Advanced users can disable the RKE2-managed API via config.yaml to install the "Experimental" CRDs manually.

## Cons
*Increased maintenance:* Adds one additional internal Helm chart that we must maintained. Although the burden is basically being always up to date ==> easily automated
*Migration complexity:* Requires a careful one-time handover for existing clusters where Traefik currently "owns" the CRDs.
*Policy/runtime dependency:* The safe-upgrades mechanism depends on Kubernetes admission policy support.

## Migration and safety strategy

To make the transition safe for existing clusters, we use a three-step rollout across minor versions:
1 - Update the `rke2-traefik-crd` chart to add `helm.sh/resource-policy: keep` to all Gateway API CRDs.
2 - Include this annotation in all RKE2 `v1.36` patch releases.
3 - Perform the ownership switch in RKE2 `v1.37` by installing `rke2-gateway-api-crd` with `takeOwnership` and removing Gateway API CRDs from `rke2-traefik-crd`.

Why this is safe:
* RKE2 upgrades do not allow skipping minor versions.
* Any cluster upgrading to RKE2 `v1.37` must come from `v1.36`, where the `keep` annotation is already present.
* This prevents CRD deletion during chart ownership changes and avoids Helm conflicts during the handover.


## Other options
Remove the Gateway API CRDs from the traefik-crd chart and do not bundle Gateway API at all until it is part of Kubernetes core


## Open Issues for Investigation

None
