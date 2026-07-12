#!/usr/bin/env bash
set -euo pipefail

out="${RUNNER_TEMP:?}/amagi_belfast_json_mvp"
report="$out/belfast-json-mvp-report.json"

python3 - "$out" "$report" <<'PY'
import json
import pathlib
import sys

out = pathlib.Path(sys.argv[1])
report_path = pathlib.Path(sys.argv[2])
report = json.loads(report_path.read_text(encoding="utf-8"))

reference = pathlib.Path("reports/golden-compatibility/reference-manifest.csv")
expected = {line.split(",", 1)[0] for line in reference.read_text(encoding="utf-8").splitlines()[1:] if line}
actual = {p.relative_to(out).as_posix() for p in out.rglob("*") if p.is_file() and p.name != report_path.name}

missing = sorted(expected - actual)
extra = sorted(actual - expected)
relevant_missing = (set(report.get("missing_source_files", [])) & expected) - actual
relevant_unsupported = (set(report.get("unsupported_files", [])) & expected) - actual
lua_generated = len(report.get("generated_files", []))
helper_paths = {
    "global/build_pools.json",
    "global/build_times.json",
    "global/requisition_ships.json",
    "global/versions.json",
}
helpers = len(actual & helper_paths)
fallback = report.get("fallback_file_reports", [])
fallback_paths = {x.get("relative_path") for x in fallback}

checks = {
    "all_outputs": len(actual) == 628,
    "missing": not relevant_missing and not missing,
    "extra": not extra,
    "lua_generated": lua_generated >= 622,
    "helpers": helpers == 4,
    "legacy_fallback": len(fallback) == 2 and report.get("total_fallback_count") == 2 and fallback_paths == {"JP/ShareCfg/card_affix.json", "JP/ShareCfg/card_template.json"} and all(x.get("source_kind") == "legacy_belfast_fallback" for x in fallback),
    "unsupported": not relevant_unsupported and not report.get("unsupported_helper_files"),
}
for name, ok in checks.items():
    print(f"{name}: {'pass' if ok else 'fail'}")
if missing:
    print("missing paths:", *missing, sep="\n  ")
if relevant_missing:
    print("missing source paths:", *sorted(relevant_missing), sep="\n  ")
if relevant_unsupported:
    print("unsupported paths:", *sorted(relevant_unsupported), sep="\n  ")
if report.get("unsupported_helper_files"):
    print("unsupported helper paths:", *report.get("unsupported_helper_files"), sep="\n  ")
if extra:
    print("extra paths:", *extra, sep="\n  ")
if not all(checks.values()):
    raise SystemExit(1)
PY

second="${RUNNER_TEMP}/amagi_belfast_json_mvp_second"
rm -rf -- "$second"
go run ./cmd/belfast_json_mvp \
  -source-root "$GITHUB_WORKSPACE" \
  -luascripts-root "$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts" \
  -legacy-fallback-root "$GITHUB_WORKSPACE" \
  -output-root "$second"

diff -r -q \
  <(find "$out" -type f ! -name belfast-json-mvp-report.json -print0 | sort -z | xargs -0 -I{} sh -c 'printf "%s  " "${1#'"$out"'/}"; sha256sum "$1"' _ {}) \
  <(find "$second" -type f ! -name belfast-json-mvp-report.json -print0 | sort -z | xargs -0 -I{} sh -c 'printf "%s  " "${1#'"$second"'/}"; sha256sum "$1"' _ {})
echo "second-run diff: 0"
git diff --check
