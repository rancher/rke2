---
title: How to Use Flags and Environment Variables
---

Throughout the RKE2 documentation, you will see some options that can be passed in as both command flags and environment variables. The below examples show how these options can be passed in both ways.

### Example: RKE2_KUBECONFIG_MODE

The option to allow writing to the kubeconfig file is useful for allowing an RKE2 cluster to be imported into Rancher. Below are two ways to pass in the option.

Using the flag `--write-kubeconfig-mode 644`:

```bash
$ curl -sfL https://get.rke2.io | sh -s - --write-kubeconfig-mode 644
```
Using the environment variable `RKE2_KUBECONFIG_MODE`:

```bash
$ curl -sfL https://get.rke2.io | RKE2_KUBECONFIG_MODE="644" sh -s -
```
