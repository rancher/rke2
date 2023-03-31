# Support for ServiceLB Load-Balancer Controller in RKE2

Date: 2022-09-30

## Status

Accepted

## Context

RKE2 does not currently bundle a load-balancer controller. Users that want to deploy Services of type
LoadBalancer must deploy a real cloud-provider chart, or use an alternative such as MetalLB or Kube-VIP.

## Decision

Taking advantage of recent changes to [move ServiceLB into the K3s stub
cloud-provider](https://github.com/k3s-io/k3s/blob/master/docs/adrs/servicelb-ccm.md), we will allow RKE2 to run ServiceLB as
part of a proper clould controller integration. This will require adding CLI flags to enable servicelb, as well as exposing
existing K3s flags to configure its namespace. Running servicelb will be opt-in, behind a new flag, to avoid changing
behavior on existing clusters.

## Consequences

* RKE2 uses less resources when ServiceLB is disabled, as several core controllers are no longer started unconditionally.
* The `--disable-cloud-controller` flag now disables the CCM's `cloud-node` and `cloud-node-lifecycle` controllers that were
historically the only supported controllers.
* The `--enable-servicelb` flag now prevents `--disable=servicelb` from being passed in to K3s, which in turn enables the CCM's
`service` controller.
* If the cloud-controller and servicelb are both disabled, the cloud-controller-manager is not run at all.
* The K3s `--servicelb-namespace` flag is now passed through instead of dropped.
