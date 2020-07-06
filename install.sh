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
#   - RKE2_*
#     Environment variables which begin with RKE2_ will be preserved for the
#     systemd service to use. Setting RKE2_URL without explicitly setting
#     a systemd exec command will default the command to "agent", and we
#     enforce that RKE2_TOKEN or RKE2_CLUSTER_SECRET is also set.
#
#   - INSTALL_RKE2_SKIP_DOWNLOAD
#     If set to true will not download rke2 hash or binary.
#
#   - INSTALL_RKE2_SYMLINK
#     If set to 'skip' will not create symlinks, 'force' will overwrite,
#     default will symlink if command does not exist in path.
#
#   - INSTALL_RKE2_SKIP_ENABLE
#     If set to true will not enable or start rke2 service.
#
#   - INSTALL_RKE2_SKIP_START
#     If set to true will not start rke2 service.
#
#   - INSTALL_RKE2_VERSION
#     Version of rke2 to download from github. Will attempt to download from the
#     stable channel if not specified.
#
#   - INSTALL_RKE2_BIN_DIR
#     Directory to install rke2 binary, links, and uninstall script to, or use
#     /usr/local/bin as the default
#
#   - INSTALL_RKE2_BIN_DIR_READ_ONLY
#     If set to true will not write files to INSTALL_RKE2_BIN_DIR, forces
#     setting INSTALL_RKE2_SKIP_DOWNLOAD=true
#
#   - INSTALL_RKE2_SYSTEMD_DIR
#     Directory to install systemd service and environment files to, or use
#     /etc/systemd/system as the default
#
#   - INSTALL_RKE2_EXEC or script arguments
#     Command with flags to use for launching rke2 in the systemd service, if
#     the command is not specified will default to "agent" if RKE2_URL is set
#     or "server" if not. The final systemd command resolves to a combination
#     of EXEC and script args ($@).
#
#     The following commands result in the same behavior:
#       curl ... | INSTALL_RKE2_EXEC="--disable=traefik" sh -s -
#       curl ... | INSTALL_RKE2_EXEC="server --disable=traefik" sh -s -
#       curl ... | INSTALL_RKE2_EXEC="server" sh -s - --disable=traefik
#       curl ... | sh -s - server --disable=traefik
#       curl ... | sh -s - --disable=traefik
#
#   - INSTALL_RKE2_NAME
#     Name of systemd service to create, will default from the rke2 exec command
#     if not specified. If specified the name will be prefixed with 'rke2-'.
#
#   - INSTALL_RKE2_TYPE
#     Type of systemd service to create, will default from the rke2 exec command
#     if not specified.
#
#   - INSTALL_RKE2_SELINUX_WARN
#     If set to true will continue if rke2-selinux policy is not found.
#
#   - INSTALL_RKE2_ETCD_USER
#     Create a user 'etcd'. If this value is set, the installation
#     will chown the etcd data-dir to this user and update the etcd
#     pod manifest.
#
#   - INSTALL_RKE2_CIS_MODE
#     Enable all options to allow RKE2 to run in CIS mode if set to true. This 
#     will add an "etcd" system user and will update the following kernel 
#     parameters and set them to the necessary values:
#         vm.panic_on_oom=0
#         kernel.panic=10
#         kernel.panic_on_oops=1
#         kernel.keys.root_maxbytes=25000000

BASE_DIR="/var/lib/rancher/rke2"
INSTALL_PATH="/usr/local/bin"
GITHUB_URL=https://github.com/rancher/rke2/releases
STORAGE_URL=https://storage.googleapis.com/rke2-ci-builds
DOWNLOADER=

USING_RKE2_USER=0
USING_ETCD_USER=0

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
    exit 1
}

# verify_system checks for the existence of either
# systemd or openrc. If either of the two do not 
# exist, the script will log and error and exit. 
verify_system() {
    if [ -x /sbin/openrc-run ]; then
        HAS_OPENRC=true
        return
    fi
    if [ -d /run/systemd ]; then
        HAS_SYSTEMD=true
        return
    fi
    fatal "can not find systemd or openrc to use as a process supervisor for rke2"
}

# quote adds quotes to command arguments.
quote() {
    for arg in "$@"; do
        printf "%s\n" "$arg" | sed "s/'/'\\\\''/g;1s/^/'/;\$s/\$/'/"
    done
}

