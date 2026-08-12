package main

import (
	"strings"
	"testing"

	"github.com/Fokiiiiiii/amagi-data/internal/belfastconv"
)

func TestIncompleteReportError(t *testing.T) {
	if err := incompleteReportError(&belfastconv.Report{}); err != nil {
		t.Fatalf("complete report rejected: %v", err)
	}
	err := incompleteReportError(&belfastconv.Report{
		MissingSourceFiles:     []string{"CN/ShareCfg/missing.json"},
		UnsupportedFiles:       []string{"JP/ShareCfg/unsupported.json"},
		UnsupportedHelperFiles: []string{"global/versions.json"},
	})
	if err == nil {
		t.Fatal("incomplete report accepted")
	}
	for _, part := range []string{"missing_source_files=1", "unsupported_files=1", "unsupported_helper_files=1"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("%q missing from %v", part, err)
		}
	}
}
