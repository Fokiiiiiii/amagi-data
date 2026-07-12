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

python3 - "$out" <<'PY'
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
    if path.is_file() and path.name != "belfast-json-mvp-report.json":
        if path.relative_to(out).as_posix() not in expected:
            path.unlink()
PY

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
