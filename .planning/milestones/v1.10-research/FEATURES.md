# Feature Research — Helix v1.10 Live Semantic Index

**Domain:** Live, evidence-backed semantic graphs for coding agents (durable graph store + live overlay + LSP enrichment + agent-facing context tools + guardrails + eval harness)
**Researched:** 2026-05-03
**Confidence:** HIGH on adjacent-product behavior and evaluation conventions; MEDIUM on confidence-tier conventions for dynamic-language type resolution; LOW on specific guardrail-receipt schemas (no published prior art — this is novel territory).

**Scope guard:** This file is *only* about v1.10 — the Live Semantic Index milestone. Existing v1.0–v1.9 features (the 41 shipped MCP tools, profile/mode gating, fuzzy edit cascade, RepoMap with PageRank, memory store, observability, signed releases) are the **baseline**, not features under research. Anywhere a feature integrates with an existing tool, the existing tool is referenced by name but not re-described.

---

## 0. Adjacent-Product Survey (Evidence Base)

| Product | What it is | Graph model | Freshness model | Agent surface |
|---|---|---|---|---|
| **Sourcegraph SCIP** | Successor to LSIF; Protobuf schema with human-readable symbol IDs; per-language indexers (scip-go/java/python/clang/typescript) | Per-document occurrence + symbol roles + relationships; cross-document via stable symbol IDs | **Batch + incremental:** indexer re-runs per commit; build-system integration to re-index only changed files; full re-index on schema bump | HTTP API + LSIF/SCIP upload; no live overlay |
| **GitHub Stack Graphs** | Tree-sitter–based name resolution that doesn't need a build; powers GitHub blob navigation | Stack-graph nodes + scopes + jump-edges; resolution is path-finding, not symbol-table lookup | Per-commit batch on push; no live update | Per-blob nav links; "best-effort" precision |
| **Meta Glean** | Production code indexing at Meta; queryable fact store with Angle DSL | Stacked immutable databases — each layer adds/hides facts non-destructively | Stacks are append-only; "live" achieved by stacking a small recent layer on top of a large base | Angle queries; powers internal code search/nav/docs |
| **Aider RepoMap** | Per-session ranked structural overview for LLM context | Tree-sitter tags → file dependency graph → personalized PageRank with mention/well-named/in-chat multipliers | **Recomputed per turn** from disk; no overlay, no persistence | Inline in LLM prompt; binary-search token-budget fit |
| **Cursor index** | Workspace embedding index for semantic chunk retrieval | Embedding chunks + symbol metadata (closed source) | **Re-index cycle, ~minutes-to-hours**; users routinely use `@file` to bypass stale index | Inline retrieval; explicit `@symbol`/`@file`/`@code` mentions |
| **Sourcegraph Cody** | RAG over remote graph + embeddings | SCIP graph + vector embeddings + zoekt | Server-side pipeline, freshness ≈ commit cadence | Chat + autocomplete; cites file ranges |
| **Augment Context Engine** | Semantic dependency graph for cross-repo agent workflows | "Real-time indexing" + dependency analysis + commit-history lineage (closed source) | Marketed as real-time/instant across distributed repos | Long-lived agent sessions; 200k context |
| **Kythe** | Google's older graph-based source indexer | Schema-rich fact graph (fact ↔ edge ↔ vname) | Per-build batch via build extractor | Cross-references service; not agent-facing |

**Implications for Helix:**

1. The market has either **batch graphs without live updates** (SCIP, Glean, Stack Graphs, Kythe) or **live retrieval without a durable graph** (Aider, Cursor, Cody). Helix v1.10's combination — DuckDB-committed snapshots + fsnotify live overlay + effective-read semantics + LSP revalidation — is **not** the dominant pattern in any single shipping product. Glean's stacked-DB pattern is the closest precedent, but Glean does not expose live overlay semantics on top of source-file mutations between snapshots.
2. Aider RepoMap is already present in Helix v1.6+. v1.10's job is to make `get_repo_map` / `get_context` *graph-backed* without losing the per-session token-budget elision that makes RepoMap useful.
3. Every adjacent product has a freshness story; **none expose freshness as a first-class API surface to the agent**. Helix's `freshness` field on every response (fresh / structurally_fresh_semantically_pending / approximate_scores / stale) is genuinely differentiated and addresses a real Cursor pain point ("wait for the next re-index cycle, or use `@file`").

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features any agent-facing semantic-graph product must ship. Missing these = product is incomplete relative to the market.

