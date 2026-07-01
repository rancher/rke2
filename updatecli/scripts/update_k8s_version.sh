#!/bin/bash
#
# update_k8s_version.sh
#
# Bumps the embedded Kubernetes patch references in the RKE2 repo to match a
# given hardened-kubernetes build tag (e.g. v1.36.2-rke2r1-build20260610).
#
# This is the "bump" half of the monthly Kubernetes patch process described in
# developer-docs/upgrading_kubernetes.md. It is invoked by the Updatecli
# kubernetes-1.3X.yml policies as a shell *target*, so it runs inside the
# checked-out release branch working tree and any file changes it makes are
# turned into a pull request by Updatecli.
#
# Updatecli contract:
#   * The resolved source value (the new hardened-kubernetes build tag) is
#     appended by Updatecli as the LAST positional argument.
#   * DRY_RUN is exported by Updatecli as "true"/"false".
#
# Usage (as wired in the policies):
#   bash ./updatecli/scripts/update_k8s_version.sh <release-branch> <NEW_IMAGE_TAG>
#
# NOTE: Once rancher/ecm-distro-tools ships a `release update rke2 references`
# command (the RKE2 equivalent of `release update k3s references`), the
# "deterministic edits" block below should be replaced by a single call:
#
#   release update rke2 references "${NEW_K8S_VERSION}" --config "..."
#
# keeping this Updatecli policy as a thin wrapper around the shared CLI.

set -euo pipefail

info() { echo '[INFO] ' "$@"; }
warn() { echo '[WARN] ' "$@" >&2; }
fatal() { echo '[ERROR] ' "$@" >&2; exit 1; }

RELEASE_BRANCH="${1:-unknown}"
# Updatecli appends the source value as the final argument.
NEW_IMAGE_TAG="${2:-}"
DRY_RUN="${DRY_RUN:-true}"

DOCKERFILE="Dockerfile"
VERSION_FILE="scripts/version.sh"
GOMOD_FILE="go.mod"

if [ -z "${NEW_IMAGE_TAG}" ]; then
    fatal "no hardened-kubernetes build tag was provided"
fi

# v1.36.2-rke2r1-build20260610 -> v1.36.2
NEW_K8S_VERSION="${NEW_IMAGE_TAG%%-rke2*}"
if ! echo "${NEW_K8S_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    fatal "could not derive a valid Kubernetes version from tag '${NEW_IMAGE_TAG}'"
fi

CURRENT_IMAGE_TAG=$(grep -E '^KUBERNETES_IMAGE_TAG=' "${VERSION_FILE}" | sed -E 's/.*:-([^}]*)}.*/\1/')
info "release branch:        ${RELEASE_BRANCH}"
info "current image tag:     ${CURRENT_IMAGE_TAG}"
info "candidate image tag:   ${NEW_IMAGE_TAG}"
info "candidate k8s version: ${NEW_K8S_VERSION}"

if [ "${CURRENT_IMAGE_TAG}" == "${NEW_IMAGE_TAG}" ]; then
    info "already on ${NEW_IMAGE_TAG}, nothing to do"
    exit 0
fi

if [ "${DRY_RUN}" == "true" ]; then
    info "dry-run is enabled, no files will be changed"
    exit 0
fi

# --- deterministic edits (replace with `release update rke2 references`) -------

info "updating ${DOCKERFILE} hardened-kubernetes image"
sed -i -E "s|(rancher/hardened-kubernetes:)[^ ]+( AS kubernetes)|\1${NEW_IMAGE_TAG}\2|g" "${DOCKERFILE}"

info "updating ${VERSION_FILE}"
sed -i -E "s|^(KUBERNETES_VERSION=\\\$\{KUBERNETES_VERSION:-)[^}]*(\})|\1${NEW_K8S_VERSION}\2|" "${VERSION_FILE}"
sed -i -E "s|^(KUBERNETES_IMAGE_TAG=\\\$\{KUBERNETES_IMAGE_TAG:-)[^}]*(\})|\1${NEW_IMAGE_TAG}\2|" "${VERSION_FILE}"

info "updating ${GOMOD_FILE} Kubernetes references"
# k8s.io/kubernetes => github.com/k3s-io/kubernetes vX.Y.Z-k3s1  (replace directive)
sed -i -E "s|(github.com/k3s-io/kubernetes )v[0-9]+\.[0-9]+\.[0-9]+-k3s1|\1${NEW_K8S_VERSION}-k3s1|g" "${GOMOD_FILE}"
# k8s.io/kubernetes vX.Y.Z  (require line, not the => replace line)
sed -i -E "s|^([[:space:]]*k8s.io/kubernetes )v[0-9]+\.[0-9]+\.[0-9]+$|\1${NEW_K8S_VERSION}|" "${GOMOD_FILE}"

# Reconcile go.sum / transitive requirements so the resulting PR builds. The k3s
# pseudo-version (github.com/k3s-io/k3s) is intentionally left to `go mod tidy`
# resolving against the matching k3s release, and to reviewer/CI verification.
if command -v go >/dev/null 2>&1; then
    info "running 'go mod tidy' to reconcile go.sum"
    if ! GOFLAGS=-mod=mod go mod tidy; then
        warn "'go mod tidy' failed; the PR may need a manual go.mod/go.sum fix"
    fi
else
    warn "go toolchain not found; skipping 'go mod tidy' (PR will need go.sum reconciliation)"
fi

info "Kubernetes references updated to ${NEW_IMAGE_TAG} on ${RELEASE_BRANCH}"
