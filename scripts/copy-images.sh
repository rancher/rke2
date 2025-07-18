#!/bin/sh
set -e

TARGET_REGISTRY=''
IMAGE_LIST=''
DRY_RUN=''

has_crane() {
    CRANE="$(command -v crane || true)"
    if [ -z "${CRANE}" ]; then
        echo "crane is not installed"
        exit 1
    fi
}

usage() {
    echo "Syncs images to a registry.
    usage: $0 [options]
    -t     target registry
    -i     image list file path
    -d     dry run
    -h     show help

list format:
    [REGISTRY]/[REPOSITORY]:[TAG]

examples:
    $0 -t registry.example.com -i build/images-all.txt
    $0 -d -t registry.example.com -i build/images-all.txt"
}

while getopts 't:i:dh' c; do
    case $c in
        t)
            TARGET_REGISTRY=$OPTARG
            ;;
        i)
            IMAGE_LIST=$OPTARG
            ;;
        d)
            DRY_RUN=true
            ;;
        h)
            usage
            exit 0
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if [ -z "${TARGET_REGISTRY}" ]; then
    echo "target registry is required"
    usage
    exit 1
fi

if [ -z "${IMAGE_LIST}" ]; then
    echo "image list file is required"
    usage
    exit 1
fi

if [ ! -f "${IMAGE_LIST}" ]; then
    echo "image listfile ${IMAGE_LIST} not found"
    exit 1
fi

has_crane

if [ -n "${DRY_RUN}" ]; then 
    echo "Dry run, no images will be copied"
fi

while read -r source_image; do
    if [ -z "${source_image}" ]; then
        continue
    fi

    image_without_registry=$(echo "${source_image}" | cut -d'/' -f2-)
    target_image="${TARGET_REGISTRY}/${image_without_registry}"

    if [ -n "${DRY_RUN}" ]; then
        echo "crane copy \"${source_image}\" \"${target_image}\" --no-clobber"
    else
        if ! crane copy "${source_image}" "${target_image}" --no-clobber; then
            echo "failed to copy ${source_image}"
            continue
        fi
    fi
done < "${IMAGE_LIST}"
