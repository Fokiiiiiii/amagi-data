$ErrorActionPreference = "Stop"
function Invoke-Native {
  param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Args)
  & $Args[0] $Args[1..($Args.Count - 1)]
  if ($LASTEXITCODE -ne 0) {
    throw "$($Args -join ' ') failed with exit code $LASTEXITCODE"
  }
}
$env:AMAGI_DATA_TEST_AZURLANE_ROOT = "$env:GITHUB_WORKSPACE\_external\AzurLaneData"
$env:AMAGI_DATA_TEST_LUASCRIPTS_ROOT = "$env:GITHUB_WORKSPACE\_external\AzurLaneLuaScripts"
Write-Host "AzurLaneData root exists: $(Test-Path $env:AMAGI_DATA_TEST_AZURLANE_ROOT)"
Write-Host "AzurLaneLuaScripts root exists: $(Test-Path $env:AMAGI_DATA_TEST_LUASCRIPTS_ROOT)"
Invoke-Native go test ./...
