#!/bin/bash

set -euo pipefail

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

resolve_chart_version() {
    local chart_name="$1"
    local chart_version="${2:-}"

    if [[ -n "${chart_version}" ]]; then
        echo "${chart_version}"
        return 0
    fi

    echo "[INFO]  no chart version provided for ${chart_name}; resolving latest from chart index" >&2
    chart_version=$(bash ./updatecli/scripts/retrieve_chart_version.sh "${chart_name}")
    if [[ -z "${chart_version}" ]]; then
        fatal "failed to resolve chart version for ${chart_name}"
    fi

    echo "${chart_version}"
}

update_chart_version() {
    info "updating chart ${1} in ${CHART_VERSIONS_FILE}"
    export CHART_TARGET="/charts/${1}.yaml"
    CURRENT_VERSION=$(yq -r '.charts[] | select(.filename == strenv(CHART_TARGET)) | .version' "${CHART_VERSIONS_FILE}")
    NEW_VERSION=${2}
    if [ "${CURRENT_VERSION}" != "${NEW_VERSION}" ]; then
        info "found version ${CURRENT_VERSION}, updating to ${NEW_VERSION}"
        if test "$DRY_RUN" == "false"; then
            sed -i "s/${CURRENT_VERSION}/${NEW_VERSION}/g" ${CHART_VERSIONS_FILE}
        else
            info "dry-run is enabled, no changes will occur"
        fi
    else
        info "no new version found"
    fi
}

update_chart_prime_images() {
    local chart_name="${1}"
    local chart_version="${2}"
    local kubernetes_minor="${3:-}"
    local constraint=""
    CHART_TEMP_DIR=$(mktemp -d)

    if [[ -z "${kubernetes_minor}" ]]; then
        info "using default PRIME image values for ${chart_name}"
    fi
    if [[ -n "${kubernetes_minor}" ]]; then
        constraint="~ ${kubernetes_minor}"
        export constraint
    fi

    info "downloading chart ${chart_name} version ${chart_version} to extract PRIME image versions for Kubernetes ${kubernetes_minor}"
    CHART_URL="https://github.com/rancher/rke2-charts/raw/main/assets/${chart_name}/${chart_name}-${chart_version}.tgz"
    if ! curl -sfL "${CHART_URL}" | tar xz -C "${CHART_TEMP_DIR}" "${chart_name}/values.yaml" 1> /dev/null; then
        fatal "failed to download or extract ${CHART_URL}"
    fi

    if [[ -n "${constraint}" ]]; then
        OVERRIDE_COUNT=$(yq -r '[.versionOverrides[] | select(.constraint == strenv(constraint))] | length' "${CHART_TEMP_DIR}/${chart_name}/values.yaml")
        if [[ "${OVERRIDE_COUNT}" != "1" ]]; then
            fatal "expected one version override with constraint '${constraint}', found ${OVERRIDE_COUNT} in ${chart_name}-${chart_version}"
        fi
    fi

    while IFS=$'\t' read -r image tag; do
        if [[ -z "${image}" || -z "${tag}" ]]; then
            continue
        fi

        image_regex=$(printf '%s' "${image}" | sed 's/[][(){}.^$*+?|\\]/\\&/g')
        target_image=$(grep -E "^[[:space:]]*\\$\\{REGISTRY\\}/${image_regex}:" "${CHART_AIRGAP_IMAGES_FILE}" | head -n 1 || true)
        if [ -z "${target_image}" ]; then
            warn "prime image ${image} not found in the airgap scripts, skipping"
            continue
        fi

        target_tag=${target_image##*:}
        if [ "${target_tag}" != "${tag}" ]; then
            info updating prime image ${image} in airgap script from version ${target_tag} to ${tag}
            if test "$DRY_RUN" == "false"; then
                sed -r -i "s~^([[:space:]]*\\$\\{REGISTRY\\}/${image_regex}:).*~\\1${tag}~" "${CHART_AIRGAP_IMAGES_FILE}"
            else
                info "dry-run is enabled, no changes will occur"
            fi
        else
            info "prime image ${image} did not update from version ${tag}"
        fi
    done < <(
        if [[ -n "${constraint}" ]]; then
            yq -r '
                (. * ((.versionOverrides[] | select(.constraint == strenv(constraint))).values // {}))
                | del(.versionOverrides)
                | ..
                | select(tag == "!!map")
                | select(has("primeRepository") and has("primeTag"))
                | select(.primeTag != "latest")
                | [.primeRepository, .primeTag]
                | @tsv
            ' "${CHART_TEMP_DIR}/${chart_name}/values.yaml"
        else
            yq -r '
                del(.versionOverrides)
                | ..
                | select(tag == "!!map")
                | select(has("primeRepository") and has("primeTag"))
                | select(.primeTag != "latest")
                | [.primeRepository, .primeTag]
                | @tsv
            ' "${CHART_TEMP_DIR}/${chart_name}/values.yaml"
        fi | sort -u
    )
}

CHART_VERSIONS_FILE="charts/chart_versions.yaml"
CHART_AIRGAP_IMAGES_FILE="scripts/build-images"
CHART_TEMP_DIR=

cleanup_chart_temp_dir() {
    if [[ -n "${CHART_TEMP_DIR}" ]]; then
        rm -rf -- "${CHART_TEMP_DIR}"
    fi
}
trap cleanup_chart_temp_dir EXIT


: "${DRY_RUN:=false}"

CHART_NAME=${1}
CHART_VERSION=$(resolve_chart_version "${CHART_NAME}" "${2:-}")

update_chart_version ${CHART_NAME} ${CHART_VERSION}
update_chart_prime_images ${CHART_NAME} ${CHART_VERSION}
