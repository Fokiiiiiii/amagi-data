#!/usr/bin/env bash
set -euo pipefail

source_root="${AMAGI_UPSTREAM_ROOT:-$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts}"
export AMAGI_DATA_TEST_LUASCRIPTS_ROOT="$source_root"

if [ -d "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" ]; then
  echo "AzurLaneLuaScripts root exists: True"
else
  echo "AzurLaneLuaScripts root exists: False"
fi

if [[ "${AMAGI_MODE:-full}" == "incremental" ]]; then
  echo "Running focused incremental/full equivalence test"
  go test ./internal/dataconv -run '^TestConvertMVPIncrementalMatchesFullBuildAfterUpstreamChanges$' -count=1
  exit
fi

go test ./...
