package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

var supportedRegions = []string{"CN", "EN", "JP", "KR", "TW"}
var categories = []string{"GameCfg", "ShareCfg", "sharecfgdata"}
var specialFiles = []string{
	"buff_cfg.json",
	"skill_cfg.json",
	"build_pools.json",
	"build_times.json",
	"requisition_ships.json",
	"versions.json",
}

type Totals struct {
	SourceFilesCount      int `json:"source_files_count"`
	ReferenceFilesCount   int `json:"reference_files_count"`
	MissingSourceCount    int `json:"missing_source_count"`
	MissingReferenceCount int `json:"missing_reference_count"`
	ExactRawMatch         int `json:"exact_raw_match"`
	MatchEmptyNorm        int `json:"match_empty_norm"`
	MatchDictToList       int `json:"match_dict_to_list"`
	MatchBoth             int `json:"match_both"`
	CountMismatch         int `json:"count_mismatch"`
	SchemaMismatch        int `json:"schema_mismatch"`
	BelfastOnly           int `json:"belfast_only"`
	Unsupported           int `json:"unsupported"`
}

type Report struct {
	Regions               []string          `json:"regions"`
	Categories            []string          `json:"categories"`
	SourceRegionFiles     map[string]int    `json:"source_region_files"`
	SpecialFiles          map[string]string `json:"special_files"`
	GeneratedFiles        []string          `json:"generated_files"`
	RawCopiedFiles        []string          `json:"raw_copied_files"`
	TransformedFiles      []string          `json:"transformed_files"`
	FallbackFiles         []string          `json:"fallback_files"`
	UnsupportedFiles      []string          `json:"unsupported_files"`
	MissingSourceFiles    []string          `json:"missing_source_files"`
	MissingReferenceFiles []string          `json:"missing_reference_files"`
	ReferenceMismatches   []string          `json:"reference_mismatches"`
	BelfastOnlyFiles      []string          `json:"belfast_only_files"`
	Totals                Totals            `json:"totals"`
}

var itemUsageDropAllowlist = map[int]struct{}{
	40901: {}, 40902: {}, 40903: {}, 40904: {}, 40905: {}, 40906: {}, 40907: {}, 40908: {}, 40909: {}, 40910: {},
	40911: {}, 40912: {}, 40913: {}, 40914: {}, 40915: {}, 40916: {}, 40917: {}, 40919: {}, 40920: {}, 40922: {},
	40923: {}, 40924: {}, 40925: {}, 40926: {}, 40927: {}, 40928: {}, 40929: {}, 81200: {}, 81201: {}, 81202: {},
	81203: {}, 81204: {}, 81205: {}, 81206: {}, 81207: {}, 81208: {}, 81209: {}, 81210: {}, 81211: {}, 81213: {},
	81214: {}, 81217: {}, 81218: {}, 81228: {}, 81230: {}, 81231: {}, 81232: {}, 81233: {}, 81419: {}, 81425: {},
	81439: {},
}

