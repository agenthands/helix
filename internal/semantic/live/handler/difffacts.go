// Package handler — production FileFactDiff populator (F-01 closure +
// Phase 68 Plan 04: precise Tier-1 / Tier-2 populator + bounded Tier-3
// reason routing).
//
// Tiering (Phase 68 D-07):
//
//   - Tier 1 (full diff): precise added + removed + changed via
//     factStore.GetLatestFileFact + provider.ExtractFile (status=ready).
//   - Tier 2 (added-only): when extractor returns status=partial, every
//     extracted symbol surfaces as RecordSymbolAdded.
//   - Tier 3 (synthetic marker): when neither tier is workable, a SINGLE
//     synthetic SymbolDiff flagged with KindChanged: true keeps the
//     recorder non-empty so ApplyRepair fires and graph_version advances.
//     The bounded synthetic-reason metric (cold_start / extract_failed /
//     extract_unsupported) explains why.
//
// Pitfall 3 (load-bearing): `diffSymbols` compares Signature TEXT, not
// the body-mixing hash field. The Go provider mixes body bytes into its
// per-symbol hash (golang/provider.go:379), so it changes on every body-
// only edit; reading it here would falsely mark body-only edits as
// graph-changing and break D-06 (graph_version short-circuit). The hash
// field MUST NOT appear in the diff path; the negative invariant is
// enforced by Plan 68-04 acceptance criteria.
package handler

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	semstore "github.com/agenthands/helix/internal/semantic/store"
)

// Tier-3 bounded reason values (closed enum, mirrored by
// obs.Metrics.LiveFileFactDiffSyntheticReasonInc).
const (
	tier3ReasonColdStart   = "cold_start"
	tier3ReasonExtractFail = "extract_failed"
	tier3ReasonExtractUnsp = "extract_unsupported"
)

// populateRecorderForFile dispatches across the three tiers. Best-effort:
// never returns an error; the Tier-3 synthetic marker is the load-bearing
// floor that keeps the recorder non-empty so graph_version advances on
// every live edit (F-01 contract).
func (h *Handler) populateRecorderForFile(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) {
	var tier3Reason string
	if handled, reason := h.tryFullDiff(ctx, repoID, path, recorder); handled {
		return
	} else {
		tier3Reason = reason
	}
	if handled, reason := h.tryAddedOnlyDiff(ctx, repoID, path, recorder); handled {
		return
	} else if reason != "" {
		// extract_failed / extract_unsupported observed in Tier-2 wins
		// over cold_start that Tier-1 reported.
		tier3Reason = reason
	}
	if tier3Reason == "" {
		// Degenerate-path default — only reached when both try*'s reasons
		// were "" (provider absent on both sides; no prior fact); the
		// caller is effectively cold-start.
		tier3Reason = tier3ReasonColdStart
	}
	// Tier-3: bounded reason metric + synthetic marker keeps recorder
	// non-empty so ApplyRepair fires (F-01 floor invariant).
	if h.FileFactDiffMetrics != nil {
		h.FileFactDiffMetrics.LiveFileFactDiffSyntheticReasonInc(tier3Reason)
	}
	recorder.RecordSymbolChanged(graphpkg.SymbolDiff{KindChanged: true})
	h.emitFileFactDiffOutcome(repoID, "synthetic")
}

// tryFullDiff implements Tier 1 (full added/removed/changed diff).
//
// Returns (handled, tier3Reason):
//   - handled=true              → Tier-1 fired; tier3Reason is "".
//   - handled=false, reason=""              → no provider OR extractor
//     returned status=Partial (Tier-2 owns) OR extractor returned an
//     unexpected error (treated as fall-through so Tier-2 still gets a
//     chance).
//   - handled=false, reason="cold_start"        → prior fact missing.
//   - handled=false, reason="extract_failed"    → ExtractFile status=Failed.
//   - handled=false, reason="extract_unsupported" → ExtractFile status=Unsupported.
func (h *Handler) tryFullDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) (bool, string) {
	if h.factStore == nil {
		return false, ""
	}
	prior, ok, err := h.factStore.GetLatestFileFact(ctx, string(repoID), path)
	if err != nil {
		// Treat as missing prior fact (cold_start) — the I/O failure is
		// observable via the store's own error metric; from the diff's
		// perspective there is nothing to compare against.
		if h.Logger != nil {
			h.Logger.Warn("populator: GetLatestFileFact failed; falling through",
				"repo", repoID, "path", path, "err", err)
		}
		return false, tier3ReasonColdStart
	}
	if !ok {
		return false, tier3ReasonColdStart
	}
	if h.extractRegistry == nil {
		// No registry → cannot extract current state; treat as cold_start
		// (we have a prior but no way to compute the diff against current).
		return false, tier3ReasonColdStart
	}
	lang := langFromExt(path)
	provider, ok := h.extractRegistry.Provider(lang)
	if !ok {
		return false, tier3ReasonColdStart
	}
	ef, err := provider.ExtractFile(ctx, string(repoID), path)
	if err != nil || ef == nil {
		return false, tier3ReasonExtractFail
	}
	switch ef.File.ExtractionStatus {
	case extract.ExtractionStatusReady:
		// happy path — fall through
	case extract.ExtractionStatusPartial:
		// Tier-2 owns the partial path; do NOT report a Tier-3 reason
		// (Tier-2 will fire, taking Tier-3 off the table).
		return false, ""
	case extract.ExtractionStatusFailed:
		return false, tier3ReasonExtractFail
	case extract.ExtractionStatusUnsupported:
		return false, tier3ReasonExtractUnsp
	default:
		return false, tier3ReasonExtractFail
	}
	diffSymbols(prior.Symbols, ef.Symbols, recorder)
	h.emitFileFactDiffOutcome(repoID, "full")
	return true, ""
}

