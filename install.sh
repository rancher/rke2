#!/bin/sh

set -e

if [ "${DEBUG}" = 1 ]; then
    set -x
fi

# Usage:
#   curl ... | ENV_VAR=... sh -
#       or
#   ENV_VAR=... ./install.sh
#

: "${INSTALL_RKE2_GITHUB_URL:="https://github.com/rancher/rke2"}"

# info logs the given argument at info log level.
info() {
    echo "[INFO] " "$@"
}

# warn logs the given argument at warn log level.
warn() {
    echo "[WARN] " "$@" >&2
}

# fatal logs the given argument at fatal log level.
fatal() {
    echo "[ERROR] " "$@" >&2
    if [ -n "${SUFFIX}" ]; then
        echo "[ALT] Please visit 'https://github.com/rancher/rke2/releases' directly and download the latest rke2-installer.${SUFFIX}.run" >&2
    fi
    exit 1
}

# setup_env defines needed environment variables.
setup_env() {
    # --- bail if we are not root ---
    if [ ! $(id -u) -eq 0 ]; then
        fatal "You need to be root to perform this install"
    fi

    # --- determine if we are installing an agent or a server ---
    if [ -z "${INSTALL_RKE2_TYPE}" ]; then
        if [ -z "${RKE2_URL}" ]; then
            INSTALL_RKE2_TYPE="server"
        else
            INSTALL_RKE2_TYPE="agent"
        fi
    fi

    # --- check for invalid characters in system name ---
    valid_chars=$(printf '%s' "${INSTALL_RKE2_NAME}" | sed -e 's/[][!#$%&()*;<=>?\_`{|}/[:space:]]/^/g;')
    if [ "${INSTALL_RKE2_NAME}" != "${valid_chars}" ]; then
        invalid_chars=$(printf '%s' "${valid_chars}" | sed -e 's/[^^]/ /g')
        fatal "invalid characters for system name:
            ${INSTALL_RKE2_NAME}
            ${invalid_chars}"
    fi

    # --- use yum install method if available
    if [ -z "${INSTALL_RKE2_METHOD}" ] && command -v yum >/dev/null 2>&1; then
        INSTALL_RKE2_METHOD=yum
    fi
}

# setup_arch set arch and suffix,
# fatal if architecture not supported.
setup_arch() {
    case ${ARCH:=$(uname -m)} in
    amd64)
        ARCH=amd64
        SUFFIX=$(uname -s | tr '[:upper:]' '[:lower:]')-${ARCH}
        ;;
    x86_64)
        ARCH=amd64
        SUFFIX=$(uname -s | tr '[:upper:]' '[:lower:]')-${ARCH}
        ;;
    *)
        fatal "unsupported architecture ${ARCH}"
        ;;
    esac
}

# verify_downloader verifies existence of
# network downloader executable.
verify_downloader() {
    cmd="$(command -v "${1}")"
    if [ -z "${cmd}" ]; then
        return 1
    fi
    if [ ! -x "${cmd}" ]; then
        return 1
    fi

    # Set verified executable as our downloader program and return success
    DOWNLOADER=${cmd}
    return 0
}

# setup_tmp creates a temporary directory
# and cleans up when done.
setup_tmp() {
    TMP_DIR=$(mktemp -d -t rke2-install.XXXXXXXXXX)
    TMP_CHECKSUMS=${TMP_DIR}/rke2.checksums
    TMP_INSTALLER=${TMP_DIR}/rke2.installer
    cleanup() {
        code=$?
        set +e
        trap - EXIT
        rm -rf "${TMP_DIR}"
        exit $code
    }
    trap cleanup INT EXIT
}

# --- use desired rke2 version if defined or find version from channel ---
get_release_version() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        version="commit ${INSTALL_RKE2_COMMIT}"
    elif [ -n "${INSTALL_RKE2_VERSION}" ]; then
        version=${INSTALL_RKE2_VERSION}
    else
        info "finding release for channel ${INSTALL_RKE2_CHANNEL}"
        # version_url="${INSTALL_RKE2_CHANNEL_URL}/${INSTALL_RKE2_CHANNEL}"
        version_url="${INSTALL_RKE2_GITHUB_URL}/releases/latest"
        case ${DOWNLOADER} in
        *curl)
            version=$(${DOWNLOADER} -w "%{url_effective}" -L -s -S ${version_url} -o /dev/null | sed -e 's|.*/||')
            ;;
        *wget)
            version=$(${DOWNLOADER} -SqO /dev/null ${version_url} 2>&1 | grep -i Location | sed -e 's|.*/||')
            ;;
        *)
            fatal "Unsupported downloader executable '${DOWNLOADER}'"
            ;;
        esac
        INSTALL_RKE2_VERSION="${version}"
    fi
    info "using ${INSTALL_RKE2_VERSION} as release"
}