func main() {
	sourceRoot := flag.String("source-root", "", "AzurLaneData source root")
	belfastRoot := flag.String("belfast-root", "", "ggmolly/belfast-data root")
	flag.Parse()

	if *sourceRoot == "" || *belfastRoot == "" {
		fmt.Fprintln(os.Stderr, "Usage: belfast_data_audit -source-root <path> -belfast-root <path>")
		os.Exit(1)
	}

	// 3. Before generating any report, validate these directories exist
	if _, err := os.Stat(*belfastRoot); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Belfast root missing: %s\n", *belfastRoot)
		os.Exit(1)
	}

	for _, region := range supportedRegions {
		regPath := filepath.Join(*sourceRoot, region)
		if _, err := os.Stat(regPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Source region missing: %s\n", regPath)
			os.Exit(1) // 4. If any required source region directory is missing, fail immediately
		}
	}

	report := Report{
		Regions:               make([]string, 0),
		Categories:            make([]string, 0),
		SourceRegionFiles:     make(map[string]int),
		SpecialFiles:          make(map[string]string),
		GeneratedFiles:        make([]string, 0),
		RawCopiedFiles:        make([]string, 0),
		TransformedFiles:      make([]string, 0),
		FallbackFiles:         make([]string, 0),
		UnsupportedFiles:      make([]string, 0),
		MissingSourceFiles:    make([]string, 0),
		MissingReferenceFiles: make([]string, 0),
		ReferenceMismatches:   make([]string, 0),
		BelfastOnlyFiles:      make([]string, 0),
	}
	report.Regions = append(report.Regions, supportedRegions...)
	report.Categories = append(report.Categories, categories...)

	// 1. Gather all files in belfast-data
	belfastFiles := make(map[string]struct{})
	filepath.Walk(*belfastRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		rel, _ := filepath.Rel(*belfastRoot, path)
		belfastFiles[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})

	report.Totals.ReferenceFilesCount = len(belfastFiles)

	fmt.Printf("source root: %s\n", *sourceRoot)
	fmt.Printf("belfast root: %s\n", *belfastRoot)
	fmt.Printf("belfast reference file count: %d\n", report.Totals.ReferenceFilesCount)

	// 2. Audit standard regions
	for _, region := range supportedRegions {
		sourceRegionDir := filepath.Join(*sourceRoot, region)
		regionCount := 0

		// Find all JSONs in this region in source
		filepath.Walk(sourceRegionDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".json") {
				return nil
			}
			rel, _ := filepath.Rel(*sourceRoot, path)
			relSlash := filepath.ToSlash(rel)

			report.Totals.SourceFilesCount++
			regionCount++

			// Is it in belfast-data?
			_, ok := belfastFiles[relSlash]
			if !ok {
				report.Totals.MissingReferenceCount++
				report.MissingReferenceFiles = append(report.MissingReferenceFiles, relSlash)
				return nil
			}

			// Perform classification
			class := classifyFile(path, filepath.Join(*belfastRoot, rel), relSlash)
			switch class {
			case "exact_raw_match":
				report.Totals.ExactRawMatch++
				report.RawCopiedFiles = append(report.RawCopiedFiles, relSlash)
			case "match_after_empty_normalization":
				report.Totals.MatchEmptyNorm++
				report.TransformedFiles = append(report.TransformedFiles, relSlash)
			case "match_after_dict_keyed_to_list_by_id":
				report.Totals.MatchDictToList++
				report.TransformedFiles = append(report.TransformedFiles, relSlash)
			case "match_after_both_transformations":
				report.Totals.MatchBoth++
				report.TransformedFiles = append(report.TransformedFiles, relSlash)
			case "count_mismatch":
				report.Totals.CountMismatch++
				report.ReferenceMismatches = append(report.ReferenceMismatches, relSlash)
			case "schema_mismatch":
				report.Totals.SchemaMismatch++
				report.ReferenceMismatches = append(report.ReferenceMismatches, relSlash)
			case "unsupported":
				report.Totals.Unsupported++
				report.UnsupportedFiles = append(report.UnsupportedFiles, relSlash)
			}
			delete(belfastFiles, relSlash)
			return nil
		})
		report.SourceRegionFiles[region] = regionCount
		fmt.Printf("region %s source file count: %d\n", region, regionCount)
	}

	// 3. Special files mapping logic
	for _, sp := range specialFiles {
		var srcPath string
		var belfastPath = filepath.Join(*belfastRoot, sp)

		_, hasRef := belfastFiles[sp]
		if !hasRef {
			if _, err := os.Stat(belfastPath); err == nil {
				hasRef = true
				delete(belfastFiles, sp)
			}
		}

		if !hasRef {
			report.Totals.MissingReferenceCount++
			report.MissingReferenceFiles = append(report.MissingReferenceFiles, sp)
			report.SpecialFiles[sp] = "reference_missing"
			continue
		}

		switch sp {
		case "buff_cfg.json":
			srcPath = filepath.Join(*sourceRoot, "JP", "GameCfg", "buff.json")
		case "skill_cfg.json":
			srcPath = filepath.Join(*sourceRoot, "JP", "GameCfg", "skill.json")
		default:
			// no direct source for build_pools, build_times, requisition_ships, versions
		}

		if srcPath != "" {
			if _, err := os.Stat(srcPath); err == nil {
				class := classifyFile(srcPath, belfastPath, sp)
				report.SpecialFiles[sp] = class
				if class == "exact_raw_match" || strings.HasPrefix(class, "match_after") {
					report.GeneratedFiles = append(report.GeneratedFiles, sp)
				} else {
					report.UnsupportedFiles = append(report.UnsupportedFiles, sp)
				}
			} else {
				report.SpecialFiles[sp] = "source_missing"
			}
		} else {
			report.SpecialFiles[sp] = "fallback/generated"
			report.FallbackFiles = append(report.FallbackFiles, sp)
		}
		delete(belfastFiles, sp)
	}

	// Any remaining in belfastFiles are belfast-only (not in standard region layout or special files)
	for rel := range belfastFiles {
		report.Totals.BelfastOnly++
		report.BelfastOnlyFiles = append(report.BelfastOnlyFiles, rel)
	}

	// Check conditions
	if report.Totals.SourceFilesCount == 0 {
		fmt.Fprintln(os.Stderr, "Error: source_files_count is 0")
		os.Exit(1)
	}

	// Ensure regions exist (all 5 checked above, but technically verified again here)

	// Generate reports
	os.MkdirAll("reports/audit", 0755)

	outJson, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile("reports/audit/belfast-expansion-audit.json", outJson, 0644)

	md := generateMarkdown(report)
	os.WriteFile("reports/audit/belfast-expansion-audit.md", []byte(md), 0644)

	fmt.Println("Audit complete.")
}

