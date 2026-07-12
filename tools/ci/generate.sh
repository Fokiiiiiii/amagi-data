#!/usr/bin/env bash
set -euo pipefail

export AMAGI_DATA_TEST_LUASCRIPTS_ROOT="$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts"

if [ -d "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" ]; then
  echo "AzurLaneLuaScripts root exists: True"
else
  echo "AzurLaneLuaScripts root exists: False"
fi

out="$RUNNER_TEMP/amagi_belfast_json_mvp"

go run ./cmd/belfast_json_mvp \
  -source-root "$GITHUB_WORKSPACE" \
  -luascripts-root "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" \
  -legacy-fallback-root "$GITHUB_WORKSPACE" \
  -output-root "$out"

if [ ! -d "$out" ]; then
  echo "output dir missing: $out" >&2
  exit 1
fi

report_path="$out/belfast-json-mvp-report.json"
if [ ! -f "$report_path" ]; then
  echo "report missing: $report_path" >&2
  exit 1
fi

echo "Publishing generated files to repository working directory"
for entry in "$out"/*; do
  name="$(basename "$entry")"
  if [ "$name" = "belfast-json-mvp-report.json" ]; then
    continue
  fi
  if [ -e "$name" ]; then
    rm -rf -- "$name"
  fi
  cp -r -- "$entry" "$name"
  echo "  - $name"
done