# quote_indent adds indentation and trailing slash 
# to quoted args.
quote_indent() {
    printf " \\\n"
    for arg in "$@"; do
        printf "\t%s \\\n" "$(quote "$arg")"
    done
}

# escape escapes most punctuation characters, except 
# quotes, forward slash, and space.
escape() {
    printf "%s" "$@" | sed -e 's/\([][!#$%&()*;<=>?\_`{|}]\)/\\\1/g;'
}

# escape_dq escapes double quotes.
escape_dq() {
    printf "%s" "$@" | sed -e 's/"/\\"/g'
}

# setup_env defines needed environment variables.
setup_env() {
    # --- use command args if passed or create default ---
    case "$1" in
        # --- if we only have flags discover if command should be server or agent ---
        (-*|"")
            if [ -z "${RKE2_URL}" ]; then
                CMD_RKE2=server
            else
                if [ -z "${RKE2_TOKEN}" ] && [ -z "${RKE2_CLUSTER_SECRET}" ]; then
                    fatal "defaulted rke2 exec command to 'agent' because RKE2_URL is defined, but RKE2_TOKEN or RKE2_CLUSTER_SECRET is not defined."
                fi
                CMD_RKE2=agent
            fi
        ;;
        # --- command is provided ---
        (*)
            CMD_RKE2=$1
            shift
        ;;
    esac
    if [ "${INSTALL_RKE2_CIS_MODE}" = true ]; then
        CMD_RKE2_EXEC=" --profile=cis-1.5 ${CMD_RKE2}$(quote_indent "$@")"
    else
        CMD_RKE2_EXEC="${CMD_RKE2}$(quote_indent "$@")"
    fi

    # --- use systemd name if defined or create default ---
    if [ -n "${INSTALL_RKE2_NAME}" ]; then
        SYSTEM_NAME=rke2-${INSTALL_RKE2_NAME}
    else
        if [ "${CMD_RKE2}" = server ]; then
            SYSTEM_NAME=rke2
        else
            SYSTEM_NAME=rke2-${CMD_RKE2}
        fi
    fi

    # --- check for invalid characters in system name ---
    valid_chars=$(printf "%s" "${SYSTEM_NAME}" | sed -e 's/[][!#$%&()*;<=>?\_`{|}/[:space:]]/^/g;' )
    if [ "${SYSTEM_NAME}" != "${valid_chars}"  ]; then
        invalid_chars=$(printf "%s" "${valid_chars}" | sed -e 's/[^^]/ /g')
        fatal "invalid characters for system name:
            ${SYSTEM_NAME}
            ${invalid_chars}"
    fi

    # --- use sudo if we are not already root ---
    SUDO=sudo
    if [ $(id -u) -eq 0 ]; then
        SUDO=
    fi

    # --- use systemd type if defined or create default ---
    if [ -n "${INSTALL_RKE2_TYPE}" ]; then
        SYSTEMD_TYPE=${INSTALL_RKE2_TYPE}
    else
        if [ "${CMD_RKE2}" = server ]; then
            SYSTEMD_TYPE=notify
        else
            SYSTEMD_TYPE=exec
        fi
    fi

    # --- use binary install directory if defined or create default ---
    if [ -n "${INSTALL_RKE2_BIN_DIR}" ]; then
        BIN_DIR=${INSTALL_RKE2_BIN_DIR}
    else
        BIN_DIR=/usr/local/bin
    fi

    # --- use systemd directory if defined or create default ---
    if [ -n "${INSTALL_RKE2_SYSTEMD_DIR}" ]; then
        SYSTEMD_DIR="${INSTALL_RKE2_SYSTEMD_DIR}"
    else
        SYSTEMD_DIR=/etc/systemd/system
    fi

    # --- set related files from system name ---
    SERVICE_RKE2=${SYSTEM_NAME}.service
    UNINSTALL_RKE2_SH=${UNINSTALL_RKE2_SH:-${BIN_DIR}/${SYSTEM_NAME}-uninstall.sh}
    KILLALL_RKE2_SH=${KILLALL_RKE2_SH:-${BIN_DIR}/rke2-killall.sh}

    # --- use service or environment location depending on systemd/openrc ---
    if [ "${HAS_SYSTEMD}" = true ]; then
        FILE_RKE2_SERVICE=${SYSTEMD_DIR}/${SERVICE_RKE2}
        FILE_RKE2_ENV=${SYSTEMD_DIR}/${SERVICE_RKE2}.env
    elif [ "${HAS_OPENRC}" = true ]; then
        ${SUDO} mkdir -p /etc/rancher/rke2
        FILE_RKE2_SERVICE=/etc/init.d/${SYSTEM_NAME}
        FILE_RKE2_ENV=/etc/rancher/rke2/${SYSTEM_NAME}.env
    fi

    # --- get hash of config & exec for currently installed rke2 ---
    PRE_INSTALL_HASHES=$(get_installed_hashes)

    # --- if bin directory is read only skip download ---
    if [ "${INSTALL_RKE2_BIN_DIR_READ_ONLY}" = true ]; then
        INSTALL_RKE2_SKIP_DOWNLOAD=true
    fi

    # --- setup channel values
    INSTALL_RKE2_CHANNEL_URL=${INSTALL_RKE2_CHANNEL_URL:-"https://update.rke2.io/v1-release/channels"}
    INSTALL_RKE2_CHANNEL=${INSTALL_RKE2_CHANNEL:-"stable"}
}

