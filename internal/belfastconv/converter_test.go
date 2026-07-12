package belfastconv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// sharedConvResult holds the output of a single full ConvertMVP run (all external
// roots) shared across integration tests to avoid redundant runs.
type sharedConvResult struct {
	outDir  string
	report  *Report
	luaRoot string
}

var (
	sharedConvOnce sync.Once
	sharedConv     *sharedConvResult
)

// TestMain cleans up the shared temp output directory after all tests finish.
func TestMain(m *testing.M) {
	code := m.Run()
	if sharedConv != nil {
		_ = os.RemoveAll(sharedConv.outDir)
	}
	os.Exit(code)
}

// initSharedConv runs ConvertMVP exactly once with the Lua source root and
// stores the result in sharedConv. It is a no-op if the root is unavailable.
func initSharedConv() {
	sharedConvOnce.Do(func() {
		luaRoot := resolveRoot("AMAGI_DATA_TEST_LUASCRIPTS_ROOT", `C:\Users\yutai\AzurLaneLuaScripts`)
		if luaRoot == "" {
			return
		}
		outDir, err := os.MkdirTemp("", "belfastconv_shared_*")
		if err != nil {
			return
		}
		report, err := ConvertMVP(Options{
			SourceRoot:     filepath.Join("..", ".."),
			OutputRoot:     outDir,
			LuaScriptsRoot: luaRoot,
		})
		if err != nil {
			_ = os.RemoveAll(outDir)
			return
		}
		sharedConv = &sharedConvResult{
			outDir:  outDir,
			report:  report,
			luaRoot: luaRoot,
		}
	})
}

// requireSharedConv returns the shared conversion result, skipping the calling
// test if the external roots are not available.
func requireSharedConv(t *testing.T) *sharedConvResult {
	t.Helper()
	initSharedConv()
	if sharedConv == nil {
		t.Skipf("skipping: full external roots not available for shared conversion")
	}
	return sharedConv
}

// resolveRoot returns the root directory from the env var or the fallback path.
// Returns "" if neither is available.
func resolveRoot(envName, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return v
	}
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback
	}
	return ""
}

