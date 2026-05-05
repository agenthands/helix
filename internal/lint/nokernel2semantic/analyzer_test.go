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