# can_skip_download checks if skip download 
# environment variable set.
can_skip_download() {
    if [ "${INSTALL_RKE2_SKIP_DOWNLOAD}" != true ]; then
        return 1
    fi
}

# verify_rke2_is_executable verify an executabe 
# rke2 binary is installed.
verify_rke2_is_executable() {
    if [ ! -x ${BIN_DIR}/rke2 ]; then
        fatal "executable rke2 binary not found at ${BIN_DIR}/rke2"
    fi
}

# setup_verify_arch set arch and suffix, 
# fatal if architecture not supported.
setup_verify_arch() {
    if [ -z "${ARCH}" ]; then
        ARCH=$(uname -m)
    fi
    case ${ARCH} in
        amd64)
            ARCH=amd64
            SUFFIX=
            ;;
        x86_64)
            ARCH=amd64
            SUFFIX=
            ;;
        arm64)
            ARCH=arm64
            SUFFIX=-${ARCH}
            ;;
        aarch64)
            ARCH=arm64
            SUFFIX=-${ARCH}
            ;;
        arm*)
            ARCH=arm
            SUFFIX=-${ARCH}hf
            ;;
        *)
            fatal "unsupported architecture ${ARCH}"
    esac
}

# verify_downloader verifies existence of
# network downloader executable.
verify_downloader() {
    cmd="$(which $1)"
    if [ -z ${cmd} ]; then
        return 1
    fi
    if [ ! -x ${cmd} ]; then
        return 1
    fi

    # Set verified executable as our downloader program and return success
    DOWNLOADER=${cmd}
    return 0
}

# setup_tmp creates a tempory directory 
# and cleans up when done.
setup_tmp() {
    TMP_DIR=$(mktemp -d -t rke2-install.XXXXXXXXXX)
    TMP_HASH=${TMP_DIR}/rke2.hash
    TMP_BIN=${TMP_DIR}/rke2.bin
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
        VERSION_RKE2="commit ${INSTALL_RKE2_COMMIT}"
    elif [ -n "${INSTALL_RKE2_VERSION}" ]; then
        VERSION_RKE2=${INSTALL_RKE2_VERSION}
    else
        info "finding release for channel ${INSTALL_RKE2_CHANNEL}"
        version_url="${INSTALL_RKE2_CHANNEL_URL}/${INSTALL_RKE2_CHANNEL}"
        case ${DOWNLOADER} in
            curl)
                VERSION_RKE2=$(curl -w "%{url_effective}" -L -s -S ${version_url} -o /dev/null | sed -e 's|.*/||')
                ;;
            wget)
                VERSION_RKE2=$(wget -SqO /dev/null ${version_url} 2>&1 | grep -i Location | sed -e 's|.*/||')
                ;;
            *)
                fatal "Incorrect downloader executable '${DOWNLOADER}'"
                ;;
        esac
    fi
    info "using ${VERSION_RKE2} as release"
}

# download downloads from github url.
download() {
    if [ $# -ne 2 ]; then
        fatal "download needs exactly 2 arguments"
    fi

    case ${DOWNLOADER} in
        *curl)
            curl -o "$1" -sfL "$2"
        ;;
        *wget)
            wget -qO "$1" "$2"
        ;;
        *)
            fatal "incorrect executable '${DOWNLOADER}'"
        ;;
    esac

    # Abort if download command failed
    if [ $? -ne 0 ]; then
        fatal "download failed"
    fi
}

