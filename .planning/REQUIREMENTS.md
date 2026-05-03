# Requirements: Helix v1.10 — Live Semantic Index

**Defined:** 2026-05-03
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Source of truth:** `SPEC-DRAFT.md` (40 sections, 12 SPEC-internal phases). Research synthesis: `.planning/research/SUMMARY.md`.

---

## v1.10 Requirements

Each requirement is testable from an agent/user perspective and maps to one roadmap phase.

### STORE — Semantic Fact Store (DuckDB)

- [ ] **STORE-01**: A semantic fact store opens at daemon start when `semantic_index.enabled=true` and persists at `<workspace>/.helix/semantic.duckdb`; daemon refuses to start if the file is corrupt only after auto-quarantining the bad file (`.corrupt.<ts>` rename) and rebuilding fresh.
- [ ] **STORE-02**: When `semantic_index.enabled=false` (or implied false because CGO=0 build has no tree-sitter), every semantic-dependent tool returns `Kind: Unsupported` with remediation text and the rest of Helix continues to work — verified by an integration test running with `semantic_index.enabled=false`.
- [ ] **STORE-03**: Schema versioning is enforced: a forward-incompatible schema version triggers a full reindex with a clear log message; a backward-compatible bump migrates in place. Schema version is stamped in every snapshot row.
- [ ] **STORE-04**: Effective-read API returns `committed snapshot ⊕ live overlay − tombstones` for files, symbols, references, and edges; verified by a per-entity unit test where overlay and snapshot disagree.
- [ ] **STORE-05**: Configuration is layered through the existing 4-layer precedence (CLI > project `.helix/project.yml` > user `~/.helix/helix_config.yml` > profile defaults) for every `semantic_index.*` key in SPEC §25.
- [ ] **STORE-06**: A vet/lint rule fails the build if `duckdb-go` is imported from any package other than `internal/semantic/store/`.

### EXTRACT — Tree-sitter Extraction & Stable Symbol Identity

- [ ] **EXTRACT-01**: Tree-sitter extraction produces typed symbols, references, imports, and syntax edges for **Go**, **TypeScript / JavaScript**, and **Python** files inside the workspace; non-supported languages emit a `partial:true` extraction marker.
- [ ] **EXTRACT-02**: Stable symbol IDs use the contract from SPEC §11.1 (LSP identity → package/module path + owner path + qualified name + kind + signature hash → file path fallback). Identity must survive whitespace-only changes, file-path renames where content hash is unchanged, and exported-symbol moves where qualified name is unchanged.
- [ ] **EXTRACT-03**: A 30+ before/after test matrix per first-class language exercises stable-key behavior across overload signatures, generics, anonymous closures, decorators (Python), and method-on-receiver renames; identity transitions are explicit.
- [ ] **EXTRACT-04**: Tree-sitter and LSP facts merge per SPEC §11.2 with the documented confidence ladder (1.00 LSP-confirmed, 0.95 merged, 0.80 ts+local, 0.70 ts-only, 0.45 heuristic).
- [ ] **EXTRACT-05**: Extraction reuses the single canonical `GrammarRegistry` injected from daemon bootstrap (no duplicate registries, BUG-04 invariant preserved).

### LIVE — Live Update Pipeline

- [ ] **LIVE-01**: A directory-level fsnotify watcher observes the workspace, debounces 250ms, coalesces per SPEC §16.2 (modified+modified→modified, created+deleted→no-op, etc.), and writes structured upserts/tombstones into the live overlay.
- [ ] **LIVE-02**: Atomic-rename save patterns from Vim, JetBrains, and VS Code do not silently kill watching — verified by an editor-fixture test that performs each editor's save dance and asserts the overlay caught the change.
- [ ] **LIVE-03**: A required (not best-effort) periodic content-hash scrub catches missed events; status is exposed via `get_semantic_graph_status`.
- [ ] **LIVE-04**: Linux `inotify` `ENOSPC` falls back gracefully to a manifest-poll mode per workspace with a user-visible warning; semantic queries still answer with stale-allowed reads.
- [ ] **LIVE-05**: Bulk-change events above `live_updates.bulk_change_threshold` (default 200) collapse to a single `bulk_update` event that schedules an incremental snapshot rebuild rather than thrashing the overlay.
- [ ] **LIVE-06**: An overlay write transaction carries a monotonic `overlay_epoch` that compaction (COMPACT-01) reads under CAS — no overlay rows committed after compaction's snapshot read are dropped by `ClearOverlay`.
- [ ] **LIVE-07**: After every successful `replace_symbol_body`, `insert_before/after_symbol`, `rename_symbol`, `safe_delete_symbol`, `replace_in_file`, and `fuzzy_edit`, the edited files emit a `ChangeHelixEdit` event into the live queue via a `postEditHook` callback (kernel does not import semantic).

