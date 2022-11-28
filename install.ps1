<#
.SYNOPSIS
  Installs Rancher RKE2 to create Windows Worker Nodes.
.DESCRIPTION
  Run the script to install all Rancher RKE2 related needs.
.Parameter Channel
    Channel to use for fetching rke2 download URL.
    Defaults to 'stable'.`
.Parameter Method
    The installation method to use. Currently tar or choco installation supported.
    Default is on Windows systems is "tar".`
.Parameter Type
    Type of rke2 service. Only the "agent" type is supported on Windows.
    Default is "agent".`
.Parameter Version
    Version of rke2 to download from github.`
.Parameter TarPrefix
    Installation prefix when using the tar installation method. This needs to match the value of CATTLE_AGENT_BIN_PREFIX
    Default is C:/usr/local, unless C:/usr/local is read-only or has a dedicated mount point,
    in which case C:/opt/rke2 is used instead.`
.Parameter Commit
    Commit of RKE2 to download from temporary cloud storage.
    If set, this forces Method=tar.
    * (for developer & QA use only)`
.Parameter AgentImagesDir
    Installation path for airgap images when installing from CI commit.
    Default is C:/var/lib/rancher/rke2/agent/images`
.Parameter ArtifactPath
    If set, the install script will use the local path for sourcing the rke2.windows-$SUFFIX and sha256sum-$ARCH.txt files
    rather than the downloading the files from the internet.
    Default is not set.`
.EXAMPLE
  Usage:
    Invoke-WebRequest ((New-Object System.Net.WebClient).DownloadString('https://github.com/rancher/rke2/blob/master/install.ps1'))
    ./install.ps1
.EXAMPLE
  Usage:
    Invoke-WebRequest ((New-Object System.Net.WebClient).DownloadString('https://github.com/rancher/rke2/blob/master/install.ps1'))
    ./install.ps1 -Channel Latest
.EXAMPLE
  Usage:
    Invoke-WebRequest ((New-Object System.Net.WebClient).DownloadString('https://github.com/rancher/rke2/blob/master/install.ps1'))
    ./install.ps1 -Channel Latest -Method Tar
#>

[CmdletBinding()]
param (
    [Parameter()]
    [String]
    $Channel = "stable",
    [Parameter()]
    [ValidateSet("choco", "tar")]
    [String]
    $Method = "tar",
    [Parameter()]
    [ValidateSet("agent")]
    [String]
    $Type = "agent",
    [Parameter()]
    [String]
    $Version,
    [Parameter()]
    [String]
    $TarPrefix = "C:/usr/local",
    [Parameter()]
    [String]
    $Commit,
    [Parameter()]
    [String]
    $AgentImagesDir = "C:/var/lib/rancher/rke2/agent/images",
    [Parameter()]
    [String]
    $ArtifactPath = "",
    [Parameter()]
    [String]
    $ChannelUrl = "https://update.rke2.io/v1-release/channels"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-InfoLog() {
    Write-Output "[INFO] $($args -join " ")"
}

function Write-WarnLog() {
    Write-Output "[WARN] $($args -join " ")"
}

function Write-DebugLog() {
    Write-Output "[DEBUG] $($args -join " ")"
}

# fatal logs the given argument at fatal log level.
function Write-FatalLog() {
    Write-Output "[ERROR] $($args -join " ")"
    if ([string]::IsNullOrEmpty($suffix)) {
        $archInfo = Get-ArchitectureInfo
        $suffix = $archInfo.Suffix
        Write-Output "[ALT] Please visit 'https://github.com/rancher/rke2/releases' directly and download the latest rke2.$suffix.tar.gz"
    }
    exit 1
}

function Confirm-WindowsFeatures {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory = $true)]
        [String[]]
        $RequiredFeatures
    )
    foreach ($feature in $RequiredFeatures) {
        $f = Get-WindowsFeature -Name $feature
        if (-not $f.Installed) {
            Write-FatalLog "Windows feature: '$feature' is not installed. Please run: Install-WindowsFeature -Name $feature"
        }
        else {
            Write-InfoLog "Windows feature: '$feature' is installed. Installation will proceed."
        }
    }
}

