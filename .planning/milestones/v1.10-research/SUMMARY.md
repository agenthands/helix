# Project Research Summary — Helix v1.10 Live Semantic Index

**Domain:** Live, evidence-backed semantic graph layered onto a shipped LSP-backed code intelligence daemon (DuckDB committed snapshots ⊕ live overlay ⊕ LSP validation ⊕ agent-facing context tools ⊕ guardrails ⊕ eval harness)
**Researched:** 2026-05-03
**Confidence:** HIGH overall — STACK and ARCHITECTURE anchored in v1.9 source, FEATURES in adjacent-product docs, PITFALLS in shipped Helix bugs and upstream issues. MEDIUM on guardrail-receipt schemas (genuinely novel) and dynamic-language type-resolution conventions.

## Executive Summary

Helix v1.10 turns the v1.9 online LSP/tree-sitter capabilities into a durable, live, evidence-backed semantic graph. The decision-defining shape is **DuckDB committed snapshots + fsnotify live overlay + effective-read semantics + LSP revalidation queue**, exposed to agents as 10 new MCP tools, gated by a guardrail/receipt policy engine, and validated end-to-end by an out-of-process eval harness comparing four modes (`baseline / native / semantic / semantic_guarded`). The adjacent-product survey shows this combination is **not** the dominant pattern in any shipping product: SCIP/Glean/Stack-Graphs ship batch graphs without live overlay; Aider/Cursor/Cody ship live retrieval without a durable graph. **Freshness-as-API** (every response carries `freshness` + `score_status` + `pending_lsp_files`) and **semantic safety receipts** (destructive edits gated on prior `find_references`/`analyze_blast_radius` evidence) are genuine differentiators with no published prior art.

The recommended approach lands in 13 phases under a clean dependency order: store first → tree-sitter extraction + stable symbol IDs → live overlay → LSP enrichment → graph scores → clustering → 10 MCP tools → existing-tool integration via strangler fig → compaction → type resolution → guardrails → eval. New code lives in **Layer 1.5** (`internal/semantic/`, `internal/phasegraph/`, `internal/guardrails/`, `internal/eval/`), between the kernel and skills, with strict acyclic imports. Integration with kernel-edit and repomap is via injected callbacks (`postEditHook`, `SetSemanticLookup`) so the kernel never imports semantic. The DuckDB binding is `github.com/duckdb/duckdb-go v2.10502.0` (CGO=1 required, single-binary preserved via bundled static libs); the v1.9 Phase 51.1 stub policy extends to auto-disable `semantic_index.enabled` on CGO=0 builds.

Top risks concentrate in **state coherence under concurrency** (overlay × graph-cache × compaction race; fsnotify dropping watches on editor atomic-rename; PageRank score instability while LSP enrichment lands asynchronously) and **agent-shaped failure modes** (receipts lost to context compaction, eval prompt leakage to commercial LLMs). Mitigations are concrete and architectural: epoch-CAS contract between overlay writes and compaction, directory-watching with content-hash scrub, `graph_version` + monotone-frontier rule on tool envelopes, server-side receipt store with ID-only forwarding. **Must-not-regress invariants from v1.9** — `LazyInit`-last middleware order, single canonical `GrammarRegistry`, structured LS readiness gates (rust-analyzer `experimental/serverStatus`, jdtls `WaitUntilJavaReady`), bounded-label metrics with no source content — all carry forward and are explicitly preserved.

## Key Findings

### Recommended Stack (additions only; v1.9 stack fixed)

