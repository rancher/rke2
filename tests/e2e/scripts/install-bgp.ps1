echo "Installing RemoteAccess, RSAT-RemoteAccess-PowerShell and Routing packages"
Install-WindowsFeature RemoteAccess
Install-WindowsFeature RSAT-RemoteAccess-PowerShell
Install-WindowsFeature Routing
