#Requires -RunAsAdministrator
<# 
.SYNOPSIS 
    Uninstalls the RKE2 Windows service and cleans the RKE2 Windows Agent (Worker) Node. Backup your data. Use at your own risk.
.DESCRIPTION 
    Run the script to uninstall the RKE2 Windows service and cleans the RKE2 Windows Agent (Worker) node of all RKE2 related data.
.NOTES
    This script needs to be run with Elevated permissions to allow for the complete collection of information.
    Backup your data.
    Use at your own risk.
.EXAMPLE 
    rke2-uninstall.ps1
    Uninstalls the RKE2 Windows service and cleans the RKE2 Windows Agent (Worker) Node
#>

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

Set-StrictMode -Version Latest

function Test-Command($cmdname) {
    return [bool](Get-Command -Name $cmdname -ErrorAction SilentlyContinue)
}

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
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
    exit 255
}

function Get-VmComputeNativeMethods() {
    $ret = 'VmCompute.PrivatePInvoke.NativeMethods' -as [type]
    if (-not $ret) {
        $signature = @'
[DllImport("vmcompute.dll")]
public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@
        $ret = Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name "NativeMethods" -PassThru
    }
    return $ret
}

function Invoke-HNSRequest {
    param
    (
        [ValidateSet('GET', 'DELETE')]
        [parameter(Mandatory = $true)] [string] $Method,
        [ValidateSet('networks', 'endpoints', 'activities', 'policylists', 'endpointstats', 'plugins')]
        [parameter(Mandatory = $true)] [string] $Type,
        [parameter(Mandatory = $false)] [string] $Action,
        [parameter(Mandatory = $false)] [string] $Data = "",
        [parameter(Mandatory = $false)] [Guid] $Id = [Guid]::Empty
    )

    $hnsPath = "/$Type"
    if ($id -ne [Guid]::Empty) {
        $hnsPath += "/$id"
    }
    if ($Action) {
        $hnsPath += "/$Action"
    }

    $response = ""
    $hnsApi = Get-VmComputeNativeMethods
    $hnsApi::HNSCall($Method, $hnsPath, "$Data", [ref]$response)

    $output = @()
    if ($response) {
        try {
            $output = ($response | ConvertFrom-Json)
            if ($output.Error) {
                Write-LogError $output;
            }
            else {
                $output = $output.Output;
            }
        }
        catch {
            Write-LogError $_.
        }
    }

    return $output;
}

# cleanup
Write-Host "Beginning the uninstall process"

