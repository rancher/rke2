#!/bin/bash

set -eu

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

CHART_INDEX_FILE_URL="https://rke2-charts.rancher.io/index.yaml"
CHART_NAME="${1}"
# Retrieves the first entry '[0]', because we expect that the versions are
# already ordered from last (more recent) to older.
CHART_VERSION=$(curl -sfL "${CHART_INDEX_FILE_URL}" | yq -r '.entries.'"${CHART_NAME}"'[0].version')

if [[ "${CHART_VERSION}" = "null" ]] || [[ -z "${CHART_VERSION}" ]]; then
    fatal "failed to retrieve the charts' index file or to parse it"
fi

echo "${CHART_VERSION}"
