# Uninstall

## Tarball Method



## RPM Method

To uninstll RKE2 from your system, if installed with from RPM, the `rke2-uninstall.sh` and `rke2-killall.sh` scripts should be downloaded. These two scripts will stop any running containers and processes, clean up used processes, and ultimately remove RKE2 from the system. Run the commands below.

```bash
curl -sL https://raw.githubusercontent.com/rancher/rke2/488bab0f48b848e408ce399c32e7f5f73ce96129/bundle/bin/rke2-uninstall.sh
chmod +x rke2-uninstall.sh
```

```bash
curl -sL https://raw.githubusercontent.com/rancher/rke2/488bab0f48b848e408ce399c32e7f5f73ce96129/bundle/bin/rke2-killall.sh
chmod +x rke2-killall.sh
```

Now run the `rke2-uninstall.sh` script. This will call the `rke2-killall.sh` script which will 

```bash
./rke2-uninstall.sh
```
