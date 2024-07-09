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
$RKE2_DATA_DIR = if ($env:RKE2_DATA_DIR) { $env:RKE2_DATA_DIR } else { "c:/var/lib/rancher/rke2" };
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
    $ProcessNames = @('rke2', 'kube-proxy', 'kubelet', 'containerd', 'wins', 'calico-node', 'flanneld')
    foreach ($ProcessName in $ProcessNames) {
        Write-LogInfo "Checking if $ProcessName process exists"
        if ((Get-Process -Name $ProcessName -ErrorAction SilentlyContinue)) {
            Write-LogInfo "$ProcessName process found, stopping now"
            Stop-Process -Name $ProcessName
            while (-Not(Get-Process -Name $ProcessName).HasExited) {
                Write-LogInfo "Waiting for $ProcessName process to stop"
                Start-Sleep -s 5
            }
        }
    }
}

# clean up firewall rules
Get-NetFirewallRule -PolicyStore ActiveStore -Name "rke2*" -ErrorAction Ignore | ForEach-Object {
    Write-LogInfo "Cleaning up firewall rule $($_.Name) ..."
    $_ | Remove-NetFirewallRule -ErrorAction Ignore | Out-Null
}

function Invoke-CleanServices () {
    $ServiceNames = @('rke2', 'wins')
    # clean up wins and rke2 service
    foreach ($ServiceName in $ServiceNames) {
        Write-LogInfo "Checking if $ServiceName service exists"
        if ((Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
            Write-LogInfo "$ServiceName service found, stopping now"
            Stop-Service -Name $ServiceName
            while ((Get-Service -Name $ServiceName).Status -ne 'Stopped') {
                Write-LogInfo "Waiting for $ServiceName service to stop"
                Start-Sleep -s 5
            }
            Write-LogInfo "$ServiceName service has stopped. Removing the $ServiceName service ..."
            if (($PSVersionTable.PSVersion.Major) -ge 6) {
                Remove-Service $ServiceName
            }
            else {
                sc.exe delete $ServiceName
            }
        }
    }
}

function Reset-HNS () {
    try {
        Get-HnsNetwork | Where-Object { $_.Name -eq 'Calico' -or $_.Name -eq 'vxlan0' -or $_.Name -eq 'nat' -or $_.Name -eq 'External' -or $_.Name -eq 'flannel.4096' } | Select-Object Name, ID | ForEach-Object {
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
            if ($symLinkCheck) {
                Get-ChildItem -Path $dir -Recurse -Attributes ReparsePoint | ForEach-Object { $_.Delete() }
            }
            Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
        }
        else {
            Write-LogInfo "$dir is empty, moving on"
        }
    }
    $cleanCustomDirs = @("$env:CATTLE_AGENT_BIN_PREFIX", "$env:CATTLE_AGENT_VAR_DIR", "$env:CATTLE_AGENT_CONFIG_DIR")
    if ($cleanCustomDirs) {
        $ErrorActionPreference = 'SilentlyContinue'
        foreach ($dirs in $cleanCustomDirs) {
            if ($dirs.Contains("/")) {
                $dirs = $dirs -Replace "/", "\"
            }
            $dirs = $dirs.Substring(0, $dirs.IndexOf('\'))
            Write-LogInfo "Cleaning $dirs..."
            if (Test-Path $dirs) {
                $symLinkCheck = "Get-ChildItem -Path $dirs -Recurse -Attributes ReparsePoint"
                if ($symLinkCheck) {
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
    $CONTAINERD_ADDRESS = "\\.\\pipe\\containerd-containerd"
    crictl config --set runtime-endpoint="npipe:$CONTAINERD_ADDRESS"
    function Invoke-Ctr {
        param (
            [parameter()]
            [string]
            $cmd
        )
        $baseCommand = "ctr -a $CONTAINERD_ADDRESS"
        Invoke-Expression -Command "$(-join $baseCommand,$cmd)"
    }

    if (ctr) {
        # We create a lockfile to prevent rke2 service from starting kubelet again
        Create-Lockfile
        Stop-Process -Name "kubelet"
        while (-Not(Get-Process -Name "kubelet").HasExited) {
            Write-LogInfo "Waiting for kubelet process to stop"
            Start-Sleep -s 5
        }
        $namespaces = $(Find-Namespaces)
        if (-Not($namespaces)) {
            $ErrorActionPreference = 'SilentlyContinue'
            $namespaces = @('default', 'cattle-system', 'kube-system', 'fleet-default', 'calico-system')
            Write-LogInfo "Could not find containerd namespaces, will use default list instead:`r`n$namespaces"
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
        }

        # Resources in the namespace take a while to disappear. Snapshots are always the last to go.
        # For 180s, snapshots are checked and namespace won't be removed until snapshots are gone.
        # If timeout is reached, we will try removing snapshots directly. If that does not work, we stop trying and user is warned
        $endTime = (Get-Date).AddSeconds(180)
        Write-Output "Tasks, containers, images and snapshots are being deleted. This may take a while (timeout 180s)"
        while ($true) {
            $namespaces = $(Find-Namespaces)
            # If there are still namespaces and timeout was not reached
            if ($namespaces -and (Get-Date) -lt $endTime) {
                foreach ($ns in $namespaces) {
                    $snapshots = $(Find-Snapshots $ns)
                    if ($snapshots) {
                        Write-Output "Snapshots still present. Retrying namespace deletion in 10s..."
                        Start-Sleep -Seconds 10
                    } else {
                        Remove-Namespace $ns
                    }
                }
            # if there are still namespaces and timeout was reached
            } elseif ($namespaces -and (Get-Date) -ge $endTime) {
                Write-Output "Warning! Not all resources in containerd namespace $ns were able to be removed. " `
                "The uninstallation script might not be able to remove all files under $RKE2_DATA_DIR"
                break
            # if there are no namespaces
            } elseif (-not $namespaces) {
                Write-Output "All containerd resources have been deleted"
                break
            }
            if ((Get-Date) -ge $endTime) {
                Write-Output "Time out waiting for containerd resources to be removed. Trying to remove snapshots directly"
                foreach ($ns in $namespaces) {
                    $snapshots = $(Find-Snapshots $ns)
                    foreach ($snapshot in $snapshots) {
                        Remove-Snapshot $ns $snapshot
                    }
                    # We wait for 20 seconds, try to remove the namespaces again and iterate one last time
                    Start-Sleep -Seconds 20
		            Remove-Namespace $ns
                }
            }
        }
    }
    else {
        Write-LogError "PATH is misconfigured or ctr is missing from PATH"
        Write-LogWarn "Cannot clean up containerd resources"
    }
}

function Find-Namespaces () {
    Invoke-Ctr -cmd "namespace list -q"
}

function Find-ContainersInNamespace() {
    $namespace = $args[0]
    Invoke-Ctr -cmd "-n $namespace container list -q"
}

function Find-Tasks() {
    $namespace = $args[0]
    Invoke-Ctr -cmd "-n $namespace task list -q"
}

function Find-Images() {
    $namespace = $args[0]
    Invoke-Ctr -cmd "-n $namespace image list -q"
}

# The snapshots list does not have option "-q" to list only the keys
function Find-Snapshots() {
    $namespace = $args[0]
    # We skip the first line which is the header KEY
    Invoke-Ctr -cmd "-n $namespace snapshot list" | Select-Object -Skip 1 | ForEach-Object {
        ($_ -split '\s+')[0]
    }
}

function Remove-Image() {
    $namespace = $args[0]
    $image = $args[1]
    Invoke-Ctr -cmd "-n $namespace image rm $image"
}

function Remove-Task() {
    $namespace = $args[0]
    $task = $args[1]
    Invoke-Ctr -cmd "-n $namespace task delete --force $task"
}

function Remove-Container() {
    $namespace = $args[0]
    $container = $args[1]
    Invoke-Ctr -cmd "-n $namespace container delete $container"
}

function Remove-Namespace() {
    $namespace = $args[0]
    Invoke-Ctr -cmd "namespace remove $namespace"
}

function Remove-Snapshot() {
    $namespace = $args[0]
    $snapshot = $args[1]
    Invoke-Ctr -cmd "-n $namespace snapshot delete $snapshot"
}

function Create-Lockfile() {
    # We fetch ctr.exe path and place the lock there
    $command = Get-Command ctr -ErrorAction SilentlyContinue
    if ($command) {
        $executablePath = $command.Source
        $dataBinDirDirectory = Split-Path -Parent $executablePath
        $lockFilePath = Join-Path -Path $dataBinDirDirectory -ChildPath "rke2-uninstall.lock"
        New-Item -ItemType File -Path $lockFilePath -Force
    } else {
	# ctr should exist at this point but just in case we add this log
        Write-Host "ctr.exe not found, container cleanup and RKE2 uninstallation is unlikely to succeed."
    }
}

function Invoke-Rke2Uninstall () {
    $env:PATH += ";$env:CATTLE_AGENT_BIN_PREFIX/bin/;$RKE2_DATA_DIR/bin"
    Remove-Containerd
    Stop-Processes
    Invoke-CleanServices
    Remove-Data
    Remove-TempData
    Reset-Environment
    Reset-MachineEnvironment
    Write-LogInfo "HNS will be cleaned next, temporary network disruption may occur. HNS cleanup is the final step."
    Reset-HNS
    Write-LogInfo "Finished!"
}

Invoke-Rke2Uninstall
exit 0