| Feature | Why Expected | SPEC mapping | Complexity | Notes |
|---|---|---|---|---|
| **Durable, queryable symbol+reference graph** | SCIP, Glean, Stack Graphs, Kythe all ship this; without it, every session pays cold-start cost | §8 Store, §9 Data Model, §13 Extraction | HIGH | DuckDB choice (§4) is the differentiator; the graph itself is table stakes |
| **Stable symbol IDs across renames/moves** | SCIP's headline feature; LSIF was rejected because globally-incrementing IDs broke incremental indexes | §11 Symbol Identity | HIGH | Hash-based stable keys with rename detection — must survive file moves |
| **Incremental update without full reindex** | SCIP's reason for existing; agents will not tolerate multi-minute re-index per save | §16 Live Update Pipeline | HIGH | fsnotify + event coalescing + per-file overlay writes |
| **Cross-file find-references / call-hierarchy** | LSP baseline; without this the graph adds no value over `Grep` | §12 Edge Vocabulary, §14 LSP Enrichment | MEDIUM | Already shipped via existing LSP-backed tools; v1.10 extends to graph-cached form |
| **Token-budgeted ranked context** | Aider RepoMap is the de-facto bar; agents have hard token ceilings | §20 Retrieval Engine + integration with `get_repo_map` / `get_context` | MEDIUM | Reuse existing v1.6 RepoMap fitter; swap data source from live tags to graph scores |
| **Tree-sitter–first extraction with LSP enrichment** | Aider/Stack Graphs prove tree-sitter is fast enough for live work; LSP for precision | §13, §14 | MEDIUM | Tree-sitter for structural facts, LSP for semantic confirmation — already proven in v1.6 RepoMap |
| **PageRank-style importance ranking** | Aider's repomap PageRank is widely cited; agents and humans both want "what matters most" | §18 PageRank | MEDIUM | Already shipped in v1.6; v1.10 extends to multiple projections (call/reference/file-dep) |
| **Indexing progress + status tool** | SCIP indexers, Cursor's UI all report "indexing N% complete"; agents need to know when to retry | §23.3 `get_semantic_graph_status`, §24.5 `get_health` extension | LOW | Extend existing `get_health` |
| **Diagnostics integration after edits** | LSP basics; agent loops break without it | §24.4 edit-tool integration | LOW | Already shipped (`verify_edit`, `get_diagnostics`); v1.10 just emits live-graph events |
| **Workspace activation + lifecycle** | Existing daemon pattern; v1.10 must hook into it without regressing it | §15 Indexing Pipeline, §39 Pipeline DAG | MEDIUM | Pipeline DAG (§39) protects this — explicit phase ordering |

### Differentiators (Where v1.10 Competes)

Features that materially differ from adjacent products and align with Helix's "rock-solid LSP-backed runtime" positioning. Each is justified against a specific competitor weakness.

