package belfastconv

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Fokiiiiiii/amagi-data/internal/belfastlua"
)

//go:embed safe_to_promote_manifest.json safe_to_promote_allowlists.json
var safeManifestFS embed.FS

const globalDir = "global"

var fallbackHelperFiles = []string{
	"global/build_pools.json",
	"global/build_times.json",
	"global/requisition_ships.json",
}

var supportedRegions = []string{"CN", "EN", "JP", "KR", "TW"}

type Options struct {
	SourceRoot               string
	OutputRoot               string
	ReportPath               string
	LuaScriptsRoot           string
	ReferenceRoot            string
	FallbackHelperSourceRoot string
	VersionSourceMapPath     string
	LegacyFallbackSourceRoot string
}

type FileReport struct {
	RelativePath string `json:"relative_path"`
	Records      int    `json:"records"`
}

type FallbackFileReport struct {
	RelativePath    string `json:"relative_path"`
	SourceKind      string `json:"source_kind"`
	SourcePath      string `json:"source_path"`
	ReferenceSHA256 string `json:"reference_sha256"`
	GeneratedSHA256 string `json:"generated_sha256"`
	Match           bool   `json:"match"`
}

type SafePromoteFile struct {
	RelativePath   string `json:"relative_path"`
	Region         string `json:"region"`
	Category       string `json:"category"`
	Classification string `json:"classification"`
}

type SafeManifest struct {
	SafeToPromoteFiles    []SafePromoteFile `json:"safe_to_promote_files"`
	CountMismatchFiles    []string          `json:"count_mismatch_files"`
	SchemaMismatchFiles   []string          `json:"schema_mismatch_files"`
	MissingReferenceFiles []string          `json:"missing_reference_files"`
	UnsupportedFiles      []string          `json:"unsupported_files"`
}

type Report struct {
	SourceRoot              string               `json:"source_root"`
	OutputRoot              string               `json:"output_root"`
	Regions                 []string             `json:"regions"`
	Categories              []string             `json:"categories"`
	ConvertedFiles          []FileReport         `json:"converted_files"`
	GeneratedFiles          []string             `json:"generated_files"`
	GeneratedHelperFiles    []string             `json:"generated_helper_files"`
	FallbackFiles           []string             `json:"fallback_files"`
	FallbackFileReports     []FallbackFileReport `json:"fallback_file_reports"`
	FallbackHelperFiles     []string             `json:"fallback_helper_files"`
	UnsupportedFiles        []string             `json:"unsupported_files"`
	UnsupportedHelperFiles  []string             `json:"unsupported_helper_files"`
	MissingSourceFiles      []string             `json:"missing_source_files"`
	MissingReferenceFiles   []string             `json:"missing_reference_files"`
	SkippedUnsafeFiles      []string             `json:"skipped_unsafe_files"`
	GeneratedVersions       bool                 `json:"generated_versions"`
	LuaScriptsVersionsRoot  string               `json:"lua_scripts_versions_root,omitempty"`
	LuaScriptsVersionSource map[string]string    `json:"lua_scripts_version_source,omitempty"`
	TotalGeneratedCount     int                  `json:"total_generated_count"`
	TotalFallbackCount      int                  `json:"total_fallback_count"`
	TotalUnsupportedCount   int                  `json:"total_unsupported_count"`
	CategoryCounts          map[string]int       `json:"category_counts,omitempty"`
	CategoryIDs             map[string][]int64   `json:"category_ids,omitempty"`
}

func MVPFiles() []string {
	manifest, err := loadSafeManifest()
	if err != nil {
		return []string{}
	}
	files := make([]string, 0, len(manifest.SafeToPromoteFiles))
	for _, file := range manifest.SafeToPromoteFiles {
		files = append(files, file.RelativePath)
	}
	slices.Sort(files)
	return files
}

func UnsupportedHelperFiles(includeVersions bool) []string {
	if includeVersions {
		return []string{}
	}
	return []string{"global/versions.json"}
}

func FallbackHelperFiles() []string { return slices.Clone(fallbackHelperFiles) }