# setup_env defines needed environment variables.
function Set-Environment() {
    # --- bail if we are not administrator ---
    $adminRole = [Security.Principal.WindowsBuiltInRole]::Administrator
    $currentRole = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    if (-NOT $currentRole.IsInRole($adminRole)) {
        Write-FatalLog "You need to be administrator to perform this install"
    }
    if ($env:CATTLE_AGENT_BIN_PREFIX) {
        $TarPrefix = $env:CATTLE_AGENT_BIN_PREFIX
        [System.Environment]::SetEnvironmentVariable('CATTLE_AGENT_BIN_PREFIX', $TarPrefix, 'Machine')
    }
    else {
        [System.Environment]::SetEnvironmentVariable('CATTLE_AGENT_BIN_PREFIX', $TarPrefix, 'Machine')
    }

    Write-Host "Using $($Channel) channel of rke2 for installation"
}


# check_method_conflict will exit with an error if the user attempts to install
# via tar method on a host with existing chocolatey package.
function Test-MethodConflict() {
    if ($Method -eq "choco") {
        $ChocoPackages = choco list --localonly
        if ($ChocoPackages.Select -Like "rke2") {
            Write-FatalLog "Cannot perform $($Method): -tar install on host with existing RKE2 Choco Files - please run rke2-uninstall.ps1 first"
        }
    }
}

# setup_arch set arch and suffix,
# fatal if architecture not supported.
function Get-ArchitectureInfo() {
    $arch = $env:PROCESSOR_ARCHITECTURE.ToLower()
    if ("$arch" -ne "amd64") {
        Write-FatalLog "unsupported architecture $(env:PROCESSOR_ARCHITECTURE)"
        exit 1
    }
    return @{ Suffix = "windows-$arch"; Arch = "$arch" }
}

# get Windows Server Build Version
function Get-BuildVersion() {
    $buildVersion = (Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion" -Name ReleaseId).ReleaseId
    if ("$buildVersion" -eq "2009") {
        $buildVersion = "20H2"
    }
    if ("$buildVersion" -eq "1809" -or "2022" -or "2004" -or "20H2") {
        return $buildVersion
    }
    else {
        Write-FatalLog "unsupported build version $buildVersion"
        exit 1
    }
}

# --- use desired rke2 version if defined or find version from channel ---
function Get-ReleaseVersion() {
    if ($Commit) {
        $Version = "commit $($Commit)"
    }
    elseif ($Version) {
        $Version = $Version
    }
    else {
        $versionUrl = "$ChannelUrl/$Channel"
        $result = New-Object System.Uri($(curl.exe -w "%{url_effective}" -L -s -S $versionUrl -o "$TMP_DIR/version.html"))
        $Version = $result.Segments | Select-Object -Last 1
        Remove-Item -Path "$TMP_DIR/version.html" -Force
    }
    return $Version
}

# download_checksums downloads the binary checksums from github or CI storage url
# and prepares the checksum value for later validation.
function Get-BinaryChecksums() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $Rke2Version,
        [Parameter()]
        [String]
        $Rke2GitHubUrl,
        [Parameter()]
        [String]
        $TempBinaryChecksums
    )

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix
    $arch = $archInfo.Arch
    $binaryChecksumsUrl = ""

    if ($CommitHash) {
        $binaryChecksumsUrl = "$StorageUrl/rke2.$suffix-$CommitHash.tar.gz.sha256sum"
        Write-Host "downloading binary checksum for commit: $CommitHash at $binaryChecksumsUrl"
        curl.exe -sfL $binaryChecksumsUrl -o $TempBinaryChecksums

        return Find-Checksum -ChecksumFilePath $TempBinaryChecksums -Pattern "rke2.$suffix.tar.gz"
    }
    else {
        $binaryChecksumsUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/sha256sum-$arch.txt"
        Write-Host "downloading binary checksum from $binaryChecksumsUrl"
        curl.exe -sfL $binaryChecksumsUrl -o $TempBinaryChecksums

        return Find-Checksum -ChecksumFilePath $TempBinaryChecksums -Pattern "rke2.$suffix.tar.gz"
    }
}

