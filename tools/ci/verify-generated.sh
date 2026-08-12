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
source = pathlib.Path.cwd() / "_external" / "AzurLaneLuaScripts"

actual = {p.relative_to(out).as_posix() for p in out.rglob("*") if p.is_file() and p.name != report_path.name}

missing = sorted(report.get("missing_source_files", []))
extra = []
relevant_missing = set(missing)
relevant_unsupported = set(report.get("unsupported_files", []))
relevant_unsupported_helpers = set(report.get("unsupported_helper_files", []))
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
expected_fallback_paths = {
    "CN/ShareCfg/card_affix.json", "CN/ShareCfg/card_template.json",
    "JP/ShareCfg/card_affix.json", "JP/ShareCfg/card_template.json",
    "TW/ShareCfg/card_affix.json", "TW/ShareCfg/card_template.json",
}
gamecfg_missing = []
for region in ("CN", "EN", "JP", "KR", "TW"):
    source_names = {"buff": "buff", "card": "card", "dorm": "dorm", "dungeon": "dungeon", "skill": "skill", "story": "storyjp" if region == "JP" else "story"}
    for target, source_name in source_names.items():
        if (source / region / "gamecfg" / source_name).is_dir() and not (out / region / "GameCfg" / f"{target}.json").is_file():
            gamecfg_missing.append(f"{region}/GameCfg/{target}.json")
stream_mismatches = []
for facade in source.glob("*/sharecfg/*.lua"):
    if "__stream__ = true" not in facade.read_text(encoding="utf-8"):
        continue
    region = facade.parents[1].name
    name = facade.with_suffix(".json").name
    upper = out / region / "ShareCfg" / name
    lower = out / region / "sharecfgdata" / name
    if not upper.is_file() or not lower.is_file():
        stream_mismatches.append(f"{region}/{name}: missing generated pair")
        continue
    if json.loads(upper.read_text(encoding="utf-8")) != json.loads(lower.read_text(encoding="utf-8")):
        stream_mismatches.append(f"{region}/{name}: ShareCfg differs from sharecfgdata")

checks = {
    "all_outputs": len(actual) > 0,
    "missing": not relevant_missing,
    "extra": not extra,
    "lua_generated": lua_generated >= 622,
    "helpers": helpers == 4,
    "legacy_fallback": len(fallback) == len(expected_fallback_paths) and report.get("total_fallback_count") == len(expected_fallback_paths) and fallback_paths == expected_fallback_paths and all(x.get("source_kind") == "legacy_belfast_fallback" for x in fallback),
    "unsupported": not relevant_unsupported and not relevant_unsupported_helpers,
    "stream_backing": not stream_mismatches,
    "gamecfg": not gamecfg_missing,
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
if stream_mismatches:
    print("stream backing mismatches:", *stream_mismatches, sep="\n  ")
if gamecfg_missing:
    print("missing GameCfg outputs:", *gamecfg_missing, sep="\n  ")
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
git -c core.whitespace=cr-at-eol diff --check
echo "git diff --check: pass"
