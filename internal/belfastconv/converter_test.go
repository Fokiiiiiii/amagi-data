package belfastconv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	testAzurRoot       = `C:\Users\yutai\repos\azurlane-compare-last-json-jp\json-last-jp`
	testBelfastRoot    = `C:\Users\yutai\repos\belfast-data`
	testLuaScriptsRoot = `C:\Users\yutai\repos\AzurLaneLuaScripts`
)

func TestConvertMVPMatchesBelfastReference(t *testing.T) {
	out := t.TempDir()
	report, err := ConvertMVP(Options{SourceRoot: testAzurRoot, OutputRoot: out, LuaScriptsRoot: testLuaScriptsRoot})
	if err != nil {
		t.Fatalf("ConvertMVP: %v", err)
	}
	if report.ItemUsageDropKept != 51 || report.ItemUsageDropDropped != 356 {
		t.Fatalf("unexpected item_usage_drop counts: kept=%d dropped=%d", report.ItemUsageDropKept, report.ItemUsageDropDropped)
	}
	for _, rel := range MVPFiles() {
		got := mustLoad(t, filepath.Join(out, filepath.FromSlash(rel)))
		want := mustLoad(t, filepath.Join(testBelfastRoot, filepath.FromSlash(rel)))
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
	out := t.TempDir()
	_, err := ConvertMVP(Options{SourceRoot: testAzurRoot, OutputRoot: out, LuaScriptsRoot: testLuaScriptsRoot})
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
	out := t.TempDir()
	_, err := ConvertMVP(Options{SourceRoot: testAzurRoot, OutputRoot: out, FallbackHelperSourceRoot: testBelfastRoot, LuaScriptsRoot: testLuaScriptsRoot})
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
