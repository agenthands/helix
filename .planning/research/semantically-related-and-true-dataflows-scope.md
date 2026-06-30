# Scope: SEMANTICALLY_RELATED + true DATA_FLOWS — closing the cbm-mcp depth gap

**Date:** 2026-06-30
**Author:** architect
**Status:** scope (pre-roadmap)
**Parent:** `.planning/research/semantic-graph-edge-extension.md`, `.planning/RESUME.md`

## Why this doc exists

The 2026-06-30 HEAD re-audit (see `RESUME.md` § Gaps) established that the
**edge-type schema** reached cbm-mcp parity (23/24 kinds emitted) but the
**graph intelligence** did not. Two specific depth gaps remain, and they are the
only ones that change what an agent can *answer*:

1. **SEMANTICALLY_RELATED** — declared, mapped, and validatable, but **no
   emission site exists**. It is the single unwired edge kind.
2. **DATA_FLOWS** — emitted, but as **AST structural-profile cosine similarity**
   (`Source: "ast_profile"`, conf 0.35), **not** the arg-to-param
   interprocedural flow the parent research doc (§Phase 2) defines as the goal.
   The edge name overpromises relative to what it computes.

Everything else the old RESUME called "missing" is in fact present at HEAD; this
doc deliberately scopes *only* these two.

## Verified current state (HEAD)

| Fact | Evidence |
|---|---|
| SEMANTICALLY_RELATED has no producer | grep: only `edge_kind_surface.go`, `MapInternalKind`, `tools_validate_edge.go`, tests — zero `EdgeKind: "SEMANTICALLY_RELATED"` emission |
| MinHash ported (powers SIMILAR_TO) | `internal/semantic/minhash/minhash.go` (K=64, LSH 32×2), `similarToEdges` (`semantic_similarity_edges.go:146`) |
| AST profile computed (powers structural DATA_FLOWS) | `classifier.ComputeProfile` → `ASTProfile [25]float32` (`classifier.go:449`), `dataFlowsEdges` (`semantic_similarity_edges.go:221`) |
| Per-symbol fingerprints already plumbed through all 11 providers | `extract.FingerprintBody` (`fingerprint.go:32`) stamps `SymbolFact.MinHash` + `.Profile` |
| No embedding/vector infra | bleve is BM25 text-only (`retrieval/bleve.go`); no vector/onnx/nomic dep in `go.mod` (grep clean) |
| CALLS edges exist + Dst-resolved in-repo | `resolvePendingDst` (`semantic_similarity_edges.go:57`) — the substrate interprocedural flow needs |
| Type resolver is a Phase-57 stub | `Store.QueryEffectiveEdges` returns empty (`store/duckdb.go:511`) — interprocedural flow must NOT depend on it |
| `EdgeFact` has no structured metadata column | `store/snapshot.go:210` — fields are scalar; only free-text `Reason` available for arg/param payload without a schema migration |
| Batch emission seam | `factsFromExtracted` collects `fingerprintedNode{nodeID, sig, profile, language}`, emits batch-level after the name→node index is built (`semantic_wiring.go:1604`) |

---

## Workstream A — SEMANTICALLY_RELATED (emit the missing edge)

**Goal:** symbols that are *about the same thing* (token/identifier-context
semantics) get linked, distinct from SIMILAR_TO (near-duplicate AST structure).

### Decision A0 — embedding engine (the load-bearing choice)

cbm-mcp uses **Random Indexing (RI)** — 768-dim sparse-random-projection
context vectors over identifier/comment tokens — NOT a neural model. RI is the
right fit for Helix's single-binary, no-runtime-dep, deterministic constraints:

| Option | Single-binary | Deterministic | New dep | Quality | Verdict |
|---|---|---|---|---|---|
| **Random Indexing (port cbm-mcp)** | ✅ pure Go | ✅ seeded | none | matches cbm-mcp | **RECOMMENDED** |
| Neural (nomic-embed-code via onnxruntime) | ❌ CGO/runtime | ✅ | onnxruntime + model | higher | rejected — breaks single-binary invariant |
| External embedding API | ❌ network/key | ✅ | API | higher | rejected — no runtime network |

RI is exactly what achieves *cbm-mcp parity* (same mechanism), so this is parity,
not a compromise. Neural embeddings are a separate, later, opt-in question.

### A-Phase 1 — RI engine (leaf package)
- New `internal/semantic/relatedidx/` (leaf: stdlib + xxhash, mirrors `minhash/`).
- `ComputeContextVector(tokens []string) []float32` — seeded sparse random
  projection (D-dim, e.g. 256/512; pin in code with rationale). Deterministic
  per token-set; identical input → identical vector.
- Token extraction: reuse the leaf-token walk from `minhash.collectLeafTokens`
  but keep **identifier text + docstring/comment tokens** (semantics) rather than
  normalized structural codes (which is what MinHash deliberately discards).
