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
DOCKERFILE_WINDOWS="Dockerfile.windows"
RKE2_CHARTS_URL="https://rke2-charts.rancher.io/index.yaml"
DRY_RUN="${DRY_RUN:-false}"

# Kubernetes version constraints for extracting image versions
# These should be updated when adding support for new Kubernetes versions
K8S_VERSION_CONSTRAINTS=(
    ">= 1.31 < 1.36"
)

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
    # Use proper yq string interpolation to avoid injection issues
    local version=$(curl -sfL "${RKE2_CHARTS_URL}" | yq -r --arg chart "${chart_name}" '.entries[$chart][].version' | sort -rV | head -n 1)
    
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
    # Use proper yq string interpolation
    local current_version=$(yq -r --arg chart "${chart_name}" '.charts[] | select(.filename == "/charts/\($chart).yaml") | .version' ${CHART_VERSIONS_FILE})
    
    if [ -z "${current_version}" ] || [ "${current_version}" = "null" ]; then
        warn "chart ${chart_name} not found in ${CHART_VERSIONS_FILE}"
        return 1
    fi
    
    if [ "${current_version}" != "${new_version}" ]; then
        info "updating chart ${chart_name} from ${current_version} to ${new_version} in ${CHART_VERSIONS_FILE}"
        if [ "$DRY_RUN" = "false" ]; then
            # Use yq with proper string interpolation to avoid injection
            yq -i --arg chart "${chart_name}" --arg version "${new_version}" \
                '.charts[] |= (select(.filename == "/charts/\($chart).yaml") | .version = $version)' \
                ${CHART_VERSIONS_FILE}
        else
            info "dry-run mode: would update ${chart_name} to ${new_version}"
        fi
        return 0
    else
        info "chart ${chart_name} already at version ${new_version}"
        return 1
    fi
}

# Function to extract image repository and tag pairs from YAML
# Returns pairs as "repo|tag" one per line
extract_image_pairs_from_yaml() {
    local values_file="${1}"
    
    # Try to extract from versionOverrides first
    for constraint in "${K8S_VERSION_CONSTRAINTS[@]}"; do
        local override_values=$(yq -r --arg constraint "${constraint}" \
            '.versionOverrides[] | select(.constraint == $constraint) | .values' \
            "${values_file}" 2>/dev/null || echo "")
        
        if [ -n "${override_values}" ]; then
            # Found a matching constraint, extract image pairs using yq
            echo "${override_values}" | yq -r '
                .. | 
                select(type == "object" and has("repo") and has("tag")) |
                .repo + "|" + (.tag | tostring)
            ' 2>/dev/null || echo ""
            return 0
        fi
    done
    
    # If no version overrides found, try to extract from root
    yq -r '
        .. | 
        select(type == "object" and has("repo") and has("tag")) |
        .repo + "|" + (.tag | tostring)
    ' "${values_file}" 2>/dev/null || echo ""
}

# Function to escape special regex characters in a string
escape_regex() {
    local string="${1}"
    # Escape characters that have special meaning in regex
    echo "${string}" | sed 's/[]\/$*.^[]/\\&/g'
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
    
    # Extract image pairs using the safer yq-based approach
    local image_pairs=$(extract_image_pairs_from_yaml "${temp_dir}/${chart_name}/values.yaml")
    
    if [ -z "${image_pairs}" ]; then
        info "no images found in chart ${chart_name}"
        rm -rf "${temp_dir}"
        return 1
    fi
    
    # Process each image/tag pair
    local updated=false
    while IFS='|' read -r image tag; do
        if [ -n "${image}" ] && [ -n "${tag}" ]; then
            # Check if this image exists in build-images
            if grep -qF "${image}" "${BUILD_IMAGES_FILE}" 2>/dev/null; then
                # Extract current tag from build-images
                local target_line=$(grep -F "${image}" "${BUILD_IMAGES_FILE}" | head -n1)
                local target_tag=$(echo "${target_line}" | sed 's/.*://;s/[[:space:]]*$//' | awk '{print $1}')
                
                if [ "${target_tag}" != "${tag}" ]; then
                    info "updating image ${image} in ${BUILD_IMAGES_FILE} from ${target_tag} to ${tag}"
                    if [ "$DRY_RUN" = "false" ]; then
                        # Escape the image name for safe use in regex
                        local escaped_image=$(escape_regex "${image}")
                        local escaped_target_tag=$(escape_regex "${target_tag}")
                        # Replace the specific tag for this image
                        sed -i -r 's~(.*'"${escaped_image}"':)'"${escaped_target_tag}"'(.*)~\1'"${tag}"'\2~g' "${BUILD_IMAGES_FILE}"
                    else
                        info "dry-run mode: would update ${image} to ${tag}"
                    fi
                    updated=true
                else
                    info "image ${image} already at ${tag}"
                fi
            fi
        fi
    done <<< "${image_pairs}"
    
    rm -rf "${temp_dir}"
    
    if [ "${updated}" = "true" ]; then
        return 0
    else
        return 1
    fi
}

# Function to update Dockerfile.windows with CNI versions
# This updates the base versions (without build tags) used on Windows
update_dockerfile_windows() {
    info "Updating Dockerfile.windows with CNI versions from build-images"
    
    local updated=false
    
    # Extract base versions from build-images for CNI components
    # Map of ENV variable to image pattern in build-images
    declare -A version_map=(
        ["CALICO_VERSION"]="hardened-calico"
        ["CNI_PLUGIN_VERSION"]="hardened-cni-plugins"
        ["FLANNEL_VERSION"]="hardened-flannel"
    )
    
    for env_var in "${!version_map[@]}"; do
        local image_pattern="${version_map[$env_var]}"
        
        # Extract version from build-images (first occurrence)
        local full_version=$(grep -m1 "${image_pattern}:" "${BUILD_IMAGES_FILE}" | sed 's/.*://;s/-build.*//' | awk '{print $1}')
        
        if [ -z "${full_version}" ]; then
            warn "Could not find version for ${image_pattern} in ${BUILD_IMAGES_FILE}"
            continue
        fi
        
        # Get current version from Dockerfile.windows
        local current_version=$(grep "^ENV ${env_var}=" "${DOCKERFILE_WINDOWS}" | sed 's/.*="\(.*\)"/\1/')
        
        if [ -z "${current_version}" ]; then
            warn "Could not find ${env_var} in ${DOCKERFILE_WINDOWS}"
            continue
        fi
        
        if [ "${current_version}" != "${full_version}" ]; then
            info "Updating ${env_var} in ${DOCKERFILE_WINDOWS} from ${current_version} to ${full_version}"
            if [ "$DRY_RUN" = "false" ]; then
                # Update the ENV line in Dockerfile.windows
                sed -i "s/^ENV ${env_var}=\".*\"/ENV ${env_var}=\"${full_version}\"/" "${DOCKERFILE_WINDOWS}"
            else
                info "dry-run mode: would update ${env_var} to ${full_version}"
            fi
            updated=true
        else
            info "${env_var} in ${DOCKERFILE_WINDOWS} already at ${full_version}"
        fi
    done
    
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
    
    # After processing all charts, update Dockerfile.windows with CNI versions
    if update_dockerfile_windows; then
        info "Successfully updated Dockerfile.windows"
        any_updates=true
    else
        info "No Dockerfile.windows updates needed"
    fi
    
    if [ "${any_updates}" = "false" ]; then
        info "No charts were updated - all charts are already at the latest version"
        exit 1
    else
        info "Successfully synchronized charts and images from rke2-charts"
        exit 0
    fi
}

main
