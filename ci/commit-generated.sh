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

git config user.email "github-actions[bot]@users.noreply.github.com"
git config user.name "github-actions[bot]"

if (( ${#targets[@]} == 0 )); then
	# Shared files alone (a version bump without data changes) must still be
	# published: the workflow records the upstream revision as processed once this
	# script succeeds, so leaving them uncommitted would drop the update for good.
	bumped=()
	for region in "${regions[@]}"; do
		if [[ "$(version_of "$prev_versions" "$region")" != "$(version_of "$curr_versions" "$region")" ]]; then
			bumped+=("$region")
		fi
	done
	if (( ${#bumped[@]} == 1 )); then
		region="${bumped[0]}"
		message="update [$region]: $(version_of "$prev_versions" "$region") -> $(version_of "$curr_versions" "$region")"
	else
		message="update shared files"
	fi
	git add --all
	git commit -m "$message"
else
	last="${targets[${#targets[@]}-1]}"
	for region in "${targets[@]}"; do
		old="$(version_of "$prev_versions" "$region")"
		new="$(version_of "$curr_versions" "$region")"
		git add -- "$region/"
		if [[ "$region" == "$last" ]]; then
			git add --all
		fi
		git commit -m "update [$region]: ${old:-?} -> ${new:-?}"
	done
fi

# Anything still uncommitted would be lost while the upstream revision is marked
# processed; fail so the state is not saved and the next run retries.
if [[ -n "$(git status --porcelain)" ]]; then
	echo "generated changes remain uncommitted:" >&2
	git status --short >&2
	exit 1
fi

git pull --rebase origin "$GITHUB_REF_NAME"
git push origin "HEAD:$GITHUB_REF_NAME"
