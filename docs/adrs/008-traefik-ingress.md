# Support for Alternative Ingress Controllers

Date: 2024-05-21

## Status

Accepted

## Context

RKE2 currently supports only a single ingress controller, ingress-nginx.
It has been requested RKE2 support alternative ingress controllers, similar to how RKE2 supports multiple CNIs. 

## Decision

* A new --ingress-controller flag will be added; the default will be only `ingress-nginx` to preserve current behavior.
* All selected ingress controllers will be deployed to the cluster.
* The first selected ingress controller will be set as the default, via the `ingressclass.kubernetes.io/is-default-class` annotation
  on the IngressClass resource.
* Any packaged ingress controllers not listed in the flag value will be disabled, similar to how inactive packaged CNIs are handled.
* RKE2 will package Traefik's HelmChart as a supported ingress controller, deploying as a Daemonset + ClusterIP Service
  for parity with the `ingress-nginx` default configuration due to RKE2's lack of a default LoadBalancer controller.
* RKE2 will use mirrored upstream Traefik images; custom-rebuilt hardened-traefik images will not be provided or supported.

## Consequences

* We will add an additional packaged component and CLI flag for ingress controller selection.
* We will need to track updates to Traefik and the Traefik chart.
* QA will need additional resources to test the new ingress controllers.
