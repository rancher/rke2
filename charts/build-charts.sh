#!/usr/bin/env bash

set -eux -o pipefail
CHARTS_DIR=$(dirname $0)

while read version filename bootstrap take_ownership; do
  CHART_VERSION=$version CHART_FILE=$CHARTS_DIR/$(basename $filename) CHART_BOOTSTRAP=$bootstrap CHART_TAKE_OWNERSHIP=$take_ownership $CHARTS_DIR/build-chart.sh
done <<< $(yq e '.charts[] | [.version, .filename, .bootstrap, .takeOwnership] | join(" ")' $CHARTS_DIR/chart_versions.yaml)
