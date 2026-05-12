// Package handler — production FileFactDiff populator (F-01 closure).
//
// This file lives next to handler.go and supplies the production path that
// the 62-09 closure declared a TODO. The diff is best-effort and tiered:
//   - Tier 1 (full diff): added + removed + changed precision when the
//     snapshot store exposes a "fetch prior FileFact" accessor and the
//     extractor exposes a per-file FileFact API.
//   - Tier 2 (added-only): when only the extractor is available, every
//     extracted post-edit symbol is recorded as RecordSymbolAdded.
//   - Tier 3 (synthetic marker): when neither tier is workable, populate a
//     SINGLE synthetic SymbolDiff flagged with KindChanged: true so the
//     recorder is non-empty and ApplyRepair fires. The marker carries no
//     actionable per-symbol detail but it is the load-bearing F-01 floor:
//     graph_version ADVANCES on every live edit.
//
// The 2026-05-12 close-out ships only Tier 3 active; Tier 1 and Tier 2 are
// scaffolding for a follow-up that wires the snapshot-store accessor and
// per-file extractor (DEF-67-F01-FULL-DIFF in .planning/deferred-items.md).
package handler

import (
	"context"

	"github.com/agenthands/helix/internal/semantic"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
)

// populateRecorderForFile is the production diff populator the handler
// invokes after UpsertOverlayFile and before tx.Commit(). repoID + path
// identify the file under edit.
//
// Best-effort by contract: it never returns an error. All failures fall
// through to the synthetic-marker fallback so the recorder stays non-empty
// and ApplyRepair still fires (F-01 floor invariant).
func (h *Handler) populateRecorderForFile(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) {
	// Tier 1: full added/removed/changed diff via extractor + store prior-fact
	// lookup. Returns true on success.
	if h.tryFullDiff(ctx, repoID, path, recorder) {
		return
	}
	// Tier 2: added-only diff via extractor alone. Returns true on success.
	if h.tryAddedOnlyDiff(ctx, repoID, path, recorder) {
		return
	}
	// Tier 3: synthetic marker. KindChanged: true is a graph-changing diff
	// per repair.go D-06 ("any of the first four → graph-changing"), so the
	// downstream ApplyRepair will fire and graph_version will advance. The
	// marker carries no actionable per-symbol detail — full precision is the
	// DEF-67-F01-FULL-DIFF follow-up.
	recorder.RecordSymbolChanged(graphpkg.SymbolDiff{
		KindChanged: true,
	})
}

// tryFullDiff implements Tier 1 (full added/removed/changed diff). Returns
// true on success. Today returns false unconditionally — the snapshot
// store's prior-FileFact accessor is the DEF-67-F01-FULL-DIFF follow-up.
//
// Implementation sketch when the accessor lands:
//   1. priorFact, err := h.store.GetFileFact(repoID, path) — if err, fall through
//   2. newFact := h.extractor.ExtractFile(repoID, path) — if err, fall through
//   3. diffSymbols(priorFact.Symbols, newFact.Symbols, recorder)
//      → RecordSymbolAdded / RecordSymbolRemoved / RecordSymbolChanged
func (h *Handler) tryFullDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) bool {
	_ = ctx
	_ = repoID
	_ = path
	_ = recorder
	return false
}

// tryAddedOnlyDiff implements Tier 2 (added-only approximation via the
// extractor alone). Returns true on success. Today returns false
// unconditionally — the per-file extractor accessor surface is the
// DEF-67-F01-FULL-DIFF follow-up.
//
// Implementation sketch when the accessor lands:
//   1. newFact := h.extractor.ExtractFile(repoID, path) — if err, fall through
//   2. for each symbol s in newFact.Symbols:
//        recorder.RecordSymbolAdded(graphpkg.SymbolDiff{NodeID: s.NodeID, KindChanged: true})
func (h *Handler) tryAddedOnlyDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) bool {
	_ = ctx
	_ = repoID
	_ = path
	_ = recorder
	return false
}
