# 5. K3S Release Dependency

Date: 2022-07-14

## Status

Proposed

## Question

Should we wait to release RKE2 until after K3S is released so that we can update the `go.mod` to use the latest released modules?

## Context

RKE2 uses pinned modules provided by K3S as libraries.
If we don't wait to release RKE2 until K3S release is complete, then the RKE2 release may not have the latest of the K3S modules.
Coupling RKE2 Release to K3S release will put unnecessary time constraints on our release process,
 updating the modules in `go.mod` after the release ensures that the modules are always in their "stable" form.

### Strengths of waiting

- RKE2 will always have the latest K3S libraries

### Weaknesses of waiting

- the RKE2 release process will have to wait until K3S is complete, placing time constraints on the RKE2 release

### Opportunities from waiting

- using the previous K3S release modules allows time for discovery of issues before they reach RKE2

### Threats caused by waiting

- unstable K3S modules might be used in RKE2, resulting in K3S bugs propagating to RKE2

### Threats caused by not waiting

- we end up shipping RKE2 with a mishmash of Kubernetes versions

## Decision


## Consequences

