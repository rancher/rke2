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
