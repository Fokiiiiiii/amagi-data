#!/usr/bin/env bash
set -euo pipefail

: "${UPSTREAM_SHA:?UPSTREAM_SHA is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"

remote="${UPSTREAM_REMOTE:-https://github.com/AzurLaneTools/AzurLaneLuaScripts.git}"
state_file="${STATE_FILE:-.github/azurlane-state}"
plan_path="${PLAN_PATH:-$RUNNER_TEMP/amagi-incremental-plan.json}"
upstream_root="${UPSTREAM_ROOT:-$RUNNER_TEMP/AzurLaneLuaScripts}"
force_full="${FORCE_FULL:-false}"
latest_sha="$UPSTREAM_SHA"

if [[ ! "$latest_sha" =~ ^[0-9a-f]{40}$ ]]; then
	echo "invalid upstream SHA: $latest_sha" >&2
	exit 1
fi

if [[ "$upstream_root" != "$RUNNER_TEMP"/* ]]; then
	echo "upstream root must be below RUNNER_TEMP: $upstream_root" >&2
	exit 1
fi

generator_hash="$({ git ls-tree -r --full-tree HEAD -- cmd/generate_data internal/dataconv internal/azurlanelua tools/ci data/global .github/workflows/validate-and-update.yml go.mod go.sum; } | sha256sum | cut -d' ' -f1)"
previous_sha=""
previous_generator_hash=""
if [[ -f "$state_file" ]]; then
	previous_sha="$(awk -F': ' '$1 == "upstream_sha" { print $2; exit }' "$state_file" | tr -d '\r')"
	previous_generator_hash="$(awk -F': ' '$1 == "generator_hash" { print $2; exit }' "$state_file" | tr -d '\r')"
fi

if [[ "$force_full" == "true" || ! "$previous_sha" =~ ^[0-9a-f]{40}$ || "$previous_generator_hash" != "$generator_hash" ]]; then
	mode="full"
elif [[ "$previous_sha" == "$latest_sha" ]]; then
	mode="unchanged"
	printf 'mode=%s\nneeds_processing=false\nneeds_sources=false\ngenerator_hash=%s\nlatest_sha=%s\nprevious_sha=%s\nplan_path=%s\nsource_root=\n' \
		"$mode" "$generator_hash" "$latest_sha" "$previous_sha" "$plan_path" >> "$GITHUB_OUTPUT"
	echo "upstream unchanged: $latest_sha"
	exit 0
else
	mode="incremental"
fi

rm -rf -- "$upstream_root"
mkdir -p "$upstream_root"
git -C "$upstream_root" init -q
git -C "$upstream_root" remote add origin "$remote"
git -C "$upstream_root" config remote.origin.promisor true
git -C "$upstream_root" config remote.origin.partialclonefilter blob:none
git -C "$upstream_root" fetch --quiet --depth=1 --filter=blob:none --no-tags origin "$latest_sha"

diff_path="$RUNNER_TEMP/amagi-upstream.diff"
source_paths=()
required_source_paths=()
sparse_patterns=()
gamecfg=()
delete_outputs=()
versions=false

append_unique() {
	local array_name=$1
	local value=$2
	local existing
	case "$array_name" in
		source_paths)
			for existing in "${source_paths[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			source_paths+=("$value")
			;;
		required_source_paths)
			for existing in "${required_source_paths[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			required_source_paths+=("$value")
			;;
		sparse_patterns)
			for existing in "${sparse_patterns[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			sparse_patterns+=("$value")
			;;
		output_paths)
			for existing in "${output_paths[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			output_paths+=("$value")
			;;
		gamecfg)
			for existing in "${gamecfg[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			gamecfg+=("$value")
			;;
		delete_outputs)
			for existing in "${delete_outputs[@]-}"; do [[ "$existing" == "$value" ]] && return; done
			delete_outputs+=("$value")
			;;
		esac
}

add_source_pair() {
	# ponytail: rebuild both sides conservatively; inspect stream metadata only if checkout volume becomes a bottleneck
	local status=$1
	local path=$2
	local region=$3
	local dir=$4
	local name=$5
	local other_dir
	local stem
	if [[ "$dir" == "sharecfg" ]]; then
		other_dir="sharecfgdata"
	else
		other_dir="sharecfg"
	fi
	stem="${name%.lua}"
	append_unique source_paths "$path"
	append_unique source_paths "$region/$other_dir/$name"
	if [[ "$status" != "D" ]]; then
		append_unique required_source_paths "$path"
	fi
	append_unique sparse_patterns "$path"
	append_unique sparse_patterns "$region/$other_dir/$name"
	append_unique output_paths "$region/ShareCfg/$stem.json"
	append_unique output_paths "$region/sharecfgdata/$stem.json"
	if [[ "$status" == "D" ]]; then
		if [[ "$dir" == "sharecfg" ]]; then
			append_unique delete_outputs "$region/ShareCfg/$stem.json"
		else
			append_unique delete_outputs "$region/sharecfgdata/$stem.json"
		fi
	fi
case "$path" in
	JP/sharecfg/battlenodescfg.lua)
		append_unique output_paths "JP/ShareCfg/battle_nodes_cfg.json"
		[[ "$status" == "D" ]] && append_unique delete_outputs "JP/ShareCfg/battle_nodes_cfg.json"
		;;
	JP/sharecfg/dorm3d_dolly.lua)
		append_unique output_paths "JP/ShareCfg/dorm3_d_dolly.json"
		[[ "$status" == "D" ]] && append_unique delete_outputs "JP/ShareCfg/dorm3_d_dolly.json"
		;;
	JP/sharecfg/informcfg.lua)
		append_unique output_paths "JP/ShareCfg/inform_cfg.json"
		[[ "$status" == "D" ]] && append_unique delete_outputs "JP/ShareCfg/inform_cfg.json"
		;;
	JP/sharecfg/informforbackyardthemetemplatecfg.lua)
		append_unique output_paths "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json"
		[[ "$status" == "D" ]] && append_unique delete_outputs "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json"
		;;
	JP/sharecfg/world_slgbuff_data.lua)
		append_unique output_paths "JP/ShareCfg/world_sl_gbuff_data.json"
		[[ "$status" == "D" ]] && append_unique delete_outputs "JP/ShareCfg/world_sl_gbuff_data.json"
		;;
esac
	# A non-deletion is valid; do not leak the last conditional's false status.
	return 0
}

add_gamecfg() {
	local region=$1
	local source_name=$2
	local target_name=$3
	local target="$region/GameCfg/$target_name.json"
	local files
	append_unique gamecfg "$target"
	append_unique output_paths "$target"
	append_unique sparse_patterns "/$region/gamecfg/$source_name/**/*.lua"
	files="$(git -C "$upstream_root" ls-tree -r --name-only "$latest_sha" -- "$region/gamecfg/$source_name")"
	if [[ -z "$files" ]] || ! printf '%s\n' "$files" | grep '\.lua$' >/dev/null; then
		append_unique delete_outputs "$target"
	fi
}

output_paths=()

if [[ "$mode" == "incremental" ]]; then
	if ! git -C "$upstream_root" fetch --quiet --depth=1 --filter=blob:none --no-tags origin "$previous_sha"; then
		echo "previous upstream SHA is unavailable; falling back to full build: $previous_sha"
		mode="full"
	elif ! git -C "$upstream_root" cat-file -e "$previous_sha^{commit}"; then
		echo "previous upstream SHA is not a commit; falling back to full build: $previous_sha"
		mode="full"
	else
		git -C "$upstream_root" diff --no-renames --name-status "$previous_sha" "$latest_sha" > "$diff_path"
		while IFS=$'\t' read -r status path; do
			[[ -z "$path" ]] && continue
			if [[ "$path" =~ ^(CN|EN|JP|KR|TW)/(sharecfg|sharecfgdata)/([^/]+\.lua)$ ]]; then
				add_source_pair "$status" "$path" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
			elif [[ "$path" =~ ^(CN|EN|JP|KR|TW)/gamecfg/(buff|card|dorm|dungeon|skill|story|storyjp)/.+$ ]]; then
				region="${BASH_REMATCH[1]}"
				source_name="${BASH_REMATCH[2]}"
				if [[ "$region" == "JP" && "$source_name" == "story" ]] || [[ "$region" != "JP" && "$source_name" == "storyjp" ]]; then
					echo "ambiguous JP gamecfg/story dependency; falling back to full build"
					mode="full"
					break
				fi
				target_name="$source_name"
				[[ "$region" == "JP" && "$source_name" == "storyjp" ]] && target_name="story"
				add_gamecfg "$region" "$source_name" "$target_name"
			elif [[ "$path" =~ ^versions/[^/]+\.txt$ ]]; then
				versions=true
				append_unique output_paths "global/versions.json"
			else
				continue
			fi
		done < "$diff_path"
		if [[ "$versions" == "true" ]]; then
			append_unique sparse_patterns "/versions/*.txt"
		fi
	fi
fi

if [[ "$mode" == "full" ]]; then
	source_paths=()
	required_source_paths=()
	gamecfg=()
	delete_outputs=()
	versions=false
	output_paths=()
	sparse_patterns=()
	for region in CN EN JP KR TW; do
		sparse_patterns+=("/$region/sharecfg/**/*.lua" "/$region/sharecfgdata/**/*.lua")
		for category in buff card dorm dungeon skill; do
			sparse_patterns+=("/$region/gamecfg/$category/**/*.lua")
		done
		if [[ "$region" == "JP" ]]; then
			sparse_patterns+=("/$region/gamecfg/storyjp/**/*.lua")
		else
			sparse_patterns+=("/$region/gamecfg/story/**/*.lua")
		fi
	done
	sparse_patterns+=("/versions/*.txt")
elif [[ "$mode" == "incremental" ]]; then
	if (( ${#source_paths[@]} == 0 && ${#gamecfg[@]} == 0 )) && [[ "$versions" != "true" ]]; then
		mode="noop"
	fi
fi

json_array() {
	if (( $# == 0 )); then
		printf '[]'
		return
	fi
	printf '%s\n' "$@" | jq -Rsc 'split("\n") | map(select(length > 0))'
}

source_json="$(json_array "${source_paths[@]-}")"
required_source_json="$(json_array "${required_source_paths[@]-}")"
gamecfg_json="$(json_array "${gamecfg[@]-}")"
delete_json="$(json_array "${delete_outputs[@]-}")"
jq -n \
	--arg mode "$mode" \
	--arg previous_sha "$previous_sha" \
	--arg latest_sha "$latest_sha" \
	--argjson source_paths "$source_json" \
	--argjson required_source_paths "$required_source_json" \
	--argjson output_paths "$(json_array "${output_paths[@]-}")" \
	--argjson gamecfg "$gamecfg_json" \
	--argjson versions "$versions" \
	--argjson delete_outputs "$delete_json" \
	'{mode: $mode, previous_sha: $previous_sha, latest_sha: $latest_sha, source_paths: $source_paths, required_source_paths: $required_source_paths, output_paths: $output_paths, gamecfg: $gamecfg, versions: $versions, delete_outputs: $delete_outputs}' > "$plan_path"

needs_sources=false
if [[ "$mode" == "full" ]] || (( ${#source_paths[@]} > 0 || ${#gamecfg[@]} > 0 )) || [[ "$versions" == "true" ]]; then
	needs_sources=true
fi

if [[ "$needs_sources" == "true" ]]; then
	git -C "$upstream_root" sparse-checkout init --no-cone
	git -C "$upstream_root" sparse-checkout set --no-cone "${sparse_patterns[@]}"
	git -C "$upstream_root" checkout --quiet --detach "$latest_sha"
else
	upstream_root=""
fi

printf 'mode=%s\nneeds_processing=true\nneeds_sources=%s\ngenerator_hash=%s\nlatest_sha=%s\nprevious_sha=%s\nplan_path=%s\nsource_root=%s\n' \
	"$mode" "$needs_sources" "$generator_hash" "$latest_sha" "$previous_sha" "$plan_path" "$upstream_root" >> "$GITHUB_OUTPUT"
echo "upstream transition: ${previous_sha:-none} -> $latest_sha ($mode)"
if [[ -f "$diff_path" ]]; then
	echo "upstream diff: $(wc -l < "$diff_path" | tr -d ' ') paths"
fi
