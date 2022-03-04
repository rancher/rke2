$CommitFile = "./commits.txt"
# (Invoke-RestMethod https://api.github.com/repos/rancher/rke2/commits?per_page=5).sha | `
# Out-File -FilePath $CommitFile

$StorageUrl = "https://storage.googleapis.com/rke2-ci-builds/rke2-images.windows-amd64-"
$TopCommit = (head -n 1 $CommitFile)
$StatusCode = Invoke-WebRequest $StorageUrl$TopCommit".tar.zst.sha256sum" -DisableKeepAlive -UseBasicParsing -Method head | % {$_.StatusCode}
while ($StatusCode -ne 200 ) {
    (Get-Content $CommitFile | Select-Object -Skip 1) | Set-Content $CommitFile
    $TopCommit = (head -n 1 $CommitFile)
    $StatusCode = Invoke-WebRequest $StorageUrl$TopCommit".tar.zst.sha256sum" -DisableKeepAlive -UseBasicParsing -Method head | % {$_.StatusCode}
}