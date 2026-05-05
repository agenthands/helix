package nosemantic2kernel_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/nosemantic2kernel"
)

// TestAnalyzer_RejectsLspenrichImportingKernel asserts the analyzer fires
// when a package whose import path is rooted under
// `internal/semantic/lspenrich/` imports `internal/kernel` (no carve-out
// match).
func TestAnalyzer_RejectsLspenrichImportingKernel(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/lspenrich/badpkg")
}

// TestAnalyzer_AllowsLspenrichImportingLspool asserts the analyzer stays
// silent on a lspenrich-prefixed package that imports
// `internal/kernel/lspool` (the carve-out allow-list path; ENRICH-01).
func TestAnalyzer_AllowsLspenrichImportingLspool(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/lspenrich/lspoolok")
}

// TestAnalyzer_AllowsLspenrichImportingWorkspace asserts the analyzer stays
// silent on a lspenrich-prefixed package that imports `internal/workspace` —
// that path is outside the forbidden prefix (`internal/kernel/...`) entirely.
func TestAnalyzer_AllowsLspenrichImportingWorkspace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/lspenrich/wsok")
}

// TestAnalyzer_IgnoresOutOfScopeKernelPackage asserts the analyzer stays
// silent on a package outside the checked-package prefix (here, an
// `internal/kernel/...` package itself). The analyzer's scope-gate is the
// importing package's path, NOT the imported path.
func TestAnalyzer_IgnoresOutOfScopeKernelPackage(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/kernel/outofscope")
}

// TestAnalyzer_IgnoresSiblingSemanticPackage asserts the analyzer stays
// silent on a semantic-rooted package that is OUTSIDE the narrowed
// `internal/semantic/lspenrich/` prefix. The narrow scope is load-bearing:
// established Phase 60 packages like internal/semantic/live/service
// implement kernel.EditNotifier and intentionally import internal/kernel.
// Broadening the analyzer's checkedPkgPrefix to the whole semantic tree
// would falsely flag those.
func TestAnalyzer_IgnoresSiblingSemanticPackage(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nosemantic2kernel.Analyzer,
		"github.com/agenthands/helix/internal/semantic/siblingok")
}
