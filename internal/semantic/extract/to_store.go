// ToStoreFacts is the locked Phase 65 unblock adapter (CONTEXT.md D-08,
// 2026-05-08 update): a pure deterministic function from per-language
// extracted facts ([]*ExtractedFile) to the store-shaped wire format
// (semanticstore.Facts).
//
// Phase 65's production buildFn at internal/daemon/semantic_wiring.go:687-742
// composes the full ingestion pipeline as:
//
//	walk → provider.Extract (D-06) → ToStoreFacts (D-08) → store.WriteSnapshotFacts
//
// Determinism contract (acceptance criterion #15): same input → byte-
// identical output across repeated same-process calls AND across repeated
// process invocations. The function does no I/O, holds no state, and
// iterates only over input slices in their natural order — there is no
// map iteration, no scheduler / store / runtime dependency, no time-now
// or rand source.
//
// Field-disposition contract (LOCKED — see 59-07-PLAN.md Task 2 for the
// full table). Every store-side field falls into exactly one of three
// buckets:
//
//  1. Sourced — mapped 1:1 from the extract-side counterpart (with
//     the necessary type cast: e.g. uint32 → int for line/column;
//     float32 → float64 for Confidence; string-typed enum → string).
//  2. Zero (INSERT-time) — left zero-value here; the store assigns at
//     WriteSnapshotFacts time (FileID, NodeID, RefID).
//  3. Zero (downstream / hard gap) — left zero-value here; an upstream
//     stage (Phase 60 LIVE-01 filesystem metadata, Phase 61 LSP
//     enrichment, Phase 62 type/scope resolver) populates later.
//     Notable hard gap: extract.Range is line/column-only (fact.go:71),
//     so StartByte / EndByte are unconditionally zero at this layer.
//
// Dropped-on-floor (intentional, documented):
//   - ExtractedFile.Imports — store.Facts has no imports column today.
//   - ExtractedFile.Types — Phase 62 territory; not yet consumed at
//     the store layer.
//   - ExtractedFile.Heritage — Phase 62 territory; ditto.
//
// When Phase 62's type resolver wants to consume these at the store
// layer, expand both store.Facts and this adapter together. Until
// then the drop is the simplest correct contract — pinned by
// TestToStoreFacts_DroppedOnFloor.
package extract

