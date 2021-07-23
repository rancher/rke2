<#
.SYNOPSIS 
    Installs Rancher RKE2 to create Windows Worker Nodes.
.DESCRIPTION 
    Run the script to install all Rancher RKE2 related neeeds. (kubernetes, docker, network) 
.NOTES
    Environment variables:
      System Agent Variables
      - CATTLE_AGENT_LOGLEVEL (default: debug)
      - CATTLE_AGENT_CONFIG_DIR (default: C:/etc/rancher/agent)
      - CATTLE_AGENT_VAR_DIR (default: C:/var/lib/rancher/agent)
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

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12



function Write-LogInfo {
    Write-Host -NoNewline -ForegroundColor Blue "INFO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}
function Write-LogWarn {
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}
function Write-LogError {
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}
function Write-LogFatal {
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
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
        $env:CATTLE_REMOTE_ENABLED = "$(echo "${CATTLE_REMOTE_ENABLED}" | tr '[:upper:]' '[:lower:]')"
    }

    if (-Not $env:CATTLE_PRESERVE_WORKDIR) {
        $env:CATTLE_PRESERVE_WORKDIR = "false"
    } 
    else {
        $env:CATTLE_PRESERVE_WORKDIR = "$(echo "${CATTLE_PRESERVE_WORKDIR}" | tr '[:upper:]' '[:lower:]')"
    }

    if (-Not $env:CATTLE_AGENT_LOGLEVEL) {
        $env:CATTLE_AGENT_LOGLEVEL = "debug"
    } 
    else {
        $env:CATTLE_AGENT_LOGLEVEL = "$(echo "${CATTLE_AGENT_LOGLEVEL}" | tr '[:upper:]' '[:lower:]')"
    }

    if ($env:CATTLE_AGENT_BINARY_LOCAL -eq "true") {
        if (-Not $env:CATTLE_AGENT_BINARY_LOCAL_LOCATION) {
            Write-LogFatal "No local binary location was specified"
        }
    }
    else {        
        $env:CATTLE_AGENT_BINARY_URL = "https://github.com/rancher/rke2/blob/master/install.ps1"
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
        Write-LogInfo "Using default agent configuration directory $($env:CATTLE_AGENT_CONFIG_DIR)"
    }

    if (-Not $env:CATTLE_AGENT_VAR_DIR) {
        $env:CATTLE_AGENT_VAR_DIR = "C:/etc/rancher/agent"
        Write-LogInfo "Using default agent var directory $($env:CATTLE_AGENT_VAR_DIR)"
    }
}

function Test-Architecture() {
    if ($env:PROCESSOR_ARCHITECTURE -ne "AMD64") {
        Write-LogFatal "Unsupported architecture $($env:PROCESSOR_ARCHITECTUR)"
    }
} 

function Invoke-Rke2AgentDownload() {
    $localLocation = "C:\var\lib\rancher\install.ps1"
    if ($env:CATTLE_AGENT_BINARY_LOCAL) {
        Write-LogInfo "Using local RKE2 installer from $($env:CATTLE_AGENT_BINARY_LOCAL_LOCATION)"
        Copy-Item -Path $env:CATTLE_AGENT_BINARY_LOCAL -Destination $localLocation
    }
    else {
        Write-LogInfo "Downloading RKE2 installer from $($env:CATTLE_AGENT_BINARY_URL)"
        Invoke-RestMethod -Uri $env:CATTLE_AGENT_BINARY_URL -OutFile $localLocation
    }
}

function Test-CaCheckSum() {
    $caCertsPath = "cacerts"
    $env:RANCHER_CERT = "$env:TEMP/ranchercert"
    if (-Not $env:CATTLE_CA_CHECKSUM) {
        Invoke-RestMethod -Url $env:CATTLE_SERVER/$caCertsPath -OutFile $env:RANCHER_CERT -SkipCertificateCheck
        if (-Not (Test-Path -Path $env:RANCHER_CERT)) {
            Write-Error "The environment variable CATTLE_CA_CHECKSUM is set but there is no CA certificate configured at $(env:CATTLE_SERVER/$caCertsPath)) "
            exit 1
        }
        openssl x509 -in $cert -noout
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Value from $($env:CATTLE_SERVER)/$($caCertsPath) does not look like an x509 certificate, exited with $($LASTEXITCODE) "
            Write-Error "Retrieved cacerts:"
            Get-Content $env:RANCHER_CERT
            exit 1
        }
        else {
            info "Value from $($env:CATTLE_SERVER)/$($caCertsPath) is an x509 certificate"
        }
        $env:CATTLE_SERVER_CHECKSUM = (Get-FileHash -Path $env:RANCHER_CERT -Algorithm SHA256).Hash.ToLower()
        if ($env:CATTLE_SERVER_CHECKSUM -ne $env:CATTLE_CA_CHECKSUM) {
            Remove-Item -Path $env:RANCHER_CERT -Force
            Write-LogError "Configured cacerts checksum $($env:CATTLE_SERVER_CHECKSUM) does not match given --ca-checksum $($env:CATTLE_CA_CHECKSUM) "
            Write-LogError "Please check if the correct certificate is configured at $($env:CATTLE_SERVER)/$($caCertsPath) ."
            exit 1
        }
    }
}

