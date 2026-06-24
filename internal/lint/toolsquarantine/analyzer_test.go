package toolsquarantine_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/toolsquarantine"
)

// TestAnalyzer_RejectsRuntimeImportingTools is the RED fixture assertion: a
// package whose import path is NOT rooted under the tools/ prefix
// (internal/leakyruntime) that imports github.com/agenthands/helix/tools/dspytune
// MUST be flagged. The leakyruntime fixture carries the matching `// want`
// directive; deleting it makes this test FAIL (the non-vacuity proof).
func TestAnalyzer_RejectsRuntimeImportingTools(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/leakyruntime")
}

// TestAnalyzer_AllowsCleanRuntime is the GREEN fixture assertion: a runtime
// package importing only stdlib (internal/goodruntime importing os/exec) MUST
// produce NO diagnostic. No `// want` directive, so analysistest fails on any
// spurious report.
func TestAnalyzer_AllowsCleanRuntime(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/goodruntime")
}

// TestAnalyzer_AllowsSlashBoundaryLookalike is the slash-boundary regression
// guard: internal/toolsupport shares the bare substring "tools" with the
// forbidden prefix but lacks the slash boundary and imports nothing under
// tools/. The analyzer MUST stay silent — proving exact-OR-slash matching, not
// bare HasPrefix.
func TestAnalyzer_AllowsSlashBoundaryLookalike(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/toolsupport")
}