| Feature | Value Proposition | Competitive position | SPEC mapping | Complexity | Notes |
|---|---|---|---|---|---|
| **Effective-read semantics (committed snapshot ⊕ live overlay)** | Agents see a single coherent graph that reflects unsaved/uncomitted changes; no "wait for next reindex" UX | **vs Cursor:** Cursor's freshness gap is its top documented complaint. **vs SCIP/Glean:** neither has overlay. **vs Aider:** Aider has freshness but no persistence. | §10 Effective Read Semantics, §17 Graph Cache and Repair | HIGH | Genuinely novel combination; risk surface is overlay/snapshot consistency |
| **First-class freshness on every response** | `freshness` field + `score_status` per projection + `pending_lsp_files` count returned with every tool call | **vs all competitors:** none expose this to agents. Cursor users discover staleness by failure. | §23.4 (and every §23.x tool), §28 Observability | LOW (once schema set) | Cheap to add, high agent-trust payoff |
| **Tiered confidence on graph edges with explicit evidence** | Edges carry `Confidence ∈ [0.20, 1.00]`, evidence kinds (`lsp_hover`, `assignment`, `doc_comment`, `heuristic`), and `validation_state` | **vs SCIP:** SCIP edges are binary precise/fuzzy. **vs Stack Graphs:** "best-effort" with no per-edge confidence. **vs pyright/sorbet:** they have internal tiering but don't expose it. | §38.2 Type Evidence Model, §38.7 Edge Emission | MEDIUM | Confidence tiers (1.00 LSP, 0.90 annotation, …, 0.20 unknown) match pyright's `strict`/`basic`/`off` and sorbet's `# typed: true/false/strict/strong` patterns at a finer grain |
| **Live LSP revalidation queue with priority boosts on edit** | After an edit, the touched file is bumped to the front of an LSP revalidation queue; agent can `wait_for_lsp` if it cares | **vs Cody/Cursor:** server-side reindex without per-edit prioritization. **vs Aider:** no LSP at all. | §21 LSP Revalidation Queue, §24.4 edit integration | MEDIUM | Maps cleanly to existing v1.9 LS dispatch wiring (Phase 56) |
| **Multi-projection PageRank (call / reference / file-dep) with per-projection freshness** | Agent picks the projection matching its question (impact = call graph; relevance = reference graph; build order = file-dep) | **vs Aider:** single PageRank over file-dep only. **vs Cody:** opaque ranking. | §18 PageRank | MEDIUM | Allows `score_status: { CALL_GRAPH_PAGERANK: "approximate", REFERENCE_PAGERANK: "stale", … }` |
| **Cluster maps + cluster explanations** | Agents get topic-level navigation ("auth subsystem", "billing pipeline") instead of raw symbol lists | **vs SCIP/Glean:** none ship clustering. **vs Cody/Augment:** "architectural understanding" claimed but not exposed as discrete clusters. | §19 Clustering, §23.7 `get_cluster_map`, §23.8 `explain_cluster` | MEDIUM | Weak-component + label-propagation is well-known; novelty is exposing it as an MCP tool |
| **Guardrail policy engine + safety receipts** | `rename_symbol` requires a prior `find_references`/`analyze_blast_radius` receipt; missing receipts → warn / require_force / enforce per profile | **vs Copilot Cloud Agent:** Copilot has org-level guardrails (allow-lists, network policy) but not per-tool semantic pre-checks. **vs Devin:** sandbox-level only. **vs Claude Code/Codex:** hooks exist but no semantic correlation across tool calls. | §36 Guardrails, G-001..G-010 | HIGH | Genuinely novel surface; closest prior art is shell-hook style PreToolUse, but those are syntactic not semantic |
| **Definition-of-Done per task class** | `DoD.md` codifies expectations: "rename = identify + find_references + rename_symbol + verify_edit + report" | **vs all:** task-class DoD is documentation territory in every other product, not a runtime-checkable artifact | §36.3 | MEDIUM | The runtime-checkable half is the receipts; the doc half is `DoD.md` itself |
| **Eval harness with baseline / native / semantic / semantic_guarded modes** | A/B Helix's value with the same agent against the same tasks across four modes; reports cost, latency, success, tool-behavior, safety | **vs SWE-bench:** SWE-bench scores agents, not tools. **vs Aider polyglot:** edits-only, no tool-attribution. **vs Cursor/Cody marketing:** unverifiable claims. | §37 Evaluation Harness | HIGH | Tool-behavior scoring (§37.7) is the differentiator: +1 for `find_references` before rename, −1 for grep-only rename |
| **Type resolution with fixpoint loop for dynamic languages** | `a.b.c.d()` resolved incrementally with confidence decay; comment fallbacks (JSDoc, PHPDoc, YARD, Python type comments) capped at 0.60 unless LSP-confirmed | **vs pyright/sorbet/Phan:** they're full type checkers; Helix's job is graph edges, not soundness. **vs SCIP-python:** SCIP indexer has no fixpoint, just per-occurrence resolution. | §38 | HIGH | Bounded fixpoint with `MaxFixpointIterations` is the safety hatch |
| **Validate-graph-edge tool** | Agent can ask "is this edge real?" and the daemon will run live LSP definition/references to confirm | **vs all competitors:** none let the agent challenge the index | §23.10 `validate_graph_edge` | LOW | Cheap given existing LSP plumbing |
| **Typed pipeline DAG with phase validation** | Daemon bootstrap, indexing, live update, eval all expressed as DAGs with cycle/missing-dep detection at compile time or startup | **vs all:** internal hygiene, but prevents the class of bug that bit `OnNotification` in v1.9 (Phase 56) | §39 | MEDIUM | This is the reason v1.10 won't repeat Phase-56-style "silently dropped" bugs |