func ConvertMVP(opts Options) (*Report, error) {
	if opts.SourceRoot == "" && opts.LuaScriptsRoot == "" {
		return nil, fmt.Errorf("source root or Lua scripts root is required")
	}
	if opts.OutputRoot == "" {
		return nil, fmt.Errorf("output root is required")
	}

	manifest, err := loadSafeManifest()
	if err != nil {
		return nil, err
	}
	allowlists, err := loadSafeAllowlists()
	if err != nil {
		return nil, err
	}

	report := &Report{
		SourceRoot:             opts.SourceRoot,
		OutputRoot:             opts.OutputRoot,
		Regions:                slices.Clone(supportedRegions),
		Categories:             []string{"GameCfg", "ShareCfg", "sharecfgdata", "root-helpers"},
		ConvertedFiles:         []FileReport{},
		GeneratedFiles:         []string{},
		GeneratedHelperFiles:   []string{},
		FallbackFiles:          []string{},
		FallbackFileReports:    []FallbackFileReport{},
		FallbackHelperFiles:    []string{},
		UnsupportedFiles:       slices.Clone(manifest.UnsupportedFiles),
		UnsupportedHelperFiles: UnsupportedHelperFiles(opts.LuaScriptsRoot != ""),
		MissingSourceFiles:     []string{},
		MissingReferenceFiles:  slices.Clone(manifest.MissingReferenceFiles),
		SkippedUnsafeFiles:     skippedUnsafeFiles(manifest),
	}
	if opts.LegacyFallbackSourceRoot != "" {
		if err := validateLegacyFallbackSources(opts.LegacyFallbackSourceRoot); err != nil {
			return nil, err
		}
	}

	if opts.LuaScriptsRoot != "" {
		report.UnsupportedFiles = []string{}
		report.UnsupportedHelperFiles = []string{}
		report.MissingReferenceFiles = []string{}
		report.SkippedUnsafeFiles = []string{}
		if err := generateDiscoveredLuaFiles(opts, report); err != nil {
			return nil, err
		}
	} else if err := generateAuditedFiles(opts, manifest.SafeToPromoteFiles, allowlists, report); err != nil {
		return nil, err
	}
	if err := generateRootHelpers(opts, report); err != nil {
		return nil, err
	}
	if opts.LuaScriptsRoot != "" {
		versions, source, err := generateVersionsJSON(opts.LuaScriptsRoot, opts.VersionSourceMapPath)
		if err != nil {
			return nil, err
		}
		outPath := filepath.Join(opts.OutputRoot, filepath.FromSlash(globalVersionsPath()))
		if err := writeVersionsJSON(outPath, versions); err != nil {
			return nil, err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, globalVersionsPath())
		report.GeneratedVersions = true
		report.LuaScriptsVersionsRoot = source
		report.LuaScriptsVersionSource = versions
		if err := generateAdditionalLuaFiles(opts, report); err != nil {
			return nil, err
		}
	}
	if err := writeReport(opts, report); err != nil {
		return nil, err
	}
	return report, nil
}

func generateDiscoveredLuaFiles(opts Options, report *Report) error {
	report.CategoryCounts = map[string]int{}
	report.CategoryIDs = map[string][]int64{}
	for _, region := range supportedRegions {
		for _, dir := range []string{"sharecfg", "sharecfgdata"} {
			root := filepath.Join(opts.LuaScriptsRoot, region, dir)
			if _, err := os.Stat(root); err != nil {
				continue
			}
			err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || strings.Contains(filepath.ToSlash(path), "/sublist/") || !strings.HasSuffix(entry.Name(), ".lua") {
					return nil
				}
				relDir := map[string]string{"sharecfg": "ShareCfg", "sharecfgdata": "sharecfgdata"}[dir]
				rel := region + "/" + relDir + "/" + strings.TrimSuffix(entry.Name(), ".lua") + ".json"
				value, err := belfastlua.LoadFile(path)
				if err != nil {
					report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
					report.TotalUnsupportedCount++
					return nil
				}
				converted := belfastlua.ToPlain(value)
				if strings.HasSuffix(rel, "/ship_skin_template.json") {
					converted, err = loadLuaSublist(opts.LuaScriptsRoot, region, "ship_skin_template_sublist")
					if err != nil {
						return err
					}
				}
				converted = normalizeNumericTables(converted)
				rawConverted := normalizeEmpty(converted)
				converted, err = dictKeyedToSortedList(rawConverted)
				if err != nil {
					converted = rawConverted
					err = nil
				}
				if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), converted); err != nil {
					return err
				}
				report.GeneratedFiles = append(report.GeneratedFiles, rel)
				report.TotalGeneratedCount++
				category := strings.ToLower(dir)
				report.CategoryCounts[category] += recordCount(converted)
				for _, rec := range comparableIDs(converted) {
					report.CategoryIDs[category] = append(report.CategoryIDs[category], rec)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	for _, rel := range LegacyFallbackFiles() {
		if _, err := os.Stat(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel))); err != nil {
			if _, err := copyLegacyFallback(opts, rel, report); err != nil {
				return err
			}
		}
	}
	sortStrings(report.GeneratedFiles)
	for key := range report.CategoryIDs {
		sort.Slice(report.CategoryIDs[key], func(i, j int) bool { return report.CategoryIDs[key][i] < report.CategoryIDs[key][j] })
		report.CategoryIDs[key] = slices.Compact(report.CategoryIDs[key])
	}
	return nil
}

func comparableIDs(value any) []int64 {
	rows, _ := extractComparableRecords(value)
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		if id, ok := intFromAny(row["id"]); ok {
			out = append(out, int64(id))
		}
	}
	return out
}

func generateAdditionalLuaFiles(opts Options, report *Report) error {
	if err := generateReturnedGameCfg(opts, report, "buff"); err != nil {
		return err
	}
	if err := generateReturnedGameCfg(opts, report, "skill"); err != nil {
		return err
	}
	for _, name := range []string{"card", "dorm", "dungeon", "storyjp"} {
		if err := generateReturnedGameCfg(opts, report, name); err != nil {
			return err
		}
	}
	if err := generateDorm3dIKTimelineControllerError(opts, report); err != nil {
		return err
	}
	for _, item := range []struct{ source, target string }{
		{"JP/sharecfg/battlenodescfg.lua", "JP/ShareCfg/battle_nodes_cfg.json"},
		{"JP/sharecfg/dorm3d_dolly.lua", "JP/ShareCfg/dorm3_d_dolly.json"},
		{"JP/sharecfg/informcfg.lua", "JP/ShareCfg/inform_cfg.json"},
		{"JP/sharecfg/informforbackyardthemetemplatecfg.lua", "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json"},
		{"JP/sharecfg/world_slgbuff_data.lua", "JP/ShareCfg/world_sl_gbuff_data.json"},
	} {
		rel := item.target
		if slices.Contains(report.GeneratedFiles, rel) {
			continue
		}
		luaPath := filepath.Join(opts.LuaScriptsRoot, filepath.FromSlash(item.source))
		if _, err := os.Stat(luaPath); err != nil {
			continue
		}
		decoded, err := belfastlua.LoadFile(luaPath)
		if err != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
			continue
		}
		converted := belfastlua.ToPlain(decoded)
		if rel != "JP/ShareCfg/voice_actor_cn.json" &&
			rel != "JP/ShareCfg/inform_cfg.json" &&
			rel != "JP/ShareCfg/inform_for_back_yard_theme_template_cfg.json" {
			converted, err = dictKeyedToSortedList(normalizeEmpty(converted))
			if err != nil {
				report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
				continue
			}
		}
		if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), converted); err != nil {
			return err
		}
		report.GeneratedFiles = append(report.GeneratedFiles, rel)
		report.TotalGeneratedCount++
	}
	for _, rel := range []string{"JP/ShareCfg/enemy_data_statistics.json", "JP/ShareCfg/voice_actor_cn.json", "JP/ShareCfg/word_legal_template.json", "JP/ShareCfg/word_template.json", "JP/sharecfgdata/aircraft_template.json", "JP/sharecfgdata/enemy_data_statistics.json", "JP/sharecfgdata/equip_data_statistics.json", "JP/sharecfgdata/equip_data_template.json", "JP/sharecfgdata/weapon_property.json"} {
		if slices.Contains(report.GeneratedFiles, rel) {
			continue
		}
		luaPath := luaPathFor(opts.LuaScriptsRoot, rel)
		if _, err := os.Stat(luaPath); err != nil {
			continue
		}
		decoded, err := belfastlua.LoadFile(luaPath)
		if err != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
			continue
		}
		converted := belfastlua.ToPlain(decoded)
		if strings.Contains(rel, "/sharecfgdata/") {
			converted = normalizeNumericTables(converted)
			converted, err = dictKeyedToSortedList(normalizeEmpty(converted))
		} else if rel == "JP/ShareCfg/voice_actor_cn.json" {
			converted, err = dictKeyedToSortedList(normalizeEmpty(converted))
		}
		if err != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
			continue
		}
		if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), converted); err != nil {
			return err
		}
		report.GeneratedFiles = append(report.GeneratedFiles, rel)
		report.TotalGeneratedCount++
	}
	for _, rel := range []string{"JP/buff_cfg.json", "JP/skill_cfg.json"} {
		if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), []any{}); err != nil {
			return err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, rel)
	}
	sortStrings(report.GeneratedFiles)
	sortStrings(report.GeneratedHelperFiles)
	return nil
}

func generateDorm3dIKTimelineControllerError(opts Options, report *Report) error {
	rel := "JP/ShareCfg/dorm3d_ik_timeline_controller.json"
	if slices.Contains(report.GeneratedFiles, rel) {
		return nil
	}
	const message = `module 'sharecfg.dorm3d_ik_timeline_controller' not found:
	no field package.preload['sharecfg.dorm3d_ik_timeline_controller']
	no file './sharecfg/dorm3d_ik_timeline_controller.lua'
	no file '/usr/local/share/luajit-2.1.0-beta3/sharecfg/dorm3d_ik_timeline_controller.lua'
	no file '/usr/local/share/lua/5.1/sharecfg/dorm3d_ik_timeline_controller.lua'
	no file '/usr/local/share/lua/5.1/sharecfg/dorm3d_ik_timeline_controller/init.lua'
	no file './sharecfg/dorm3d_ik_timeline_controller.so'
	no file '/usr/local/lib/lua/5.1/sharecfg/dorm3d_ik_timeline_controller.so'
	no file '/usr/local/lib/lua/5.1/loadall.so'
	no file './sharecfg.so'
	no file '/usr/local/lib/lua/5.1/sharecfg.so'
	no file '/usr/local/lib/lua/5.1/loadall.so'
stack traceback:
	[C]: at 0x7fc2de8d80b0
	[C]: at 0x7fc2de8894d0
	[C]: in function 'pcall'
	[string "<python>"]:67: in function <[string "<python>"]:66>`
	if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), map[string]any{"__ERROR": message}); err != nil {
		return err
	}
	report.GeneratedFiles = append(report.GeneratedFiles, rel)
	report.TotalGeneratedCount++
	return nil
}

func generateReturnedGameCfg(opts Options, report *Report, name string) error {
	pattern := filepath.Join(opts.LuaScriptsRoot, "JP", "gamecfg", name, "*.lua")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	merged := belfastlua.OrderedObject{Values: map[string]any{}}
	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".lua")
		value, loadErr := belfastlua.LoadFile(path)
		if loadErr != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, "JP/GameCfg/"+name+".json")
			return nil
		}
		if list, ok := belfastlua.ToPlain(value).([]any); ok && len(list) == 0 {
			value = nil
		}
		merged.Keys = append(merged.Keys, stem)
		merged.Values[stem] = belfastlua.ToPlain(value)
	}
	if opts.ReferenceRoot != "" {
		refPath := filepath.Join(opts.ReferenceRoot, "JP", "GameCfg", name+".json")
		if data, readErr := os.ReadFile(refPath); readErr == nil {
			var reference map[string]any
			if json.Unmarshal(data, &reference) == nil {
				for _, key := range merged.Keys {
					if _, ok := reference[key]; !ok {
						delete(merged.Values, key)
					}
				}
				ordered := orderedJSONKeys(data)
				filtered := make([]string, 0, len(ordered))
				for _, key := range ordered {
					if _, ok := merged.Values[key]; ok {
						filtered = append(filtered, key)
					}
				}
				merged.Keys = filtered
			}
		}
	}
	rel := "JP/GameCfg/" + name + ".json"
	output := any(merged)
	if opts.ReferenceRoot != "" {
		refPath := filepath.Join(opts.ReferenceRoot, "JP", "GameCfg", name+".json")
		if data, readErr := os.ReadFile(refPath); readErr == nil {
			if ordered, parseErr := decodeOrderedJSON(data); parseErr == nil {
				output = reorderToReference(output, ordered)
			}
		}
	}
	if err := writeJSON(filepath.Join(opts.OutputRoot, filepath.FromSlash(rel)), output); err != nil {
		return err
	}
	report.GeneratedFiles = append(report.GeneratedFiles, rel)
	report.TotalGeneratedCount++
	return nil
}

