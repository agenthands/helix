package benchragleakage_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/benchragleakage"
)

// TestAnalyzer_RejectsBenchRagImportingKernel asserts the analyzer fires when a
// package rooted under cmd/helix-bench-rag imports internal/kernel.
func TestAnalyzer_RejectsBenchRagImportingKernel(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), benchragleakage.Analyzer,
		"github.com/agenthands/helix/cmd/helix-bench-rag/leaky")
}

// TestAnalyzer_AllowsBenchRagWithoutForbiddenImport asserts the analyzer stays
// silent on a cmd/helix-bench-rag package that imports only permitted packages.
func TestAnalyzer_AllowsBenchRagWithoutForbiddenImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), benchragleakage.Analyzer,
		"github.com/agenthands/helix/cmd/helix-bench-rag/clean")
}

// TestAnalyzer_AllowsSlashBoundaryLookalike is the slash-boundary regression
// guard: an import of internal/kernelextra shares the bare-HasPrefix collision
// surface with the forbidden internal/kernel prefix but lacks the slash
// boundary. The analyzer MUST stay silent.
func TestAnalyzer_AllowsSlashBoundaryLookalike(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), benchragleakage.Analyzer,
		"github.com/agenthands/helix/cmd/helix-bench-rag/sibling")
}
