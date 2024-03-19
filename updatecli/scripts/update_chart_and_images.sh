#!/bin/bash

info()
{
    echo '[INFO] ' "$@"
}
warn()
{
    echo '[WARN] ' "$@" >&2
}
fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

update_chart_version() {
    info "updating chart ${1} in ${CHART_VERSIONS_FILE}"
    CURRENT_VERSION=$(yq -r '.charts[] | select(.filename == "/charts/'"${1}"'.yaml") | .version' ${CHART_VERSIONS_FILE})
    NEW_VERSION=${2}
    if [ "${CURRENT_VERSION}" != "${NEW_VERSION}" ]; then
        info "found version ${CURRENT_VERSION}, updating to ${NEW_VERSION}"
        chart_updated=true
        if test "$DRY_RUN" == "false"; then
            sed -i "s/${CURRENT_VERSION}/${NEW_VERSION}/g" ${CHART_VERSIONS_FILE}
        else
            info "dry-run is enabled, no changes will occur"
        fi
    else
        info "no new version found"
    fi
}

update_chart_images() {
    info "downloading chart ${1} version ${2} to extract image versions"
    CHART_URL="https://github.com/rancher/rke2-charts/raw/main/assets/${1}/${1}-${2}.tgz"
    curl -s -L ${CHART_URL} | tar xzv ${1}/values.yaml 1> /dev/null
    if test "$chart_updated" == "true"; then
        # get all images and tags for the latest constraint
        IMAGES_TAG=$(yq -y -r '.versionOverrides[] | select( .constraint == "~ 1.27" or .constraint == ">= 1.24 < 1.28") | .values' ${1}/values.yaml | grep -E "repo|tag")
        while IFS= read -r line ; do 
            if grep "repo" <<< ${line} &> /dev/null; then
              image=${line#*: }
              tag_line=$(echo "${IMAGES_TAG}" | grep -A1 ${image} 2>&1| sed -n '2 p' | tr -d " ")
              tag=${tag_line#*:}
              target_image=$(grep ${image} ${CHART_AIRGAP_IMAGES_FILE})
              if [ -z "${target_image}" ]; then
                fatal "image ${image} not found in the airgap scripts"
              fi
              target_tag=${target_image#*:}
              if [ "$target_tag" != "${tag}" ]; then
                info updating image ${image} in airgap script from version ${target_tag} to ${tag}
                if test "$DRY_RUN" == "false"; then
                    sed -r -i 's~(.*'${image}':).*~\1'${tag}'~g' ${CHART_AIRGAP_IMAGES_FILE}
                else
                    info "dry-run is enabled, no changes will occur"
                fi
              else
                info "image ${image} did not update from version ${tag}"
              fi
            else
              continue
            fi 
        done <<< "$IMAGES_TAG"
    else
        info "no new version found"
    fi
    # removing downloaded artifacts
    rm -rf ${1}/
}

CHART_VERSIONS_FILE="charts/chart_versions.yaml"
CHART_AIRGAP_IMAGES_FILE="scripts/build-images"


CHART_NAME=${1}
CHART_VERSION=${2}
chart_updated=false

update_chart_version ${CHART_NAME} ${CHART_VERSION}
update_chart_images ${CHART_NAME} ${CHART_VERSION}
