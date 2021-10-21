#Requires -RunAsAdministrator
<#
.SYNOPSIS 
    Installs Rancher RKE2 to create Windows Worker Nodes.
.DESCRIPTION 
    Run the script to install all Rancher RKE2 related needs. (kubernetes, containerd, network)
.NOTES
    Environment variables:
      System Agent Variables
      - CATTLE_AGENT_LOGLEVEL (default: debug)
      - CATTLE_AGENT_CONFIG_DIR (default: C:/etc/rancher/agent)
      - CATTLE_AGENT_VAR_DIR (default: C:/var/lib/rancher/agent)
      - CATTLE_AGENT_BIN_PREFIX (default: C:/usr/local)

      Rancher 2.6+ Variables
      - CATTLE_SERVER
      - CATTLE_TOKEN
      - CATTLE_CA_CHECKSUM
      - CATTLE_ROLE_CONTROLPLANE=false
      - CATTLE_ROLE_ETCD=false
      - CATTLE_ROLE_WORKER=false
      - CATTLE_LABELS
      - CATTLE_TAINTS

      Advanced Environment Variables
      - CATTLE_AGENT_BINARY_URL (default: latest GitHub release)
      - CATTLE_PRESERVE_WORKDIR (default: false)
      - CATTLE_REMOTE_ENABLED (default: true)
      - CATTLE_ID (default: autogenerate)
      - CATTLE_AGENT_BINARY_LOCAL (default: false)
      - CATTLE_AGENT_BINARY_LOCAL_LOCATION (default: )
.EXAMPLE 
    
