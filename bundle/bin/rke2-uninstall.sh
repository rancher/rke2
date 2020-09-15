#!/bin/sh
set -ex

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

: "${INSTALL_RKE2_ROOT:="/usr/local"}"

uninstall_killall()
{
    _killall="$(dirname "$0")/rke2-killall.sh"
    if [ -e "${_killall}" ]; then
      eval "${_killall}"
    fi
}

uninstall_disable_services()
{
    if command -v systemctl >/dev/null 2>&1; then
        systemctl disable rke2-server || true
        systemctl disable rke2-agent || true
        systemctl reset-failed rke2-server || true
        systemctl reset-failed rke2-agent || true
        systemctl daemon-reload
    fi
}

uninstall_remove_files()
{
    rm -f "${INSTALL_RKE2_ROOT}/lib/systemd/system/rke2*.service"
    rm -f "${INSTALL_RKE2_ROOT}/bin/rke2"
    rm -f "${INSTALL_RKE2_ROOT}/bin/rke2-killall.sh"
    rm -rf "${INSTALL_RKE2_ROOT}/share/rke2"
    rm -rf /etc/rancher/rke2
    rm -rf /var/lib/kubelet
    rm -rf /var/lib/rancher/rke2
}

uninstall_remove_self()
{
    rm -f "${INSTALL_RKE2_ROOT}/bin/rke2-uninstall.sh"
}

uninstall_killall
trap uninstall_remove_self EXIT
uninstall_disable_services
uninstall_remove_files
