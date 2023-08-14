<#
.SYNOPSIS
  A quickstart script to Setup and Install standalone RKE2 in Windows to be used as Worker Nodes.
  This script enables features, sets up environment variables and adds default configuration that are needed to install RKE2 in Windows and join a cluster.`
.DESCRIPTION
  Run the script to setup and install all RKE2 related needs and to join a cluster.
.Parameter ServerIP
    Server IP of Primary server where RKE2 is already installed and the worker will join.`
.Parameter Token
    Token of Primary server.`
.Parameter Mode
    Installation Mode of RKE2
    Can be either INSTALL_RKE2_VERSION or INSTALL_RKE2_COMMIT`
.Parameter Version
    Version of RKE2 or Commit of RKE2 to download from cloud storage.
    If Commit Mode set, this forces Method=tar in install script.
    * (for developer & QA use only)`

.EXAMPLE
  Usage:
    Invoke-WebRequest ((New-Object System.Net.WebClient).DownloadString('https://github.com/rancher/rke2/blob/master/windows/rke2-quickstart.ps1'))
    ./rke2-quickstart.ps1 $ServerIP <server-IP> $Token <server-token> $Mode <install-mode> $Version <rke2-version>
#>

[CmdletBinding()]
param (
    [Parameter(Mandatory=$true)]
    [String]
    $ServerIP,
    [Parameter(Mandatory=$true)]
    [String]
    $Version,
    [Parameter(Mandatory=$true)]
    [String]
    $Token,
    [Parameter(Mandatory=$false)]
    [String]
    $Mode
)

function Write-InfoLog() {
    Write-Output "[INFO] $($args -join " ")"
}

function Write-WarnLog() {
    Write-Output "[WARN] $($args -join " ")"
}

function Write-DebugLog() {
    Write-Output "[DEBUG] $($args -join " ")"
}

# Set the version or commit based of cli
function Set-Version() {
    if ($PSBoundParameters.ContainsKey("Mode")) {
        if (($Mode -like "*VERSION") -or ($Mode -like "*COMMIT")) {
            $modeArr = $Mode.Split("_")
            $mode = (Get-Culture).TextInfo.ToTitleCase($modeArr[2].ToLower())
            $version = "$($mode) $($Version)"
        } else {
            Write-InfoLog "Unsupported Install Mode: $($Mode)"
        }
    } else {
        if ($Version -match "rke2r") {
            $version = "Version $($Version)"
        } else {
            $version = "Commit $($Version)"
        }
    }
    
    return $version
}

function Enable-Features() {
    Enable-WindowsOptionalFeature -Online -FeatureName Containers -All
}

function Setup-Config(){
    Write-InfoLog "Creating rke2 directory..."
    New-Item -Type Directory c:/etc/rancher/rke2 -Force
    Write-InfoLog "Fetch public IP..."
    $publicIP = (Invoke-WebRequest -uri "https://api.ipify.org/").Content.Trim()
    Write-InfoLog "Setting up rke2 config.yaml file..."
    Set-Content -Path c:/etc/rancher/rke2/config.yaml -Value "server: https://$($ServerIP):9345`ntoken: $Token`nnode-external-ip: $publicIP`n"
    Get-Content -Path c:/etc/rancher/rke2/config.yaml
}

function Setup-EnvironmentVariables(){
    Write-InfoLog "Setting up environment vars..."
    [System.Environment]::SetEnvironmentVariable(
        "Path",[System.Environment]::GetEnvironmentVariable(
            "Path", [System.EnvironmentVariableTarget]::Machine) + ";c:\var\lib\rancher\rke2\bin;c:\usr\local\bin",
    [System.EnvironmentVariableTarget]::Machine)
}

function Install-rke2(){
    Write-InfoLog "Downloading install script..."
    Invoke-WebRequest -Uri https://raw.githubusercontent.com/rancher/rke2/master/install.ps1 -Outfile C:\Users\Administrator\install.ps1
    $version = Set-Version
    Write-InfoLog "Installing rke2 with $version..."
    Invoke-Expression -Command "C:\Users\Administrator\install.ps1 -$version"
}

function Start-rke2(){
    Write-InfoLog "Adding rke2-agent service..."
    Invoke-Expression -Command "C:\usr\local\bin\rke2.exe agent service --add"
    Write-InfoLog "Starting rke2-agent service..."
    Start-Service rke2
}


Enable-Features
Setup-Config
Setup-EnvironmentVariables
Install-rke2
Start-rke2
