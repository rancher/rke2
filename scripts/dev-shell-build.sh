#!/bin/bash

set -e -x

cd $(dirname $0)/..

IMAGE_REPO=ranchertest

. ./scripts/version.sh

mkdir -p build
# getting preloaded images ready
# creating a special tarball to be used within rke2 dev shell and peer
docker save ${IMAGE_REPO}/kubernetes:${VERSION} \
      -o build/rke2-k8s-image-amd64.tar

docker save  rancher/rke2-runtime:${VERSION}-${GOARCH} \
      -o build/rke2-runtime-image-amd64.tar

if [ ! -f dist/artifacts/rke2-airgap-images-amd64.tar ]; then
  echo "please run make package-airgap before running dev-shell"
  exit 1
fi

# build the dev shell image
docker build -t ${PROG}-dev --target shell .
