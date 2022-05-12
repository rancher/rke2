# Automated Upgrades

### Overview

You can manage rke2 cluster upgrades using Rancher's system-upgrade-controller. This is a Kubernetes-native approach to cluster upgrades. It leverages a [custom resource definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-resources), the `plan`, and a [controller](https://kubernetes.io/docs/concepts/architecture/controller/) that schedules upgrades based on the configured plans.

A plan defines upgrade policies and requirements. This documentation will provide plans with defaults appropriate for upgrading a rke2 cluster. For more advanced plan configuration options, please review the [CRD](https://github.com/rancher/system-upgrade-controller/blob/master/pkg/apis/upgrade.cattle.io/v1/types.go).

The controller schedules upgrades by monitoring plans and selecting nodes to run upgrade [jobs](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/) on. A plan defines which nodes should be upgraded through a [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). When a job has run to completion successfully, the controller will label the node on which it ran accordingly.

>**Note:** The upgrade job that is launched must be highly privileged. It is configured with the following:
>
- Host `IPC`, `NET`, and `PID` namespaces
- The `CAP_SYS_BOOT` capability
- Host root mounted at `/host` with read and write permissions

For more details on the design and architecture of the system-upgrade-controller or its integration with rke2, see the following Git repositories:

- [system-upgrade-controller](https://github.com/rancher/system-upgrade-controller)
- [rke2-upgrade](https://github.com/rancher/rke2-upgrade)

To automate upgrades in this manner you must:

1. Install the system-upgrade-controller into your cluster
2. Configure plans


### Install the system-upgrade-controller
The system-upgrade-controller can be installed as a deployment into your cluster. The deployment requires a service-account, clusterRoleBinding, and a configmap. To install these components, run the following command:
```
kubectl apply -f https://github.com/rancher/system-upgrade-controller/releases/download/v0.9.1/system-upgrade-controller.yaml
```
The controller can be configured and customized via the previously mentioned configmap, but the controller must be redeployed for the changes to be applied.


### Configure plans
It is recommended that you minimally create two plans: a plan for upgrading server (master / control-plane) nodes and a plan for upgrading agent (worker) nodes. As needed, you can create additional plans to control the rollout of the upgrade across nodes. The following two example plans will upgrade your cluster to rke2 v1.23.1+rke2r2. Once the plans are created, the controller will pick them up and begin to upgrade your cluster.
```
# Server plan
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: server-plan
  namespace: system-upgrade
  labels:
    rke2-upgrade: server
spec:
  concurrency: 1
  nodeSelector:
    matchExpressions:
       - {key: rke2-upgrade, operator: Exists}
       - {key: rke2-upgrade, operator: NotIn, values: ["disabled", "false"]}
       # When using k8s version 1.19 or older, swap control-plane with master
       - {key: node-role.kubernetes.io/control-plane, operator: In, values: ["true"]}
  serviceAccountName: system-upgrade
  cordon: true
#  drain:
#    force: true
  upgrade:
    image: rancher/rke2-upgrade
  version: v1.23.1-rke2r2
---
# Agent plan
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: agent-plan
  namespace: system-upgrade
  labels:
    rke2-upgrade: agent
spec:
  concurrency: 2
  nodeSelector:
    matchExpressions:
      - {key: rke2-upgrade, operator: Exists}
      - {key: rke2-upgrade, operator: NotIn, values: ["disabled", "false"]}
      # When using k8s version 1.19 or older, swap control-plane with master
      - {key: node-role.kubernetes.io/control-plane, operator: NotIn, values: ["true"]}
  prepare:
    args:
    - prepare
    - server-plan
    image: rancher/rke2-upgrade
  serviceAccountName: system-upgrade
  cordon: true
  drain:
    force: true
  upgrade:
    image: rancher/rke2-upgrade
  version: v1.23.1-rke2r2

```


There are a few important things to call out regarding these plans:

1. The plans must be created in the same namespace where the controller was deployed.

2. The `concurrency` field indicates how many nodes can be upgraded at the same time. 

3. The server-plan targets server nodes by specifying a label selector that selects nodes with the `node-role.kubernetes.io/control-plane` label (`node-role.kubernetes.io/master` for 1.19 or older). The agent-plan targets agent nodes by specifying a label selector that select nodes without that label. Optionally, additional labels can be included, like in the example above, which requires label "rke2-upgrade" to exist and not have the value "disabled" or "false".

4. The `prepare` step in the agent-plan will cause upgrade jobs for that plan to wait for the server-plan to complete before they execute.

5. Both plans have the `version` field set to v1.23.1+rke2r2. Alternatively, you can omit the `version` field and set the `channel` field to a URL that resolves to a release of rke2. This will cause the controller to monitor that URL and upgrade the cluster any time it resolves to a new release. This works well with the [release channels](basic_upgrade.md/#release-channels). Thus, you can configure your plans with the following channel to ensure your cluster is always automatically upgraded to the newest stable release of rke2:
```
apiVersion: upgrade.cattle.io/v1
kind: Plan
...
spec:
  ...
  channel: https://update.rke2.io/v1-release/channels/stable

```

As stated, the upgrade will begin as soon as the controller detects that a plan was created. Updating a plan will cause the controller to re-evaluate the plan and determine if another upgrade is needed.

You can monitor the progress of an upgrade by viewing the plan and jobs via kubectl:
```
kubectl -n system-upgrade get plans -o yaml
kubectl -n system-upgrade get jobs -o yaml
```
