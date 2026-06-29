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
$env:AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT = "$env:GITHUB_WORKSPACE\_external\belfast-data"
Write-Host "AzurLaneData root exists: $(Test-Path $env:AMAGI_DATA_TEST_AZURLANE_ROOT)"
Write-Host "AzurLaneLuaScripts root exists: $(Test-Path $env:AMAGI_DATA_TEST_LUASCRIPTS_ROOT)"
Write-Host "belfast-data root exists: $(Test-Path $env:AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT)"
Write-Host "belfast item_data_statistics exists: $(Test-Path (Join-Path $env:AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT 'JP/sharecfgdata/item_data_statistics.json'))"
Write-Host "belfast build_pools exists: $(Test-Path (Join-Path $env:AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT 'build_pools.json'))"
Invoke-Native go test ./...
$out = Join-Path $env:RUNNER_TEMP "amagi_belfast_json_mvp"
Invoke-Native go run ./cmd/belfast_json_mvp `
  -source-root $env:AMAGI_DATA_TEST_AZURLANE_ROOT `
  -luascripts-root $env:AMAGI_DATA_TEST_LUASCRIPTS_ROOT `
  -output-root $out `
  -copy-helper-fallback-from $env:AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT
