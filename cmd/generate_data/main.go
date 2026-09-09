package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Fokiiiiiii/amagi-data/internal/dataconv"
)

func main() {
	sourceRoot := flag.String("source-root", "", "repository/source root")
	outputRoot := flag.String("output-root", filepath.Join(os.TempDir(), "amagi_data_generation"), "output root")
	luaScriptsRoot := flag.String("luascripts-root", "", "AzurLaneLuaScripts root")
	fallbackRoot := flag.String("copy-helper-fallback-from", "", "existing data root for fallback helpers")
	versionSourceMap := flag.String("version-source-map", "", "region-specific versions source map")
	referenceRoot := flag.String("reference-root", "", "reference root used to select aggregate Lua records")
	legacyFallbackRoot := flag.String("legacy-fallback-root", "", "repository root containing the fixed legacy fallback files")
	reportPath := flag.String("report-path", "", "report path")
	incrementalPlanPath := flag.String("incremental-plan", "", "incremental conversion plan")
	flag.Parse()

	opts := dataconv.Options{
		SourceRoot:               *sourceRoot,
		OutputRoot:               *outputRoot,
		ReportPath:               *reportPath,
		LuaScriptsRoot:           *luaScriptsRoot,
		ReferenceRoot:            *referenceRoot,
		FallbackHelperSourceRoot: *fallbackRoot,
		VersionSourceMapPath:     *versionSourceMap,
		LegacyFallbackSourceRoot: *legacyFallbackRoot,
	}
	var report *dataconv.Report
	var err error
	if *incrementalPlanPath != "" {
		plan, planErr := dataconv.ReadIncrementalPlan(*incrementalPlanPath)
		if planErr != nil {
			err = planErr
		} else {
			report, err = dataconv.ConvertMVPIncremental(opts, plan)
		}
	} else {
		report, err = dataconv.ConvertMVP(opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := incompleteReportError(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("converted=%d generated_helpers=%d\n", len(report.ConvertedFiles), len(report.GeneratedHelperFiles))
}

func incompleteReportError(report *dataconv.Report) error {
	if report == nil {
		return errors.New("conversion produced no report")
	}
	problems := []string{}
	if len(report.MissingSourceFiles) > 0 {
		problems = append(problems, fmt.Sprintf("missing_source_files=%d", len(report.MissingSourceFiles)))
	}
	if len(report.UnsupportedFiles) > 0 {
		problems = append(problems, fmt.Sprintf("unsupported_files=%d", len(report.UnsupportedFiles)))
	}
	if len(report.UnsupportedHelperFiles) > 0 {
		problems = append(problems, fmt.Sprintf("unsupported_helper_files=%d", len(report.UnsupportedHelperFiles)))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("incomplete conversion: %s", strings.Join(problems, ", "))
}