// tryAddedOnlyDiff implements Tier 2 (added-only approximation: every
// post-edit symbol is RecordSymbolAdded). Documented over-advance price
// per 68-RESEARCH.md Pitfall 4.
//
// Returns (handled, tier3Reason):
//   - handled=true              → Tier-2 fired; tier3Reason is "".
//   - handled=false, reason=""  → status=Ready (Tier-1 owns) OR no
//     provider.
//   - handled=false, reason="extract_failed"     → ExtractFile failed.
//   - handled=false, reason="extract_unsupported" → ExtractFile status=Unsupported.
func (h *Handler) tryAddedOnlyDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) (bool, string) {
	if h.extractRegistry == nil {
		return false, ""
	}
	lang := langFromExt(path)
	provider, ok := h.extractRegistry.Provider(lang)
	if !ok {
		return false, ""
	}
	ef, err := provider.ExtractFile(ctx, string(repoID), path)
	if err != nil || ef == nil {
		return false, tier3ReasonExtractFail
	}
	switch ef.File.ExtractionStatus {
	case extract.ExtractionStatusPartial:
		// happy path
	case extract.ExtractionStatusReady:
		// Tier-1 owns; do not double-emit.
		return false, ""
	case extract.ExtractionStatusFailed:
		return false, tier3ReasonExtractFail
	case extract.ExtractionStatusUnsupported:
		return false, tier3ReasonExtractUnsp
	default:
		return false, tier3ReasonExtractFail
	}
	for _, s := range ef.Symbols {
		recorder.RecordSymbolAdded(graphpkg.SymbolDiff{
			NodeID:      graphpkg.NodeID(s.ID),
			KindChanged: true,
		})
	}
	h.emitFileFactDiffOutcome(repoID, "added-only")
	return true, ""
}

// diffSymbols compares the prior []semstore.PriorSymbol against the
// current []extract.SymbolFact and records added / removed / changed
// SymbolDiff entries on recorder. Matching is by SymbolFact.ID
// (= semantic.SymbolID = NodeID per the stable-key derivation in
// Phase 59).
//
// PITFALL 3 GUARD (load-bearing): SignatureChanged is set from
// `p.Signature != n.Signature` — the human-readable signature TEXT —
// not from any body-mixing hash field. The Go provider's hash mixes
// function body bytes (golang/provider.go:379), so reading that hash
// here would mark body-only edits as graph-changing and break D-06
// (graph_version short-circuit). The negative invariant (no mention
// of the body-mixing hash field in this file) is enforced by Plan
// 68-04 acceptance criteria; `TestDiffSymbols_BodyOnlyNotGraphChanging`
// is the runtime regression test.
func diffSymbols(prior []semstore.PriorSymbol, curr []extract.SymbolFact, rec *FileFactDiffRecorder) {
	priorByID := make(map[semantic.SymbolID]semstore.PriorSymbol, len(prior))
	for _, s := range prior {
		priorByID[s.ID] = s
	}
	currByID := make(map[semantic.SymbolID]extract.SymbolFact, len(curr))
	for _, s := range curr {
		currByID[s.ID] = s
	}
	// removed: in prior, not in curr.
	for id := range priorByID {
		if _, still := currByID[id]; !still {
			rec.RecordSymbolRemoved(graphpkg.SymbolDiff{NodeID: graphpkg.NodeID(id)})
		}
	}
	// added: in curr, not in prior. KindChanged=true marks the added
	// symbol as graph-changing per D-06.
	for id := range currByID {
		if _, was := priorByID[id]; !was {
			rec.RecordSymbolAdded(graphpkg.SymbolDiff{
				NodeID:      graphpkg.NodeID(id),
				KindChanged: true,
			})
		}
	}
	// changed: present in both — compare Signature TEXT (not the body-
	// mixing hash), Visibility (mapped to ExportedChanged), Kind, StableKey.
	for id, n := range currByID {
		p, ok := priorByID[id]
		if !ok {
			continue
		}
		d := graphpkg.SymbolDiff{NodeID: graphpkg.NodeID(id)}
		if p.Signature != n.Signature {
			d.SignatureChanged = true
		}
		if p.Visibility != n.Visibility {
			d.ExportedChanged = true
		}
		if p.Kind != string(n.Kind) {
			d.KindChanged = true
		}
		// StableKey on the store side is the canonicalized string; on
		// the extract side it's the StableSymbolKey struct. Compare via
		// the canonical form (this matches what the store wrote).
		if p.StableKey != extract.CanonicalizeStableSymbolKey(n.StableKey) {
			d.StableKeyChanged = true
		}
		if d.SignatureChanged || d.ExportedChanged || d.KindChanged || d.StableKeyChanged {
			rec.RecordSymbolChanged(d)
		}
	}
}

// emitFileFactDiffOutcome bumps the {tier,repo} outcome counter. Nil-safe.
func (h *Handler) emitFileFactDiffOutcome(repoID semantic.RepoID, tier string) {
	if h.FileFactDiffMetrics == nil {
		return
	}
	h.FileFactDiffMetrics.LiveFileFactDiffInc(tier, string(repoID))
}

// langFromExt maps a file path to the canonical extract.Provider language
// identifier. Returns "" for unrecognized extensions; the caller treats
// "" as "no provider available" and falls through. Duplicated from
// internal/daemon/semantic_wiring.go:langFromExt to avoid pulling the
// daemon package into the handler import graph (vet-nokernel2semantic
// boundary). 4 cases, ~8 LOC — duplication is cheaper than a new shared
// internal/semantic/lang helper for this small surface.
func langFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	default:
		return ""
	}
}
