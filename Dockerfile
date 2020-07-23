ARG KUBERNETES_VERSION=dev
# Build environment
FROM ranchertest/build-base:v1.14.2 AS build
# Yep nothing special here yet

# Shell used for debugging
FROM build AS shell
RUN apt-get update && \
    apt-get install -y iptables socat vim bash-completion less psmisc libseccomp-dev sudo
RUN cp -f /etc/skel/.bashrc /etc/skel/.profile /root/ && \
    echo 'alias abort="echo -e '\''q\ny\n'\'' | ./bin/dlv connect :2345"' >> /root/.bashrc
ENV PATH=/var/lib/rancher/rke2/bin:$PATH
ENV KUBECONFIG=/etc/rancher/rke2/rke2.yaml
VOLUME /var/lib/rancher/rke2
# This makes it so we can run and debug k3s too
VOLUME /var/lib/rancher/k3s

# Dapper/Drone/CI environment
FROM build AS dapper
ENV DAPPER_ENV REPO TAG DRONE_TAG PAT_USERNAME PAT_TOKEN
ENV DAPPER_OUTPUT ./dist ./bin ./build
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_TARGET dapper
ENV DAPPER_RUN_ARGS "-v rke2-pkg:/go/pkg -v rke2-cache:/root/.cache/go-build"
WORKDIR /source
# End Dapper stuff

# rke-runtime image
# This image includes any host level programs that we might need. All binaries
# must be placed in bin/ of the file image and subdirectories of bin/ will be flattened during installation.
# This means bin/foo/bar will become bin/bar when rke2 installs this to the host
FROM ranchertest/kubernetes:${KUBERNETES_VERSION} AS k8s
FROM ranchertest/runc:v1.0.0-rc10 AS runc
FROM ranchertest/containerd:v1.3.3 AS containerd

FROM scratch AS release
COPY --from=k8s \
    /usr/local/bin/kubectl \
    /usr/local/bin/kubelet \
    /bin/
COPY --from=runc \
    /usr/local/bin/runc \
    /bin/
COPY --from=containerd \
    /usr/local/bin/containerd-shim-runc-v2 \
    /usr/local/bin/containerd \
    /usr/local/bin/containerd-shim \
    /usr/local/bin/ctr \
    /usr/local/bin/containerd-shim-runc-v1 \
    /bin/
COPY ./build/static/charts /charts
