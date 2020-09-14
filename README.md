# RKE2

We're starting to experiment with our next-generation Kubernetes distribution. This is currently an alpha project that will evolve a lot. Stay tuned.

## What is this?

RKE2 is a fully compliant Kubernetes distribution, with the following changes:

- Packaged as a single binary.
- Kubelet running as an embedded process in the agent and server.
- Master components running as static pods.
- All components images are compiled using Goboring library.
- The following addons are installed as helm charts:
  - CoreDNS
  - Kube-proxy
  - Canal
  - Nginx Ingress Controller
  - Metrics Server

## Installation

RKE2 comes with a convenient installation script that will, by default, install via `yum` on RPM-based systems while unpacking the tarball bundle to `/usr/local` on all others.

### Quick Start

To install via the default mechanism for your system:

```sh
curl -fsSL https://raw.githubusercontent.com/rancher/rke2/master/install.sh | sudo sh -
```

To install via unpacking the tarball on an RPM-based system:
```sh
curl -fsSL https://raw.githubusercontent.com/rancher/rke2/master/install.sh --output install.sh
sudo INSTALL_RKE2_METHOD=tar sh ./install.sh
```

To install a specific version:
```sh
sudo INSTALL_RKE2_METHOD=tar INSTALL_RKE2_VERSION='v1.18.8-beta19+rke2' sh ./install.sh
```

## Configuration

### CIS Mode

If you plan to run your RKE2 cluster in CIS mode you will want to:

- apply these sysctls (at [/usr/local/share/rke2/rke2-cis-sysctl.conf](bundle/share/rke2/rke2-cis-sysctl.conf) for tar installations):
```
vm.panic_on_oom=0
vm.overcommit_memory=1
kernel.keys.root_maxbytes=25000000
kernel.panic=10
kernel.panic_on_oops=1
```
- create an `rke2` and `etcd` user
```
useradd -d /var/lib/rancher/rke2 -r -s /bin/false rke2
useradd -d /var/lib/rancher/rke2/server/db -r -s /bin/false etcd
usermod -aG etcd rke2
```

## Operation

To start using the rke2 cluster:

```sh
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml PATH=$PATH:/var/lib/rancher/rke2/bin
kubectl get nodes
```

`RKE2_TOKEN` is created at `/var/lib/rancher/rke2/server/node-token` on your server. To install on worker nodes we should pass `RKE2_URL` along with `RKE2_TOKEN` or `RKE2_CLUSTER_SECRET` environment variables, for example:

```sh
RKE2_URL=https://myserver:6443 RKE2_TOKEN=XXX ./install.sh
```

## Automated deployment

We provide a simple automated way to install RKE2 on AWS via terraform scripts, this method requires terraform to be installed and access to AWS cloud, to get started please checkout the [rke2-build](https://github.com/rancher/rke2-build) repo.

## RPM Repositories

Signed RPMs are published for RKE2 within the `rpm.rancher.io` RPM repository. In order to use the RPM repository, on a CentOS 7 or RHEL 7 system, run the following bash snippet:

```bash
cat << EOF > /etc/yum.repos.d/rancher-rke2-1-18-testing.repo
[rancher-rke2-common-testing]
name=Rancher RKE2 Common Latest
baseurl=https://rpm.rancher.io/rke2/testing/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key

[rancher-rke2-1-18-testing]
name=Rancher RKE2 1.18 Latest
baseurl=https://rpm.rancher.io/rke2/testing/1.18/centos/7/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
```

After this, you can either run:

```bash
yum -y install rke2-server
```

or 

```bash
yum -y install rke2-agent
```

The RPM will install a corresponding `rke2-server.service` or `rke2-agent.service` systemd unit that can be invoked like: `systemctl start rke2-server`. Make sure that you configure `rke2` before you start it, by following the `Configuration File` instructions below.

## Configuration File

In addition to configuring RKE2 with environment variables and cli arguments, RKE2 can also use a config file.

By default, values present in a `yaml` file located at `/etc/rancher/rke2/config.yaml` will be used on install.

An example of a basic `server` config file is below:

```yaml
# /etc/rancher/rke2/config.yaml
write-kubeconfig-mode: "0644"
tls-san:
  - "foo.local"
node-label:
  - "foo=bar"
  - "something=amazing"
```

In general, cli arguments map to their respective yaml key, with repeatable cli args being represented as yaml lists.

An identical configuration using solely cli arguments is shown below to demonstrate this:

```bash
rke2 server \
  --write-kubeconfig-mode "0644"    \
  --tls-san "foo.local"             \
  --node-label "foo=bar"            \
  --node-label "something=amazing"
```

It is also possible to use both a configuration file and cli arguments.  In these situations, values will be loaded from both sources, but cli arguments will take precedence.  For repeatable arguments such as `--node-label`, the cli arguments will overwrite all values in the list.

Finally, the location of the config file can be changed either through the cli argument `--config FILE, -c FILE`, or the environment variable `$RKE2_CONFIG_FILE`.
