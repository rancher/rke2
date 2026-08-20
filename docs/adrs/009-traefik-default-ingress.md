# Drop Support for Ingress-Nginx

Date: 2024-07-25

## Status

Accepted

## Context

### Summary

For most of its existence, RKE2 has shipped with ingress-nginx as its Ingress Controller. In
`008-traefik-default-ingress.md`, we added support for alternative Ingress controllers, and started shipping
Traefik. The team would like to explore switching to deploying Traefik as the default, with a long-term goal
of dropping support and maintenance of the rke2-ingress-nginx chart and our hardened-ingress-nginx images.

* Ingress-nginx is a community project without a major corporate sponsor, and the project is currently suffering
  from a shortage of maintainers.
* Nginx plugins are shipped as shared libraries that have dependencies on other libraries provided by the base
  image distribution, which makes updating the base image a complicated and error-prone process.
* The controller image itself has a very complicated build process, and bundles both a golang-based ingress
  controller, and the C-based Nginx Open Source web server/proxy. The image must be updated to address
  vulnerabilities in golang, golang modules, nginx, and shared libraries used by nginx.

### Pros

* Improved user experience by shipping an ingress controller that supports non-disruptive hot reloading of
  ingress configuration changes.
* Improved user experience for users managing both K3s and RKE2 clusters due to consistent component selection
  across both distros.
* Reduced team workload by removing need to maintain our hardened fork of the ingress-nginx image.

### Cons

* Users will need to transition to either the community ingress-nginx chart, or to traefik.
* Users may initially find it more difficult to use traefik, as ingress-nginx has been the de-facto default
  ingress controller for a long time. Lots of documentation, tutorials, and charts reference
  ingress-nginx-specific annotations that are not respected by other controllers.
* The team will need to take on additional work to maintain a hardened build of the traefik image.
* The team will need to maintain additional code in RKE2 to handle leaving the current rke2-ingress-nginx
  chart in place after the default has been changed.

## Decision

* TBA

## Consequences

* TBA
