#!/bin/bash
# Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
iterations=0
response=$(curl -s -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/rancher/rke2/commits?per_page=5&sha=$1")
type=$(echo "$response" | jq -r type)

# Verify if the response is an array with the rke2 commits
if [[ $type == "object" ]]; then
    message=$(echo "$response" | jq -r .message)
    if [[ $message == "API rate limit exceeded for "* ]]; then
        echo "Github API rate limit exceeded"
	exit 1
    fi
    echo "Github API returned a non-expected response ${message}"
    exit 1
elif [[ $type == "array" ]]; then
    echo ${response} | jq -r '.[] | .sha'  &> "$2"
fi

curl -s --fail https://rke2-ci-builds.s3.amazonaws.com/rke2-images.linux-amd64-$(head -n 1 $2).tar.zst.sha256sum
while [ $? -ne 0 ]; do
    ((iterations++))
    if [ "$iterations" -ge 6 ]; then
        echo "No valid commits found"
        exit 1
    fi
    sed -i 1d "$2"
    sleep 1
    curl -s --fail https://rke2-ci-builds.s3.amazonaws.com/rke2-images.linux-amd64-$(head -n 1 $2).tar.zst.sha256sum
done
