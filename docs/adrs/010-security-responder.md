# security-responder client

Date: 2025-10-27

## Status

Proposed

## Context

### Background

RKE2 currently lacks two critical capabilities essential for its maintainability and user security. Firstly, there is no structured mechanism for users to voluntarily share cluster metadata (such as Kubernetes version or CNI plugins). This data is vital for maintainers to understand real-world adoption, correctly prioritize future development or testing efforts.

Secondly, users lack a rapid, in-cluster method to learn about security threats. This includes an immediate list of CVEs impacting their current components and a recommended, secure version they can upgrade to.


### Current State

- No telemetry collection exists in rke2. The team lack insights into deployment patterns, version adoption or selected configurations
- Difficult for users to learn about existing CVEs impacting their current RKE2 version

### Requirements

- Collect only non-personally identifiable cluster metadata
- Opt-out mechanism with clear documentation
- Minimal resource overhead
- Fails gracefully in disconnected environments
- There is no need for retry mechanisms or a persistent daemon; the data is non-critical and loss of a few data points harmless. Resource savings on the nodes are more important.
- Work well in rke2
- Provides useful information to users

## Decision

Implement a `security-responder` client at `github.com/rancher/rke2-security-responder` (similar to existing components) as a separate, optional component deployed via the rke2 manifest system that is triggered periodically.

### Architecture

- **Deployment Method**: `CronJob` in `kube-system` namespace
- **Location**: `/var/lib/rancher/rke2/server/manifests/security-responder.yaml`
- **Scheduling**: CronJob running thrice daily (`0 */8 * * *`)
- **Configuration**: ConfigMap-based with environment variable override
- **Default State**: Enabled by default (opt-out well documented)

### Data Collection

The collected data will include the following information:
- Kubernetes version
- clusteruuid
- serverNodeCount
- agentNodeCount
- cni-plugin
- ingress-controller
- os
- selinux

Example payload structure:
```json
{
  "appVersion": "v1.31.6+rke2r1",
  "extraTagInfo": {
    "kubernetesVersion": "v1.31.6",
    "clusteruuid": "53741f60-f208-48fc-ae81-8a969510a598"
  },
  "extraFieldInfo": {
    "serverNodeCount": 3,
    "agentNodeCount": 2,
    "cni-plugin": "flannel",
    "ingress-controller": "rke2-ingress-nginx",
    "os": "ubuntu",
    "selinux": "enabled"
  }
}
```

The `clusteruuid` is needed to differentiate between different deployments (the UUID of `kube-system`). It is completely random and does not expose privacy considerations. We could even consider hashing it to increase the obfuscation.

### Configuration Interface Example

The security-responder is packaged using a helm chart. We can interact with it as we do with other helm charts. For example, to disable it: 

```yaml
# /etc/rancher/rke2/config.yaml
disable:
- rke2-security-responder
```

## Alternatives Considered

### Agent-based Implementation

Would require agents on all nodes. Periodic CronJob is more efficient for cluster-level metadata collection.

### Instrumenting/leveraging update.rke2.io

No easy access to CDN logs, no insights into deployed versions, not as privacy-preserving.

## Consequences

Basic telemetry coverage and analytics to improve project decisions and project visibility.
