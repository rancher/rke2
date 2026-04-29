# Prime tool: rke2-patcher

## Established

2026-03-20

## Revisit by

2026-03-20

## Subject

Currently, when a critical CVE is identified, users must typically wait for the next scheduled RKE2 monthly release for a fix and that can take up to four weeks. This delay leaves clusters vulnerable, forcing users to implement manual workarounds or restrictive security policies. These temporary measures are often labor-intensive, costly, and disruptive to standard operations. We can improve this process by allowing users to consume patched images as soon as they are built and verified, rather than waiting for the next full release cycle.

## Status

Proposed

## rke2-patcher tool

`rke2-patcher` is a binary deployed in one control-plane node of an RKE2 cluster that can address the described challenges.

Goals:

- reduce the CVE exposure window for selected RKE2 components
- keep operations explicit and risk-controlled
- avoid introducing a long-running in-cluster controller
- keep patching compatible with existing RKE2 chart customization workflows (by using `HelmChartConfig`)
- support disconnected or restricted environments by allowing alternate registries and scanner image overrides


### Command surface

- `rke2-patcher --version`
  - prints CLI version
  - attempts to print connected cluster version
  - still succeeds when cluster access is unavailable

```bash
$> ./rke2-patcher --version
rke2-patcher 0.6.0
cluster version: v1.35.2+rke2r1
```

- `rke2-patcher image-cve <component>`
  - resolves the currently running image for the selected component
  - scans CVEs using cluster mode by default (`RKE2_PATCHER_CVE_MODE=cluster`) via a Kubernetes Job
  - supports local mode (`RKE2_PATCHER_CVE_MODE=local`) for host-based scanning

```bash
$> ./rke2-patcher image-cve coredns
scanner mode: cluster
Checking CVEs with in-cluster scanner job. Please wait...
component: coredns
image: rancher/hardened-coredns:v1.14.1-build20260206
scanner mode: cluster
scanner: trivy-job
CVEs (5):
- CVE-2026-25679
- CVE-2026-26017
- CVE-2026-26018
- CVE-2026-33186
- SUSE-SU-2026:0759-1
```

- `rke2-patcher image-list <component> [--with-cves] [--verbose]`
  - lists ordered candidate tags for the running component image,
  - marks in-use tags when cluster data is available,
  - with `--with-cves`, scans current/previous/newer tags and prints CVE summary table,
  - with `--verbose`, prints full vulnerability lists instead of truncated output.

```bash
$> ./rke2-patcher image-list coredns --with-cves
Checking CVEs with in-cluster scanner job for 4 images. Please wait...
COMPONENT:  coredns
REPOSITORY: rancher/hardened-coredns

TAG                      STATUS     CVE COUNT  VULNERABILITIES
v1.14.2-build20260310    NEWER      1          CVE-2026-33186
v1.14.2-build20260309    NEWER      2          CVE-2026-25679, CVE-2026-33186
v1.14.1-build20260206    CURRENT*   5          CVE-2026-25679, CVE-2026-26017...
v1.14.1-build20260203    PREVIOUS   7          CVE-2025-68121, CVE-2026-25679...
```

- `rke2-patcher image-patch <component> [--dry-run] [--revert]`
  - generates and writes a `HelmChartConfig` override into `server/manifests`,
  - with `--dry-run`, prints target output without writing,
  - with `--revert`, moves to the previous tag in the ordered list.

```bash
$> sudo ./rke2-patcher image-patch coredns --dry-run
component: coredns
current image: rancher/hardened-coredns:v1.14.1-build20260206
current tag: v1.14.1-build20260206
new tag: v1.14.2-build20260309
dry-run: true
would write HelmChartConfig: /var/lib/rancher/rke2/server/manifests/coredns-config-rke2-patcher.yaml
---
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
spec:
  valuesContent: |-
    image:
      repository: rancher/hardened-coredns
      tag: v1.14.2-build20260309
```


### Safety and compatibility guardrails

- Refuse forward patching across minor version boundaries. Only patch versions or "hardened" date changes are allowed.
- Refuse reverting when current tag is the baseline version for that RKE2 version.
- Limit forward patching to one patch per component per detected RKE2 cluster version. Extra patches require an RKE2 upgrade
  - State is persisted in `patch-limit-state.json` under:
    - `RKE2_PATCHER_CACHE_DIR` when set, else
    - `<RKE2 data dir>/server/rke2-patcher-cache/patch-limit-state.json`.
- When matching `HelmChartConfig` objects already exist, require explicit operator confirmation before merge and before final write.

### Airgap and alternate registry support

- `RKE2_PATCHER_REGISTRY` controls the registry endpoint used for tag discovery in `image-list` and `image-patch`.
- Supported formats include host-only, host:port, and URL forms (`https://`/`http://`).
- In disconnected environments, operators can point `RKE2_PATCHER_REGISTRY` to an internal mirror registry that contains the relevant Rancher component repositories and tags.
- For cluster CVE mode, scanner image pull source is configurable with `RKE2_PATCHER_CVE_SCANNER_IMAGE`, allowing use of an internally mirrored Trivy image.
- If in-cluster scanner-job image pulls are not possible, operators can use local scanning mode (`RKE2_PATCHER_CVE_MODE=local`) with a local scanner binary.

Example:

```bash
RKE2_PATCHER_REGISTRY=registry.internal.example:5000 \
RKE2_PATCHER_CVE_SCANNER_IMAGE=registry.internal.example:5000/security/trivy:latest \
./rke2-patcher image-patch calico-operator --dry-run
```

### Supported components (current set)

- `traefik`
- `ingress-nginx`
- `coredns`
- `dns-node-cache`
- `calico-operator`
- `cilium-operator`
- `metrics-server`
- `flannel`
- `canal-calico`
- `canal-flannel`
- `csi-snapshotter`
- `coredns-cluster-autoscaler`
- `snapshot-controller`

### Runtime requirements and configuration

- Kubernetes API access is required for `image-cve`, `image-patch`, and cluster-version detection.
- Registry access is required for tag discovery (`RKE2_PATCHER_REGISTRY`, default `registry.rancher.com`).
- Key environment variables:
  - `KUBECONFIG`
  - `RKE2_PATCHER_DATA_DIR`
  - `RKE2_PATCHER_CACHE_DIR`
  - `RKE2_PATCHER_HELM_NAMESPACE`
  - `RKE2_PATCHER_CVE_MODE`
  - `RKE2_PATCHER_CVE_NAMESPACE`
  - `RKE2_PATCHER_CVE_SCANNER_IMAGE`
  - `RKE2_PATCHER_CVE_JOB_TIMEOUT`

## Consequences

- Users can consume validated image fixes before full RKE2 release rollout.
- Risk is bounded by explicit CLI flags (e.g. --dry-run) and patch guardrails.
- Existing chart customization remains the mechanism of record through `HelmChartConfig`.
- Operations depend on Kubernetes API and registry connectivity.
- Airgap deployments can use internal mirrors without changing command surface.

## Open questions

* Are the current guardrails enough to keep risk under control? Or should QA be involved?