package belfastconv

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportMarshalEmitsEmptyArrays(t *testing.T) {
	report := Report{}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	text := string(data)
	for _, field := range []string{
		"regions",
		"categories",
		"converted_files",
		"generated_files",
		"generated_helper_files",
		"fallback_files",
		"fallback_helper_files",
		"unsupported_files",
		"unsupported_helper_files",
		"missing_source_files",
		"reference_mismatches",
	} {
		if !strings.Contains(text, `"`+field+`":[]`) {
			t.Fatalf("expected %s to marshal as [] in %s", field, text)
		}
		if strings.Contains(text, `"`+field+`":null`) {
			t.Fatalf("expected %s not to marshal as null in %s", field, text)
		}
	}
}
