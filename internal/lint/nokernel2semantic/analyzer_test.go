package nokernel2semantic_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/nokernel2semantic"
)

// TestAnalyzer_RejectsKernelImportingSemantic asserts the analyzer fires when
// a package whose import path is rooted under `internal/kernel/` imports a
// package rooted under `internal/semantic/`.
func TestAnalyzer_RejectsKernelImportingSemantic(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
		"github.com/agenthands/helix/internal/kernel/badpkg")
}

// TestAnalyzer_AllowsKernelWithoutSemanticImport asserts the analyzer stays
// silent on a kernel-prefixed package that does NOT import semantic.
func TestAnalyzer_AllowsKernelWithoutSemanticImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
		"github.com/agenthands/helix/internal/kernel/goodpkg")
}

// TestAnalyzer_AllowsKernelImportingSemanticInteg asserts that the Phase 65
// wave-0 allowlist is honored: a kernel-prefixed package importing the
// explicitly allowlisted `internal/semantic/integ` types-only seam reports
// ZERO diagnostics. (RESEARCH.md Common Pitfalls §1; M-vet.)
func TestAnalyzer_AllowsKernelImportingSemanticInteg(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
		"github.com/agenthands/helix/internal/kernel/integimport")
}

// TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike is the A6
// regression guard: an import of `internal/semantic/integ_evil` shares the
// bare-`HasPrefix` collision surface with the allowed `internal/semantic/
// integ` but lacks the slash boundary. The amended analyzer MUST still
// fire. Without this test, a future bare-`HasPrefix` regression would
// silently open an escape hatch.
func TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
		"github.com/agenthands/helix/internal/kernel/integlookalike")
}
