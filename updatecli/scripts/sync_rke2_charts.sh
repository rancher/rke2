#!/bin/bash

set -eu

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

# Files to update
CHART_VERSIONS_FILE="charts/chart_versions.yaml"
BUILD_IMAGES_FILE="scripts/build-images"
RKE2_CHARTS_URL="https://rke2-charts.rancher.io/index.yaml"
DRY_RUN="${DRY_RUN:-false}"

# List of charts to sync from rke2-charts
# These are the bootstrap charts that are regularly updated
CHARTS_TO_SYNC=(
    "rke2-cilium"
    "rke2-canal"
    "rke2-coredns"
    "rke2-metrics-server"
    "rke2-multus"
    "rke2-flannel"
)

# Function to get the latest version of a chart from rke2-charts
get_latest_chart_version() {
    local chart_name="${1}"
    local version=$(curl -sfL "${RKE2_CHARTS_URL}" | yq -r '.entries.'"${chart_name}"'[].version' | sort -rV | head -n 1)
    
    if [[ "${version}" = "null" ]] || [[ -z "${version}" ]]; then
        warn "failed to retrieve version for chart ${chart_name}"
        return 1
    fi
    
    echo "${version}"
}

# Function to update chart version in chart_versions.yaml
update_chart_version() {
    local chart_name="${1}"
    local new_version="${2}"
    local current_version=$(yq -r '.charts[] | select(.filename == "/charts/'"${chart_name}"'.yaml") | .version' ${CHART_VERSIONS_FILE})
    
    if [ -z "${current_version}" ] || [ "${current_version}" = "null" ]; then
        warn "chart ${chart_name} not found in ${CHART_VERSIONS_FILE}"
        return 1
    fi
    
    if [ "${current_version}" != "${new_version}" ]; then
        info "updating chart ${chart_name} from ${current_version} to ${new_version} in ${CHART_VERSIONS_FILE}"
        if [ "$DRY_RUN" = "false" ]; then
            # Use yq to update the version to avoid potential conflicts with similar version strings
            yq -i '.charts[] |= (select(.filename == "/charts/'"${chart_name}"'.yaml") | .version = "'"${new_version}"'")' ${CHART_VERSIONS_FILE}
        else
            info "dry-run mode: would update ${chart_name} to ${new_version}"
        fi
        return 0
    else
        info "chart ${chart_name} already at version ${new_version}"
        return 1
    fi
}

# Function to extract and update images from a chart
update_chart_images() {
    local chart_name="${1}"
    local chart_version="${2}"
    
    info "downloading chart ${chart_name} version ${chart_version} to extract image versions"
    local chart_url="https://github.com/rancher/rke2-charts/raw/main/assets/${chart_name}/${chart_name}-${chart_version}.tgz"
    
    # Download and extract chart  
    local temp_dir=$(mktemp -d)
    if ! curl -sfL "${chart_url}" | tar xz -C "${temp_dir}" 2>/dev/null; then
        warn "failed to download or extract chart ${chart_name}-${chart_version} from ${chart_url}"
        rm -rf "${temp_dir}"
        return 1
    fi
    
    # Check if values.yaml exists
    if [ ! -f "${temp_dir}/${chart_name}/values.yaml" ]; then
        warn "values.yaml not found for chart ${chart_name}"
        rm -rf "${temp_dir}"
        return 1
    fi
    
    # Get images and tags - similar to update_chart_and_images.sh
    # Try to extract from versionOverrides for latest Kubernetes version first
    local images_tag=$(yq -y -r '.versionOverrides[] | select(.constraint == "~ 1.27" or .constraint == ">= 1.24 < 1.28") | .values' "${temp_dir}/${chart_name}/values.yaml" 2>/dev/null | grep -E "repo|tag" || echo "")
    
    if [ -z "${images_tag}" ]; then
        warn "no version overrides found for chart ${chart_name}, trying default values"
        # Fall back to default values if no version overrides
        images_tag=$(yq -r '.' "${temp_dir}/${chart_name}/values.yaml" | grep -E "repo:|tag:" || echo "")
    fi
    
    if [ -z "${images_tag}" ]; then
        info "no images found in chart ${chart_name}"
        rm -rf "${temp_dir}"
        return 1
    fi
    
    # Process each repo/tag pair
    local updated=false
    while IFS= read -r line; do
        if grep "repo" <<< "${line}" &> /dev/null; then
            local image=${line#*: }
            # Get the corresponding tag line
            local tag_line=$(echo "${images_tag}" | grep -A1 "${image}" 2>&1 | sed -n '2 p' | tr -d " ")
            local tag=${tag_line#*:}
            
            if [ -n "${image}" ] && [ -n "${tag}" ] && [ "${tag}" != "${line}" ]; then
                # Check if this image exists in build-images
                if grep -q "${image}" "${BUILD_IMAGES_FILE}" 2>/dev/null; then
                    local target_image=$(grep "${image}" "${BUILD_IMAGES_FILE}" | head -n1)
                    local target_tag=${target_image#*:}
                    # Clean up potential trailing characters
                    target_tag=$(echo "${target_tag}" | awk '{print $1}')
                    
                    if [ "${target_tag}" != "${tag}" ]; then
                        info "updating image ${image} in ${BUILD_IMAGES_FILE} from ${target_tag} to ${tag}"
                        if [ "$DRY_RUN" = "false" ]; then
                            # Use more precise regex to replace only the tag part
                            sed -i -r 's~(.*'"${image}"':)'"${target_tag}"'(.*)~\1'"${tag}"'\2~g' "${BUILD_IMAGES_FILE}"
                        else
                            info "dry-run mode: would update ${image} to ${tag}"
                        fi
                        updated=true
                    else
                        info "image ${image} already at ${tag}"
                    fi
                fi
            fi
        fi
    done <<< "${images_tag}"
    
    rm -rf "${temp_dir}"
    
    if [ "${updated}" = "true" ]; then
        return 0
    else
        return 1
    fi
}

# Main logic: Update all charts and their images
main() {
    local any_updates=false
    
    info "Starting sync from rke2-charts repository..."
    
    for chart in "${CHARTS_TO_SYNC[@]}"; do
        info "Processing chart: ${chart}"
        local latest_version=$(get_latest_chart_version "${chart}")
        
        if [ -n "${latest_version}" ]; then
            # Try to update chart version
            if update_chart_version "${chart}" "${latest_version}"; then
                any_updates=true
                # Update images for this chart
                if update_chart_images "${chart}" "${latest_version}"; then
                    info "Successfully updated images for ${chart}"
                else
                    info "No image updates needed for ${chart}"
                fi
            fi
        else
            warn "Could not get latest version for ${chart}"
        fi
    done
    
    if [ "${any_updates}" = "false" ]; then
        info "No charts were updated - all charts are already at the latest version"
        exit 1
    else
        info "Successfully synchronized charts and images from rke2-charts"
        exit 0
    fi
}

main