### Anti-Features (Looks Good, Don't Build)

Things competitors ship or users will request, that Helix should explicitly refuse.

| Anti-Feature | Why Tempting | Why Problematic | What to Do Instead |
|---|---|---|---|
| **Vector/embedding search inside Helix** | Cursor and Cody prove embeddings work for fuzzy code Q&A | **Out of scope per PROJECT.md.** Augment/Cursor do this better; embeddings are a different cost/staleness profile (re-embed on change is expensive); pollutes the "graph + LSP" mental model | Keep `Out of Scope`. Let the agent compose Helix's graph tools with its own embedding tools (Augment, etc.) |
| **Knowledge-graph storage (RDF/SPARQL/property-graph DB)** | Glean's Angle DSL, Kythe's vname graph, modern KG stacks | **Out of scope per PROJECT.md.** CodeGraphContext/GitNexus own this. DuckDB columnar is the right fit for *this* graph (snapshots, range scans, joins) | Keep `Out of Scope`; DuckDB choice (§4) is correct |
| **"Real-time across the whole repo" indexing claim** | Augment markets it; sounds impressive | Sets an expectation Helix cannot meet on cold start (LSP indexing on jdtls/rust-analyzer is minutes, not seconds); creates a credibility liability | Document tiered freshness explicitly: tree-sitter fast path is sub-second; LSP enrichment is queued and reported via `pending_lsp_revalidations` |
| **Auto-execute high-risk operations when guardrails pass** | Convenience: "if checks pass, just do it" | Conflates *guardrail satisfied* with *user authorized*. Guardrails are necessary, not sufficient. | Default `enforce` only on `review` profile; `read`/`edit` profiles `warn`; `admin` `warn` unless configured. Always require user-visible action for destructive ops |
| **Embedding the LSP language servers in-process** | Eliminates IPC overhead | Already rejected by v1.0 architecture; jdtls/rust-analyzer are JVM/Rust processes — cannot in-process | Continue worker-pool model from v1.0; v1.10 just adds revalidation queue on top |
| **Soundness-grade dynamic type inference (pyright/sorbet equivalent)** | Pyright produces near-tsserver precision on Python | We are not a type checker; we emit graph edges. Pyright is 60k+ LOC of dedicated type narrowing. | Stop at "tiered confidence with evidence"; cap doc-comment evidence at 0.60; let the LSP (pyright/pylsp) be the ground truth when present |
| **"Best agent on SWE-bench" leaderboard chase** | SWE-bench is the visible benchmark; topping it is marketable | SWE-bench scores *agents*, not tools. Helix should improve any agent's SWE-bench score — that's the eval design (§37.2 modes). Optimizing for the leaderboard with a specific agent risks overfitting | §37 modes — measure delta, not absolute |
| **Per-edge "explanation" via LLM** | Glean-style "explain why this edge exists" with natural language | Adds LLM dependency to the index; non-deterministic; makes the graph unverifiable. Evidence kinds (§38.2) are already structured and auditable | Keep evidence kinds enum; let the agent generate prose if it wants |
| **Persistent guardrail receipts across sessions** | "Once the user did `find_references`, the agent shouldn't have to redo it next session" | Stale receipts on stale graphs are dangerous: code changed, the receipt is meaningless. Receipts must be tied to `graph_version` and `freshness` | Receipt has `GraphVersion` field (§36.4) — invalidate on graph version change |
| **Auto-rebuild from clean on every Helix start** | Simplicity, no overlay invariants to maintain | Defeats the entire snapshot+overlay design; multi-minute startup is a regression vs v1.9 | Lazy/incremental load (§16); full reindex on schema bump only |

---

## Feature Dependencies

