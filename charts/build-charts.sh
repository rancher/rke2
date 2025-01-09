#!/usr/bin/env bash

set -eux -o pipefail
CHARTS_DIR=$(dirname $0)

while read version filename bootstrap; do
  CHART_VERSION=$version CHART_FILE=$CHARTS_DIR/$(basename $filename) CHART_BOOTSTRAP=$bootstrap $CHARTS_DIR/build-chart.sh
done <<< $(yq e '.charts[] | [.version, .filename, .bootstrap] | join(" ")' $CHARTS_DIR/chart_versions.yaml)
