#!/usr/bin/env bash

set -eux -o pipefail

: "${KUBERNETES_VERSION:=v0.0.0-0}"
: "${CHART_FILE?required}"
: "${CHART_NAME:="$(basename "${CHART_FILE%%.yaml}")"}"
: "${CHART_PACKAGE:="${CHART_NAME%%-crd}"}"
: "${TAR_OPTS:=--owner=0 --group=0 --mode=gou-s+r --numeric-owner --no-acls --no-selinux --no-xattrs}"
: "${CHART_URL:="${CHART_REPO:="https://rke2-charts.rancher.io"}/assets/${CHART_PACKAGE}/${CHART_NAME}-${CHART_VERSION:="0.0.0"}.tgz"}"
: "${CHART_TMP:=$(mktemp --suffix .tar.gz)}"
: "${YAML_TMP:=$(mktemp --suffix .yaml)}"

cleanup() {
  exit_code=$?
  trap - EXIT INT
  rm -rf ${CHART_TMP} ${CHART_TMP/tar.gz/tar} ${YAML_TMP}
  exit ${exit_code}
}
trap cleanup EXIT INT

if [ "$CHART_VERSION" == "0.0.0" ]; then
  echo "# ${CHART_NAME} has been removed" > "${CHART_FILE}"
  exit
fi

curl -fsSL "${CHART_URL}" -o "${CHART_TMP}"
gunzip ${CHART_TMP}

# Extract out Chart.yaml, inject a version requirement and bundle-id annotation, and delete/replace the one in the original tarball
tar -xOf ${CHART_TMP/.gz/} ${CHART_NAME}/Chart.yaml > ${YAML_TMP}
yq -i e ".kubeVersion = \">= ${KUBERNETES_VERSION}\" | .annotations.\"fleet.cattle.io/bundle-id\" = \"rke2\"" ${YAML_TMP}
tar --delete -b 8192 -f ${CHART_TMP/.gz/} ${CHART_NAME}/Chart.yaml
tar --transform="s|.*|${CHART_NAME}/Chart.yaml|" ${TAR_OPTS} -vrf ${CHART_TMP/.gz/} ${YAML_TMP}

pigz -11 ${CHART_TMP/.gz/}

cat <<-EOF > "${CHART_FILE}"
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: "${CHART_NAME}"
  namespace: "${CHART_NAMESPACE:="kube-system"}"
  annotations:
    helm.cattle.io/chart-url: "${CHART_URL}"
    rke2.cattle.io/inject-cluster-config: "true"
spec:
  bootstrap: ${CHART_BOOTSTRAP:=false}
  chartContent: $(base64 -w0 < "${CHART_TMP}")
EOF
