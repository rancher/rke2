#!/bin/bash

# Script to to point rke2 to the docker registry running on the host
# This is used to avoid hitting dockerhub rate limits on E2E runners
ip_addr=$1

mkdir -p /etc/rancher/rke2/
echo "mirrors:
  docker.io:
    endpoint:
      - \"http://$ip_addr:5000\"" >> /etc/rancher/rke2/registries.yaml