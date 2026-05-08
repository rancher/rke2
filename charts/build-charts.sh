#!/usr/bin/env bash

set -eux -o pipefail
CHARTS_DIR=$(dirname $0)

while IFS='|' read version filename bootstrap take_ownership package; do
  CHART_VERSION=$version CHART_FILE=$CHARTS_DIR/$(basename $filename) CHART_BOOTSTRAP=$bootstrap CHART_TAKE_OWNERSHIP=$take_ownership CHART_PACKAGE=$package $CHARTS_DIR/build-chart.sh
done <<< $(yq e '.charts[] | [(.version // ""), (.filename // ""), (.bootstrap // ""), (.takeOwnership // ""), (.package // "")] | join("|")' $CHARTS_DIR/chart_versions.yaml)
