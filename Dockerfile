ARG KUBERNETES_VERSION=dev

# Base image for common build tools
FROM rancher/hardened-build-base:v1.24.13b1 AS base
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
        if [ "${ARCH}" = "amd64" ]; then \
            SONOBUOY_SHA256="c21a6e925d28e4d029a4a1d62bc222fdf4c8eb52aa94aa2482ae61cf2acd50f6"; \
        else \
            SONOBUOY_SHA256="a3de5c34a4feb46e1948484b543d6106a5cf8f069bddda1d043a443a4b8d7f26"; \
        fi && \
        cd /tmp && \
        curl -sL "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${VERSION}/sonobuoy_${VERSION}_${OS}_${ARCH}.tar.gz" -o sonobuoy.tar.gz && \
        echo "${SONOBUOY_SHA256}  sonobuoy.tar.gz" | sha256sum -c - && \
        tar -xzf sonobuoy.tar.gz -C /usr/local/bin && \
        rm -f /tmp/sonobuoy.tar.gz; \
    fi

RUN if [ "${ARCH}" = "amd64" ] || [ "${ARCH}" = "arm64" ]; then \
        GH_VERSION=2.53.0 && \
        if [ "${ARCH}" = "amd64" ]; then \
            GH_SHA256="ed2caf962730e0f593a2b6cae42a9b827b8a9c8bdd6efb56eae7feec38bdd0c6"; \
        else \
            GH_SHA256="22c4254025ef5acd7e5406a0eade879e868204861fcb3cd51a95a20cda5d221a"; \
        fi && \
        cd /tmp && \
        curl -sL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${ARCH}.tar.gz" -o gh.tar.gz && \
        echo "${GH_SHA256}  gh.tar.gz" | sha256sum -c - && \
        tar --strip-components=2 -xzf gh.tar.gz -C /usr/local/bin gh_${GH_VERSION}_linux_${ARCH}/bin/gh && \
        rm -f /tmp/gh.tar.gz; \
    fi

COPY channels.yaml /tmp/channels.yaml
RUN STABLE_VERSION=$(yq '.channels[] | select(.name == "stable") | .latest | sub("\+.*", "")' /tmp/channels.yaml) && \
    cd /tmp && \
    curl --retry 3 -sL https://dl.k8s.io/release/${STABLE_VERSION}/bin/linux/${ARCH}/kubectl -o kubectl && \
    curl --retry 3 -sL https://dl.k8s.io/release/${STABLE_VERSION}/bin/linux/${ARCH}/kubectl.sha256 -o kubectl.sha256 && \
    echo "$(cat kubectl.sha256)  kubectl" | sha256sum -c - && \
    install -m 0755 kubectl /usr/local/bin/kubectl && \
    rm -f /tmp/kubectl /tmp/kubectl.sha256 /tmp/channels.yaml

RUN GOLANGCI_VERSION=v1.55.2 && \
    case "${ARCH}" in \
        amd64) GOLANGCI_SHA256="ca21c961a33be3bc15e4292dc40c98c8dcc5463a7b6768a3afc123761630c09c" ;; \
        arm64) GOLANGCI_SHA256="8eb0cee9b1dbf0eaa49871798c7f8a5b35f2960c52d776a5f31eb7d886b92746" ;; \
        *) echo "Unsupported architecture for golangci-lint: ${ARCH}" && exit 1 ;; \
    esac && \
    cd /tmp && \
    curl -sL "https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_VERSION}/golangci-lint-${GOLANGCI_VERSION#v}-linux-${ARCH}.tar.gz" -o golangci-lint.tar.gz && \
    echo "${GOLANGCI_SHA256}  golangci-lint.tar.gz" | sha256sum -c - && \
    tar --strip-components=1 -xzf golangci-lint.tar.gz "golangci-lint-${GOLANGCI_VERSION#v}-linux-${ARCH}/golangci-lint" && \
    install -m 0755 golangci-lint /usr/local/bin/golangci-lint && \
    rm -f /tmp/golangci-lint /tmp/golangci-lint.tar.gz
    
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
        GOCR_ARCHIVE="go-containerregistry_Linux_arm64.tar.gz" && \
        GOCR_SHA256="b04ee6e4904d9219c76383f5b73521a63f69ecc93c0b1840846eebfd071a6355"; \
    else \
        GOCR_ARCHIVE="go-containerregistry_Linux_x86_64.tar.gz" && \
        GOCR_SHA256="8ef3564d264e6b5ca93f7b7f5652704c4dd29d33935aff6947dd5adefd05953e"; \
    fi && \
    cd /tmp && \
    wget -q https://github.com/google/go-containerregistry/releases/download/${GOCR_VERSION}/${GOCR_ARCHIVE} -O gocr.tar.gz && \
    echo "${GOCR_SHA256}  gocr.tar.gz" | sha256sum -c - && \
    tar -xzf gocr.tar.gz crane && \
    install -m 0755 crane /usr/local/bin/crane && \
    rm -f /tmp/gocr.tar.gz /tmp/crane

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
FROM rancher/hardened-kubernetes:v1.33.10-rke2r3-build20260407 AS kubernetes
FROM rancher/hardened-containerd:v2.2.2-k3s1-build20260312 AS containerd
FROM rancher/hardened-crictl:v1.33.0-build20260303 AS crictl
FROM rancher/hardened-runc:v1.4.1-build20260313 AS runc

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