# download_airgap_checksums downloads the checksum file for the airgap image tarball
# and prepares the checksum value for later validation.
function Get-ImageChecksums() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $Rke2Version,
        [Parameter()]
        [String]
        $Rke2GitHubUrl,
        [Parameter()]
        [String]
        $TempImageChecksums
    )

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix
    $arch = $archInfo.Arch
    $imageChecksumsUrl = ""

    if ($CommitHash) {
        $imageChecksumsUrl = "$StorageUrl/rke2-images.$suffix-$CommitHash.tar.zst.sha256sum"
        Write-Host "downloading image checksum for commit: $CommitHash at $imageChecksumsUrl"
        curl.exe -sfL $imageChecksumsUrl -o $TempImageChecksums

        return Find-Checksum -ChecksumFilePath $TempImageChecksums -Pattern "rke2-images.$suffix.tar.zst"
    }
    else {
        $imageChecksumsUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/sha256sum-$arch.txt"
        Write-Host "downloading image checksum from $imageChecksumsUrl"
        curl.exe -sfL $imageChecksumsUrl -o $TempImageChecksums

        return Find-Checksum -ChecksumFilePath $TempImageChecksums -Pattern "rke2-windows-$BuildVersion-$arch-images.tar.gz"
    }
}
# download_tarball downloads binary from github or CI storage url.
function Get-BinaryTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $Rke2Version,
        [Parameter()]
        [String]
        $Rke2GitHubUrl,
        [Parameter()]
        [String]
        $TempTarball
    )    

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix  

    $tarballUrl = ""
    if ($CommitHash) {
        $tarballUrl = "$StorageUrl/rke2.$suffix-$CommitHash.tar.gz"
        Write-Host "downloading binary tarball for commit: $CommitHash at $tarballUrl"
        curl.exe -sfL $tarballUrl -o $TempTarball
    }
    else {
        $tarballUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/rke2.$suffix.tar.gz"
        Write-InfoLog "downloading binary tarball at $tarballUrl"
        curl.exe -sfL $tarballUrl -o $TempTarball
    }
}

# stage_local_checksums stages the binary local checksum hash for validation.
function Copy-LocalBinaryChecksums() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )  
    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    
    $arch = $archInfo.Arch

    if ($CommitHash) {
        Write-InfoLog "staging local binary checksum from $Path/rke2.$suffix-$CommitHash.tar.gz.sha256sum"
        Copy-Item -Path "$Path/rke2.$suffix-$CommitHash.tar.gz.sha256sum" -Destination $DestinationPath -Force
        $expectedBinaryChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2.$suffix.tar.gz"
        $expectedBinaryAirgapChecksum = ""
        if (Test-Path -Path "$Path/rke2.$suffix-$CommitHash.tar.gz" -PathType Leaf) {
            # TODO: Select-String
            $expectedBinaryAirgapChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2.$suffix.tar.gz"
        }
        return @{ ExpectedBinaryChecksum = $expectedBinaryChecksum; ExpectedBinaryAirgapChecksum = $expectedBinaryAirgapChecksum }
    }
    else {
        Copy-Item -Path "$Path/sha256sum-$arch.txt" -Destination $DestinationPath -Force
        $expectedBinaryChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2.$suffix.tar.gz"
        $expectedBinaryAirgapChecksum = ""
        if (Test-Path -Path "$Path/rke2.$suffix.tar.gz" -PathType Leaf) {
            # TODO: Select-String
            $expectedBinaryAirgapChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2.$suffix.tar.gz"
        }
        return @{ ExpectedBinaryChecksum = $expectedBinaryChecksum; ExpectedBinaryAirgapChecksum = $expectedBinaryAirgapChecksum }
    }
     return @{ ExpectedBinaryChecksum = ""; ExpectedBinaryAirgapChecksum = "" }
}