```
[Store contract §8] ──> [Data model §9] ──> [Symbol identity §11]
                                                |
                                                v
[Extraction §13] ──> [Indexing pipeline §15] ──> [Snapshot]
                                                    |
                                                    v
[Live update §16] ──> [Overlay] ──> [Effective read §10]
                                          |
                                          v
[Graph cache + repair §17]
        |
        +──> [PageRank §18] ──> [Clustering §19]
        |              |
        |              v
        +──> [Retrieval engine §20]
                       |
                       +──> [`get_semantic_context` §23.4]
                       +──> [`find_related_symbols` §23.6]
                       +──> [`get_cluster_map` §23.7]
                       +──> [`get_change_impact_graph` §23.9]

[LSP enrichment §14] ──> [Revalidation queue §21] ──> [`validate_graph_edge` §23.10]
                                                  └─> [validation_state on edges §38.7]

[Type resolution §38] ──depends on──> [Extraction §13] + [LSP enrichment §14]
                     ──emits──> [RESOLVES_TO / CALLS / USES_TYPE edges §12]

[Existing tools §24] ──read from──> [Effective read §10]
[Edit tools §24.4] ──emit events to──> [Live update queue §16]

[Guardrails §36] ──reads──> [graph + receipts]
                ──gates──> [rename_symbol, safe_delete_symbol, replace_symbol_body, fuzzy_edit]

[Eval harness §37] ──exercises──> [all of the above across 4 modes]

[Pipeline DAG §39] ──validates──> [bootstrap order, indexing order, live-update order]
```

### Critical Dependency Notes

- **Effective read (§10) is the keystone.** Every retrieval tool reads through it; every edit/fsnotify event writes through the overlay path. Get this wrong and every downstream tool returns inconsistent results. **Land §8/§9/§10/§11 before any §23 tool.**
- **Symbol identity (§11) blocks rename detection.** Without stable IDs across moves/renames, the live update pipeline cannot distinguish "symbol moved" from "symbol deleted + new symbol added", which corrupts call graphs.
- **PageRank (§18) and clustering (§19) consume the graph but do not block tool exposure** — `freshness=approximate` covers the gap during recompute.
- **LSP revalidation queue (§21) is downstream of §14 enrichment and §16 live update.** It can ship after the structural pipeline works.
- **Guardrails (§36) require graph + receipts but NOT clustering/PageRank.** Can ship in parallel with §18/§19.
- **Eval harness (§37) requires everything above to be addressable.** Lands last but design (§37.4 schema, §37.2 modes) should be locked early so other phases emit the right traces.
- **Existing tools (§24)** must keep working with semantic index *disabled* (config `semantic_index.enabled: false` in §25). Non-negotiable: this is the rollback story.

---

## MVP Definition

### Launch With (v1.10.0)

Minimum to ship a credible "Live Semantic Index" milestone.

- [ ] **Store + schema + snapshot writer** (§8, §9, §32 migration plan) — DuckDB schema, snapshot commit, schema versioning
- [ ] **Tree-sitter extraction for Go / TS+JS / Python** (§13) — first-class languages per spec
- [ ] **Stable symbol IDs across renames** (§11) — graph-foundational
- [ ] **Effective-read semantics** (§10) — overlay ⊕ snapshot single-API
- [ ] **Live update pipeline** (§16) — fsnotify + event coalescing + overlay writes + cache repair
- [ ] **LSP enrichment for hover/definition/references** (§14 subset) — minimum for confidence promotion
- [ ] **`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`** (§23.1–23.3) — control plane
- [ ] **`get_semantic_context`** (§23.4) — the headline retrieval tool
- [ ] **Integration: `get_repo_map` / `get_context` delegate to graph when present** (§24.1, §24.2) — preserves existing UX
- [ ] **Integration: edit tools emit `ChangeHelixEdit` events** (§24.4) — closes the write loop
- [ ] **`get_health` semantic section** (§24.5) — observability minimum
- [ ] **Guardrails G-001..G-005 with `warn` enforcement default** (§36.2) — covers the highest-risk operations (rename, delete, public API edit, large fuzzy edit, dependency edits)
- [ ] **Safety receipts schema + evaluator** (§36.4, §36.5) — even if only `warn`, the receipts must be emitted to enable later `enforce`
- [ ] **Single-projection PageRank (call graph)** (§18 subset) — multi-projection can wait
- [ ] **Eval harness skeleton with baseline / native / semantic modes** (§37.2 minus `semantic_guarded`) — proves the value
- [ ] **Pipeline DAG validation for bootstrap + indexing + live-update** (§39) — prevents Phase-56-style regressions
- [ ] **Configuration surface** (§25) — including `semantic_index.enabled: false` rollback switch
- [ ] **v1.9 carry-over: PKG-01 SC-3** — first signed release with real minisign keypair (blocking distribution; engineering-complete)

