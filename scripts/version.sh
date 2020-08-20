#!/bin/bash
set -x

PROG=rke2
REPO=${REPO:-rancher}
K3S_PKG=github.com/rancher/k3s
RKE2_PKG=github.com/rancher/rke2
GO=${GO-go}
GOARCH=${GOARCH:-$("${GO}" env GOARCH)}
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

GIT_TAG=$DRONE_TAG
TREE_STATE=clean
COMMIT=$DRONE_COMMIT
REVISION=$(git rev-parse HEAD)$(if ! git diff --no-ext-diff --quiet --exit-code; then echo .dirty; fi)
PLATFORM=${GOOS}-${GOARCH}
RELEASE=${PROG}.${PLATFORM}
# hardcode k8s version unless its set specifically
KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.18.4}

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
    VERSION="${KUBERNETES_VERSION}-dev+${COMMIT:0:8}$DIRTY"
fi

DOCKERIZED_VERSION="${VERSION/+/-}" # this mimics what kubernetes builds do