function Copy-LocalImageChecksums() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )  
    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    
    $arch = $archInfo.Arch

    if ($CommitHash) {
        if (Test-Path -Path "$Path/rke2-images.$suffix-$CommitHash.tar.zst" -PathType Leaf) {
            $expectedImageAirgapChecksum = ""
            $expectedImageChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2-images.$suffix.tar.zst"
            Write-InfoLog "staging local image checksum from $Path/rke2-images.$suffix-$CommitHash.tar.zst.sha256sum"
            Copy-Item -Path "$Path/rke2-images.$suffix-$CommitHash.tar.zst.sha256sum" -Destination $DestinationPath -Force
            $expectedImageAirgapChecksum = Find-Checksum -ChecksumFilePath $TempImageChecksums -Pattern "rke2-images.$suffix.tar.zst"
            return @{ ExpectedImageChecksum = $expectedImageChecksum ; ExpectedImageAirgapChecksum = $expectedImageAirgapChecksum }
        }
    }
    # TODO: possibly add a condition where commithash is not set and no local checksums are present
    else {
        if (Test-Path -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.gz" -PathType Leaf) {
            $expectedImageAirgapChecksum = ""
            $expectedImageChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2-windows-$BuildVersion-$arch-images.tar.gz"
            Copy-Item -Path "$Path/sha256sum-$arch.txt" -Destination $DestinationPath -Force
            $expectedImageAirgapChecksum = Find-Checksum -ChecksumFilePath $TempImageChecksums -Pattern "rke2-windows-$BuildVersion-$arch-images.tar.gz"
            return @{ ExpectedImageChecksum = $expectedImageChecksum ; ExpectedImageAirgapChecksum = $expectedImageAirgapChecksum }
        }
        elseif (Test-Path -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.zst" -PathType Leaf) {
            $expectedImageAirgapChecksum = ""
            $expectedImageChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2-windows-$BuildVersion-$arch-images.tar.zst"
            Write-Host "staging local image checksums from $Path/sha256sum-$arch.txt"
            Copy-Item -Path "$Path/sha256sum-$arch.txt" -Destination $DestinationPath -Force
            $expectedImageAirgapChecksum = Find-Checksum -ChecksumFilePath $TempImageChecksums -Pattern "rke2-windows-$BuildVersion-$arch-images.tar.zst"
            return @{ ExpectedImageChecksum = $expectedImageChecksum ; ExpectedImageAirgapChecksum = $expectedImageAirgapChecksum }
        }
    }
    return @{ ExpectedImageChecksum = ""; ExpectedImageAirgapChecksum = "" }
}


# stage_local_tarball stages the local airgap binary tarball.
function Copy-LocalBinaryTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )
    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    

    if (-Not $CommitHash) {
        Write-InfoLog "staging local binary tarball from $ArtifactPath/rke2.$suffix.tar.gz"
        Copy-Item -Path "$Path/rke2.$suffix.tar.gz" -Destination $DestinationPath -Force
    }
    elseif (Test-Path -Path "$Path/rke2.$suffix-$CommitHash.tar.zst" -PathType Leaf) {
        Write-InfoLog "staging local binary tarball from $ArtifactPath/rke2.$suffix-$CommitHash.tar.zst"
        Copy-Item -Path "$Path/rke2.$suffix-$CommitHash.tar.zst" -Destination $DestinationPath -Force
        ]    
    }
}

