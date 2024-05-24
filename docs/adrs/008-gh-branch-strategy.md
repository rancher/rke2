# 8. Branching Strategy in Github

Proposal Date: 2024-05-23

## Status

Accepted

## Context

RKE2 is released at the same cadence as upstream Kubernetes. This requires management of multiple versions at any given point in time. The current branching strategy uses `release-v[MAJOR].[MINOR]`, with the `master` branch corresponding to the highest version released based on [semver](https://semver.org/). Github's Tags are then used to cut releases, which are just point-in-time snapshots of the specified branch at a given point. As there is the potential for bugs and regressions to be on present on any given branch, this branching and release strategy requires a code freeze to QA the branch without new potentially breaking changes going in.

## Decision
All code changes go into the `master` branch. We maintain branches for all current release versions in the format `release-v[MAJOR].[MINOR]`. When changes made in master are necessary in a release, they should be backported directly into the release branches. If ever there are changes required only in the release branches and not in master, such as when bumping the kubernetes version from upstream, those can be made directly into the release branches themselves.

## Consequences

- Allows for constant development, with code freeze only relevant for the release branches.
- This requires maintaining one additional branch than the current workflow, which also means one additional issue.
- Testing would be more constant from the master branch.
- When a new minor release is available, the creation of the new release branch will be the responsibility of the engineer that merges the PR bumping Kubernetes to the new minor version. It will happen as soon as that PR is merged.
