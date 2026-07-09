package belfastconv

import (
	"fmt"
	"os"
	"path/filepath"
)

func copyFallbackHelpers(sourceRoot, outputRoot string, report *Report) error {
	for _, rel := range fallbackHelperFiles {
		src := filepath.Join(sourceRoot, filepath.Base(filepath.FromSlash(rel)))
		dst := filepath.Join(outputRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read fallback helper %s: %w", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write fallback helper %s: %w", rel, err)
		}
		report.FallbackHelperFiles = append(report.FallbackHelperFiles, rel)
	}
	return nil
}
