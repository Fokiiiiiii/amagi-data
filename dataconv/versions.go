package dataconv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var regionNames = []string{"CN", "EN", "JP", "KR", "TW"}

type generatedID int

type versionSourceMapEntry struct {
	Commit  string `json:"commit"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

func generateVersionsJSON(luaScriptsRoot string) (map[string]string, string, error) {
	versionsRoot, err := findVersionsRoot(luaScriptsRoot)
	if err != nil {
		return nil, "", err
	}
	out := make(map[string]string, len(regionNames))
	for _, region := range regionNames {
		version, err := readVersionValue(filepath.Join(versionsRoot, region+".txt"))
		if err != nil {
			return nil, "", err
		}
		out[region] = version
	}
	return out, versionsRoot, nil
}

func writeVersionsJSON(path string, versions map[string]string) error {
	order := []string{"TW", "KR", "CN", "JP", "EN"}
	var builder strings.Builder
	builder.WriteString("{\r\n")
	for i, region := range order {
		version, ok := versions[region]
		if !ok || version == "" {
			return fmt.Errorf("versions missing %s", region)
		}
		fmt.Fprintf(&builder, "    %q:%q", region, version)
		if i < len(order)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\r\n")
	}
	builder.WriteString("}\r\n")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(builder.String()), 0644)
}

func findVersionsRoot(luaScriptsRoot string) (string, error) {
	candidate := filepath.Join(luaScriptsRoot, "versions")
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		return candidate, nil
	}
	var found string
	err = filepath.WalkDir(luaScriptsRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && strings.EqualFold(filepath.Base(path), "versions") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("could not locate versions directory under %s", luaScriptsRoot)
	}
	return found, nil
}

func readVersionValue(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read version file %s: %w", path, err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("version file %s is empty", path)
	}
	return value, nil
}
