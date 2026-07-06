#!/bin/bash
# Emit the highest ingress-nginx "-primeN" tag available for the nginx base
# version that is currently pinned in scripts/build-images.
#
# The base version (e.g. v1.15.1) is derived from the file itself, so this only
# ever advances the prime (CVE rebuild) suffix and never silently jumps the
# nginx minor/patch line - that remains a deliberate, human change.
set -eu

BUILD_IMAGES_FILE="scripts/build-images"
INGRESS_NGINX_REPO="https://github.com/rancher/ingress-nginx.git"

current=$(sed -nr 's/^INGRESS_NGINX_PRIME_TAG=(.*)$/\1/p' "${BUILD_IMAGES_FILE}")
if [ -z "${current}" ]; then
    echo "failed to read INGRESS_NGINX_PRIME_TAG from ${BUILD_IMAGES_FILE}" >&2
    exit 1
fi

base=${current%-prime*}

latest=$(git ls-remote --tags --refs "${INGRESS_NGINX_REPO}" "${base}-prime*" \
    | sed 's#.*refs/tags/##' \
    | awk -F'-prime' '{print $2, $0}' \
    | sort -n \
    | tail -1 \
    | awk '{print $2}')

# If nothing is found, fall back to the current tag so updatecli is a no-op.
if [ -z "${latest}" ]; then
    echo "${current}"
else
    echo "${latest}"
fi
