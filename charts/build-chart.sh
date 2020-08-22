#!/usr/bin/env bash
set -eux -o pipefail
: "${CHART_FILE?required}"
cat <<-EOF > "${CHART_FILE}"
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: "${CHART_NAME:="$(basename "${CHART_FILE%%-chart.yml}")"}"
  namespace: "${CHART_NAMESPACE:="kube-system"}"
  annotations:
    helm.cattle.io/chart-url: "${CHART_URL:="${CHART_REPO:="https://rke2-charts.rancher.io"}/assets/${CHART_NAME}/${CHART_NAME}-${CHART_VERSION:="v0.0.0"}.tgz"}"
spec:
  bootstrap: ${CHART_BOOTSTRAP:=false}
  chartContent: $(curl -fsSL "${CHART_URL}" | base64 -w0)
EOF