function Get-ConnectionInfo() {
    if ($env:CATTLE_REMOTE_ENABLED -eq "true") {
        $requestParams = @{
            Uri     = "$($env:CATTLE_SERVER)/v3/connect/agent"
            Headers = @{
                'Authorization'               = "Bearer $($env:CATTLE_TOKEN)"
                'X-Cattle-Id'                 = $env:CATTLE_ID
                'X-Cattle-Role-Etcd'          = $env:CATTLE_ROLE_ETCD
                'X-Cattle-Role-Control-Plane' = $env:CATTLE_ROLE_CONTROLPLANE
                'X-Cattle-Role-Worker'        = $env:CATTLE_ROLE_WORKER
                'X-Cattle-Labels'             = $env:CATTLE_LABELS
                'X-Cattle-Taints'             = $env:CATTLE_TAINTS
            }
            OutFile = "$($env:CATTLE_AGENT_VAR_DIR)/rancher2_connection_info.json"
        }

        if (-Not $env:CATTLE_CA_CHECKSUM) {
            Invoke-RestMethod @requestParams -SkipCertificateCheck -OutFile ""
        }
        else {
            $cert = Get-PfxCertificate -FilePath $env:RANCHER_CERT
            Invoke-RestMethod @requestParams -Certificate $cert -OutFile ""
        }
    }
}

function Get-Rke2Config() {
    $configPath = "C:\etc\rancher\rke2\config.yaml"
    if (-Not(Test-Path $configPath)) {
        $requestParams = @{
            Uri     = "$($env:CATTLE_SERVER)/v3/connect/config-yaml"
            Headers = @{
                'Authorization'               = "Bearer $($env:CATTLE_TOKEN)"
                'X-Cattle-Id'                 = $env:CATTLE_ID
                'X-Cattle-Role-Worker'        = $env:CATTLE_ROLE_WORKER
            }
        }
        
        $response = $null
        if (-Not $env:CATTLE_CA_CHECKSUM) {
            $response = Invoke-RestMethod @requestParams -SkipCertificateCheck
        }
        else {
            $cert = Get-PfxCertificate -FilePath $env:RANCHER_CERT
            $response = Invoke-RestMethod @requestParams -Certificate $cert
        }
        $config = "server: $($response.server)
token: $($response.token)
node-label:
  - `"$($response."node-label"[0])`""
        Set-Content -Path $configPath -Value $config -Force
    }
    
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
        $env:CATTLE_ID = (Get-FileHash -InputStream $stream -Algorithm SHA256).Hash.ToLower()
        Set-Content -Path "$($env:CATTLE_AGENT_CONFIG_DIR)/cattle-id" -Value $env:CATTLE_ID
        return
    }
    Write-LogInfo "Not generating Cattle ID"
}

function Invoke-RancherInstall () {
    $rke2ServiceName = "rke2"
    Get-Args
    Set-Environment
    New-Directories
    
    if (-Not $env:CATTLE_CA_CHECKSUM) { Test-CaCheckSum }

    if ((Get-Service -Name $rke2ServiceName -ErrorAction SilentlyContinue)) {
        Stop-Service -Name $rke2ServiceName
        while ((Get-Service $rke2ServiceName).Status -ne 'Stopped') { Start-Sleep -s 5 }
    }

    Invoke-Rke2AgentDownload

    if (-Not $env:CATTLE_TOKEN) {
        New-CattleId
    }

    Get-Rke2Config   

    Invoke-Expression -Command "C:\var\lib\rancher\install.ps1"
    
    if ((Get-Service -Name $rke2ServiceName -ErrorAction SilentlyContinue)) {
        Stop-Service -Name $rke2ServiceName
        while ((Get-Service $rke2ServiceName).Status -ne 'Running') { Start-Sleep -s 5 }
    }
    else {
        # Create Windows Service
        Write-LogInfo "Enabling RKE2 agent service"
        rke2.exe agent service --add
    }
    
}

Invoke-RancherInstall
exit 0