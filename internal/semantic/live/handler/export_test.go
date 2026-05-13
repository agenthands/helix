package handler

import (
	"github.com/agenthands/helix/internal/semantic/extract"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/store"
)

// LastRecorderSnapshotForTest returns the most recent pre-Compute recorder
// snapshot the Handler captured during populateRecorderForFile. Plan 68-04
// Open Question Q4 test seam.
func LastRecorderSnapshotForTest(h *Handler) graphpkg.FileFactDiff {
	if h == nil {
		return graphpkg.FileFactDiff{}
	}
	return h.lastRecorderSnapshot
}

// DiffSymbolsForTest exposes the unexported diffSymbols package-internal
// helper for direct unit testing.
func DiffSymbolsForTest(prior []store.PriorSymbol, curr []extract.SymbolFact, rec *FileFactDiffRecorder) {
	diffSymbols(prior, curr, rec)
}

// SetPopulateRecorderForTest installs a recorder populator on h. The
// populator is invoked AFTER UpsertOverlayFile and BEFORE Commit, on the
// recorder threaded through the tx span. Production code paths leave the
// underlying field nil; this setter is the single seam handler_test.go
// uses to simulate Phase 60 P04 / future type-resolver retrofit
// populators without pre-implementing those phases.
//
// 62-09 closure (truth #22): the populator is the contract bridge between
// today's empty-diff production state and the future populators that will
// fill the recorder in real flow.
func SetPopulateRecorderForTest(h *Handler, fn func(*FileFactDiffRecorder)) {
	if h == nil {
		return
	}
	h.populateRecorderForTest = fn
}
