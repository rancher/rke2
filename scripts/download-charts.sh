#!/bin/bash
set -ex

CHARTS_DIR=build/static/charts
MANIFEST_DIR=manifests
CHARTS="canal:v3.13.3 coredns:1.10.101 kube-proxy:v1.18.4 metrics-server:2.11.100 nginx-ingress:1.36.300"
CHARTS_REPO="https://dev-charts.rancher.io"

mkdir -p ${CHARTS_DIR}
for chart in ${CHARTS}; do
  chart_name=$(echo "${chart}" | cut -d ":" -f 1)
  chart_version=$(echo "${chart}" | cut -d ":" -f 2)
  curl -sfL ${CHARTS_REPO}/${chart_name}/${chart_name}-${chart_version}.tgz -o ${CHARTS_DIR}/${chart_name}-$chart_version.tgz
  chart_content=$(base64 -w 0 ${CHARTS_DIR}/${chart_name}-${chart_version}.tgz)
  sed -e "s|%{CHART_CONTENT}%|${chart_content}|g" ${MANIFEST_DIR}/${chart_name}.yml >${CHARTS_DIR}/${chart_name}-chart.yml
  rm ${CHARTS_DIR}/${chart_name}-${chart_version}.tgz
done
