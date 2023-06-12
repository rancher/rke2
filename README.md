# RKE2
![RKE2](https://docs.rke2.io/img/logo-horizontal-rke2.svg)

RKE2, also known as RKE Government, is Rancher's next-generation Kubernetes distribution.

It is a fully [conformant Kubernetes distribution](https://landscape.cncf.io/?selected=rke-government) that focuses on security and compliance within the U.S. Federal Government sector.

To meet these goals, RKE2 does the following:

- Provides [defaults and configuration options](https://docs.rke2.io/security/hardening_guide/) that allow clusters to pass the [CIS Kubernetes Benchmark](https://docs.rke2.io/security/cis_self_assessment123/) with minimal operator intervention
- Enables [FIPS 140-2 compliance](https://docs.rke2.io/security/fips_support/)
- Supports SELinux policy and [Multi-Category Security (MCS)](https://selinuxproject.org/page/NB_MLS) label enforcement
- Regularly scans components for CVEs using [trivy](https://github.com/aquasecurity/trivy) in our build pipeline

For more information and detailed installation and operation instructions, [please visit our docs](https://docs.rke2.io/).

## Quick Start
Here's the ***extremely*** quick start:
```sh
curl -sfL https://get.rke2.io | sh -
systemctl enable rke2-server.service
systemctl start rke2-server.service
# Wait a bit
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml PATH=$PATH:/var/lib/rancher/rke2/bin
kubectl get nodes
```
For a bit more, [check out our full quick start guide](https://docs.rke2.io/install/quickstart/).

## Installation

A full breakdown of installation methods and information can be found [here](https://docs.rke2.io/install/methods/).

## Configuration File

The primary way to configure RKE2 is through its [config file](https://docs.rke2.io/install/configuration#configuration-file). Command line arguments and environment variables are also available, but RKE2 is installed as a systemd service and thus these are not as easy to leverage.

By default, RKE2 will launch with the values present in the YAML file located at `/etc/rancher/rke2/config.yaml`.

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

In general, cli arguments map to their respective yaml key, with repeatable cli args being represented as yaml lists. So, an identical configuration using solely cli arguments is shown below to demonstrate this:

```bash
rke2 server \
  --write-kubeconfig-mode "0644"    \
  --tls-san "foo.local"             \
  --node-label "foo=bar"            \
  --node-label "something=amazing"
```

It is also possible to use both a configuration file and cli arguments.  In these situations, values will be loaded from both sources, but cli arguments will take precedence.  For repeatable arguments such as `--node-label`, the cli arguments will overwrite all values in the list.

Finally, the location of the config file can be changed either through the cli argument `--config FILE, -c FILE`, or the environment variable `$RKE2_CONFIG_FILE`.

## FAQ

- [How is this different from RKE1 or K3s?](https://docs.rke2.io/#how-is-this-different-from-rke-or-k3s)
- [Why two names?](https://docs.rke2.io/#why-two-names)

## Security

Security issues in RKE2 can be reported by sending an email to [security@rancher.com](mailto:security@rancher.com). Please do not open security issues here.