# download_hash downloads hash from github url.
download_hash() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        HASH_URL=${STORAGE_URL}/rke2${SUFFIX}-${INSTALL_RKE2_COMMIT}.sha256sum
    else
        HASH_URL=${GITHUB_URL}/download/${VERSION_RKE2}/sha256sum-${ARCH}.txt
    fi
    info "downloading hash ${HASH_URL}"
    download "${TMP_HASH}" "${HASH_URL}"
    HASH_EXPECTED=$(awk -F ' ' '{print $1}' "${TMP_HASH}")
}

# installed_hash_matches checks hash against 
# installed version.
installed_hash_matches() {
    if [ -x ${BIN_DIR}/rke2 ]; then
        HASH_INSTALLED=$(sha256sum ${BIN_DIR}/rke2 | awk -F ' ' '{print $1}')
        if [ "${HASH_EXPECTED}" = "${HASH_INSTALLED}" ]; then
            return
        fi
    fi
    return 1
}

# download_binary downloads binary from github url.
download_binary() {
    if [ -n "${INSTALL_RKE2_COMMIT}" ]; then
        BIN_URL=${STORAGE_URL}/rke2${SUFFIX}-${INSTALL_RKE2_COMMIT}
    else
        BIN_URL=${GITHUB_URL}/download/${VERSION_RKE2}/rke2-${VERSION_RKE2}.linux-${ARCH}
    fi
    info "downloading binary at ${BIN_URL}"
    download "${TMP_BIN}" "${BIN_URL}"
}

# verify_binary verifies the downloaded 
# binary hash.
verify_binary() {
    info "verifying binary download"
    HASH_BIN=$(sha256sum "${TMP_BIN}" | awk -F ' ' '{print $1}')
    if [ "${HASH_EXPECTED}" != "${HASH_BIN}" ]; then
        fatal "download sha256 does not match ${HASH_EXPECTED}, got ${HASH_BIN}"
    fi
}

# setup_binary sets up permissions and moves 
# the binary to the system directory.
setup_binary() {
    chmod 755 "${TMP_BIN}"
    info "installing rke2 to ${BIN_DIR}/rke2"
    if [ ${USING_RKE2_USER} ]; then
        ${SUDO} chown "${INSTALL_RKE2_USER}":"${INSTALL_RKE2_USER}" "${TMP_BIN}"
    else
        ${SUDO} chown root:root "${TMP_BIN}"
    fi
    ${SUDO} mv -f "${TMP_BIN}" "${BIN_DIR}"/rke2
}

# setup_selinux sets up selinux policy.
setup_selinux() {
    policy_hint="please install:
    yum install -y container-selinux selinux-policy-base
    rpm -i https://rpm.rancher.io/rke2-selinux-0.1.1-rc1.el7.noarch.rpm
"
    policy_error=fatal
    if [ "${INSTALL_RKE2_SELINUX_WARN}" = true ]; then
        policy_error=warn
    fi

    if ! ${SUDO} chcon -u system_u -r object_r -t container_runtime_exec_t ${BIN_DIR}/rke2 >/dev/null 2>&1; then
        if ${SUDO} grep '^\s*SELINUX=enforcing' /etc/selinux/config >/dev/null 2>&1; then
            ${policy_error} "Failed to apply container_runtime_exec_t to ${BIN_DIR}/rke2, ${policy_hint}"
        fi
    else
        if [ ! -f /usr/share/selinux/packages/rke2.pp ]; then
            ${policy_error} "Failed to find the rke2-selinux policy, ${policy_hint}"
        fi
    fi
}

# download_and_verify downloads and verifies rke2.
download_and_verify() {
    if can_skip_download; then
       info "skipping rke2 download and verify"
       verify_rke2_is_executable
       return
    fi

    setup_verify_arch
    verify_downloader curl || verify_downloader wget || fatal "can not find curl or wget for downloading files"
    setup_tmp
    get_release_version
    download_hash

    if installed_hash_matches; then
        info "skipping binary download. installed rke2 matches hash"
        return
    fi

    download_binary
    verify_binary
    setup_binary
}

