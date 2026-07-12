package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type counts struct {
	Total      int `json:"total"`
	Matched    int `json:"matched"`
	Mismatched int `json:"mismatched"`
	Missing    int `json:"missing"`
	Extra      int `json:"extra"`
}

type summary struct {
	LuaDerived     counts `json:"lua_derived"`
	BelfastHelpers counts `json:"belfast_helpers"`
	AllOutputs     counts `json:"all_outputs"`
}

type helper struct {
	Path             string
	SourceKind       string
	SourcePath       string
	GenerationMethod string
}

type manifestRow struct {
	Path              string
	Classification    string
	ReferenceExists   bool
	GeneratedExists   bool
	DuplicateCount    int
	ComparisonEnabled bool
	Reason            string
}

type manifestSummary struct {
	ReferenceTotalFiles int `json:"reference_total_files"`
	LuaDerivedFiles     int `json:"lua_derived_files"`
	BelfastHelperFiles  int `json:"belfast_helper_files"`
	ExcludedFiles       int `json:"excluded_files"`
	DuplicatePaths      int `json:"duplicate_paths"`
	UnknownFiles        int `json:"unknown_files"`
}

type versionSource struct {
	Repository      string `json:"repository"`
	Commit          string `json:"commit"`
	Path            string `json:"path"`
	Version         string `json:"version"`
	CommitDate      string `json:"commit_date"`
	BeforeReference bool   `json:"before_reference"`
	SelectionReason string `json:"selection_reason"`
}

type semanticRow struct {
	Classification string
	FirstPath      string
	ReferenceType  string
	GeneratedType  string
}

func fileMap(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = path
		return nil
	})
	return result, err
}

func digest(path string) (string, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), b, nil
}

func compare(reference, generated map[string]string) counts {
	keys := map[string]bool{}
	for key := range reference {
		keys[key] = true
	}
	for key := range generated {
		keys[key] = true
	}
	result := counts{}
	for key := range keys {
		result.Total++
		r, rok := reference[key]
		g, gok := generated[key]
		if !rok {
			result.Extra++
			continue
		}
		if !gok {
			result.Missing++
			continue
		}
		rh, rb, _ := digest(r)
		gh, gb, _ := digest(g)
		if rh == gh && string(rb) == string(gb) {
			result.Matched++
		} else {
			result.Mismatched++
		}
	}
	return result
}

func add(a, b counts) counts {
	return counts{Total: a.Total + b.Total, Matched: a.Matched + b.Matched, Mismatched: a.Mismatched + b.Mismatched, Missing: a.Missing + b.Missing, Extra: a.Extra + b.Extra}
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\r', '\n')
	return os.WriteFile(path, b, 0644)
}