func TestNormalizeEmptyAndDictOrdering(t *testing.T) {
	got := normalizeEmpty(map[string]any{
		"empty":  map[string]any{},
		"nested": []any{map[string]any{}},
	})
	want := map[string]any{
		"empty":  []any{},
		"nested": []any{[]any{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeEmpty mismatch: %#v", got)
	}

	listified, err := dictKeyedToSortedList(map[string]any{
		"2": map[string]any{"id": 2, "name": "b"},
		"1": map[string]any{"id": 1, "name": "a"},
	})
	if err != nil {
		t.Fatalf("dictKeyedToSortedList: %v", err)
	}
	gotList, ok := listified.([]any)
	if !ok || len(gotList) != 2 {
		t.Fatalf("unexpected listified result: %#v", listified)
	}
	if gotList[0].(map[string]any)["id"] != 1 || gotList[1].(map[string]any)["id"] != 2 {
		t.Fatalf("expected id ordering 1,2 got %#v", gotList)
	}
}

func TestConvertMVPGeneratesOnlyAuditedSafeFiles(t *testing.T) {
	sc := requireSharedConv(t)
	report := sc.report
	out := sc.outDir

	if len(report.GeneratedFiles) == 0 {
		t.Fatalf("expected generated audited files")
	}
	for _, rel := range []string{
		"CN/sharecfgdata/item_data_statistics.json",
		"EN/sharecfgdata/item_data_statistics.json",
		"JP/sharecfgdata/item_data_statistics.json",
		"KR/sharecfgdata/item_data_statistics.json",
		"TW/sharecfgdata/item_data_statistics.json",
	} {
		if !containsString(report.GeneratedFiles, rel) {
			t.Fatalf("%s should be promoted", rel)
		}
	}
	if !containsString(report.GeneratedFiles, "CN/ShareCfg/achievement_data_template.json") {
		t.Fatalf("expected audited safe file to be generated")
	}
	for _, rel := range []string{
		"CN/ShareCfg/auto_pilot_template.json",
		"EN/ShareCfg/auto_pilot_template.json",
		"JP/ShareCfg/auto_pilot_template.json",
		"KR/ShareCfg/auto_pilot_template.json",
		"TW/ShareCfg/auto_pilot_template.json",
		"CN/ShareCfg/class_upgrade_group.json",
		"EN/ShareCfg/class_upgrade_group.json",
		"JP/ShareCfg/class_upgrade_group.json",
		"KR/ShareCfg/class_upgrade_group.json",
		"TW/ShareCfg/class_upgrade_group.json",
		"CN/ShareCfg/guildset.json",
		"EN/ShareCfg/guildset.json",
		"JP/ShareCfg/guildset.json",
		"KR/ShareCfg/guildset.json",
		"TW/ShareCfg/guildset.json",
	} {
		if !containsString(report.GeneratedFiles, rel) {
			t.Fatalf("%s should now be generated", rel)
		}
	}
	if len(report.SkippedUnsafeFiles) != 0 {
		t.Fatalf("expected skipped_unsafe_files to be empty, got %v", report.SkippedUnsafeFiles)
	}
	if !containsString(report.GeneratedFiles, "JP/sharecfgdata/ship_data_statistics.json") {
		t.Fatalf("expected known audited safe file to be generated")
	}
	if !containsString(report.GeneratedHelperFiles, "global/versions.json") {
		t.Fatalf("expected generated_helper_files to contain versions.json, got %v", report.GeneratedHelperFiles)
	}
	if !containsString(report.GeneratedHelperFiles, "JP/buff_cfg.json") || !containsString(report.GeneratedHelperFiles, "JP/skill_cfg.json") {
		t.Fatalf("expected root helper files to be generated, got %v", report.GeneratedHelperFiles)
	}
	if !containsString(report.GeneratedHelperFiles, "global/build_pools.json") || !containsString(report.GeneratedHelperFiles, "global/build_times.json") || !containsString(report.GeneratedHelperFiles, "global/requisition_ships.json") {
		t.Fatalf("expected static helper files to be generated, got %v", report.GeneratedHelperFiles)
	}
	if containsString(report.UnsupportedHelperFiles, "global/versions.json") {
		t.Fatalf("versions.json should not be unsupported when generation succeeds: %v", report.UnsupportedHelperFiles)
	}
	for _, rel := range []string{
		"JP/buff_cfg.json",
		"JP/skill_cfg.json",
		"global/build_pools.json",
		"global/build_times.json",
		"global/requisition_ships.json",
		"global/versions.json",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected helper output %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		"buff_cfg.json",
		"skill_cfg.json",
		"build_pools.json",
		"build_times.json",
		"requisition_ships.json",
		"versions.json",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("did not expect legacy root helper output %s", rel)
		}
	}

	// Note: Removed comparison with belfast reference data after eliminating the belfast-data
	// fallback dependency. Generated files are now verified through conversion tests only.
}

// TestVersionsGeneratedFromLuaScriptsMetadata verifies versions.json content
// using the shared ConvertMVP output, which already includes lua-script version
// generation, eliminating a redundant full conversion run.
//
// We check structural invariants rather than exact version strings because the
// upstream AzurLaneLuaScripts versions change with every game patch.
func TestVersionsGeneratedFromLuaScriptsMetadata(t *testing.T) {
	sc := requireSharedConv(t)
	got := mustLoad(t, filepath.Join(sc.outDir, "global", "versions.json"))
	versions, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("versions.json expected object, got %T: %#v", got, got)
	}
	for _, region := range []string{"CN", "EN", "JP", "KR", "TW"} {
		v, exists := versions[region]
		if !exists {
			t.Fatalf("versions.json missing region %s: %#v", region, got)
		}
		s, isStr := v.(string)
		if !isStr || s == "" {
			t.Fatalf("versions.json[%s] expected non-empty string, got %#v", region, v)
		}
	}
	if len(versions) != 5 {
		t.Fatalf("versions.json expected exactly 5 regions, got %d: %#v", len(versions), got)
	}
}

// TestFallbackHelpersCopiedOnlyWhenSourceProvided confirms all fallback helper
// files are present in the shared output directory.
func TestFallbackHelpersCopiedOnlyWhenSourceProvided(t *testing.T) {
	sc := requireSharedConv(t)
	for _, rel := range FallbackHelperFiles() {
		if _, err := os.Stat(filepath.Join(sc.outDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected fallback helper %s: %v", rel, err)
		}
	}
}

func mustLoad(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return v
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
