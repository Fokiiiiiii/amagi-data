#!/usr/bin/env bash
set -euo pipefail

# The build job checks out only code, CI and global/ (see the workflow): the
# generated region data is hundreds of MB, and an incremental update touches a
# handful of files. Widen the checkout to exactly what generation reads or rewrites.
mode="${AMAGI_MODE:?AMAGI_MODE is required}"

if [[ "$(git config --type=bool core.sparseCheckout || true)" != "true" ]]; then
	echo "working tree is not sparse; nothing to widen"
	exit 0
fi

case "$mode" in
	full)
		# A full build rewrites every region and prunes stale outputs from disk.
		git sparse-checkout disable
		;;
	incremental)
		: "${AMAGI_INCREMENTAL_PLAN:?AMAGI_INCREMENTAL_PLAN is required for incremental checkout}"
		# Planned outputs include every deleted output, so deletions are staged too.
		mapfile -t patterns < <(jq -r '(.output_paths + .delete_outputs)[] | "/" + .' "$AMAGI_INCREMENTAL_PLAN" | sort -u)
		if (( ${#patterns[@]} > 0 )); then
			git sparse-checkout add "${patterns[@]}"
		fi
		echo "checked out ${#patterns[@]} planned output paths"
		;;
	*)
		echo "mode $mode generates nothing; checkout left as is"
		;;
esac
