package belfastconv

import "encoding/json"

type reportJSON Report

func (r Report) MarshalJSON() ([]byte, error) {
	out := reportJSON(r)
	out.Regions = stringsOrEmpty(out.Regions)
	out.Categories = stringsOrEmpty(out.Categories)
	out.ConvertedFiles = fileReportsOrEmpty(out.ConvertedFiles)
	out.GeneratedFiles = stringsOrEmpty(out.GeneratedFiles)
	out.GeneratedHelperFiles = stringsOrEmpty(out.GeneratedHelperFiles)
	out.FallbackFiles = stringsOrEmpty(out.FallbackFiles)
	out.FallbackHelperFiles = stringsOrEmpty(out.FallbackHelperFiles)
	out.UnsupportedFiles = stringsOrEmpty(out.UnsupportedFiles)
	out.UnsupportedHelperFiles = stringsOrEmpty(out.UnsupportedHelperFiles)
	out.MissingSourceFiles = stringsOrEmpty(out.MissingSourceFiles)
	out.ReferenceMismatches = stringsOrEmpty(out.ReferenceMismatches)
	return json.Marshal(out)
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func fileReportsOrEmpty(values []FileReport) []FileReport {
	if values == nil {
		return []FileReport{}
	}
	return values
}
