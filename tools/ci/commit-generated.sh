#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"

if [[ -z "$(git status --short)" ]]; then
	echo "No generated file changes to commit"
	exit 0
fi

git config user.email "github-actions[bot]@users.noreply.github.com"
git config user.name "github-actions[bot]"

prev_versions="$(git show HEAD:global/versions.json 2>/dev/null || echo '{}')"
curr_versions="$(cat global/versions.json 2>/dev/null || echo '{}')"

message="$(PREV_VERSIONS="$prev_versions" CURR_VERSIONS="$curr_versions" python3 - <<'PY'
import json
import os

prev = json.loads(os.environ["PREV_VERSIONS"])
curr = json.loads(os.environ["CURR_VERSIONS"])

changes = [(region, prev.get(region), curr[region])
           for region in sorted(curr) if prev.get(region) != curr[region]]

if not changes:
	print("data: sync generated data [skip ci]")
elif len(changes) == 1:
	region, old, new = changes[0]
	summary = f"{old} -> {new}" if old else new
	print(f"update [{region}]: {summary} [skip ci]")
else:
	regions = ", ".join(region for region, _, _ in changes)
	lines = [f"update [{regions}] [skip ci]", ""]
	for region, old, new in changes:
		lines.append(f"{region}: {old} -> {new}" if old else f"{region}: {new}")
	print("\n".join(lines))
PY
)"

git add --all
git commit -m "$message"
git pull --rebase origin "$GITHUB_REF_NAME"
git push origin "HEAD:$GITHUB_REF_NAME"
