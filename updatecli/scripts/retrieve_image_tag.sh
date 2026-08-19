#!/bin/bash

set -eu

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

# Retrieve an image tag from a published chart's values.yaml.
#
# Usage: retrieve_image_tag.sh <chart-name> <chart-version> <yq-key>
#   <chart-name>     e.g. rke2-metrics-server
#   <chart-version>  e.g. 3.13.106
#   <yq-key>         yq expression for the tag, e.g. .image.tag
CHART_NAME="${1}"
CHART_VERSION="${2}"
YQ_KEY="${3}"

VALUES_URL="https://raw.githubusercontent.com/rancher/rke2-charts/refs/heads/main/charts/${CHART_NAME}/${CHART_NAME}/${CHART_VERSION}/values.yaml"

IMAGE_TAG=$(curl -sfL "${VALUES_URL}" | yq -r "${YQ_KEY}")

if [[ "${IMAGE_TAG}" = "null" ]] || [[ -z "${IMAGE_TAG}" ]]; then
    fatal "failed to retrieve image tag for key '${YQ_KEY}' from ${CHART_NAME}-${CHART_VERSION}"
fi

echo "${IMAGE_TAG}"
