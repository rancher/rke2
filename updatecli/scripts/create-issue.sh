#!/bin/bash

GITHUB_REPOSITORY="rancher/rke2"
CHART_VERSIONS_FILE="charts/chart_versions.yaml"
CHART_NAME=${1}
CHART_VERSION=${2}

check-issue() {
    MILESTONES_JSON=$(gh api repos/${GITHUB_REPOSITORY}/milestones)
    TODAY=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    TARGET_MILESTONE=$(echo "$MILESTONES_JSON" | jq -r --arg today "$TODAY" '
      [ .[] | select(.title | contains("Release Cycle")) | select(.due_on >= $today) | select(.state == "open")] 
      | sort_by(.due_on) 
      | .[0].title
    ')

    if [ "$TARGET_MILESTONE" == "null" ] || [ -z "$TARGET_MILESTONE" ]; then
      echo "No unexpired Release Cycle milestone found."
      exit 1
    fi

    ISSUE_TITLE="Update CNIs for $TARGET_MILESTONE"

    issue=$(
        gh issue list \
            --repo ${GITHUB_REPOSITORY} \
            --state "open" \
            --search "${ISSUE_TITLE}" \
            --json number,body
    )
    if [[ $(echo "$issue" | jq '. | length') -eq 0 ]]; then
       issue_url=$(gh issue create \
           --title "${ISSUE_TITLE}" \
           --body "Update CNIs for latest release:
- ${CHART_NAME}:${CHART_VERSION}" \
           --repo ${GITHUB_REPOSITORY} \
           --milestone "${TARGET_MILESTONE}" 2>&1)
       if [ $? -eq 0 ]; then
	  number=$(echo "$issue_url" | awk -F'/' '{print $NF}')
       else
          echo "Failed to create issue"
          exit 1
       fi
    else
       number=$(echo $issue | jq -s 'sort_by(.[].number) | .[0]' | jq -r '.[].number')
       body=$(echo $issue | jq -s 'sort_by(.[].number) | .[0]' | jq -r '.[].body')
       new_body=$(update_issue_body "${body}" ${CHART_NAME} ${CHART_VERSION})
       issue_url=$(gh issue edit \
              ${number} \
              --repo ${GITHUB_REPOSITORY} \
	      --body "${new_body}")
    fi
    echo $number
}

update_issue_body() {
    local original_body="$1"
    local chart_name="$2"
    local new_chart="$2:$3"

    if echo "$original_body" | grep -q "$chart_name:"; then
        echo "$original_body" | sed "s|^- $chart_name:.*|- $new_chart|"
    else
        if [[ -z "$original_body" ]]; then
            echo "$new_chart"
        else
            printf "%s\n%s" "$original_body" "- $new_chart"
        fi
    fi
}

CURRENT_VERSION=$(yq -r '.charts[] | select(.filename == "/charts/'"${CHART_NAME}"'.yaml") | .version' ${CHART_VERSIONS_FILE})
if [ "${CURRENT_VERSION}" != "${CHART_VERSION}" ]; then
    check-issue
fi
