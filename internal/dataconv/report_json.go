package dataconv

import "encoding/json"

type reportJSON Report

func (r Report) MarshalJSON() ([]byte, error) {
	out := reportJSON(r)
	out.Regions = stringsOrEmpty(out.Regions)
	out.Categories = stringsOrEmpty(out.Categories)
	out.GeneratedFiles = stringsOrEmpty(out.GeneratedFiles)
	out.GeneratedHelperFiles = stringsOrEmpty(out.GeneratedHelperFiles)
	out.UnsupportedFiles = stringsOrEmpty(out.UnsupportedFiles)
	out.UnsupportedHelperFiles = stringsOrEmpty(out.UnsupportedHelperFiles)
	out.MissingSourceFiles = stringsOrEmpty(out.MissingSourceFiles)
	out.MissingReferenceFiles = stringsOrEmpty(out.MissingReferenceFiles)
	out.SkippedUnsafeFiles = stringsOrEmpty(out.SkippedUnsafeFiles)
	return json.Marshal(out)
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
