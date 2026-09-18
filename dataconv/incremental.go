package dataconv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Fokiiiiiii/amagi-data/azurlanelua"
)

// IncrementalPlan is the small, source-controlled contract between the CI
// planner and the converter. SourcePaths are exact Lua files; GameCfg contains
// aggregate outputs that must be rebuilt from their whole category directory.
type IncrementalPlan struct {
	Mode                string          `json:"mode"`
	PreviousSHA         string          `json:"previous_sha"`
	LatestSHA           string          `json:"latest_sha"`
	SourcePaths         []string        `json:"source_paths"`
	RequiredSourcePaths []string        `json:"required_source_paths"`
	OutputPaths         []string        `json:"output_paths"`
	GameCfg             []string        `json:"gamecfg"`
	GameCfgSources      []GameCfgSource `json:"gamecfg_sources"`
	Versions            bool            `json:"versions"`
	DeleteOutputs       []string        `json:"delete_outputs"`
}

// GameCfgSource is a single Lua file inside a GameCfg category that changed
// upstream. It lets the converter refresh just that entry of the bundle instead
// of re-parsing the whole category directory.
type GameCfgSource struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
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
	for _, source := range plan.GameCfgSources {
		if err := validatePlanPath(source.Path); err != nil {
			return IncrementalPlan{}, err
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
	report := newReport(opts)
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

	type incrementalSource struct{ region, dir, name string }
	sources := make([]incrementalSource, 0, len(plan.SourcePaths))
	for _, rel := range plan.SourcePaths {
		region, dir, name, ok := incrementalLuaPath(rel)
		if !ok {
			return nil, fmt.Errorf("unsupported incremental source path: %s", rel)
		}
		sources = append(sources, incrementalSource{region, dir, name})
	}
	// Convert sharecfgdata before sharecfg so a stream facade in the same plan reuses
	// the backing file's output instead of parsing that Lua source a second time.
	slices.SortStableFunc(sources, func(a, b incrementalSource) int {
		return streamBackingRank(a.dir) - streamBackingRank(b.dir)
	})
	backed := map[string]streamBackedOutput{}
	for _, source := range sources {
		if err := generateDiscoveredLuaFile(opts, report, source.region, source.dir, source.name, backed); err != nil {
			return nil, err
		}
	}
	if err := generateIncrementalSpecialFiles(opts, report, plan.SourcePaths); err != nil {
		return nil, err
	}

	changedBundleSources := groupGameCfgSources(plan.GameCfgSources)
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
		refreshed := false
		if sources := changedBundleSources[rel]; len(sources) > 0 {
			var err error
			refreshed, err = refreshGameCfgBundle(opts, report, parts[0], sourceName, category, sources)
			if err != nil {
				return nil, err
			}
		}
		if !refreshed {
			if err := generateReturnedGameCfg(opts, report, parts[0], sourceName, category); err != nil {
				return nil, err
			}
		}
		if !slices.Contains(report.GeneratedFiles, rel) && !slices.Contains(plan.DeleteOutputs, rel) {
			return nil, fmt.Errorf("incremental GameCfg output was not generated: %s", rel)
		}
	}

	if plan.Versions {
		versions, source, err := generateVersionsJSON(opts.LuaScriptsRoot)
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

// groupGameCfgSources maps each changed Lua file to the bundle it belongs to.
// A path that is not a direct child of a category directory yields no group, so
// that bundle falls back to a full rebuild.
func groupGameCfgSources(sources []GameCfgSource) map[string][]GameCfgSource {
	grouped := map[string][]GameCfgSource{}
	for _, source := range sources {
		parts := strings.Split(filepath.ToSlash(source.Path), "/")
		if len(parts) != 4 || parts[1] != "gamecfg" || !strings.HasSuffix(parts[3], ".lua") {
			continue
		}
		region, sourceName := parts[0], parts[2]
		targetName := sourceName
		if region == "JP" && sourceName == "storyjp" {
			targetName = "story"
		}
		rel := region + "/GameCfg/" + targetName + ".json"
		grouped[rel] = append(grouped[rel], source)
	}
	return grouped
}

// refreshGameCfgBundle rewrites a GameCfg bundle from the previously generated
// JSON plus only the Lua files that changed. A category directory holds up to
// ~13k files, so re-parsing all of them to pick up one edit dominated the
// incremental build. Reporting false means the previous output could not be
// reused and the caller must rebuild the bundle from scratch.
func refreshGameCfgBundle(opts Options, report *Report, region, sourceName, targetName string, sources []GameCfgSource) (bool, error) {
	if opts.SourceRoot == "" {
		return false, nil
	}
	previous, err := os.ReadFile(filepath.Join(opts.SourceRoot, region, "GameCfg", targetName+".json"))
	if err != nil {
		return false, nil
	}
	decoded, err := decodeOrderedJSON(previous)
	if err != nil {
		return false, nil
	}
	bundle, ok := decoded.(azurlanelua.OrderedObject)
	if !ok {
		return false, nil
	}
	prefix := region + "/gamecfg/" + sourceName + "/"
	for _, source := range sources {
		stem, ok := strings.CutPrefix(filepath.ToSlash(source.Path), prefix)
		if !ok || !strings.HasSuffix(stem, ".lua") {
			return false, nil
		}
		stem = strings.TrimSuffix(stem, ".lua")
		if source.Deleted {
			delete(bundle.Values, stem)
			continue
		}
		value, loadErr := loadLuaFile(opts, filepath.Join(opts.LuaScriptsRoot, filepath.FromSlash(source.Path)))
		if loadErr != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, region+"/GameCfg/"+targetName+".json")
			return true, nil
		}
		if list, ok := azurlanelua.ToPlain(value).([]any); ok && len(list) == 0 {
			value = nil
		}
		bundle.Values[stem] = azurlanelua.ToPlain(value)
	}
	// A full rebuild emits keys in sorted path order, which for files sharing one
	// directory is the same as sorted stem order.
	keys := make([]string, 0, len(bundle.Values))
	for key := range bundle.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	bundle.Keys = keys

	rel := region + "/GameCfg/" + targetName + ".json"
	if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), bundle); err != nil {
		return true, err
	}
	report.GeneratedFiles = append(report.GeneratedFiles, rel)
	report.TotalGeneratedCount++
	return true, nil
}

func streamBackingRank(dir string) int {
	if dir == "sharecfgdata" {
		return 0
	}
	return 1
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
