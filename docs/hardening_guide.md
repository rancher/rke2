# Hardening Guide

This document provides prescriptive guidance for hardening a production installation of RKE2. It outlines the configurations and controls required to address Kubernetes benchmark controls from the Center for Information Security (CIS).

If RKE2 is run with the `profile` flag set to "cis-1.5", it performs a number of system checks and applies a restrictive NetworkPolicy and PodSecurityPolicy. 

RKE2 checks for 4 sysctl kernel parameters (see below) as well as the existence of the "etcd" user. If any of the sysctl kernel parameters aren't set to the expected value or the "etcd" users doesn't exist, RKE2 will immediately cease operation. The kubelet flag `protect-kernel-parameters` is also set.

The restrictive NetworkPolicy allows for only traffic between pods in the same namespace with the exception of DNS. This restrictive policy only applies to the built-in namespaces: `kube-system`, `kube-public`, and `default` namespaces. Operators must manage network policies as normal for additional namespace.

The restrictive PodSecurityPolicy addresses CIS controls defined in section 5.2. More details can be found below.

## Overview

For more detail about evaluating a hardened cluster against the official CIS benchmark, refer to the [CIS Benchmark Rancher Self-Assessment Guide](cis_self_assessment.md).

## Configure Kernel Runtime Parameters

The following sysctl configuration is recommended for all nodes type in the cluster. Set the following parameters in `/etc/sysctl.d/90-kubelet.conf`.

```sh
vm.overcommit_memory=1
vm.panic_on_oom=0
kernel.panic=10
kernel.panic_on_oops=1
```

If tarball installation method used, run:

`sysctl -p /usr/local/share/rke2/rke2-cis-sysctl.conf`

This file should be copied to `/etc/sysctl.d`.

Here is a simple shell script that will configure your system so RKE2 will run with the `--profile=cis-1.5` argument.

```bash
#!/bin/sh

set -e

echo "CIS Setup Starting..."
echo "Creating etcd user and group"

useradd -r -c "etcd user" -s /sbin/nologin -M etcd

echo "Setting kernel parameters..."

sysctl -w vm.panic_on_oom=0
sysctl -w vm.overcommit_memory=1
sysctl -w kernel.panic=10
sysctl -w kernel.panic_on_oops=1

echo "CIS Setup Complete!

exit 0
```