func writeManifest(out string, luaRef, luaGen map[string]string, helpers []helper, refRoot, genRoot string) error {
	ref := map[string]string{}
	gen := map[string]string{}
	for path, value := range luaRef {
		ref["JP/"+path] = value
	}
	for path, value := range luaGen {
		gen["JP/"+path] = value
	}
	for _, h := range helpers {
		ref[h.Path] = filepath.Join(refRoot, filepath.FromSlash(h.Path[7:]))
		gen[h.Path] = filepath.Join(genRoot, filepath.FromSlash(h.Path))
	}
	keys := map[string]bool{}
	for path := range ref {
		keys[path] = true
	}
	for path := range gen {
		keys[path] = true
	}
	all := make([]string, 0, len(keys))
	for path := range keys {
		all = append(all, path)
	}
	sort.Strings(all)
	caseCount := map[string]int{}
	for path := range ref {
		caseCount[strings.ToLower(path)]++
	}
	rows := make([]manifestRow, 0, len(all))
	summary := manifestSummary{}
	for _, path := range all {
		classification := "unknown"
		reason := "path is outside the fixed JP/helper manifest"
		if strings.HasPrefix(path, "JP/") {
			classification, reason = "lua_derived", "JP reference data"
		}
		if strings.HasPrefix(path, "global/") {
			classification, reason = "belfast_helper", "Belfast helper; separately verified"
		}
		duplicate := caseCount[strings.ToLower(path)] - 1
		row := manifestRow{Path: path, Classification: classification, ReferenceExists: ref[path] != "", GeneratedExists: gen[path] != "", DuplicateCount: duplicate, ComparisonEnabled: classification != "unknown", Reason: reason}
		if !row.ReferenceExists && row.GeneratedExists {
			row.Reason = "generated-only path"
		}
		rows = append(rows, row)
		if row.ReferenceExists {
			summary.ReferenceTotalFiles++
		}
		switch classification {
		case "lua_derived":
			summary.LuaDerivedFiles++
		case "belfast_helper":
			summary.BelfastHelperFiles++
		case "excluded_non_data":
			summary.ExcludedFiles++
		default:
			summary.UnknownFiles++
		}
		if duplicate > 0 {
			summary.DuplicatePaths++
		}
	}
	file, err := os.Create(filepath.Join(out, "reference-manifest.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	_ = w.Write([]string{"path", "classification", "reference_exists", "generated_exists", "duplicate_count", "comparison_enabled", "reason"})
	for _, row := range rows {
		_ = w.Write([]string{row.Path, row.Classification, fmt.Sprintf("%t", row.ReferenceExists), fmt.Sprintf("%t", row.GeneratedExists), fmt.Sprintf("%d", row.DuplicateCount), fmt.Sprintf("%t", row.ComparisonEnabled), row.Reason})
	}
	w.Flush()
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	if err := w.Error(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(out, "manifest-summary.json"), summary)
}

func versionCandidates(luaRoot, referenceRoot, referenceCommit, out string) error {
	data, err := os.ReadFile(filepath.Join(referenceRoot, "versions.json"))
	if err != nil {
		return err
	}
	expected := map[string]string{}
	if err := json.Unmarshal(data, &expected); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(out, "version-reference.json"), expected); err != nil {
		return err
	}
	regions := []string{"CN", "EN", "JP", "KR", "TW"}
	args := []string{"-C", luaRoot, "log", "--all", "--since=2026-03-01", "--until=2026-05-31", "--format=%H|%cI", "--", "versions/CN.txt", "versions/EN.txt", "versions/JP.txt", "versions/KR.txt", "versions/TW.txt"}
	log, err := exec.Command("git", args...).Output()
	if err != nil {
		return err
	}
	type candidate struct {
		Commit, Date string
		Values       map[string]string
	}
	seen := map[string]bool{}
	candidates := []candidate{}
	for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(parts) != 2 || seen[parts[0]] {
			continue
		}
		seen[parts[0]] = true
		c := candidate{Commit: parts[0], Date: parts[1], Values: map[string]string{}}
		refs := strings.Builder{}
		for _, region := range regions {
			refs.WriteString(c.Commit + ":versions/" + region + ".txt\n")
		}
		cmd := exec.Command("git", "-C", luaRoot, "cat-file", "--batch")
		cmd.Stdin = strings.NewReader(refs.String())
		batch, batchErr := cmd.Output()
		if batchErr == nil {
			reader := bufio.NewReader(bytes.NewReader(batch))
			for _, region := range regions {
				header, readErr := reader.ReadString('\n')
				if readErr != nil {
					break
				}
				fields := strings.Fields(strings.TrimSpace(header))
				if len(fields) < 3 || fields[1] != "blob" {
					continue
				}
				size, parseErr := strconv.Atoi(fields[2])
				if parseErr != nil {
					break
				}
				value := make([]byte, size)
				if _, readErr = io.ReadFull(reader, value); readErr != nil {
					break
				}
				_, _ = reader.ReadByte()
				c.Values[region] = strings.TrimSpace(string(value))
			}
		}
		candidates = append(candidates, c)
	}
	file, err := os.Create(filepath.Join(out, "version-snapshot-candidates.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	_ = w.Write([]string{"commit", "date", "CN", "EN", "JP", "KR", "TW", "matched_regions", "full_match"})
	for _, c := range candidates {
		matched := 0
		full := true
		values := []string{}
		for _, region := range regions {
			value := c.Values[region]
			values = append(values, value)
			if value == expected[region] && value != "" {
				matched++
			} else {
				full = false
			}
		}
		_ = w.Write(append([]string{c.Commit, c.Date}, append(values, fmt.Sprintf("%d", matched), fmt.Sprintf("%t", full))...))
	}
	w.Flush()
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	if err := w.Error(); err != nil {
		return err
	}
	refDateBytes, err := exec.Command("git", "-C", referenceRoot, "show", "-s", "--format=%cI", referenceCommit).Output()
	if err != nil {
		return err
	}
	refDate, err := time.Parse(time.RFC3339, strings.TrimSpace(string(refDateBytes)))
	if err != nil {
		return err
	}
	sources := map[string]versionSource{}
	for _, region := range regions {
		var beforeCandidate, afterCandidate *candidate
		var beforeTime, afterTime time.Time
		for i := range candidates {
			c := &candidates[i]
			if c.Values[region] != expected[region] || c.Values[region] == "" {
				continue
			}
			date, parseErr := time.Parse(time.RFC3339, c.Date)
			if parseErr != nil {
				continue
			}
			if !date.After(refDate) && (beforeCandidate == nil || date.After(beforeTime)) {
				beforeCandidate, beforeTime = c, date
			}
			if date.After(refDate) && (afterCandidate == nil || date.Before(afterTime)) {
				afterCandidate, afterTime = c, date
			}
		}
		selected, selectedTime := beforeCandidate, beforeTime
		if selected == nil {
			selected, selectedTime = afterCandidate, afterTime
		}
		if selected == nil {
			sources[region] = versionSource{Repository: "AzurLaneTools/AzurLaneLuaScripts", Path: "versions/" + region + ".txt", Version: expected[region], SelectionReason: "no matching commit found in scanned history"}
			continue
		}
		before := !selectedTime.After(refDate)
		reason := "closest matching commit before reference timestamp"
		if !before {
			reason = "only matching commit found after reference timestamp"
		}
		sources[region] = versionSource{Repository: "AzurLaneTools/AzurLaneLuaScripts", Commit: selected.Commit, Path: "versions/" + region + ".txt", Version: selected.Values[region], CommitDate: selected.Date, BeforeReference: before, SelectionReason: reason}
	}
	return writeJSON(filepath.Join(out, "version-source-map.json"), sources)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func jsonShape(path string) (string, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return "json_decode_failure", 0
	}
	switch typed := value.(type) {
	case []any:
		return "array", len(typed)
	case map[string]any:
		return "object", len(typed)
	default:
		return fmt.Sprintf("%T", value), 1
	}
}

func writeMismatchRootCauses(out, referenceRoot, generatedRoot string) error {
	categoryFile, err := os.Open(filepath.Join(out, "mismatch-summary.csv"))
	if err != nil {
		return err
	}
	categoryRows, err := csv.NewReader(categoryFile).ReadAll()
	_ = categoryFile.Close()
	if err != nil {
		return err
	}
	semanticFile, err := os.Open(filepath.Join(out, "mismatch-semantic-summary.csv"))
	if err != nil {
		return err
	}
	semanticRows, err := csv.NewReader(semanticFile).ReadAll()
	_ = semanticFile.Close()
	if err != nil {
		return err
	}
	semantic := map[string]semanticRow{}
	for i, row := range semanticRows {
		if i == 0 || len(row) < 7 {
			continue
		}
		semantic[row[0]] = semanticRow{Classification: row[1], FirstPath: row[2], ReferenceType: row[4], GeneratedType: row[5]}
	}
	primary := map[string][3]string{
		"byte-only/escaping": {"semantic_equal_byte_different", "byte_formatting", "preserve exact serializer bytes"},
		"object key order":   {"object_key_order", "semantic_equal_byte_different", "derive category order rule"},
		"record count":       {"snapshot_difference", "record_order", "investigate source snapshot before parser changes"},
		"field":              {"missing_field", "extra_field", "compare structural fields"},
		"type":               {"nested_table_shape", "nil_or_empty", "infer shape by structural path"},
	}
	file, err := os.Create(filepath.Join(out, "mismatch-root-causes.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	_ = w.Write([]string{"path", "primary_cause", "secondary_cause", "first_json_path", "reference_type", "generated_type", "reference_count", "generated_count", "fix_strategy"})
	for i, row := range categoryRows {
		if i == 0 || len(row) < 3 {
			continue
		}
		cause, ok := primary[row[0]]
		if !ok {
			cause = [3]string{"unknown", "unknown", "hold for investigation"}
		}
		for _, path := range strings.Split(row[2], ";") {
			if path == "" {
				continue
			}
			refBytes, refErr := os.ReadFile(filepath.Join(referenceRoot, filepath.FromSlash(path)))
			genBytes, genErr := os.ReadFile(filepath.Join(generatedRoot, filepath.FromSlash(path)))
			if refErr == nil && genErr == nil && bytes.Equal(refBytes, genBytes) {
				continue
			}
			s := semantic[path]
			refType, refCount := jsonShape(filepath.Join(referenceRoot, filepath.FromSlash(path)))
			genType, genCount := jsonShape(filepath.Join(generatedRoot, filepath.FromSlash(path)))
			if s.ReferenceType != "" && s.ReferenceType != "-" {
				refType = s.ReferenceType
			}
			if s.GeneratedType != "" && s.GeneratedType != "-" {
				genType = s.GeneratedType
			}
			_ = w.Write([]string{path, cause[0], cause[1], s.FirstPath, refType, genType, fmt.Sprintf("%d", refCount), fmt.Sprintf("%d", genCount), cause[2]})
		}
	}
	w.Flush()
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	return w.Error()
}

func writeResidualDossier(out, referenceRoot, generatedRoot, referenceCommit string) error {
	rows := []map[string]string{}
	rootCauseFile, err := os.Open(filepath.Join(out, "mismatch-root-causes.csv"))
	if err != nil {
		return err
	}
	rootCauseRows, err := csv.NewReader(rootCauseFile).ReadAll()
	_ = rootCauseFile.Close()
	if err != nil {
		return err
	}
	for i, row := range rootCauseRows {
		if i == 0 || len(row) < 9 {
			continue
		}
		rows = append(rows, map[string]string{"path": row[0], "status": "mismatched", "root_cause": row[1], "first_json_path": row[3], "reference_type": row[4], "generated_type": row[5], "reference_value_summary": "", "generated_value_summary": "", "reference_record_count": row[6], "generated_record_count": row[7], "reference_only_ids": "", "generated_only_ids": "", "candidate_lua_files": "", "candidate_table_names": "", "source_commit": "", "reference_commit": referenceCommit, "recommended_action": row[8]})
	}
	missingFile, err := os.Open(filepath.Join(out, "missing-provenance.csv"))
	if err != nil {
		return err
	}
	missingRows, err := csv.NewReader(missingFile).ReadAll()
	_ = missingFile.Close()
	if err != nil {
		return err
	}
	for i, row := range missingRows {
		if i == 0 || len(row) < 8 {
			continue
		}
		rows = append(rows, map[string]string{"path": row[0], "status": "missing", "root_cause": row[4], "first_json_path": "", "reference_type": "", "generated_type": "", "reference_value_summary": "", "generated_value_summary": "", "reference_record_count": "", "generated_record_count": "", "reference_only_ids": "", "generated_only_ids": "", "candidate_lua_files": row[2], "candidate_table_names": row[3], "source_commit": "", "reference_commit": referenceCommit, "recommended_action": row[7]})
	}
	// The dossier is derived from the current comparison, so its size may only
	// shrink as residuals are fixed. The phase summary remains the authoritative
	// completion gate and still fails while mismatches or missing files remain.
	fields := []string{"path", "status", "root_cause", "first_json_path", "reference_type", "generated_type", "reference_value_summary", "generated_value_summary", "reference_record_count", "generated_record_count", "reference_only_ids", "generated_only_ids", "candidate_lua_files", "candidate_table_names", "source_commit", "reference_commit", "recommended_action"}
	file, err := os.Create(filepath.Join(out, "residual-dossier.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	_ = w.Write(fields)
	for _, row := range rows {
		values := make([]string, len(fields))
		for i, field := range fields {
			values[i] = row[field]
		}
		_ = w.Write(values)
	}
	w.Flush()
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	if err := w.Error(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(out, "residual-dossier.json"), rows)
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	ref := fs.String("reference-root", "", "JP reference root")
	gen := fs.String("generated-root", "", "generated JP root")
	refHelpers := fs.String("helper-reference-root", "", "reference belfast-data root")
	genRoot := fs.String("generated-output-root", "", "generated output root")
	luaRoot := fs.String("lua-repository-root", "_external/AzurLaneLuaScripts", "AzurLaneLuaScripts git checkout")
	referenceCommit := fs.String("reference-commit", "33c61c5c239e267e77573d945bfcca691114d60f", "belfast-data reference commit")
	out := fs.String("output-root", "reports/golden-compatibility", "report output directory")
	fs.Parse(os.Args[1:])
	if *ref == "" || *gen == "" || *refHelpers == "" || *genRoot == "" {
		panic("reference-root, generated-root, helper-reference-root, and generated-output-root are required")
	}

	luaRef, err := fileMap(*ref)
	if err != nil {
		panic(err)
	}
	luaGen, err := fileMap(*gen)
	if err != nil {
		panic(err)
	}
	lua := compare(luaRef, luaGen)

	helpers := []helper{
		{Path: "global/build_pools.json", SourceKind: "static_belfast_helper", SourcePath: "build_pools.json", GenerationMethod: "copy from belfast-data"},
		{Path: "global/build_times.json", SourceKind: "static_belfast_helper", SourcePath: "build_times.json", GenerationMethod: "copy from belfast-data"},
		{Path: "global/requisition_ships.json", SourceKind: "static_belfast_helper", SourcePath: "requisition_ships.json", GenerationMethod: "copy from belfast-data"},
		{Path: "global/versions.json", SourceKind: "version_text_generated", SourcePath: "versions/{CN,EN,JP,KR,TW}.txt", GenerationMethod: "versions/*.txt -> JSON"},
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	if err := writeManifest(*out, luaRef, luaGen, helpers, *refHelpers, *genRoot); err != nil {
		panic(err)
	}
	if err := versionCandidates(*luaRoot, *refHelpers, *referenceCommit, *out); err != nil {
		fmt.Fprintf(os.Stderr, "version candidate scan skipped: %v\n", err)
	}
	if err := writeMismatchRootCauses(*out, *ref, *gen); err != nil {
		fmt.Fprintf(os.Stderr, "mismatch root cause report skipped: %v\n", err)
	}
	if err := writeResidualDossier(*out, *refHelpers, *genRoot, *referenceCommit); err != nil {
		fmt.Fprintf(os.Stderr, "residual dossier skipped: %v\n", err)
	}
	file, err := os.Create(filepath.Join(*out, "helper-files.csv"))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	w := csv.NewWriter(file)
	_ = w.Write([]string{"path", "source_kind", "source_path", "generation_method", "reference_sha256", "generated_sha256", "match"})
	hc := counts{Total: len(helpers)}
	for _, h := range helpers {
		rp := filepath.Join(*refHelpers, filepath.FromSlash(h.Path[7:]))
		gp := filepath.Join(*genRoot, filepath.FromSlash(h.Path))
		rh, rb, rerr := digest(rp)
		gh, gb, gerr := digest(gp)
		match := rerr == nil && gerr == nil && rh == gh && string(rb) == string(gb)
		if match {
			hc.Matched++
		} else {
			hc.Mismatched++
		}
		_ = w.Write([]string{h.Path, h.SourceKind, h.SourcePath, h.GenerationMethod, rh, gh, fmt.Sprintf("%t", match)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		panic(err)
	}

	result := summary{LuaDerived: lua, BelfastHelpers: hc, AllOutputs: add(lua, hc)}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	b = append(b, '\r', '\n')
	if err := os.WriteFile(filepath.Join(*out, "phase1-summary.json"), b, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("lua_derived=%+v belfast_helpers=%+v all_outputs=%+v\n", lua, hc, result.AllOutputs)
	if lua.Mismatched != 0 || lua.Missing != 0 || lua.Extra != 0 || hc.Matched != 4 || hc.Mismatched != 0 || hc.Missing != 0 || hc.Extra != 0 {
		os.Exit(1)
	}
}
