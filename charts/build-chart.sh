#!/usr/bin/env bash

set -eux -o pipefail

: "${CHART_FILE?required}"
: "${CHART_NAME:="$(basename "${CHART_FILE%%.yaml}")"}"
# Assuming charts that only contain crds are called $CHART_MAIN_NAME-crd
if [[ ${CHART_NAME} = *crd* ]]; then
    : "${CHART_URL:="${CHART_REPO:="https://rke2-charts.rancher.io"}/assets/${CHART_NAME%%-crd}/${CHART_NAME}-${CHART_VERSION:="v0.0.0"}.tgz"}"
else
    : "${CHART_URL:="${CHART_REPO:="https://rke2-charts.rancher.io"}/assets/${CHART_NAME}/${CHART_NAME}-${CHART_VERSION:="v0.0.0"}.tgz"}"
fi
curl -fsSL "${CHART_URL}" -o "${CHART_TMP:=$(mktemp)}"
cat <<-EOF > "${CHART_FILE}"
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: "${CHART_NAME}"
  namespace: "${CHART_NAMESPACE:="kube-system"}"
  annotations:
    helm.cattle.io/chart-url: "${CHART_URL}"
spec:
  bootstrap: ${CHART_BOOTSTRAP:=false}
  chartContent: $(base64 -w0 < "${CHART_TMP}")
EOF