# stage_local_airgap_tarball stages the local checksum hash for validation.
function Copy-LocalAirgapTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )   

    $archInfo = Get-ArchitectureInfo
    $arch = $archInfo.Arch
    $suffix = $archInfo.Suffix

    if (-Not $CommitHash) {
        if (Test-Path -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.zst" -PathType Leaf) {
            Write-InfoLog "staging local zst airgap image tarball from $Path/rke2-windows-$BuildVersion-$arch-images.tar.zst"
            Copy-Item -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.zst" -Destination $DestinationPath -Force
        }
        elseif (Test-Path -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.gz" -PathType Leaf) {
            Write-InfoLog "staging local gz airgap image tarball from $Path/rke2-windows-$BuildVersion-$arch-images.tar.gz"
            Copy-Item -Path "$Path/rke2-windows-$BuildVersion-$arch-images.tar.gz" -Destination $DestinationPath -Force
        }
    }
    elseif (Test-Path -Path "$Path/rke2-images.$suffix-$CommitHash.tar.zst" -PathType Leaf) {
        Write-InfoLog "staging local image tarball with commit $CommitHash from $Path/rke2-images.$suffix-$CommitHash.tar.zst"
        Copy-Item -Path "$Path/rke2-images.$suffix-$CommitHash.tar.zst" -Destination $DestinationPath -Force
    }
}


# verify_tarball verifies the downloaded installer checksum.
function Test-TarballChecksum() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $Tarball,
        [Parameter()]
        [String]
        $ExpectedChecksum
    )
    Write-InfoLog "verifying tarball"
    $actualChecksum = (Get-FileHash -Path $Tarball -Algorithm SHA256).Hash.ToLower()
    if ($ExpectedChecksum -ne $actualChecksum) {
        Write-FatalLog "downloaded sha256 does not match expected checksum: $ExpectedChecksum, instead got: $actualChecksum"
    }
}

# unpack_tarball extracts the tarball, correcting paths as necessary
function Expand-Tarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [string]
        $InstallPath,
        [Parameter()]
        [String]
        $Tarball
    )
    Write-InfoLog "unpacking tarball file to $InstallPath"
    New-Item -Path $InstallPath -Type Directory -Force | Out-Null
    tar xzf "$Tarball" -C "$InstallPath"
}

function Find-Checksum() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $ChecksumFilePath,
        [Parameter()]
        [String]
        $Pattern
    )
    try {
        $matchInfo = Select-String -Path $ChecksumFilePath -Pattern $Pattern
        if ($matchInfo) {
            return $matchInfo.Line.Split(" ")[0]   
        }
        return ""
    }
    catch {
        Write-FatalLog "Checksum file wasn't found: $ChecksumFilePath"  
    }     
}

function Test-Download {
    [CmdletBinding()]
    param (
        [Parameter()]
        [string]
        $Url
    )

    try {
        curl.exe --head -sfL $Url
        return $true
    } 
    catch { 
        return $false 
    }
}

# download_airgap_tarball downloads the airgap image tarball.
function Get-AirgapTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $TempAirgapTarball        
    )

    if (-Not $CommitHash) {
        return
    }    

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    
    $arch = $archInfo.Arch

    if ($CommitHash) {
        $AirgapTarballUrl = "$StorageUrl/rke2-images.$suffix-$CommitHash.tar.zst"
        Write-InfoLog "downloading airgap tarball with commit $CommitHash from $AirgapTarballUrl"
        curl.exe -sfL $AirgapTarballUrl -o $TempAirgapTarball
    }
    # prepare for windows airgap image bug fix
    else {
        $AirgapTarballUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/rke2-windows-$BuildVersion-$arch-images.tar.gz"
        Write-InfoLog "downloading airgap tarball from $AirgapTarballUrl"
        curl.exe -sfL $AirgapTarballUrl -o $TempAirgapTarball
    }
}

