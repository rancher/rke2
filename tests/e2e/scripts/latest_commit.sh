#!/bin/bash
# Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
iterations=0
curl -s -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/rancher/rke2/commits?per_page=5&sha=$1" | jq -r '.[] | .sha'  &> "$2"
curl -s --fail https://storage.googleapis.com/rke2-ci-builds/rke2-images.linux-amd64-$(head -n 1 $2).tar.zst.sha256sum
while [ $? -ne 0 ]; do
    ((iterations++))
    if [ "$iterations" -ge 6 ]; then
        echo "No valid commits found"
        exit 1
    fi
    sed -i 1d "$2"
    sleep 1
    curl -s --fail https://storage.googleapis.com/rke2-ci-builds/rke2-images.linux-amd64-$(head -n 1 $2).tar.zst.sha256sum
done