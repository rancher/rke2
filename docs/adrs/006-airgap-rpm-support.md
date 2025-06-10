# RPM Airgap Support

We should better support RPM installation in air gapped environments

## Established

2023-10-20

## Revisit by

2024-10-01

## Subject

1. When releasing, we bundle RPMs into groups by OS, copy them to a directory, use createrepo to generate local repo information, tarball the directory, and add it as an artifact on the release.
   1. this should be a new process which has no affect on existing users
2. Given install method 'rpm' and variable 'INSTALL_RKE2_ARTIFACT_PATH', the install.sh script looks for a local rpm installation at the given path and installs rke2 using that repo.
   1. a new optional code path is injected to handle this new install type resulting in the least risk potential
3. Given variable 'INSTALL_CUSTOM_RPM_SITE', when run, the install.sh script uses the value of that variable when creating repo files.
   1. specifically the `rpm_site` value in the install.sh is based on this variable
   2. the variable should default to the current hard coded value

## Status

Requesting Feedback

## Context

Users who are concerned with security often deploy in air gapped environments.
These users also often want selinux enforcing.
The rke2 selinux policies rely on RPM installation.
There is a significant use case for RPM installation in air gapped environments.
This use case closely aligns with goals for rke2.

Risks:

- Any change to the install script has the potential to cause interruption with users outside of this use case
  - this is generally true, as with any change we need to weigh the potential value against the risk
    - the potential value here is to reduce the barrier to entry for security minded users
    - the risk can be minimised in implementation to one or two lines of conditionals
- We need to make sure this doesn't interrupt rancher-system-agent or system-agent-installer-rke2
  - the change is to enable a use case for stand-alone RKE2, not to teach Rancher RPM management
  - since we are talking about changing the install script, and the system-agent-installer-rke2 uses the install script, this risk is valid
  - we need to make sure the implementation doesn't make a breaking change to the install script interface
  - nothing in this change requires breaking changes to the interface
    - add an optional variable to use a custom RPM site
    - add context to having both the 'rpm' install method and artifact path variables set

Other Points:

- Strength: RPM installation in air gapped environments will be as simple as tar installation in air gapped environments.
- Strength: This improves our ability to test and enables higher quality support for this use case.
- Strength: When well documented, bundling all dependencies and building within a specific operating system can improve reliability.
- Weakness: RPM bundling requires another step in the release process
- Weakness: RPM bundles have the potential to include dependencies that are not supported in a particular version of an operating system

  - this is only when RPMs have external dependencies, which ours currently don't
- Opportunity: Enable automatically deploying and testing air gapped environments with selinux enforcing on CIS provided VM images.