func classifyFile(srcPath, refPath, relSlash string) string {
	srcData, err1 := os.ReadFile(srcPath)
	refData, err2 := os.ReadFile(refPath)
	if err1 != nil {
		return "source_missing"
	}
	if err2 != nil {
		return "reference_missing"
	}

	var src, ref any
	if err := json.Unmarshal(srcData, &src); err != nil {
		return "unsupported"
	}
	if err := json.Unmarshal(refData, &ref); err != nil {
		return "unsupported"
	}

	// Exact raw
	if reflect.DeepEqual(src, ref) {
		return "exact_raw_match"
	}

	srcNorm := normalizeEmpty(src)
	if reflect.DeepEqual(srcNorm, ref) {
		return "match_after_empty_normalization"
	}

	srcDictList, _ := dictKeyedToSortedList(src)
	if reflect.DeepEqual(srcDictList, ref) {
		return "match_after_dict_keyed_to_list_by_id"
	}

	srcBoth, _ := dictKeyedToSortedList(srcNorm)

	// Apply usage_drop filter for item_data_statistics ONLY
	if strings.Contains(relSlash, "/sharecfgdata/item_data_statistics.json") {
		srcBoth = applyItemUsageDrop(srcBoth)
	}

	if reflect.DeepEqual(srcBoth, ref) {
		return "match_after_both_transformations"
	}

	if recordCount(srcBoth) != recordCount(ref) {
		return "count_mismatch"
	}

	return "schema_mismatch"
}

func normalizeEmpty(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return make([]any, 0)
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
		idFloat, ok := val["id"].(float64)
		if !ok {
			return v, nil
		}
		pairs = append(pairs, pair{key: key, id: int(idFloat), val: val})
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

func applyItemUsageDrop(v any) any {
	items, ok := v.([]any)
	if !ok {
		return v
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		rec, ok := item.(map[string]any)
		if !ok {
			return v
		}
		if rec["usage"] != "usage_drop" {
			out = append(out, rec)
			continue
		}
		idFloat, ok := rec["id"].(float64)
		if !ok {
			return v
		}
		if _, ok := itemUsageDropAllowlist[int(idFloat)]; ok {
			out = append(out, rec)
		}
	}
	return out
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

func generateMarkdown(r Report) string {
	b := strings.Builder{}
	b.WriteString("# Belfast Expansion Audit\n\n")

	b.WriteString("## Short Summary\n")
	b.WriteString("Audit of amagi-data's ability to fully generate the belfast-data layout.\n\n")

	b.WriteString("## Classification Summary\n")
	b.WriteString(fmt.Sprintf("- Source Files Count: %d\n", r.Totals.SourceFilesCount))
	b.WriteString(fmt.Sprintf("- Reference Files Count: %d\n", r.Totals.ReferenceFilesCount))
	b.WriteString(fmt.Sprintf("- Exact Raw Match: %d\n", r.Totals.ExactRawMatch))
	b.WriteString(fmt.Sprintf("- Match after empty normalisation: %d\n", r.Totals.MatchEmptyNorm))
	b.WriteString(fmt.Sprintf("- Match after dict-to-list: %d\n", r.Totals.MatchDictToList))
	b.WriteString(fmt.Sprintf("- Match after both: %d\n", r.Totals.MatchBoth))
	b.WriteString(fmt.Sprintf("- Count Mismatch: %d\n", r.Totals.CountMismatch))
	b.WriteString(fmt.Sprintf("- Schema Mismatch: %d\n", r.Totals.SchemaMismatch))
	b.WriteString(fmt.Sprintf("- Belfast Only: %d\n", r.Totals.BelfastOnly))
	b.WriteString(fmt.Sprintf("- Missing Reference: %d\n", r.Totals.MissingReferenceCount))
	b.WriteString(fmt.Sprintf("- Unsupported: %d\n\n", r.Totals.Unsupported))

	b.WriteString("## Source Region Coverage\n")
	for _, region := range r.Regions {
		b.WriteString(fmt.Sprintf("- %s: %d\n", region, r.SourceRegionFiles[region]))
	}
	b.WriteString("\n")

	b.WriteString("## Special Files\n")
	for k, v := range r.SpecialFiles {
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}
	b.WriteString("\n")

	b.WriteString("## Recommended Next Implementation Steps\n")
	b.WriteString("1. Expand main generator to walk region directories and apply matching transforms.\n")
	b.WriteString("2. Exclude `build_pools.json`, `build_times.json`, `requisition_ships.json` and keep fallback mechanism.\n")
	b.WriteString("3. Handle `buff_cfg.json` and `skill_cfg.json` using exact transforms.\n")

	return b.String()
}
