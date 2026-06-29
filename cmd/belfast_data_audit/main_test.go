package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAuditCountsAndEmptyArrays(t *testing.T) {
	sourceRoot := externalRoot(t, "AMAGI_DATA_TEST_AZURLANE_ROOT", `C:\Users\yutai\AzurLaneData`)
	belfastRoot := externalRoot(t, "AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT", `C:\Users\yutai\belfast-data`)

	if _, err := os.Stat(filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.json")); err != nil {
		t.Fatalf("expected reports/audit/belfast-expansion-audit.json to exist: %v", err)
	}

	report, manifest, err := runAudit(sourceRoot, belfastRoot)
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
	if report.SafeToPromoteCount != 604 || len(report.SafeToPromoteFiles) != 604 {
		t.Fatalf("safe_to_promote mismatch: count=%d len=%d", report.SafeToPromoteCount, len(report.SafeToPromoteFiles))
	}
	if report.SafeToPromoteCount != len(report.ExactRawMatchFiles)+len(report.MatchEmptyNormFiles)+len(report.MatchDictToListFiles)+len(report.MatchBothFiles) {
		t.Fatalf(
			"safe_to_promote_count=%d want exact_raw_match+match_empty_norm+match_dict_to_list+match_both=%d",
			report.SafeToPromoteCount,
			len(report.ExactRawMatchFiles)+len(report.MatchEmptyNormFiles)+len(report.MatchDictToListFiles)+len(report.MatchBothFiles),
		)
	}
	if len(report.MatchEmptyNormFiles) != 0 {
		t.Fatalf("expected match_empty_norm_files to stay empty, got %d", len(report.MatchEmptyNormFiles))
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if containsSafeFile(report.SafeToPromoteFiles, rel) {
			t.Fatalf("%s should not be promoted", rel)
		}
		if !containsClassifiedFile(report.CountMismatchFiles, rel) {
			t.Fatalf("%s should remain a count mismatch", rel)
		}
	}
	if !containsClassifiedFile(report.CountMismatchFiles, "CN/ShareCfg/achievement_data_template.json") {
		t.Fatalf("expected known count mismatch in count_mismatch_files")
	}
	if !containsClassifiedFile(report.SchemaMismatchFiles, "CN/ShareCfg/auto_pilot_template.json") {
		t.Fatalf("expected known schema mismatch in schema_mismatch_files")
	}
	if len(manifest.SafeToPromoteFiles) != 604 {
		t.Fatalf("manifest safe_to_promote_files=%d want 604", len(manifest.SafeToPromoteFiles))
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if containsSafeFile(manifest.SafeToPromoteFiles, rel) {
			t.Fatalf("%s should not be in safe_to_promote_manifest", rel)
		}
	}
	if len(report.TransformRuleEvidence) != 9 {
		t.Fatalf("transform_rule_evidence=%d want 9", len(report.TransformRuleEvidence))
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if !containsTransformRuleEvidence(report.TransformRuleEvidence, rel, "rejected", "usage_drop_rule_validation") {
			t.Fatalf("expected rejected usage_drop validation evidence for %s", rel)
		}
		if !containsProbableTransformRule(report.ProbableTransformRules, rel, "rejected") {
			t.Fatalf("expected rejected probable transform rule for %s", rel)
		}
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
		"count_mismatch_files",
		"schema_mismatch_files",
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
	buckets, ok := reportDoc["schema_mismatch_buckets"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema_mismatch_buckets to be present in report json")
	}
	wantBucketCounts := map[string]int{
		"map_vs_list_shape":          30,
		"empty_object_vs_empty_array": 0,
		"scalar_vs_array":             5,
		"key_order_or_id_sort":        0,
		"field_value_delta":           5,
		"unknown_schema_mismatch":     0,
	}
	reportFiles := make(map[string]struct{}, len(report.SchemaMismatchFiles))
	for _, file := range report.SchemaMismatchFiles {
		reportFiles[file.RelativePath] = struct{}{}
	}
	seen := make(map[string]struct{}, len(reportFiles))
	for bucketName, wantCount := range wantBucketCounts {
		rawFiles, ok := buckets[bucketName]
		if !ok {
			t.Fatalf("expected schema_mismatch_buckets to include %s", bucketName)
		}
		files, ok := rawFiles.([]any)
		if !ok {
			t.Fatalf("expected schema_mismatch_buckets[%s] to be a JSON array", bucketName)
		}
		if len(files) != wantCount {
			t.Fatalf("schema_mismatch_buckets[%s]=%d want %d", bucketName, len(files), wantCount)
		}
		for _, rawFile := range files {
			rel, ok := rawFile.(string)
			if !ok {
				t.Fatalf("expected schema_mismatch_buckets[%s] entries to be strings", bucketName)
			}
			if _, dup := seen[rel]; dup {
				t.Fatalf("schema_mismatch_buckets contains duplicate file %s", rel)
			}
			seen[rel] = struct{}{}
		}
	}
	if len(seen) != len(reportFiles) {
		t.Fatalf("schema_mismatch_buckets covered %d files, want %d", len(seen), len(reportFiles))
	}
	for rel := range reportFiles {
		if _, ok := seen[rel]; !ok {
			t.Fatalf("schema_mismatch_buckets missing %s", rel)
		}
	}

	markdownPath := filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.md")
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}
	for _, needle := range []string{
		"## Schema Mismatch Buckets",
		"map_vs_list_shape (30)",
		"scalar_vs_array (5)",
		"field_value_delta (5)",
	} {
		if !strings.Contains(string(markdown), needle) {
			t.Fatalf("expected markdown to contain %q", needle)
		}
	}
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
