#!/usr/bin/env bash
set -euo pipefail

source_root="${AMAGI_UPSTREAM_ROOT:-$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts}"
export AMAGI_DATA_TEST_LUASCRIPTS_ROOT="$source_root"

if [ -d "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" ]; then
  echo "AzurLaneLuaScripts root exists: True"
else
  echo "AzurLaneLuaScripts root exists: False"
fi

out="$RUNNER_TEMP/amagi_data_generation"
rm -rf -- "$out"

args=(
  -source-root "$GITHUB_WORKSPACE"
  -luascripts-root "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT"
  -legacy-fallback-root "$GITHUB_WORKSPACE"
  -output-root "$out"
)
if [[ "${AMAGI_MODE:-full}" == "incremental" ]]; then
  : "${AMAGI_INCREMENTAL_PLAN:?AMAGI_INCREMENTAL_PLAN is required for incremental generation}"
  args+=(-incremental-plan "$AMAGI_INCREMENTAL_PLAN")
fi
go run ./cmd/generate_data "${args[@]}"

if [ ! -d "$out" ]; then
  echo "output dir missing: $out" >&2
  exit 1
fi

report_path="$out/generation-report.json"
if [ ! -f "$report_path" ]; then
  echo "report missing: $report_path" >&2
  exit 1
fi

if [[ "${AMAGI_MODE:-full}" != "incremental" ]]; then
python3 - "$report_path" "$GITHUB_WORKSPACE" <<'PY'
import json
import pathlib
import sys

report = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
workspace = pathlib.Path(sys.argv[2])
expected = set()
for key in ("generated_files", "generated_helper_files", "fallback_files", "fallback_helper_files"):
  expected.update(report.get(key, []))

roots = []
for region in ("CN", "EN", "JP", "KR", "TW"):
  for category in ("GameCfg", "ShareCfg", "sharecfgdata"):
    roots.append(workspace / region / category)
for root in roots:
  if not root.is_dir():
    continue
  for path in sorted(root.rglob("*")):
    if not path.is_file():
      continue
    rel = path.relative_to(workspace).as_posix()
    if rel not in expected:
      path.unlink()
      print(f"  - deleted stale {rel}")
PY
fi

echo "Publishing generated files to repository working directory"
publish_entry() {
  local entry=$1
  local rel=${entry#"$out"/}
  local destination="$GITHUB_WORKSPACE/$rel"
  case "$rel" in
    ""|/*|../*|*/../*)
      echo "invalid generated path: $rel" >&2
      exit 1
      ;;
  esac
  mkdir -p -- "$(dirname "$destination")"
  rm -rf -- "$destination"
  cp -r -- "$entry" "$destination"
  echo "  - $rel"
}

if [[ "${AMAGI_MODE:-full}" == "incremental" ]]; then
  while IFS= read -r -d '' entry; do
    publish_entry "$entry"
  done < <(find "$out" -type f ! -name "generation-report.json" -print0)
else
  for entry in "$out"/*; do
    [[ "$(basename "$entry")" == "generation-report.json" ]] && continue
    publish_entry "$entry"
  done
fi

if [[ "${AMAGI_MODE:-full}" == "incremental" ]]; then
  jq -r '.delete_outputs[]?' "$AMAGI_INCREMENTAL_PLAN" | while IFS= read -r rel; do
    case "$rel" in
      ""|/*|../*|*/../*)
        echo "invalid deleted path: $rel" >&2
        exit 1
        ;;
    esac
    rm -rf -- "$GITHUB_WORKSPACE/$rel"
    echo "  - deleted $rel"
  done
fi
