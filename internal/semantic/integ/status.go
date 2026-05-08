// status.go — value types crossing the SemanticLookup interface boundary.
//
// All types here are plain-data: pointers to *Store / *Engine / scheduler
// internals never appear in the integ package surface. The kernel-allowlist
// rule (M-vet) depends on this — kernel packages can import integ for the
// types without dragging duckdb-go or the retrieval engine along.
package integ

// SymbolID is the stable Phase 59 EXTRACT-02 symbol identity (LSP
// identity → package/module path + qualified name + kind + signature
// hash → file path fallback). Stored as a string at the integ boundary
// so kernel callers can pass it back through the lookup without a
// store-side type dependency.
type SymbolID string

// RankedFile is one entry in a ranked-file list returned by RankFiles or
// RankFromSeeds. Score is a per-projection PageRank value; consumers sort
// descending. GraphVersion is the (repo_id, projection)-stamped version
// the score was computed against — Phase 62 D-07 contract — and is
// monotonic per workspace.
type RankedFile struct {
	Path         string
	Score        float64
	Projection   string // "call_graph" | "reference" | "file_dependency"
	GraphVersion uint64
}

// Impact is one element of the blast-radius expansion frontier. Confidence
// follows the Phase 62 D-12 ladder; Refuted=true means a Pass 2 LSP
// validation actively contradicted the edge (confidence drops to 0.20).
type Impact struct {
	SymbolID   SymbolID
	EdgeKind   string
	Confidence float64
	Evidence   Evidence
	Refuted    bool
}

// Evidence carries per-impact provenance. Edges are the originating graph
// edges; Ranks are the per-step PageRank weights used to derive Confidence;
// LSPLocations are concrete file:line:col witnesses (only present after
// Pass 2 validation).
type Evidence struct {
	Edges        []Edge
	Ranks        []float64
	LSPLocations []LSPLocation
}

// Edge is a directed graph edge between two stable SymbolIDs. Confidence
// follows the Phase 62 D-12 ladder; Kind names the projection edge type
// ("calls" | "references" | "imports" | ...).
type Edge struct {
	From       SymbolID
	To         SymbolID
	Kind       string
	Confidence float64
}

// ValidatedEdge wraps an Edge with the post-Pass-2 LSP outcome. When
// LSPConfirmed=true the consumer flips Edge.Confidence to 1.00; when
// LSPConfirmed=false (and an LSP probe was attempted) the consumer sets
// Refuted=true on the Impact and drops Edge.Confidence to 0.20. Edges that
// do NOT meet the "critical edge" test (crosses public-API OR confidence
// < 0.80) are passed through unchanged with LSPConfirmed=false.
type ValidatedEdge struct {
	Edge         Edge
	LSPConfirmed bool
}

// LSPLocation pins a concrete witness for a refutation/confirmation. Line/
// column are 1-based, matching LSP's external surface (Phase 65 keeps the
// public-facing kernel convention).
type LSPLocation struct {
	Path string
	Line uint32
	Col  uint32
}

// SemanticStatus is the closed-shape struct returned by Status(). Closed
// enums:
//   - State ∈ {StatusReady, StatusDisabled, StatusBuilding, StatusError}
//   - Store ∈ {"duckdb"} (Phase 60 D-04 store kind; only one value today)
//   - LastErrorReason ∈ {"" | every FallbackReason value}
//
// All other fields are numeric counters / monotonic metrics safe to log.
type SemanticStatus struct {
	State            string
	Store            string
	LatestSnapshotID uint64
	GraphVersion     uint64
	OverlayActive    bool
	PendingLSP       int
	LastLiveUpdateMs int64
	LastErrorReason  string
}

// Status state closed-enum constants. Wire-strings are SPEC §24.5 contract;
// rename only via a deliberate envelope-shape change.
const (
	StatusReady    = "ready"
	StatusDisabled = "disabled"
	StatusBuilding = "building"
	StatusError    = "error"
)
