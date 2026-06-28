package belfastconv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var regionNames = []string{"CN", "EN", "JP", "KR", "TW"}

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
