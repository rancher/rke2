---
title: Windows Uninstall
---

# Windows Uninstall

> **Note:**  Uninstalling the RKE2 Windows Agent deletes all of the node data.

Depending on the method used to install RKE2, the uninstallation process varies.

## Tarball Method

To uninstall the RKE2 Windows Agent installed via the tarball method from your system, simply run the command below. This will shutdown all RKE2 Windows processes, remove the RKE2 Windows binary, and clean up the files used by RKE2.

```powershell
c:/usr/local/bin/rke2-uninstall.ps1
```
