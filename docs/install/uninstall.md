---
title: Uninstall
---

# Uninstall

This document explains how to fully uninstall RKE2 on a node, based on the installation method that was used.

## Tarball Method

To uninstall RKE2 from your system, simply run the command below. This will shutdown process, remove the RKE2 binary, and clean up files used by RKE2.

```bash
rke2-uninstall.sh
```

## RPM Method

To uninstll RKE2 from your system, if installed with from RPM, a few commands need to be ran. 

> **Note:** RPM based installs currently do not install the rke2-uninstall.sh script. This is a known issue that will be addressed in a future release. This document instructs you on how to download and use the necessary scripts.

```bash
yum remove -y 'rke2-*'
rm -rf /run/k3s
```

Once those commands are ran, the `rke2-uninstall.sh` and `rke2-killall.sh` scripts should be downloaded. These two scripts will stop any running containers and processes, clean up used processes, and ultimately remove RKE2 from the system. Run the commands below.

```bash
curl -sL https://raw.githubusercontent.com/rancher/rke2/488bab0f48b848e408ce399c32e7f5f73ce96129/bundle/bin/rke2-uninstall.sh --output rke2-uninstall.sh
chmod +x rke2-uninstall.sh
mv rke2-uninstall.sh /usr/local/bin
```

```bash
curl -sL https://raw.githubusercontent.com/rancher/rke2/488bab0f48b848e408ce399c32e7f5f73ce96129/bundle/bin/rke2-killall.sh --output rke2-killall.sh
chmod +x rke2-killall.sh
mv rke2-killall.sh /usr/local/bin
```

Now run the `rke2-uninstall.sh` script. This will call the `rke2-killall.sh` script which will 

```bash
rke2-uninstall.sh
```