### ENRICH — LSP Enrichment

- [ ] **ENRICH-01**: An async LSP enrichment worker reuses the existing `kernel.Pool().AcquireLease(...)` API via a small `LeaseAcquirer` interface; semantic does not import `internal/kernel` (only `internal/kernel/lspool` types and `internal/workspace`).
- [ ] **ENRICH-02**: Enrichment uses a priority queue: foreground tool calls preempt; high-priority `ChangeHelixEdit` revalidation runs head-of-line; background indexing runs at concurrency cap = 1 by default.
- [ ] **ENRICH-03**: Enrichment honors v1.9 LS readiness gates — rust-analyzer `experimental/serverStatus` and jdtls `JdtlsAdapter.WaitUntilJavaReady` — before issuing enrichment requests; never bypasses readiness for "best-effort" reasons.
- [ ] **ENRICH-04**: Per-file enrichment respects the SPEC §14.2 budget (`timeout_per_file=5s`, `timeout_total=120s`, `max_symbols_per_file=200`, etc.); on budget exhaustion the file is marked `partial:true, partial_reason:"budget exhausted"` and remains queryable.
- [ ] **ENRICH-05**: A `git checkout` storm or 100-file edit burst does not bury foreground tool calls — verified by a stress test asserting interactive p95 stays under the foreground-tool budget while enrichment runs.

### GRAPH — Graph Engine & Ranking

- [ ] **GRAPH-01**: A weighted PageRank single projection (CALL_GRAPH) computes deterministically over the effective graph; same input across runs produces byte-identical scores.
- [ ] **GRAPH-02**: Personalized PageRank with seed weighting from a request payload (files + symbol names) returns a ranked node list under a configurable epsilon (`pagerank.epsilon`, default `1e-6`).
- [ ] **GRAPH-03**: Each ranked tool response carries `graph_version` and `enrichment_level` so callers can detect drift between successive calls; tiebreaks use stable-key NodeID ordering.
- [ ] **GRAPH-04**: Incremental local PageRank repair runs only inside a bounded `max_local_pagerank_nodes` frontier (default 5000); above the threshold scores are marked `stale` and a full recompute is scheduled after idle.
- [ ] **GRAPH-05**: Score persistence carries `status` (`exact | approximate | stale | missing`); MCP responses surface this in `score_status`.
- [ ] **GRAPH-06**: A weak-component pass over the effective graph supports cluster identification (the algorithm is shipped; cluster MCP tools — `get_cluster_map`, `explain_cluster` — are deferred to v1.10.x). The pass is deterministic.

### TOOLS — New MCP Tools (P0 set: 4 of 10)

- [ ] **TOOL-01**: `index_semantic_graph` MCP tool (mode `review+` / `admin`) builds or refreshes a committed snapshot; supports `auto`, `full`, `incremental`, `refresh` modes; returns snapshot id, graph version, files indexed/reused, partial state, freshness, duration.
- [ ] **TOOL-02**: `refresh_semantic_graph` MCP tool (mode `read+`) applies pending live source changes without forcing a full reindex; supports `wait_for_lsp` and `paths` filters; returns graph version, files updated, deltas, pending LSP, freshness.
- [ ] **TOOL-03**: `get_semantic_graph_status` MCP tool (mode `read+`) returns the SPEC §23.3 status object: latest snapshot id, graph version, overlay state, pending LSP count, freshness, per-projection score status, cluster status, last-live-update latency.
- [ ] **TOOL-04**: `get_semantic_context` MCP tool (mode `read+`) returns ranked, evidence-backed context for a task/symbol/file selection under a token budget; response always includes `freshness_mode`, `graph_version`, `overlay_active`, `freshness`, `pending_lsp_files`, and per-candidate `evidence` + `confidence`.
- [ ] **TOOL-05**: All four tools respect profile/mode gating per SPEC §30.2; `tools/list` filters them out for profiles that don't include them; `get_tool_help` returns parameter docs for each.

