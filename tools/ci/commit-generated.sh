#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"

if [[ -z "$(git status --short)" ]]; then
	echo "No generated file changes to commit"
	exit 0
fi

prev_versions="$(git show HEAD:global/versions.json 2>/dev/null || echo '{}')"
curr_versions="$(cat global/versions.json 2>/dev/null || echo '{}')"

message="$(PREV_VERSIONS="$prev_versions" CURR_VERSIONS="$curr_versions" python3 - <<'PY'
import json
import os

prev = json.loads(os.environ["PREV_VERSIONS"])
curr = json.loads(os.environ["CURR_VERSIONS"])

changes = [(region, prev.get(region), curr[region])
           for region in sorted(curr) if prev.get(region) != curr[region]]

if changes:
	# Every bumped region goes on the subject line: a commit list only shows that
	# line, so the versions have to be visible there rather than in the body.
	summary = ", ".join(
		f"[{region}]: {old} -> {new}" if old else f"[{region}]: {new}"
		for region, old, new in changes
	)
	print(f"update {summary} [skip ci]")
PY
)"

if [[ -z "$message" ]]; then
	echo "No region version change; skipping commit"
	exit 0
fi

git config user.email "github-actions[bot]@users.noreply.github.com"
git config user.name "github-actions[bot]"
git add --all
git commit -m "$message"
git pull --rebase origin "$GITHUB_REF_NAME"
git push origin "HEAD:$GITHUB_REF_NAME"
