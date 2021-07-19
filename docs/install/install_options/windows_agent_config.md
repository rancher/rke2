---
title: Windows Agent Configuration Reference
---

This is a reference to all parameters that can be used to configure the Windows RKE2 agent.

### Windows RKE2 Agent CLI Help

```console
NAME:
   rke2.exe agent - Run Windows node agent

USAGE:
   rke2.exe agent [OPTIONS]

OPTIONS:
--Channel           Channel to use for fetching RKE2 download URL (Default: "stable")
--Method            The installation method to use. Currently tar or choco installation supported. (Default: "tar")
--Type              Type of RKE2 service. Only the "agent" type is supported on Windows. (Default: "agent")
--Version           Version of rke2 to download from Github
--TarPrefix         Installation prefix when using the tar installation method. (Default: `C:/usr/local` unless `C:/usr/local` is read-only or has a dedicated mount point, in which case `C:/opt/rke2` is used instead)
--Commit            (experimental/agent) Commit of RKE2 to download from temporary cloud storage. If set, this forces `--Method=tar`. Intended for development purposes only.
--AgentImagesDir    Installation path for airgap images when installing from CI commit. (Default: `C:/var/lib/rancher/rke2/agent/images`)
--ArtifactPath      If set, the install script will use the local path for sourcing the `rke2.windows-$SUFFIX` and `sha256sum-$ARCH.txt` files rather than the downloading the files from the Internet. Disabled by default.
```