# create_symlinks adds additional utility links.
create_symlinks() {
    info "creating symlinks..."
    for bin in ${BASE_DIR}/data/*/bin/*; do
        ln -sf "${bin}" "${INSTALL_PATH}"/"$(basename ${bin})"
    done
}

# create_killall creates the killall script.
create_killall() {
    if [ "${INSTALL_RKE2_BIN_DIR_READ_ONLY}" = true ]; then
        return
    fi
    info "creating killall script ${KILLALL_RKE2_SH}"
    ${SUDO} tee "${KILLALL_RKE2_SH}" >/dev/null << \EOF
#!/bin/sh
[ $(id -u) -eq 0 ] || exec sudo $0 $@

for bin in ${BASE_DIR}/data/**/bin/; do
    [ -d $bin ] && export PATH=$PATH:$bin:$bin/aux
done

set -x

for service in /etc/systemd/system/rke2*.service; do
    [ -s ${service} ] && systemctl stop $(basename ${service})
done

for service in /etc/init.d/rke2*; do
    [ -x ${service} ] && ${service} stop
done

pschildren() {
    ps -e -o ppid= -o pid= | \
    sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
    grep -w "^$1" | \
    cut -f2
}

pstree() {
    for pid in $@; do
        echo ${pid}
        for child in $(pschildren ${pid}); do
            pstree ${child}
        done
    done
}

killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        pstree $@;
        set -x;
    ) 2>/dev/null
}

getshims() {
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w 'rke2/data/[^/]*/bin/containerd-shim' | cut -f1
}

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount() {
    { set +x; } 2>/dev/null
    MOUNTS=
    while read ignore mount ignore; do
        MOUNTS="${mount}\n${MOUNTS}"
    done </proc/self/mounts
    MOUNTS=$(printf ${MOUNTS} | grep "^$1" | sort -r)
    if [ -n "${MOUNTS}" ]; then
        set -x
        umount ${MOUNTS}
    else
        set -x
    fi
}

do_unmount '/run/rke2'
do_unmount '${BASE_DIR}'
do_unmount '/var/lib/kubelet/pods'
do_unmount '/run/netns/cni-'

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
    iface=${iface%%@*}
    [ -z "$iface" ] || ip link delete $iface
done
ip link delete cni0
ip link delete flannel.1
rm -rf /var/lib/cni/
iptables-save | grep -v KUBE- | grep -v CNI- | iptables-restore
EOF
    ${SUDO} chmod 755 "${KILLALL_RKE2_SH}"

    if [ ${USING_RKE2_USER} ]; then
        ${SUDO} chown "${INSTALL_RKE2_USER}":"${INSTALL_RKE2_USER}" "${KILLALL_RKE2_SH}"
    else 
        ${SUDO} chown root:root "${KILLALL_RKE2_SH}"
    fi
}

# create_uninstall creates the uninstall script.
create_uninstall() {
    if [ "${INSTALL_RKE2_BIN_DIR_READ_ONLY}" = true ]; then
        return
    fi
    info "creating uninstall script ${UNINSTALL_RKE2_SH}"
    ${SUDO} tee "${UNINSTALL_RKE2_SH}" >/dev/null << EOF
#!/bin/sh
set -x
[ \$(id -u) -eq 0 ] || exec sudo \$0 \$@

${KILLALL_RKE2_SH}

if which systemctl; then
    systemctl disable ${SYSTEM_NAME}
    systemctl reset-failed ${SYSTEM_NAME}
    systemctl daemon-reload
fi
if which rc-update; then
    rc-update delete ${SYSTEM_NAME} default
fi

rm -f ${FILE_RKE2_SERVICE}
rm -f ${FILE_RKE2_ENV}

remove_uninstall() {
    rm -f ${UNINSTALL_RKE2_SH}
}
trap remove_uninstall EXIT

if (ls ${SYSTEMD_DIR}/rke2*.service || ls /etc/init.d/rke2*) >/dev/null 2>&1; then
    set +x; echo 'Additional rke2 services installed, skipping uninstall of rke2'; set -x
    exit
fi

for cmd in kubectl crictl ctr; do
    if [ -L ${BIN_DIR}/\$cmd ]; then
        rm -f ${BIN_DIR}/\$cmd
    fi
done

rm -rf /etc/rancher/rke2
rm -rf "${BASE_DIR}"
rm -rf /var/lib/kubelet
rm -f ${BIN_DIR}/rke2
rm -f ${KILLALL_RKE2_SH}
EOF
    ${SUDO} chmod 755 "${UNINSTALL_RKE2_SH}"

    if [ ${USING_RKE2_USER} ]; then
        ${SUDO} chown "${INSTALL_RKE2_USER}":"${INSTALL_RKE2_USER}" "${UNINSTALL_RKE2_SH}"
    else
        ${SUDO} chown root:root "${UNINSTALL_RKE2_SH}"
    fi
}

