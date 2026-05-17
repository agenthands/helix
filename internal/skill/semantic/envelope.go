package semantic

// This file declares the closed-enum types and shared response shapes used by
// all four semantic tools. Frozen at end-of-W0 (Phase 64-03); wave-2 plans
// only consume.
//
// Sources of truth (per .planning/phases/64-new-mcp-tools/64-CONTEXT.md):
//   - SPEC §23.1 — index_semantic_graph response shape
//   - SPEC §23.2 — refresh_semantic_graph response shape
//   - SPEC §23.3 — get_semantic_graph_status response shape
//   - SPEC §23.4 — get_semantic_context request + response shape
//   - SPEC §26.2 — Freshness closed enum

// Freshness is the closed enum for the freshness field across all four tools.
// Source: SPEC §26.2.
type Freshness string

const (
	// FreshnessFresh: latest committed snapshot is current AND no overlay
	// pending rows AND no LSP queue depth.
	FreshnessFresh Freshness = "fresh"
	// FreshnessStale: latest committed snapshot exists but is older than the
	// current graph_version.
	FreshnessStale Freshness = "stale"
	// FreshnessStructurallyFreshSemanticallyPending: structural overlay
	// updates have applied (graph_version bumped) but LSP enrichment is
	// still pending for some files.
	FreshnessStructurallyFreshSemanticallyPending Freshness = "structurally_fresh_semantically_pending"
	// FreshnessOverlayActive: overlay has uncommitted pending rows; the
	// effective graph diverges from the latest committed snapshot.
	FreshnessOverlayActive Freshness = "overlay_active"
)

// IndexStatus is the closed enum for the status field of index_semantic_graph
// responses. Source: CONTEXT.md "Claude's Discretion" + SPEC §23.1.
type IndexStatus string

const (
	// IndexStatusCommitted: a snapshot was successfully committed.
	IndexStatusCommitted IndexStatus = "committed"
	// IndexStatusBuilding: the index build is still running in the
	// background (returned with Partial=true on tool-call timeout, per D-04).
	IndexStatusBuilding IndexStatus = "building"
	// IndexStatusFailed: the index build failed before commit.
	IndexStatusFailed IndexStatus = "failed"
)

// FreshnessMode is the closed enum for the freshness_mode request input on
// get_semantic_context. Source: SPEC §23.4 input shape.
type FreshnessMode string

const (
	// FreshnessModeAllowStale: serve results regardless of overlay/LSP state.
	FreshnessModeAllowStale FreshnessMode = "allow_stale"
	// FreshnessModeRequireCurrent: only serve when overlay is empty AND LSP
	// queue is drained.
	FreshnessModeRequireCurrent FreshnessMode = "require_current"
	// FreshnessModeValidateLive: drain pending live updates before serving.
	FreshnessModeValidateLive FreshnessMode = "validate_live"
)

// ClusterStatus is the structured cluster_status surfaced by
// get_semantic_graph_status. Phase 69-04 grows the struct additively with
// ComputedAt + MemberCount; both are omitempty so the pre-Phase-69 JSON shape
// (state + optional reason) is preserved for callers that have not yet adopted
// the new fields. Phase 69-05 wires a real source via the new
// SchedulerAccessor.ClusterStatus path; Phase 69-06 wires a sql adapter.
type ClusterStatus struct {
	// State is the closed-enum cluster status:
	// "unknown" | "current" | "stale" | "building".
	//
	// NOTE: "building" is reserved for a future phase. Phase 69 emits only
	// "unknown" | "current" | "stale".
	State string `json:"state"`
	// Reason is a free-form human reason; only populated when State !=
	// "current".
	Reason string `json:"reason,omitempty"`
	// ComputedAt is the unix-millis timestamp when the cluster bucket was
	// materialized (sourced from semantic_clusters.computed_at; CONTEXT.md
	// D4). Omitted when zero.
	ComputedAt int64 `json:"computed_at,omitempty"`
	// MemberCount is the COUNT(*) of rows in semantic_cluster_members for
	// the bucket. Omitted when zero.
	MemberCount int `json:"member_count,omitempty"`
}

