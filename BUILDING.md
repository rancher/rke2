# Building RKE2

## Prerequisites

By default, RKE2 is built with Dapper which uses Docker. To build RKE2 you will need to install these packages:
- bash
- docker
- gcc (CGO, don't ya know, if using `scripts/build` directly)
- go (check the `go.mod` for which series of go, e.g. 1.14.x, 1.15.x, etc)
- make

### Required for Running
When running RKE2 you will also need to install these packages:
- ca-certificates

## Building

```shell script
# this will build inside of a container via docker.
# use `make build` to leverage host-local tooling
make
```

## Local Docker Image Build

### Image Targets
- `build-env`: toolchain/build container (not runnable as an rke2 server)
- `runtime`: runtime bundle image (`rke2-runtime`)
- `test`: runnable local image that includes `/bin/rke2` and local airgap artifacts

### Changes Required For Local `test` Image Builds
- Fixed Dockerfile line continuation parse error in the `gh` install step.
- Converted legacy `ENV key value` syntax to `ENV key=value`.
- Updated the `test` stage airgap artifact path to use architecture-aware output:
  - `dist/artifacts/rke2-images.linux-${TARGETARCH}.tar.zst`
- Added compatibility handling in `scripts/build-local-test-image` so older refs that still
  expect `build/images/rke2-images.linux-amd64.tar.zst` can be built from non-amd64 hosts.

### One-Command Build (Recommended)
Build a runnable local image with versioned tags:

```shell
make build-local-test-image
```

By default this now tags:
- `rancher/rke2-test:<resolved-version>-<goos>-<arch>`
- `rancher/rke2-test:<resolved-version>`

Optional overrides:

```shell
IMAGE_TAG=my-rke2:test make build-local-test-image
TARGETARCH=amd64 make build-local-test-image
RKE2_REF=v1.34.4+rke2r1 make build-local-test-image
```

`RKE2_REF` builds from a git tag/branch in an isolated local clone, so older versions can be built without changing your current checkout.

### What The One-Command Build Does
1. Builds `bin/rke2` in Dockerized build env.
2. Builds runtime/images metadata in Dockerized build env.
3. Creates `dist/artifacts/rke2-images.linux-<arch>.tar.zst` from `build/images.txt`.
4. Builds Docker `--target test` and tags the result with versioned test image tags.

## Running

### rke2 (dev-shell)
To run locally in a container, there is a handy `make` target:
```shell script
make dev-shell
```

This will spin up a privileged container and setup the environment ready for you to invoke `./bin/rke2` at your leisure.
Since the `rancher/rke2-runtime` image was built locally and likely not yet pushed, this, along with the airgap images,
has been bind-mounted into the container ready to be imported into containerd on start-up.

### rke2 (generic)

To run the built artifact(s) locally or on a remote host:
- install prerequisites mentioned above
- copy `./bin/rke2` to the path on your host
- if not testing air-gapped, copy these (local) image tarballs to `/var/lib/rancher/rke2/agent/images/`:
  - `./build/images/rke2-runtime.tar`
  - `./build/images/rke2-kubernetes.tar`
- if testing air-gapped, copy this (local + remote) image tarball to `/var/lib/rancher/rke2/agent/images/`:
  - `./build/images/rke2-airgap.tar`
- run rke2 server: `rke2 server --token=test`

### kubectl

It isn't obvious but `kubectl` will be installed and ready to use after starting up `rke2`. To use it you will need to:
- `export KUBECONFIG=/etc/rancher/rke2/rke2.yaml`
- `export PATH=/var/lib/rancher/rke2/bin:$PATH`
