#!/usr/bin/env bash
set -euo pipefail

export AMAGI_DATA_TEST_AZURLANE_ROOT="$GITHUB_WORKSPACE/_external/AzurLaneData"
export AMAGI_DATA_TEST_LUASCRIPTS_ROOT="$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts"

if [ -d "$AMAGI_DATA_TEST_AZURLANE_ROOT" ]; then
  echo "AzurLaneData root exists: True"
else
  echo "AzurLaneData root exists: False"
fi

if [ -d "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" ]; then
  echo "AzurLaneLuaScripts root exists: True"
else
  echo "AzurLaneLuaScripts root exists: False"
fi

go test ./...