### Add After Validation (v1.10.x)

- [ ] **Multi-projection PageRank** (§18 full) — call / reference / file-dep with per-projection freshness — trigger: agents asking "rank by impact vs by relevance"
- [ ] **Clustering + `get_cluster_map` / `explain_cluster`** (§19, §23.7, §23.8) — trigger: positive eval signal on context-quality metrics
- [ ] **`find_related_symbols`, `get_change_impact_graph`, `explain_symbol_deep`, `validate_graph_edge`** (§23.5, 23.6, 23.9, 23.10) — trigger: §23.4 retrieval is stable
- [ ] **`semantic_guarded` eval mode** (§37.2) — trigger: G-001..G-005 receipts emitted reliably
- [ ] **Guardrails G-006..G-010** (§36.2) — trigger: G-001..G-005 produce no false positives in eval
- [ ] **Type resolution + access-chain resolver** (§38) — trigger: ≥1 dynamic-language workspace in eval suite shows graph-recall gap vs static-language workspaces
- [ ] **Comment-based type fallback (JSDoc/PHPDoc/YARD/Python comments)** (§38.6) — trigger: JS/Python real-repo eval shows annotation density >20%
- [ ] **Idle compaction** (§22) — trigger: overlay rows > N or snapshot age > T

### Future Consideration (v1.11+)

- [ ] **Java first-class extraction tier** — Helix already supports Java via jdtls; tree-sitter extraction tier is incremental work
- [ ] **Rust first-class extraction tier** — same rationale, deferred because rust-analyzer readiness signals are complex
- [ ] **PHP / Ruby type resolution with full doc-format fallback chains** — niche relative to JS/Python; defer until eval shows demand
- [ ] **Cross-repo semantic graph (multi-workspace clustering)** — Augment-style; large scope, defer until single-repo case is proven
- [ ] **`enforce` enforcement default for `review` profile** — only after `warn` mode proves no false positives in eval
- [ ] **Persistent receipts across sessions tied to immutable git commits** — interesting if `graph_version` ↔ commit-hash mapping becomes reliable
- [ ] **PKG-DEFER-03/04/05** — Homebrew tap, Scoop bucket, native Linux package — re-evaluate post-v1.10 release shape

---

## Feature Prioritization Matrix

