#!/bin/bash

info()
{
    echo '[INFO] ' "$@"
}
fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

if [ $# -ne 1 ]; then
    fatal "usage: $0 <hardened-kubernetes-tag>"
fi

CURRENT_TAG=$(sed -nE 's/^KUBERNETES_IMAGE_TAG=\$\{KUBERNETES_IMAGE_TAG:-([^}]*)\}$/\1/p' scripts/version.sh)
NEW_TAG=${1}

if [ -z "${CURRENT_TAG}" ]; then
    fatal "unable to determine the current hardened-kubernetes tag"
fi

if [ "${CURRENT_TAG}" != "${NEW_TAG}" ]; then
    info "hardened-kubernetes should be updated from ${CURRENT_TAG} to ${NEW_TAG}"
    exit 0
fi

fatal "hardened-kubernetes already uses ${NEW_TAG}"