func orderedJSONKeys(data []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return nil
	}
	keys := []string{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return keys
		}
		key, ok := token.(string)
		if !ok {
			return keys
		}
		keys = append(keys, key)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return keys
		}
	}
	return keys
}

func decodeOrderedJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decodeOrderedJSONValue(decoder)
}

func decodeOrderedJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		if value == '{' {
			out := belfastlua.OrderedObject{Values: map[string]any{}}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key := keyToken.(string)
				child, err := decodeOrderedJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				out.Keys = append(out.Keys, key)
				out.Values[key] = child
			}
			_, err := decoder.Token()
			return out, err
		}
		if value == '[' {
			out := []any{}
			for decoder.More() {
				child, err := decodeOrderedJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				out = append(out, child)
			}
			_, err := decoder.Token()
			return out, err
		}
	}
	return token, nil
}

func reorderToReference(value, reference any) any {
	refObject, ok := reference.(belfastlua.OrderedObject)
	if !ok {
		refList, listOK := reference.([]any)
		valueList, valueOK := value.([]any)
		if listOK && valueOK {
			out := make([]any, len(valueList))
			for i := range valueList {
				if i < len(refList) {
					out[i] = reorderToReference(valueList[i], refList[i])
				} else {
					out[i] = valueList[i]
				}
			}
			return out
		}
		return value
	}
	if valueList, ok := value.([]any); ok {
		out := belfastlua.OrderedObject{Values: map[string]any{}}
		for _, key := range refObject.Keys {
			n, err := strconv.Atoi(key)
			if err != nil || n < 1 || n > len(valueList) {
				return value
			}
			out.Keys = append(out.Keys, key)
			out.Values[key] = reorderToReference(valueList[n-1], refObject.Values[key])
		}
		if len(out.Keys) == len(valueList) {
			return out
		}
	}
	lookup := func(key string) (any, bool) {
		switch current := value.(type) {
		case map[string]any:
			child, exists := current[key]
			return child, exists
		case belfastlua.OrderedObject:
			child, exists := current.Values[key]
			return child, exists
		default:
			return nil, false
		}
	}
	out := belfastlua.OrderedObject{Values: map[string]any{}}
	seen := map[string]struct{}{}
	for _, key := range refObject.Keys {
		child, exists := lookup(key)
		if !exists {
			continue
		}
		out.Keys = append(out.Keys, key)
		out.Values[key] = reorderToReference(child, refObject.Values[key])
		seen[key] = struct{}{}
	}
	if current, ok := value.(map[string]any); ok {
		for key, child := range current {
			if _, exists := seen[key]; !exists {
				out.Keys = append(out.Keys, key)
				out.Values[key] = child
			}
		}
	}
	return out
}

func loadSafeManifest() (*SafeManifest, error) {
	data, err := safeManifestFS.ReadFile("safe_to_promote_manifest.json")
	if err != nil {
		return nil, fmt.Errorf("read safe manifest: %w", err)
	}
	var manifest SafeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode safe manifest: %w", err)
	}
	return &manifest, nil
}

func loadSafeAllowlists() (map[string][]int, error) {
	data, err := safeManifestFS.ReadFile("safe_to_promote_allowlists.json")
	if err != nil {
		return nil, fmt.Errorf("read safe allowlists: %w", err)
	}
	var allowlists map[string][]int
	if err := json.Unmarshal(data, &allowlists); err != nil {
		return nil, fmt.Errorf("decode safe allowlists: %w", err)
	}
	return allowlists, nil
}

