// Package integ is the types-only seam between the Phase 60–64 semantic
// engine (daemon-resident) and Phase 65 MCP-tool consumers. It owns the
// SemanticLookup interface (D-03), closed-enum constants for Source (D-04)
// and FallbackReason (D-05), value types crossing the seam, error sentinels,
// and the NoopLookup default.
//
// Architecture invariants enforced by adjacent gates:
//
//   - vet-noduckdb: this package MUST NOT import duckdb-go (or any package
//     that imports it). All types here are plain-data; the production
//     adapter that wraps the actual store lives daemon-side
//     (internal/daemon/semantic_wiring.go integSemanticLookup).
//
//   - vet-nokernel2semantic: kernel packages (internal/kernel/*) may import
//     this package only. The Phase 65 65-00 amendment adds an exact-or-
//     slash-boundary allowlist for "internal/semantic/integ"; any other
//     internal/semantic/* import from kernel still fails the build.
//
//   - M-readtier (read+ tier): every SemanticLookup method is read-only.
//     The grep canary at internal/daemon/integ_lookup_test.go
//     (TestIntegSemanticLookup_ReadTierCanary) blocks any production
//     integSemanticLookup method body from referencing the snapshot-write
//     surface (BeginSnapshot / CommitSnapshot / AbortSnapshot /
//     WriteSnapshotFacts / OnFlush / BumpGraphVersion).
//
//   - M-cold (D-06): Available() reflects "configured + store live" only.
//     It does not trigger background indexing on cold start; agents that
//     want fresh ranking call index_semantic_graph explicitly.
//
//   - WR-NEW-01: every error flowing through SemanticLookup is classified
//     by ClassifyLookupErr (source.go) before reaching the MCP envelope.
//     Raw error text never appears on the wire.
//
// The package surface is intentionally small. Adding a method here is a
// SemanticLookup contract change and ripples to every consumer adapter
// (Pattern 1) plus the production daemon adapter (Pattern 2).
package integ
