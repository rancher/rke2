#!/bin/bash

set -euo pipefail

info()
{
    echo '[INFO] ' "$@"
}
fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

if [ $# -ne 2 ]; then
    fatal "usage: $0 <hardened-kubernetes-tag> <hardened-build-base-tag>"
fi

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "${REPO_ROOT}"

KUBERNETES_IMAGE_TAG=${1}
BUILD_BASE_TAG=${2}
KUBERNETES_VERSION=$(echo "${KUBERNETES_IMAGE_TAG}" | sed -nE 's/^(v[0-9]+\.[0-9]+\.[0-9]+)-rke2r[0-9]+-build[0-9]{8}$/\1/p')

if [ -z "${KUBERNETES_VERSION}" ]; then
    fatal "unable to derive the Kubernetes version from ${KUBERNETES_IMAGE_TAG}"
fi

VERSION_REGEX=$(printf '%s\n' "${KUBERNETES_VERSION}" | sed 's/\./\\./g')
K3S_VERSION=$(git ls-remote --tags https://github.com/k3s-io/kubernetes.git \
    | sed 's#.*refs/tags/##' \
    | grep -E "^${VERSION_REGEX}-k3s[0-9]+$" \
    | sort -V \
    | tail -n 1)

if [ -z "${K3S_VERSION}" ]; then
    fatal "unable to determine the matching k3s-io/kubernetes tag for ${KUBERNETES_VERSION}"
fi

LINUX_KUBECTL_SHA256=$(curl -fsSL "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/linux/amd64/kubectl.sha256" | tr -d '\n')
WINDOWS_KUBECTL_SHA256=$(curl -fsSL "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/windows/amd64/kubectl.exe.sha256" | tr -d '\n')
WINDOWS_KUBELET_SHA256=$(curl -fsSL "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/windows/amd64/kubelet.exe.sha256" | tr -d '\n')
WINDOWS_KUBE_PROXY_SHA256=$(curl -fsSL "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/windows/amd64/kube-proxy.exe.sha256" | tr -d '\n')

CURRENT_KUBERNETES_VERSION=$(sed -nE 's/^KUBERNETES_VERSION=\$\{KUBERNETES_VERSION:-([^}]*)\}$/\1/p' scripts/version.sh)
CURRENT_KUBERNETES_IMAGE_TAG=$(sed -nE 's/^KUBERNETES_IMAGE_TAG=\$\{KUBERNETES_IMAGE_TAG:-([^}]*)\}$/\1/p' scripts/version.sh)
CURRENT_BUILD_BASE_TAG=$(sed -nE 's/^FROM rancher\/hardened-build-base:([^ ]+) AS base$/\1/p' Dockerfile)
CURRENT_K3S_VERSION=$(sed -nE 's/^\tk8s.io\/api => .* (v[0-9]+\.[0-9]+\.[0-9]+-k3s[0-9]+)$/\1/p' go.mod | head -n 1)

if [ -z "${CURRENT_KUBERNETES_VERSION}" ] || [ -z "${CURRENT_KUBERNETES_IMAGE_TAG}" ] || [ -z "${CURRENT_BUILD_BASE_TAG}" ] || [ -z "${CURRENT_K3S_VERSION}" ]; then
    fatal "unable to determine the current versions from the repository"
fi

info "updating hardened-kubernetes from ${CURRENT_KUBERNETES_IMAGE_TAG} to ${KUBERNETES_IMAGE_TAG}"
info "updating hardened-build-base from ${CURRENT_BUILD_BASE_TAG} to ${BUILD_BASE_TAG}"
info "updating k3s-io/kubernetes from ${CURRENT_K3S_VERSION} to ${K3S_VERSION}"

python - "${KUBERNETES_VERSION}" "${KUBERNETES_IMAGE_TAG}" "${BUILD_BASE_TAG}" "${CURRENT_KUBERNETES_VERSION}" "${CURRENT_KUBERNETES_IMAGE_TAG}" "${CURRENT_BUILD_BASE_TAG}" "${CURRENT_K3S_VERSION}" "${K3S_VERSION}" "${LINUX_KUBECTL_SHA256}" "${WINDOWS_KUBECTL_SHA256}" "${WINDOWS_KUBELET_SHA256}" "${WINDOWS_KUBE_PROXY_SHA256}" <<'PY'
import re
import sys
from pathlib import Path

(
    kubernetes_version,
    kubernetes_image_tag,
    build_base_tag,
    current_kubernetes_version,
    current_kubernetes_image_tag,
    current_build_base_tag,
    current_k3s_version,
    k3s_version,
    linux_kubectl_sha256,
    windows_kubectl_sha256,
    windows_kubelet_sha256,
    windows_kube_proxy_sha256,
) = sys.argv[1:]

dockerfile = Path("Dockerfile")
text = dockerfile.read_text()
text = text.replace(
    f"FROM rancher/hardened-build-base:{current_build_base_tag} AS base",
    f"FROM rancher/hardened-build-base:{build_base_tag} AS base",
    1,
)
text = text.replace(
    f"FROM rancher/hardened-kubernetes:{current_kubernetes_image_tag} AS kubernetes",
    f"FROM rancher/hardened-kubernetes:{kubernetes_image_tag} AS kubernetes",
    1,
)
dockerfile.write_text(text)

version_script = Path("scripts/version.sh")
text = version_script.read_text()
text = text.replace(
    f"KUBERNETES_VERSION=${{KUBERNETES_VERSION:-{current_kubernetes_version}}}",
    f"KUBERNETES_VERSION=${{KUBERNETES_VERSION:-{kubernetes_version}}}",
    1,
)
text = text.replace(
    f"KUBERNETES_IMAGE_TAG=${{KUBERNETES_IMAGE_TAG:-{current_kubernetes_image_tag}}}",
    f"KUBERNETES_IMAGE_TAG=${{KUBERNETES_IMAGE_TAG:-{kubernetes_image_tag}}}",
    1,
)
version_script.write_text(text)

dockerfile_windows = Path("Dockerfile.windows")
text = dockerfile_windows.read_text()
text = text.replace(
    f"FROM rancher/hardened-build-base:{current_build_base_tag} AS build-env",
    f"FROM rancher/hardened-build-base:{build_base_tag} AS build-env",
    1,
)
text = re.sub(
    r'RUN KUBECTL_VERSION=v[0-9]+\.[0-9]+\.[0-9]+ && \\\n    KUBECTL_SHA256="[^"]+" ;; \\',
    lambda _: (
        f'RUN KUBECTL_VERSION={kubernetes_version} && \\\n'
        f'    KUBECTL_SHA256="{linux_kubectl_sha256}" ;; \\'
    ),
    text,
    count=1,
)
text = re.sub(
    r'(RUN case "\$\{KUBERNETES_VERSION\}" in \\\n)(.*?)(        \*\) echo "Unsupported KUBERNETES_VERSION for pinned Windows binaries: \$\{KUBERNETES_VERSION\}" && exit 1 ;;\s*\\\n)',
    lambda match: (
        match.group(1)
        + f'        {kubernetes_version}) \\\n'
        + f'            KUBECTL_SHA256="{windows_kubectl_sha256}" && \\\n'
        + f'            KUBELET_SHA256="{windows_kubelet_sha256}" && \\\n'
        + f'            KUBE_PROXY_SHA256="{windows_kube_proxy_sha256}"; \\\n'
        + f'            ;; \\\n'
        + match.group(3)
    ),
    text,
    count=1,
    flags=re.S,
)
dockerfile_windows.write_text(text)

go_mod = Path("go.mod")
text = go_mod.read_text()
text = text.replace(current_k3s_version, k3s_version)
go_mod.write_text(text)
PY

go mod tidy
