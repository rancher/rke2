ARG KUBERNETES_VERSION=dev

# Base image for common build tools
FROM rancher/hardened-build-base:v1.25.7b1 AS base
ARG BUILDARCH
ENV ARCH $BUILDARCH
RUN set -x && \
    apk --no-cache add \
    bash \
    curl \
    file \
    git \
    findutils \
    libseccomp-dev \
    rsync \
    gcc \
    bsd-compat-headers \
    aws-cli \
    pigz \
    tar \
    yq \
    helm

RUN if [ "${ARCH}" = "amd64" ]; then \
    	apk --no-cache add mingw-w64-gcc; \
    fi

FROM registry.suse.com/bci/bci-base AS rpm-macros
RUN zypper install -y systemd-rpm-macros

# Build environment
FROM base AS build-env
ARG BUILDARCH
ENV ARCH $BUILDARCH
RUN if [ "${ARCH}" = "amd64" ] || [ "${ARCH}" = "arm64" ]; then \
        VERSION=0.56.10 OS=linux && \
        curl -sL "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${VERSION}/sonobuoy_${VERSION}_${OS}_${ARCH}.tar.gz" | \
        tar -xzf - -C /usr/local/bin; \
    fi

RUN curl -sL "https://github.com/cli/cli/releases/download/v2.53.0/gh_2.53.0_linux_${ARCH}.tar.gz" | \ 
    tar --strip-components=2 -xzvf - -C /usr/local/bin gh_2.53.0_linux_${ARCH}/bin/gh;

COPY channels.yaml /tmp/channels.yaml
RUN STABLE_VERSION=$(yq '.channels[] | select(.name == "stable") | .latest | sub("\+.*", "")' /tmp/channels.yaml) && \
    curl --retry 3 -sL https://dl.k8s.io/release/${STABLE_VERSION}/bin/linux/${ARCH}/kubectl -o /usr/local/bin/kubectl && \
    chmod a+x /usr/local/bin/kubectl

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

RUN GOCR_VERSION="v0.20.7" && \
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
RUN git config --global --add safe.directory /source

COPY --from=rpm-macros /usr/lib/rpm/macros.d/macros.systemd /usr/lib/rpm/macros.d

# Shell used for debugging
FROM build-env AS shell
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

FROM base AS charts
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
FROM rancher/hardened-containerd:v2.1.5-k3s1-build20260109 AS containerd
FROM rancher/hardened-crictl:v1.35.0-build20251219 AS crictl
FROM rancher/hardened-runc:v1.4.0-build20251210 AS runc
FROM rancher/hardened-kubernetes:v1.35.2-rke2r1-build20260227 AS kubernetes

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