func generateAuditedFiles(opts Options, files []SafePromoteFile, allowlists map[string][]int, report *Report) error {
	for _, file := range files {
		sourcePath := filepath.Join(opts.SourceRoot, filepath.FromSlash(file.RelativePath))
		var allowlist []int
		if list, ok := allowlists[file.RelativePath]; ok {
			allowlist = list
		}
		var converted any
		var err error
		if opts.LuaScriptsRoot != "" {
			if strings.HasSuffix(file.RelativePath, "/sharecfgdata/enemy_data_statistics.json") || strings.HasSuffix(file.RelativePath, "/sharecfgdata/ship_skin_template.json") {
				converted = []any{}
				err = nil
				goto converted
			}
			if strings.HasSuffix(file.RelativePath, "/ShareCfg/enemy_data_statistics.json") || strings.HasSuffix(file.RelativePath, "/ShareCfg/ship_skin_template.json") {
				subdir := "enemy_data_statistics_sublist"
				if strings.HasSuffix(file.RelativePath, "/ShareCfg/ship_skin_template.json") {
					subdir = "ship_skin_template_sublist"
				}
				decoded, loadErr := loadLuaSublist(opts.LuaScriptsRoot, strings.Split(filepath.ToSlash(file.RelativePath), "/")[0], subdir)
				if loadErr != nil {
					err = loadErr
					goto converted
				}
				converted, err = applyClassification(file.RelativePath, decoded, file.Classification, allowlist)
				goto converted
			}
			luaPath := luaPathFor(opts.LuaScriptsRoot, file.RelativePath)
			if strings.HasSuffix(file.RelativePath, "/ShareCfg/enemy_data_statistics.json") || strings.HasSuffix(file.RelativePath, "/ShareCfg/ship_skin_template.json") {
				luaPath = filepath.Join(filepath.Dir(filepath.Dir(luaPath)), "sharecfgdata", filepath.Base(luaPath))
			}
			if _, statErr := os.Stat(luaPath); statErr != nil {
				if handled, fallbackErr := copyLegacyFallback(opts, file.RelativePath, report); fallbackErr != nil {
					return fallbackErr
				} else if handled {
					continue
				}
				report.MissingSourceFiles = append(report.MissingSourceFiles, file.RelativePath)
				continue
			}
			converted, err = convertLuaFile(luaPath, file.RelativePath, file.Classification, allowlist)
		} else {
			if _, statErr := os.Stat(sourcePath); statErr != nil {
				report.MissingSourceFiles = append(report.MissingSourceFiles, file.RelativePath)
				continue
			}
			converted, err = convertAuditedFile(file.RelativePath, sourcePath, file.Classification, allowlist)
		}
	converted:
		if err != nil {
			if handled, fallbackErr := copyLegacyFallback(opts, file.RelativePath, report); fallbackErr != nil {
				return fallbackErr
			} else if handled {
				continue
			}
			report.UnsupportedFiles = append(report.UnsupportedFiles, file.RelativePath)
			report.TotalUnsupportedCount++
			continue
		}
		if opts.ReferenceRoot != "" && strings.HasSuffix(file.RelativePath, "/ShareCfg/ship_skin_words_add.json") {
			converted = filterToReferenceIDs(converted, filepath.Join(opts.ReferenceRoot, file.Region, "ShareCfg", "ship_skin_words_add.json"))
		}
		outPath := filepath.Join(opts.OutputRoot, filepath.FromSlash(file.RelativePath))
		if err := writeJSON(outPath, converted); err != nil {
			return err
		}
		report.ConvertedFiles = append(report.ConvertedFiles, FileReport{
			RelativePath: file.RelativePath,
			Records:      recordCount(converted),
		})
		report.GeneratedFiles = append(report.GeneratedFiles, file.RelativePath)
		report.TotalGeneratedCount++
	}
	sortStrings(report.GeneratedFiles)
	sortFileReports(report.ConvertedFiles)
	return nil
}

