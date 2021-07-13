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
    Installation prefix when using the tar installation method.
    Default is C:/usr/local, unless C:/usr/local is read-only or has a dedicated mount point,
    in which case C:/opt/rke2 is used instead.`
.Parameter Commit
    Commit of RKE2 to download from temporary cloud storage.
    If set, this forces Method=tar.
    * (for developer & QA use only)`
.Parameter AgentImagesDir
    Installation path for airgap images when installing from CI commit
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
    ./install.ps1 -Channel Latest -Mehtod Tar
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
    $TarPrefix = "C:\usr\local",
    [Parameter()]
    [Switch]
    $Commit,
    [Parameter()]
    [String]
    $AgentImagesDir = "C:\var\lib\rancher\rke2\agent\images",
    [Parameter()]
    [String]
    $ArtifactPath = "",
    [Parameter()]
    [String]
    $ChannelUrl = "https://update.rke2.io/v1-release/channels"
)

$ErrorActionPreference = 'Stop'

function Write-InfoLog() {
    Write-Info "[INFO] " "$@"
}

function Write-WarnLog() {
    Write-Warn "[WARN] " "$@"
}

# fatal logs the given argument at fatal log level.
function Write-FatalLog() {
    Write-Output "[ERROR] " "$@"
    if ([string]::IsNullOrEmpty($SUFFIX)) {
        Write-Fatal "[ALT] Please visit 'https://github.com/rancher/rke2/releases' directly and download the latest rke2.$SUFFIX.tar.gz"
    }
    exit 1
}

# setup_env defines needed environment variables.
function Set-Environment()
{
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $DefaultTarPrefix
    )
    # --- bail if we are not administrator ---
    $adminRole = [Security.Principal.WindowsBuiltInRole]::Administrator
    $currentRole = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    If (-NOT $currentRole.IsInRole($adminRole))
    {
        Write-FatalLog "You need to be administrator to perform this install"
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
    if(-Not ($arch == "amd64")) {
        Write-FatalLog "unsupported architecture $(env:PROCESSOR_ARCHITECTURE)"
        exit 1
    }
    return @{ Suffix = "windows-$(env:ARCH)"; Arch = $arch}
}

# --- use desired rke2 version if defined or find version from channel ---
function Get-ReleaseVersion() {
	$version = ""
	if (-Not $Commit) {
		$version = "commit $($Commit)}"
	}
	elseif (-Not $Version) {
		$version = $Version
	}
	else {
		Write-InfoLog "finding release for channel $($Channel)"
		$versionUrl = "$INSTALL_RKE2_CHANNEL_URL}/$INSTALL_RKE2_CHANNEL"
		$result = [System.Net.HttpWebRequest]::Create($versionUrl).GetResponse().ResponseUri.Segments | Select-Object -Last 1 
		$lastDot = $result.LastIndexOf('.')
		$Version = $result.Substring(0, $lastDot)
	}
}

# download_checksums downloads hash from github url.
function Get-Checksums() {
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
        $TempChecksums
    )    

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    
    $arch = $archInfo.Arch
    $checksumsUrl = "" 

    if (-Not $CommitHash) {
        $checksumsUrl = "$StorageUrl/rke2.$suffix$CommitHash.tar.gz.sha256sum"
    }
    else {
        $checksumsUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/sha256sum-$arch.txt"
    }
    Write-InfoLog "downloading checksums at $checksumsUrl"
    Invoke-RestMethod -Uri $checksumsUrl -OutFile $TempChecksums
    return Find-Checksum -ChecksumFilePath $TempChecksums -Pattern "rke2.$suffix.tar.gz"
}

# download_tarball downloads binary from github url.
function Get-Tarball() {
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
    if (-Not $CommitHash) {
        $tarballUrl = "$StorageUrl/rke2.$suffix$CommitHash.tar.gz"
    }
    else {
        $tarballUrl = "$Rke2GitHubUrl/releases/download/$Rke2Version/rke2.$suffix.tar.gz"
    }

    Write-InfoLog "downloading tarball at $tarballUrl"
    Invoke-RestMethod -Uri $tarballUrl -OutFile $TempTarball
}

# stage_local_checksums stages the local checksum hash for validation.
function Copy-LocalChecksums() {
    [CmdletBinding()]
    param (
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

    Write-InfoLog "staging local checksums from $-Path/sha256sum-$arch.txt"
    Copy-Item -Path "$-Path/sha256sum-$ARCH.txt" -Destination $DestinationPath -Force
    
    #TODO: 
    $expectedChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2.$suffix.tar.gz"
    $expectedAirgapChecksum = ""
    
    if (Test-Path -Path "$Path/rke2-images.$suffix.tar.zst" -PathType Leaf) {
        # TODO: Select-String
        $expectedAirgapChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2-images.$suffix.tar.zst"
    }
    elseif (Test-Path -Path "$Path/rke2-images.$suffix.tar.gz" -PathType Leaf) {     
        # TODO: Select-String   
        $expectedAirgapChecksum = Find-Checksum -ChecksumFilePath $DestinationPath -Pattern "rke2-images.$suffix.tar.gz"
    }

    return @{ ExpectedChecksum = $expectedChecksum; ExpectedAirgapChecksum = $expectedAirgapChecksum }
}

# stage_local_tarball stages the local tarball.
function Copy-LocalTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )       
    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix  
    Write-InfoLog "staging tarball from $ArtifPathactPath/rke2.$suffix.tar.gz"
    Copy-Item -Path "$Path/rke2.$suffix.tar.gz" -Destination $DestinationPath -Force
}

