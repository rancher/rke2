# Calico CRD migration

## Established

## Revisit by

## Subject

This migration strategy specifically addresses how Helm, CRD API versions, and RKE2's packaged HelmChart CRD controller interact during a major Calico upgrade without risking CRD deletion or custom resource loss.

Here is a structured, technical breakdown of your proposed migration architecture, complete with flow lifecycle, risks, and recommended safety mechanisms.
### Architectural Strategy Overview

To safely migrate Calico CRDs from crd.projectcalico.org/v1 to crd.projectcalico.org/v3 on RKE2:

#### Decouple and isolate Helm releases: Use three separate Helm chart packages:

- rke2-calico-crd-v1: legacy chart containing v1 CRD definitions.

- rke2-calico-crd-v1v3: dual-version chart containing both v1 and v3 CRD definitions (enabling dual-version serving during conversion).

- rke2-calico-crd-v3: target chart containing v3 CRD definitions.

#### Dynamic Startup Selection:
RKE2's Helm controller / startup routine inspects existing cluster state before applying chart manifests. It locks onto whichever CRD version (v1 or v3) is active to avoid accidental downgrades or CRD re-creation.

#### Dedicated Migration Job (Pod): A transient migration controller pod handles the multi-step upgrade sequence:

- Upgrades the CRD release to the v1v3 dual-version chart.

- Triggers Calico’s native DatastoreMigration workflow.

- Upgrades Calico components and finalized CRDs to v3.

### Migration Execution Flow

```
[ Phase 1: Pre-Migration State (RKE2 normal startup)]
RKE2 Node Startup ---> Detects v1 CRD installed ---> Deploys rke2-calico-crd-v1

[ Phase 2: Migration Trigger (Manually running a pod on the cluster)]
Migration Pod Started
   |---> 1. Helm upgrade CRD chart to rke2-calico-crd-v1v3 (Dual-version serving)
   |---> 2. Apply Calico DatastoreMigration CRD / Trigger Calico conversion controller
   |---> 3. Verify data conversion (crd.projectcalico.org/v1 -> v3)
   |---> 4. Helm upgrade CRD chart to rke2-calico-crd-v3
   |---> 5. Mark cluster migration complete in ConfigMap / annotation

[ Phase 3: Post-Migration / RKE2 Upgrade (Done on an already migrated cluster)]
RKE2 Node Restart / Upgrade ---> Detects v3 CRD installed ---> Deploys rke2-calico-crd-v3
```

### Detailed Component Breakdown
#### RKE2 Startup & Detection Logic

To prevent RKE2's internal Helm controller (helm-controller.k3s.cattle.io) from blindly applying its packaged rke2-calico-crd chart over an active cluster:

- Detection Query: Check API server for crd.projectcalico.org version or specific annotations on the CRD objects.
- Branching Rule:
  -  If v1 only exists → Render/Apply rke2-calico-crd-v1 manifest.

  -  If v3 present (or v1 and v3 both present) → Render/Apply rke2-calico-crd-v3 manifest.

#### Dual-Version Chart (v1v3) Necessity

Kubernetes requires that before an API version can be dropped from a CRD:

-  Both v1 and v3 must be defined in spec.versions.

-  v3 must be set as served: true and storage: true.

-  v1 must remain served: true while conversion completes.

Packaging the v1v3 chart guarantees that existing v1 resources remain accessible to the API server while Calico's DatastoreMigration job reads v1 objects and rewrites them into v3.
#### Migration Pod Workflow

The migration runner pod executes as a Kubernetes Job or DaemonSet with cluster-admin privileges:

- Helm Upgrade to Dual-Version:
  Runs helm upgrade targeting rke2-calico-crd-v1v3 with --force or standard upgrade flags so the API server accepts both API schemas.

- Execute Datastore Migration:
  Applies the DatastoreMigration resource:
```YAML
apiVersion: crd.projectcalico.org/v1
kind: DatastoreMigration
metadata:
  name: calico-migration
spec:
  datastoreType: kubernetes
```

- Monitor Migration Status:
Polls DatastoreMigration.status.phase until state reaches Completed.

- Finalize CRD & Calico Core Upgrade:

  - Upgrades CRDs to rke2-calico-crd-v3 (dropping v1 from spec.versions).

  - Updates Calico node/felix/cni deployments to support Calico v3.

  - Sets an explicit state marker (e.g., kubectl annotate node --all calico.rke2.io/crd-version=v3).

## Status

## Context

### Strength of doing process
### Weakness of doing process
### Threats involved in not doing process
### Threats involved in doing process
### Opportunities involved in doing process

### Pros
### Cons