> Deferred to v1.10.x: `explain_symbol_deep`, `find_related_symbols`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`, `validate_graph_edge`. Multi-projection PageRank also deferred.

### INTEG — Existing-Tool Integration (Strangler Fig)

- [ ] **INTEG-01**: `get_repo_map` consults `repomapSkill.SetSemanticLookup(lookup)` when available and uses persisted graph scores + clusters; falls back to existing tree-sitter + PageRank path when semantic is disabled, building, or returns an error. Zero source change to `internal/repomap` engine.
- [ ] **INTEG-02**: `get_context` delegates to the semantic retrieval engine when available with the same fallback contract.
- [ ] **INTEG-03**: `analyze_blast_radius` uses semantic graph expansion + LSP validation of critical edges when available; returns `confidence` and `evidence` per impacted node; falls back when semantic is disabled.
- [ ] **INTEG-04**: `get_health` includes a `semantic_index` section with store kind, latest snapshot status, graph version, overlay active flag, pending LSP count, last live-update latency, and last error.
- [ ] **INTEG-05**: Every MCP envelope from a semantic-aware tool returns a `source` field (`semantic | tree_sitter | fallback`) so callers can detect path drift; index-disabled goldens are preserved.

### COMPACT — Compaction & Retention

- [ ] **COMPACT-01**: A compaction worker merges the live overlay into a new committed snapshot after `compact_after_idle_ms` (default 5000) when no edit/overlay/LSP transaction is active; previous committed snapshot is preserved on failure.
- [ ] **COMPACT-02**: Compaction reads the live `overlay_epoch` under CAS; `ClearOverlay` deletes only rows ≤ captured epoch — overlay rows committed during compaction are retained for the next pass. Verified by a property test ("interleave overlay writes with compaction").
- [ ] **COMPACT-03**: A `CHECKPOINT` runs at compaction commit; a config-gated weekly `VACUUM` reclaims DuckDB on-disk space; long-repo bench fixture confirms growth bounded.
- [ ] **COMPACT-04**: Snapshot retention keeps the last `snapshot_retention` snapshots (default 5); older snapshots are deleted in a single transaction.
- [ ] **COMPACT-05**: Compaction crash recovery: a single transaction OR a `compaction_journal` ensures partial commits leave overlay+snapshot consistent. Verified by a kill-mid-compact integration test.

### TYPES — Type Resolution & Access Chains

- [ ] **TYPES-01**: Tiered type-resolution emits `RESOLVES_TO`, `CALLS`, and `USES_TYPE` edges with confidence per the SPEC §38.2 ladder (1.00 LSP / 0.90 annotation / 0.80 constructor / 0.70 assignment / 0.60 doc-comment / 0.45 heuristic / 0.20 unknown).
- [ ] **TYPES-02**: Access-chain resolver supports `a.b.c.d()` patterns up to `max_chain_depth` (default 8); a fixpoint loop runs up to `max_fixpoint_iterations` (default 8) and exits early on no progress.
- [ ] **TYPES-03**: Comment-based fallbacks parse JSDoc / TSDoc / PHPDoc / YARD / Python type comments where enabled; comment-derived edges never exceed confidence 0.60 unless independently confirmed by LSP.
- [ ] **TYPES-04**: Non-converged chains are emitted with low confidence + `unresolved` markers; they are NEVER emitted as `validated`.

### GUARD — Agent Guardrails (G-001..G-005, warn-default)

- [ ] **GUARD-01**: A `GuardrailMiddleware` is installed at daemon step 14b.5 between `SuggestionMiddleware` and `LazyInitMiddleware`; LIFO execution order is `LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`. The `LazyInit-last-installed (executes-first)` invariant is preserved.
- [ ] **GUARD-02**: Five guardrail rules are enforced at warn-default: G-001 no rename-by-grep, G-002 no delete without reference check, G-003 no public API edit without blast-radius analysis, G-004 no large fuzzy edit without prior `read_file`/`get_context`, G-005 no security-sensitive change without diagnostics after edit.
- [ ] **GUARD-03**: Safety receipts are stored server-side in the daemon (5-min TTL keyed by `graph_version`); destructive tools look up claims by ID — receipts are never persisted client-side. ID-only forwarding survives agent context compaction.
- [ ] **GUARD-04**: Receipt invalidation is automatic when `graph_version` advances past the receipt's snapshot; a stale receipt fails the guardrail check with a clear "graph drifted" error.
- [ ] **GUARD-05**: `TelemetryMiddleware` classifies guardrail outcomes as `guardrail_blocked` and `guardrail_warned` (alongside existing `success / timeout / circuit_open / internal`); RED metrics surface them.
- [ ] **GUARD-06**: `GUARDRAILS.md` and `DoD.md` are committed and explain rule semantics + completion criteria for rename / delete / public-API-change task classes.
- [ ] **GUARD-07**: Enforcement levels (`off | warn | require_force | enforce`) are configurable per profile; default is `warn` for `read`/`edit` profiles and `require_force` for `review` profile on high-risk pre-checks.

> Deferred to v1.10.x: G-006..G-010 (prefer-symbol-edits, no-stale-impact, no-false-certainty, multi-file-verify, generated-files); `enforce` mode hardening.

### EVAL — Evaluation Harness

- [ ] **EVAL-01**: An evaluation harness runs `baseline / native / semantic / semantic_guarded` modes against the same task and emits per-task `EvalResult` (success, patch applies, tests pass, diagnostics clean, duration, tokens, cost, edit count, guardrail compliance, context precision/recall).
- [ ] **EVAL-02**: Each mode runs in an out-of-process Helix daemon subprocess with isolated config dir; baseline mode uses a new `--profile=baseline` that strips Helix tools entirely; agent connects via stdio forwarder.
- [ ] **EVAL-03**: An in-process `make eval-quick` variant runs a small fixture suite without subprocess overhead for fast local CI.
- [ ] **EVAL-04**: Reports include `eval_report.json`, `eval_report.md` (mode comparison table, success/cost/latency, tool-call distribution, guardrail compliance, failure examples), `cost_summary.json`, `tool_behavior.json`, `safety_compliance.json`, plus per-task traces and patches.
- [ ] **EVAL-05**: Tool-behavior scoring records whether agents used the right tool for rename/delete/public-API tasks (e.g., `+1` for `rename_symbol` after `find_references`, `-1` for grep-rename).
- [ ] **EVAL-06**: Eval corpus is synthetic-only by default with the OSS Helix repo as a permissible secondary source; commercial-LLM API calls are configured for retention-zero where the provider supports it. Provider data-retention TOS is verified at planning time and recorded in EVAL.md.
- [ ] **EVAL-07**: Cross-model judging is informational, never a CI gate; a flaky judge does not block merges.

### DAG — Pipeline DAG Library

- [ ] **DAG-01**: A `internal/phasegraph/` library provides `PhaseSpec`, `PhaseGraph`, `ValidatePhaseGraph`, and `RunPhaseGraph`; uses stdlib only (Kahn topo-sort + cycle/missing-dep detection + reverse-topological shutdown).
- [ ] **DAG-02**: The semantic-index pipeline (full + incremental snapshot build), the live-update pipeline, and the eval pipeline are each described as a typed phase DAG with declared `Requires`/`Provides`/`Run`/`Validate`/`Shutdown`.
- [ ] **DAG-03**: A pipeline DAG with a duplicate phase ID, missing dependency, or cycle fails validation before execution; error returns include the offending phase IDs and (when configured) writes a `.helix/debug/phasegraph-*.dot` debug file.
- [ ] **DAG-04**: Daemon bootstrap remains imperative in v1.10; a `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` marker is recorded.

### REL — v1.9 Carryover (Release & Distribution)

- [ ] **REL-01** (was PKG-01 SC-3): The first signed Helix release (v1.10.0) is published end-to-end via `goreleaser` with a real maintainer minisign keypair; CI pre-flight rejects PLACEHOLDER pubkey; `helix upgrade` verifies signature and atomically swaps the binary on a real download.
- [ ] **REL-02** (was PKG-DEFER-03): A Homebrew tap publishes `helix` via the existing release artifacts; `brew install agenthands/helix/helix` works on darwin/amd64 and darwin/arm64.
- [ ] **REL-03** (was PKG-DEFER-04): A Scoop bucket publishes `helix` via the existing release artifacts; `scoop install helix` works on windows/amd64.
- [ ] **REL-04** (was PKG-DEFER-05): A native Linux package (`.deb` and/or `.rpm`) is produced by goreleaser and validated to install + register the daemon on a clean Ubuntu/Fedora image. May be downscoped to one format if upstream signing limitations make both formats blocking.
- [ ] **REL-05** (Phase 51 architectural fix): The reproducibility gate compares against a real release artifact (or, alternatively, `CONTRIBUTING.md` is updated to reflect the documented Pass-3 limitation), closing v1.9's deployment-gated SC-3 caveat.
- [ ] **REL-06** (Phase 55 follow-up): `forwarder.tools.call` span is unified with the gRPC server span so a single trace covers stdio → forwarder → daemon → kernel; verified by an end-to-end trace assertion.

---

## Future Requirements (deferred to v1.10.x or later)

- 6 P1 MCP tools: `explain_symbol_deep`, `find_related_symbols`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`, `validate_graph_edge`
- Multi-projection PageRank (REFERENCE_PAGERANK, FILE_DEPENDENCY_PAGERANK, TYPE_HIERARCHY_PAGERANK, CHANGE_IMPACT_PAGERANK, SECURITY_SURFACE_PAGERANK, RETRIEVAL_CONTEXT_PAGERANK)
- Cluster-shipping algorithms beyond weak-components (label propagation refinement, deterministic split policy, cluster labeling/summary generation)
- Guardrails G-006..G-010 (prefer-symbol-edits, no-stale-impact, no-false-certainty, multi-file-verify, generated-files)
- `enforce` mode for guardrails (require_force/enforce thresholds)
- First-class extraction for Java and Rust (today best-effort via tree-sitter generic mode)
- Cross-repo / multi-workspace semantic graphs
- Bootstrap migration to `phasegraph.RunPhaseGraph` (planned v1.11)

