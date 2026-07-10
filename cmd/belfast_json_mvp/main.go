package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Fokiiiiiii/amagi-data/internal/belfastconv"
)

func main() {
	sourceRoot := flag.String("source-root", "", "AzurLaneData source root")
	outputRoot := flag.String("output-root", filepath.Join(os.TempDir(), "amagi_belfast_json_mvp"), "output root")
	luaScriptsRoot := flag.String("luascripts-root", "", "AzurLaneLuaScripts root")
	reportPath := flag.String("report-path", "", "report path")
	flag.Parse()

	report, err := belfastconv.ConvertMVP(belfastconv.Options{
		SourceRoot:     *sourceRoot,
		OutputRoot:     *outputRoot,
		ReportPath:     *reportPath,
		LuaScriptsRoot: *luaScriptsRoot,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("converted=%d generated_helpers=%d\n", len(report.ConvertedFiles), len(report.GeneratedHelperFiles))
}
