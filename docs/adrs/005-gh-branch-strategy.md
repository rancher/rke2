# 5. Branching Strategy in Github

Date: 2023-03-29

## Status

Proposed

## Context

RKE2 is released at the same cadence as upstream Kubernetes. This requires management of multiple versions at any given point in time. The current branching strategy uses `release-v[MAJOR].[MINOR]`, with the `master` branch corresponding to the highest version released based on [semver](https://semver.org/). Github's Tags are then used to cut releases, which are just point-in-time snapshots of the specified branch at a given point. As there is the potential for bugs and regressions to be on present on any given branch, this branching and release strategy requires a code freeze to QA the branch without new potentially breaking changes going in.

## Decision

We will introduce additional branches as `dev-v[MAJOR].[MINOR]` that will be where all standard development will occur. The `release-v[MAJOR].[MINOR]` branches will still be used to release from.

## Consequences

- Allows for constant development and no code freeze.
- Any critical CVEs that would necessitate releasing again outside of the standard cadence can be added directly to the `release` branches and then ported back to the `dev` branches.
- Additional overhead for whomever is managing the releases as there are more PRs required to bring the `release` branches in line with `dev` branches.
- Additional storage required for commitid builds due to more branches.