# systemd_disable disables the current 
# service if loaded.
systemd_disable() {
    ${SUDO} rm -f "/etc/systemd/system/${SERVICE_RKE2}" || true
    ${SUDO} rm -f "/etc/systemd/system/${SERVICE_RKE2}.env" || true
    ${SUDO} systemctl disable "${SYSTEM_NAME}" >/dev/null 2>&1 || true
}

# create_env_file captures current env and creates
# a file containing rke2_ variables.
create_env_file() {
    info "env: creating environment file ${FILE_RKE2_ENV}"
    UMASK=$(umask)
    umask 0377
    env | grep '^RKE2_' | ${SUDO} tee "${FILE_RKE2_ENV}" >/dev/null
    env | grep -E -i '^(NO|HTTP|HTTPS)_PROXY' | ${SUDO} tee -a "${FILE_RKE2_ENV}" >/dev/null
    umask "${UMASK}"
}

# create_systemd_service_file writes the
# systemd service file.
create_systemd_service_file() {
    info "systemd: Creating service file ${FILE_RKE2_SERVICE}"
    ${SUDO} tee "${FILE_RKE2_SERVICE}" >/dev/null << EOF
[Unit]
Description=Rancher Kubernetes Engine v2
Documentation=https://rke2.io
Wants=network-online.target

[Install]
WantedBy=multi-user.target

[Service]
Type=${SYSTEMD_TYPE}
EnvironmentFile=${FILE_RKE2_ENV}
KillMode=process
Delegate=yes
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
Restart=always
RestartSec=5s
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=${BIN_DIR}/rke2 \\
    ${CMD_RKE2_EXEC}

EOF
    echo "HOME=/root" > /etc/systemd/system/rke2.service.env
}

# create_openrc_service_file writes the openrc
# service file.
create_openrc_service_file() {
    LOG_FILE=/var/log/${SYSTEM_NAME}.log

    info "openrc: Creating service file ${FILE_RKE2_SERVICE}"
    ${SUDO} tee "${FILE_RKE2_SERVICE}" >/dev/null << EOF
#!/sbin/openrc-run

depend() {
    after network-online
    want cgroups
}

start_pre() {
    rm -f /tmp/rke2.*
}

supervisor=supervise-daemon
name=${SYSTEM_NAME}
command="${BIN_DIR}/rke2"
command_args="$(escape_dq "${CMD_RKE2_EXEC}")
    >>${LOG_FILE} 2>&1"

output_log=${LOG_FILE}
error_log=${LOG_FILE}

pidfile="/var/run/${SYSTEM_NAME}.pid"
respawn_delay=5

set -o allexport
if [ -f /etc/environment ]; then source /etc/environment; fi
if [ -f ${FILE_RKE2_ENV} ]; then source ${FILE_RKE2_ENV}; fi
set +o allexport
EOF
    ${SUDO} chmod 0755 "${FILE_RKE2_SERVICE}"

    ${SUDO} tee "/etc/logrotate.d/${SYSTEM_NAME}" >/dev/null << EOF
${LOG_FILE} {
	missingok
	notifempty
	copytruncate
}
EOF
}

# create_service_file writes the supervisor 
# service file.
create_service_file() {
    if [ "${HAS_SYSTEMD}" = true ]; then
        create_systemd_service_file
    fi
    if [ "${HAS_OPENRC}" = true ]; then 
        create_openrc_service_file
    fi
    return 0
}

# get_installed_hashes gets the hashes of the 
# current rke2 binary and service files.
get_installed_hashes() {
    ${SUDO} sha256sum ${BIN_DIR}/rke2 "${FILE_RKE2_SERVICE}" "${FILE_RKE2_ENV}" 2>&1 || true
}

# systemd_enable enables and starts systemd service.
systemd_enable() {
    info "systemd: Enabling ${SYSTEM_NAME} unit"
    ${SUDO} systemctl enable ${FILE_RKE2_SERVICE} >/dev/null
    ${SUDO} systemctl daemon-reload >/dev/null
}

