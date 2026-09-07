# Change Default FailureAction to `retry` for Bundled Charts

Date: 2026-07-23

## Status

Accepted

## Context

RKE2 includes a bundled Helm controller that uses a container image that bundles the helm CLI tools to install, upgrade, and uninstall Helm charts.

The image used by the Helm controller recently moved from bundling v3 releases of the Helm CLI, to v4. This was due to Helm v3 reaching end-of-life status, which made it no longer eligible for security updates.

One of the changes included in Helm v4 is to use [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management) to effect changes to resources managed by Helm.
One side-effect of this is that Helm now respects field ownership, and any change made to field set by Helm, cannot be changed by another entity, without causing conflicts that move the Chart release into a `failed` state.

#### Example 1:
1. A Chart's template and values specify a Deployment with 1 replica
2. The user manually scales the Deployment (ex: `kubectl scale deployment EXAMPLE-DEPLOYMENT --replicas=3`)
3. The user makes a change to the chart version or values, triggering a `helm upgrade` operation
4. The upgrade fails and creates a new release version with `failed` status, due to `kubectl scale` now owning the Deployment's `.spec.replicas` field.

#### Example 2:
1. A Chart deploys a Custom Resource, and a Operator that manages that custom resource (ex: Calico's `Installation` resource)
2. The Operator runs, and updates the Custom Resource to modify fields based on the running state of the cluster.
3. The user makes a change to the chart version or values that do not affect the Custom Resource, triggering a `helm upgrade` operation.
4. The upgrade fails and creates a new release version with `failed` status, due to the operator now owning some of the fields on the Custom Resource.

Historically, the only things that would cause the chart release to move into a `failed` state were incompatible changes to Kubernetes resources that required delete and recreate of a resource, or unexpected early termination of a Helm upgrade Pod.
With the new behavior of Helm v4, failure is much more likely.

When the Chart moves into a `failed` state, the default failure policy is `reinstall`, which uninstalls and reinstalls the chart to restore it to a known-good state.
While this was effective in ensuring that charts were consistently installed and upgraded, there were scenarios where this could cause unexpected outages or data loss.
For this reason, many users have deployed HelmChartConfigs that override the failure policy to `abort`, which simply leaves the chart in a failed state, awaiting external action from the cluster administrator to resolve the issue.

The Helm controller recently added a new field (`.spec.forceConflicts`) and a new failure policy (`.spec.failurePolicy: retry`) to allow Helm to retry the operation and reclaim management of the conflicting fields.

## Decision

1. Helm charts bundled with RKE2 will set `.spec.forceConflicts: true` to restore the previous behavior of ignoring field management conflicts.
2. Helm charts bundled with RKE2 will set `.spec.failurePolicy: retry` to non-disruptively retry upgrades in case of failure, instead of uninstalling and reinstalling.
3. These changes will only be made to HelmChart resources bundled with RKE2.

#### Notes
If users have deployed HelmChartConfig resources that override the failure policy, those will still be respected.
If the user has overridden the policy to `abort`, the Helm controller will not retry the upgrade with `--force-conflicts` set, nor will it uninstall and reinstall.
The administrator will need to manually correct the failure before the chart can be upgraded or new values applied.

Any HelmChart resources deployed by users will continue to see the legacy behavior of the default failure policy and Helm v4 field management conflicts.

## Consequences

1. Helm charts bundled with RKE2 will ignore conflicts in server-side apply field management, by use of Helm v4's `--force-conflicts` flag. This re-aligns with the default behavior of prior releases of RKE2 that used Helm v3.
2. Helm charts bundled with RKE2 will retry the upgrade when the latest helm release is `failed` state, or was moved to `failed` state due to being left in a `pending` state due to interrupted upgrade operation.
   Retry operations will be visible to users by continuous creation of Events reporting the attempt, attached to the HelmChart resource, emitted by helm-controller. Users will also observe the Helm Job and Pods being recreated to retry the upgrade.
3. Helm charts bundled with RKE2 may be less resilient to failed upgrades if the failure cannot be resolved by a simple retry of the operation.
