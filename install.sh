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

# Environment variables:
#
#   - INSTALL_RKE2_CHANNEL
#     Channel to use for fetching rke2 download URL.
#     Defaults to 'latest'.
#
#   - INSTALL_RKE2_METHOD
#     The installation method to use.
#     Default is on RPM-based systems is "rpm", all else "tar".
#
#   - INSTALL_RKE2_TYPE
#     Type of rke2 service. Can be either "server" or "agent".
#     Default is "server".
#
#   - INSTALL_RKE2_EXEC
#     This is an alias for INSTALL_RKE2_TYPE, included for compatibility with K3s.
#     If both are set, INSTALL_RKE2_TYPE is preferred.
#
#   - INSTALL_RKE2_VERSION
#     Version of rke2 to download from github.
#
#   - INSTALL_RKE2_RPM_RELEASE_VERSION
#     Version of the rke2 RPM release to install.
#     Format would be like "1.el7" or "2.el8"
#
#   - INSTALL_RKE2_TAR_PREFIX
#     Installation prefix when using the tar installation method.
#     Default is /usr/local, unless /usr/local is read-only or has a dedicated mount point,
#     in which case /opt/rke2 is used instead.
#
#   - INSTALL_RKE2_COMMIT
#     Commit of RKE2 to download from temporary cloud storage.
#     If set, this forces INSTALL_RKE2_METHOD=tar.
#     * (for developer & QA use)
#
#   - INSTALL_RKE2_AGENT_IMAGES_DIR
#     Installation path for airgap images when installing from CI commit
#     Default is /var/lib/rancher/rke2/agent/images
#
#   - INSTALL_RKE2_ARTIFACT_PATH
#     If set, the install script will use the local path for sourcing the rke2.linux-$SUFFIX and sha256sum-$ARCH.txt files
#     rather than the downloading the files from the internet.
#     Default is not set.
#


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
        echo "[ALT] Please visit 'https://github.com/rancher/rke2/releases' directly and download the latest rke2.${SUFFIX}.tar.gz" >&2
    fi
    exit 1
}

# check_target_mountpoint return success if the target directory is on a dedicated mount point
check_target_mountpoint() {
    mountpoint -q "${INSTALL_RKE2_TAR_PREFIX}"
}

# check_target_ro returns success if the target directory is read-only
check_target_ro() {
    touch "${INSTALL_RKE2_TAR_PREFIX}"/.rke2-ro-test && rm -rf "${INSTALL_RKE2_TAR_PREFIX}"/.rke2-ro-test
    test $? -ne 0
}


# setup_env defines needed environment variables.
setup_env() {
    STORAGE_URL="https://storage.googleapis.com/rke2-ci-builds"
    INSTALL_RKE2_GITHUB_URL="https://github.com/rancher/rke2"
    DEFAULT_TAR_PREFIX="/usr/local"
    # --- bail if we are not root ---
    if [ ! $(id -u) -eq 0 ]; then
        fatal "You need to be root to perform this install"
    fi

    # --- make sure install channel has a value
    if [ -z "${INSTALL_RKE2_CHANNEL}" ]; then
        INSTALL_RKE2_CHANNEL="stable"
    fi

    # --- make sure install type has a value
    if [ -z "${INSTALL_RKE2_TYPE}" ]; then
        INSTALL_RKE2_TYPE="${INSTALL_RKE2_EXEC:-server}"
    fi

    # --- use yum install method if available by default
    if [ -z "${INSTALL_RKE2_ARTIFACT_PATH}" ] && [ -z "${INSTALL_RKE2_COMMIT}" ] && [ -z "${INSTALL_RKE2_METHOD}" ] && command -v yum >/dev/null 2>&1; then
        INSTALL_RKE2_METHOD="yum"
    fi

    # --- install tarball to /usr/local by default, except if /usr/local is on a separate partition or is read-only
    # --- in which case we go into /opt/rke2.
    if [ -z "${INSTALL_RKE2_TAR_PREFIX}" ]; then
        INSTALL_RKE2_TAR_PREFIX=${DEFAULT_TAR_PREFIX}
        if check_target_mountpoint || check_target_ro; then
            INSTALL_RKE2_TAR_PREFIX="/opt/rke2"
            warn "${DEFAULT_TAR_PREFIX} is read-only or a mount point; installing to ${INSTALL_RKE2_TAR_PREFIX}"
        fi
    fi

    if [ -z "${INSTALL_RKE2_AGENT_IMAGES_DIR}" ]; then
        INSTALL_RKE2_AGENT_IMAGES_DIR="/var/lib/rancher/rke2/agent/images"
    fi
}

