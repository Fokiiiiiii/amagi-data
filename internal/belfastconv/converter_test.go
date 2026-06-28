package belfastconv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestConvertMVPMatchesBelfastReference(t *testing.T) {
	azurRoot, luaScriptsRoot, belfastRoot := testFixtureRoots(t, true, true, true)
	out := t.TempDir()
	report, err := ConvertMVP(Options{
		SourceRoot:               azurRoot,
		OutputRoot:               out,
		LuaScriptsRoot:           luaScriptsRoot,
		FallbackHelperSourceRoot: belfastRoot,
	})
	if err != nil {
		t.Fatalf("ConvertMVP: %v", err)
	}
	if report.ItemUsageDropKept != 51 || report.ItemUsageDropDropped != 356 {
		t.Fatalf("unexpected item_usage_drop counts: kept=%d dropped=%d", report.ItemUsageDropKept, report.ItemUsageDropDropped)
	}
	if !containsString(report.GeneratedHelperFiles, "versions.json") {
		t.Fatalf("expected generated_helper_files to contain versions.json, got %v", report.GeneratedHelperFiles)
	}
	if !reflect.DeepEqual(report.FallbackHelperFiles, []string{"build_pools.json", "build_times.json", "requisition_ships.json"}) {
		t.Fatalf("unexpected fallback_helper_files: %v", report.FallbackHelperFiles)
	}
	if containsString(report.UnsupportedHelperFiles, "versions.json") {
		t.Fatalf("versions.json should not be unsupported when generation succeeds: %v", report.UnsupportedHelperFiles)
	}
	for _, rel := range MVPFiles() {
		got := mustLoad(t, filepath.Join(out, filepath.FromSlash(rel)))
		want := mustLoad(t, filepath.Join(belfastRoot, filepath.FromSlash(rel)))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s mismatch", rel)
		}
	}
	item := mustLoadArray(t, filepath.Join(out, "JP/sharecfgdata/item_data_statistics.json"))
	if len(item) != 2378 {
		t.Fatalf("expected item_data_statistics count 2378, got %d", len(item))
	}
}

func TestVersionsGeneratedFromLuaScriptsMetadata(t *testing.T) {
	azurRoot, luaScriptsRoot, _ := testFixtureRoots(t, true, true, false)
	out := t.TempDir()
	_, err := ConvertMVP(Options{SourceRoot: azurRoot, OutputRoot: out, LuaScriptsRoot: luaScriptsRoot})
	if err != nil {
		t.Fatalf("ConvertMVP: %v", err)
	}
	got := mustLoad(t, filepath.Join(out, "versions.json"))
	want := map[string]any{"CN": "9.7.243", "EN": "9.3.222", "JP": "9.3.256", "KR": "8.5.33", "TW": "8.5.83"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versions.json mismatch: %#v", got)
	}
}

func TestFallbackHelpersCopiedOnlyWhenSourceProvided(t *testing.T) {
	azurRoot, luaScriptsRoot, belfastRoot := testFixtureRoots(t, true, true, true)
	out := t.TempDir()
	_, err := ConvertMVP(Options{SourceRoot: azurRoot, OutputRoot: out, FallbackHelperSourceRoot: belfastRoot, LuaScriptsRoot: luaScriptsRoot})
	if err != nil {
		t.Fatalf("ConvertMVP: %v", err)
	}
	for _, rel := range FallbackHelperFiles() {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err != nil {
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

func mustLoadArray(t *testing.T, path string) []any {
	t.Helper()
	v := mustLoad(t, path)
	a, ok := v.([]any)
	if !ok {
		t.Fatalf("expected array at %s", path)
	}
	return a
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func testFixtureRoots(t *testing.T, needAzur, needLua, needBelfast bool) (string, string, string) {
	t.Helper()
	azurRoot := testEnvRoot(t, "AMAGI_DATA_TEST_AZURLANE_ROOT", needAzur)
	luaRoot := testEnvRoot(t, "AMAGI_DATA_TEST_LUASCRIPTS_ROOT", needLua)
	belfastRoot := testEnvRoot(t, "AMAGI_DATA_TEST_BELFAST_FALLBACK_ROOT", needBelfast)
	return azurRoot, luaRoot, belfastRoot
}

func testEnvRoot(t *testing.T, name string, required bool) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		if required {
			t.Skipf("skipping external integration test: %s is not set", name)
		}
		return ""
	}
	return value
}