import (
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// ToStoreFacts converts a slice of per-file extraction outputs to the
// store-shaped wire format. Pure deterministic function; no I/O.
//
// Nil and empty inputs return a zero-value Facts. Nil entries within
// a non-empty slice are silently skipped (Phase 65 buildFn may push
// nils on extraction error paths, and absorbing them here keeps the
// upstream loop simple).
//
// See package doc for the full field-disposition contract.
func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts {
	if len(files) == 0 {
		return semanticstore.Facts{}
	}

	out := semanticstore.Facts{
		Files:      make([]semanticstore.FileFact, 0, len(files)),
		Symbols:    make([]semanticstore.SymbolFact, 0, 4*len(files)),
		References: make([]semanticstore.ReferenceFact, 0, 4*len(files)),
		// Edges left nil — Phase 62 territory, not in the P04 emit path.
	}

	for _, ef := range files {
		if ef == nil {
			// Defensive nil-skip — Phase 65 buildFn may push nils on
			// extraction error paths. Pinned by TestToStoreFacts_NilSafety.
			continue
		}
		out.Files = append(out.Files, fileFactToStore(ef.File))
		for _, s := range ef.Symbols {
			out.Symbols = append(out.Symbols, symbolFactToStore(s))
		}
		for _, r := range ef.References {
			out.References = append(out.References, referenceFactToStore(r))
		}
		// ef.Imports / ef.Types / ef.Heritage intentionally dropped —
		// see package doc comment.
	}

	return out
}

// fileFactToStore maps the extract per-file metadata to the store
// FileFact wire shape. Path + Language are the only fields sourced
// at this layer; the rest are populated downstream:
//
//   - FileID is assigned by store at WriteSnapshotFacts INSERT time.
//   - RepoID / ContentHash / SizeBytes / LineCount are populated by
//     Phase 60 LIVE-01 (filesystem metadata) and Phase 61 enrichment.
//   - Generated / Ignored / IgnoreReason may be set by Phase 65
//     buildFn upstream; not at the adapter layer.
//
// store.FileFact has no extraction_status column today; the partial
// extraction status flows through a separate column-map at
// WriteSnapshotFacts time. The adapter still emits a Files row for
// non-ready files (ROADMAP acceptance criterion #10 precondition) —
// pinned by TestToStoreFacts_PartialFileEmitsRow.
func fileFactToStore(f FileFact) semanticstore.FileFact {
	return semanticstore.FileFact{
		Path:     f.Path,
		Language: f.Language,
		// Other fields: zero-value; populated downstream.
	}
}

// symbolFactToStore maps the extract SymbolFact (in-memory shape per
// fact.go:84-104) to the store-side SymbolFact (wire shape per
// snapshot.go:144-171).
//
// Per the SPEC §11.2 confidence ladder, Confidence is widened from
// float32 → float64 with no truncation. Per SPEC §11.1 closed enum,
// Exported is derived from `Visibility == "exported"`.
//
// StableKey is canonicalized via CanonicalizeStableSymbolKey
// (stable_id.go:28) — the NUL-joined SPEC §11.1 form.
//
// Hard gap: StartByte / EndByte are unconditionally zero. extract.Range
// is line/column-only (fact.go:71-79); the adapter cannot synthesize
// byte offsets without re-reading the source file, which would break
// the no-I/O contract. Phase 60 / Phase 65 may backfill from filesystem
// data at a later layer.
func symbolFactToStore(s SymbolFact) semanticstore.SymbolFact {
	return semanticstore.SymbolFact{
		SymbolID:         uint64(s.ID),
		Language:         s.Language,
		Kind:             string(s.Kind),
		Name:             s.Name,
		QualifiedName:    s.QualifiedName,
		StableKey:        CanonicalizeStableSymbolKey(s.StableKey),
		StartLine:        int(s.Range.Start.Line),
		StartCol:         int(s.Range.Start.Column),
		EndLine:          int(s.Range.End.Line),
		EndCol:           int(s.Range.End.Column),
		Signature:        s.Signature,
		SignatureHash:    s.SignatureHash,
		Visibility:       s.Visibility,
		Exported:         s.Visibility == "exported",
		Confidence:       float64(s.Confidence),
		ExtractionSource: s.ExtractionSource,
		// NodeID / FileID assigned by store at INSERT time.
		// OwnerSymbolID / ParentScopeID / PackagePath populated by
		// Phase 62 type/scope resolver.
		// StartByte / EndByte: hard gap — extract.Range is line/column-only.
		// ContentHash / LSPIdentity populated by Phase 61 enrichment.
	}
}

// referenceFactToStore maps the extract ReferenceFact (fact.go:114-130)
// to the store-side ReferenceFact (snapshot.go:174-193).
//
// The extract field name is `Kind` (typed `ReferenceKind`); the store
// field name is `RefKind` (typed `string`). Cast to string is safe —
// ReferenceKind is a typed string per fact.go:28.
//
// Hard gap: StartByte / EndByte are unconditionally zero. Same
// rationale as symbolFactToStore (extract.Range is line/column-only).
func referenceFactToStore(r ReferenceFact) semanticstore.ReferenceFact {
	return semanticstore.ReferenceFact{
		Name:             r.Name,
		RefKind:          string(r.Kind),
		ReceiverText:     r.ReceiverText,
		StartLine:        int(r.Range.Start.Line),
		StartCol:         int(r.Range.Start.Column),
		EndLine:          int(r.Range.End.Line),
		EndCol:           int(r.Range.End.Column),
		ValidationState:  r.ValidationState,
		Confidence:       float64(r.Confidence),
		Reason:           r.Reason,
		ResolutionSource: r.ResolutionSource,
		// RefID / NodeID / FileID assigned by store at INSERT time.
		// ScopeSymbolID populated by Phase 62 scope-resolver from r.ContainerID.
		// ResolvedSymbolID populated by Phase 62 type-resolver from r.ResolvedTarget.
		// StartByte / EndByte: hard gap — extract.Range is line/column-only.
	}
}
