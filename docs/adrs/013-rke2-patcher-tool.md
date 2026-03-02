# Prime tool: rke2-patcher

## Established

2026-03-20

## Revisit by

2026-06-11

## Subject

Currently, when a critical CVE is identified, users must typically wait for the next scheduled RKE2 monthly release for a fix and that can take up to four weeks. This delay leaves clusters vulnerable, forcing users to implement manual workarounds or restrictive security policies. These temporary measures are often labor-intensive, costly, and disruptive to standard operations. We can improve this process by allowing users to consume patched images as soon as they are built and verified, rather than waiting for the next full release cycle.

## Status

Proposed

## rke2-patcher tool

`rke2-patcher` is a CLI that inspects and patches selected RKE2 component images.

Scope and compatibility:

- requires RKE2 Prime clusters with `prime: true`
- supports two execution modes:
  - host binary mode on a control-plane node (or equivalent environment with Kubernetes API access)
  - in-cluster mode (as a pod in the cluster)

Goals:

- reduce the CVE exposure window for selected RKE2 components
- keep operations explicit and risk-controlled
- avoid introducing a long-running in-cluster controller
- keep patching compatible with existing RKE2 chart customization workflows (by using `HelmChartConfig`)
- support disconnected or restricted environments by allowing alternate registries and scanner image overrides


### Command surface

- `rke2-patcher --version`
  - always prints CLI version
  - attempts to print connected cluster version from Kubernetes API `/version`

- `rke2-patcher --config`
  - prints effective/default/source values for runtime configuration
  - includes registry, scanner mode/image/namespace/timeout, and state ConfigMap coordinates

- `rke2-patcher image-cve <component>`
  - resolves the current running image for the selected component
  - default mode is cluster scan (`RKE2_PATCHER_CVE_MODE=cluster`) using an in-cluster Trivy Job
  - supports local mode (`RKE2_PATCHER_CVE_MODE=local`) using local scanners (`trivy`, `grype` fallback)
  - local mode uses a shared VEX cache at `$HOME/rke2-patcher-cache/vex/rancher.openvex.json`

- `rke2-patcher image-list <component> [--with-cves] [--verbose]`
  - lists ordered release tags for the running component repository
  - marks tags currently in use by running pods
  - applies the same 45-day eligibility policy as `image-patch`
  - output is split into:
    - `eligible tags` (patchable on current cluster)
    - `newer tags requiring RKE2 upgrade` (visible, but blocked)
  - `--with-cves` scans only eligible tags (including current/previous) and prints CVE table
  - `--verbose` expands vulnerability details in the CVE table

- `rke2-patcher image-patch <component> [--dry-run] [--yes|-y]`
  - resolves running image and selects the next eligible newer tag
  - generates/applies `HelmChartConfig` patch to the cluster
  - `--dry-run` prints generated content without writing
  - `--yes` auto-approves merge/apply prompts in non-interactive flows

- `rke2-patcher image-reconcile <component> [--yes|-y]`
  - reconciles stale patch entries after RKE2 upgrade
  - can also revert current same-version patch for the component (with confirmation)
  - removes only patcher-managed image override keys; does not delete the `HelmChartConfig`


### Safety and compatibility guardrails

- Prime guardrail: patching is allowed only when cluster charts indicate `global.prime.enabled=true`.
- Minor-version guardrail: refuse forward patching to newer minor lines.
- Time-window guardrail: refuse patch target tags outside a 45-day window from cluster zero-day.
  - goal is to avoid users staying in the same RKE2 version forever
  - zero-day is derived from running kube-apiserver image (`rancher/hardened-kubernetes`) build date
  - `rke2-ingress-nginx` is exempt from this date-window rule
- Reconcile gate: if stale patch entries from a previous RKE2 version exist, forward patching is blocked until reconcile is performed.
- Merge safety: when matching `HelmChartConfig` objects already exist, operator confirmation is required before merge and write (unless `--yes`).
- State backend: persisted in Kubernetes ConfigMap:
  - name: `rke2-patcher-state`
  - key: `patch-limit-state.json`
  - namespace: `RKE2_PATCHER_CVE_NAMESPACE` (default `rke2-patcher`)

### Airgap and alternate registry support

- `RKE2_PATCHER_REGISTRY` controls the registry endpoint used for tag discovery in `image-list` and `image-patch`.
- Supported formats include host-only, host:port, and URL forms (`https://`/`http://`).
- In disconnected environments, operators can point `RKE2_PATCHER_REGISTRY` to an internal mirror registry that contains the relevant Rancher component repositories and tags.
- For cluster CVE mode, scanner image pull source is configurable with `RKE2_PATCHER_CVE_SCANNER_IMAGE`, allowing use of an internally mirrored Trivy image.

Example:

```bash
RKE2_PATCHER_REGISTRY=registry.internal.example:5000 \
RKE2_PATCHER_CVE_SCANNER_IMAGE=registry.internal.example:5000/security/trivy:latest \
./rke2-patcher image-patch rke2-canal-calico --dry-run
```

- If in-cluster scanner-job image pulls are not possible, operators can use local scanning mode (`RKE2_PATCHER_CVE_MODE=local`) with a local scanner binary. This mode is also faster

Example:

```bash
$> $RKE2_PATCHER_CVE_MODE=local ./rke2-patcher image-cve rke2-coredns
component: rke2-coredns
image: registry.rancher.com/rancher/hardened-coredns:v1.14.3-build20260511
scanner: trivy
CVEs (2):
- CVE-2026-42504
- SUSE-SU-2026:2231-1
```

### Supported components (current set)

- `rke2-traefik`
- `rke2-ingress-nginx`
- `rke2-coredns`
- `rke2-dns-node-cache`
- `rke2-metrics-server`
- `rke2-flannel`
- `rke2-canal-calico`
- `rke2-canal-flannel`
- `rke2-coredns-cluster-autoscaler`
- `rke2-snapshot-controller`