# stage_local_airgap_tarball stages the local checksum hash for validation.
function Copy-LocalAirgapTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $Path,
        [Parameter()]
        [String]
        $DestinationPath
    )   

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix  
    if (!(Test-Path -Path "$Path/rke2-images.$suffix.tar.zst" -PathType Leaf)) {
        Write-InfoLog "staging zst airgap image tarball from $Path/rke2-images.$suffix.tar.zst"
        Copy-Item -Path "$Path/rke2-images.$suffix.tar.zst" -Destination $DestinationPath -Force
        return "zst"
    }
    elseif (!(Test-Path -Path "$Path/rke2-images.$suffix.tar.gz" -PathType Leaf)) {
        Write-InfoLog "staging gzip airgap image tarball from $Path/rke2-images.$suffix.tar.gz"
        Copy-Item -Path "$Path/rke2-images.$suffix.tar.gz" -Destination $DestinationPath -Force
        return "gz"
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
        Write-FatalLog "download sha256 does not match $ExpectedChecksum, got $actualChecksum"
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
    New-Item -Path $InstallPath -Type Directory -Force
    tar xzf "$Tarball" -C "$InstallPath"
    Write-InfoLog "install complete; you may want to run:  `$env:PATH+=`";$INSTALL_RKE2_TAR_PREFIX\bin`""    
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
        if($matchInfo) {
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
        Invoke-WebRequest -Uri $Url -Method Head 
        return $true
    } 
    catch { 
        return $false 
    }
}

# download_airgap_checksums downloads the checksum file for the airgap image tarball
# and prepares the checksum value for later validation.
function Get-AirgapChecksums() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $CommitHash,
        [Parameter()]
        [String]
        $AirgapChecksumsUrl,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $TempAirgapChecksums
    )
    
    if ($CommitHash){
        return
    }

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix  

    $AirgapChecksumsUrl= "$StorageUrl/rke2-images.$suffix$CommitHash.tar.zst.sha256sum"
    # try for zst first; if that fails use gz for older release branches
    if (!(Test-Download -Uri $AirgapChecksumsUrl)) {
        $AirgapChecksumsUrl = "$StorageUrl/rke2-images.$suffix$CommitHash.tar.gz.sha256sum"
    }
    Write-InfoLog "downloading airgap checksums at $AirgapChecksumsUrl"
    Invoke-RestMethod -Uri $AirgapChecksumsUrl -OutFile $-TempAirgapChecksums
    return Find-Checksum -Path $-TempAirgapChecksums -Pattern "rke2-images.$suffix.tar"
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
        $AirgapTarballUrl,
        [Parameter()]
        [String]
        $StorageUrl,
        [Parameter()]
        [String]
        $TempAirgapTarball        
    )

    if ($CommitHash){
        return
    }    

    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix    

    $AirgapTarballUrl= "$StorageUrl/rke2-images.$suffix$CommitHash.tar.zst"

    # try for zst first; if that fails use gz for older release branches
    if (!(Test-Download -Url $AirgapTarballUrl)) {
        $AirgapTarballUrl = "$StorageUrl/rke2-images.$suffix$CommitHash.tar.gz"
    }
    Write-InfoLog "downloading airgap tarball at $AirgapTarballUrl"
    Invoke-RestMethod -Uri $AirgapTarballUrl -OutFile $TempAirgapTarball
}

# verify_airgap_tarball compares the airgap image tarball checksum to the value
# calculated by CI when the file was uploaded.
function Test-AirgapTarballChecksum() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $ExpectedAirGapChecksum,
        [Parameter()]
        [String]
        $TempAirGapTarball
    )

    if ($ExpectedAirGapChecksum) {
        return
    }
    Write-InfoLog "verifying airgap tarball"
    $actualAirgapChecksum = $(sha256sum "$TempAirGapTarball" | awk '{print $1}')
    if ($ExpectedAirGapChecksum -ne $actualAirgapChecksum) {
        Write-FatalLog "download sha256 does not match $ExpectedAirGapChecksum, got $actualAirgapChecksum"
    }
}

# install_airgap_tarball moves the airgap image tarball into place.
function Install-AirgapTarball() {
    [CmdletBinding()]
    param (
        [Parameter()]
        [String]
        $InstallAgentImageDir,
        [Parameter()]
        [String]
        $TempAirgapTarball,
        [Parameter()]
        [String]
        $ExpectedAirGapChecksum,
        [Parameter()]
        [String]
        $AirgapTarballFormat,
        [Parameter()]
        [String]
        $TempAirgapChecksums
    )    

    if ($ExpectedAirGapChecksum) {
        return
    }
    New-Item -Path "$InstallAgentImageDir" -ItemType "Directory"
    $archInfo = Get-ArchitectureInfo
    $suffix = $archInfo.Suffix

    Write-InfoLog "installing airgap tarball to $InstallAgentImageDir"
    Move-Item -Path $TempAirgapTarball -Destination "$InstallAgentImageDir/rke2-images.$suffix.tar.zst" -Force
}

# Globals
$STORAGE_URL = "https://storage.googleapis.com/rke2-ci-builds"
$INSTALL_RKE2_GITHUB_URL = "https://github.com/rancher/rke2"
$DEFAULT_TAR_PREFIX = "C:\usr\local"

Set-Environment -DefaultTarPrefix $DEFAULT_TAR_PREFIX
Test-MethodConflict

switch ($Method) {
    "tar" { 
        $temp = ""
        if($env:TMP){
            $temp = $env:TMP
        }
        elseif($env:TEMP){
            $temp = $env:TEMP
        }
        New-Item -Path $temp -Name "rke2-install"  -ItemType "Directory"
        
        $TMP_DIR = Join-Path -Path $temp -ChildPath "rke2-install"
        $TMP_CHECKSUMS = Join-Path -Path $TMP_DIR -ChildPath "rke2.checksums"
        $TMP_TARBALL = Join-Path -Path $TMP_DIR -ChildPath "rke2.tarball"
        $TMP_AIRGAP_CHECKSUMS = Join-Path -Path $TMP_DIR -ChildPath "rke2-images.checksums"
        $TMP_AIRGAP_TARBALL = Join-Path -Path $TMP_DIR -ChildPath "rke2-images.tarball"	

        if (-Not $ArtifactPath){
            $checksums = Copy-LocalChecksums -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_CHECKSUMS
            $CHECKSUM_EXPECTED = $checksums.ExpectedChecksum
            $AIRGAP_CHECKSUM_EXPECTED = $checksums.ExpectedAirgapChecksum
    
            $AIRGAP_TARBALL_FORMAT = Copy-LocalAirgapTarball -Path $ArtifactPath -DestinationPath $TMP_AIRGAP_TARBALL        
            Copy-LocalTarball -Path $ArtifactPath -DestinationPath $TMP_TARBAL
        }
        else {
            Get-ReleaseVersion
            Write-InfoLog "using ${Version: -commit $Commit} as release"
            $AIRGAP_CHECKSUM_EXPECTED = Get-AirgapChecksums -CommitHash $Commit -AirgapChecksumsUrl $AIRGAP_CHECKSUMS_URL -StorageUrl $STORAGE_URL -TempAirgapChecksums $TMP_AIRGAP_CHECKSUMS
            Get-AirgapTarball -CommitHash $Commit -AirgapTarballUrl $AIRGAP_TARBALL_URL -StorageUrl $STORAGE_URL -TempAirgapTarball $TMP_AIRGAP_TARBALL
            $CHECKSUM_EXPECTED = Get-Checksums -CommitHash $Commit -StorageUrl $STORAGE_URL -Rke2Version $Version -Rke2GitHubUrl $INSTALL_RKE2_GITHUB_URL -TempChecksums $TMP_CHECKSUMS   
            Get-Tarball -CommitHash $Commit -StorageUrl $STORAGE_URL -Rke2Version $Version -Rke2GitHubUrl $INSTALL_RKE2_GITHUB_URL -TempTarball $TMP_TARBALL
        }
    
        Test-AirgapTarballChecksum -ExpectedAirGapChecksum $AIRGAP_CHECKSUM_EXPECTED -TempAirGapTarball $TMP_AIRGAP_TARBALL   
        Install-AirgapTarball -InstallAgentImageDir $INSTALL_RKE2_AGENT_IMAGES_DIR -TempAirgapTarball $TMP_AIRGAP_TARBALL -ExpectedAirGapChecksum $AIRGAP_CHECKSUM_EXPECTED -AirgapTarballFormat $AIRGAP_TARBALL_FORMAT -TempAirgapChecksums $TMP_AIRGAP_CHECKSUMS
        Test-TarballChecksum -Tarball $TMP_TARBALL -ExpectedChecksum $CHECKSUM_EXPECTED
        Expand-Tarball -InstallPath $INSTALL_RKE2_TAR_PREFIX -Tarball $TMP_TARBALL
     }
    "choco" {  
        Write-FatalLog "Currently unsupported installation method. $Method will be supported soon.."
    }
    Default {
        Write-FatalLog "Invalid installation method. $Method not supported."
    }
}
exit 0
