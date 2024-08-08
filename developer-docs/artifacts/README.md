# Sources

This directory contains documentation on the artifacts used to build, release, test, and run RKE2.
The general idea is to give developers a way to quickly reference the list of artifacts without having to dig through the multiple codebases.

## Example use cases

### CVE scanning

- you might use this to cross reference CVE scans made on container images to find a list of binaries that need to be updated
- you might use this to cross reference CVE scans made on container images to find the impact of the CVE on the RKE2 project

### Hidden Dependencies

sometimes how things are built is not always clear in the codebase, this can be used to find hidden dependencies
- you might need to understand the build process for a helm chart to understand how we patch them from upstream for RKE2
