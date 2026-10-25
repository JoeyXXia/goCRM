$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$target = Join-Path $projectRoot 'server/config.local.yaml'
if (Test-Path -LiteralPath $target) {
    throw 'server/config.local.yaml already exists; keep it or move it manually before preparing again.'
}
$source = Get-Content -LiteralPath (Join-Path $projectRoot 'server/config.yaml') -Raw
$key = [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
$source = $source.Replace('signing-key: qmPlus', "signing-key: $key")
Set-Content -LiteralPath $target -Value $source -Encoding utf8
Write-Output 'Created ignored server/config.local.yaml with a fresh JWT signing key.'
Write-Output 'From server/: go run . -c config.local.yaml'
Write-Output 'Initialize a dedicated MySQL database using the frontend; do not edit tracked config.yaml with credentials.'