#>
#Make sure this params matches the CmdletBinding below
param (
    [Parameter()]
    [String]
    $Address,
    [Parameter()]
    [String]
    $CaChecksum,
    [Parameter()]
    [String]
    $InternalAddress,
    [Parameter()]
    [String]
    $Label,
    [Parameter()]
    [String]
    $NodeName,
    [Parameter()]
    [String]
    $Server,
    [Parameter()]
    [String]
    $Taint,
    [Parameter()]
    [String]
    $Token,
    [Parameter()]
    [Switch]
    $Worker
)
function Rke2-Installer {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $Address,
        [Parameter()]
        [String]
        $CaChecksum,
        [Parameter()]
        [String]
        $InternalAddress,
        [Parameter()]
        [String]
        $Label,
        [Parameter()]
        [String]
        $NodeName,
        [Parameter()]
        [String]
        $Server,
        [Parameter()]
        [String]
        $Taint,
        [Parameter()]
        [String]
        $Token,
        [Parameter()]
        [Switch]
        $Worker
    )
    Set-StrictMode -Version Latest
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls -bor [Net.SecurityProtocolType]::Tls11 -bor [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

    function Write-LogInfo {
        Write-Host -NoNewline -ForegroundColor Blue "INFO: "
        Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
    }
    function Write-LogWarn {
        Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
        Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
    }
    function Write-LogError {
        Write-Host -NoNewline -ForegroundColor DarkRed "ERROR: "
        Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
    }
    function Write-LogFatal {
        Write-Host -NoNewline -ForegroundColor DarkRed "FATAL: "
        Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
        exit 255
    }

    function Get-Args {
        if ($Address) {
            $env:CATTLE_ADDRESS = $Address
        }

        if ($CaChecksum) {
            $env:CATTLE_CA_CHECKSUM = $CaChecksum
        }

        if ($InternalAddress) {
            $env:CATTLE_INTERNAL_ADDRESS = $InternalAddress
        }

        if ($Label) {
            if ($env:CATTLE_LABELS) {
                $env:CATTLE_LABELS += ",$Label"
            }
            else {
                $env:CATTLE_LABELS = $Label
            }
        }

        if ($NodeName) {
            $env:CATTLE_NODE_NAME = $NodeName
        }

        if ($Server) {
            $env:CATTLE_SERVER = $Server
        }

        if ($Taint) {
            if ($env:CATTLE_TAINTS) {
                $env:CATTLE_TAINTS += ",$Taint"
            }
            else {
                $env:CATTLE_TAINTS = $Taint
            }
        }

        if ($Token) {
            $env:CATTLE_TOKEN = $Token
        }

        if ($Worker) {
            $env:CATTLE_ROLE_WORKER = "true"
        }
    }

    function Set-Path {
        $env:PATH += ";C:\var\lib\rancher\rke2\bin;C:\usr\local\bin"
        $environment = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
        $environment = $environment.Insert($environment.Length, ";C:\var\lib\rancher\rke2\bin;C:\usr\local\bin")
        [System.Environment]::SetEnvironmentVariable("Path", $environment, "Machine")
    }

    function Set-Environment {
        if (-Not $env:CATTLE_ROLE_CONTROLPLANE) {
            $env:CATTLE_ROLE_CONTROLPLANE = "false"
        }

        if (-Not $env:CATTLE_ROLE_ETCD) {
            $env:CATTLE_ROLE_ETCD = "false"
        }

        if (-Not $env:CATTLE_ROLE_WORKER) {
            $env:CATTLE_ROLE_WORKER = "false"
        }

        if (-Not $env:CATTLE_REMOTE_ENABLED) {
            $env:CATTLE_REMOTE_ENABLED = "true"
        }
        else {
            #$env:CATTLE_REMOTE_ENABLED = "$(echo "${CATTLE_REMOTE_ENABLED}" | tr '[:upper:]' '[:lower:]')"
        }

        if (-Not $env:CATTLE_PRESERVE_WORKDIR) {
            $env:CATTLE_PRESERVE_WORKDIR = "false"
        }
        else {
            #$env:CATTLE_PRESERVE_WORKDIR = "$(echo "${CATTLE_PRESERVE_WORKDIR}" | tr '[:upper:]' '[:lower:]')"
        }

        if (-Not $env:CATTLE_AGENT_LOGLEVEL) {
            $env:CATTLE_AGENT_LOGLEVEL = "debug"
        }
        else {
            #$env:CATTLE_AGENT_LOGLEVEL = "$(echo "${CATTLE_AGENT_LOGLEVEL}" | tr '[:upper:]' '[:lower:]')"
        }

        if ($env:CATTLE_AGENT_BINARY_LOCAL -eq "true") {
            if (-Not $env:CATTLE_AGENT_BINARY_LOCAL_LOCATION) {
                Write-LogFatal "No local binary location was specified"
            }
        }
        else {
            if (-Not $env:CATTLE_AGENT_BINARY_URL) {
                $env:CATTLE_AGENT_BINARY_URL = "https://raw.githubusercontent.com/rancher/rke2/master/install.ps1"
            }
        }

        if ($env:CATTLE_REMOTE_ENABLED -eq "true") {
            if (-Not $env:CATTLE_TOKEN) {
                Write-LogInfo "Environment variable CATTLE_TOKEN was not set. Will not retrieve a remote connection configuration from Rancher2"
            }
            else {
                if (-Not $env:CATTLE_SERVER) {
                    Write-LogFatal "Environment variable CATTLE_SERVER was not set"
                }
            }
        }

        if (-Not $env:CATTLE_AGENT_CONFIG_DIR) {
            $env:CATTLE_AGENT_CONFIG_DIR = "C:/etc/rancher/agent"
            [System.Environment]::SetEnvironmentVariable('CATTLE_AGENT_CONFIG_DIR', "C:/etc/rancher/agent", 'Machine')
            Write-LogInfo "Using default agent configuration directory $( $env:CATTLE_AGENT_CONFIG_DIR )"
        }

        if (-Not (Test-Path $env:CATTLE_AGENT_CONFIG_DIR)) {
            New-Item -Path $env:CATTLE_AGENT_CONFIG_DIR -ItemType Directory -Force
        }

        if (-Not $env:CATTLE_AGENT_VAR_DIR) {
            $env:CATTLE_AGENT_VAR_DIR = "C:/var/lib/rancher/agent"
            [System.Environment]::SetEnvironmentVariable('CATTLE_AGENT_VAR_DIR', "C:/var/lib/rancher/agent", 'Machine')
            Write-LogInfo "Using default agent var directory $( $env:CATTLE_AGENT_VAR_DIR )"
        }

        if (-Not (Test-Path $env:CATTLE_AGENT_VAR_DIR)) {
            New-Item -Path $env:CATTLE_AGENT_VAR_DIR -ItemType Directory -Force
        }

        if (-Not $env:CATTLE_AGENT_BIN_PREFIX) {
            $env:CATTLE_AGENT_BIN_PREFIX = "C:/usr/local"
            [System.Environment]::SetEnvironmentVariable('CATTLE_AGENT_BIN_PREFIX', "C:/usr/local", 'Machine')
            Write-LogInfo "Using default agent bin prefix $( $env:CATTLE_AGENT_BIN_PREFIX )"
        }

        if (-Not (Test-Path $env:CATTLE_AGENT_BIN_PREFIX)) {
            New-Item -Path "$($env:CATTLE_AGENT_BIN_PREFIX)/bin" -ItemType Directory -Force
        }
        
        $env:CATTLE_ADDRESS = Get-Address -Value $env:CATTLE_ADDRESS
        $env:CATTLE_INTERNAL_ADDRESS = Get-Address -Value $env:CATTLE_INTERNAL_ADDRESS
    }

    function Test-Architecture() {
        if ($env:PROCESSOR_ARCHITECTURE -ne "AMD64") {
            Write-LogFatal "Unsupported architecture $( $env:PROCESSOR_ARCHITECTURE )"
        }
    }

    function Invoke-Rke2AgentDownload() {
        $localLocation = "$($env:CATTLE_AGENT_BIN_PREFIX)/bin"
        if ($env:CATTLE_AGENT_BINARY_LOCAL) {
            Write-LogInfo "Using local RKE2 installer from $($env:CATTLE_AGENT_BINARY_LOCAL_LOCATION)"
            Copy-Item -Path $env:CATTLE_AGENT_BINARY_LOCAL -Destination "$($localLocation)/install.ps1"
        }
        else {
            Write-LogInfo "Downloading RKE2 installer from $($env:CATTLE_AGENT_BINARY_URL)"
            curl.exe -sfL $env:CATTLE_AGENT_BINARY_URL -o "$($localLocation)/install.ps1"
        }
    }

    function Test-CaCheckSum() {
        $caCertsPath = "cacerts"
        $env:RANCHER_CERT = "$env:TEMP/ranchercert"
        if (-Not $env:CATTLE_CA_CHECKSUM) {
            return
        }

        curl.exe --insecure -sfL $env:CATTLE_SERVER/$caCertsPath -o $env:RANCHER_CERT
        if (-Not(Test-Path -Path $env:RANCHER_CERT)) {
            Write-Error "The environment variable CATTLE_CA_CHECKSUM is set but there is no CA certificate configured at $( $env:CATTLE_SERVER )/$( $caCertsPath )) "
            exit 1
        }
        #Test-Certificate -Cert $cert
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Value from $( $env:CATTLE_SERVER )/$( $caCertsPath ) does not look like an x509 certificate, exited with $( $LASTEXITCODE ) "
            Write-Error "Retrieved cacerts:"
            Get-Content $env:RANCHER_CERT
            exit 1
        }
        else {
            Write-LogInfo "Value from $( $env:CATTLE_SERVER )/$( $caCertsPath ) is an x509 certificate"
        }
        $env:CATTLE_SERVER_CHECKSUM = (Get-FileHash -Path $env:RANCHER_CERT -Algorithm SHA256).Hash.ToLower()
        if ($env:CATTLE_SERVER_CHECKSUM -ne $env:CATTLE_CA_CHECKSUM) {
            Remove-Item -Path $env:RANCHER_CERT -Force
            Write-LogError "Configured cacerts checksum $( $env:CATTLE_SERVER_CHECKSUM ) does not match given --ca-checksum $( $env:CATTLE_CA_CHECKSUM ) "
            Write-LogError "Please check if the correct certificate is configured at $( $env:CATTLE_SERVER )/$( $caCertsPath ) ."
            exit 1
        }
    }
    function Get-Rke2Config() {
        $retries = 0
        $path = "C:\etc\rancher\rke2"
        $file = "config.yaml"
        if (-Not(Test-Path $path)) {
            New-Item -Path $path -ItemType Directory
        }
        $configFile = Join-Path -Path $path -ChildPath $file
        if (-Not(Test-Path $configFile)) {
            $Uri = "$($env:CATTLE_SERVER)/v3/connect/config-yaml"
            Write-LogInfo "Pulling RKE2 config.yaml from $Uri"
            try {
                if (-Not $env:CATTLE_CA_CHECKSUM) {
                    do {
                        # $ErrorActionPreference = "SilentlyContinue"
                        curl.exe -sfL $Uri -o $configFile -H "Authorization: Bearer $($env:CATTLE_TOKEN)" -H "X-Cattle-Id: $($env:CATTLE_ID)" -H "X-Cattle-Role-Worker: $($env:CATTLE_ROLE_WORKER)" -H "X-Cattle-Labels: $($env:CATTLE_LABELS)" -H "X-Cattle-Taints: $($env:CATTLE_TAINTS)" -H "X-Cattle-Address: $($env:CATTLE_ADDRESS)" -H "X-Cattle-Internal-Address: $($env:CATTLE_INTERNAL_ADDRESS)" -H "Content-Type: application/json"
                        $retries++
                        if (-Not(Test-Path $configFile)) {
                            Start-Sleep -Seconds 12
                        }
                    } 
                    while ((-Not(Test-Path $configFile) -and $retries -lt 6 -and $LASTEXITCODE -ne 0))
                }
                else {
                    do {
                        # $ErrorActionPreference = "SilentlyContinue"
                        curl.exe --insecure --cacert $env:RANCHER_CERT -sfL $Uri -o $configFile -H "Authorization: Bearer $($env:CATTLE_TOKEN)" -H "X-Cattle-Id: $($env:CATTLE_ID)" -H "X-Cattle-Role-Worker: $($env:CATTLE_ROLE_WORKER)" -H "X-Cattle-Labels: $($env:CATTLE_LABELS)" -H "X-Cattle-Taints: $($env:CATTLE_TAINTS)" -H "X-Cattle-Address: $($env:CATTLE_ADDRESS)" -H "X-Cattle-Internal-Address: $($env:CATTLE_INTERNAL_ADDRESS)" -H "Content-Type: application/json"
                        $retries++
                        if (-Not(Test-Path $configFile)) {
                            Start-Sleep -Seconds 12
                        }        
                    } 
                    while ((-Not(Test-Path $configFile) -and $retries -lt 6 -and $LASTEXITCODE -ne 0))
                } 
                trap {
                    if ($retries -lt 6) {
                        Write-LogInfo "retry number: $retries"
                        continue
                    }
                    if ($retries -ge 6) {
                        throw [System.Net.WebException]::new()
                    }
                    elseif (-Not(Test-Path $configFile)) {
                        throw [System.IO.FileNotFoundException]
                    }
                    else {
                        throw [Microsoft.PowerShell.Commands.WriteErrorException]::new()                    
                    }
                }
            }
            catch [System.IO.FileNotFoundException] {
                Write-Error -Message "RKE2 config file wasn't found after $retries retries. Max Retries Exceeded." -Exception ([System.Net.WebException]::new()) -ErrorAction Stop -ForegroundColor Red
                exit 1
            }
            catch [System.Net.WebException] {
                Write-Error -Message "RKE2 config file $configFile was not available from $Uri" -Exception ([System.IO.FileNotFoundException]) -ErrorAction Stop  -ForegroundColor Red
                exit 1
            }
            catch [Microsoft.PowerShell.Commands.WriteErrorException] {
                Write-LogFatal "An unexpected error occurred while running the RKE2 Windows Agent Rancher installation script: `r`n$($_)"
            }

            if ((Test-Path $configFile)) {
                Write-LogInfo "RKE2 config.yaml pulled successfully"
            }
        }
    }

    function Get-Rke2Info() {
        $Uri = "$($env:CATTLE_SERVER)/v3/connect/cluster-info"
        $path = "C:/etc/rancher/rke2"
        $file = "info.json"
        if (-Not(Test-Path $path)) {
            New-Item -Path $path -ItemType Directory
        }

        $infoFile = Join-Path -Path $path -ChildPath $file
        if (-Not $env:CATTLE_CA_CHECKSUM) {
            curl.exe -sfL $Uri -o $infoFile -H "Authorization: Bearer $($env:CATTLE_TOKEN)" -H "X-Cattle-Id: $($env:CATTLE_ID)" -H "X-Cattle-Field: kubernetesversion" -H "Content-Type: application/json"
        }
        else {
            curl.exe --insecure --cacert $env:RANCHER_CERT -sfL $Uri -o $infoFile -H "Authorization: Bearer $($env:CATTLE_TOKEN)" -H "X-Cattle-Id: $($env:CATTLE_ID)" -H "X-Cattle-Field: kubernetesversion" -H "Content-Type: application/json"
        }
        Write-LogInfo "$(Get-Content $infoFile)"
        $clusterInfo = Get-Content $infoFile | ConvertFrom-Json
        if ([bool]($clusterInfo.PSobject.Properties.name -match "kubernetesversion")) {
            $env:CATTLE_RKE2_VERSION = $clusterInfo.kubernetesversion
        }
        Remove-Item -Path $infoFile -Force
    }

    function New-CattleId() {
        if (-Not $env:CATTLE_ID) {
            Write-LogInfo "Generating Cattle ID"

            if (Test-Path -Path "$($env:CATTLE_AGENT_CONFIG_DIR)/cattle-id") {
                $env:CATTLE_ID = Get-Content -Path "$($env:CATTLE_AGENT_CONFIG_DIR)/cattle-id"
                Write-LogInfo "Cattle ID was already detected as $($env:CATTLE_ID). Not generating a new one."
                return
            }
            $stream = [IO.MemoryStream]::new([Text.Encoding]::UTF8.GetBytes($env:COMPUTERNAME))
            $env:CATTLE_ID = (Get-FileHash -InputStream $stream -Algorithm SHA256).Hash.ToLower().Substring(0, 62)
            Set-Content -Path "$($env:CATTLE_AGENT_CONFIG_DIR)/cattle-id" -Value $env:CATTLE_ID
            return
        }
        Write-LogInfo "Not generating Cattle ID"
    }

    function Get-Address() {
        [CmdletBinding()]
        param (
            [Parameter()]
            [String]
            $Value
        )
        if (!$Value) {
            # If nothing is given, return empty (it will be automatically determined later if empty)
            return ""
        }
        # If given address is a network interface on the system, retrieve configured IP on that interface (only the first configured IP is taken)
        elseif (Get-NetAdapter -Name $Value -ErrorAction SilentlyContinue) {
            return $(Get-NetIpConfiguration | Where-Object { $null -ne $_.IPv4DefaultGateway -and $_.NetAdapter.Status -ne "Disconnected" }).IPv4Address.IPAddress
        }
        # Loop through cloud provider options to get IP from metadata, if not found return given value
        else {
            switch ($Value) {
                awslocal { return curl.exe --connect-timeout 60 --max-time 60 -s http://169.254.169.254/latest/meta-data/local-ipv4 }
                awspublic { return curl.exe --connect-timeout 60 --max-time 60 -s http://169.254.169.254/latest/meta-data/public-ipv4 }
                doprivate { return curl.exe --connect-timeout 60 --max-time 60 -s http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address }
                dopublic { return curl.exe --connect-timeout 60 --max-time 60 -s http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address }
                azprivate { return curl.exe --connect-timeout 60 --max-time 60 -s -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-08-01&format=text" }
                azpublic { return curl.exe --connect-timeout 60 --max-time 60 -s -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text" }
                gceinternal { return curl.exe --connect-timeout 60 --max-time 60 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip }
                gceexternal { return curl.exe --connect-timeout 60 --max-time 60 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip }
                packetlocal { return curl.exe --connect-timeout 60 --max-time 60 -s https://metadata.packet.net/2009-04-04/meta-data/local-ipv4 }
                packetpublic { return curl.exe --connect-timeout 60 --max-time 60 -s https://metadata.packet.net/2009-04-04/meta-data/public-ipv4 }
                ipify { return curl.exe --connect-timeout 60 --max-time 60 -s https://api.ipify.org }
                Default {
                    return $Value
                }
            }          
        }
    }

    function Invoke-RancherInstall() {
        $rke2ServiceName = "rke2"
        Test-Architecture
        Get-Args
        Set-Environment
        Set-Path
        Test-CaCheckSum

        if ((Get-Service -Name $rke2ServiceName -ErrorAction SilentlyContinue)) {
            Stop-Service -Name $rke2ServiceName
            while ((Get-Service $rke2ServiceName).Status -ne 'Stopped') {
                Start-Sleep -s 5
            }
        }

        Invoke-Rke2AgentDownload
        New-CattleId
        Get-Rke2Config
        Get-Rke2Info

        if ($env:CATTLE_RKE2_VERSION) {
            Invoke-Expression -Command "$($env:CATTLE_AGENT_BIN_PREFIX)/bin/install.ps1 -Version $($env:CATTLE_RKE2_VERSION)"
        }
        else {
            Invoke-Expression -Command "$($env:CATTLE_AGENT_BIN_PREFIX)/bin/install.ps1"
        }

        Write-LogInfo "Checking if RKE2 agent service exists"
        if ((Get-Service -Name $rke2ServiceName -ErrorAction SilentlyContinue)) {
            Write-LogInfo "RKE2 agent service found, stopping now"
            Stop-Service -Name $rke2ServiceName
            while ((Get-Service $rke2ServiceName).Status -ne 'Stopped') {
                Write-LogInfo "Waiting for RKE2 agent service to stop"
                Start-Sleep -s 5
            }
        }
        else {
            # Create Windows Service
            Write-LogInfo "RKE2 agent service not found, enabling agent service"
            Push-Location "$($env:CATTLE_AGENT_BIN_PREFIX)/bin"
            rke2.exe agent service --add
            Pop-Location
            Start-Sleep -s 5
        }

        Write-LogInfo "Starting the RKE2 agent service"
        Start-Service -Name $rke2ServiceName
    }

    Invoke-RancherInstall
}