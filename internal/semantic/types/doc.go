// Package types implements the Phase 62 P05 type-resolution layer.
//
// SCOPE
//
// This package owns the tiered resolver dispatcher (D-11), the SPEC §38.2
// 7-tier confidence ladder, the bounded access-chain walker (max depth 8),
// the bounded fixpoint loop (max iter 8) with state-hash early-exit, and
// the storage-side edge emission helper that routes every write through
// the D-14 two-phase merge predicate (`tx.UpsertEdgesWithMerge`, P02).
//
// LAYOUT (D-11)
//
//   - resolver.go: Resolver interface + Dispatcher + Request/Response types.
//   - ladder.go:   7 confidence constants + CapCommentConfidence + helpers.
//   - chain.go:    ResolveAccessChain (depth-bounded walker).
//   - fixpoint.go: FixpointResolve (iter-bounded loop with state.Hash()).
//   - emit.go:     EmitEdges (two-phase merge via tx.UpsertEdgesWithMerge).
//
// Per-language resolvers live in subpackages (golang/, typescript/,
// python/, java/, php/, ruby/) and are registered with the Dispatcher by
// the daemon at bootstrap.
//
// INVARIANTS
//
//   - TYPES-01: RESOLVES_TO/CALLS/USES_TYPE edges are emitted with the 7-tier
//     ladder confidence (1.00 / 0.90 / 0.80 / 0.70 / 0.60 / 0.45 / 0.20).
//
//   - TYPES-02: chains are bounded at maxDepth=8; fixpoint loops are bounded
//     at maxIter=8 with early-exit on no-progress.
//
//   - TYPES-03: comment-derived edges NEVER exceed confidence 0.60 unless
//     independently confirmed by LSP. Enforced by CapCommentConfidence in
//     emit.go and at the storage boundary by tx.UpsertEdgesWithMerge.
//
//   - TYPES-04: non-converged chains emit with low confidence + unresolved
//     markers, NEVER "validated". Enforced by FixpointResolve and EmitEdges.
//
//   - D-12: comment parsers are hand-rolled (regex-only); no new third-party
//     dependencies. PHP/Ruby ALWAYS emit confidence=0.20 unresolved (never
//     silently skip). Java is LSP-conditional — short-circuits on Phase 61
//     LSP-validated RESOLVES_TO edges, otherwise 0.20 unresolved.
//
//   - D-13: fixpoint scope is per-package — Go = file's directory; TypeScript
//     = nearest tsconfig.json; Python = walk to __init__.py. CROSS-PACKAGE
//     CHAINS DO NOT RESOLVE IN v1 — emit at last-in-package confidence with
//     validation_state="unresolved".
//
//   - D-14: comment edges enter the overlay at confidence=0.60 immediately;
//     a matching LSP edge for the same (src, dst, kind) deletes the 0.60
//     row and inserts at 1.00 — predicate enforced by
//     internal/semantic/store.OverlayTx.UpsertEdgesWithMerge.
//
// THREAT BOUNDARIES
//
//   - In-process: this package is in-process; reads facts from the store via
//     a narrow EffectiveReader seam; emits edges via a narrow EmitTx seam
//     that wraps store.OverlayTx.
//   - Comment→graph: two-phase merge enforced at storage; comment writes
//     enter at 0.60 and are silently upgraded by LSP via the merge predicate.
//   - Cross-workspace isolation: per-workspace mutex re-use via
//     store.OverlayTx (Phase 60 D-04). NO sync.Map / second mutex map in
//     this package — resolvers are stateless functions over (req, store).
package types
