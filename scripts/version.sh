#!/bin/bash
set -x

PROG=rke2
REGISTRY=${REGISTRY:-docker.io}
REPO=${REPO:-rancher}
K3S_PKG=github.com/k3s-io/k3s
RKE2_PKG=github.com/rancher/rke2
GO=${GO-go}
GOARCH=${GOARCH:-$("${GO}" env GOARCH)}
ARCH=${ARCH:-"${GOARCH}"}
GOOS=${GOOS:-$("${GO}" env GOOS)}
if [ -z "$GOOS" ]; then
    if [ "${OS}" == "Windows_NT" ]; then
      GOOS="windows"
    else
      UNAME_S=$(shell uname -s)
		  if [ "${UNAME_S}" == "Linux" ]; then
			    GOOS="linux"
		  elif [ "${UNAME_S}" == "Darwin" ]; then
				  GOOS="darwin"
		  elif [ "${UNAME_S}" == "FreeBSD" ]; then
				  GOOS="freebsd"
		  fi
    fi
fi

GIT_TAG=$GITHUB_ACTION_TAG
TREE_STATE=clean
COMMIT=$DRONE_COMMIT
REVISION=$(git rev-parse HEAD)$(if ! git diff --no-ext-diff --quiet --exit-code; then echo .dirty; fi)
PLATFORM=${GOOS}-${GOARCH}
RELEASE=${PROG}.${PLATFORM}
# hardcode versions unless set specifically
KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.34.2}
KUBERNETES_IMAGE_TAG=${KUBERNETES_IMAGE_TAG:-v1.34.2-rke2r1-build20251112}
ETCD_VERSION=${ETCD_VERSION:-v3.6.5-k3s1}
PAUSE_VERSION=${PAUSE_VERSION:-3.6}
CCM_VERSION=${CCM_VERSION:-v1.34.2-0.20251010190833-cf0d35a732d1-build20251017}

if [ -d .git ]; then
    if [ -z "$GIT_TAG" ]; then
        GIT_TAG=$(git tag -l --contains HEAD | head -n 1)
    fi
    if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
        DIRTY="-dirty"
        TREE_STATE=dirty
    fi

    COMMIT=$(git log -n3 --pretty=format:"%H %ae" | grep -v ' drone@localhost$' | cut -f1 -d\  | head -1)
    if [ -z "${COMMIT}" ]; then
        COMMIT=$(git rev-parse HEAD || true)
    fi
fi

if [[ -n "$GIT_TAG" ]]; then
    VERSION=$GIT_TAG
else
    VERSION="${KUBERNETES_VERSION}+dev.${COMMIT:0:8}$DIRTY"
fi

if [[ "${VERSION}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)([-+][a-zA-Z0-9.]+)?[-+]((rke2r[0-9]+|dev.*))$ ]]; then
    VERSION_MAJOR=${BASH_REMATCH[1]}
    VERSION_MINOR=${BASH_REMATCH[2]}
    PATCH=${BASH_REMATCH[3]}
    RC=${BASH_REMATCH[4]}
    RKE2_PATCH=${BASH_REMATCH[5]}
    echo "VERSION=${VERSION} parsed as MAJOR=${MAJOR} MINOR=${MINOR} PATCH=${PATCH} RC=${RC} RKE2_PATCH=${RKE2_PATCH}"
fi

DEPENDENCIES_URL="https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/dependencies.yaml"
VERSION_GOLANG="go"$(curl -sL "${DEPENDENCIES_URL}" | yq e '.dependencies[] | select(.name == "golang: upstream version").version' -)

DOCKERIZED_VERSION="${VERSION/+/-}" # this mimics what kubernetes builds do
