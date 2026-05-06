// Package cluster ships Phase 62 GRAPH-06: deterministic weak-component
// clustering over the effective semantic graph (snapshot ⊕ overlay −
// tombstones), persisted via store.OverlayTx under the existing per-
// workspace mutex (D-04 contract from Phase 60).
//
// Determinism contract (T-62-04-T1 mitigation):
//
//   - Iterate input nodes in sorted order.
//   - Sorted union-find: when merging two roots, the smaller (by NodeID
//     value) becomes the new root.
//   - Output Cluster slice sorted by ID ascending; each Members slice
//     sorted ascending.
//
// Same input → byte-identical output across runs. The hex-digest test
// (TestWeakComponents_HexDigest) freezes this guarantee against pinned
// sha256 goldens; -count=10 amplifies same-process determinism.
//
// Algorithm-only delivery: the cluster MCP tools (`get_cluster_map`,
// `explain_cluster`) are explicitly OUT OF SCOPE for v1 and deferred to
// v1.10.x per the Phase 62 roadmap (62-CONTEXT.md "Out of scope"). This
// package only ships the algorithm + persistence helpers; downstream
// tools may read from `semantic_clusters` and `semantic_cluster_members`
// once they ship.
//
// Cross-workspace isolation (T-62-04-T4 mitigation): RunClusterDetection
// uses the existing store.BeginOverlayTx per-workspace mutex — there is
// NO second mutex map in this package.
package cluster
