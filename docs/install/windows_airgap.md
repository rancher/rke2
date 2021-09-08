# Windows Air-Gap Install
**Windows Support is currently Experimental as of v1.21.3+rke2r1**
**Windows Support requires choosing Calico as the CNI for the RKE2 cluster**

RKE2 Windows Agent (Worker) Nodes can be used in an air-gapped environment with two different methods. This requires first completing the RKE2 [airgap setup](airgap.md)

You can either deploy using the `rke2-windows-<BUILD_VERSION>-amd64-images.tar.gz` tarball release artifact, or by using a private registry. There are currently three tarball artifacts released for Windows in accordance with our validated [Windows versions](https://docs.rke2.io/install/requirements/#windows).

- rke2-windows-1809-amd64-images.tar.gz
- rke2-windows-2004-amd64-images.tar.gz
- rke2-windows-20H2-amd64-images.tar.gz

All files mentioned in the steps can be obtained from the assets of the desired released rke2 version [here](https://github.com/rancher/rke2/releases).

#### Prepare the Windows Agent Node
**Note** The Windows Server Containers feature needs to be enabled for the RKE2 agent to work.

Open a new Powershell window with Administrator privileges
```powershell
powershell -Command "Start-Process PowerShell -Verb RunAs"
```

In the new Powershell window, run the following command.
```powershell
Enable-WindowsOptionalFeature -Online -FeatureName containers –All
```
This will require a reboot for the `Containers` feature to properly function.

## Windows Tarball Method
    
1. Download the Windows images tarballs and binary from the RKE2 release artifacts list for the version of RKE2 that you are using.
        
    #### Using tar.gz image tarballs

    - **Windows Server 2019 LTSC (amd64) (OS Build 17763.2061)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-1809-amd64-images.tar.gz -OutFile /var/lib/rancher/rke2/agent/images/rke2-windows-1809-amd64-images.tar.gz 
    ```


    - **Windows Server SAC 2004 (amd64) (OS Build 19041.1110)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'  
    Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-2004-amd64-images.tar.gz -OutFile c:/var/lib/rancher/rke2/agent/images/rke2-windows-2004-amd64-images.tar.gz
    ```

    - **Windows Server SAC 20H2 (amd64) (OS Build 19042.1110)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'  
    Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-20H2-amd64-images.tar.gz -OutFile c:/var/lib/rancher/rke2/agent/images/rke2-windows-20H2-amd64-images.tar.gz 
    ```

    #### Using tar.zst image tarballs

    - **Windows Server 2019 LTSC (amd64) (OS Build 17763.2061)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'  
    Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-1809-amd64-images.tar.zst -OutFile /var/lib/rancher/rke2/agent/images/rke2-windows-1809-amd64-images.tar.zst 
    ```


    - **Windows Server SAC 2004 (amd64) (OS Build 19041.1110)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'  
    Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-2004-amd64-images.tar.zst -OutFile c:/var/lib/rancher/rke2/agent/images/rke2-windows-2004-amd64-images.tar.zst 
    ```

    - **Windows Server SAC 20H2 (amd64) (OS Build 19042.1110)**

    ``` powershell
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest hhttps://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-20H2-amd64-images.tar.zst -OutFile c:/var/lib/rancher/rke2/agent/images/rke2-windows-20H2-amd64-images.tar.zst
    ```

    - Use `rke2-windows-<BUILD_VERSION>-amd64.tar.gz` or `rke2-windows-<BUILD_VERSION>-amd64.tar.zst`. Zstandard offers better compression ratios and faster decompression speeds compared to pigz.

2. Ensure that the `/var/lib/rancher/rke2/agent/images/` directory exists on the node.

    ```powershell
    New-Item -Type Directory c:\usr\local\bin -Force
    New-Item -Type Directory c:\var\lib\rancher\rke2\bin -Force
    ```

3. Copy the compressed archive to `/var/lib/rancher/rke2/agent/images/` on the node, ensuring that the file extension is retained.

4. [Install RKE2](#install-windows-rke2)

## Private Registry Method
As of RKE2 v1.20, private registry support honors all settings from the [containerd registry configuration](containerd_registry_configuration.md). This includes endpoint override and transport protocol (HTTP/HTTPS), authentication, certificate verification, etc.

Prior to RKE2 v1.20, private registries must use TLS, with a cert trusted by the host CA bundle. If the registry is using a self-signed cert, you can add the cert to the host CA bundle with `update-ca-certificates`. The registry must also allow anonymous (unauthenticated) access.

1. Add all the required system images to your private registry. A list of images can be obtained from the `.txt` file corresponding to each tarball referenced above, or you may `docker load` the airgap image tarballs, then tag and push the loaded images.
2. If using a private or self-signed certificate on the registry, add the registry's CA cert to the containerd registry configuration, or operating system's trusted certs for releases prior to v1.20.
3. [Install RKE2](#install-windows-rke2) using the `system-default-registry` parameter, or use the [containerd registry configuration](containerd_registry_configuration.md) to use your registry as a mirror for docker.io.

## Install Windows RKE2

These steps should only be performed after completing one of either the [Tarball Method](#windows-tarball-method) or [Private Registry Method](#private-registry-method).

1. Obtain the Windows RKE2 binary file `rke2-windows-amd64.exe`. Ensure the binary is named `rke2.exe` and place it in `c:/usr/local/bin`. 
```powershell
Invoke-WebRequest https://github.com/rancher/rke2/releases/download/v1.21.4%2Brke2r2/rke2-windows-amd64.exe -OutFile c:/usr/local/bin/rke2.exe
```

2. Configure the rke2-agent for Windows
```powershell
New-Item -Type Directory c:/etc/rancher/rke2 -Force
Set-Content -Path c:/etc/rancher/rke2/config.yaml -Value @"
server: https://<server>:9345
token: <token from server node>
"@
```

To read more about the config.yaml file, see the [Install Options documentation.](./install_options/install_options.md#configuration-file)

3. Configure your PATH
```powershell
$env:PATH+=";c:\var\lib\rancher\rke2\bin;c:\usr\local\bin"

[Environment]::SetEnvironmentVariable(
    "Path",
    [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";c:\var\lib\rancher\rke2\bin;c:\usr\local\bin",
    [EnvironmentVariableTarget]::Machine)
```

4. Start the RKE2 Windows service by running the binary with the desired parameters. Please see the [Windows Agent Configuration reference](install_options/windows_agent_config.md) for additional parameters.  

```powershell
c:\usr\local\bin\rke2.exe agent service --add
```

For example, if using the Private Registry Method, your config file would have the following:
```yaml
system-default-registry: "registry.example.com:5000"
```

**Note:** The `system-default-registry` parameter must specify only valid RFC 3986 URI authorities, i.e. a host and optional port.

If you would prefer to use CLI parameters only instead, run the binary with the desired parameters. 

```powershell
c:/usr/local/bin/rke2.exe agent --token <> --server <>
```
