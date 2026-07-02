# Edge Types Reference

Helix's semantic graph (`internal/semantic/`) is a typed, language-agnostic edge
graph over code symbols. This document describes the edge kinds a committed
snapshot can hold, how each is produced, and which `helix` verbs read them.

The closed lowercase MCP-surface enum is `EdgeKindSurface`
(`internal/skill/semantic/edge_kind_surface.go`); the extractor/resolver internal
kind strings map one-way into it via `MapInternalKind`.

## Families

| Family | Edges | Produced by |
|---|---|---|
| Call / type | `calls`, `references`, `has_type`, `uses_type` | LSP call-hierarchy + type resolver (Note: tree-sitter emits no `CALLS` edge — `calls` is LSP-only) |
| Structure | `contains`, `defines`, `imports`, `implements`, `extends` | tree-sitter batch (`factsFromExtracted`); targets resolved against the batch name→node index (`resolvePendingDst`) |
| Call classification | `http_calls`, `async_calls`, `emits`, `listens_on`, `configures`, `writes` | `classifier.ClassifyCall` (11-language pattern tables) |
| Graph structure | `handles`, `tests`, `member_of` | tree-sitter batch (name/route heuristics) |
| Co-change / cross-repo | `file_changes_with`, `cross_imports`, `cross_calls` | git co-change mining; cross-repo resolver |
| Similarity | `similar_to`, `structural_twin`, `semantically_related` | MinHash+LSH / ASTProfile cosine / Random-Indexing (all computed at extraction via `FingerprintBody`, all 11 providers) |
| Data flow | `data_flows` | `dataFlowEdges` — case-1 caller.param→callee.param (v2.9); `inBodyDataFlowEdges` — in-body-origin producer.function→consumer.param (v2.13); `returnBridgeEdges` — return-bridge param→enclosing-function (v2.13) |

## Edge kinds

### `data_flows` (v2.9 param→param · v2.13 in-body origins + return-bridge)
- **Internal kind:** `DATA_FLOWS` (all variants). The `Source` marker distinguishes
  the three producers; all three surface as `data_flows` (`MapInternalKind`,
  `edge_kind_surface.go`).
- **`Source: def_use`** (v2.9 · conf ~0.55) — case-1 **caller.param → callee.param**.
  `dataflow.AnalyzeFlow` computes a per-function flow summary (param → reaches-return
  / reaches-call(name, argPos)) at extraction time; `dataFlowEdges` binds
  caller.param → callee.param through each resolved in-repo call.
- **`Source: def_use_inbody`** (v2.13 · conf ~0.50) — **producer.function →
  consumer.param**. The return value of an in-body call (`local := producer();
  sink(local)`) is an origin; the edge anchors on the producer's *function* node
  (the honest available identity for a return value — a return has no distinct
  graph node) and the consumer's *parameter* node. Emitted by `inBodyDataFlowEdges`.
- **`Source: def_use_return`** (v2.13 · conf ~0.55) — **param → enclosing-function**.
  A "return-bridge" recorded when a parameter's value reaches its function's return
  (`transform(x){ return x }`). Emitted by `returnBridgeEdges`; the minimal
  zero-schema connector that makes in-body origins composable for multi-hop
  reachability (`producer.fn → transform.param → transform.fn → sink.param`).
- **Anti-mis-bind (all variants):** a callee/producer name resolves only when it maps
  to exactly one node (`nameCount==1`); external/overloaded/unresolved endpoint ⇒ no
  fabricated edge.
- **Honest scope:** syntactic case-1 data dependence — param→param pass-through PLUS
  in-body call-return origins and the return-bridge. Binding is **symbol-node
  identity** (no persistent variable-level nodes). Still NOT taint analysis:
  **variable-level precision, field/heap flow, and source/sink sanitization remain
  deferred.**
- **Language coverage (v2.13 — all 11 proven E2E):** the flow engine records the
  in-body flow across all 11 grammars (Phase 138 all-11 `AnalyzeFlow` unit matrix),
  emission is proven for all 11 (`factsFromExtracted`), and the in-body edge is proven
  **end-to-end through the shipped `helix` binary** (real `helix index-semantic-graph`
  → `explain-symbol-deep`) for **all 11 languages**: Go, TypeScript, Java, C#, Python,
  C, C++, Rust, Kotlin, PHP, Ruby (Phase 140). v2.13 wired the last four into the
  daemon's full-index walk (`langFromExt` now maps `.rs/.kt/.php/.rb`); their type
  resolvers remain nil-stubs, so they contribute symbols / references / similarity /
  `data_flows` but **no `has_type` / `uses_type`** edges.
