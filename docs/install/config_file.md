---
title: Configuration File
---

At runtime, RKE2 will check for the presence of `/etc/rancher/rke2/config.yaml`. The config.yaml file can be utilized to specify CLI arguments.

An example of a basic `server` config file is below:

```yaml
# config.yaml
write-kubeconfig-mode: "0644"
tls-san:
  - "foo.local"
node-label:
  - "foo=bar"
  - "something=amazing"
```

In general, CLI arguments map to their respective YAML key, with repeatable CLI arguments being represented as YAML lists.

An identical configuration using solely CLI arguments is shown below to demonstrate this:

```bash
# bash
rke2 server \
  --write-kubeconfig-mode "0644"    \
  --tls-san "foo.local"             \
  --node-label "foo=bar"            \
  --node-label "something=amazing"
```

It is also possible to use both a configuration file and CLI arguments.  In these situations, values will be loaded from both sources, but CLI arguments will take precedence.  For repeatable arguments such as `--node-label`, the CLI arguments will overwrite all values in the list.

Finally, the location of the config file can be changed either through the cli argument `--config FILE, -c FILE`, or the environment variable `$RKE2_CONFIG_FILE`.