// RetrievalStatus is the structured retrieval_status nested inside
// StatusResult (Phase 69-04 STATUS-01). It exposes bleve corpus state +
// indexed cardinalities + last-compact timestamp for the get_semantic_graph_status
// envelope.
//
// Reason is a closed enum. When the retrieval engine is healthy the field is
// the empty string (omitted via omitempty); when degraded, Reason carries the
// highest-priority degradation cause, in this priority order (highest →
// lowest):
//
//  1. "bleve-unavailable"             — engine missing entirely (no bleve handle)
//  2. "corpus_version-uninitialized"  — Recoverer never ran successfully
//  3. "corpus_version-lag"            — bleve.corpus_version < store.CurrentGraphVersion
//  4. "compactor-never-ran"           — last_compact_at meta absent
//  5. ""                              — no degradation; all fields consistent
//
// Producers (Plan 69-05's RetrievalAccessor.RetrievalStatus implementation)
// MUST select the highest-priority reason that applies; lower-priority causes
// are not surfaced when a higher-priority one is active.
//
// Privacy: only cardinalities (file/symbol counts) and a monotonic version
// are reported. No file paths, no symbol identifiers (T-69-01 mitigation).
type RetrievalStatus struct {
	// CorpusVersion is bleve.corpus_version meta (monotonic uint64).
	CorpusVersion uint64 `json:"corpus_version"`
	// IndexedFiles is the count of distinct source files reflected in the
	// bleve corpus.
	IndexedFiles int64 `json:"indexed_files"`
	// IndexedSymbols is the count of symbols reflected in the bleve corpus.
	IndexedSymbols int64 `json:"indexed_symbols"`
	// LastCompactAt is the unix-millis timestamp of the most-recent
	// compactor flush, or zero if the compactor has never run.
	//
	// Note: zero is a meaningful "never-ran" signal (mapped to Reason
	// "compactor-never-ran"); tests assert non-zero rather than presence-only
	// (Pitfall 4 of RESEARCH.md).
	LastCompactAt int64 `json:"last_compact_at"`
	// Reason is the highest-priority degradation reason; see priority order
	// in the struct doc-comment. Omitted when empty.
	Reason string `json:"reason,omitempty"`
}

// CommonEnvelope is the shared response envelope embedded by every tool's
// response struct.
type CommonEnvelope struct {
	Freshness     Freshness `json:"freshness"`
	GraphVersion  uint64    `json:"graph_version"`
	OverlayActive bool      `json:"overlay_active"`
}

// IndexResult is the index_semantic_graph response (SPEC §23.1).
type IndexResult struct {
	CommonEnvelope
	SnapshotID   uint64      `json:"snapshot_id"`
	Status       IndexStatus `json:"status"`
	Partial      bool        `json:"partial"`
	FilesIndexed int64       `json:"files_indexed"`
	FilesReused  int64       `json:"files_reused"`
	DurationMs   int64       `json:"duration_ms"`
}

// RefreshResult is the refresh_semantic_graph response (SPEC §23.2).
type RefreshResult struct {
	CommonEnvelope
	FilesUpdated    int  `json:"files_updated"`
	PendingLSP      bool `json:"pending_lsp"`
	PendingLSPFiles int  `json:"pending_lsp_files"`
}

// StatusResult is the get_semantic_graph_status response (SPEC §23.3).
//
// Phase 69-04 (STATUS-01) adds the nested RetrievalStatus field. The
// top-level RetrievalPending bool is intentionally retained — Phase 64
// consumers (get_semantic_context handler chain) depend on the flat shape, so
// the new nested field is purely additive.
type StatusResult struct {
	CommonEnvelope
	LatestSnapshotID uint64            `json:"latest_snapshot_id"`
	ScoreStatus      map[string]string `json:"score_status"` // projection -> status string
	ClusterStatus    ClusterStatus     `json:"cluster_status"`
	LastLiveUpdateMs int64             `json:"last_live_update_ms"`
	PendingLSPFiles  int               `json:"pending_lsp_files"`
	// RetrievalPending is the Phase 64 retrieval-pending flag. KEPT AT TOP
	// LEVEL — do not move into RetrievalStatus.
	RetrievalPending bool `json:"retrieval_pending"`
	// RetrievalStatus is the Phase 69-04 nested retrieval state.
	RetrievalStatus RetrievalStatus `json:"retrieval_status"`
}

// ContextResult is the get_semantic_context response (SPEC §23.4).
type ContextResult struct {
	CommonEnvelope
	FreshnessMode    FreshnessMode      `json:"freshness_mode"`
	PendingLSPFiles  int                `json:"pending_lsp_files"`
	RetrievalPending bool               `json:"retrieval_pending"`
	Candidates       []ContextCandidate `json:"candidates"`
}

