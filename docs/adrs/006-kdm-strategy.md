# 6. KDM Branching Strategy for RKE2

Date: 2023-09-14

## Status

New

## Context

Rancher Manager uses [KDM](https://github.com/rancher/kontainer-driver-metadata/) for its integration of RKE2. This manages the following behaviors for each RKE2 version:
- Minimum Rancher Manager version
- Maximum Rancher Manager version
- Default version when provisioning via Rancher Manager
- Server Args in RKE2 version
- Agent Args in RKE2 version
- Chart versions in RKE2 version
- Specific Rancher Manager features, such as secrets encryption

Changes for RKE2 are made in the `channels-rke2.yaml` file, and then `go generate` is run to update the `data.json` file with these changes. The `data.json` file is the only file Rancher Manager itself uses.

When a change goes into the `release-x.y` branch of KDM, that is considered production and becomes live for all users of Rancher Manager. The current branching strategy waits for the Rancher Manager team to set a specific development branch in KDM to make PRs and merges against so that the RKE2 team can validate its integration with Rancher Manager. The Rancher Manager team also makes changes in that branch of KDM that they want to release by a certain date, often to enable the latest Kubernetes patch versions for RKE1.

## Decision

To enable a "release-when-ready" approach in KDM, we will have specific branches dedicated to RKE2 in KDM. This allows the Rancher Manager team to release either new RKE1 versions or new RKE2 versions at separate times. It also reduces the cross-team impact of KDM releases as each team will only have their own branches to manage.

## Consequences

When releasing KDM, a new `go generate` might have to be run to ensure data is not lost from the `data.json` file, depending on how Github does with merges on this file.

