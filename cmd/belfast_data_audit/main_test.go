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
		if containsSafeFile(report.SafeToPromoteFiles, rel) {
			t.Fatalf("%s should not be promoted", rel)
		}
		if !containsClassifiedFile(report.CountMismatchFiles, rel) {
			t.Fatalf("%s should remain a count mismatch", rel)
		}
	}
	if len(report.CountMismatchFiles) != 5 {
		t.Fatalf("count_mismatch_files=%d want 5", len(report.CountMismatchFiles))
	}
	if len(report.CountMismatchBuckets) == 0 {
		t.Fatalf("expected count_mismatch_buckets to be populated")
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
		"match_reference_subset_files",
		"count_mismatch_files",
		"count_mismatch_buckets",
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
	buckets, ok := reportDoc["count_mismatch_buckets"].(map[string]any)
	if !ok {
		t.Fatalf("expected count_mismatch_buckets to be present in report json")
	}
	if len(buckets) == 0 {
		t.Fatalf("expected count_mismatch_buckets to contain entries")
	}
	seen := make(map[string]struct{})
	totalBucketFiles := 0
	for bucketName, rawBucket := range buckets {
		bucket, ok := rawBucket.(map[string]any)
		if !ok {
			t.Fatalf("expected count_mismatch_buckets[%s] to be a JSON object", bucketName)
		}
		for _, field := range []string{"name", "file_count", "files", "representative_files", "source_count", "reference_count", "delta", "status"} {
			if _, ok := bucket[field]; !ok {
				t.Fatalf("expected count_mismatch_buckets[%s] to contain %s", bucketName, field)
			}
		}
		rawCount, ok := bucket["file_count"].(float64)
		if !ok {
			t.Fatalf("expected count_mismatch_buckets[%s].file_count to be numeric", bucketName)
		}
		totalBucketFiles += int(rawCount)
		rawFiles, ok := bucket["files"].([]any)
		if !ok || len(rawFiles) == 0 {
			t.Fatalf("expected count_mismatch_buckets[%s].files to be a non-empty array", bucketName)
		}
		for _, rawFile := range rawFiles {
			rel, ok := rawFile.(string)
			if !ok {
				t.Fatalf("expected count_mismatch_buckets[%s].files entries to be strings", bucketName)
			}
			if _, dup := seen[rel]; dup {
				t.Fatalf("count_mismatch_buckets contains duplicate file %s", rel)
			}
			seen[rel] = struct{}{}
		}
		rawRep, ok := bucket["representative_files"].([]any)
		if !ok || len(rawRep) == 0 {
			t.Fatalf("expected count_mismatch_buckets[%s].representative_files to be a non-empty array", bucketName)
		}
		for _, rawFile := range rawRep {
			rel, ok := rawFile.(string)
			if !ok {
				t.Fatalf("expected count_mismatch_buckets[%s] entries to be strings", bucketName)
			}
			if _, ok := seen[rel]; !ok {
				t.Fatalf("count_mismatch_buckets[%s].representative_files contains %s not present in files", bucketName, rel)
			}
		}
	}
	if totalBucketFiles != len(report.CountMismatchFiles) {
		t.Fatalf("count_mismatch_buckets summed to %d, want %d", totalBucketFiles, len(report.CountMismatchFiles))
	}
	if len(seen) != len(report.CountMismatchFiles) {
		t.Fatalf("count_mismatch_buckets covered %d files, want %d", len(seen), len(report.CountMismatchFiles))
	}
	for _, file := range report.CountMismatchFiles {
		if _, ok := seen[file.RelativePath]; !ok {
			t.Fatalf("count_mismatch_buckets missing file %s", file.RelativePath)
		}
	}

	markdownPath := filepath.Join("..", "..", "reports", "audit", "belfast-expansion-audit.md")
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}
	for _, needle := range []string{
		"## Count Mismatch Buckets",
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
	got, want, ok := referenceIDSubsetMatch(src, ref)
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
