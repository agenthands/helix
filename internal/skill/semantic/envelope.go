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
// get_semantic_graph_status. Production adapter returns
// {State: "unknown", Reason: "phase-62-clustering-no-status-accessor"} until
// Phase 65/67 wires a live source (closes checker W1). The Reason field gives
// observability the gap-tracking signal it needs.
type ClusterStatus struct {
	// State is the closed-enum cluster status:
	// "unknown" | "current" | "stale" | "building".
	State string `json:"state"`
	// Reason is a free-form human reason; only populated when State !=
	// "current".
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
type StatusResult struct {
	CommonEnvelope
	LatestSnapshotID uint64            `json:"latest_snapshot_id"`
	ScoreStatus      map[string]string `json:"score_status"` // projection -> status string
	ClusterStatus    ClusterStatus     `json:"cluster_status"`
	LastLiveUpdateMs int64             `json:"last_live_update_ms"`
	PendingLSPFiles  int               `json:"pending_lsp_files"`
	RetrievalPending bool              `json:"retrieval_pending"`
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
