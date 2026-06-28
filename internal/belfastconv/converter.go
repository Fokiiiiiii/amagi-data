package belfastconv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var mvpFiles = []string{
	"JP/sharecfgdata/item_data_statistics.json",
	"JP/sharecfgdata/ship_data_statistics.json",
	"JP/sharecfgdata/weapon_property.json",
	"JP/sharecfgdata/equip_data_template.json",
	"JP/ShareCfg/ship_skin_template.json",
}

var fallbackHelperFiles = []string{
	"build_pools.json",
	"build_times.json",
	"requisition_ships.json",
}

var itemUsageDropAllowlist = map[int]struct{}{
	40901: {}, 40902: {}, 40903: {}, 40904: {}, 40905: {}, 40906: {}, 40907: {}, 40908: {}, 40909: {}, 40910: {},
	40911: {}, 40912: {}, 40913: {}, 40914: {}, 40915: {}, 40916: {}, 40917: {}, 40919: {}, 40920: {}, 40922: {},
	40923: {}, 40924: {}, 40925: {}, 40926: {}, 40927: {}, 40928: {}, 40929: {}, 81200: {}, 81201: {}, 81202: {},
	81203: {}, 81204: {}, 81205: {}, 81206: {}, 81207: {}, 81208: {}, 81209: {}, 81210: {}, 81211: {}, 81213: {},
	81214: {}, 81217: {}, 81218: {}, 81228: {}, 81230: {}, 81231: {}, 81232: {}, 81233: {}, 81419: {}, 81425: {},
	81439: {},
}

type Options struct {
	SourceRoot               string
	OutputRoot               string
	ReportPath               string
	LuaScriptsRoot           string
	FallbackHelperSourceRoot string
}

type FileReport struct {
	RelativePath string `json:"relative_path"`
	Records      int    `json:"records"`
}

type Report struct {
	SourceRoot              string            `json:"source_root"`
	OutputRoot              string            `json:"output_root"`
	ConvertedFiles          []FileReport      `json:"converted_files"`
	GeneratedHelperFiles    []string          `json:"generated_helper_files"`
	FallbackHelperFiles     []string          `json:"fallback_helper_files"`
	ItemUsageDropDropped    int               `json:"item_usage_drop_dropped"`
	ItemUsageDropKept       int               `json:"item_usage_drop_kept"`
	UnsupportedHelperFiles  []string          `json:"unsupported_helper_files"`
	GeneratedVersions       bool              `json:"generated_versions"`
	LuaScriptsVersionsRoot  string            `json:"lua_scripts_versions_root,omitempty"`
	LuaScriptsVersionSource map[string]string `json:"lua_scripts_version_source,omitempty"`
}

func MVPFiles() []string { return slices.Clone(mvpFiles) }

func UnsupportedHelperFiles(includeVersions bool) []string {
	if includeVersions {
		return []string{}
	}
	return []string{"versions.json"}
}

func FallbackHelperFiles() []string { return slices.Clone(fallbackHelperFiles) }

func ConvertMVP(opts Options) (*Report, error) {
	if opts.SourceRoot == "" {
		return nil, fmt.Errorf("source root is required")
	}
	if opts.OutputRoot == "" {
		return nil, fmt.Errorf("output root is required")
	}
	report := &Report{
		SourceRoot:             opts.SourceRoot,
		OutputRoot:             opts.OutputRoot,
		UnsupportedHelperFiles: UnsupportedHelperFiles(opts.LuaScriptsRoot != ""),
	}
	for _, rel := range mvpFiles {
		if err := convertOne(rel, opts.SourceRoot, opts.OutputRoot, report); err != nil {
			return nil, err
		}
	}
	if opts.LuaScriptsRoot != "" {
		versions, source, err := generateVersionsJSON(opts.LuaScriptsRoot)
		if err != nil {
			return nil, err
		}
		outPath := filepath.Join(opts.OutputRoot, "versions.json")
		if err := writeJSON(outPath, versions); err != nil {
			return nil, err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, "versions.json")
		report.GeneratedVersions = true
		report.LuaScriptsVersionsRoot = source
	}
	if opts.FallbackHelperSourceRoot != "" {
		if err := copyFallbackHelpers(opts.FallbackHelperSourceRoot, opts.OutputRoot, report); err != nil {
			return nil, err
		}
	}
	if err := writeReport(opts, report); err != nil {
		return nil, err
	}
	return report, nil
}

func convertOne(rel, sourceRoot, outputRoot string, report *Report) error {
	data, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("decode %s: %w", rel, err)
	}
	converted, dropped, kept, err := transformFile(rel, decoded)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outputRoot, filepath.FromSlash(rel)), converted); err != nil {
		return err
	}
	report.ConvertedFiles = append(report.ConvertedFiles, FileReport{RelativePath: rel, Records: recordCount(converted)})
	report.ItemUsageDropDropped += dropped
	report.ItemUsageDropKept += kept
	return nil
}

func transformFile(rel string, decoded any) (any, int, int, error) {
	normalized := normalizeEmpty(decoded)
	listified, err := dictKeyedToSortedList(normalized)
	if err != nil {
		return nil, 0, 0, err
	}
	if rel != "JP/sharecfgdata/item_data_statistics.json" {
		return listified, 0, 0, nil
	}
	items, ok := listified.([]any)
	if !ok {
		return nil, 0, 0, fmt.Errorf("item_data_statistics must become a list")
	}
	out := make([]any, 0, len(items))
	var dropped, kept int
	for _, item := range items {
		rec, ok := item.(map[string]any)
		if !ok {
			return nil, 0, 0, fmt.Errorf("item_data_statistics record must be object")
		}
		if rec["usage"] != "usage_drop" {
			out = append(out, rec)
			continue
		}
		id, ok := intFromAny(rec["id"])
		if !ok {
			return nil, 0, 0, fmt.Errorf("item_data_statistics usage_drop missing numeric id")
		}
		if _, ok := itemUsageDropAllowlist[id]; ok {
			kept++
			out = append(out, rec)
			continue
		}
		dropped++
	}
	return out, dropped, kept, nil
}

func normalizeEmpty(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return []any{}
		}
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			out[k] = normalizeEmpty(val)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, val := range typed {
			out[i] = normalizeEmpty(val)
		}
		return out
	default:
		return v
	}
}

func dictKeyedToSortedList(v any) (any, error) {
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
		val, ok := raw.(map[string]any)
		if !ok {
			return v, nil
		}
		id, ok := intFromAny(val["id"])
		if !ok {
			return v, nil
		}
		pairs = append(pairs, pair{key: key, id: id, val: val})
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
	for _, p := range pairs {
		out = append(out, p.val)
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
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeReport(opts Options, report *Report) error {
	reportPath := opts.ReportPath
	if reportPath == "" {
		reportPath = filepath.Join(opts.OutputRoot, "belfast-json-mvp-report.json")
	}
	return writeJSON(reportPath, report)
}
