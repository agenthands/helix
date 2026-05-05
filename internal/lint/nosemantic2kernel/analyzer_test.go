package nosemantic2kernel_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/nosemantic2kernel"
)

// TestAnalyzer_RejectsSemanticImportingKernel asserts the analyzer fires when
// a package whose import path is rooted under `internal/semantic/` imports
// `internal/kernel` (no carve-out match).
func TestAnalyzer_RejectsSemanticImportingKernel(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/badpkg")
}

// TestAnalyzer_AllowsSemanticImportingLspool asserts the analyzer stays
// silent on a semantic-prefixed package that imports `internal/kernel/lspool`
// (the carve-out allow-list path; ENRICH-01).
func TestAnalyzer_AllowsSemanticImportingLspool(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/lspoolok")
}

// TestAnalyzer_AllowsSemanticImportingWorkspace asserts the analyzer stays
// silent on a semantic-prefixed package that imports `internal/workspace` —
// that path is outside the forbidden prefix (`internal/kernel/...`) entirely.
func TestAnalyzer_AllowsSemanticImportingWorkspace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/wsok")
}

// TestAnalyzer_IgnoresOutOfScopePackage asserts the analyzer stays silent on
// a package outside the checked-package prefix (here, an `internal/kernel/...`
// package itself). The analyzer's scope-gate is the importing package's path,
// NOT the imported path.
func TestAnalyzer_IgnoresOutOfScopePackage(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/kernel/outofscope")
}
