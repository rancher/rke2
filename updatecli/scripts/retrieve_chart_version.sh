#!/bin/bash

set -eu

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

# Use the rke2-charts main branch index (the source of truth for the chart
# tarballs and images this automation consumes from
# github.com/rancher/rke2-charts/raw/main/assets). The served Helm repo at
# rke2-charts.rancher.io can lag behind main, which would cause this automation
# to miss newly published chart versions.
CHART_INDEX_FILE_URL="https://github.com/rancher/rke2-charts/raw/main/index.yaml"
export CHART_NAME="${1}"
# Versions are unordered inside the charts file, so we must sort by version in
# reverse order and get the highest.
CHART_VERSION=$(curl -sfL "${CHART_INDEX_FILE_URL}" | yq -r '.entries[strenv(CHART_NAME)][].version' | sort -rV | head -n 1)

if [[ "${CHART_VERSION}" = "null" ]] || [[ -z "${CHART_VERSION}" ]]; then
    fatal "failed to retrieve the charts' index file or to parse it"
fi

echo "${CHART_VERSION}"
