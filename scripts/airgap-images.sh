#!/bin/bash

set -ex

cd $(dirname $0)/..

. ./scripts/version.sh

mkdir -p ./build/images/
airgap_image_file='scripts/airgap/image-list.txt'
images=$(cat "${airgap_image_file}")
xargs -n1 docker pull <<< "${images}"
docker save ${images} -o ./build/images/airgap.tar