- **Read by:** `helix trace-data-flow` (v2.10 — source→sink reachability from a seed;
  the BFS is kind-agnostic node-ID adjacency, so a **function seed** now mechanically
  reaches in-body targets over `def_use_inbody` edges — full function-seed support in
  the verb's documented contract is a fast-follow; the documented seed stays a
  parameter); `helix explain-symbol-deep` (edges verbatim, no kind filter).

### Similarity (v2.8)
- **`similar_to`** (`SIMILAR_TO`, source `minhash`, conf ~0.45) — MinHash near-clone
  bodies (Jaccard ≥ 0.95 via LSH). Structural SHAPE.
- **`structural_twin`** (`STRUCTURAL_TWIN`, source `ast_profile`, conf 0.35) —
  control-flow/expression-shape profile cosine ≥ 0.985. Renamed from a formerly
  mis-named `DATA_FLOWS` in v2.8 (B0-a). Structural SHAPE, distinct from `similar_to`.
- **`semantically_related`** (`SEMANTICALLY_RELATED`, source `random_index`, conf
  ~0.50) — Random-Indexing context vectors over identifier/comment VOCABULARY,
  cosine ≥ 0.55. Orthogonal to the two structural edges by construction.

### Call / type
- **`calls`** — caller→callee. LSP call-hierarchy only (no tree-sitter `CALLS` edge
  at HEAD). 7-tier confidence ladder.
- **`references`, `has_type`, `uses_type`** — symbol references; type resolution
  (has_type = "this symbol HAS this type"); type usage. LSP/resolver.

### Structure
- **`defines`** — container→child. NOTE: the tree-sitter `DEFINES` emitter is a flat
  "file's first symbol owns everything" heuristic (NOT a per-function map); do not
  use it to recover a function's parameters (v2.9 D1b finding).
- **`contains`, `imports`, `implements`, `extends`** — scope/import/heritage edges;
  in-repo targets resolved via the batch name index.

### Call classification / channels
- **`http_calls`, `async_calls`** — framework HTTP/async call sites (per-language
  catalogs). **`emits`, `listens_on`** — event channel producers/consumers.
- **`configures`, `writes`** — config/persistence call classification.
- **`handles`, `tests`, `member_of`** — handler→route; test→production; member→container.

### Co-change / cross-repo
- **`file_changes_with`** — git co-change coupling. **`cross_imports`, `cross_calls`** —
  cross-repo import/call edges.

## Read surface (which verb surfaces a given edge kind)

- **`helix explain-symbol-deep`** — surfaces EVERY stored edge kind verbatim
  (`IncomingEdgesOf`/`OutgoingEdgesOf` apply NO edge-kind filter; `MapInternalKind`
  maps each internal kind to the surface enum). The general edge reader.
- **`helix trace-data-flow`** (v2.10) — consumes `data_flows` for source→sink
  reachability from a seed. Documented seed is a parameter, but the BFS is a
  kind-agnostic node-ID adjacency walk over ALL `data_flows` edges (`def_use` +
  `def_use_inbody` + `def_use_return`), so post-v2.13 a **function seed** also
  reaches in-body targets (over the producer.function → consumer.param
  `def_use_inbody` edge). Full function-seed support in the verb's documented
  contract is a fast-follow.
- **`helix get-change-impact-graph`** — hardcoded to the `call_graph` projection
  (filters `edge_kind = 'CALLS'`) and rewrites every returned edge `Kind` to
  `"calls"`; it CANNOT surface other edge kinds. (A multi-projection rework is a
  deferred milestone.)

## Storage

Edges live in `semantic_edges` (and the live overlay) keyed by
`(snapshot_id, edge_id)`, with `src_node_id`/`dst_node_id` (which ARE the symbol
`symbol_id` uint64s — same namespace), `edge_kind`, `source`, `confidence`,
`validation_state`, `reason`. EdgeIDs are stamped dense 1-based per snapshot in the
build pipeline (the v2.8 PK-collision fix).

## Language coverage

11 first-class extractors: Go, TypeScript/JavaScript, Python, Java, C#, Rust, C,
C++, Kotlin, PHP, Ruby (`internal/semantic/extract/<lang>/`, one `queries.scm`
each). The graph engine itself is language-agnostic (`NodeID`/`EdgeKind`); only the
extractor + type-resolver depth varies (full type resolution for Go/TypeScript/Python
and — since v2.11 — Java/C#/C/C++; LSP-conditional stubs for PHP/Ruby/Rust/Kotlin). See
`docs/type-resolution.md` for the per-language tier ladder.
