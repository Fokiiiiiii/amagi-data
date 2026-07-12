#!/usr/bin/env bash
set -euo pipefail

export AMAGI_DATA_TEST_LUASCRIPTS_ROOT="$GITHUB_WORKSPACE/_external/AzurLaneLuaScripts"

if [ -d "$AMAGI_DATA_TEST_LUASCRIPTS_ROOT" ]; then
  echo "AzurLaneLuaScripts root exists: True"
else
  echo "AzurLaneLuaScripts root exists: False"
fi

go test ./...