# verify_airgap_tarball compares the airgap image tarball checksum to the value
# calculated by CI when the file was uploaded.
function Test-AirgapTarballChecksum() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $ExpectedImageAirGapChecksum,
        [Parameter()]
        [String]
        $TempAirGapTarball
    )

    if (-Not $CommitHash) {
        return
    }

    if (-Not $ExpectedImageAirGapChecksum) {
        return
    }
    Write-InfoLog "verifying airgap tarball $TempAirGapTarball"
    $actualImageAirgapChecksum = (Get-FileHash -Algorithm SHA256 -Path "$TempAirGapTarball").Hash.ToLower()
    if ($ExpectedImageAirGapChecksum -ne $actualImageAirgapChecksum) {
        Write-FatalLog "downloaded sha256 does not match $ExpectedImageAirGapChecksum, got $actualImageAirgapChecksum"
    }
}

# install_airgap_tarball moves the airgap image tarball into place.
function Install-AirgapTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $InstallAgentImageDir,
        [Parameter()]
        [String]
        $TempAirgapTarball,
        [Parameter()]
        [String]
        $ExpectedImageAirGapChecksum,
        [Parameter()]
        [String]
        $TempImageChecksums
    )

    if (-Not $CommitHash) {
        return
    }

    if (-Not $ExpectedImageAirGapChecksum) {
        return
    }

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    
    $arch = $archInfo.Arch

    
    if ( -Not (Test-Path $InstallAgentImageDir)) {
        New-Item -Path $InstallAgentImageDir -ItemType Directory | Out-null
    }
    if ($CommitHash) {
        Write-InfoLog "installing airgap tarball with commit $CommitHash to $InstallAgentImageDir"
        Move-Item -Path $TempAirgapTarball -Destination "$InstallAgentImageDir/rke2-images.$suffix-$CommitHash.tar.zst" -Force
    }
    # prepare for windows airgap image bug fix
    else {
        Write-InfoLog "installing airgap tarball to $InstallAgentImageDir"
        if ([IO.Path]::GetExtension($TempAirgapTarball) -eq ".zst") {
            Move-Item -Path $TempAirgapTarball -Destination "$InstallAgentImageDir/rke2-windows-$BuildVersion-$arch-images.tar.zst" -Force
        }
        elseif ([IO.Path]::GetExtension($TempAirgapTarball) -eq ".gz") {
            Move-Item -Path $TempAirgapTarball -Destination "$InstallAgentImageDir/rke2-windows-$BuildVersion-$arch-images.tar.gz" -Force
        }
    }
}

# Globals
$STORAGE_URL = "https://rke2-ci-builds.s3.amazonaws.com"
$INSTALL_RKE2_GITHUB_URL = "https://github.com/rancher/rke2"

Confirm-WindowsFeatures -RequiredFeatures @("Containers")
Set-Environment 
Test-MethodConflict

