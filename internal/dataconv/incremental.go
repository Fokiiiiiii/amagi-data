package dataconv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Fokiiiiiii/amagi-data/internal/azurlanelua"
)

// IncrementalPlan is the small, source-controlled contract between the CI
// planner and the converter. SourcePaths are exact Lua files; GameCfg contains
// aggregate outputs that must be rebuilt from their whole category directory.
type IncrementalPlan struct {
	Mode                string   `json:"mode"`
	PreviousSHA         string   `json:"previous_sha"`
	LatestSHA           string   `json:"latest_sha"`
	SourcePaths         []string `json:"source_paths"`
	RequiredSourcePaths []string `json:"required_source_paths"`
	OutputPaths         []string `json:"output_paths"`
	GameCfg             []string `json:"gamecfg"`
	Versions            bool     `json:"versions"`
	DeleteOutputs       []string `json:"delete_outputs"`
}

func ReadIncrementalPlan(path string) (IncrementalPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IncrementalPlan{}, fmt.Errorf("read incremental plan %s: %w", path, err)
	}
	var plan IncrementalPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return IncrementalPlan{}, fmt.Errorf("decode incremental plan %s: %w", path, err)
	}
	if plan.Mode != "incremental" {
		return IncrementalPlan{}, fmt.Errorf("incremental plan mode must be incremental, got %q", plan.Mode)
	}
	for _, paths := range [][]string{plan.SourcePaths, plan.RequiredSourcePaths, plan.OutputPaths, plan.GameCfg, plan.DeleteOutputs} {
		for _, rel := range paths {
			if err := validatePlanPath(rel); err != nil {
				return IncrementalPlan{}, err
			}
		}
	}
	return plan, nil
}

func validatePlanPath(rel string) error {
	clean := filepath.ToSlash(filepath.Clean(rel))
	if rel == "" || clean != rel || clean == "." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return fmt.Errorf("invalid relative path in incremental plan: %q", rel)
	}
	return nil
}

func ConvertMVPIncremental(opts Options, plan IncrementalPlan) (*Report, error) {
	if opts.LuaScriptsRoot == "" {
		return nil, fmt.Errorf("Lua scripts root is required for incremental conversion")
	}
	if opts.OutputRoot == "" {
		return nil, fmt.Errorf("output root is required")
	}
	manifest, err := loadSafeManifest()
	if err != nil {
		return nil, err
	}
	report := newReport(opts, manifest)
	resetLuaReport(report)
	report.CategoryCounts = map[string]int{}
	report.CategoryIDs = map[string][]int64{}

	for _, rel := range plan.RequiredSourcePaths {
		if _, err := os.Stat(filepath.Join(opts.LuaScriptsRoot, filepath.FromSlash(rel))); err != nil {
			return nil, fmt.Errorf("required incremental source is missing: %s: %w", rel, err)
		}
	}

	for _, rel := range plan.DeleteOutputs {
		path := filepath.Join(opts.OutputRoot, filepath.FromSlash(rel))
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove deleted output %s: %w", rel, err)
		}
	}

	for _, rel := range plan.SourcePaths {
		region, dir, name, ok := incrementalLuaPath(rel)
		if !ok {
			return nil, fmt.Errorf("unsupported incremental source path: %s", rel)
		}
		if err := generateDiscoveredLuaFile(opts, report, region, dir, name); err != nil {
			return nil, err
		}
	}
	if err := generateIncrementalSpecialFiles(opts, report, plan.SourcePaths); err != nil {
		return nil, err
	}

	for _, rel := range slices.Clone(plan.GameCfg) {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 || parts[1] != "GameCfg" || !strings.HasSuffix(parts[2], ".json") {
			return nil, fmt.Errorf("invalid incremental GameCfg output: %s", rel)
		}
		category := strings.TrimSuffix(parts[2], ".json")
		sourceName := category
		if parts[0] == "JP" && category == "story" {
			sourceName = "storyjp"
		}
		if err := generateReturnedGameCfg(opts, report, parts[0], sourceName, category); err != nil {
			return nil, err
		}
		if !slices.Contains(report.GeneratedFiles, rel) && !slices.Contains(plan.DeleteOutputs, rel) {
			return nil, fmt.Errorf("incremental GameCfg output was not generated: %s", rel)
		}
	}

	if plan.Versions {
		versions, source, err := generateVersionsJSON(opts.LuaScriptsRoot, "")
		if err != nil {
			return nil, err
		}
		if err := writeVersionsJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(globalVersionsPath())), versions); err != nil {
			return nil, err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, globalVersionsPath())
		report.GeneratedVersions = true
		report.LuaScriptsVersionsRoot = source
		report.LuaScriptsVersionSource = versions
	}

	sortStrings(report.GeneratedFiles)
	sortStrings(report.GeneratedHelperFiles)
	for key := range report.CategoryIDs {
		slices.Sort(report.CategoryIDs[key])
		report.CategoryIDs[key] = slices.Compact(report.CategoryIDs[key])
	}
	if err := writeReport(opts, report); err != nil {
		return nil, err
	}
	return report, nil
}

func incrementalLuaPath(rel string) (string, string, string, bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 3 || !slices.Contains(supportedRegions, parts[0]) || !slices.Contains([]string{"sharecfg", "sharecfgdata"}, parts[1]) || !strings.HasSuffix(parts[2], ".lua") {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func generateIncrementalSpecialFiles(opts Options, report *Report, sourcePaths []string) error {
	for _, source := range sourcePaths {
		target, ok := incrementalSpecialTargets[source]
		if !ok {
			continue
		}
		if err := generateAdditionalLuaFile(opts, report, source, target); err != nil {
			return err
		}
	}
	return nil
}

var incrementalSpecialTargets = map[string]string{
	"JP/sharecfg/battlenodescfg.lua":                    "JP/ShareCfg/battle_nodes_cfg.json",
	"JP/sharecfg/dorm3d_dolly.lua":                      "JP/ShareCfg/dorm3_d_dolly.json",
	"JP/sharecfg/informcfg.lua":                         "JP/ShareCfg/inform_cfg.json",
	"JP/sharecfg/informforbackyardthemetemplatecfg.lua": "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json",
	"JP/sharecfg/world_slgbuff_data.lua":                "JP/ShareCfg/world_sl_gbuff_data.json",
}

func generateAdditionalLuaFile(opts Options, report *Report, source, target string) error {
	if slices.Contains(report.GeneratedFiles, target) {
		return nil
	}
	luaPath := filepath.Join(opts.LuaScriptsRoot, filepath.FromSlash(source))
	if _, err := os.Stat(luaPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	decoded, err := loadLuaFile(opts, luaPath)
	if err != nil {
		report.UnsupportedFiles = append(report.UnsupportedFiles, target)
		return nil
	}
	converted := azurlanelua.ToPlain(decoded)
	if target != "JP/ShareCfg/inform_cfg.json" && target != "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json" {
		converted, err = dictKeyedToSortedList(normalizeEmpty(converted))
		if err != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, target)
			return nil
		}
	}
	if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(target)), converted); err != nil {
		return err
	}
	report.GeneratedFiles = append(report.GeneratedFiles, target)
	report.TotalGeneratedCount++
	return nil
}
