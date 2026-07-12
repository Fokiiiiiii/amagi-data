# amagi-data

AzurLaneLuaScriptsのLuaデータを、Belfast互換のJSONへ変換するツールです。

## Phase 1 golden verification

基準referenceは`ggmolly/belfast-data`のcommit
`33c61c5c239e267e77573d945bfcca691114d60f`です。JPの基準Lua snapshotは
`AzurLaneTools/AzurLaneLuaScripts`のcommit
`60a5349677dd3bc907152e09cb0fef5f4ad7bab1`（JP `9.2.821`）として固定しています。
根拠は`reports/golden-compatibility/source-identification.json`に保存します。

Lua-derivedの生成はLuaを直接入力にします。例外として、Lua tableを持たないBelfast独自helper
`global/build_pools.json`、`global/build_times.json`、`global/requisition_ships.json`は
reference形式を基準にstatic helperとして別検証します。`global/versions.json`は
`AzurLaneLuaScripts/versions/*.txt`から別生成します。

```powershell
go run ./cmd/belfast_json_mvp `
  -luascripts-root _external/AzurLaneLuaScripts `
  -reference-root C:\Users\yutai\belfast-data `
  -copy-helper-fallback-from C:\Users\yutai\belfast-data `
  -version-source-map reports/golden-compatibility/version-source-map.json `
  -output-root _generated/golden-jp `
  -report-path reports/golden-compatibility/generation-report.json

go run ./cmd/belfast_golden_verify `
  -reference-root C:\Users\yutai\belfast-data\JP `
  -generated-root _generated/golden-jp\JP `
  -manifest reports/golden-compatibility/reference-manifest.csv `
  -report reports/golden-compatibility/report.json

go run ./cmd/belfast_phase1_report `
  -reference-root C:\Users\yutai\belfast-data\JP `
  -generated-root _generated/golden-jp\JP `
  -helper-reference-root C:\Users\yutai\belfast-data `
  -generated-output-root _generated/golden-jp `
  -lua-repository-root _external/AzurLaneLuaScripts `
  -output-root reports/golden-compatibility
```

比較レポートは`reports/golden-compatibility/`へ保存します。`phase1-summary.json`は
`lua_derived`、`belfast_helpers`、`all_outputs`を分離します。helperの個別hashは
`helper-files.csv`に記録します。固定対象は`reference-manifest.csv`と
`manifest-summary.json`に保存します。referenceの5地域versionは
`version-reference.json`、snapshot候補は`version-snapshot-candidates.csv`に保存します。
地域別source mapは`version-source-map.json`、Lua残差の原因別集計は
`mismatch-root-causes.csv`に保存します。
比較はSHA-256、サイズ、ファイル一覧、最初のbyte差分を含み、
1byteの差分も失敗にします。JSONはcompact UTF-8、HTML escapeなし、CRLF trailing newlineで出力します。

現在はPhase 1未完了です。Lua parserはrestricted parserで、任意Luaコードを実行しません。残作業はShareCfgのnested table key order保持、全対象ファイルのLua入力対応、全SHA一致の確認です。Phase 1完了後に別タスクで最新Luaへの切り替え（Phase 2）を行います。

## Phase 1一括確認

実行時の生成先は一時ディレクトリへ置き、既存の未commit変更を保持したまま確認します。

```powershell
cd C:\Users\yutai\amagi-data
$ErrorActionPreference = "Stop"
function Invoke-Native {
    param([Parameter(Mandatory=$true)][string]$File,
          [Parameter(ValueFromRemainingArguments=$true)][string[]]$Args)
    & $File @Args
    if ($LASTEXITCODE -ne 0) { throw "$File $($Args -join ' ') failed with exit code $LASTEXITCODE" }
}
Invoke-Native git status --short
Invoke-Native git branch --show-current
Invoke-Native git remote -v
Invoke-Native go test ./...
Invoke-Native go run ./cmd/belfast_golden_verify `
    -reference-root C:\Users\yutai\belfast-data\JP `
    -generated-root work\golden-jp-second-final\JP `
    -manifest reports\golden-compatibility\reference-manifest.csv `
    -report reports\golden-compatibility\report-current.json
Invoke-Native go run ./cmd/belfast_phase1_report `
    -reference-root C:\Users\yutai\belfast-data\JP `
    -generated-root work\golden-jp-second-final\JP `
    -helper-reference-root C:\Users\yutai\belfast-data `
    -generated-output-root work\golden-jp-second-final `
    -lua-repository-root _external\AzurLaneLuaScripts `
    -output-root reports\golden-compatibility
Invoke-Native git diff --check
```

`residual-dossier.csv/json` は比較実行時点の残差から自動生成します。残差がある場合、report commandは非ゼロ終了します。

現行snapshotに存在しない`item_data_template.lua`は、最後に一致した履歴commitから復元したsource overrideとして
`reports/golden-compatibility/source-overrides.csv`に記録します。`dorm3d_ik_timeline_controller.json`は
LuaJITのmodule-not-found error artifactを`error-artifacts.csv`で別記録します。

## Acknowledgment List 

https://github.com/AzurLaneTools

https://github.com/ggmolly/belfast

Thanks!
