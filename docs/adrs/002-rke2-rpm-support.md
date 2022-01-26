# 2. RPM support for RKE2

Date: 2022-01-20

## Status

Accepted

## Context

RKE2 publishes RPMs for distribution of RKE2 through the https://github.com/rancher/rke2-packaging repository. These RPMs are built using automated calls to `rpmbuild` and corresponding GPG signing/publishing plugins, and publish RPMs to the `rpm.rancher.io`/`rpm-testing.rancher.io` S3-backed buckets.

## Decision

Until a more robust RPM building/mechanism is established for RKE2, we will not add any new platforms for RPM publishing beyond the existing CentOS/RHEL 7 and 8 RPMs that are published. We will publish selinux policy RPMs for new platforms as needed, and ensure the selinux RPMs are compatible with the tarball installation method for the platform in question.

This decision can be re-evaluated in the future if a more robust RPM publishing technique/platform is developed/made available. 

## Consequences

The only supported installation method for all platforms except CentOS 7/8 with selinux support will be a combination of the use of a tarball install in conjunction with an selinux policy RPM.

