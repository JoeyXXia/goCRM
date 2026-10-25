param([string]$BaseUrl = 'http://127.0.0.1:8888')
$ErrorActionPreference = 'Stop'
$health = Invoke-RestMethod "$BaseUrl/health" -NoProxy -TimeoutSec 15
if ($health -ne 'ok') { throw 'Health response was not ok.' }
$init = Invoke-RestMethod "$BaseUrl/init/checkdb" -NoProxy -Method Post -TimeoutSec 15
if ($init.code -ne 0 -or $null -eq $init.data.needInit) { throw 'Unexpected initialization response.' }
$denied = Invoke-RestMethod "$BaseUrl/menu/getMenu" -NoProxy -Method Post -TimeoutSec 15 -SkipHttpErrorCheck -StatusCodeVariable denialStatus
if ($denied.code -ne 7 -or $denialStatus -ne 401) { throw 'Expected anonymous menu rejection with HTTP 401 and code 7.' }
Write-Output "PASS health; needInit=$($init.data.needInit); anonymous menu access rejected (code=$($denied.code))."
