ARG KUBERNETES_VERSION=dev

# Build environment
FROM rancher/hardened-build-base:v1.24.9b1 AS build
ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH
RUN set -x && \
    apk --no-cache add \
    bash \
    curl \
    file \
    git \
    libseccomp-dev \
    rsync \
    gcc \
    bsd-compat-headers \
    py-pip \
    py3-pip \
    pigz \
    tar \
    yq \
    helm

RUN if [ "${ARCH}" = "amd64" ]; then \
    	apk --no-cache add mingw-w64-gcc; \
    fi

FROM registry.suse.com/bci/bci-base AS rpm-macros
RUN zypper install -y systemd-rpm-macros

# Dapper/Drone/CI environment
FROM build AS dapper
ENV DAPPER_ENV GODEBUG CI GOCOVER REPO TAG GITHUB_ACTION_TAG PAT_USERNAME PAT_TOKEN KUBERNETES_VERSION DOCKER_BUILDKIT DRONE_BUILD_EVENT IMAGE_NAME AWS_SECRET_ACCESS_KEY AWS_ACCESS_KEY_ID ENABLE_REGISTRY DOCKER_USERNAME DOCKER_PASSWORD GH_TOKEN REGISTRY
ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH
ENV DAPPER_OUTPUT ./dist ./bin ./build
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_TARGET dapper
ENV DAPPER_RUN_ARGS "--privileged --network host -v /tmp:/tmp -v rke2-pkg:/go/pkg -v rke2-cache:/root/.cache/go-build -v trivy-cache:/root/.cache/trivy"
RUN if [ "${ARCH}" = "amd64" ] || [ "${ARCH}" = "arm64" ]; then \
        VERSION=0.56.10 OS=linux && \
        curl -sL "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${VERSION}/sonobuoy_${VERSION}_${OS}_${ARCH}.tar.gz" | \
        tar -xzf - -C /usr/local/bin; \
    fi

RUN curl -sL "https://github.com/cli/cli/releases/download/v2.53.0/gh_2.53.0_linux_${ARCH}.tar.gz" | \ 
    tar --strip-components=2 -xzvf - -C /usr/local/bin gh_2.53.0_linux_${ARCH}/bin/gh;

RUN curl -sL https://dl.k8s.io/release/$( \
    curl -sL https://dl.k8s.io/release/stable.txt \
    )/bin/linux/${ARCH}/kubectl -o /usr/local/bin/kubectl && \
    chmod a+x /usr/local/bin/kubectl

RUN python3 -m pip install awscli
RUN curl -sL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.55.2
RUN set -x && \
    apk --no-cache add \
    libarchive-tools \
    zstd \
    jq \
    python3 && \
    if [ "${ARCH}" != "s390x" ] || [ "${GOARCH}" != "arm64" ]; then \
    	apk add --no-cache rpm-dev; \
    fi

RUN GOCR_VERSION="v0.20.2" && \
    if [ "${ARCH}" = "arm64" ]; then \
        wget https://github.com/google/go-containerregistry/releases/download/${GOCR_VERSION}/go-containerregistry_Linux_arm64.tar.gz && \
        tar -zxvf go-containerregistry_Linux_arm64.tar.gz && \
        mv crane /usr/local/bin && \
        chmod a+x /usr/local/bin/crane; \
    else \
        wget https://github.com/google/go-containerregistry/releases/download/${GOCR_VERSION}/go-containerregistry_Linux_x86_64.tar.gz && \
        tar -zxvf go-containerregistry_Linux_x86_64.tar.gz && \
        mv crane /usr/local/bin && \
        chmod a+x /usr/local/bin/crane; \
    fi

WORKDIR /source

COPY --from=rpm-macros /usr/lib/rpm/macros.d/macros.systemd /usr/lib/rpm/macros.d
# End Dapper stuff

# Shell used for debugging
FROM dapper AS shell
RUN set -x && \
    apk --no-cache add \
    bash-completion \
    iptables \
    less \
    psmisc \
    rsync \
    socat \
    sudo \
    vim
# For integration tests
RUN go get github.com/onsi/ginkgo/v2 github.com/onsi/gomega/...
RUN GO111MODULE=off GOBIN=/usr/local/bin go get github.com/go-delve/delve/cmd/dlv
RUN echo 'alias abort="echo -e '\''q\ny\n'\'' | dlv connect :2345"' >> /root/.bashrc
ENV PATH=/var/lib/rancher/rke2/bin:$PATH
ENV KUBECONFIG=/etc/rancher/rke2/rke2.yaml
VOLUME /var/lib/rancher/rke2
# This makes it so we can run and debug k3s too
VOLUME /var/lib/rancher/k3s

FROM build AS charts
ARG CHART_REPO="https://rke2-charts.rancher.io"
ARG KUBERNETES_VERSION=""
ARG CACHEBUST="cachebust"
COPY charts/ /charts/
RUN echo ${CACHEBUST}>/dev/null
RUN /charts/build-charts.sh
RUN rm -vf /charts/*.sh /charts/*.md /charts/chart_versions.yaml

# rke2-runtime image
# This image includes any host level programs that we might need. All binaries
# must be placed in bin/ of the file image and subdirectories of bin/ will be flattened during installation.
# This means bin/foo/bar will become bin/bar when rke2 installs this to the host
FROM rancher/hardened-kubernetes:v1.34.2-rke2r1-build20251112 AS kubernetes
FROM rancher/hardened-containerd:v2.1.5-k3s1-build20251106 AS containerd
FROM rancher/hardened-crictl:v1.34.0-build20251017 AS crictl
FROM rancher/hardened-runc:v1.3.3-build20251105 AS runc

FROM scratch AS runtime-collect
COPY --from=runc \
    /usr/local/bin/runc \
    /bin/
COPY --from=crictl \
    /usr/local/bin/crictl \
    /bin/
COPY --from=containerd \
    /usr/local/bin/containerd \
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

FROM scratch AS runtime
LABEL org.opencontainers.image.url="https://hub.docker.com/r/rancher/rke2-runtime"
LABEL org.opencontainers.image.source="https://github.com/rancher/rke2"
COPY --from=runtime-collect / /

FROM ubuntu:24.04 AS test
ARG TARGETARCH
VOLUME /var/lib/rancher/rke2
VOLUME /var/lib/kubelet
VOLUME /var/lib/cni
VOLUME /var/log
COPY bin/rke2 /bin/
# use built air-gap images
COPY build/images/rke2-images.linux-amd64.tar.zst /var/lib/rancher/rke2/agent/images/
COPY build/images.txt /images.txt

# use rke2 bundled binaries
ENV PATH=/var/lib/rancher/rke2/bin:$PATH
# for kubectl
ENV KUBECONFIG=/etc/rancher/rke2/rke2.yaml
# for crictl
ENV CONTAINER_RUNTIME_ENDPOINT="unix:///run/k3s/containerd/containerd.sock"
# for ctr
RUN mkdir -p /run/containerd && \
    ln -s /run/k3s/containerd/containerd.sock /run/containerd/containerd.sock
# for go dns bug
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf
# for conformance testing
RUN chmod 1777 /tmp
RUN set -x && \
    export DEBIAN_FRONTEND=noninteractive && \
    apt-get -y update && \
    apt-get -y upgrade && \
    apt-get -y install \
    bash \
    bash-completion \
    ca-certificates \
    conntrack \
    ebtables \
    ethtool \
    iptables \
    jq \
    less \
    socat \
    vim 
ENTRYPOINT ["/bin/rke2"]
CMD ["server"]
