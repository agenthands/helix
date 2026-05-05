// Package semantic holds the shared type aliases and configuration shape used
// by the semantic-graph subsystem (Phase 57+, v1.10).
//
// SPEC reference: SPEC-DRAFT.md §7 (typed identifiers) and §25 (config keys).
//
// This package is intentionally tiny — it carries only the cross-package
// types that BOTH internal/semantic/store/ AND internal/config/ reference,
// so neither can pull in the duckdb-go dependency by accident. The store
// package owns the duckdb-go import (D-12, STORE-06).
package semantic

// SnapshotID is the monotone identifier for a semantic snapshot row in
// semantic_snapshots (SPEC-DRAFT.md §9.2). Snapshots are immutable; readers
// resolve effective facts via snapshot ⊕ overlay − tombstones (SPEC §10).
type SnapshotID uint64

// FileID is the monotone identifier for a file row in semantic_files
// (SPEC-DRAFT.md §9.4). Stable within a single store; not portable across
// stores or rebuilds.
type FileID uint64

// SymbolID is the monotone identifier for a symbol row in semantic_symbols
// (SPEC-DRAFT.md §9.5).
type SymbolID uint64

// ReferenceID is the monotone identifier for a reference row in
// semantic_references (SPEC-DRAFT.md §9.6).
type ReferenceID uint64

// EdgeID is the monotone identifier for an edge row in semantic_edges
// (SPEC-DRAFT.md §9.7).
type EdgeID uint64

// ImportID is the monotone identifier for an import row in
// semantic_imports (SPEC-DRAFT.md §9.x). Phase 59 introduces it alongside
// the tree-sitter extraction layer.
type ImportID uint64

// TypeFactID is the monotone identifier for a type-annotation fact row
// in semantic_type_facts (SPEC-DRAFT.md §9.x). Phase 59 introduces it.
type TypeFactID uint64

// HeritageID is the monotone identifier for a heritage-edge row
// (extends / implements / embeds) in semantic_heritage (SPEC-DRAFT.md §9.x).
// Phase 59 introduces it.
type HeritageID uint64

// WorkspaceID is the opaque identifier for a workspace registered with the
// daemon's kernel. It is the key under which the extraction scheduler tracks
// per-workspace state, in-flight jobs, and subscribers (Phase 59 P03, D-04).
// Stable for the lifetime of a daemon process; not portable across daemon
// restarts.
type WorkspaceID string

// RepoID is the opaque identifier under which the semantic store's overlay
// and snapshot tables key their per-workspace rows (DDL column `repo_id`).
// Phase 60 P04 introduces it as a typed alias so cross-package signatures
// (live.SourceChangeEvent, scheduler.IncrementalHandler, store helpers)
// can carry it explicitly instead of stringly-typed `string` parameters.
//
// Currently a 1:1 alias with WorkspaceID at the daemon-wiring layer (the
// daemon converts WorkspaceKey → repoID via a single function) but kept
// distinct so future repo-vs-workspace fan-out (e.g., multi-workspace per
// monorepo) can refine the relation without churning every signature.
type RepoID string

// Freshness classifies the staleness of a fact returned by the effective-read
// API (SPEC-DRAFT.md §10). Values are written into log fields and (later)
// surfaced as a metric label, so the enum is closed.
type Freshness string

const (
	// FreshnessFresh means the fact comes from the live overlay or from a
	// snapshot whose underlying file mtime matches the fact's recorded mtime.
	FreshnessFresh Freshness = "fresh"
	// FreshnessStale means the fact comes from a snapshot but the file has
	// been modified since indexing; consumers should warn but may still serve.
	FreshnessStale Freshness = "stale"
	// FreshnessUnknown means the fact cannot be classified (e.g., the file
	// is missing, mtime is unreadable). Consumers should treat as stale.
	FreshnessUnknown Freshness = "unknown"
)
