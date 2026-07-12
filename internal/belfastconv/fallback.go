package belfastconv

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const legacyFallbackSourceKind = "legacy_belfast_fallback"

var legacyFallbackFiles = map[string]string{
	"JP/ShareCfg/card_affix.json":    "34001e477e937596f68366b7b49cce737c0a8f054ee35a06faf5ff0e08f44752",
	"JP/ShareCfg/card_template.json": "b05230de60d36070989cb809f7a61c30086bf216bcc33a3129e440ce70e45ee6",
}

func LegacyFallbackFiles() []string {
	return []string{"JP/ShareCfg/card_affix.json", "JP/ShareCfg/card_template.json"}
}

func validateLegacyFallbackSources(sourceRoot string) error {
	for _, rel := range LegacyFallbackFiles() {
		path := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read legacy fallback source %s: %w", rel, err)
		}
		want := legacyFallbackFiles[rel]
		got := sha256Hex(data)
		if got != want {
			return fmt.Errorf("legacy fallback SHA-256 mismatch for %s: got %s want %s", rel, got, want)
		}
	}
	return nil
}

func copyLegacyFallback(opts Options, rel string, report *Report) (bool, error) {
	want, ok := legacyFallbackFiles[rel]
	if !ok || opts.LegacyFallbackSourceRoot == "" {
		return false, nil
	}
	sourcePath := filepath.Join(opts.LegacyFallbackSourceRoot, filepath.FromSlash(rel))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return true, fmt.Errorf("read legacy fallback source %s: %w", rel, err)
	}
	if got := sha256Hex(data); got != want {
		return true, fmt.Errorf("legacy fallback SHA-256 mismatch for %s: got %s want %s", rel, got, want)
	}
	destination := filepath.Join(opts.OutputRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return true, err
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return true, fmt.Errorf("write legacy fallback %s: %w", rel, err)
	}
	report.FallbackFiles = append(report.FallbackFiles, rel)
	report.FallbackFileReports = append(report.FallbackFileReports, FallbackFileReport{
		RelativePath: rel, SourceKind: legacyFallbackSourceKind, SourcePath: rel,
		ReferenceSHA256: want, GeneratedSHA256: sha256Hex(data), Match: true,
	})
	report.TotalFallbackCount++
	return true, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

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