# download downloads from github url.
download() {
    if [ $# -ne 2 ]; then
        fatal "download needs exactly 2 arguments"
    fi

    case ${DOWNLOADER} in
    *curl)
        curl -o "$1" -fsSL "$2"
        ;;
    *wget)
        wget -qO "$1" "$2"
        ;;
    *)
        fatal "downloader executable not supported: '${DOWNLOADER}'"
        ;;
    esac

    # Abort if download command failed
    if [ $? -ne 0 ]; then
        fatal "download failed"
    fi
}

# download_checksums downloads hash from github url.
download_checksums() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        fatal "downloading by commit is currently not supported"
        # CHECKSUMS_URL=${STORAGE_URL}/rke2${SUFFIX}-${INSTALL_RKE2_COMMIT}.sha256sum
    else
        CHECKSUMS_URL=${INSTALL_RKE2_GITHUB_URL}/releases/download/${INSTALL_RKE2_VERSION}/sha256sum-${ARCH}.txt
    fi
    info "downloading checksums at ${CHECKSUMS_URL}"
    download "${TMP_CHECKSUMS}" "${CHECKSUMS_URL}"
    CHECKSUM_EXPECTED=$(grep "rke2-installer.${SUFFIX}.run" "${TMP_CHECKSUMS}" | awk '{print $1}')
}

# download_installer downloads binary from github url.
download_installer() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        fatal "downloading by commit is currently not supported"
        # INSTALLER_URL=${STORAGE_URL}/rke2-installer.${SUFFIX}-${INSTALL_RKE2_COMMIT}.run
    else
        INSTALLER_URL=${INSTALL_RKE2_GITHUB_URL}/releases/download/${INSTALL_RKE2_VERSION}/rke2-installer.${SUFFIX}.run
    fi
    info "downloading installer at ${INSTALLER_URL}"
    download "${TMP_INSTALLER}" "${INSTALLER_URL}"
}

# verify_installer verifies the downloaded installer checksum.
verify_installer() {
    info "verifying installer"
    CHECKSUM_ACTUAL=$(sha256sum "${TMP_INSTALLER}" | awk '{print $1}')
    if [ "${CHECKSUM_EXPECTED}" != "${CHECKSUM_ACTUAL}" ]; then
        fatal "download sha256 does not match ${CHECKSUM_EXPECTED}, got ${CHECKSUM_ACTUAL}"
    fi
}

do_rpm() {
    cat <<-EOF > "/etc/yum.repos.d/rancher-rke2-${1}.repo"
[rancher-rke2-common-${1}]
name=Rancher RKE2 Common (${1})
baseurl=https://rpm-${1}.rancher.io/rke2/${1}/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm-${1}.rancher.io/public.key
[rancher-rke2-1-18-${1}]
name=Rancher RKE2 1.18 (${1})
baseurl=https://rpm-${1}.rancher.io/rke2/${1}/1.18/centos/7/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm-${1}.rancher.io/public.key
EOF
    yum -y install "rke2-${INSTALL_RKE2_TYPE}"
}

do_installer() {
    verify_downloader curl || verify_downloader wget || fatal "can not find curl or wget for downloading files"
    setup_tmp
    get_release_version
    download_checksums
    download_installer
    verify_installer
    sh "${TMP_INSTALLER}"
}

do_install() {
    setup_env
    setup_arch

    case ${INSTALL_RKE2_METHOD-"installer"} in
    yum | rpm | dnf)
        do_rpm "${INSTALL_RKE2_CHANNEL-"testing"}"
        ;;
    installer)
        do_installer "${INSTALL_RKE2_CHANNEL-"testing"}"
        ;;
    *)
        fatal "unknown installation method: ${INSTALL_RKE2_METHOD}"
        ;;
    esac
}

do_install
exit 0