# check_method_conflict will exit with an error if the user attempts to install
# via tar method on a host with existing RPMs.
check_method_conflict() {
    case ${INSTALL_RKE2_METHOD} in
    yum | rpm | dnf)
        return
        ;;
    *)
        if rpm -q rke2-common >/dev/null 2>&1; then
            fatal "Cannot perform ${INSTALL_RKE2_METHOD:-tar} install on host with existing RKE2 RPMs - please run rke2-uninstall.sh first"
        fi
        ;;
    esac
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
    s390x)
        ARCH=s390x
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
    TMP_TARBALL=${TMP_DIR}/rke2.tarball
    TMP_AIRGAP_CHECKSUMS=${TMP_DIR}/rke2-images.checksums
    TMP_AIRGAP_TARBALL=${TMP_DIR}/rke2-images.tarball
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
        INSTALL_RKE2_CHANNEL_URL=${INSTALL_RKE2_CHANNEL_URL:-'https://update.rke2.io/v1-release/channels'}
        version_url="${INSTALL_RKE2_CHANNEL_URL}/${INSTALL_RKE2_CHANNEL}"
        case ${DOWNLOADER} in
        *curl)
            version=$(${DOWNLOADER} -w "%{url_effective}" -L -s -S "${version_url}" -o /dev/null | sed -e 's|.*/||')
            ;;
        *wget)
            version=$(${DOWNLOADER} -SqO /dev/null "${version_url}" 2>&1 | grep -i Location | sed -e 's|.*/||')
            ;;
        *)
            fatal "Unsupported downloader executable '${DOWNLOADER}'"
            ;;
        esac
        INSTALL_RKE2_VERSION="${version}"
    fi
}

# check_download performs a HEAD request to see if a file exists at a given url
check_download() {
    case ${DOWNLOADER} in
    *curl)
        curl -o "/dev/null" -fsLI -X HEAD "$1"
        ;;
    *wget)
        wget -q --spider "$1"
        ;;
    *)
        fatal "downloader executable not supported: '${DOWNLOADER}'"
        ;;
    esac
}

# download downloads a file from a url using either curl or wget
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
        CHECKSUMS_URL=${STORAGE_URL}/rke2.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.gz.sha256sum
    else
        CHECKSUMS_URL=${INSTALL_RKE2_GITHUB_URL}/releases/download/${INSTALL_RKE2_VERSION}/sha256sum-${ARCH}.txt
    fi
    info "downloading checksums at ${CHECKSUMS_URL}"
    download "${TMP_CHECKSUMS}" "${CHECKSUMS_URL}"
    CHECKSUM_EXPECTED=$(grep "rke2.${SUFFIX}.tar.gz" "${TMP_CHECKSUMS}" | awk '{print $1}')
}

# download_tarball downloads binary from github url.
download_tarball() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        TARBALL_URL=${STORAGE_URL}/rke2.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.gz
    else
        TARBALL_URL=${INSTALL_RKE2_GITHUB_URL}/releases/download/${INSTALL_RKE2_VERSION}/rke2.${SUFFIX}.tar.gz
    fi
    info "downloading tarball at ${TARBALL_URL}"
    download "${TMP_TARBALL}" "${TARBALL_URL}"
}

