# Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
param ($Branch, $CommitFile)
(Invoke-RestMethod "https://api.github.com/repos/rancher/rke2/commits?per_page=5&sha=$Branch").sha | `
Out-File -FilePath $CommitFile

$StorageUrl = "https://storage.googleapis.com/rke2-ci-builds/rke2-images.windows-amd64-"
$TopCommit = (Get-Content -TotalCount 1 $CommitFile)
$StatusCode = Invoke-WebRequest $StorageUrl$TopCommit".tar.zst.sha256sum" -DisableKeepAlive -UseBasicParsing -Method head | % {$_.StatusCode}
$Iterations = 0
while (($StatusCode -ne 200) -AND ($Iterations -lt 6)) {
    $Iterations++
    (Get-Content $CommitFile | Select-Object -Skip 1) | Set-Content $CommitFile
    $TopCommit = (Get-Content -TotalCount 1 $CommitFile)
    $StatusCode = Invoke-WebRequest $StorageUrl$TopCommit".tar.zst.sha256sum" -DisableKeepAlive -UseBasicParsing -Method head | % {$_.StatusCode}
}

if ($Iterations -ge 6){
    Write-Host echo "No valid commits found" 
    Exit 1
}