- **`github.com/duckdb/duckdb-go` v2.10502.0** (DuckDB 1.5.2): fact-store driver, official post-donation repo, `database/sql` + Appender, bundled static libs cover the 6-archive matrix. **CGO=1 required** — extends Phase 51.1 stub policy.
- **`gonum.org/v1/gonum` v0.16.0 test-only**: oracle for hand-rolled PageRank/components; never imported from production per ADR-005.
- **`github.com/tiktoken-go/tokenizer` v0.6.x**: pure-Go embedded vocab; rejected `pkoukk/tiktoken-go` because it network-downloads vocab.
- **`github.com/bluekeyes/go-gitdiff` v0.8.x**: pure-Go patch parse + apply; rejected `sourcegraph/go-diff` as parser-only.
- **`fsnotify` v1.9.0** (already vendored): kept; build recursive walker, atomic-rename re-attach, `ENOSPC` degraded-mode in-tree.
- **Pipeline DAG**: stdlib only (~80 LOC Kahn's algorithm).
- **Type resolution / fixpoint**: stdlib only; comment parsers regex-tractable.

**CGO posture (load-bearing):** `semantic_index.enabled=true` requires CGO=1. CGO=0 builds compile via `//go:build !cgo` stubs that auto-return `Kind: Unsupported`. Daemon must auto-set `cfg.SemanticIndex.Enabled = false` when `treesitter.Available == false`. **What NOT to pull in:** any KG/RDF stack, embedding/vector libs, heavyweight DAG/workflow engines, gonum-in-production, recursive-watch wrappers.

### Expected Features

**Must have (table stakes):** durable queryable symbol+reference graph with stable IDs across renames; incremental update without full reindex; cross-file find-references / call-hierarchy; token-budgeted ranked context; tree-sitter-first extraction with LSP enrichment; PageRank importance ranking; indexing-progress + status; diagnostics integration after edits.

**Should have (differentiators):**
- Effective-read semantics (snapshot ⊕ overlay) — addresses Cursor's top complaint
- First-class freshness on every response
- Tiered confidence on edges with explicit evidence kinds (1.00 LSP / 0.90 annotation / … / 0.20 unknown) + `validation_state`
- Live LSP revalidation queue with priority boosts on edit
- Multi-projection PageRank with per-projection freshness
- Cluster maps + cluster explanations
- Guardrail policy engine + safety receipts (semantic correlation across tool calls; novel)
- Eval harness with 4 modes
- Type resolution with bounded fixpoint for dynamic languages (capped, comment-fallback ≤ 0.60)
- `validate_graph_edge` (agent challenges the index)
- Typed pipeline DAG with phase validation at startup

**Defer (v1.10.x / v1.11+):** multi-projection PageRank, clustering tools, P1 retrieval companions (`find_related_symbols` / `get_change_impact_graph` / `explain_symbol_deep` / `validate_graph_edge`), `semantic_guarded` mode + G-006..G-010, type resolution + access chains, comment-based fallback, idle compaction tuning, Java/Rust first-class extraction, cross-repo graphs, v1.9 carry-over (PKG-01 SC-3, distros, Phase 51 reproducibility scope, Phase 55 forwarder span).

**Anti-features (refusals):** vector/embedding search; RDF/SPARQL KG; "real-time across whole repo" marketing; auto-execute when guardrails pass; in-process LSP servers; soundness-grade type inference; SWE-bench leaderboard chase; LLM-generated edge explanations; persistent receipts detached from `graph_version`; full-rebuild on every Helix start.

### Architecture Approach

**Layer 1.5 placement** between kernel and skills. Strict acyclic imports — kernel never depends on semantic; coupling inverted via callbacks.

**Major components:**
1. **Semantic Service** owns `*sql.DB` for DuckDB at `<workspace>/.helix/semantic.duckdb`; opens at new daemon bootstrap step **2.5** between language registry and kernel creation; disable path returns `nil`, all callsites guard.
2. **Live Update Pipeline** — fsnotify directory-watcher with atomic-rename re-attach + content-hash scrub; coalescer (250ms debounce, 200-event bulk threshold); overlay writer with `overlay_epoch`; idle compaction goroutine.
3. **LSP Enrichment Worker** — single goroutine with `container/heap` priority queue; reuses `kernel.Pool().AcquireLease(...)` via small `LeaseAcquirer` interface (no kernel import); honors v1.9 readiness gates.
4. **Graph + Rank + Cluster** — in-memory cache loaded from DuckDB; hand-rolled multi-projection weighted PageRank with personalized restart and incremental local repair.
5. **MCP Tools + Skill** — 10 new tools registered via existing skill adapter pattern (new daemon step 13.5).
6. **Existing-Tool Integration (strangler fig)** — `repomapSkill.SetSemanticLookup(lookup)` mirrors `SetEnrichFn`; `get_repo_map` / `get_context` / `analyze_blast_radius` / `get_health` consult semantic when available, fall back to v1.9. Zero source change to `internal/repomap` engine.
7. **Edit-Tool Hook (callback inversion)** — `edit.RegisterTools` gains `postEditHook func(ctx, []string, source)`; daemon wires `d.semantic.LiveQueue().EnqueueChangeEventsFunc()`. Threaded through `fileops.replace_in_file` / `fuzzy_edit` too. `nil`-safe.
8. **Guardrail Middleware** — installed at new step **14b.5** between Suggestion and LazyInit. Execution order: `LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`. **LazyInit MUST remain installed last (executes first).** Telemetry gains `guardrail_blocked` / `guardrail_warned` outcome classes.
9. **Pipeline DAG** — strangler fig: stdlib library used immediately for new graphs (semantic indexing, live update, eval); imperative `daemon.New` bootstrap stays with `// TODO(v1.11): migrate` marker.
10. **Eval Harness** — out-of-process by default; subprocess `helix daemon` per task with isolated config; agent (Anthropic / DeepSeek refactored from `test/oracle/llm/`) over stdio forwarder; in-process variant for `make eval-quick`. Each mode is a config preset. New `--profile=baseline` strips Helix tools entirely.

**Shutdown ordering:** semantic.Run under daemon errgroup; on cancel: stop accepting events → drain coalescer → drain LSP queue → flush overlay (no compaction) → close DuckDB → kernel shuts down LS workers (v1.9 ordering preserved last).

**DuckDB cross-process rule:** all store access via daemon. CLI subcommands and eval harness MUST NOT reopen the `.duckdb` file (DuckDB single-writer). Lock with vet rule forbidding `duckdb-go` import outside `internal/semantic/store/`.

### Critical Pitfalls (top 5; full set in PITFALLS.md)

1. **C1 — Overlay × graph-cache × compaction race.** Three writers, three independent monotonic clocks, no single linearization point; ClearOverlay can drop overlay rows committed after MergeBaseAndOverlay. **Mitigation:** epoch-CAS — compaction snapshots `overlay_epoch`, ClearOverlay deletes rows ≤ captured epoch. Property test in P8.
2. **C2 — fsnotify silent watcher death on atomic-rename.** Vim/JetBrains/VS Code save patterns kill inode-attached watchers; Linux inotify per-user limit blows up on >8k-dir repos. **Mitigation:** watch directories not files; eager re-attach on RENAME/REMOVE; required (not best-effort) 5-min content-hash scrub; ENOSPC → manifest-poll fallback per workspace; surface in `get_semantic_graph_status`.
3. **C3 — PageRank score instability while LSP enrichment lands.** Async confidence promotion (0.4 → 1.0) re-ranks the graph between calls. **Mitigation:** every ranked envelope returns `graph_version` + `enrichment_level`; default to last-committed-snapshot scores; bound incremental repair frontier (`max_repair_nodes`, fail-closed); determinism test (NodeID-sorted tiebreak).
4. **C8 — LSP enrichment death spiral.** Post-`git checkout` flood buries jdtls (just stabilized in Phase 56); interactive p95 climbs as Helix indexes more. **Mitigation:** revalidation queue strictly lower priority via separate token bucket + concurrency cap (default 1 worker); interactive deadline preempts; do NOT flood after `bulk_update` (lazy-on-tool-call drives enrichment); honor v1.9 readiness gates before enqueueing.
5. **C7 — Stable symbol ID drift on rename / generics / overloads / anonymous symbols.** Breaks `analyze_blast_radius`, `get_change_impact_graph`, `find_related_symbols`. **Mitigation:** key contract `repo_id || package_path || enclosing_chain || name || arity || receiver_type` — explicitly NOT line/column, NOT `signature_hash`, NOT generic type-args. AST-shape hash for anonymous. 30+ before/after test matrix per language. Lock contract in P1.

**Other notable risks:** C5 cross-process DuckDB lock (architectural rule + vet lint); C6 DuckDB JSON growth (CHECKPOINT + weekly VACUUM, evidence size cap); M1 overlay rows lost in compaction crash (single-tx OR `compaction_journal`); M4 receipts lost to context compaction (server-side receipt store, ID-only forwarding); M5 eval prompt leakage (synthetic-only corpora, retention-zero); M6 metrics cardinality (extend v1.2 bounded-label CI lint); M7 fallback path drift (`source` field in every envelope; index-disabled goldens).

### Must-Not-Regress Invariants (v1.9 carryover)

- **Middleware install LIFO order:** `LazyInit` MUST remain installed last (executes first). Guardrail inserts at 14b.5 between Suggestion and LazyInit.
- **Single canonical `GrammarRegistry`** (BUG-04, Phase 49) injected from daemon bootstrap; semantic extractors consume the same instance.
- **Structured LS readiness gates** (BUG-02 rust-analyzer `experimental/serverStatus`, Phase 56 jdtls `JdtlsAdapter.WaitUntilJavaReady`, `Worker.Start` `OnNotification` wiring) — semantic LSP enrichment honors these.
- **Bounded-label metrics with no source content** in metrics or traces — extend v1.2 cardinality allowlist; PromQL validator (registry-driven, fail-closed) covers new families.
- **Single-binary + CGO=0 stub policy** — daemon refuses semantic with `Kind: Unsupported` and remediation text.

## Implications for Roadmap

13 phases honoring the dependency order from ARCHITECTURE.md "Suggested build order". Deviations from SPEC §32: pipeline DAG library split out as P0.5 so P1/P2/P10 can consume it; type resolution (P11) moved between P8 and P9 (improves edge precision; doesn't gate other phases).

### Phase P0: Schema & Store + Config + Cardinality
**Rationale:** Foundation; locks cross-process access rule (C5), JSON-vs-typed-column discipline (C6), cardinality allowlist (M6) before any consumer depends on the store.
**Delivers:** `internal/semantic/{store,types,config}`; DuckDB schema; schema versioning; `semantic_index.enabled` config + 4-layer precedence; vet rule forbidding `duckdb-go` outside `internal/semantic/store/`; CGO=0 stub.
**Avoids:** C5, C6, M5, M6.

### Phase P0.5: Pipeline DAG Library
**Rationale:** Consumed by P1/P2/P10 planners. Cheap (~80 LOC stdlib).
**Delivers:** `internal/phasegraph/`.
**Avoids:** m1 half-migration (bootstrap stays imperative with TODO(v1.11) marker).

### Phase P1: Tree-sitter Extraction + Stable Symbol IDs
**Rationale:** Symbol identity is the foundational graph contract — must lock before live overlay or any consumer.
**Delivers:** `internal/semantic/{extract,resolve}`; Go / TS+JS / Python first-class extraction; key contract; 30+ before/after test matrix per language.
**Avoids:** C7.
**Research flag:** language-specific edge cases (Go generics, TS overloads, Python decorators).

### Phase P2: Live Overlay + Watcher
**Rationale:** First daemon integration; introduces `postEditHook`. Must define epoch contract before P8 depends on it.
**Delivers:** `internal/semantic/live/*`; directory-watcher + atomic-rename re-attach + ENOSPC fallback + scrub; coalescer; overlay writer with `overlay_epoch`; `postEditHook` wired into `edit.RegisterTools` and `fileops.RegisterTools`; daemon bootstrap step 9.5.
**Avoids:** C1 race, C2 watcher misses, M2 bulk threshold.
**Research flag:** fsnotify edge cases across editors require concrete fixture validation.

### Phase P3: LSP Enrichment Worker
**Rationale:** Depends on lspool lease API; consumes v1.9 readiness gates.
**Delivers:** `internal/semantic/lspenrich/*`; priority queue; `LeaseAcquirer` interface (no kernel import); concurrency cap; readiness-gate integration.
**Avoids:** C8 death spiral; preserves BUG-02 / Phase 56 invariants.

### Phase P4: Graph Scores (single-projection MVP)
**Rationale:** Reuses v1.6 RepoMap PageRank; multi-projection deferred.
**Delivers:** `internal/semantic/{graph,rank}/*`; weighted PageRank with shared `applyDanglingMass`; `graph_version`; bounded incremental repair; determinism test.
**Avoids:** C3, C4.

### Phase P5: Clustering (defer to v1.10.x)
**Delivers:** `internal/semantic/cluster/*`; cluster snapshot keyed on `graph_version`.
**Avoids:** C3 (cluster drift mirrors score drift).

### Phase P6: 10 New MCP Tools
**Rationale:** Existing skill registration pattern; `get_semantic_context` is the headline tool.
**Delivers:** `internal/semantic/tools/*` + `internal/skill/semantic/*`; P0 set of 4 (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`); remaining 6 in v1.10.x; `freshness` field on every response; stable-key tiebreak.
**Avoids:** M8 non-determinism.

### Phase P7: Existing-Tool Integration (strangler fig)
**Rationale:** Lowest-risk integration — zero source change to `internal/repomap` engine.
**Delivers:** `repomapSkill.SetSemanticLookup(lookup)`; `get_repo_map` / `get_context` / `analyze_blast_radius` / `get_health` consult semantic-when-available with automatic v1.9 fallback; `source` field in result envelopes; index-disabled goldens preserved.
**Avoids:** M7.

### Phase P8: Compaction & Retention
**Rationale:** Honors P2 epoch contract.
**Delivers:** idle-debounced compaction; `CHECKPOINT` end-of-compact; weekly `VACUUM` (config-gated); single-tx commit OR `compaction_journal`; long-repo bench fixture.
**Avoids:** C1 (epoch CAS), M1, C6.

### Phase P11: Type Resolution (out-of-order, between P8 and P9)
**Rationale:** Improves edge precision; doesn't gate other phases. Comment-fallback work load-bearing for JS/Python eval signal.
**Delivers:** `internal/semantic/typeresolve/*`; bounded fixpoint with hard iteration cap (`max_fixpoint_iters=8`) + early-exit on no progress; conservative emission on non-convergence; comment-derived evidence capped at 0.7.
**Avoids:** M3, m3.
**Research flag:** comment-format grammar subset (JSDoc / PHPDoc / YARD / Python typing comments).

### Phase P9: Guardrails + Middleware
**Rationale:** Depends on graph + receipts; must come after graph scores so policy can read freshness/`score_status`.
**Delivers:** `internal/guardrails/*`; `internal/mcp/guardrail_middleware.go`; daemon step 14b.5; G-001..G-005 with `warn` default; server-side receipt store (5-min TTL + `graph_version` invalidation); ID-only forwarding; destructive tools look up claims server-side; telemetry outcomes; `DoD.md` + `GUARDRAILS.md`.
**Avoids:** M4.
**Research flag:** receipt-schema design has no published prior art.

### Phase P10: Eval Harness
**Rationale:** Depends on a working semantic stack + guardrails.
**Delivers:** `internal/eval/*` + `internal/cli/eval/*`; subprocess daemon orchestration with isolated config dirs; agent over stdio forwarder; 4 modes; cross-model judging (informational, never CI gate); synthetic-only corpora + OSS Helix repo; retention-zero provider config; `make eval-quick` in-process.
**Avoids:** M5, m4.
**Research flag:** task-class DoD definitions and tool-behavior scoring rubric calibration; provider data-retention TOS re-verification.

### Phase P12: Pipeline DAG bootstrap migration → v1.11
v1.10 lands `phasegraph` library at P0.5 and uses it for new graphs only. Migrating imperative `daemon.New` 16-step bootstrap touches every test that constructs a daemon; deferred per strangler fig.

### v1.9 Carry-over (folded into v1.10)

- **PKG-01 SC-3** — cut first signed release with real minisign keypair (deployment-gated; engineering-side checks already pass).
- **PKG-DEFER-03/04/05** — Homebrew tap, Scoop bucket, native Linux package; re-scope based on first signed release feedback.
- **Phase 51 reproducibility-gate architectural fix** — real-release-vs-Pass-3 OR CONTRIBUTING.md wording softening.
- **Phase 55 `forwarder.tools.call` span unification** with gRPC server span.

These four items can land in parallel with the early P0–P3 semantic phases (different files, different reviewers) — schedule before semantic stack is feature-complete to avoid release risk concentration.

### Research Flags

**Phases needing deeper research during planning:**
- **P1:** stable symbol ID edge cases per language (overloads, generics, anonymous closures, decorators).
- **P2:** fsnotify atomic-rename behavior across Vim/JetBrains/VS Code; ENOSPC fallback ergonomics.
- **P9:** safety-receipt schema is novel; design must hold up against agent context-compaction and `enforce`-mode false-positive rate.
- **P10:** task-class DoD definitions and scoring rubric; provider data-retention TOS re-verification.
- **P11:** comment-format grammar subset selection.

**Standard patterns (skip deep research):** P0, P0.5, P3, P4 (MVP), P6, P7, P8.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | DuckDB binding, fsnotify gaps, gonum-vs-hand-rolled, eval libs verified at file/issue level. MEDIUM only on `tiktoken-go/tokenizer` and `bluekeyes/go-gitdiff` (READMEs not deep-verified). |
| Features | HIGH on adjacent products + eval conventions; MEDIUM on Cursor/Cody/Augment freshness behavior (closed source); LOW on guardrail-receipt schema (no prior art). |
| Architecture | HIGH | Verified against `internal/daemon/daemon.go`, SPEC §5/6/24/36/39, v1.9 callback patterns. MEDIUM on test-harness `:memory:` ergonomics and forwarder gRPC propagation for eval subprocess. |
| Pitfalls | HIGH for items grounded in shipped Helix bugs (BUG-01..04, Phase 56) and upstream issues (fsnotify #17/#80/#214/#254/#372, DuckDB #77/#4899); MEDIUM on cross-product war stories. |

**Overall confidence:** HIGH. Decision-defining choices (Layer 1.5 placement, DuckDB CGO posture + auto-disable, postEditHook callback, lookup-callback inversion for repomap, guardrail middleware position 14b.5, strangler-fig phasegraph for new graphs only, out-of-process eval harness) all anchored in v1.9 source and SPEC contracts. Unknowns isolated to novel surfaces (guardrail receipts, eval scoring rubric) and dynamic-language type-resolution edges.

### Gaps to Address During Planning

- **DuckDB driver canonical path.** STACK.md notes `marcboeker/go-duckdb` → `duckdb/duckdb-go` donation; ARCHITECTURE.md still references the legacy path. Lock import path during P0.
- **Test harness DuckDB strategy.** `test/harness/Runner` constructs daemons in-process; with semantic enabled every test creates a `.duckdb` file unless `:memory:` is used. Confirm at P0.
- **Forwarder gRPC propagation of semantic config** for eval subprocess (likely `HELIX_CONFIG_PATH`); confirm at P10.
- **Bootstrap step renumbering convention.** Decimal additions (2.5 / 12e–12i / 14b.5) cross 16+ steps; reviewer ergonomics may justify a one-time renumber.
- **Receipt TTL × `graph_version` invalidation interplay.** Long-running guardrail-warned operations need P9 prototype.
- **Eval task corpus.** Synthetic-only is the threat-model rule but has lower discriminating power; OSS Helix repo + permissively-licensed SWE-bench Verified subset are options. Lock at P10.
- **Provider data-retention shifts.** Re-verify Anthropic/OpenAI/DeepSeek TOS at P10 implementation.

## Sources

**Primary (HIGH):** `SPEC-DRAFT.md` (40 sections), `.planning/PROJECT.md`, `internal/daemon/daemon.go` (lines 199-203, 241-255, 296-336, 349-380, 481-525, 765-825), `internal/mcp/lazy_init.go:106-108`, `CLAUDE.md`. [duckdb/duckdb-go](https://github.com/duckdb/duckdb-go), [DuckDB Concurrency](https://duckdb.org/docs/current/connect/concurrency), [fsnotify](https://github.com/fsnotify/fsnotify) issues #17/#80/#214/#254/#372, [gonum graph/network](https://pkg.go.dev/gonum.org/v1/gonum/graph/network), [SCIP](https://sourcegraph.com/blog/announcing-scip), [Glean](https://engineering.fb.com/2024/12/19/developer-tools/glean-open-source-code-indexing/), [Stack graphs](https://github.blog/open-source/introducing-stack-graphs/), [Aider repomap](https://aider.chat/docs/repomap.html), [SWE-bench](https://www.swebench.com/).

**Secondary (MEDIUM):** [tiktoken-go/tokenizer](https://github.com/tiktoken-go/tokenizer), [bluekeyes/go-gitdiff](https://github.com/bluekeyes/go-gitdiff) (READMEs only), [Cursor indexing](https://towardsdatascience.com/how-cursor-actually-indexes-your-codebase/), [Copilot guardrails](https://docs.github.com/en/copilot/tutorials/cloud-agent/build-guardrails), pyright/sorbet/Phan/Psalm tier conventions.

**Tertiary (LOW — needs validation):** Sourcegraph zoekt / gopls overlay-correctness post-mortems; Anthropic/OpenAI/DeepSeek TOS (shifting); guardrail-receipt prior art (none found, novel design needs P9 prototype).
