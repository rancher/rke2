#!/bin/bash

info()
{
    echo '[INFO] ' "$@"
}
warn()
{
    echo '[WARN] ' "$@" >&2
}
fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}


CHART_VERSIONS_FILE="charts/chart_versions.yaml"


CHART_NAME=${1}
CHART_VERSION=${2}

CURRENT_VERSION=$(yq -r '.charts[] | select(.filename == "/charts/'"${1}"'.yaml") | .version' ${CHART_VERSIONS_FILE})
if [ "${CURRENT_VERSION}" != "${CHART_VERSION}" ]; then
    info "chart ${CHART_NAME} should be updated from version ${CURRENT_VERSION} to ${CHART_VERSION}"
    exit 0
fi
fatal "chart ${CHART_NAME} has the latest version"