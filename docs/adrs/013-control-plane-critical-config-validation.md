# Control-Plane Critical Config Validation

Date: 2026-06-01

## Subject
Given HA control-plane requirements, when a new RKE2 server joins an existing cluster, then we validate specific critical configuration before allowing the node to proceed.

## Status
Proposed

## Context

RKE2 requires some server settings to be consistent across all control-plane nodes (for example, CNI, ingress-controller, system-default-registry (or prime)... selection). If these values diverge, cluster behavior can become non-deterministic and difficult to troubleshoot. For example, our support team has already detected this in a couple of users while migrating the ingress controller. If one control-plane sets traefik but another one sets ingress-nginx, the cluster enters a never ending loop of ingresses installing and uninstalling.

K3s already has a join-time validation flow for `CriticalControlArgs` through the existing `/v1-<program>/config` endpoint and compare logic in cluster bootstrap. If that config is not the same, the new control-plane node fails to bootstrap very early with a self-explanatory error.

The key design question is how to validate RKE2-specific critical settings while preserving:
* early failure (before meaningful side effects)
* upgrade compatibility
* maintainability
* K3s stays generic as it is owned by CNCF

## Options

### Option 1: Extend K3s join payload with explicit extension data and compare it at join time (PREFERABLE)

Description:
* Add a generic, opaque field in K3s critical config (kitchen sink style), for example `critical-extra-config`.
* RKE2 serializes its critical settings into that field.
* K3s treats the field as opaque and performs strict equality comparison during join.

Pros:
* Fail-fast at join time using existing bootstrap validation path.
* Keeps K3s generic if the field is opaque and distribution-agnostic.
* Single config transport channel and no extra services.

Cons:
* It requires changes in K3s.
* Requires careful compatibility handling when using control-plane nodes with different versions (e.g. when upgrading).

### Option 2: Validate via Kubernetes resources after startup (ConfigMap/CR-based)

Description:
* Persist first-server critical config in a Kubernetes resource.
* Joining servers read and compare their local config against that resource.

Pros:
* Can be implemented primarily in RKE2.
* Does not require changing K3s config.

Cons:
* Validation happens later (API server and some components may already be starting).
* Potential for partial side effects before failure (e.g. CNI plugin may have already configured the node with networking custom stuff).

### Option 3: Add a second RKE2-specific HTTP config endpoint

Description:
* Keep K3s `/config` as-is.
* Add a new RKE2 endpoint to serve RKE2 critical config.
* Join flow consumes both K3s config (as it does today via embedding) and RKE2 config.

Pros:
* Explicit RKE2 semantics without modifying K3s payload schema.

Cons:
* Additional authn/authz and config server: bigger maintenance burden and troubleshooting complexity

### Option 4: ??? (I could not think of another one)

## Compatibility Notes

* We should test this when upgrading and there is a mix of versions with control-planes. E.g. if one side does not carry the extension field, comparison should be skipped or treated as compatibility mode until all servers are on versions that support the field.
* Serialization must be canonical (stable key order / deterministic content) to avoid false mismatches.

## Open Questions

* What specific parameters should be included?

