ARG KUBERNETES_VERSION=dev

# Build environment
FROM rancher/hardened-build-base:v1.18.1b7 AS build
ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH
RUN set -x \
    && apk --no-cache add \
    bash \
    curl \
    file \
    git \
    libseccomp-dev \
    rsync \
    gcc \
    bsd-compat-headers \
    py-pip \
    pigz \
    tar \
    yq

RUN if [ "${ARCH}" != "s390x" ]; then \
    	apk --no-cache add mingw-w64-gcc; \
    fi

FROM registry.suse.com/bci/bci-base AS rpm-macros
RUN zypper install -y systemd-rpm-macros

# Dapper/Drone/CI environment
FROM build AS dapper
ENV DAPPER_ENV GODEBUG REPO TAG DRONE_TAG PAT_USERNAME PAT_TOKEN KUBERNETES_VERSION DOCKER_BUILDKIT DRONE_BUILD_EVENT IMAGE_NAME GCLOUD_AUTH ENABLE_REGISTRY
ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH
ENV DAPPER_OUTPUT ./dist ./bin ./build
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_TARGET dapper
ENV DAPPER_RUN_ARGS "--privileged --network host -v /tmp:/tmp -v rke2-pkg:/go/pkg -v rke2-cache:/root/.cache/go-build -v trivy-cache:/root/.cache/trivy"
RUN if [ "${ARCH}" = "amd64" ] || [ "${ARCH}" = "arm64" ]; then \
    VERSION=0.50.0 OS=linux && \
    curl -sL "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${VERSION}/sonobuoy_${VERSION}_${OS}_${ARCH}.tar.gz" | \
    tar -xzf - -C /usr/local/bin; \
    fi
RUN curl -sL https://storage.googleapis.com/kubernetes-release/release/$( \
    curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt \
    )/bin/linux/${ARCH}/kubectl -o /usr/local/bin/kubectl && \
    chmod a+x /usr/local/bin/kubectl; \
    pip install codespell

RUN curl -sL https://raw.githubusercontent.com/golangci/golangci-lint/v1.45.2/install.sh | sh -s;
RUN set -x \
    && apk --no-cache add \
    libarchive-tools \
    zstd \
    jq \
    python2 \
    \
    && if [ "${ARCH}" != "s390x" ]; then \
    	apk add --no-cache rpm-dev; \
    fi

RUN GOCR_VERSION="v0.5.1" && \
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

RUN VERSION=0.16.0 && \
    if [ "${ARCH}" = "arm64" ]; then \
    wget https://github.com/aquasecurity/trivy/releases/download/v${VERSION}/trivy_${VERSION}_Linux-ARM64.tar.gz && \
    tar -zxvf trivy_${VERSION}_Linux-ARM64.tar.gz && \
    mv trivy /usr/local/bin; \
    else \
    wget https://github.com/aquasecurity/trivy/releases/download/v${VERSION}/trivy_${VERSION}_Linux-64bit.tar.gz && \
    tar -zxvf trivy_${VERSION}_Linux-64bit.tar.gz && \
    mv trivy /usr/local/bin; \
    fi
WORKDIR /source

COPY --from=rpm-macros /usr/lib/rpm/macros.d/macros.systemd /usr/lib/rpm/macros.d
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
RUN CHART_VERSION="1.11.502"                  CHART_FILE=/charts/rke2-cilium.yaml         CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="v3.22.2-build2022050902"   CHART_FILE=/charts/rke2-canal.yaml          CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="v3.23.103"                 CHART_FILE=/charts/rke2-calico.yaml         CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="v3.23.103"                 CHART_FILE=/charts/rke2-calico-crd.yaml     CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="1.19.400"                  CHART_FILE=/charts/rke2-coredns.yaml        CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="4.1.004"                   CHART_FILE=/charts/rke2-ingress-nginx.yaml  CHART_BOOTSTRAP=false  /charts/build-chart.sh
RUN CHART_VERSION="2.11.100-build2021111904"  CHART_FILE=/charts/rke2-metrics-server.yaml CHART_BOOTSTRAP=false  /charts/build-chart.sh
RUN CHART_VERSION="v3.8-build2021110403"      CHART_FILE=/charts/rke2-multus.yaml         CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="1.2.201"                   CHART_FILE=/charts/rancher-vsphere-cpi.yaml CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="2.5.1-rancher101"          CHART_FILE=/charts/rancher-vsphere-csi.yaml CHART_BOOTSTRAP=true   /charts/build-chart.sh
RUN CHART_VERSION="0.1.1300"                  CHART_FILE=/charts/harvester-cloud-provider.yaml CHART_BOOTSTRAP=true /charts/build-chart.sh
RUN CHART_VERSION="0.1.1100"                  CHART_FILE=/charts/harvester-csi-driver.yaml     CHART_BOOTSTRAP=true /charts/build-chart.sh
RUN rm -vf /charts/*.sh /charts/*.md

# rke-runtime image
# This image includes any host level programs that we might need. All binaries
# must be placed in bin/ of the file image and subdirectories of bin/ will be flattened during installation.
# This means bin/foo/bar will become bin/bar when rke2 installs this to the host
FROM rancher/hardened-kubernetes:v1.24.2-rke2r1-build20220617 AS kubernetes
FROM rancher/hardened-containerd:v1.6.6-k3s1-build20220606 AS containerd
FROM rancher/hardened-crictl:v1.24.0-build20220506 AS crictl
FROM rancher/hardened-runc:v1.1.2-build20220606 AS runc

FROM scratch AS runtime-collect
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

FROM scratch AS runtime
COPY --from=runtime-collect / /

FROM ubuntu:20.04 AS test
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
RUN mkdir -p /run/containerd \
    &&  ln -s /run/k3s/containerd/containerd.sock /run/containerd/containerd.sock
# for go dns bug
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf
# for conformance testing
RUN chmod 1777 /tmp
RUN set -x \
    && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y update \
    && apt-get -y upgrade \
    && apt-get -y install \
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
