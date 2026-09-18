#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"

if [[ -z "$(git status --short)" ]]; then
	echo "No generated file changes to commit"
	exit 0
fi

prev_versions="$(git show HEAD:global/versions.json 2>/dev/null || echo '{}')"
curr_versions="$(cat global/versions.json 2>/dev/null || echo '{}')"

version_of() {
	printf '%s' "$1" | jq -r --arg region "$2" '.[$region] // ""'
}

# Mirror the upstream repository's history: one "update [XX]: old -> new" commit per
# region whose files changed, even when the version itself did not move. Shared
# files (global/versions.json and friends) ride along with the last region commit,
# so the versions entry in a file listing points at a region update just as the
# upstream versions/ directory does.
regions=(CN EN JP KR TW)
targets=()
for region in "${regions[@]}"; do
	if [[ -n "$(git status --short -- "$region/")" ]]; then
		targets+=("$region")
	fi
done

if (( ${#targets[@]} == 0 )); then
	echo "No region data changed; leaving shared files uncommitted"
	git status --short
	exit 0
fi

git config user.email "github-actions[bot]@users.noreply.github.com"
git config user.name "github-actions[bot]"

last="${targets[${#targets[@]}-1]}"
for region in "${targets[@]}"; do
	old="$(version_of "$prev_versions" "$region")"
	new="$(version_of "$curr_versions" "$region")"
	git add -- "$region/"
	if [[ "$region" == "$last" ]]; then
		git add --all
	fi
	git commit -m "update [$region]: ${old:-?} -> ${new:-?} [skip ci]"
done

git pull --rebase origin "$GITHUB_REF_NAME"
git push origin "HEAD:$GITHUB_REF_NAME"