## Out of Scope (explicit refusals)

- **Vector / embedding search** — Augment Context Engine does this better; we ship structural retrieval only.
- **RDF / SPARQL knowledge graph storage** — CodeGraphContext / GitNexus own this space.
- **Soundness-grade type inference** — best-effort tiered confidence only; not a competitor to pyright/sorbet/Phan.
- **"Real-time across whole repo"** marketing claim — we ship freshness as an explicit API field, not as a guarantee.
- **Auto-execute on guardrail-pass** — guardrails are advisory until profile-configured otherwise; never silently broaden tool authority.
- **In-process LSP servers** — we wrap existing language servers, never reimplement.
- **SWE-bench leaderboard chase** — our eval proves agent-with-Helix vs agent-without; we do not ship to the SWE-bench external leaderboard.
- **LLM-generated edge explanations** — every edge carries source + evidence + confidence from deterministic extractors, not LLM hallucination.
- **Persistent receipts detached from `graph_version`** — receipts are server-side and graph-version-keyed; client-stored receipts that survive reindex are explicitly forbidden.
- **Full-rebuild on every Helix start** — incremental indexing is non-negotiable; full reindex is gated on schema-version or change-ratio thresholds.

## Traceability

| REQ-ID | Phase |
|---|---|
| (filled by `gsd-roadmapper` after roadmap is created) | |
