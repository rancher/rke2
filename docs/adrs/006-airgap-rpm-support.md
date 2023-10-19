# RPM Airgap Support

We should better support RPM installation in air gapped environments

## Established

2023-10-20

## Revisit by

2024-10-01

## Subject

1. When releasing, we bundle RPMs into groups by OS, copy them to a directory, use createrepo to generate local repo information, tarball the directory, and add it as an artifact on the release.
2. Given install method 'rpm' and variable 'INSTALL_RKE2_ARTIFACT_PATH', when run, the install.sh script looks for a local rpm installation at the given path and installs rke2 using that repo.
3. Given variable 'INSTALL_CUSTOM_RPM_SITE', when run, the install.sh script uses the value of that variable when creating repo files.
   1. specifically the `rpm_site` value in the install.sh is based on this variable

## Status

Requesting Feedback

## Context

Users who are concerned with security often deploy in air gapped environments.
These users also often want selinux enforcing.
The rke2 selinux policies rely on RPM installation.
There is a significant use case for RPM installation in air gapped environments.
This use case closely aligns with goals for rke2.

Strengths:

- RPM installation in air gapped environments will be as simple as tar installation in air gapped environments.
- This improves our ability to test and enables higher quality support for this use case.
- When well documented, bundling all dependencies and building within a specific operating system can improve reliability.

Weaknesses:

* RPM bundling requires another step in the release process
* RPM bundles have the potential to include dependencies that are not supported in a particular version of an operating system
  * this is only when RPMs have external dependencies, which ours currently don't

Threats involved in not doing process:

* Users with this use case continue to depend on some infrastructure (beyond a hypervisor) being in place before they can use rke2;

Threats involved in doing process:

* Any change to the install script has the potential to cause interruption with users outside of this use case.

Opportunities:

* Enable automatically testing air gapped environments with selinux enforcing on CIS provided VM images.
* Enable automatically deploying air gapped environments with selinux enforcing on CIS provided VM images.