- Acceptance: unit tests — determinism, cosine(self)=1, near-synonym fixtures
  rank above unrelated; all pure, no network.

### A-Phase 2 — fingerprint plumbing
- Add `SymbolFact.ContextVec []float32` next to `MinHash`/`Profile` (`fact.go`).
- `FingerprintBody` computes it for fingerprintable kinds (`fingerprint.go`) —
  one shared entry point, all 11 providers inherit it automatically.
- Carry on `fingerprintedNode` (`semantic_similarity_edges.go:131`).
- Acceptance: a provider e2e test asserts `ContextVec` is populated for a Go
  function body and empty for a container kind.

### A-Phase 3 — emission
- `semanticallyRelatedEdges(nodes)` mirroring `dataFlowsEdges`: language-bucketed,
  cosine ≥ threshold (tune; start ~0.80), per-node fan-out cap (`MaxEdgesPerNode`),
  dedup on (min,max), deterministic ordering. `Source: "random_index"`,
  `EdgeKind: "SEMANTICALLY_RELATED"`, confidence ~0.45–0.55.
- Wire into `factsFromExtracted` batch emission alongside SIMILAR_TO/DATA_FLOWS.
- **Distinctness guard (anti-vacuity):** a test proving SEMANTICALLY_RELATED and
  SIMILAR_TO are *not* the same edge set on a fixture — two functions with the
  same domain vocabulary but different structure must get SEMANTICALLY_RELATED and
  NOT SIMILAR_TO, and vice-versa. Without this the edge is theater.
- Acceptance: e2e on a real-Go fixture emits ≥1 SEMANTICALLY_RELATED edge;
  distinctness guard green; `validate_graph_edge --edge-kind semantically_related`
  now resolves against live edges.

**Effort:** 2–3 phases. **New deps:** none.

---

## Workstream B — true DATA_FLOWS (arg-to-param interprocedural flow)

**Goal:** a real dataflow edge — "value produced at A reaches B" via assignment,
argument-to-parameter binding, and field access — usable as the substrate for
taint analysis (the parent doc's stated payoff, and the SMTC-tier capability
Helix lacks).

### Decision B0 — naming (resolve before building)
The current structural edge already occupies `DATA_FLOWS`. Two honest options:
- **B0-a (RECOMMENDED): rename the existing structural edge** to a truthful kind
  (e.g. `STRUCTURAL_TWIN` / fold into SIMILAR_TO family) and reclaim `DATA_FLOWS`
  for real flow. Cleanest semantics; one surface-enum migration + doc update.
- **B0-b: keep both under `DATA_FLOWS`, discriminate on `Source`**
  (`ast_profile` vs `def_use`/`arg_param`). No enum churn, but `DATA_FLOWS`
  permanently means two different things — readers must filter by `Source`.

This is a genuine fork (>1 defensible answer) → run `anvil-design-options`
before committing. Recommendation leans B0-a for long-term clarity.

### B-Phase 1 — intraprocedural def-use (foundation)
- Per-function def-use chains over the tree-sitter body: assignments, reads,
  writes, returns. Lives in `classifier`/new `dataflow` leaf pkg.
- Emit intra-function DATA_FLOWS edges (def → use) at high confidence (tree-sitter
  syntactic, ~0.70). Node granularity decision: symbol-level today (no sub-symbol
  node ids exist) → edges are function-internal summaries, or introduce
  variable-level nodes (larger; defer).
- Acceptance: a fixture function `x := src(); sink(x)` yields a def→use chain;
  no edge when the value is not forwarded.

### B-Phase 2 — arg-to-param binding (the parity feature)
- At each resolved CALLS edge (callee in-repo), bind **caller argument position →
  callee parameter** using the callee's extracted signature.
- **Storage gap:** `EdgeFact` has no structured metadata. Either (a) encode
  `arg:i→param:j` in the free-text `Reason` field (no migration, lossy to query),
  or (b) add a `semantic_edges` metadata column (migration; queryable). Pick in
  B0; (b) is required if taint queries must filter on parameter index.
- Only meaningful where callee is in-repo (external callees stay flow-opaque,
  honestly marked) — consistent with the existing DstNodeID=0 discipline.
- Acceptance: `caller` passing a tainted arg into `callee(param)` produces an
  arg→param DATA_FLOWS edge with correct positions; external callee → no fabricated edge.

### B-Phase 3 — interprocedural propagation + taint substrate
- Compose Phase-1 def-use + Phase-2 arg→param along resolved CALLS to answer
  source→sink reachability. MUST build on `resolvePendingDst`/CALLS, **never** on
  the Phase-57 `QueryEffectiveEdges` stub (returns empty).
