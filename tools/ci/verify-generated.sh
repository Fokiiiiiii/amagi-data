#!/usr/bin/env bash
set -euo pipefail

out="${RUNNER_TEMP:?}/amagi_belfast_json_mvp"
report="$out/belfast-json-mvp-report.json"

python3 - "$out" "$report" <<'PY'
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
relevant_unsupported_helpers = (set(report.get("unsupported_helper_files", [])) & expected) - actual
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
    "unsupported": not relevant_unsupported and not relevant_unsupported_helpers,
}
for name, ok in checks.items():
    print(f"{name}: {'pass' if ok else 'fail'}")
if missing:
    print("missing paths:", *missing, sep="\n  ")
if relevant_missing:
    print("missing source paths:", *sorted(relevant_missing), sep="\n  ")
if relevant_unsupported:
    print("unsupported paths:", *sorted(relevant_unsupported), sep="\n  ")
if relevant_unsupported_helpers:
    print("unsupported helper paths:", *sorted(relevant_unsupported_helpers), sep="\n  ")
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

python3 - "$second" <<'PY'
import pathlib
import sys

out = pathlib.Path(sys.argv[1])
expected = {
    line.split(",", 1)[0]
    for line in pathlib.Path("reports/golden-compatibility/reference-manifest.csv")
    .read_text(encoding="utf-8")
    .splitlines()[1:]
    if line
}
for path in out.rglob("*"):
    if path.is_file() and path.name != "belfast-json-mvp-report.json" and path.relative_to(out).as_posix() not in expected:
        path.unlink()
PY

python3 - "$out" "$second" <<'PY'
import hashlib
import json
import pathlib
import sys

def hashes(root):
    root = pathlib.Path(root)
    result = {}
    for path in root.rglob("*"):
        if path.is_file() and path.name != "belfast-json-mvp-report.json":
            result[path.relative_to(root).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result

first, second = hashes(sys.argv[1]), hashes(sys.argv[2])
if first != second:
    print("second-run differences:")
    for path in sorted(set(first) | set(second)):
        if first.get(path) != second.get(path):
            print(path)
    raise SystemExit(1)
PY
echo "second-run diff: 0"
git diff --check
