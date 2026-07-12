package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type fileResult struct {
	Path            string `json:"path"`
	ReferenceSHA256 string `json:"reference_sha256,omitempty"`
	GeneratedSHA256 string `json:"generated_sha256,omitempty"`
	ReferenceSize   int    `json:"reference_size,omitempty"`
	GeneratedSize   int    `json:"generated_size,omitempty"`
	Match           bool   `json:"match"`
	ByteOffset      int    `json:"byte_offset,omitempty"`
}
type report struct {
	ReferenceRoot   string       `json:"reference_root"`
	GeneratedRoot   string       `json:"generated_root"`
	TotalFiles      int          `json:"total_files"`
	MatchedFiles    int          `json:"matched_files"`
	MismatchedFiles int          `json:"mismatched_files"`
	MissingFiles    []string     `json:"missing_files"`
	ExtraFiles      []string     `json:"extra_files"`
	Files           []fileResult `json:"files"`
}

func files(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = path
		return nil
	})
	return out, err
}

func filesFromManifest(path, root string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for i, name := range header {
		columns[name] = i
	}
	result := map[string]string{}
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if record[columns["classification"]] != "lua_derived" || record[columns["comparison_enabled"]] != "true" {
			continue
		}
		path := record[columns["path"]]
		const prefix = "JP/"
		if len(path) < len(prefix) || path[:len(prefix)] != prefix {
			continue
		}
		rel := path[len(prefix):]
		candidate := filepath.Join(root, filepath.FromSlash(rel))
		if _, statErr := os.Stat(candidate); statErr == nil {
			result[rel] = candidate
		}
	}
	return result, nil
}
func hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	ref := fs.String("reference-root", "", "golden reference root")
	gen := fs.String("generated-root", "", "generated root")
	out := fs.String("report", "", "report path")
	manifest := fs.String("manifest", "", "fixed comparison manifest CSV")
	fs.Parse(os.Args[1:])
	if *ref == "" || *gen == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "reference-root, generated-root, and report are required")
		os.Exit(2)
	}
	var err error
	r := report{ReferenceRoot: *ref, GeneratedRoot: *gen, MissingFiles: []string{}, ExtraFiles: []string{}, Files: []fileResult{}}
	var a map[string]string
	var b map[string]string
	if *manifest != "" {
		a, err = filesFromManifest(*manifest, *ref)
		if err != nil {
			panic(err)
		}
		b, err = filesFromManifest(*manifest, *gen)
	} else {
		a, err = files(*ref)
		if err != nil {
			panic(err)
		}
		b, err = files(*gen)
	}
	if err != nil {
		panic(err)
	}
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	all := make([]string, 0, len(keys))
	for k := range keys {
		all = append(all, k)
	}
	sort.Strings(all)
	for _, rel := range all {
		ap, aok := a[rel]
		bp, bok := b[rel]
		if !aok {
			r.ExtraFiles = append(r.ExtraFiles, rel)
			continue
		}
		if !bok {
			r.MissingFiles = append(r.MissingFiles, rel)
			continue
		}
		aa, _ := os.ReadFile(ap)
		bb, _ := os.ReadFile(bp)
		fr := fileResult{Path: rel, ReferenceSHA256: hash(aa), GeneratedSHA256: hash(bb), ReferenceSize: len(aa), GeneratedSize: len(bb), Match: string(aa) == string(bb)}
		if !fr.Match {
			fr.ByteOffset = firstDiff(aa, bb)
		} else {
			r.MatchedFiles++
		}
		r.Files = append(r.Files, fr)
	}
	r.TotalFiles = len(r.Files) + len(r.MissingFiles) + len(r.ExtraFiles)
	r.MismatchedFiles = r.TotalFiles - r.MatchedFiles - len(r.MissingFiles) - len(r.ExtraFiles)
	sort.Strings(r.MissingFiles)
	sort.Strings(r.ExtraFiles)
	data, _ := json.MarshalIndent(r, "", "  ")
	data = append(data, '\r', '\n')
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, data, 0644); err != nil {
		panic(err)
	}
	if r.MismatchedFiles > 0 || len(r.MissingFiles) > 0 || len(r.ExtraFiles) > 0 {
		fmt.Printf("matched=%d mismatched=%d missing=%d extra=%d\n", r.MatchedFiles, r.MismatchedFiles, len(r.MissingFiles), len(r.ExtraFiles))
		os.Exit(1)
	}
	fmt.Printf("matched=%d\n", r.MatchedFiles)
}
