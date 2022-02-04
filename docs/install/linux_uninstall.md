---
title: Linux Uninstall
---

# Linux Uninstall

> **Note:**  Uninstalling RKE2 deletes the cluster data and all of the scripts.

Depending on the method used to install RKE2, the uninstallation process varies.

## RPM Method

To uninstall RKE2 installed via the RPM method from your system, simply run the commands corresponding to the version of RKE2 you have installed, either as the root user or through `sudo`. This will shutdown RKE2 process, remove the RKE2 RPMs, and clean up files used by RKE2.

=== "RKE2 v1.18.13+rke2r1 and newer"
    Starting with RKE2 `v1.18.13+rke2r1`, the bundled `rke2-uninstall.sh` script will remove the corresponding RPM packages during the uninstall process. Simply run the following command:

    ```bash
    /usr/bin/rke2-uninstall.sh
    ```

=== "RKE2 Prior to v1.18.13+rke2r1"
    If you are running a version of RKE2 that is older than `v1.18.13+rke2r1`, you will need to manually remove the RKE2 RPMs after calling the `rke2-uninstall.sh` script.

    ```sh
    /usr/bin/rke2-uninstall.sh
    yum remove -y 'rke2-*'
    rm -rf /run/k3s
    ```

=== "RKE2 Prior to v1.18.11+rke2r1"
    RPM based installs older than and including `v1.18.10+rke2r1` did not package the `rke2-uninstall.sh` script. These instructions provide guidance on how to download and use the necessary scripts.

    First, remove the corresponding RKE2 packages and `/run/k3s` directory.

    ```bash
    yum remove -y 'rke2-*'
    rm -rf /run/k3s
    ```

    Once those commands are run, the rke2-uninstall.sh and rke2-killall.sh scripts should be downloaded. These two scripts will stop any running containers and processes, clean up used processes, and ultimately remove RKE2 from the system. Run the commands below.

    ```bash
    curl -sL https://raw.githubusercontent.com/rancher/rke2/master/bundle/bin/rke2-uninstall.sh --output rke2-uninstall.sh
    chmod +x rke2-uninstall.sh
    mv rke2-uninstall.sh /usr/local/bin

    curl -sL https://raw.githubusercontent.com/rancher/rke2/master/bundle/bin/rke2-killall.sh --output rke2-killall.sh
    chmod +x rke2-killall.sh
    mv rke2-killall.sh /usr/local/bin

    ```

    Now run the rke2-uninstall.sh script. This will also call the rke2-killall.sh.
    
    ```bash
    /usr/local/bin/rke2-uninstall.sh
    ```

## Tarball Method

To uninstall RKE2 installed via the Tarball method from your system, simply run the command below. This will shutdown process, remove the RKE2 binary, and clean up files used by RKE2.

```bash
/usr/local/bin/rke2-uninstall.sh
```
