# Gateway controller: AgentGateway

## Established

2026-07-14

## Revisit by

-

## Subject

RKE2 currently supports two ingress controllers: `ingress-nginx` and `Traefik`. The former is discontinued and will be soon removed from RKE2. Traefik is a solid project but it does not fulfill all our ingress/gateway controller requirements. Consequently, we decided a few months ago to identify an additional ingress/gateway controller to replace `ingress-nginx` and serve as an alternative to Traefik in scenarios where Traefik is insufficient. 

In the meantime, the SUSE Security team (NeuVector) approached us with several requirements for the ingress/gateway controller. Because these requirements cannot be met by the Traefik community version, we kicked off a collaborative research effort comparing several ingress/gateway controllers.

This ADR shows the results of that research and recommends `AgentGateway` as the `ingress-nginx` replacement.

## Status

Proposed

## Requirements

The requirements for this second ingress/gateway controller are:

1 - It supports Kubernetes Gateway API spec
2 - It performs reliably in high-scaling scenarios
3 - It is backed by an open-source foundation with a healthy and active community
4 - It is not complex to maintain
5 - It supports the ingress related requirements of the CNCF AI Conformance (e.g. Gateway API Inference Extension (GAIE))
6 - (SUSE Security team) It must be extensible to allow building a Web Application Firewall (WAF)

While Traefik is an excellent ingress/gateway controller that fulfills several of these criteria, it falls short on others. For example, Traefik is governed by a VC-funded company rather than an open-source foundation. Additionally, peerformance benchmarks indicate some degradation in highly scalable environments. Moreover, the Traefik community version, does not allow the necessary extensibility.

## Research

| Options | Gateway API | High-scale | Open source | Simple to maintain | AI ingress (GAIE) | WAF Extensibility |
|---|---|---|---|---|---|---|
| Envoy Gateway | ✅ Full conformance | ⚠️ Some performance degradation and errors [2] | ✅ CNCF project healthy community | ⚠️ Envoy proxy (C++ beast)  | ✅ Envoy AI Gateway add-on | ✅ Via Envoy extensions |
| kGateway | ✅ Full conformance | ✅ Very scalable [1] | ⚠️ CNCF sandbox but only one VC-funded company contributing (solo.io) | ⚠️ Envoy proxy (C++ beast) | ❌ Via agentgateway (separate project) | ✅ Possible to extend |
| Contour | ✅ Supported | ⚠️ Missing benchmark. Probably similar to Envoy Gateway | ⚠️ CNCF incubating, LF project but not a lot of activity lately | ⚠️ Envoy proxy (C++ beast) | ❌ No AI/inference features | ❌ Not easy to extend |
| AgentGateway | ✅ Full Conformance | ✅ Rust-based, built for stateful/concurrent AI workloads [2] | ⚠️ AAIF/Linux Foundation but only one VC-funded company contributing (solo.io) | ⚠️ Go + Rust | ✅ Native GAIE implementation | ✅ Possible to extend and inject a WAF |


[AgentGateway](https://github.com/agentgateway/agentgateway) looks like the best candidate at this point. However, we should pay attention at its community evolution. It is a fairly new project (1 year old) that is currently developed by only one VC-funded company (solo.io).

[1] https://github.com/howardjohn/gateway-api-bench/blob/main/README.md#summary-of-findings
[2] https://github.com/howardjohn/gateway-api-bench/blob/main/README-v2.md#summary-of-findings


## Integrate Agentgateway in RKE2

Given the current roadmap, set of priorities and team size, natively integrating and providing L3 support for an additional ingress/gateway controller is highly challenging. However, the SUSE Security team requires a prompt decision to being implementing their WAF on top of the chosen gateway controller.

To balance these constraints, the best path forward is to use Application Collection (AppCo) to package and integrate AgentGateway. Because the SUSE Security team already consume its components via AppCo, and because an ingress/gateway controller is ont required to bootstrap the core RKE2 cluster, it can easily be installed as a post-bootstrap component.

Additionally, AppCo can have FIPS-compliant and hardened images, fulfilling RKE2's security requirements.

If priorities shift or team bandwidth increases, we can evaluate migrating AgentGateway to a native out-of-the-box integration into RKE2 (similar to Traefik).


### AppCo in RKE2 Example

The following example demonstrates how to install an AppCo-packaged component in RKE2. We will pick Traefik for the example:

1 - Log into the AppCo web application and generate a service account to obtain your credentials (user/password)
2 - Create a Kubernetes image pull secret using these credentials:

```
kubectl create secret docker-registry application-collection \
--docker-server=dp.apps.rancher.io \
--docker-username=USERNAME --docker-password=PASSWORD
```

3 - Locate the application in the AppCo portal and deploy it using the provided Helm command:

```
helm install traefik oci://dp.apps.rancher.io/charts/traefik-proxy \
--version 41.0.2 --set global.imagePullSecrets={application-collection}
```

For easy installation in Airgap environments, AppCo supports [Hauler](https://docs.apps.rancher.io/howto-guides/integrate-with-hauler)


## Decision

* Adopt AgentGateway as the 2nd gateway controller to replace `ingress-nginx`
* Initially use Application Collection as the way to consume AgentGateway
* Describe in docs how to consume AgentGateway and define one test case in QASE
