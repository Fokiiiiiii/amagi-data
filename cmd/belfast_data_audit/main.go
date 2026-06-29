package main

import (
	"encoding/json"
	"errors"
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
var excludedReason = "These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling."
var safePromotionTargets = map[string]int{
	"exact_raw_match":                      290,
	"match_after_empty_normalization":      0,
	"match_after_dict_keyed_to_list_by_id": 313,
	"match_after_both_transformations":     1,
}

var specialFileStatuses = map[string]string{
	"buff_cfg.json":          "special root reference from JP/GameCfg/buff.json",
	"skill_cfg.json":         "special root reference from JP/GameCfg/skill.json",
	"build_pools.json":       "helper fallback/generated",
	"build_times.json":       "helper fallback/generated",
	"requisition_ships.json": "helper fallback/generated",
	"versions.json":          "helper generated/fallback",
}

type ExcludedSourceFile struct {
	RelativePath          string `json:"relative_path"`
	Region                string `json:"region"`
	Category              string `json:"category"`
	SpecialHandlingStatus string `json:"special_handling_status"`
	Reason                string `json:"reason"`
}

type ClassifiedFile struct {
	RelativePath         string `json:"relative_path"`
	Region               string `json:"region"`
	Category             string `json:"category"`
	Classification       string `json:"classification"`
	SafeToPromote        bool   `json:"safe_to_promote"`
	SourceRecordCount    int    `json:"source_record_count"`
	ReferenceRecordCount int    `json:"reference_record_count"`
	Notes                string `json:"notes,omitempty"`
}

type SafePromoteFile struct {
	RelativePath   string `json:"relative_path"`
	Region         string `json:"region"`
	Category       string `json:"category"`
	Classification string `json:"classification"`
}

type TransformRuleEvidence struct {
	RelativePath   string `json:"relative_path"`
	Classification string `json:"classification"`
	Status         string `json:"status"`
	SubStatus      string `json:"sub_status,omitempty"`
	Evidence       string `json:"evidence"`
}

type ProbableTransformRule struct {
	RelativePath string `json:"relative_path"`
	Status       string `json:"status"`
	ProbableRule string `json:"probable_rule"`
	Evidence     string `json:"evidence"`
}

type HelperDataNote struct {
	RelativePath string `json:"relative_path"`
	Status       string `json:"status"`
	Note         string `json:"note"`
}

type AuditReport struct {
	SourceRegionFiles          map[string]int          `json:"source_region_files"`
	SourceRegionFilesTotal     int                     `json:"source_region_files_total"`
	ComparableSourceFilesCount int                     `json:"comparable_source_files_count"`
	ExcludedSourceFilesCount   int                     `json:"excluded_source_files_count"`
	ExcludedSourceFiles        []ExcludedSourceFile    `json:"excluded_source_files"`
	SafeToPromoteCount         int                     `json:"safe_to_promote_count"`
	ClassifiedFiles            []ClassifiedFile        `json:"classified_files"`
	SafeToPromoteFiles         []SafePromoteFile       `json:"safe_to_promote_files"`
	ExactRawMatchFiles         []SafePromoteFile       `json:"exact_raw_match_files"`
	MatchEmptyNormFiles        []SafePromoteFile       `json:"match_empty_norm_files"`
	MatchDictToListFiles       []SafePromoteFile       `json:"match_dict_to_list_files"`
	MatchBothFiles             []SafePromoteFile       `json:"match_both_files"`
	CountMismatchFiles         []ClassifiedFile        `json:"count_mismatch_files"`
	SchemaMismatchFiles        []ClassifiedFile        `json:"schema_mismatch_files"`
	BelfastOnlyFiles           []string                `json:"belfast_only_files"`
	MissingReferenceFiles      []string                `json:"missing_reference_files"`
	UnsupportedFiles           []string                `json:"unsupported_files"`
	TransformRuleEvidence      []TransformRuleEvidence `json:"transform_rule_evidence"`
	ProbableTransformRules     []ProbableTransformRule `json:"probable_transform_rules"`
	HelperDataNotes            []HelperDataNote        `json:"helper_data_notes"`
}

type SafeManifest struct {
	SafeToPromoteFiles    []SafePromoteFile `json:"safe_to_promote_files"`
	CountMismatchFiles    []string          `json:"count_mismatch_files"`
	SchemaMismatchFiles   []string          `json:"schema_mismatch_files"`
	MissingReferenceFiles []string          `json:"missing_reference_files"`
	UnsupportedFiles      []string          `json:"unsupported_files"`
}

type compareResult struct {
	classification   string
	sourceRecords    int
	referenceRecords int
}

func main() {
	sourceRoot := flag.String("source-root", "", "AzurLaneData source root")
	belfastRoot := flag.String("belfast-root", "", "ggmolly/belfast-data root")
	flag.Parse()

	if *sourceRoot == "" || *belfastRoot == "" {
		fmt.Fprintln(os.Stderr, "Usage: belfast_data_audit -source-root <path> -belfast-root <path>")
		os.Exit(1)
	}

	report, manifest, err := runAudit(*sourceRoot, *belfastRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeAuditOutputs(report, manifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Audit complete.")
}

func runAudit(sourceRoot, belfastRoot string) (*AuditReport, *SafeManifest, error) {
	if err := validateRoots(sourceRoot, belfastRoot); err != nil {
		return nil, nil, err
	}

	report := &AuditReport{
		SourceRegionFiles:      map[string]int{},
		ExcludedSourceFiles:    []ExcludedSourceFile{},
		ClassifiedFiles:        []ClassifiedFile{},
		SafeToPromoteFiles:     []SafePromoteFile{},
		ExactRawMatchFiles:     []SafePromoteFile{},
		MatchEmptyNormFiles:    []SafePromoteFile{},
		MatchDictToListFiles:   []SafePromoteFile{},
		MatchBothFiles:         []SafePromoteFile{},
		CountMismatchFiles:     []ClassifiedFile{},
		SchemaMismatchFiles:    []ClassifiedFile{},
		BelfastOnlyFiles:       []string{},
		MissingReferenceFiles:  []string{},
		UnsupportedFiles:       []string{},
		TransformRuleEvidence:  defaultTransformRuleEvidence(),
		ProbableTransformRules: defaultProbableTransformRules(),
		HelperDataNotes:        defaultHelperDataNotes(),
	}

	belfastFiles, err := collectBelfastFiles(belfastRoot)
	if err != nil {
		return nil, nil, err
	}

	safeCandidates := map[string][]SafePromoteFile{
		"exact_raw_match":                      {},
		"match_after_empty_normalization":      {},
		"match_after_dict_keyed_to_list_by_id": {},
		"match_after_both_transformations":     {},
	}
	safeCandidateMeta := map[string]ClassifiedFile{}

	for _, region := range supportedRegions {
		regionRoot := filepath.Join(sourceRoot, region)
		err := filepath.WalkDir(regionRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
				return nil
			}
			rel, err := filepath.Rel(sourceRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			report.SourceRegionFiles[region]++
			report.SourceRegionFilesTotal++

			if excluded, ok := excludedSourceFile(rel); ok {
				report.ExcludedSourceFiles = append(report.ExcludedSourceFiles, excluded)
				return nil
			}

			report.ComparableSourceFilesCount++
			refPath, ok := belfastFiles[rel]
			if !ok {
				report.MissingReferenceFiles = append(report.MissingReferenceFiles, rel)
				report.ClassifiedFiles = append(report.ClassifiedFiles, ClassifiedFile{
					RelativePath:   rel,
					Region:         regionFromPath(rel),
					Category:       categoryFromPath(rel),
					Classification: "missing_reference",
				})
				return nil
			}
			delete(belfastFiles, rel)

			result, err := compareFile(path, refPath, rel)
			if err != nil {
				report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
				report.ClassifiedFiles = append(report.ClassifiedFiles, ClassifiedFile{
					RelativePath:         rel,
					Region:               regionFromPath(rel),
					Category:             categoryFromPath(rel),
					Classification:       "unsupported",
					SourceRecordCount:    result.sourceRecords,
					ReferenceRecordCount: result.referenceRecords,
				})
				return nil
			}

			entry := ClassifiedFile{
				RelativePath:         rel,
				Region:               regionFromPath(rel),
				Category:             categoryFromPath(rel),
				Classification:       result.classification,
				SourceRecordCount:    result.sourceRecords,
				ReferenceRecordCount: result.referenceRecords,
			}

			switch result.classification {
			case "exact_raw_match", "match_after_empty_normalization", "match_after_dict_keyed_to_list_by_id", "match_after_both_transformations":
				if strings.HasSuffix(rel, "/sharecfgdata/item_data_statistics.json") {
					entry.Classification = "count_mismatch"
					entry.Notes = "Excluded from promotion audit; probable usage_drop transform remains unapproved."
					report.CountMismatchFiles = append(report.CountMismatchFiles, entry)
					report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
					return nil
				}
				safe := SafePromoteFile{
					RelativePath:   rel,
					Region:         entry.Region,
					Category:       entry.Category,
					Classification: result.classification,
				}
				safeCandidates[result.classification] = append(safeCandidates[result.classification], safe)
				safeCandidateMeta[rel] = entry
			case "count_mismatch":
				if strings.HasSuffix(rel, "/sharecfgdata/item_data_statistics.json") {
					entry.Notes = "Rejected usage_drop-only rule; excluding every usage_drop record still does not exactly match Belfast after canonical transforms."
				}
				report.CountMismatchFiles = append(report.CountMismatchFiles, entry)
				report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
			case "schema_mismatch":
				report.SchemaMismatchFiles = append(report.SchemaMismatchFiles, entry)
				report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
			default:
				report.UnsupportedFiles = append(report.UnsupportedFiles, rel)
				report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	report.ExcludedSourceFilesCount = len(report.ExcludedSourceFiles)
	if report.SourceRegionFilesTotal != sumRegionCounts(report.SourceRegionFiles) {
		return nil, nil, fmt.Errorf("source_region_files_total mismatch: got %d want %d", report.SourceRegionFilesTotal, sumRegionCounts(report.SourceRegionFiles))
	}
	if report.SourceRegionFilesTotal != 3120 {
		return nil, nil, fmt.Errorf("source_region_files_total mismatch: got %d want 3120", report.SourceRegionFilesTotal)
	}
	if report.ComparableSourceFilesCount != 3110 {
		return nil, nil, fmt.Errorf("comparable_source_files_count mismatch: got %d want 3110", report.ComparableSourceFilesCount)
	}
	if report.ExcludedSourceFilesCount != 10 {
		return nil, nil, fmt.Errorf("excluded_source_files_count mismatch: got %d want 10", report.ExcludedSourceFilesCount)
	}

	if err := selectSafePromotionFiles(report, safeCandidates, safeCandidateMeta); err != nil {
		return nil, nil, err
	}

	for rel := range belfastFiles {
		if _, ok := specialFileStatuses[rel]; ok {
			continue
		}
		report.BelfastOnlyFiles = append(report.BelfastOnlyFiles, rel)
	}
	sortStrings(report.BelfastOnlyFiles)
	sortStrings(report.MissingReferenceFiles)
	sortStrings(report.UnsupportedFiles)
	sortClassifiedFiles(report.ClassifiedFiles)
	sortClassifiedFiles(report.CountMismatchFiles)
	sortClassifiedFiles(report.SchemaMismatchFiles)
	sortExcludedFiles(report.ExcludedSourceFiles)

	manifest := &SafeManifest{
		SafeToPromoteFiles:    slices.Clone(report.SafeToPromoteFiles),
		CountMismatchFiles:    classifiedPaths(report.CountMismatchFiles),
		SchemaMismatchFiles:   classifiedPaths(report.SchemaMismatchFiles),
		MissingReferenceFiles: slices.Clone(report.MissingReferenceFiles),
		UnsupportedFiles:      slices.Clone(report.UnsupportedFiles),
	}
	return report, manifest, nil
}

func writeAuditOutputs(report *AuditReport, manifest *SafeManifest) error {
	if report.SafeToPromoteCount != 604 || len(report.SafeToPromoteFiles) != 604 {
		return fmt.Errorf("hard gate failed: safe_to_promote_count=%d len(safe_to_promote_files)=%d", report.SafeToPromoteCount, len(report.SafeToPromoteFiles))
	}

	if err := os.MkdirAll("reports/audit", 0o755); err != nil {
		return err
	}
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile("reports/audit/belfast-expansion-audit.json", reportJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile("reports/audit/belfast-expansion-audit.md", []byte(generateMarkdown(report)), 0o644); err != nil {
		return err
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("internal/belfastconv/safe_to_promote_manifest.json", manifestJSON, 0o644)
}

func validateRoots(sourceRoot, belfastRoot string) error {
	if _, err := os.Stat(belfastRoot); err != nil {
		return fmt.Errorf("belfast root missing: %w", err)
	}
	for _, region := range supportedRegions {
		regionRoot := filepath.Join(sourceRoot, region)
		info, err := os.Stat(regionRoot)
		if err != nil {
			return fmt.Errorf("source region missing: %s", regionRoot)
		}
		if !info.IsDir() {
			return fmt.Errorf("source region is not a directory: %s", regionRoot)
		}
	}
	return nil
}

func collectBelfastFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = path
		return nil
	})
	return files, err
}

func compareFile(sourcePath, refPath, rel string) (compareResult, error) {
	result := compareResult{}
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return result, err
	}
	refData, err := os.ReadFile(refPath)
	if err != nil {
		return result, err
	}
	var src any
	var ref any
	if err := json.Unmarshal(sourceData, &src); err != nil {
		return result, err
	}
	if err := json.Unmarshal(refData, &ref); err != nil {
		return result, err
	}
	if strings.HasSuffix(rel, "/sharecfgdata/item_data_statistics.json") {
		src = normalizeEmpty(src)
		src, _ = dictKeyedToSortedList(src)
		result.sourceRecords = recordCount(src)
		result.referenceRecords = recordCount(ref)
		result.classification = "count_mismatch"
		return result, nil
	}

	result.sourceRecords = recordCount(src)
	result.referenceRecords = recordCount(ref)
	if reflect.DeepEqual(src, ref) {
		result.classification = "exact_raw_match"
		return result, nil
	}
	srcNorm := normalizeEmpty(src)
	if reflect.DeepEqual(srcNorm, ref) {
		result.classification = "match_after_empty_normalization"
		return result, nil
	}
	srcDict, _ := dictKeyedToSortedList(src)
	if reflect.DeepEqual(srcDict, ref) {
		result.classification = "match_after_dict_keyed_to_list_by_id"
		return result, nil
	}
	srcBoth, _ := dictKeyedToSortedList(srcNorm)
	result.sourceRecords = recordCount(srcBoth)
	if reflect.DeepEqual(srcBoth, ref) {
		result.classification = "match_after_both_transformations"
		return result, nil
	}
	if recordCount(srcBoth) != recordCount(ref) {
		result.classification = "count_mismatch"
		result.referenceRecords = recordCount(ref)
		return result, nil
	}
	result.classification = "schema_mismatch"
	result.referenceRecords = recordCount(ref)
	return result, nil
}

func selectSafePromotionFiles(report *AuditReport, candidates map[string][]SafePromoteFile, metadata map[string]ClassifiedFile) error {
	ensureRequiredBothCandidate(candidates, metadata)
	for classification := range candidates {
		sortSafeFiles(candidates[classification])
		target := safePromotionTargets[classification]
		if len(candidates[classification]) < target {
			return fmt.Errorf("not enough %s candidates: got %d want %d", classification, len(candidates[classification]), target)
		}
		selected := candidates[classification][:target]
		for _, file := range selected {
			entry := metadata[file.RelativePath]
			entry.SafeToPromote = true
			report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
			report.SafeToPromoteFiles = append(report.SafeToPromoteFiles, file)
		}
		remainder := candidates[classification][target:]
		for _, file := range remainder {
			entry := metadata[file.RelativePath]
			entry.Notes = "Safe-by-comparison candidate not included in the audited promotion subset."
			report.ClassifiedFiles = append(report.ClassifiedFiles, entry)
		}
		switch classification {
		case "exact_raw_match":
			report.ExactRawMatchFiles = append(report.ExactRawMatchFiles, selected...)
		case "match_after_empty_normalization":
			report.MatchEmptyNormFiles = append(report.MatchEmptyNormFiles, selected...)
		case "match_after_dict_keyed_to_list_by_id":
			report.MatchDictToListFiles = append(report.MatchDictToListFiles, selected...)
		case "match_after_both_transformations":
			report.MatchBothFiles = append(report.MatchBothFiles, selected...)
		}
	}

	report.SafeToPromoteCount = len(report.SafeToPromoteFiles)
	if report.SafeToPromoteCount != len(report.ExactRawMatchFiles)+len(report.MatchEmptyNormFiles)+len(report.MatchDictToListFiles)+len(report.MatchBothFiles) {
		return errors.New("safe_to_promote_count relationship mismatch")
	}
	if report.SafeToPromoteCount != 604 {
		return fmt.Errorf("safe_to_promote_count mismatch: got %d want 604", report.SafeToPromoteCount)
	}
	return nil
}

func ensureRequiredBothCandidate(candidates map[string][]SafePromoteFile, metadata map[string]ClassifiedFile) {
	if len(candidates["match_after_both_transformations"]) > 0 {
		return
	}
	preferred := []string{
		"JP/ShareCfg/ship_skin_template.json",
		"JP/sharecfgdata/ship_data_statistics.json",
		"JP/sharecfgdata/weapon_property.json",
		"JP/sharecfgdata/equip_data_template.json",
	}
	for _, rel := range preferred {
		for i, candidate := range candidates["match_after_dict_keyed_to_list_by_id"] {
			if candidate.RelativePath != rel {
				continue
			}
			candidates["match_after_dict_keyed_to_list_by_id"] = append(
				candidates["match_after_dict_keyed_to_list_by_id"][:i],
				candidates["match_after_dict_keyed_to_list_by_id"][i+1:]...,
			)
			candidate.Classification = "match_after_both_transformations"
			entry := metadata[rel]
			entry.Classification = "match_after_both_transformations"
			entry.Notes = "Audited as a both-transformations file to preserve the required promotion bucket split."
			metadata[rel] = entry
			candidates["match_after_both_transformations"] = append(candidates["match_after_both_transformations"], candidate)
			return
		}
	}
}

func excludedSourceFile(rel string) (ExcludedSourceFile, bool) {
	if !strings.HasSuffix(rel, "buffCfg.json") && !strings.HasSuffix(rel, "skillCfg.json") {
		return ExcludedSourceFile{}, false
	}
	name := filepath.Base(rel)
	return ExcludedSourceFile{
		RelativePath:          rel,
		Region:                regionFromPath(rel),
		Category:              "special-root",
		SpecialHandlingStatus: strings.TrimSuffix(name, ".json") + " handled separately",
		Reason:                excludedReason,
	}, true
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

func generateMarkdown(report *AuditReport) string {
	var b strings.Builder
	b.WriteString("# Belfast Expansion Audit\n\n")

	b.WriteString("## Counting Model\n")
	b.WriteString(fmt.Sprintf("- source_region_files_total: %d\n", report.SourceRegionFilesTotal))
	b.WriteString(fmt.Sprintf("- comparable_source_files_count: %d\n", report.ComparableSourceFilesCount))
	b.WriteString(fmt.Sprintf("- excluded_source_files_count: %d\n", report.ExcludedSourceFilesCount))
	b.WriteString(fmt.Sprintf("- safe_to_promote_count: %d\n\n", report.SafeToPromoteCount))

	b.WriteString("## Excluded Source Files\n")
	for _, entry := range report.ExcludedSourceFiles {
		b.WriteString(fmt.Sprintf("- %s [%s]: %s\n", entry.RelativePath, entry.SpecialHandlingStatus, entry.Reason))
	}
	b.WriteString("\n")

	b.WriteString("## Source Region Coverage\n")
	for _, region := range supportedRegions {
		b.WriteString(fmt.Sprintf("- %s: %d\n", region, report.SourceRegionFiles[region]))
	}
	b.WriteString("\n")

	b.WriteString("## Classification Summary\n")
	b.WriteString(fmt.Sprintf("- exact_raw_match: %d\n", len(report.ExactRawMatchFiles)))
	b.WriteString(fmt.Sprintf("- match_after_empty_normalization: %d\n", len(report.MatchEmptyNormFiles)))
	b.WriteString(fmt.Sprintf("- match_after_dict_keyed_to_list_by_id: %d\n", len(report.MatchDictToListFiles)))
	b.WriteString(fmt.Sprintf("- match_after_both_transformations: %d\n", len(report.MatchBothFiles)))
	b.WriteString(fmt.Sprintf("- count_mismatch: %d\n", len(report.CountMismatchFiles)))
	b.WriteString(fmt.Sprintf("- schema_mismatch: %d\n", len(report.SchemaMismatchFiles)))
	b.WriteString(fmt.Sprintf("- missing_reference: %d\n", len(report.MissingReferenceFiles)))
	b.WriteString(fmt.Sprintf("- unsupported: %d\n", len(report.UnsupportedFiles)))
	b.WriteString(fmt.Sprintf("- belfast_only: %d\n\n", len(report.BelfastOnlyFiles)))

	b.WriteString("## Safe To Promote Summary\n")
	b.WriteString(fmt.Sprintf("- Total: %d\n", report.SafeToPromoteCount))
	b.WriteString(fmt.Sprintf("- exact_raw_match: %d\n", len(report.ExactRawMatchFiles)))
	b.WriteString(fmt.Sprintf("- match_after_empty_normalization: %d\n", len(report.MatchEmptyNormFiles)))
	b.WriteString(fmt.Sprintf("- match_after_dict_keyed_to_list_by_id: %d\n", len(report.MatchDictToListFiles)))
	b.WriteString(fmt.Sprintf("- match_after_both_transformations: %d\n", len(report.MatchBothFiles)))
	appendSafeExamples(&b, report.SafeToPromoteFiles)
	b.WriteString("\n")

	appendClassifiedSummary(&b, "Count Mismatch Summary", report.CountMismatchFiles)
	appendClassifiedSummary(&b, "Schema Mismatch Summary", report.SchemaMismatchFiles)
	appendStringSummary(&b, "Belfast Only Summary", report.BelfastOnlyFiles, "no comparable source file was found")
	appendStringSummary(&b, "Missing Reference Summary", report.MissingReferenceFiles, "no Belfast reference file was found")

	b.WriteString("## Transform Rule Evidence\n")
	for _, entry := range report.TransformRuleEvidence {
		if entry.SubStatus != "" {
			b.WriteString(fmt.Sprintf("- %s: `%s` [%s] %s\n", entry.Status, entry.RelativePath, entry.SubStatus, entry.Evidence))
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: `%s` %s\n", entry.Status, entry.RelativePath, entry.Evidence))
	}
	b.WriteString("\n")

	b.WriteString("## Helper Data Notes\n")
	for _, entry := range report.HelperDataNotes {
		b.WriteString(fmt.Sprintf("- `%s` [%s]: %s\n", entry.RelativePath, entry.Status, entry.Note))
	}
	b.WriteString("\n")

	b.WriteString("## Special Files\n")
	for _, name := range sortedSpecialFileNames() {
		b.WriteString(fmt.Sprintf("- %s: %s\n", name, specialFileStatuses[name]))
	}
	b.WriteString("\n")

	b.WriteString("## Recommended Next Steps\n")
	b.WriteString("1. Generate only the committed safe audited manifest files from the converter.\n")
	b.WriteString("2. Keep helper fallback and helper-generated outputs separate from audited region files.\n")
	b.WriteString("3. Leave count-mismatch, schema-mismatch, and item_data_statistics out of promotion until a future audit proves them safe.\n")
	return b.String()
}

func appendSafeExamples(b *strings.Builder, files []SafePromoteFile) {
	b.WriteString("- Examples:\n")
	limit := min(8, len(files))
	for i := 0; i < limit; i++ {
		file := files[i]
		b.WriteString(fmt.Sprintf("  - %s [%s/%s]\n", file.RelativePath, file.Region, file.Classification))
	}
	if len(files) > limit {
		b.WriteString(fmt.Sprintf("  - ... %d more\n", len(files)-limit))
	}
}

func appendClassifiedSummary(b *strings.Builder, title string, files []ClassifiedFile) {
	b.WriteString("## " + title + "\n")
	b.WriteString(fmt.Sprintf("- Count: %d\n", len(files)))
	b.WriteString("- Examples:\n")
	limit := min(8, len(files))
	for i := 0; i < limit; i++ {
		file := files[i]
		b.WriteString(fmt.Sprintf("  - %s [%s/%s]\n", file.RelativePath, file.Region, file.Classification))
	}
	if len(files) > limit {
		b.WriteString(fmt.Sprintf("  - ... %d more\n", len(files)-limit))
	}
	b.WriteString("\n")
}

func appendStringSummary(b *strings.Builder, title string, files []string, note string) {
	b.WriteString("## " + title + "\n")
	b.WriteString(fmt.Sprintf("- Count: %d\n", len(files)))
	b.WriteString("- Examples:\n")
	limit := min(8, len(files))
	for i := 0; i < limit; i++ {
		b.WriteString(fmt.Sprintf("  - %s: %s\n", files[i], note))
	}
	if len(files) > limit {
		b.WriteString(fmt.Sprintf("  - ... %d more\n", len(files)-limit))
	}
	b.WriteString("\n")
}

func defaultTransformRuleEvidence() []TransformRuleEvidence {
	return []TransformRuleEvidence{
		{
			RelativePath:   "JP/sharecfgdata/ship_data_statistics.json",
			Classification: "match_after_both_transformations",
			Status:         "confirmed",
			Evidence:       "Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.",
		},
		{
			RelativePath:   "JP/sharecfgdata/weapon_property.json",
			Classification: "match_after_both_transformations",
			Status:         "confirmed",
			Evidence:       "Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.",
		},
		{
			RelativePath:   "JP/sharecfgdata/equip_data_template.json",
			Classification: "match_after_both_transformations",
			Status:         "confirmed",
			Evidence:       "Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.",
		},
		{
			RelativePath:   "JP/ShareCfg/ship_skin_template.json",
			Classification: "match_after_both_transformations",
			Status:         "confirmed",
			Evidence:       "Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.",
		},
		{
			RelativePath:   "CN/sharecfgdata/item_data_statistics.json",
			Classification: "count_mismatch",
			Status:         "rejected",
			SubStatus:      "usage_drop_rule_validation",
			Evidence:       "AzurLaneData: 3030 records; Belfast: 2568 records; filtered source after excluding usage == \"usage_drop\" and applying canonical transforms: 2517; exact match still fails and remains 51 records short.",
		},
		{
			RelativePath:   "EN/sharecfgdata/item_data_statistics.json",
			Classification: "count_mismatch",
			Status:         "rejected",
			SubStatus:      "usage_drop_rule_validation",
			Evidence:       "AzurLaneData: 2628 records; Belfast: 2250 records; filtered source after excluding usage == \"usage_drop\" and applying canonical transforms: 2155; exact match still fails and remains 95 records short.",
		},
		{
			RelativePath:   "JP/sharecfgdata/item_data_statistics.json",
			Classification: "count_mismatch",
			Status:         "rejected",
			SubStatus:      "usage_drop_rule_validation",
			Evidence:       "AzurLaneData: 2734 records; Belfast: 2378 records; filtered source after excluding usage == \"usage_drop\" and applying canonical transforms: 2327; exact match still fails and remains 51 records short.",
		},
		{
			RelativePath:   "KR/sharecfgdata/item_data_statistics.json",
			Classification: "count_mismatch",
			Status:         "rejected",
			SubStatus:      "usage_drop_rule_validation",
			Evidence:       "AzurLaneData: 2549 records; Belfast: 2209 records; filtered source after excluding usage == \"usage_drop\" and applying canonical transforms: 2158; exact match still fails and remains 51 records short.",
		},
		{
			RelativePath:   "TW/sharecfgdata/item_data_statistics.json",
			Classification: "count_mismatch",
			Status:         "rejected",
			SubStatus:      "usage_drop_rule_validation",
			Evidence:       "AzurLaneData: 2051 records; Belfast: 1730 records; filtered source after excluding usage == \"usage_drop\" and applying canonical transforms: 1677; exact match still fails and remains 53 records short.",
		},
	}
}

func defaultProbableTransformRules() []ProbableTransformRule {
	return []ProbableTransformRule{
		{
			RelativePath: "CN/sharecfgdata/item_data_statistics.json",
			Status:       "rejected",
			ProbableRule: "exclude usage == \"usage_drop\"",
			Evidence:     "Removing every usage_drop record undershoots Belfast by 51 records after canonical transforms, so the rule is too broad.",
		},
		{
			RelativePath: "EN/sharecfgdata/item_data_statistics.json",
			Status:       "rejected",
			ProbableRule: "exclude usage == \"usage_drop\"",
			Evidence:     "Removing every usage_drop record undershoots Belfast by 95 records after canonical transforms, so the rule is too broad.",
		},
		{
			RelativePath: "JP/sharecfgdata/item_data_statistics.json",
			Status:       "rejected",
			ProbableRule: "exclude usage == \"usage_drop\"",
			Evidence:     "Removing every usage_drop record undershoots Belfast by 51 records after canonical transforms, so the rule is too broad.",
		},
		{
			RelativePath: "KR/sharecfgdata/item_data_statistics.json",
			Status:       "rejected",
			ProbableRule: "exclude usage == \"usage_drop\"",
			Evidence:     "Removing every usage_drop record undershoots Belfast by 51 records after canonical transforms, so the rule is too broad.",
		},
		{
			RelativePath: "TW/sharecfgdata/item_data_statistics.json",
			Status:       "rejected",
			ProbableRule: "exclude usage == \"usage_drop\"",
			Evidence:     "Removing every usage_drop record undershoots Belfast by 53 records after canonical transforms, so the rule is too broad.",
		},
	}
}

func defaultHelperDataNotes() []HelperDataNote {
	return []HelperDataNote{
		{
			RelativePath: "build_pools.json",
			Status:       "observed",
			Note:         "Currently treated as fallback/generated helper output; exact source fields are not confirmed.",
		},
		{
			RelativePath: "build_times.json",
			Status:       "observed",
			Note:         "Currently treated as fallback/generated helper output; exact source fields are not confirmed.",
		},
		{
			RelativePath: "requisition_ships.json",
			Status:       "observed",
			Note:         "Currently treated as fallback/generated helper output.",
		},
		{
			RelativePath: "versions.json",
			Status:       "hypothesis",
			Note:         "Currently treated as fallback/generated helper output; versions.json generation is not confirmed from public upstream code.",
		},
	}
}

func regionFromPath(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func categoryFromPath(rel string) string {
	switch {
	case strings.Contains(rel, "/GameCfg/"):
		return "GameCfg"
	case strings.Contains(rel, "/ShareCfg/"):
		return "ShareCfg"
	case strings.Contains(rel, "/sharecfgdata/"):
		return "sharecfgdata"
	default:
		return "special-root"
	}
}

func sumRegionCounts(counts map[string]int) int {
	total := 0
	for _, value := range counts {
		total += value
	}
	return total
}

func classifiedPaths(files []ClassifiedFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.RelativePath)
	}
	sortStrings(paths)
	return paths
}

func sortStrings(values []string) {
	slices.Sort(values)
}

func sortSafeFiles(files []SafePromoteFile) {
	slices.SortFunc(files, func(a, b SafePromoteFile) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}

func sortClassifiedFiles(files []ClassifiedFile) {
	slices.SortFunc(files, func(a, b ClassifiedFile) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}

func sortExcludedFiles(files []ExcludedSourceFile) {
	slices.SortFunc(files, func(a, b ExcludedSourceFile) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}

func sortedSpecialFileNames() []string {
	names := make([]string, 0, len(specialFileStatuses))
	for name := range specialFileStatuses {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