| Feature | User Value | Cost | Priority | Justification |
|---|---|---|---|---|
| Effective-read (§10) | HIGH | HIGH | **P0** | Keystone for every other tool |
| Symbol identity (§11) | HIGH | HIGH | **P0** | Blocks rename detection in live updates |
| Live update pipeline (§16) | HIGH | HIGH | **P0** | Without this, "live" claim is false |
| Tree-sitter extraction Go/TS/JS/Py (§13) | HIGH | MEDIUM | **P0** | Existing v1.6 RepoMap pattern; proven |
| `get_semantic_context` (§23.4) | HIGH | MEDIUM | **P0** | The headline tool agents will reach for first |
| LSP enrichment for hover/def/refs (§14 subset) | HIGH | MEDIUM | **P0** | Confidence promotion; needed for evidence model |
| Edit-tool integration (§24.4) | HIGH | LOW | **P0** | Closes the write loop; cheap given existing tools |
| `get_health` semantic section (§24.5) | MEDIUM | LOW | **P0** | Operational must-have |
| Guardrails G-001..G-005 + receipts (§36) | HIGH | MEDIUM | **P0** | Defining differentiator; warn-mode is low-friction |
| Pipeline DAG validation (§39) | MEDIUM | LOW | **P0** | Prevents Phase-56-style regressions; cheap |
| Single-projection PageRank (§18 subset) | HIGH | MEDIUM | **P0** | Reuses v1.6 implementation |
| Eval harness baseline/native/semantic modes (§37 subset) | HIGH | HIGH | **P0** | Proves value; without it the milestone is unverifiable |
| Multi-projection PageRank (§18 full) | MEDIUM | MEDIUM | **P1** | Differentiator but `semantic` mode works with one projection |
| Clustering (§19) + cluster tools (§23.7/8) | MEDIUM | MEDIUM | **P1** | Differentiator; not blocking core retrieval |
| Type resolution + access chains (§38) | MEDIUM | HIGH | **P1** | Critical for dynamic languages but Go/TS get LSP coverage from v1 |
| `find_related_symbols`, `get_change_impact_graph` (§23.6, 23.9) | MEDIUM | LOW | **P1** | Cheap once core graph exists |
| `validate_graph_edge` (§23.10) | MEDIUM | LOW | **P1** | Cheap; high agent-trust value |
| Guardrails G-006..G-010 (§36.2) | MEDIUM | MEDIUM | **P1** | Lower-frequency operations |
| `semantic_guarded` eval mode (§37.2) | MEDIUM | LOW | **P1** | Reuses harness from P0 |
| `explain_symbol_deep` (§23.5) | MEDIUM | LOW | **P1** | Nice agent affordance |
| Comment-based type fallbacks (§38.6) | LOW (Go-heavy users) / HIGH (JS/Py users) | MEDIUM | **P2** | Defer until eval signal |
| Idle compaction (§22) | LOW | LOW | **P2** | Operational hygiene; not user-facing |
| Java/Rust first-class extraction | MEDIUM | HIGH | **P2** | jdtls/rust-analyzer LSP path already works; tree-sitter parity is incremental |
| Cross-repo / multi-workspace | LOW | HIGH | **P3** | Out of v1.10 scope |

---

## Competitor Feature Comparison (Direct)

| Feature | Sourcegraph SCIP | GitHub Stack Graphs | Meta Glean | Aider | Cursor / Cody | Helix v1.10 |
|---|---|---|---|---|---|---|
| Durable graph store | ✓ (SCIP files) | ✓ (per-repo) | ✓ (stacked DBs) | ✗ | partial (server-side) | ✓ (DuckDB) |
| Live overlay | ✗ | ✗ | append-layer (not live) | per-turn rebuild | ✗ (re-index cycle) | ✓ (§10) |
| Stable symbol IDs | ✓ | ✓ (path-based) | ✓ (vname) | ✗ | opaque | ✓ (§11) |
| Incremental update | ✓ (per-commit) | ✓ (per-commit) | ✓ (stack-layer) | ✓ (per-turn) | partial | ✓ (per-event) |
| Tree-sitter + LSP enrichment | per-language indexer | tree-sitter only | LSP-style facts | tree-sitter only | embeddings + LSP | ✓ (§13 + §14) |
| PageRank ranking | ✗ | ✗ | ✗ | ✓ (single-projection) | opaque | ✓ (multi-projection in P1) |
| Clustering | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ (P1, §19) |
| Tiered confidence on edges | ✗ (binary) | ✗ ("best-effort") | per-fact provenance | ✗ | ✗ | ✓ (§38) |
| Freshness exposed to agent | ✗ | ✗ | ✗ | ✗ | ✗ (implicit, by failure) | ✓ (every response) |
| Agent guardrails / receipts | n/a | n/a | n/a | n/a | hooks (syntactic) | ✓ (§36, semantic) |
| Eval harness comparing modes | n/a | n/a | n/a | external benchmarks | external benchmarks | ✓ (§37, baseline/native/semantic/semantic_guarded) |

---

## Sources

