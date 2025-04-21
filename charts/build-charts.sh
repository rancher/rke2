#!/usr/bin/env bash

set -eux -o pipefail
CHARTS_DIR=$(dirname $0)

while read version filename bootstrap package; do
  if [ -n "$package" ]; then export CHART_PACKAGE="${package}"; else unset CHART_PACKAGE; fi
  CHART_VERSION=$version CHART_FILE=$CHARTS_DIR/$(basename $filename) CHART_BOOTSTRAP=$bootstrap $CHARTS_DIR/build-chart.sh
done <<< $(yq e '.charts[] | [.version, .filename, .bootstrap, .package] | join(" ")' $CHARTS_DIR/chart_versions.yaml)