- This is where a future SMTC-style `trace-taint-path` verb would read.
- Acceptance: multi-hop fixture `src()` → `f(x)` → `g(x)` → `sink(x)` is
  reachable end-to-end; a broken hop breaks reachability (revert-and-fail guard).

**Effort:** 3–4 phases (parent doc rates this "High"; concur — interprocedural
analysis is the hardest item in the parity backlog). **New deps:** none for
syntactic def-use; a metadata column iff B0-b/queryable arg-param chosen.

---

## Cross-cutting: the read surface (do not skip)

> **Corrected 2026-06-30 (red-team B1, ground-truthed against HEAD).** An
> earlier draft claimed only `validate-graph-edge` and `get-change-impact-graph`
> traverse edges. That is wrong, and dangerously so — *both* of those tools
> structurally CANNOT surface a new edge kind in the real binary:
> - `get-change-impact-graph` runs `ExpandFrom` against the hardcoded
>   `call_graph` projection (SQL filters `edge_kind = 'CALLS'`) AND `bfsExpand`
>   rewrites every returned edge's `Kind` to `"calls"` — a `--edge-kinds
>   semantically_related` filter then matches nothing.
> - `validate-graph-edge` depends on `SetEdgeEvidence`, which is deferred to
>   Phase 75 and unwired in production → returns `evidence_lookup_unavailable`
>   for every kind in the real CLI.
>
> The tool that DOES surface arbitrary edge kinds is **`explain-symbol-deep`**:
> its `IncomingEdgesOf`/`OutgoingEdgesOf` accessors apply NO edge-kind filter and
> `shapeEdges` maps every stored `InternalKind` through `MapInternalKind`, so a
> committed SEMANTICALLY_RELATED edge appears verbatim in
> `edges_incoming`/`edges_outgoing`. This is the cheap, correct read-surface
> target.

New analytical edges with no verb to read them are invisible to agents. Each
workstream's final phase MUST either:
- confirm the edges flow through `explain-symbol-deep` (the proven path), and/or
- scope a dedicated reader (a `find-related-symbols` extension for
  SEMANTICALLY_RELATED; a `trace-data-flow` verb for real DATA_FLOWS; or the
  larger `get-change-impact-graph` multi-projection rework — milestone-sized,
  touches that tool's D-09 invariants, explicitly out of Workstream A scope).

A parity claim is not done until the capability is reachable from `helix <verb>`.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| RI vector storage bloat (D-dim × every symbol) | Med | Med | dim cap; store only fingerprintable kinds; quantize |
| SEMANTICALLY_RELATED ≈ SIMILAR_TO (redundant edge) | Med | High | mandatory distinctness anti-vacuity guard (A-Phase 3) |
| `DATA_FLOWS` name collision misleads consumers | High | Med | resolve in B0 before any emission |
| Interprocedural scope creep (full taint engine) | High | High | phase gate: def-use → arg-param → propagation; ship each independently |
| Building on the Phase-57 resolver stub | Low | High | explicit constraint: use CALLS/resolvePendingDst only |
| Single-binary invariant broken by neural embeddings | Low | High | A0 pins Random Indexing, pure Go, no runtime dep |

## Decision points to settle before roadmapping
1. **A0** — Random Indexing confirmed as the embedding engine? (recommend yes)
2. **B0** — rename structural DATA_FLOWS (B0-a) vs Source-discriminate (B0-b)? → `anvil-design-options`
3. **B-Phase 2 storage** — free-text `Reason` vs new metadata column for arg→param?
4. **Node granularity** — symbol-level only, or introduce variable-level nodes for true def-use? (larger; likely defer)
5. **Read surface** — extend existing verbs vs new `trace-data-flow` verb?

## Suggested sequencing
Workstream A (2–3 phases, no deps, self-contained, low risk) **first** — it
closes the one unwired edge kind cleanly and de-risks the fingerprint-plumbing
pattern. Workstream B (3–4 phases, harder, needs the B0 fork resolved) **second**.
Total: ~5–7 phases — a milestone in its own right ("Graph Intelligence Depth").

## References
- `internal/daemon/semantic_similarity_edges.go` — SIMILAR_TO / DATA_FLOWS emission (the pattern to mirror)
- `internal/semantic/minhash/minhash.go` — port template for the RI engine
- `internal/semantic/classifier/classifier.go:449` — `ASTProfile` (structural, not semantic)
- `internal/semantic/extract/fingerprint.go` — the shared per-provider fingerprint seam
- `internal/semantic/store/snapshot.go:210` — `EdgeFact` (no metadata column)
- `internal/semantic/store/duckdb.go:511` — Phase-57 resolver stub (do not depend on)
- `.planning/research/semantic-graph-edge-extension.md` — parent parity research
- `.planning/RESUME.md` — HEAD audit + corrected gap list
