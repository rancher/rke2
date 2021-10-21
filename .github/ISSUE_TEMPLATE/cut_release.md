---
name: Cut Release
about: Create a new release checklist
title: 'Cut VERSION'
labels: 'kind/release'
assignees: ''

---

**Summary:**
Task covering patch release work.

Dev Complete: RELEASE_DATE (Typically ~1 week prior to upstream release date)

**List of required releases:**

_To release as soon as able for QA:_
- VERSION

_To release once have approval from QA:_
- VERSION (Never release on a Friday unless specified otherwise)

**Prep work:**
- [ ] PM: Dev and QA team to be notified of the incoming releases - add event to team calendar
- [ ] PM: Dev and QA team to be notified of the date we will mark the latest release as stable - add event to team calendar [ONLY APPLICABLE FOR LATEST MINOR RELEASE]
- [ ] QA: Review changes and understand testing efforts
- [ ] Release Captain: Prepare release notes in our private [release-notes repo](https://github.com/rancherlabs/release-notes) (submit PR for changes taking care to carefully check links and the components, once merged, create the release in GitHub and mark as a draft and check the pre-release box, fill in title, set target release branch, leave tag version blank for now until we are ready to release)
- [ ] QA: Validate and close out all issues in the release milestone.

**Vendor and release work:**
To find more information on specific steps, please see documentation [here](https://github.com/rancher/rke2/blob/master/developer-docs/upgrading_kubernetes.md)
- [ ] Release Captain: Tag new Hardened Kubernetes release
- [ ] Release Captain: Update Helm chart versions
- [ ] Release Captain: Update RKE2
- [ ] Release Captain: Tag new RKE2 RC
- [ ] Release Captain: Tag new RKE2 packaging RC "testing"
- [ ] Release Captain: Tag the RKE2 release
- [ ] Release Captain: Add release notes to release
- [ ] Release Captain: Tag RKE2 packaging release "testing"
- [ ] Release Captain: Tag RKE2 packaging release "latest"

**Post-Release work:**
- [ ] Release Captain: Once release is fully complete (CI is all green and all release artifacts exist), edit the release, uncheck "Pre-release", and save.
- [ ] Wait 24 hours
- [ ] Release Captain: Tag RKE2 packaging "stable"
- [ ] Release Captain: Update stable release in channels.yaml
- [ ] Release Captain: Prepare PRs as needed to update [KDM](https://github.com/rancher/kontainer-driver-metadata/) in the appropriate dev branches.  For more information on the structure of the PR, see the [docs](https://github.com/rancher/rke2/blob/master/developer-docs/upgrading_kubernetes.md#update-rancher-kdm)
- [ ] PM: Close the milestone in GitHub.
