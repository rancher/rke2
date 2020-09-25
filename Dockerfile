ARG KUBERNETES_VERSION=dev
# Build environment
FROM rancher/hardened-build-base:v1.13.15b4 AS build
RUN set -x \
 && apk --no-cache add \
    bash \
    curl \
    file \
    git \
    libseccomp-dev \
    rsync

# Dapper/Drone/CI environment
FROM build AS dapper
ENV DAPPER_ENV GODEBUG REPO TAG DRONE_TAG PAT_USERNAME PAT_TOKEN KUBERNETES_VERSION DOCKER_BUILDKIT
ENV DAPPER_OUTPUT ./dist ./bin ./build
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_TARGET dapper
ENV DAPPER_RUN_ARGS "-v rke2-pkg:/go/pkg -v rke2-cache:/root/.cache/go-build"
RUN curl -sL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.27.0
WORKDIR /source
# End Dapper stuff

# Shell used for debugging
FROM dapper AS shell
RUN set -x \
 && apk --no-cache add \
    bash-completion \
    iptables \
    less \
    psmisc \
    rsync \
    socat \
    sudo \
    vim
RUN GO111MODULE=off GOBIN=/usr/local/bin go get github.com/go-delve/delve/cmd/dlv
RUN echo 'alias abort="echo -e '\''q\ny\n'\'' | dlv connect :2345"' >> /root/.bashrc
ENV PATH=/var/lib/rancher/rke2/bin:$PATH
ENV KUBECONFIG=/etc/rancher/rke2/rke2.yaml
VOLUME /var/lib/rancher/rke2
# This makes it so we can run and debug k3s too
VOLUME /var/lib/rancher/k3s

FROM build AS build-k8s-codegen
ARG KUBERNETES_VERSION
RUN git clone -b ${KUBERNETES_VERSION} --depth=1 https://github.com/kubernetes/kubernetes.git ${GOPATH}/src/github.com/kubernetes/kubernetes
WORKDIR ${GOPATH}/src/github.com/kubernetes/kubernetes
# force code generation
RUN make WHAT=cmd/kube-apiserver
ARG TAG
# build statically linked executables
RUN echo "export GIT_COMMIT=$(git rev-parse HEAD)" \
    >> /usr/local/go/bin/go-build-static-k8s.sh
RUN echo "export BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >> /usr/local/go/bin/go-build-static-k8s.sh
RUN echo "export GO_LDFLAGS=\"-linkmode=external \
    -X k8s.io/component-base/version.gitVersion=${TAG} \
    -X k8s.io/component-base/version.gitCommit=\${GIT_COMMIT} \
    -X k8s.io/component-base/version.gitTreeState=clean \
    -X k8s.io/component-base/version.buildDate=\${BUILD_DATE} \
    -X k8s.io/client-go/pkg/version.gitVersion=${TAG} \
    -X k8s.io/client-go/pkg/version.gitCommit=\${GIT_COMMIT} \
    -X k8s.io/client-go/pkg/version.gitTreeState=clean \
    -X k8s.io/client-go/pkg/version.buildDate=\${BUILD_DATE} \
    \"" >> /usr/local/go/bin/go-build-static-k8s.sh
RUN echo 'go-build-static.sh -gcflags=-trimpath=${GOPATH}/src/github.com/kubernetes/kubernetes -mod=vendor -tags=selinux,osusergo,netgo ${@}' \
    >> /usr/local/go/bin/go-build-static-k8s.sh
RUN chmod -v +x /usr/local/go/bin/go-*.sh

FROM build-k8s-codegen AS build-k8s
RUN go-build-static-k8s.sh -o bin/kube-apiserver           ./cmd/kube-apiserver
RUN go-build-static-k8s.sh -o bin/apiextensions-apiserver  ./vendor/k8s.io/apiextensions-apiserver
RUN go-build-static-k8s.sh -o bin/kube-controller-manager  ./cmd/kube-controller-manager
RUN go-build-static-k8s.sh -o bin/kube-scheduler           ./cmd/kube-scheduler
RUN go-build-static-k8s.sh -o bin/kube-proxy               ./cmd/kube-proxy
RUN go-build-static-k8s.sh -o bin/kubeadm                  ./cmd/kubeadm
RUN go-build-static-k8s.sh -o bin/kubectl                  ./cmd/kubectl
RUN go-build-static-k8s.sh -o bin/kubelet                  ./cmd/kubelet
RUN go-assert-static.sh bin/*
RUN go-assert-boring.sh bin/*
RUN install -s bin/* /usr/local/bin/
RUN kube-proxy --version

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest AS kubernetes
RUN microdnf update -y           && \
    microdnf install -y iptables && \
    rm -rf /var/cache/yum
COPY --from=build-k8s \
    /usr/local/bin/ \
    /usr/local/bin/

FROM build AS charts
ARG CHARTS_REPO="https://rke2-charts.rancher.io"
ARG CACHEBUST="cachebust"
COPY charts/ /charts/
RUN echo ${CACHEBUST}>/dev/null
RUN CHART_VERSION="v3.13.3"     CHART_FILE=/charts/rke2-canal-chart.yml             CHART_BOOTSTRAP=true    /charts/build-chart.sh
RUN CHART_VERSION="1.10.101"    CHART_FILE=/charts/rke2-coredns-chart.yml           CHART_BOOTSTRAP=true    /charts/build-chart.sh
RUN CHART_VERSION="1.36.300"    CHART_FILE=/charts/rke2-ingress-nginx-chart.yml     CHART_BOOTSTRAP=false   /charts/build-chart.sh
RUN CHART_VERSION="v1.18.9"     CHART_FILE=/charts/rke2-kube-proxy-chart.yml        CHART_BOOTSTRAP=true    /charts/build-chart.sh
RUN CHART_VERSION="2.11.100"    CHART_FILE=/charts/rke2-metrics-server-chart.yml    CHART_BOOTSTRAP=false   /charts/build-chart.sh
RUN rm -vf /charts/*.sh /charts/*.md

# rke-runtime image
# This image includes any host level programs that we might need. All binaries
# must be placed in bin/ of the file image and subdirectories of bin/ will be flattened during installation.
# This means bin/foo/bar will become bin/bar when rke2 installs this to the host
FROM rancher/k3s:v1.18.9-k3s1 AS k3s
FROM rancher/hardened-containerd:v1.3.6-k3s2 AS containerd
FROM rancher/hardened-crictl:v1.18.0 AS crictl
FROM rancher/hardened-runc:v1.0.0-rc92 AS runc

FROM scratch AS runtime
COPY --from=k3s \
    /bin/socat \
    /bin/
COPY --from=runc \
    /usr/local/bin/runc \
    /bin/
COPY --from=crictl \
    /usr/local/bin/crictl \
    /bin/
COPY --from=containerd \
    /usr/local/bin/containerd \
    /usr/local/bin/containerd-shim \
    /usr/local/bin/containerd-shim-runc-v1 \
    /usr/local/bin/containerd-shim-runc-v2 \
    /usr/local/bin/ctr \
    /bin/
COPY --from=kubernetes \
    /usr/local/bin/kubectl \
    /usr/local/bin/kubelet \
    /bin/
COPY --from=charts \
    /charts/ \
    /charts/