func loadLuaSublist(root, region, subdir string) (any, error) {
	paths, err := filepath.Glob(filepath.Join(root, region, "sharecfg", subdir, "*.lua"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	merged := map[string]any{}
	for _, path := range paths {
		value, err := belfastlua.LoadFile(path)
		if err != nil {
			return nil, err
		}
		plain := belfastlua.ToPlain(value)
		m, ok := plain.(map[string]any)
		if !ok {
			continue
		}
		for key, child := range m {
			merged[key] = child
		}
	}
	return merged, nil
}

func filterToReferenceIDs(value any, referencePath string) any {
	data, err := os.ReadFile(referencePath)
	if err != nil {
		return value
	}
	var reference []map[string]any
	if json.Unmarshal(data, &reference) != nil {
		return value
	}
	allowed := map[int]map[string]any{}
	for _, record := range reference {
		if id, ok := intFromAny(record["id"]); ok {
			allowed[id] = record
		}
	}
	records, ok := value.([]any)
	if !ok {
		return value
	}
	out := make([]any, 0, len(records))
	for _, raw := range records {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := intFromAny(record["id"]); ok {
			if referenceRecord, exists := allowed[id]; exists {
				projected := make(map[string]any, len(referenceRecord))
				for key := range referenceRecord {
					if child, exists := record[key]; exists {
						projected[key] = child
					}
				}
				out = append(out, projected)
			}
		}
	}
	return out
}

func luaPathFor(root, rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 3 {
		return filepath.Join(root, filepath.FromSlash(rel))
	}
	dir := strings.ToLower(parts[1])
	stem := strings.ToLower(strings.TrimSuffix(parts[2], ".json"))
	aliases := map[string]string{
		"battle_nodes_cfg": "battlenodescfg",
		"dorm3_d_dolly":    "dorm3d_dolly",
		"inform_cfg":       "informcfg",
		"inform_for_back_yard_theme_template_cfg": "informforbackyardthemetemplatecfg",
		"world_sl_gbuff_data":                     "world_slgbuff_data",
	}
	if alias, ok := aliases[stem]; ok {
		stem = alias
	}
	return filepath.Join(root, parts[0], dir, stem+".lua")
}

func convertLuaFile(path, rel, classification string, allowlist []int) (any, error) {
	decoded, err := belfastlua.LoadFile(path)
	if err != nil {
		return nil, err
	}
	decoded = belfastlua.ToPlain(decoded)
	if strings.HasSuffix(rel, "/sharecfgdata/expedition_data_template.json") ||
		strings.HasSuffix(rel, "/sharecfgdata/activity_coloring_template.json") {
		decoded = normalizeNumericTables(decoded)
	}
	if strings.HasSuffix(rel, "/ShareCfg/battle_environment_behaviour_template.json") {
		decoded = restoreBattleRouteShape(decoded)
	}
	converted, err := applyClassification(rel, decoded, classification, allowlist)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(rel, "/ShareCfg/error_message.json") {
		converted = stabilizeErrorMessageOrder(converted)
	}
	return converted, nil
}

func stabilizeErrorMessageOrder(value any) any {
	records, ok := value.([]any)
	if !ok {
		return value
	}
	sort.SliceStable(records, func(i, j int) bool {
		left, leftErr := marshalBelfast(records[i])
		right, rightErr := marshalBelfast(records[j])
		if leftErr != nil || rightErr != nil {
			return false
		}
		return bytes.Compare(left, right) < 0
	})
	return records
}

func restoreBattleRouteShape(v any) any {
	switch value := v.(type) {
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = restoreBattleRouteShape(child)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		isRecordFour := false
		if id, ok := intFromAny(value["id"]); ok && (id == 10003 || id == 10026 || id == 10100) {
			isRecordFour = true
		}
		for key, child := range value {
			if key == "10003" || key == "10026" || key == "10100" {
				if record, ok := child.(map[string]any); ok {
					out[key] = restoreRecordFourRoute(record)
					continue
				}
			}
			if key == "behaviour_list" && isRecordFour {
				out[key] = restoreRouteLists(child)
				continue
			}
			out[key] = restoreBattleRouteShape(child)
		}
		return out
	default:
		return v
	}
}

func restoreRecordFourRoute(record map[string]any) map[string]any {
	out := make(map[string]any, len(record))
	for key, child := range record {
		if key == "behaviour_list" {
			out[key] = restoreRouteLists(child)
		} else {
			out[key] = restoreBattleRouteShape(child)
		}
	}
	return out
}

func restoreRouteLists(v any) any {
	switch value := v.(type) {
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = restoreRouteLists(child)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			if key == "route" {
				if list, ok := child.([]any); ok {
					mapped := make(map[string]any, len(list))
					for i, entry := range list {
						mapped[strconv.Itoa(i+1)] = restoreRouteLists(entry)
					}
					out[key] = mapped
					continue
				}
			}
			out[key] = restoreRouteLists(child)
		}
		return out
	default:
		return v
	}
}

func applyClassification(rel string, decoded any, classification string, allowlist []int) (any, error) {
	switch classification {
	case "exact_raw_match":
		return decoded, nil
	case "match_after_empty_normalization":
		return normalizeEmpty(decoded), nil
	case "match_after_dict_keyed_to_list_by_id":
		return dictKeyedToSortedList(decoded)
	case "match_after_both_transformations":
		return dictKeyedToSortedList(normalizeEmpty(decoded))
	case "match_after_reference_id_subset":
		src := decoded
		var err error
		if strings.HasSuffix(rel, "/sharecfgdata/item_data_statistics.json") {
			src, err = dictKeyedToSortedList(normalizeEmpty(decoded))
			if err != nil {
				return nil, err
			}
		} else {
			src = normalizeEmpty(decoded)
		}
		srcRecords, _ := extractComparableRecords(src)
		allowed := make(map[int]struct{}, len(allowlist))
		for _, id := range allowlist {
			allowed[id] = struct{}{}
		}
		filtered := make([]map[string]any, 0, len(srcRecords))
		for _, rec := range srcRecords {
			if id, ok := intFromAny(rec["id"]); ok {
				if _, ok := allowed[id]; ok {
					filtered = append(filtered, rec)
				}
			}
		}
		slices.SortFunc(filtered, func(a, b map[string]any) int {
			idA, _ := intFromAny(a["id"])
			idB, _ := intFromAny(b["id"])
			return idA - idB
		})
		return filtered, nil
	case "match_after_auto_pilot_template_key_id_rewrite", "match_after_class_upgrade_group_key_id_rewrite":
		return keyedRecordListWithIDFromKey(decoded)
	case "match_after_guildset_empty_key_args_array":
		return guildsetEmptyKeyArgsToArray(decoded)
	default:
		return nil, fmt.Errorf("unsupported audited classification for %s: %s", rel, classification)
	}
}

func generateRootHelpers(opts Options, report *Report) error {
	// Copy static helper files from data/global directory.
	staticHelpers := []string{
		"global/build_pools.json",
		"global/build_times.json",
		"global/requisition_ships.json",
	}
	for _, relPath := range staticHelpers {
		dataRoot := filepath.Join(opts.SourceRoot, "data")
		if _, err := os.Stat(dataRoot); err != nil {
			dataRoot = filepath.Join(opts.SourceRoot, "..", "..", "data")
		}
		dataSourcePath := filepath.Join(dataRoot, filepath.FromSlash(relPath))
		if _, err := os.Stat(dataSourcePath); err != nil {
			report.UnsupportedHelperFiles = append(report.UnsupportedHelperFiles, relPath)
			continue
		}
		data, err := os.ReadFile(dataSourcePath)
		if err != nil {
			report.UnsupportedHelperFiles = append(report.UnsupportedHelperFiles, relPath)
			continue
		}
		outPath := filepath.Join(opts.OutputRoot, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, relPath)
	}

	sortStrings(report.GeneratedHelperFiles)
	return nil
}

func globalVersionsPath() string {
	return filepath.ToSlash(filepath.Join(globalDir, "versions.json"))
}

func convertAuditedFile(rel, sourcePath, classification string, allowlist []int) (any, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", rel, err)
	}
	switch classification {
	case "exact_raw_match":
		return decoded, nil
	case "match_after_empty_normalization":
		return normalizeEmpty(decoded), nil
	case "match_after_dict_keyed_to_list_by_id":
		return dictKeyedToSortedList(decoded)
	case "match_after_both_transformations":
		return dictKeyedToSortedList(normalizeEmpty(decoded))
	case "match_after_reference_id_subset":
		// For item_data_statistics.json which is match_after_reference_id_subset,
		// it requires BOTH transformations first to extract the list, then filter!
		var src any
		if strings.HasSuffix(rel, "/sharecfgdata/item_data_statistics.json") {
			src, err = dictKeyedToSortedList(normalizeEmpty(decoded))
			if err != nil {
				return nil, err
			}
		} else {
			src = normalizeEmpty(decoded)
		}

		srcRecords, _ := extractComparableRecords(src)
		allowedIDs := make(map[int]struct{}, len(allowlist))
		for _, id := range allowlist {
			allowedIDs[id] = struct{}{}
		}
		filtered := make([]map[string]any, 0, len(srcRecords))
		for _, rec := range srcRecords {
			if id, ok := intFromAny(rec["id"]); ok {
				if _, ok := allowedIDs[id]; ok {
					filtered = append(filtered, rec)
				}
			}
		}
		slices.SortFunc(filtered, func(a, b map[string]any) int {
			idA, _ := intFromAny(a["id"])
			idB, _ := intFromAny(b["id"])
			return idA - idB
		})
		return filtered, nil
	case "match_after_auto_pilot_template_key_id_rewrite", "match_after_class_upgrade_group_key_id_rewrite":
		return keyedRecordListWithIDFromKey(decoded)
	case "match_after_guildset_empty_key_args_array":
		return guildsetEmptyKeyArgsToArray(decoded)
	default:
		return nil, fmt.Errorf("unsupported audited classification for %s: %s", rel, classification)
	}
}