switch ($Method) {
    "tar" { 
        $temp = ""
        if ($env:TMP) {
            $temp = $env:TMP
        }
        elseif ($env:TEMP) {
            $temp = $env:TEMP
        }
        if (Test-Path "$temp/rke2-install") {
            Remove-Item -Path "$temp/rke2-install" -Force -Recurse
        }
        New-Item -Path $temp -Name rke2-install -ItemType Directory | Out-Null

        $archInfo = Get-ArchitectureInfo
        $suffix = $archInfo.Suffix    
        $arch = $archInfo.Arch
        $BuildVersion = Get-BuildVersion        
        $TMP_DIR = Join-Path -Path $temp -ChildPath "rke2-install"
        $TMP_BINARY_CHECKSUMS = Join-Path -Path $TMP_DIR -ChildPath "rke2.checksums"
        $TMP_BINARY_TARBALL = Join-Path -Path $TMP_DIR -ChildPath "rke2.tarball"
        $TMP_AIRGAP_CHECKSUMS = Join-Path -Path $TMP_DIR -ChildPath "rke2-images.checksums"
        $TMP_AIRGAP_TARBALL = Join-Path -Path $TMP_DIR -ChildPath "rke2-images.tarball"

        if ($ArtifactPath) {
            if ($Commit) {
                $binaryChecksums = Copy-LocalBinaryChecksums -CommitHash $Commit -Path $ArtifactPath -DestinationPath $TMP_BINARY_CHECKSUMS
                $imageChecksums = Copy-LocalImageChecksums -CommitHash $Commit -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_CHECKSUMS
                $BINARY_CHECKSUM_EXPECTED = $binaryChecksums.ExpectedBinaryAirgapChecksum
                $AIRGAP_CHECKSUM_EXPECTED = $imageChecksums.ExpectedImageAirgapChecksum
                Copy-LocalAirgapTarball -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_TARBALL
                Copy-LocalBinaryTarball -Path $ArtifactPath -DestinationPath $TMP_BINARY_TARBAL
            }
            else {
                $binaryChecksums = Copy-LocalBinaryChecksums -Path $ArtifactPath -DestinationPath $TMP_BINARY_CHECKSUMS
                $imageChecksums = Copy-LocalImageChecksums -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_CHECKSUMS
                $BINARY_CHECKSUM_EXPECTED = $binaryChecksums.ExpectedBinaryAirgapChecksum
                $AIRGAP_CHECKSUM_EXPECTED = $imageChecksums.ExpectedImageAirgapChecksum
                Copy-LocalAirgapTarball -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_TARBALL
                Copy-LocalBinaryTarball -Path $ArtifactPath -DestinationPath $TMP_BINARY_TARBALL
            }
        }
        else {
            $Version = Get-ReleaseVersion
            Write-InfoLog "using $Version as release"
            Write-InfoLog "Version: $Version `r`nStorage URL: $STORAGE_URL `r`nGithub URL: $INSTALL_RKE2_GITHUB_URL `r`nBinary Checksums: $TMP_BINARY_CHECKSUMS `r`nImage Checksums: $TMP_AIRGAP_CHECKSUMS"
            $AIRGAP_CHECKSUM_EXPECTED = Get-ImageChecksums -CommitHash $Commit -StorageUrl $STORAGE_URL -Rke2Version $Version -Rke2GitHubUrl $INSTALL_RKE2_GITHUB_URL -TempImageChecksums $TMP_AIRGAP_CHECKSUMS
            Get-AirgapTarball -CommitHash $Commit -StorageUrl $STORAGE_URL -TempAirgapTarball $TMP_AIRGAP_TARBALL
            $BINARY_CHECKSUM_EXPECTED = Get-BinaryChecksums -CommitHash $Commit -StorageUrl $STORAGE_URL -Rke2Version $Version -Rke2GitHubUrl $INSTALL_RKE2_GITHUB_URL -TempBinaryChecksums $TMP_BINARY_CHECKSUMS
            Get-BinaryTarball -CommitHash $Commit -StorageUrl $STORAGE_URL -Rke2Version $Version -Rke2GitHubUrl $INSTALL_RKE2_GITHUB_URL -TempTarball $TMP_BINARY_TARBALL
        }
        Test-AirgapTarballChecksum -CommitHash $Commit -ExpectedImageAirgapChecksum $AIRGAP_CHECKSUM_EXPECTED -TempAirGapTarball $TMP_AIRGAP_TARBALL
        Install-AirgapTarball -CommitHash $Commit -InstallAgentImageDir $AgentImagesDir -TempAirgapTarball $TMP_AIRGAP_TARBALL -ExpectedImageAirgapChecksum $AIRGAP_CHECKSUM_EXPECTED -TempImageChecksums $TMP_AIRGAP_CHECKSUMS
        Test-TarballChecksum -Tarball $TMP_BINARY_TARBALL -ExpectedChecksum $BINARY_CHECKSUM_EXPECTED
        Expand-Tarball -InstallPath $TarPrefix -Tarball $TMP_BINARY_TARBALL
        Write-InfoLog "install complete; you may want to run:  `$env:PATH+=`";$TarPrefix\bin;C:\var\lib\rancher\rke2\bin`""
    }
    "choco" {  
        Write-FatalLog "Currently unsupported installation method. $Method will be supported soon.."
    }
    Default {
        Write-FatalLog "Invalid installation method. $Method not supported."
    }
}