**Adjacent products:**
- [SCIP — a better code indexing format than LSIF (Sourcegraph blog)](https://sourcegraph.com/blog/announcing-scip)
- [SCIP Code Intelligence Protocol (GitHub)](https://github.com/sourcegraph/scip)
- [Indexing code at scale with Glean (Engineering at Meta)](https://engineering.fb.com/2024/12/19/developer-tools/glean-open-source-code-indexing/)
- [Glean (GitHub)](https://github.com/facebookincubator/Glean)
- [Introducing stack graphs (GitHub Blog)](https://github.blog/open-source/introducing-stack-graphs/)
- [Stack graphs: Name resolution at scale (Creager, arXiv)](https://arxiv.org/pdf/2211.01224)

**Agent-facing context retrieval:**
- [Aider Repository map docs](https://aider.chat/docs/repomap.html)
- [Building a better repository map with tree sitter (Aider)](https://aider.chat/2023/10/22/repomap.html)
- [Repository Mapping System (Aider DeepWiki)](https://deepwiki.com/Aider-AI/aider/4.1-repository-mapping-system)
- [How Cursor Actually Indexes Your Codebase (Towards Data Science)](https://towardsdatascience.com/how-cursor-actually-indexes-your-codebase/)
- [Sourcegraph Cody vs Cursor vs Augment Code (Augment)](https://www.augmentcode.com/tools/sourcegraph-cody-vs-cursor-vs-augment-code-for-enterprise-development)
- [Cursor vs Sourcegraph Cody: Embeddings and Monorepo at Scale (Augment)](https://www.augmentcode.com/tools/cursor-vs-sourcegraph-cody-embeddings-and-monorepo-scale)

**Guardrails for coding agents:**
- [Building guardrails for GitHub Copilot cloud agent (GitHub Docs)](https://docs.github.com/en/copilot/tutorials/cloud-agent/build-guardrails)
- [AI Coding Agent Security: Practical Guardrails for Claude Code, Copilot, and Codex (DEV)](https://dev.to/maxkrivich/ai-coding-agent-security-practical-guardrails-for-claude-code-copilot-and-codex-och)
- [Protecting from Dangerous AI Commands with Copilot CLI Hooks](https://thebrasstacksjournal.substack.com/p/protecting-yourself-from-dangerous)
- [Guardrails for Generative AI: Securing Developer Workflows (Microsoft)](https://techcommunity.microsoft.com/blog/azureinfrastructureblog/guardrails-for-generative-ai-securing-developer-workflows/4505801)

**Eval harness conventions:**
- [SWE-bench Leaderboards](https://www.swebench.com/)
- [SWE-bench Overview](https://www.swebench.com/SWE-bench/)
- [SWE-Bench Pro Public Leaderboard (Scale)](https://labs.scale.com/leaderboard/swe_bench_pro_public)
- [SWE-Bench Verified Leaderboard (llm-stats)](https://llm-stats.com/benchmarks/swe-bench-verified)

**Type resolution prior art (training-data MEDIUM confidence; not separately searched here because the spec §38 is opinionated and self-consistent):**
- pyright (Microsoft) — `strict`/`basic`/`off` tiers, structural narrowing
- Sorbet (Stripe) — `# typed: false/true/strict/strong` per-file gradient
- Phan / Psalm — PHP, doc-comment-driven inference
- TypeScript `tsserver` — full LSP with structural types

**SPEC source of truth:**
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/SPEC-DRAFT.md` (sections 8–24, 36–39 — the canonical contract this research maps to)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/PROJECT.md` (v1.10 milestone goal + Out of Scope guardrails)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/TOOL.md` (existing 41-tool baseline)

---

## Confidence Assessment

| Area | Level | Reason |
|---|---|---|
| Adjacent products' graph models (SCIP, Glean, Stack Graphs, Aider) | HIGH | Verified against official blogs/docs; well-documented designs |
| Cursor / Cody / Augment freshness behavior | MEDIUM | Closed-source; relying on vendor docs and third-party comparisons (Augment-authored comparisons noted as biased — used for technical claims, not value judgments) |
| Guardrail prior art (Copilot Cloud Agent, Claude Code hooks) | MEDIUM | Public docs cover capabilities, not specifically "semantic pre-check receipts" — Helix's design here is genuinely novel |
| Eval harness conventions (SWE-bench, Aider polyglot) | HIGH | Public leaderboards and benchmark repos |
| Type-resolution tiering precedent (pyright/sorbet) | MEDIUM | Training-data; not separately verified in this pass — the spec §38 design is self-consistent and doesn't claim to match any specific competitor's tier numbers |
| Mapping to SPEC-DRAFT.md sections | HIGH | Read directly from the spec |

---

*Feature research for v1.10 Live Semantic Index milestone — 2026-05-03*
