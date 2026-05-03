package noduckdb_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/noduckdb"
)

func TestAnalyzer_RejectsImportFromBadpkg(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noduckdb.Analyzer, "badpkg")
}

func TestAnalyzer_AllowsImportFromGoodpkg(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noduckdb.Analyzer,
		"github.com/agenthands/helix/internal/semantic/store/goodpkg")
}
