package daemon

// daemon_test_export_test.go exposes test-only accessors on *Daemon for the
// EXTRACT-05 dual regression test (Phase 59 P05 Task 2). These accessors
// are gated to the daemon test package by the _test.go suffix — production
// callers cannot reach them.

import (
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

// KernelForTest returns the daemon's kernel for the Phase 76 no_lsp wiring
// regression test (TestNoLSPWiring). Lets the test assert
// k.LSPSubsystemDisabled() and k.EditNotifier() directly.
func (d *Daemon) KernelForTest() *kernel.Kernel {
	return d.kernel
}

// SemanticExtractRegistryForTest returns the daemon's semantic extractor
// registry, or nil when cfg.SemanticIndex.Enabled was false at construction.
// EXTRACT-05 regression test seam.
func (d *Daemon) SemanticExtractRegistryForTest() *extract.Registry {
	return d.semanticExtractRegistry
}

// GrammarRegistryForTest returns the daemon-singleton tree-sitter grammar
// registry. The EXTRACT-05 invariant is "every consumer holds this exact
// pointer" — the test compares this pointer against the registry held by
// extract.Registry, BodyExtractor, and the repomap skill.
func (d *Daemon) GrammarRegistryForTest() *treesitter.GrammarRegistry {
	return d.grammarRegistry
}

// BodyExtractorForTest returns the daemon's body extractor for the
// EXTRACT-05 reflection-based pointer-equality assertion (BodyExtractor.registry
// is unexported; reflection is the only way without piercing the kernel/edit
// package boundary).
func (d *Daemon) BodyExtractorForTest() interface{} {
	return d.bodyExtractor
}

// semanticBundleForTest returns the daemon's semantic bundle, or nil when
// cfg.SemanticIndex.Enabled was false at construction. Phase 81 Plan 04 gate
// test seam: asserts the bundle is STILL built under the ablation gate
// (build-but-block, D-04) and provides a non-nil bundle to the gate helpers.
func (d *Daemon) semanticBundleForTest() *semanticBundle {
	return d.semantic
}

// effSemanticDisabledForTest exposes the composition-root gate decision for the
// Phase 81 Plan 07 (CR-01) background-pipeline gate test.
func (d *Daemon) effSemanticDisabledForTest() bool {
	return d.effSemanticDisabled
}

// fileFactStoreWiredForTest reports whether the live handler's FileFactStore
// read-driver is wired. Phase 81 Plan 07 (CR-01) seam: under the gate the
// SetFileFactStore call is skipped, so this is false even though the store +
// bundle stay built (build-but-block, D-04). Returns false when the live
// bundle / handler is absent.
func (d *Daemon) fileFactStoreWiredForTest() bool {
	if d.live == nil || d.live.handler == nil {
		return false
	}
	return d.live.handler.HasFactStore()
}