function Stop-Processes () {

    Get-Process -ErrorAction Ignore -Name "rke2*" | ForEach-Object {
        Write-LogInfo "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
    
    Get-Process -ErrorAction Ignore -Name "kube-proxy*" | ForEach-Object {
        Write-LogInfo "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
    
    Get-Process -ErrorAction Ignore -Name "kubelet*" | ForEach-Object {
        Write-LogInfo "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
    
    Get-Process -ErrorAction Ignore -Name "containerd*" | ForEach-Object {
        Write-LogInfo "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
    
    Get-Process -ErrorAction Ignore -Name "wins*" | ForEach-Object {
        Write-LogInfo "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
}

# clean up firewall rules
Get-NetFirewallRule -PolicyStore ActiveStore -Name "rke2*" -ErrorAction Ignore | ForEach-Object {
    Write-LogInfo "Cleaning up firewall rule $($_.Name) ..."
    $_ | Remove-NetFirewallRule -ErrorAction Ignore | Out-Null
}

function Invoke-CleanServices () {
    # clean up rke2 service
    Get-Service -Name "rke2" -ErrorAction Ignore | Where-Object { $_.Status -eq "Running" } | ForEach-Object {
        if ($_.Status -eq "Running") {
            Write-LogInfo "Stopping rke2 service ..."
            $_ | Stop-Service -Force -ErrorAction Ignore
            Write-LogInfo "Removing the rke2 service ..."
            if (($PSVersionTable.PSVersion.Major) -ge 6) {
                Remove-Service rke2
            }
            else {
                sc.exe delete rke2
            }
        }
        else {
            Write-LogInfo "Removing the rke2 service ..."
            if (($PSVersionTable.PSVersion.Major) -ge 6) {
                Remove-Service rke2
            }
            else {
                sc.exe delete rke2
            }
        }
    }

    # clean up wins service
    Get-Service -Name "wins" -ErrorAction Ignore | Where-Object { $_.Status -eq "Running" } | ForEach-Object {
        if ($_.Status -eq "Running") {
            Write-LogInfo "Stopping wins service ..."
            $_ | Stop-Service -Force -ErrorAction Ignore
            Write-LogInfo "Removing the wins service ..."
            if (($PSVersionTable.PSVersion.Major) -ge 6) {
                Remove-Service wins
            }
            else {
                sc.exe delete wins
            }
        }
        else {
            Write-LogInfo "Removing the wins service ..."
            if (($PSVersionTable.PSVersion.Major) -ge 6) {
                Remove-Service wins
            }
            else {
                sc.exe delete wins
            }
        }
    }
}

function Reset-HNS () {
    try {
        Get-HnsNetwork | Where-Object { $_.Name -eq 'Calico' -or $_.Name -eq 'vxlan0' -or $_.Name -eq 'nat' -or $_.Name -eq 'External' } | Select-Object Name, ID | ForEach-Object {
            Write-LogInfo "Cleaning up HnsNetwork $($_.Name) ..."
            hnsdiag delete networks $($_.ID)
        }

        Invoke-HNSRequest -Method "GET" -Type "policylists" | Where-Object { -not [string]::IsNullOrEmpty($_.Id) } | ForEach-Object {
            Write-LogInfo "Cleaning up HNSPolicyList $($_.ID) ..."
            Invoke-HNSRequest -Method "DELETE" -Type "policylists" -Id ($_.ID)
        }

        Get-HnsEndpoint  | Select-Object Name, ID | ForEach-Object {
            Write-LogInfo "Cleaning up HnsEndpoint $($_.Name) ..."
            hnsdiag delete endpoints $($_.ID)
        }
    
        Get-HnsNamespace  | Select-Object ID | ForEach-Object {
            Write-LogInfo "Cleaning up HnsEndpoint $($_.ID) ..."
            hnsdiag delete namespace $($_.ID)
        }
    }
    catch {
        Write-LogWarn "Could not clean: $($_)"
    }   
}

# clean up data
function Remove-Data () {
    $cleanDirs = @("c:/usr", "c:/etc", "c:/run", "c:/var")
    foreach ($dir in $cleanDirs) {
        Write-LogInfo "Cleaning $dir..."
        if (Test-Path $dir) {
            $symLinkCheck = "Get-ChildItem -Path $dir -Recurse -Attributes ReparsePoint"
            if (!([string]::IsNullOrEmpty($symLinkCheck))) {
                Get-ChildItem -Path $dir -Recurse -Attributes ReparsePoint | ForEach-Object { $_.Delete() }
            }
            Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
        }
        else {
            Write-LogInfo "$dir is empty, moving on"
        }
    }
    $cleanCustomDirs = @("$env:CATTLE_AGENT_BIN_PREFIX", "$env:CATTLE_AGENT_VAR_DIR", "$env:CATTLE_AGENT_CONFIG_DIR")
    if (!([string]::IsNullOrEmpty($cleanCustomDirs))) {
        $ErrorActionPreference = 'SilentlyContinue'
        foreach ($dirs in $cleanCustomDirs) {
            if ($dirs.Contains("/")) {
                $dirs = $dirs -Replace "/", "\"
            }
            $dirs = $dirs.Substring(0, $dirs.IndexOf('\'))
            Write-LogInfo "Cleaning $dirs..."
            if (Test-Path $dirs) {
                $symLinkCheck = "Get-ChildItem -Path $dirs -Recurse -Attributes ReparsePoint"
                if (!([string]::IsNullOrEmpty($symLinkCheck))) {
                    Get-ChildItem -Path $dirs -Recurse -Attributes ReparsePoint | ForEach-Object { $_.Delete() }
                }
                Remove-Item -Path $dirs -Recurse -Force -ErrorAction SilentlyContinue
            }
            else {
                Write-LogInfo "$dirs is empty, moving on"
            }
        }
    }
}

function Remove-TempData () {
    Write-LogInfo "Cleaning Temp Install Directory..."
    if (Test-Path -Path C:\Users\Administrator\AppData\Local\Temp\rke2-install) {
        Remove-Item -Force -Recurse C:\Users\Administrator\AppData\Local\Temp\rke2-install | Out-Null
    }
}

function Reset-Environment () {
    $customVars = @('CATTLE_AGENT_BINARY_URL', 'CATTLE_AGENT_CONFIG_DIR', 'CATTLE_AGENT_BIN_PREFIX', 'CATTLE_AGENT_LOGLEVEL', 'CATTLE_AGENT_VAR_DIR', 'CATTLE_CA_CHECKSUM', 'CATTLE_ID', 'CATTLE_LABELS', 'CATTLE_PRESERVE_WORKDIR', 'CATTLE_REMOTE_ENABLED', 'CATTLE_RKE2_VERSION', 'CATTLE_ROLE_CONTROLPLANE', 'CATTLE_ROLE_ETCD', 'CATTLE_ROLE_WORKER', 'CATTLE_SERVER', 'CATTLE_SERVER_CHECKSUM', 'CATTLE_TOKEN', 'RANCHER_CERT', 'RKE2_RESOLV_CONF', 'RKE2_PATH' )
    Write-LogInfo "Cleaning RKE2 Environment Variables"
    try {
        ForEach ($v in $customVars) {
            if ([Environment]::GetEnvironmentVariable($v, "Process")) {
                Write-LogInfo "Cleaning $v"
                [Environment]::SetEnvironmentVariable($v, $null, "Process")
            }
            if ([Environment]::GetEnvironmentVariable($v, "User")) {
                Write-LogInfo "Cleaning $v"
                [Environment]::SetEnvironmentVariable($v, $null, "User")
            }
        }
    }
    catch {
        Write-LogWarn "Could not reset environment variables: $($_)"
    }
}

function Reset-MachineEnvironment () {
    $customMachineVars = @('CATTLE_AGENT_VAR_DIR', 'CATTLE_AGENT_CONFIG_DIR', 'CATTLE_AGENT_BIN_PREFIX')
    Write-LogInfo "Cleaning RKE2 Machine Environment Variables"
    try {
        ForEach ($v in $customMachineVars) {
            if ([Environment]::GetEnvironmentVariable($v, "Machine")) {
                Write-LogInfo "Cleaning $v"
                [Environment]::SetEnvironmentVariable($v, $null, "Machine")
            }
            if ([Environment]::GetEnvironmentVariable($v, "User")) {
                Write-LogInfo "Cleaning $v"
                [Environment]::SetEnvironmentVariable($v, $null, "User")
            }
        }
    }
    catch {
        Write-LogWarn "Could not reset machine environment variable: $($_)"
    }
}
function Remove-Containerd () {
    if (Test-Command ctr) {
        $namespaces = $(Find-Namespaces)
        if (-Not($namespaces)) {
            $ErrorActionPreference = 'SilentlyContinue'
            $namespaces = @('default', 'cattle-system', 'kube-system', 'fleet-default', 'calico-system')
            Write-LogInfo "Could not find containerd namespaces using default list `r`n$namespaces"
            return $namespaces
        }
        foreach ($ns in $namespaces) {
            $tasks = $(Find-Tasks $ns)
            foreach ($task in $tasks) {
                Remove-Task $ns $task
            }
            $containers = $(Find-ContainersInNamespace $ns)
            foreach ($container in $containers) {
                Remove-Container $ns $container
            }

            $images = $(Find-Images $ns)
            foreach ($image in $images) {
                Remove-Image $ns $image
            }
            Remove-Namespace $ns
            # TODO
            # clean pods with crictl
        }    
    }
    else {
        Write-LogError "PATH is misconfigured or ctr is missing from PATH"
        Write-LogWarn "Cannot clean up containerd resources"
    }
}

function Find-Namespaces () {
    ctr --namespace=$namespace namespace list -q
    # return $namespace
}
function Find-ContainersInNamespace() {
    $namespace = $1
    ctr --namespace=$namespace container list -q
}

function Find-Tasks() {
    $namespace = $1
    ctr -n $namespace task list -q
}

function Find-Images() {
    $namespace = $1
    ctr -n $namespace image list -q
}

function Remove-Image() {
    $namespace = $1
    $image = $2
    ctr -n $namespace image rm $image
}

function Remove-Task() {
    $namespace = $1
    $task = $2
    ctr -n $namespace task delete --force $task
}

function Remove-Container() {
    $namespace = $1
    $container = $2
    ctr --namespace=$namespace container delete $container
}

function Remove-Namespace() {
    $namespace = $1
    ctr namespace remove $namespace
}

function Rke2-Uninstall () {
    $env:PATH+=";$env:CATTLE_AGENT_BIN_PREFIX/bin/;c:\var\lib\rancher\rke2\bin"
    Remove-Containerd
    Reset-HNS
    Stop-Processes
    Invoke-CleanServices
    Remove-Data
    Remove-TempData
    Reset-Environment
    Reset-MachineEnvironment
    Write-LogInfo "Finished!"
}

Rke2-Uninstall
exit 0
