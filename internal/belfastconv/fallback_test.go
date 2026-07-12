package belfastconv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyFallbackManifestIsExactlyTwoFiles(t *testing.T) {
	want := []string{"JP/ShareCfg/card_affix.json", "JP/ShareCfg/card_template.json"}
	got := LegacyFallbackFiles()
	if len(got) != len(want) {
		t.Fatalf("fallback count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fallback[%d] = %q, want %q", i, got[i], want[i])
		}
		if legacyFallbackFiles[got[i]] == "" {
			t.Fatalf("fallback %q has no fixed SHA-256", got[i])
		}
	}
}

func TestLegacyFallbackCopiesOnlyOnRequest(t *testing.T) {
	sourceRoot := t.TempDir()
	outputRoot := t.TempDir()
	for _, rel := range LegacyFallbackFiles() {
		source := filepath.Join(".", "..", "..", rel)
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateLegacyFallbackSources(sourceRoot); err != nil {
		t.Fatal(err)
	}
	report := &Report{}
	for _, rel := range LegacyFallbackFiles() {
		handled, err := copyLegacyFallback(Options{OutputRoot: outputRoot, LegacyFallbackSourceRoot: sourceRoot}, rel, report)
		if err != nil || !handled {
			t.Fatalf("copy fallback %s: handled=%v err=%v", rel, handled, err)
		}
	}
	if len(report.FallbackFileReports) != 2 || report.TotalFallbackCount != 2 {
		t.Fatalf("fallback report = %#v", report)
	}
}
