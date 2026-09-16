#!/bin/bash

set -eu

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

# Retrieve an image tag from the vSphere chart override matching a Kubernetes minor.
#
# Usage: retrieve_vsphere_image_tag.sh <chart-name> <chart-version> <kubernetes-minor> <yq-key>
CHART_NAME="${1}"
CHART_VERSION="${2}"
KUBERNETES_MINOR="${3}"
YQ_KEY="${4}"

VALUES_URL="https://raw.githubusercontent.com/rancher/rke2-charts/refs/heads/main/charts/${CHART_NAME}/${CHART_NAME}/${CHART_VERSION}/values.yaml"

VALUES_FILE=$(mktemp)
trap 'rm -f "${VALUES_FILE}"' EXIT
curl -sfL "${VALUES_URL}" -o "${VALUES_FILE}"

if [[ -z "${KUBERNETES_MINOR}" ]]; then
    fatal "Kubernetes minor is required"
fi

CONSTRAINT="~ ${KUBERNETES_MINOR}"
export CONSTRAINT
OVERRIDE_COUNT=$(yq -r '[.versionOverrides[] | select(.constraint == strenv(CONSTRAINT))] | length' "${VALUES_FILE}")
if [[ "${OVERRIDE_COUNT}" != "1" ]]; then
    fatal "expected one version override with constraint '${CONSTRAINT}', found ${OVERRIDE_COUNT} in ${CHART_NAME}-${CHART_VERSION}"
fi

IMAGE_TAG=$(yq -r "(.versionOverrides[] | select(.constraint == strenv(CONSTRAINT)))${YQ_KEY}" "${VALUES_FILE}")
if [[ "${IMAGE_TAG}" = "null" ]] || [[ -z "${IMAGE_TAG}" ]]; then
    fatal "failed to retrieve image tag for key '${YQ_KEY}' from ${CHART_NAME}-${CHART_VERSION} constraint '${CONSTRAINT}'"
fi

echo "${IMAGE_TAG}"