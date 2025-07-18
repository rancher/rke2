#!/bin/bash

set -eu

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

CHART_INDEX_FILE_URL="https://rke2-charts.rancher.io/index.yaml"
CHART_NAME="${1}"
# Versions are unordered inside the charts file, so we must sort by version in
# reverse order and get the highest.
CHART_VERSION=$(curl -sfL "${CHART_INDEX_FILE_URL}" | yq -r '.entries.'"${CHART_NAME}"'[].version' | sort -rV | head -n 1)

if [[ "${CHART_VERSION}" = "null" ]] || [[ -z "${CHART_VERSION}" ]]; then
    fatal "failed to retrieve the charts' index file or to parse it"
fi

echo "${CHART_VERSION}"