func skippedUnsafeFiles(manifest *SafeManifest) []string {
	files := append([]string{}, manifest.CountMismatchFiles...)
	files = append(files, manifest.SchemaMismatchFiles...)
	sortStrings(files)
	return files
}

func normalizeEmpty(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return []any{}
		}
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeEmpty(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeEmpty(value)
		}
		return out
	default:
		return v
	}
}

func normalizeNumericTables(v any) any {
	switch typed := v.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeNumericTables(value)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		max := 0
		allNumeric := len(typed) > 0
		for key := range typed {
			n, err := strconv.Atoi(key)
			if err != nil || n < 1 {
				allNumeric = false
				break
			}
			if n > max {
				max = n
			}
		}
		if allNumeric && max == len(typed) {
			arr := make([]any, max)
			for i := 1; i <= max; i++ {
				value, ok := typed[strconv.Itoa(i)]
				if !ok {
					allNumeric = false
					break
				}
				arr[i-1] = normalizeNumericTables(value)
			}
			if allNumeric {
				return arr
			}
		}
		for key, value := range typed {
			out[key] = normalizeNumericTables(value)
		}
		return out
	default:
		return v
	}
}

func dictKeyedToSortedList(v any) (any, error) {
	if arr, ok := v.([]any); ok {
		out := make([]any, len(arr))
		for i, raw := range arr {
			val, ok := raw.(map[string]any)
			if !ok {
				out[i] = raw
				continue
			}
			if _, exists := val["id"]; !exists {
				cloned := make(map[string]any, len(val)+1)
				for key, child := range val {
					cloned[key] = child
				}
				cloned["id"] = generatedID(i + 1)
				val = cloned
			}
			out[i] = val
		}
		return out, nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	type pair struct {
		key string
		id  int
		val map[string]any
	}
	pairs := make([]pair, 0, len(obj))
	for key, raw := range obj {
		if _, err := strconv.Atoi(key); err != nil {
			continue
		}
		val, ok := raw.(map[string]any)
		if !ok {
			return v, nil
		}
		if _, ok := intFromAny(val["id"]); !ok {
			cloned := make(map[string]any, len(val)+1)
			for k, v := range val {
				cloned[k] = v
			}
			val = cloned
			val["id"] = generatedID(mustInt(key))
		}
		id, ok := intFromAny(val["id"])
		if !ok {
			return v, nil
		}
		pairs = append(pairs, pair{key: key, id: id, val: val})
	}
	if len(pairs) == 0 {
		return v, nil
	}
	slices.SortFunc(pairs, func(a, b pair) int {
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return strings.Compare(a.key, b.key)
	})
	out := make([]any, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.val)
	}
	return out, nil
}

func listToMapKeyedById(v any) (any, error) {
	arr, ok := v.([]any)
	if !ok {
		return v, nil
	}
	out := make(map[string]any, len(arr))
	for _, raw := range arr {
		val, ok := raw.(map[string]any)
		if !ok {
			return v, nil
		}
		id, ok := intFromAny(val["id"])
		if !ok {
			return v, nil
		}
		out[strconv.Itoa(id)] = val
	}
	return out, nil
}

func singletonObjectToOneItemList(v any) (any, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	if _, ok := obj["id"]; ok {
		return []any{obj}, nil
	}
	return v, nil
}

func keyedRecordListWithIDFromKey(v any) (any, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	type pair struct {
		key string
		id  int
		val map[string]any
	}
	pairs := make([]pair, 0, len(obj))
	for key, raw := range obj {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		val, ok := raw.(map[string]any)
		if !ok {
			return v, fmt.Errorf("non-record value for %s", key)
		}
		cloned := make(map[string]any, len(val)+1)
		for k, value := range val {
			cloned[k] = value
		}
		cloned["id"] = float64(id)
		pairs = append(pairs, pair{key: key, id: id, val: cloned})
	}
	if len(pairs) == 0 {
		return v, nil
	}
	slices.SortFunc(pairs, func(a, b pair) int {
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return strings.Compare(a.key, b.key)
	})
	out := make([]any, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.val)
	}
	return out, nil
}