// ContextCandidate is one ranked candidate in a ContextResult.
type ContextCandidate struct {
	SymbolID   string          `json:"symbol_id"`
	Confidence float64         `json:"confidence"`
	Evidence   ContextEvidence `json:"evidence"`
}

// ContextEvidence is the per-candidate evidence surfaced to the agent.
type ContextEvidence struct {
	TextRank     int      `json:"text_rank"`
	GraphRank    int      `json:"graph_rank"`
	MatchedTerms []string `json:"matched_terms"`
	TopEdges     []string `json:"top_edges"` // capped at 5
}

// ----------------------------------------------------------------------------
// Phase 71-02 — FreshnessV2 envelope (D5) + FreshnessStatus / FreshnessSource
// closed enums. Strictly additive: the existing Phase 64 Freshness type,
// ClusterStatus, RetrievalStatus, and CommonEnvelope shapes are NOT touched.
//
// Lineage:
//   - D5 (71-CONTEXT.md) — three-identifier freshness envelope
//     (graph_version, snapshot_id, extractor_run_id) plus closed-enum status
//     and source fields.
//   - Phase 69 D1 — three-state status closed enum {current, stale, unknown}
//     reused verbatim here as FreshnessStatus (a NEW named type distinct
//     from the Phase 64 four-state Freshness type so the two contracts can
//     coexist without aliasing).
// ----------------------------------------------------------------------------

// FreshnessStatus is the closed three-state status enum embedded in
// FreshnessV2. Distinct named type from Freshness so the Phase 64 envelope
// contract is preserved unchanged.
type FreshnessStatus string

const (
	// FreshnessStatusCurrent: the read reflects the latest committed snapshot
	// with no pending overlay or LSP work.
	FreshnessStatusCurrent FreshnessStatus = "current"
	// FreshnessStatusStale: the read reflects a snapshot older than the
	// current graph version, OR overlay/LSP work is pending.
	FreshnessStatusStale FreshnessStatus = "stale"
	// FreshnessStatusUnknown: no committed snapshot is available yet (e.g.,
	// initial index build hasn't completed).
	FreshnessStatusUnknown FreshnessStatus = "unknown"
)

// FreshnessSource is the closed enum identifying which subsystem produced the
// result. Source: D5 — `source ∈ {graph, type_resolver_ladder, ast_fallback}`.
type FreshnessSource string

const (
	// FreshnessSourceGraph: result came from the committed semantic graph.
	FreshnessSourceGraph FreshnessSource = "graph"
	// FreshnessSourceTypeResolverLadder: result was derived via the type
	// resolver ladder (Phase 62 TYPES-01..04).
	FreshnessSourceTypeResolverLadder FreshnessSource = "type_resolver_ladder"
	// FreshnessSourceASTFallback: result came from tree-sitter AST fallback
	// because graph/LSP information was unavailable.
	FreshnessSourceASTFallback FreshnessSource = "ast_fallback"
)

// FreshnessV2 is the Phase 71 D5 freshness envelope carrying the three
// freshness identifiers (graph_version, snapshot_id, extractor_run_id), the
// server-clock as-of timestamp, and the closed-enum status + source fields.
//
// All fields are `omitempty` so degraded paths emitting a partial envelope
// (e.g., source==ast_fallback with no snapshot) do not surface zero-value
// keys — consistent with Pattern A in 71-PATTERNS.md (additive struct
// extension via omitempty).
type FreshnessV2 struct {
	GraphVersion   uint64          `json:"graph_version,omitempty"`
	SnapshotID     uint64          `json:"snapshot_id,omitempty"`
	ExtractorRunID string          `json:"extractor_run_id,omitempty"`
	AsOfUnixMs     int64           `json:"as_of_unix_ms,omitempty"`
	Status         FreshnessStatus `json:"status,omitempty"`
	Source         FreshnessSource `json:"source,omitempty"`
}

// TextRank is one text-rank entry returned by RetrievalAccessor.QueryBleve.
type TextRank struct {
	SymbolID string
	Score    float64
}

// GraphRank is one graph-rank entry returned by RetrievalAccessor.PersonalizedPageRank.
type GraphRank struct {
	SymbolID string
	Score    float64
}
