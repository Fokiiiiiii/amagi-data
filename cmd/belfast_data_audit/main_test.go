package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunAuditCountsAndEmptyArrays(t *testing.T) {
	sourceRoot := externalRoot(t, "AMAGI_DATA_TEST_AZURLANE_ROOT", `C:\Users\yutai\AzurLaneData`)
	belfastRoot := externalRoot(t, "AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT", `C:\Users\yutai\belfast-data`)

	if _, err := os.Stat(filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.json")); err != nil {
		t.Fatalf("expected reports/audit/belfast-expansion-audit.json to exist: %v", err)
	}

	report, manifest, _, err := runAudit(sourceRoot, belfastRoot)
	if err != nil {
		t.Fatalf("runAudit: %v", err)
	}

	sum := 0
	for _, count := range report.SourceRegionFiles {
		sum += count
	}
	if report.SourceRegionFilesTotal != sum {
		t.Fatalf("source_region_files_total=%d want sum(source_region_files)=%d", report.SourceRegionFilesTotal, sum)
	}
	if report.SourceRegionFilesTotal != 3120 {
		t.Fatalf("source_region_files_total=%d want 3120", report.SourceRegionFilesTotal)
	}
	if report.ComparableSourceFilesCount != 3110 {
		t.Fatalf("comparable_source_files_count=%d want 3110", report.ComparableSourceFilesCount)
	}
	if report.ExcludedSourceFilesCount != 10 {
		t.Fatalf("excluded_source_files_count=%d want 10", report.ExcludedSourceFilesCount)
	}
	if len(report.ExcludedSourceFiles) != 10 {
		t.Fatalf("len(excluded_source_files)=%d want 10", len(report.ExcludedSourceFiles))
	}
	for _, path := range []string{
		"CN/buffCfg.json",
		"CN/skillCfg.json",
		"EN/buffCfg.json",
		"EN/skillCfg.json",
		"JP/buffCfg.json",
		"JP/skillCfg.json",
		"KR/buffCfg.json",
		"KR/skillCfg.json",
		"TW/buffCfg.json",
		"TW/skillCfg.json",
	} {
		if !containsExcludedFile(report.ExcludedSourceFiles, path) {
			t.Fatalf("expected excluded_source_files to contain %s", path)
		}
	}
	if report.SafeToPromoteCount != len(report.ExactRawMatchFiles)+len(report.MatchEmptyNormFiles)+len(report.MatchDictToListFiles)+len(report.MatchBothFiles)+len(report.MatchReferenceSubsetFiles) {
		t.Fatalf(
			"safe_to_promote_count=%d want exact_raw_match+match_empty_norm+match_dict_to_list+match_both+match_reference_subset=%d",
			report.SafeToPromoteCount,
			len(report.ExactRawMatchFiles)+len(report.MatchEmptyNormFiles)+len(report.MatchDictToListFiles)+len(report.MatchBothFiles)+len(report.MatchReferenceSubsetFiles),
		)
	}
	if len(report.MatchEmptyNormFiles) != 0 {
		t.Fatalf("expected match_empty_norm_files to stay empty, got %d", len(report.MatchEmptyNormFiles))
	}
	if len(report.MatchReferenceSubsetFiles) == 0 {
		t.Fatalf("expected match_reference_subset_files to contain promoted files")
	}
	for _, file := range report.MatchReferenceSubsetFiles {
		if !containsSafeFile(report.SafeToPromoteFiles, file.RelativePath) {
			t.Fatalf("expected %s to be promoted", file.RelativePath)
		}
		if containsClassifiedFile(report.CountMismatchFiles, file.RelativePath) {
			t.Fatalf("expected %s to leave count_mismatch_files", file.RelativePath)
		}
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if !containsSafeFile(report.SafeToPromoteFiles, rel) {
			t.Fatalf("%s should be promoted", rel)
		}
		if containsClassifiedFile(report.CountMismatchFiles, rel) {
			t.Fatalf("%s should not remain a count mismatch", rel)
		}
	}
	if len(report.CountMismatchFiles) != 0 {
		t.Fatalf("count_mismatch_files=%d want 0", len(report.CountMismatchFiles))
	}
	if len(report.CountMismatchBuckets) != 0 {
		t.Fatalf("expected count_mismatch_buckets to be empty")
	}
	bucketSum := 0
	for _, bucket := range report.CountMismatchBuckets {
		bucketSum += bucket.FileCount
	}
	if bucketSum != len(report.CountMismatchFiles) {
		t.Fatalf("count_mismatch bucket sum=%d want %d", bucketSum, len(report.CountMismatchFiles))
	}
	if len(manifest.SafeToPromoteFiles) != report.SafeToPromoteCount {
		t.Fatalf("manifest safe_to_promote_files=%d want %d", len(manifest.SafeToPromoteFiles), report.SafeToPromoteCount)
	}
	if !containsSafeFile(manifest.SafeToPromoteFiles, "CN/sharecfgdata/shop_template.json") {
		t.Fatalf("expected CN/sharecfgdata/shop_template.json to be in safe_to_promote_manifest")
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if !containsSafeFile(manifest.SafeToPromoteFiles, rel) {
			t.Fatalf("%s should be in safe_to_promote_manifest", rel)
		}
	}
	if len(report.TransformRuleEvidence) != 4 {
		t.Fatalf("transform_rule_evidence=%d want 4", len(report.TransformRuleEvidence))
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	text := string(data)
	for _, field := range []string{
		"excluded_source_files",
		"classified_files",
		"safe_to_promote_files",
		"exact_raw_match_files",
		"match_empty_norm_files",
		"match_dict_to_list_files",
		"match_both_files",
		"match_reference_subset_files",
		"count_mismatch_files",
		"count_mismatch_buckets",
		"schema_mismatch_files",
		"schema_mismatch_buckets",
		"belfast_only_files",
		"missing_reference_files",
		"unsupported_files",
		"transform_rule_evidence",
		"probable_transform_rules",
		"helper_data_notes",
	} {
		if strings.Contains(text, `"`+field+`":null`) {
			t.Fatalf("expected %s not to marshal as null in %s", field, text)
		}
	}

	reportPath := filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.json")
	reportJSON, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report json: %v", err)
	}
	var reportDoc map[string]any
	if err := json.Unmarshal(reportJSON, &reportDoc); err != nil {
		t.Fatalf("unmarshal report json: %v", err)
	}
	buckets, ok := reportDoc["count_mismatch_buckets"].(map[string]any)
	if !ok {
		t.Fatalf("expected count_mismatch_buckets to be present in report json")
	}
	if len(buckets) != 0 {
		t.Fatalf("expected count_mismatch_buckets to be empty")
	}

	if len(report.SchemaMismatchFiles) != 15 {
		t.Fatalf("schema_mismatch_files=%d want 15", len(report.SchemaMismatchFiles))
	}
	if len(report.SchemaMismatchBuckets) == 0 {
		t.Fatalf("expected schema_mismatch_buckets to be populated")
	}
	schemaSeen := make(map[string]struct{})
	schemaBucketSum := 0
	for bucketName, bucket := range report.SchemaMismatchBuckets {
		if bucket.FileCount == 0 {
			t.Fatalf("expected schema_mismatch_buckets[%s].file_count to be positive", bucketName)
		}
		if len(bucket.Files) == 0 {
			t.Fatalf("expected schema_mismatch_buckets[%s].files to be populated", bucketName)
		}
		schemaBucketSum += bucket.FileCount
		for _, rel := range bucket.Files {
			if _, dup := schemaSeen[rel]; dup {
				t.Fatalf("schema_mismatch_buckets contains duplicate file %s", rel)
			}
			schemaSeen[rel] = struct{}{}
		}
		for _, rel := range bucket.RepresentativeFiles {
			if _, ok := schemaSeen[rel]; !ok {
				t.Fatalf("schema_mismatch_buckets[%s].representative_files contains %s not present in files", bucketName, rel)
			}
		}
	}
	if schemaBucketSum != len(report.SchemaMismatchFiles) {
		t.Fatalf("schema_mismatch_buckets summed to %d, want %d", schemaBucketSum, len(report.SchemaMismatchFiles))
	}
	if len(schemaSeen) != len(report.SchemaMismatchFiles) {
		t.Fatalf("schema_mismatch_buckets covered %d files, want %d", len(schemaSeen), len(report.SchemaMismatchFiles))
	}
	for _, file := range report.SchemaMismatchFiles {
		if _, ok := schemaSeen[file.RelativePath]; !ok {
			t.Fatalf("schema_mismatch_buckets missing file %s", file.RelativePath)
		}
	}

	markdownPath := filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.md")
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}
	for _, needle := range []string{
		"## Count Mismatch Buckets",
		"## Schema Mismatch Buckets",
	} {
		if !strings.Contains(string(markdown), needle) {
			t.Fatalf("expected markdown to contain %q", needle)
		}
	}

	var exactPromoted, subsetPromoted string
	for _, file := range report.SafeToPromoteFiles {
		switch file.Classification {
		case "exact_raw_match":
			if exactPromoted == "" {
				exactPromoted = file.RelativePath
			}
		case "match_after_reference_id_subset":
			if subsetPromoted == "" {
				subsetPromoted = file.RelativePath
			}
		}
	}
	if exactPromoted == "" || subsetPromoted == "" {
		t.Fatalf("expected promoted exact raw and reference subset files to be populated")
	}
	assertKeyedExactMatch(
		t,
		filepath.Join(sourceRoot, filepath.FromSlash(exactPromoted)),
		filepath.Join(belfastRoot, filepath.FromSlash(exactPromoted)),
	)
	assertReferenceSubsetMatch(
		t,
		filepath.Join(sourceRoot, filepath.FromSlash(subsetPromoted)),
		filepath.Join(belfastRoot, filepath.FromSlash(subsetPromoted)),
	)
}