# stage_local_checksums stages the local checksum hash for validation.
stage_local_checksums() {
    info "staging local checksums from ${INSTALL_RKE2_ARTIFACT_PATH}/sha256sum-${ARCH}.txt"
    cp -f "${INSTALL_RKE2_ARTIFACT_PATH}/sha256sum-${ARCH}.txt" "${TMP_CHECKSUMS}"
    CHECKSUM_EXPECTED=$(grep "rke2.${SUFFIX}.tar.gz" "${TMP_CHECKSUMS}" | awk '{print $1}')
    if [ -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.zst" ]; then
        AIRGAP_CHECKSUM_EXPECTED=$(grep "rke2-images.${SUFFIX}.tar.zst" "${TMP_CHECKSUMS}" | awk '{print $1}')
    elif [ -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.gz" ]; then
        AIRGAP_CHECKSUM_EXPECTED=$(grep "rke2-images.${SUFFIX}.tar.gz" "${TMP_CHECKSUMS}" | awk '{print $1}')
    fi
}

# stage_local_tarball stages the local tarball.
stage_local_tarball() {
    info "staging tarball from ${INSTALL_RKE2_ARTIFACT_PATH}/rke2.${SUFFIX}.tar.gz"
    cp -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2.${SUFFIX}.tar.gz" "${TMP_TARBALL}"
}

# stage_local_airgap_tarball stages the local checksum hash for validation.
stage_local_airgap_tarball() {
    if [ -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.zst" ]; then
        info "staging zst airgap image tarball from ${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.zst"
        cp -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.zst" "${TMP_AIRGAP_TARBALL}"
        AIRGAP_TARBALL_FORMAT=zst
    elif [ -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.gz" ]; then
        info "staging gzip airgap image tarball from ${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.gz"
        cp -f "${INSTALL_RKE2_ARTIFACT_PATH}/rke2-images.${SUFFIX}.tar.gz" "${TMP_AIRGAP_TARBALL}"
        AIRGAP_TARBALL_FORMAT=gz
    fi
}

# verify_tarball verifies the downloaded installer checksum.
verify_tarball() {
    info "verifying tarball"
    CHECKSUM_ACTUAL=$(sha256sum "${TMP_TARBALL}" | awk '{print $1}')
    if [ "${CHECKSUM_EXPECTED}" != "${CHECKSUM_ACTUAL}" ]; then
        fatal "download sha256 does not match ${CHECKSUM_EXPECTED}, got ${CHECKSUM_ACTUAL}"
    fi
}

# unpack_tarball extracts the tarball, correcting paths and moving systemd units as necessary
unpack_tarball() {
    info "unpacking tarball file to ${INSTALL_RKE2_TAR_PREFIX}"
    mkdir -p ${INSTALL_RKE2_TAR_PREFIX}
    tar xzf "${TMP_TARBALL}" -C "${INSTALL_RKE2_TAR_PREFIX}"
    if [ "${INSTALL_RKE2_TAR_PREFIX}" != "${DEFAULT_TAR_PREFIX}" ]; then
        info "updating tarball contents to reflect install path"
        sed -i "s|${DEFAULT_TAR_PREFIX}|${INSTALL_RKE2_TAR_PREFIX}|" ${INSTALL_RKE2_TAR_PREFIX}/lib/systemd/system/rke2-*.service ${INSTALL_RKE2_TAR_PREFIX}/bin/rke2-uninstall.sh
        info "moving systemd units to /etc/systemd/system"
        mv -f ${INSTALL_RKE2_TAR_PREFIX}/lib/systemd/system/rke2-*.service /etc/systemd/system/
        info "install complete; you may want to run:  export PATH=\$PATH:${INSTALL_RKE2_TAR_PREFIX}/bin"
    fi
}

# download_airgap_checksums downloads the checksum file for the airgap image tarball
# and prepares the checksum value for later validation.
download_airgap_checksums() {
    if [ -z "${INSTALL_RKE2_COMMIT}" ]; then
        return
    fi
    AIRGAP_CHECKSUMS_URL=${STORAGE_URL}/rke2-images.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.zst.sha256sum
    # try for zst first; if that fails use gz for older release branches
    if ! check_download "${AIRGAP_CHECKSUMS_URL}"; then
        AIRGAP_CHECKSUMS_URL=${STORAGE_URL}/rke2-images.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.gz.sha256sum
    fi
    info "downloading airgap checksums at ${AIRGAP_CHECKSUMS_URL}"
    download "${TMP_AIRGAP_CHECKSUMS}" "${AIRGAP_CHECKSUMS_URL}"
    AIRGAP_CHECKSUM_EXPECTED=$(grep "rke2-images.${SUFFIX}.tar" "${TMP_AIRGAP_CHECKSUMS}" | awk '{print $1}')
}

# download_airgap_tarball downloads the airgap image tarball.
download_airgap_tarball() {
    if [ -z "${INSTALL_RKE2_COMMIT}" ]; then
        return
    fi
    AIRGAP_TARBALL_URL=${STORAGE_URL}/rke2-images.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.zst
    # try for zst first; if that fails use gz for older release branches
    if ! check_download "${AIRGAP_TARBALL_URL}"; then
        AIRGAP_TARBALL_URL=${STORAGE_URL}/rke2-images.${SUFFIX}-${INSTALL_RKE2_COMMIT}.tar.gz
    fi
    info "downloading airgap tarball at ${AIRGAP_TARBALL_URL}"
    download "${TMP_AIRGAP_TARBALL}" "${AIRGAP_TARBALL_URL}"
}

# verify_airgap_tarball compares the airgap image tarball checksum to the value
# calculated by CI when the file was uploaded.
verify_airgap_tarball() {
    if [ -z "${AIRGAP_CHECKSUM_EXPECTED}" ]; then
        return
    fi
    info "verifying airgap tarball"
    AIRGAP_CHECKSUM_ACTUAL=$(sha256sum "${TMP_AIRGAP_TARBALL}" | awk '{print $1}')
    if [ "${AIRGAP_CHECKSUM_EXPECTED}" != "${AIRGAP_CHECKSUM_ACTUAL}" ]; then
        fatal "download sha256 does not match ${AIRGAP_CHECKSUM_EXPECTED}, got ${AIRGAP_CHECKSUM_ACTUAL}"
    fi
}

# install_airgap_tarball moves the airgap image tarball into place.
install_airgap_tarball() {
    if [ -z "${AIRGAP_CHECKSUM_EXPECTED}" ]; then
        return
    fi
    mkdir -p "${INSTALL_RKE2_AGENT_IMAGES_DIR}"
    # releases that provide zst artifacts can read from the compressed archive; older releases
    # that produce only gzip artifacts need to have the tarball decompressed ahead of time
    if grep -qF '.tar.zst' "${TMP_AIRGAP_CHECKSUMS}" || [ "${AIRGAP_TARBALL_FORMAT}" = "zst" ]; then
        info "installing airgap tarball to ${INSTALL_RKE2_AGENT_IMAGES_DIR}"
        mv -f "${TMP_AIRGAP_TARBALL}" "${INSTALL_RKE2_AGENT_IMAGES_DIR}/rke2-images.${SUFFIX}.tar.zst"
    else
        info "decompressing airgap tarball to ${INSTALL_RKE2_AGENT_IMAGES_DIR}"
        gzip -dc "${TMP_AIRGAP_TARBALL}" > "${INSTALL_RKE2_AGENT_IMAGES_DIR}/rke2-images.${SUFFIX}.tar"
    fi
}

# do_install_rpm builds a yum repo config from the channel and version to be installed,
# and calls yum to install the required packates.
do_install_rpm() {
    maj_ver="7"
    if [ -r /etc/redhat-release ] || [ -r /etc/centos-release ] || [ -r /etc/oracle-release ]; then
        dist_version="$(. /etc/os-release && echo "$VERSION_ID")"
        maj_ver=$(echo "$dist_version" | sed -E -e "s/^([0-9]+)\.?[0-9]*$/\1/")
        case ${maj_ver} in
            7|8)
                :
                ;;
            *) # In certain cases, like installing on Fedora, maj_ver will end up being something that is not 7 or 8
                maj_ver="7"
                ;;
        esac
    fi
    case "${INSTALL_RKE2_CHANNEL}" in
        v*.*)
            # We are operating with a version-based channel, so we should parse our version out
            rke2_majmin=$(echo "${INSTALL_RKE2_CHANNEL}" | sed -E -e "s/^v([0-9]+\.[0-9]+).*/\1/")
            rke2_rpm_channel=$(echo "${INSTALL_RKE2_CHANNEL}" | sed -E -e "s/^v[0-9]+\.[0-9]+-(.*)/\1/")
            # If our regex fails to capture a "sane" channel out of the specified channel, fall back to `stable`
            if [ "${rke2_rpm_channel}" = ${INSTALL_RKE2_CHANNEL} ]; then
                info "using stable RPM repositories"
                rke2_rpm_channel="stable"
            fi
            ;;
        *)
            get_release_version
            rke2_majmin=$(echo "${INSTALL_RKE2_VERSION}" | sed -E -e "s/^v([0-9]+\.[0-9]+).*/\1/")
            rke2_rpm_channel=${1}
            ;;
    esac
    info "using ${rke2_majmin} series from channel ${rke2_rpm_channel}"
    rpm_site="rpm.rancher.io"
    if [ "${rke2_rpm_channel}" = "testing" ]; then
        rpm_site="rpm-${rke2_rpm_channel}.rancher.io"
    fi
    rm -f /etc/yum.repos.d/rancher-rke2*.repo
    cat <<-EOF >"/etc/yum.repos.d/rancher-rke2.repo"
[rancher-rke2-common-${rke2_rpm_channel}]
name=Rancher RKE2 Common (${1})
baseurl=https://${rpm_site}/rke2/${rke2_rpm_channel}/common/centos/${maj_ver}/noarch
enabled=1
gpgcheck=1
gpgkey=https://${rpm_site}/public.key
[rancher-rke2-${rke2_majmin}-${rke2_rpm_channel}]
name=Rancher RKE2 ${rke2_majmin} (${1})
baseurl=https://${rpm_site}/rke2/${rke2_rpm_channel}/${rke2_majmin}/centos/${maj_ver}/x86_64
enabled=1
gpgcheck=1
gpgkey=https://${rpm_site}/public.key
EOF
    if [ -z "${INSTALL_RKE2_VERSION}" ]; then
        yum -y install "rke2-${INSTALL_RKE2_TYPE}"
    else
        rke2_rpm_version=$(echo "${INSTALL_RKE2_VERSION}" | sed -E -e "s/[\+-]/~/g" | sed -E -e "s/v(.*)/\1/")
        if [ -n "${INSTALL_RKE2_RPM_RELEASE_VERSION}" ]; then
            yum -y install "rke2-${INSTALL_RKE2_TYPE}-${rke2_rpm_version}-${INSTALL_RKE2_RPM_RELEASE_VERSION}"
        else
            yum -y install "rke2-${INSTALL_RKE2_TYPE}-${rke2_rpm_version}"
        fi
    fi
}

do_install_tar() {
    setup_tmp

    if [ -n "${INSTALL_RKE2_ARTIFACT_PATH}" ]; then
        stage_local_checksums
        stage_local_airgap_tarball
        stage_local_tarball
    else
        get_release_version
        info "using ${INSTALL_RKE2_VERSION:-commit $INSTALL_RKE2_COMMIT} as release"
        download_airgap_checksums
        download_airgap_tarball
        download_checksums
        download_tarball
    fi

    verify_airgap_tarball
    install_airgap_tarball
    verify_tarball
    unpack_tarball
    systemctl daemon-reload
}

do_install() {
    setup_env
    check_method_conflict
    setup_arch
    if [ -z "${INSTALL_RKE2_ARTIFACT_PATH}" ]; then
        verify_downloader curl || verify_downloader wget || fatal "can not find curl or wget for downloading files"
    fi

    case ${INSTALL_RKE2_METHOD} in
    yum | rpm | dnf)
        do_install_rpm "${INSTALL_RKE2_CHANNEL}"
        ;;
    *)
        do_install_tar "${INSTALL_RKE2_CHANNEL}"
        ;;
    esac
}

do_install
exit 0