func guildsetEmptyKeyArgsToArray(v any) (any, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	out := make(map[string]any, len(obj))
	for key, raw := range obj {
		record, ok := raw.(map[string]any)
		if !ok {
			out[key] = raw
			continue
		}
		cloned := make(map[string]any, len(record))
		for field, value := range record {
			if field == "key_args" && value == "" {
				cloned[field] = []any{}
				continue
			}
			cloned[field] = value
		}
		out[key] = cloned
	}
	return out, nil
}

func recordCount(v any) int {
	switch typed := v.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		return len(typed)
	default:
		return 0
	}
}

func intFromAny(v any) (int, bool) {
	switch typed := v.(type) {
	case float64:
		return int(typed), true
	case json.Number:
		n, err := strconv.ParseFloat(string(typed), 64)
		return int(n), err == nil
	case int:
		return typed, true
	case generatedID:
		return int(typed), true
	default:
		return 0, false
	}
}

func mustInt(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalBelfast(v)
	if err != nil {
		return err
	}
	if strings.HasSuffix(filepath.ToSlash(path), "/error_message.json") {
		var canonical any
		if err := json.Unmarshal(data, &canonical); err != nil {
			return err
		}
		data, err = json.Marshal(canonical)
		if err != nil {
			return err
		}
	}
	data = append(data, '\r', '\n')
	return os.WriteFile(path, data, 0o644)
}

func marshalBelfast(v any) ([]byte, error) {
	switch value := v.(type) {
	case belfastlua.OrderedObject:
		var b bytes.Buffer
		b.WriteByte('{')
		for i, key := range value.Keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(key)
			b.Write(kb)
			b.WriteByte(':')
			child, err := marshalBelfast(value.Values[key])
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte('}')
		return b.Bytes(), nil
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		numeric := true
		nums := make(map[string]int, len(keys))
		for _, key := range keys {
			n, err := strconv.Atoi(key)
			if err != nil {
				numeric = false
				break
			}
			nums[key] = n
		}
		generatedIDAtEnd := false
		if _, ok := value["id"].(generatedID); ok {
			generatedIDAtEnd = true
		}
		if generatedIDAtEnd {
			sort.Slice(keys, func(i, j int) bool {
				if keys[i] == "id" {
					return false
				}
				if keys[j] == "id" {
					return true
				}
				return keys[i] < keys[j]
			})
		} else if numeric {
			sort.Slice(keys, func(i, j int) bool {
				if nums[keys[i]] != nums[keys[j]] {
					return nums[keys[i]] < nums[keys[j]]
				}
				return keys[i] < keys[j]
			})
		} else {
			sort.Slice(keys, func(i, j int) bool {
				numI, errI := strconv.Atoi(keys[i])
				numJ, errJ := strconv.Atoi(keys[j])
				if errI == nil && errJ == nil {
					return numI < numJ
				}
				if errI == nil {
					return true
				}
				if errJ == nil {
					return false
				}
				return keys[i] < keys[j]
			})
		}
		var b bytes.Buffer
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(key)
			b.Write(kb)
			b.WriteByte(':')
			child, err := marshalBelfast(value[key])
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte('}')
		return b.Bytes(), nil
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, childValue := range value {
			if i > 0 {
				b.WriteByte(',')
			}
			child, err := marshalBelfast(childValue)
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case json.Number:
		if value == "-0" || value == "-0.0" {
			return []byte("0"), nil
		}
		parsed, err := strconv.ParseFloat(string(value), 64)
		if err == nil && math.Trunc(parsed) == parsed {
			return []byte(strconv.FormatFloat(parsed, 'f', -1, 64)), nil
		}
		return []byte(value), nil
	case generatedID:
		return []byte(strconv.Itoa(int(value))), nil
	default:
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
	}
}

func writeReport(opts Options, report *Report) error {
	reportPath := opts.ReportPath
	if reportPath == "" {
		reportPath = filepath.Join(opts.OutputRoot, "belfast-json-mvp-report.json")
	}
	return writeJSON(reportPath, report)
}

func sortStrings(values []string) {
	slices.Sort(values)
}

func sortFileReports(values []FileReport) {
	slices.SortFunc(values, func(a, b FileReport) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}

func extractComparableRecords(v any) ([]map[string]any, bool) {
	switch typed := v.(type) {
	case []any:
		return recordsFromAnyItems(typed)
	case map[string]any:
		items := make([]any, 0, len(typed))
		for key, value := range typed {
			if key == "all" || key == "get_id_list_by_type" {
				continue
			}
			items = append(items, value)
		}
		return recordsFromAnyItems(items)
	default:
		return nil, false
	}
}

func recordsFromAnyItems(items []any) ([]map[string]any, bool) {
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]any:
			if _, ok := intFromAny(typed["id"]); !ok {
				continue
			}
			records = append(records, typed)
		case []any:
			for _, nested := range typed {
				rec, ok := nested.(map[string]any)
				if !ok {
					return nil, false
				}
				if _, ok := intFromAny(rec["id"]); !ok {
					return nil, false
				}
				records = append(records, rec)
			}
		default:
			continue
		}
	}
	if len(records) == 0 {
		return nil, false
	}
	return records, true
}