# systemd_start starts systemd.
systemd_start() {
    info "systemd: starting ${SYSTEM_NAME}"
    ${SUDO} systemctl restart "${SYSTEM_NAME}"
}

# openrc_enable enables and starts openrc service.
openrc_enable() {
    info "openrc: enabling ${SYSTEM_NAME} service for default runlevel"
    ${SUDO} rc-update add "${SYSTEM_NAME}" default >/dev/null
}

# openrc_start starts openrc.
openrc_start() {
    info "openrc: starting ${SYSTEM_NAME}"
    ${SUDO} "${FILE_RKE2_SERVICE}" restart
}

# service_enable_and_start starts up the supervisor service.
service_enable_and_start() {
    if [ "${INSTALL_RKE2_SKIP_ENABLE}" = true ]; then
        return
    fi

    if [ "${HAS_SYSTEMD}" = true ]; then
        systemd_enable
    fi

    if [ "${HAS_OPENRC}" = true ]; then
        openrc_enable
    fi

    if [ "${INSTALL_RKE2_SKIP_START}" = true ]; then
        return
    fi

    POST_INSTALL_HASHES=$(get_installed_hashes)
    if [ "${PRE_INSTALL_HASHES}" = "${POST_INSTALL_HASHES}" ]; then
        info "no change detected so skipping service start"
        return
    fi

    if [ "${HAS_SYSTEMD}" = true ]; then
        systemd_start
    fi

    if [ "${HAS_OPENRC}" = true ]; then
        openrc_start
    fi
    return 0
}

# create_user creates a new 
# user with the given name.
create_user() {
    if [ -z "$1" ]; then
        echo "error: no user given for creation"
        exit 1
    fi
    if [ -z "$2" ]; then
        echo "error: no user description given"
        exit 1
    fi

    if [ "$(id -u "$1" 2>/dev/null)" != 1 ]; then
        no_login=$(command -v nologin)
        
        if [ ! -z "${no_login}" ]; then
            useradd -r -d "${BASE_DIR}" -c "$2" -s "${no_login}" "$1"
        else
            useradd -r -d "${BASE_DIR}" -c "$2" -s /bin/false "$1"
        fi
    else 
        info "$1 exists. moving on..."
    fi
}

# re-evaluate args to include env command
eval set -- $(escape "${INSTALL_RKE2_EXEC}") $(quote "$@")

# setup_rke2_user creates the rke2 user and group, home
# directory, and sets necessary ownership.
setup_rke2_user() {
    mkdir -p "${BASE_DIR}"
    create_user "$1" "RKE2 Service User"
    chown -R "$1":"$1" "$(dirname ${BASE_DIR})"
    USING_RKE2_USER=1
}

# setup_etcd_user creates the etcd user, provides a description
# and adds it to the rke2 group if it exists.
setup_etcd_user() {
    create_user "$1" "ETCD Service User"
    if [ "$(id -u "rke2" 2>/dev/null)" = 1 ]; then
        usermod -a -G "${INSTALL_RKE2_USER}" "${INSTALL_RKE2_ETCD_USER}"
    fi
    USING_ETCD_USER=1
}

# update_kernel_params adjusts the necessary kernel parameters
# to allow RKE2 to run in CIS mode.
update_kernel_params() {
    for param in vm.panic_on_oom=0 kernel.panic=10 kernel.panic_on_oops=1 kernel.keys.root_maxbytes=25000000; do
        sysctl -w ${param}
        echo ${param} >> /etc/sysctl.d/local.conf
    done
}

# main
{
    if [ "${INSTALL_RKE2_CIS_MODE}" = true ]; then
        update_kernel_params
        setup_etcd_user "etcd"
    fi

    if [ "${INSTALL_RKE2_USER}" = true ]; then
        setup_rke2_user "rke2"
    fi

    if [ "${INSTALL_RKE2_ETCD_USER}" = true ] && [ ${USING_ETCD_USER} != 1 ] ; then
        setup_etcd_user "etcd"
    fi

    verify_system
    setup_env "$@"
    download_and_verify
    setup_selinux
    create_killall
    create_uninstall
    systemd_disable
    create_env_file
    create_service_file
    service_enable_and_start
    create_symlinks
}

exit 0
