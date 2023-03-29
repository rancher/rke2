# 4. Release Candidate 'RC' Version Format

Date: 2022-07-14

## Status

Rejected

## Question

Should we remove the `-rc` text from the prerelease section of RKE2 releases so that we can reduce the overall steps necessary in releasing?

## Context

Our workflow for generating Release Candidates for RKE2 is the same as our workflow for generating GA RKE2 releases,
with the exception of the "-rc" text in the prerelease section of the git tag.

### Strengths

- reduce CI time by producing one less release
- reduce manual effort by producing one less release (no need to update KDM)
- reduce the time from a release being approved to it being published
- improve reliability of the artifacts by promoting the artifacts tested rather than rebuilding them

### Weaknesses

- if we don't rebuild hardened images, we wouldn't have a way to know the version number of the release candidate
- testing would be more difficult because we wouldn't know the difference between an "rc-1" and an "rc-2"
- GitHub won't let you generate duplicate releases/tags
  - we would either have to delete the release and move the tag (essentially removing the rc version)
  - or figure out some other way to version the release candidates

### Opportunities

- normalizing the process would make it easier to automate
- SLSA compliance states that certification "is not transitive" predicating artifact orientation

### Threats

- a customer might mistake a RC artifact for a GA artifact

## Decision

We need to be able to quickly reference the differences between a release candidate and a general admission release,
and the risk that a user might mistake an RC artifact for a GA artifact is too high for the benefits provided.

## Consequences

We will (continue to) place `-rc` in the prerelease section of the version number for RKE2 tags and releases.
For example : `v1.24.3-rc1+rke2r1`