func assertKeyedExactMatch(t *testing.T, sourcePath, refPath string) {
	t.Helper()
	src := readJSONAny(t, sourcePath)
	ref := readJSONAny(t, refPath)
	got, err := dictKeyedToSortedList(src)
	if err != nil {
		t.Fatalf("dictKeyedToSortedList: %v", err)
	}
	if !reflect.DeepEqual(got, ref) {
		t.Fatalf("expected transformed source to equal Belfast reference for %s", sourcePath)
	}
}

func assertReferenceSubsetMatch(t *testing.T, sourcePath, refPath string) {
	t.Helper()
	src := readJSONAny(t, sourcePath)
	ref := readJSONAny(t, refPath)
	got, want, _, ok := referenceIDSubsetMatch(src, ref)
	if !ok {
		t.Fatalf("referenceIDSubsetMatch rejected %s", sourcePath)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected reference subset transform to equal Belfast reference for %s", sourcePath)
	}
}

func readJSONAny(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return value
}

func externalRoot(t *testing.T, envName, fallback string) string {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback
	}
	t.Skipf("skipping external audit test: %s is not set and fallback %s is unavailable", envName, fallback)
	return ""
}

func containsSafeFile(files []SafePromoteFile, target string) bool {
	for _, file := range files {
		if file.RelativePath == target {
			return true
		}
	}
	return false
}

func containsClassifiedFile(files []ClassifiedFile, target string) bool {
	for _, file := range files {
		if file.RelativePath == target {
			return true
		}
	}
	return false
}

func containsExcludedFile(files []ExcludedSourceFile, target string) bool {
	for _, file := range files {
		if file.RelativePath == target {
			return true
		}
	}
	return false
}

func containsTransformRuleEvidence(files []TransformRuleEvidence, target, status, subStatus string) bool {
	for _, file := range files {
		if file.RelativePath == target && file.Status == status && file.SubStatus == subStatus {
			return true
		}
	}
	return false
}

func containsProbableTransformRule(files []ProbableTransformRule, target, status string) bool {
	for _, file := range files {
		if file.RelativePath == target && file.Status == status {
			return true
		}
	}
	return false
}
