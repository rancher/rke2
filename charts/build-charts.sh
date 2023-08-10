#!/usr/bin/env bash

set -eux -o pipefail

while read version filename bootstrap; do
    CHART_VERSION=$version CHART_FILE=$filename CHART_BOOTSTRAP=$bootstrap /charts/build-chart.sh
done <<< $(yq e '.charts[] | [.version, .filename, .bootstrap] | join(" ")' /charts/chart_versions.yaml)
