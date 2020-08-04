#!/bin/bash
set -e -x

cd $(dirname $0)

export CONTAINER_RUNTIME_ENDPOINT=/run/k3s/containerd/containerd.sock
crictl images -o json \
    | jq -r '.images[].repoTags[0] | select(. != null)' \
    | tee image-list.txt
