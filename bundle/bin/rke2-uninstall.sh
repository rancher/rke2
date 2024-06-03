#!/bin/sh
set -ex


# helper function for timestamped logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

# helper function for logging error and exiting with a message
error() {
    log "ERROR: $*" >&2
    exit 1
}

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    error "$(basename "$0"): must be run as root"
fi

# check_target_mountpoint return success if the target directory is on a dedicated mount point
check_target_mountpoint() {
    mountpoint -q "$1"
}

# check_target_ro returns success if the target directory is read-only
check_target_ro() {
    touch "$1"/.rke2-ro-test && rm -rf "$1"/.rke2-ro-test
    test $? -ne 0
}

RKE2_DATA_DIR=${RKE2_DATA_DIR:-/var/lib/rancher/rke2}

. /etc/os-release

if [ -r /etc/redhat-release ] || [ -r /etc/centos-release ] || [ -r /etc/oracle-release ] || [ -r /etc/amazon-linux-release ]; then
    # If redhat/oracle family os is detected, double check whether installation mode is yum or tar.
    # yum method assumes installation root under /usr
    # tar method assumes installation root under /usr/local
    if rpm -q rke2-common >/dev/null 2>&1; then
        : "${INSTALL_RKE2_ROOT:="/usr"}"
    else
        : "${INSTALL_RKE2_ROOT:="/usr/local"}"
    fi
elif [ "${ID_LIKE%%[ ]*}" = "suse" ]; then
    if rpm -q rke2-common >/dev/null 2>&1; then
        : "${INSTALL_RKE2_ROOT:="/usr"}"
        if [ -x /usr/sbin/transactional-update ]; then
            transactional_update="transactional-update -c --no-selfupdate -d run"
        fi
    elif check_target_mountpoint "/usr/local" || check_target_ro "/usr/local"; then
        # if /usr/local is mounted on a specific mount point or read-only then
        # install we assume that installation happened in /opt/rke2
        : "${INSTALL_RKE2_ROOT:="/opt/rke2"}"
    else
        : "${INSTALL_RKE2_ROOT:="/usr/local"}"
    fi
else
    : "${INSTALL_RKE2_ROOT:="/usr/local"}"
fi

uninstall_killall()
{
    _killall="$(dirname "$0")/rke2-killall.sh"
    log "Running killall script"
    if [ -e "${_killall}" ]; then
      eval "${_killall}" || error "Failed to execute killall script"
    fi
}

uninstall_disable_services()
{
    log "Disabling rke2 services"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl disable rke2-server || true
        systemctl disable rke2-agent || true
        systemctl reset-failed rke2-server || true
        systemctl reset-failed rke2-agent || true
        systemctl daemon-reload || error "Failed to reload systemd daemon"
    fi
}

uninstall_remove_files()
{

    if [ -r /etc/redhat-release ] || [ -r /etc/centos-release ] || [ -r /etc/oracle-release ] || [ -r /etc/amazon-linux-release ]; then
        log "Removing rke2 packages"
        yum remove -y "rke2-*" || error "Failed to remove rke2 packages"
        rm -f /etc/yum.repos.d/rancher-rke2*.repo || error "Failed to remove yum repo files"
    fi

    if [ "${ID_LIKE%%[ ]*}" = "suse" ]; then
         if rpm -q rke2-common >/dev/null 2>&1; then
            log "Removing rke2 packages using zypper"
            # rke2 rpm detected
            uninstall_cmd="zypper remove -y rke2-server rke2-agent rke2-common rke2-selinux"
            if [ "${TRANSACTIONAL_UPDATE=false}" != "true" ] && [ -x /usr/sbin/transactional-update ]; then
                uninstall_cmd="transactional-update -c --no-selfupdate -d run $uninstall_cmd"
            fi
            $uninstall_cmd || error "Failed to remove rke2 packages using zypper"
            rm -f /etc/zypp/repos.d/rancher-rke2*.repo || error "Failed to remove zypp repo files"
         fi
    fi

    log "Removing rke2 files"
    $transactional_update find "${INSTALL_RKE2_ROOT}/lib/systemd/system" -name rke2-*.service -type f -delete || error "Failed to remove rke2 service files"
    $transactional_update find "${INSTALL_RKE2_ROOT}/lib/systemd/system" -name rke2-*.env -type f -delete || error "Failed to remove rke2 env files"
    find /etc/systemd/system -name rke2-*.service -type f -delete || error "Failed to remove rke2 systemd service files"
    $transactional_update rm -f "${INSTALL_RKE2_ROOT}/bin/rke2" || error "Failed to remove rke2 binary"
    $transactional_update rm -f "${INSTALL_RKE2_ROOT}/bin/rke2-killall.sh" || error "Failed to remove rke2 killall script"
    $transactional_update rm -rf "${INSTALL_RKE2_ROOT}/share/rke2" || error "Failed to remove rke2 share directory"
    rm -rf /etc/rancher/rke2 || error "Failed to remove /etc/rancher/rke2"
    rm -rf /etc/rancher/node || error "Failed to remove /etc/rancher/node"
    rm -d /etc/rancher || true
    rm -rf /etc/cni || error "Failed to remove /etc/cni"
    rm -rf /opt/cni/bin || error "Failed to remove /opt/cni/bin"
    rm -rf /var/lib/kubelet || true
    rm -rf "${RKE2_DATA_DIR}" || error "Failed to remove /var/lib/rancher/rke2"
    rm -d /var/lib/rancher || true

    if type fapolicyd >/dev/null 2>&1; then
        log "Removing fapolicyd rules"
        if [ -f /etc/fapolicyd/rules.d/80-rke2.rules ]; then
            rm -f /etc/fapolicyd/rules.d/80-rke2.rules || error "Failed to remove fapolicyd rules"
        fi
        fagenrules --load || error "Failed to reload fapolicyd rules"
        systemctl try-restart fapolicyd || error "Failed to restart fapolicyd"
    fi
}


uninstall_remove_self()
{
    cleanup
    log "Removing uninstall script"
    $transactional_update rm -f "${INSTALL_RKE2_ROOT}/bin/rke2-uninstall.sh" || error "Failed to remove uninstall script"
}

# Define a cleanup function that triggers on exit
cleanup() {
  # Check if last command's exit status was not equal to 0
  if [ $? -ne 0 ]; then
    echo -e "\e[31mCleanup didn't complete successfully\e[0m"
  else
    log "Cleanup completed successfully"
  fi
}

uninstall_remove_policy()
{
    log "Removing SELinux policy"
    semodule -r rke2 || true
}

# Set a trap to log an error if the script exits unexpectedly
trap cleanup EXIT
uninstall_killall
trap - EXIT

trap uninstall_remove_self EXIT
uninstall_disable_services
uninstall_remove_files
uninstall_remove_policy

log "Cleanup completed successfully"