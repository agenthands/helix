// source.go — closed-enum types Source (D-04) and FallbackReason (D-05) plus
// the ClassifyLookupErr error→FallbackReason classifier.
//
// Each constant's wire-string is locked by source_test.go; renaming a value
// is a SPEC §24 envelope contract change and must be reviewed across every
// consumer.
package integ

import "errors"

// Source identifies which retrieval/ranking pipeline produced a tool's
// payload. SPEC §24 closed enum.
type Source string

const (
	// SourceSemantic — payload sourced from the Phase 60–64 semantic engine
	// (persisted graph scores, retrieval engine, validated edges).
	SourceSemantic Source = "semantic"
	// SourceTreeSitter — payload sourced from the v1.9 tree-sitter pipeline
	// because semantic_index is disabled by config (D-04).
	SourceTreeSitter Source = "tree_sitter"
	// SourceFallback — payload sourced from the v1.9 path because semantic
	// is enabled-but-unavailable (no snapshot yet, build in progress, error
	// state, bleve segment rebuilding). Pair with FallbackReason on the
	// envelope.
	SourceFallback Source = "fallback"
)

// FallbackReason explains why an envelope reports SourceFallback. Closed
// enum per D-05 / SPEC §24.
type FallbackReason string

const (
	// FallbackReasonIndexDisabled — config flag is off. Defensive value:
	// per Pitfall §3, in steady state the consumer's source-selection step
	// emits SourceTreeSitter (not SourceFallback) when config is off, so
	// this value should never appear on the wire. ClassifyLookupErr never
	// returns this value.
	FallbackReasonIndexDisabled FallbackReason = "index_disabled"
	// FallbackReasonNoSnapshotYet — semantic enabled, no committed snapshot
	// exists yet. D-06: cold start does NOT trigger background indexing;
	// agents must call index_semantic_graph explicitly.
	FallbackReasonNoSnapshotYet FallbackReason = "no_snapshot_yet"
	// FallbackReasonIndexBuilding — a build is currently running for this
	// workspace; the lookup returns ErrIndexBuilding while it is in flight.
	FallbackReasonIndexBuilding FallbackReason = "index_building"
	// FallbackReasonIndexError — the most recent build or live update
	// errored; the snapshot may be stale or absent.
	FallbackReasonIndexError FallbackReason = "index_error"
	// FallbackReasonBleveRebuilding — the bleve retrieval segment is being
	// rebuilt; text-anchored ranking is unavailable until the rebuild
	// completes.
	FallbackReasonBleveRebuilding FallbackReason = "bleve_rebuilding"
)

// Error sentinels for the lookup-error classifier. Wrapping is supported:
// the classifier uses errors.Is, so consumers can return
// fmt.Errorf("upstream: %w", integ.ErrNoSnapshot) and still classify
// correctly.
var (
	ErrNoSnapshot      = errors.New("integ: no committed snapshot yet")
	ErrIndexBuilding   = errors.New("integ: index build in progress")
	ErrIndexErrored    = errors.New("integ: index in error state")
	ErrBleveRebuilding = errors.New("integ: bleve segment rebuilding")
)

// ClassifyLookupErr maps a lookup error onto the closed FallbackReason enum.
//
// Doctrine (Pitfall §3): this classifier NEVER returns
// FallbackReasonIndexDisabled. The "config disabled" case is the consumer's
// source-selection responsibility — it emits SourceTreeSitter (D-04), not
// SourceFallback. Keeping that decision out of the classifier preserves the
// semantic distinction between "feature off" and "feature on but errored".
//
// Behavior:
//   - nil err → "" (empty FallbackReason; no fallback envelope is needed).
//   - errors.Is(err, ErrNoSnapshot)      → FallbackReasonNoSnapshotYet
//   - errors.Is(err, ErrIndexBuilding)   → FallbackReasonIndexBuilding
//   - errors.Is(err, ErrBleveRebuilding) → FallbackReasonBleveRebuilding
//   - errors.Is(err, ErrIndexErrored)    → FallbackReasonIndexError
//   - any other non-nil err              → FallbackReasonIndexError
//
// The classifier is the mandatory chokepoint between raw error text and the
// envelope (WR-NEW-01: never leak fmt.Sprintf("%v", err) into envelope JSON).
func ClassifyLookupErr(err error) FallbackReason {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrNoSnapshot):
		return FallbackReasonNoSnapshotYet
	case errors.Is(err, ErrIndexBuilding):
		return FallbackReasonIndexBuilding
	case errors.Is(err, ErrBleveRebuilding):
		return FallbackReasonBleveRebuilding
	case errors.Is(err, ErrIndexErrored):
		return FallbackReasonIndexError
	default:
		return FallbackReasonIndexError
	}
}
