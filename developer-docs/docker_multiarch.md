# Context
We build [many upstream components](https://github.com/rancher/ecm-distro-tools/issues/375) as Docker images as part of rke2 and we now use multi-arch images that include both amd64 and arm64 layers in the same image.

During the migration of these builds from Drone to Github Actions, we noticed that the arm64 build is very slow due to the fact that most Dockerfiles use qemu emulation to build the arm64 version of the image.
Using multi-arch best practices as documented by Docker would allow us to use cross-compilation instead of emulation for most of the process and would greatly reduce the build time.

For example, with `image-build-coredns`, the build time for the arm64 image goes from 14mn to 2mn (similar to amd64) when using cross-compilation instead of emulation.

# How does it work?
When building a multi-arch image with `docker buildx`, some parts of the Dockerfile instructions are run on the native architecture where docker is running
and others are run on the target architecture of the image through qemu emulation.
In our case, we build on amd64 runners and we target both amd64 and arm64 as destination architectures.

Docker sets [environment variables](https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope) to define the native build platform (`BUILDPLATFORM`) and the platform for which we are building the image (`TARGETPLATFORM`).

Our goal is to run as much as possible of the build process on the native platform to reduce the build time.

## Running the build on the native platform
We can force the build to run on the native architecture of the runner thanks to the `--from` parameter of the `FROM` instruction.
This will looks like this:
```Dockerfile
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} as base-builder
```

All the Docker instructions in this build stage will run natively on the `$BUILDPLATFORM` platform.

```Dockerfile
ARG TARGETPLATFORM
```

After this line, if we build a multi-arch image, docker will create a fork of the build process for each target platform.
Each fork runs with a different value of `TARGETPLATFORM`.
This happens for example when using 
```yaml
platforms: linux/amd64, linux/arm64
```
with the `docker/build-push-action@v5` Github Action.

By contrast, if we run the build with 
```Dockerfile
FROM ${GO_IMAGE} as base-builder
```
the whole build is forked and emulated for each `TARGETPLATFORM` which is much slower.
See https://docs.docker.com/build/guide/multi-platform/#build-using-emulation for more details.

## Cross-compilation
# Helper scripts
Docker provides an image called `tonistiigi/xx` which contains scripts to help cross-compiling multi-arch images.
We mirror this image as `rancher/mirrored-tonistiigi-xx` for use in rancher-related images.

These scripts include:
* `xx-apk` to install Alpine packages for the target architecture instead of the native one
* `xx-verify` to check that the final binary was compiled for the right arch
* `xx-info` to get information about the build context like the os, arch or libc version
* `xx-go` to help setup Go cross-compilation by properly setting the variables like `GOOS`, `GOARCH`...

The full documentation for these scripts is available [here](https://github.com/tonistiigi/xx).

Note that the doc recommends explicitly setting the `CGO_ENABLED` variable which is done in `go-build-static.sh`.

# Set-up
```Dockerfile
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} as base-builder
# copy xx scripts to your build stage
COPY --from=xx / /
RUN apk add file make git clang lld
ARG TARGETPLATFORM
# setup required packages
RUN set -x && \
    xx-apk --no-cache add musl-dev gcc lld 
```

In order to cross-compile with Go with `CGO_ENABLED=1`, we need to
* copy the helper script to the build stage
* install packages for the native arch with `apk`
* install packages for the target arch with `xx-apk`

We can then use `base-builder` as base to create our builder.

# Compiling
In the `coredns-builder` stage, we do as much work as possible before the line
```Dockerfile
ARG TARGETPLATFORM
```
to reduce the overhead of the build.
This includes cloning the Git repository and using `go mod download` to perform the potentially network-heavy operations only once.

Then we simply need to add `xx-go --wrap && \` when calling `go-build-static.sh`:
```Dockerfile
RUN xx-go --wrap && \
    GO_LDFLAGS="-linkmode=external -X ${PKG}/coremain.GitCommit=$(git rev-parse --short HEAD)" \
    go-build-static.sh -gcflags=-trimpath=${GOPATH}/src -o bin/coredns .
```

# Stripping the final binary
In the current builds, we use
```Dockerfile
RUN install -s bin/* /usr/local/bin
```
to strip the binary and reduce the final image size.
However, stripping a cross-compiled binary creates issues so the simplest way is to strip the binary in a different stage runningin the target arch:
```Dockerfile
FROM ${GO_IMAGE} as strip_binary
#strip needs to run on TARGETPLATFORM, not BUILDPLATFORM
COPY --from=coredns-builder /usr/local/bin/coredns /coredns
RUN strip /coredns
```
and then copy the resulting file to the final image.