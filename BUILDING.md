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
- libseccomp (libseccomp2 on Debian/Ubuntu)
- ca-certificates

## Building

```shell script
# for non air-gap testing
make build image
# for air-gap testing
make build-airgap
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
- copy `./build/images/rke2-runtime.tar` to `/var/lib/rancher/rke2/agent/images/` on your host
- if testing airgap, also copy `./build/images/rke2-airgap.tar` to `/var/lib/rancher/rke2/agent/images/` on your host
- run rke2 server: `rke2 server --token=test`

### kubectl

It isn't obvious but `kubectl` will be installed and ready to use after starting up `rke2`. To use it you will need to:
- `export KUBECONFIG=/etc/rancher/rke2/rke2.yaml`
- `export PATH="$(ls -td /var/lib/rancher/rke2/data/*/bin | head -n 1):$PATH"`
