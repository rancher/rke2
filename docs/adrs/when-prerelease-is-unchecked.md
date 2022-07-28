# When Prerelease Box is Unchecked

The release order is important, we need to make sure we are not introducing any bugs in the order of release.

## Established

2022-07-28

## Revisit by

2023-07-28

## Subject

Given approval for GA release, after channel server is updated,
  the last operation we perform is to uncheck the prerelease box on the releases.

## Status

Approved

## Context

### Strength of doing process

- this is a good final marker for the release

### Weakness of doing process

- the channel server will not be aware of the release until the box is unchecked

### Threats involved in not doing process

- a user who expects to use the "latest" release may not be able to see the release in the channel server

### Threats involved in doing process

- no clear marker for the end of the release process

### Opportunities involved in doing process

- lead time becomes more clear with a mechanism to denote the end of a release