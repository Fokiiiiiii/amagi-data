package belfastconv

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

//go:embed safe_to_promote_manifest.json
var safeManifestFS embed.FS

var fallbackHelperFiles = []string{
	"build_pools.json",
	"build_times.json",
	"requisition_ships.json",
}

var supportedRegions = []string{"CN", "EN", "JP", "KR", "TW"}

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
	SourceRoot              string            `json:"source_root"`
	OutputRoot              string            `json:"output_root"`
	Regions                 []string          `json:"regions"`
	Categories              []string          `json:"categories"`
	ConvertedFiles          []FileReport      `json:"converted_files"`
	GeneratedFiles          []string          `json:"generated_files"`
	GeneratedHelperFiles    []string          `json:"generated_helper_files"`
	FallbackFiles           []string          `json:"fallback_files"`
	FallbackHelperFiles     []string          `json:"fallback_helper_files"`
	UnsupportedFiles        []string          `json:"unsupported_files"`
	UnsupportedHelperFiles  []string          `json:"unsupported_helper_files"`
	MissingSourceFiles      []string          `json:"missing_source_files"`
	MissingReferenceFiles   []string          `json:"missing_reference_files"`
	SkippedUnsafeFiles      []string          `json:"skipped_unsafe_files"`
	GeneratedVersions       bool              `json:"generated_versions"`
	LuaScriptsVersionsRoot  string            `json:"lua_scripts_versions_root,omitempty"`
	LuaScriptsVersionSource map[string]string `json:"lua_scripts_version_source,omitempty"`
	TotalGeneratedCount     int               `json:"total_generated_count"`
	TotalFallbackCount      int               `json:"total_fallback_count"`
	TotalUnsupportedCount   int               `json:"total_unsupported_count"`
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

	manifest, err := loadSafeManifest()
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
		FallbackHelperFiles:    []string{},
		UnsupportedFiles:       slices.Clone(manifest.UnsupportedFiles),
		UnsupportedHelperFiles: UnsupportedHelperFiles(opts.LuaScriptsRoot != ""),
		MissingSourceFiles:     []string{},
		MissingReferenceFiles:  slices.Clone(manifest.MissingReferenceFiles),
		SkippedUnsafeFiles:     skippedUnsafeFiles(manifest),
	}

	if err := generateAuditedFiles(opts.SourceRoot, opts.OutputRoot, manifest.SafeToPromoteFiles, report); err != nil {
		return nil, err
	}
	if err := generateRootHelpers(opts.SourceRoot, opts.OutputRoot, report); err != nil {
		return nil, err
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
		report.LuaScriptsVersionSource = versions
	}
	if opts.FallbackHelperSourceRoot != "" {
		if err := copyFallbackHelpers(opts.FallbackHelperSourceRoot, opts.OutputRoot, report); err != nil {
			return nil, err
		}
		report.TotalFallbackCount = len(report.FallbackHelperFiles)
		report.FallbackFiles = append(report.FallbackFiles, report.FallbackHelperFiles...)
	}
	if err := writeReport(opts, report); err != nil {
		return nil, err
	}
	return report, nil
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

func generateAuditedFiles(sourceRoot, outputRoot string, files []SafePromoteFile, report *Report) error {
	for _, file := range files {
		sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(file.RelativePath))
		if _, err := os.Stat(sourcePath); err != nil {
			report.MissingSourceFiles = append(report.MissingSourceFiles, file.RelativePath)
			continue
		}
		converted, err := convertAuditedFile(file.RelativePath, sourcePath, file.Classification)
		if err != nil {
			report.UnsupportedFiles = append(report.UnsupportedFiles, file.RelativePath)
			report.TotalUnsupportedCount++
			continue
		}
		outPath := filepath.Join(outputRoot, filepath.FromSlash(file.RelativePath))
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

func generateRootHelpers(sourceRoot, outputRoot string, report *Report) error {
	helpers := []struct {
		sourceRel string
		targetRel string
	}{
		{sourceRel: "JP/GameCfg/buff.json", targetRel: "buff_cfg.json"},
		{sourceRel: "JP/GameCfg/skill.json", targetRel: "skill_cfg.json"},
	}
	for _, helper := range helpers {
		converted, err := convertAuditedFile(helper.sourceRel, filepath.Join(sourceRoot, filepath.FromSlash(helper.sourceRel)), "match_after_empty_normalization")
		if err != nil {
			report.UnsupportedHelperFiles = append(report.UnsupportedHelperFiles, helper.targetRel)
			report.TotalUnsupportedCount++
			continue
		}
		if err := writeJSON(filepath.Join(outputRoot, helper.targetRel), converted); err != nil {
			return err
		}
		report.GeneratedHelperFiles = append(report.GeneratedHelperFiles, helper.targetRel)
	}
	sortStrings(report.GeneratedHelperFiles)
	return nil
}

func convertAuditedFile(rel, sourcePath, classification string) (any, error) {
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
	case "match_after_list_to_map_keyed_by_id":
		return listToMapKeyedById(decoded)
	case "match_after_list_to_map_both_transformations":
		return listToMapKeyedById(normalizeEmpty(decoded))
	case "match_after_singleton_object_to_one_item_list":
		return singletonObjectToOneItemList(decoded)
	case "match_after_singleton_both_transformations":
		return singletonObjectToOneItemList(normalizeEmpty(decoded))
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

func sortStrings(values []string) {
	slices.Sort(values)
}

func sortFileReports(values []FileReport) {
	slices.SortFunc(values, func(a, b FileReport) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}
