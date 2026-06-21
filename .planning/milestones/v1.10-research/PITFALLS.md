# Pitfalls Research — Helix v1.10 Live Semantic Index

**Domain:** Live semantic graph layer added to a shipped LSP-backed code intelligence daemon
**Researched:** 2026-05-03
**Confidence:** HIGH for items grounded in shipped Helix bugs (v1.5–v1.9), DuckDB / fsnotify upstream issues, and gopls/jdtls/rust-analyzer behavior we already exercise. MEDIUM for cross-product war stories (Sourcegraph zoekt, Pyright, rust-analyzer save-analysis) where we relied on public post-mortems.

Scope is **adding** a DuckDB snapshot + live overlay + graph cache + LSP enrichment + new MCP tools to a daemon that already ships warm LS workers, RepoMap, fuzzy editing, and a multi-oracle test harness. Pitfalls are framed against SPEC-DRAFT.md sections 16, 17, 18, 22, 29 — flagging where the spec's stated mitigation is *not* sufficient.

For each pitfall: **warning sign** (telemetry/test that catches it), **prevention** (invariant or test contract), **phase to address** (mapped to SPEC-DRAFT.md §32 phase numbering: P0 store, P1 extraction, P2 overlay, P3 LSP enrichment, P4 scores, P5 clusters, P6 MCP tools, P7 existing-tool integration, P8 compaction, P9 guardrails, P10 eval, P11 type resolution, P12 pipeline DAG).

---

## Critical Pitfalls (cause rewrites or silent correctness failures)

### C1. Overlay-write vs graph-cache-repair vs compaction race

**What goes wrong:** Three writers touch overlapping state — `BeginOverlayTx` (live update), `GraphCache.ApplyRepair` (in-memory fan-out), and `CompactOverlay` (snapshot rebuild). SPEC §16.3 commits the overlay tx *before* `GraphCache.ApplyRepair`, then enqueues LSP revalidation. SPEC §22.2 acquires `CompactionLocks.Acquire(repoID)` and reads "overlay" / "snapshot" but does not state whether overlay writes are blocked during compaction or whether `GraphCache` is frozen. If a `UpdateChangedFile` commits after `MergeBaseAndOverlay(base, overlay)` snapshots overlay rows but before `Store.ClearOverlay`, the new overlay row is **lost** when ClearOverlay runs.

**Why it happens:** The lock boundary is named per-repo but the *contents* it protects are split across DuckDB rows, an in-memory `LiveGraphCache`, and a `GraphVersion` counter. Three independent monotonic clocks (overlay `created_at`, `GraphVersion`, snapshot `committed_at`) and no single linearization point.

**Warning sign:**
- "Effective read returned a symbol whose latest overlay row has `committed_at > snapshot.committed_at` but whose `effective_status='snapshot'`" — assertion in §10 effective-read path.
- Compaction runs report `cleared_rows` greater than `merged_rows` (we cleared rows we didn't fold in).
- Soak test: 50 concurrent edits + forced compaction every 100ms; final overlay+snapshot reconcile mismatches.

**Prevention:**
- Make compaction take the **same** transaction lock as overlay writes for the snapshot-phase, OR use an explicit `overlay_epoch` that compaction snapshots and ClearOverlay deletes only rows with `overlay_epoch <= captured_epoch` (CAS pattern). Spec §22.1 mentions "no active overlay transaction" — make this a hard barrier, not a precondition check.
- Property test: replay a recorded edit storm against a model store; assert effective-read converges to the same answer regardless of compaction interleaving.
- Linearizability test (Jepsen-lite, see Sourcegraph zoekt's own approach): inject random pauses between overlay-commit and `GraphCache.ApplyRepair`; confirm no read sees a state where the graph cache has an edge that DuckDB does not (and vice versa).

**Phase to address:** P2 (live overlay) must define the epoch contract; P8 (compaction) must honor it. Add a compaction × edit-storm goldenized property test gated in P8.

---

### C2. fsnotify silently stops watching after editor atomic-rename

**What goes wrong:** Vim, JetBrains IDEA, VS Code, and many other editors save by writing to a temp file then `rename(temp, target)` (or unlink-then-create). On Linux inotify, the *inode* the watcher attached to is gone — fsnotify upstream issues #17, #80, #214, #254, #255, #372 all report the watcher dies and never fires again for that path. On macOS FSEvents the event arrives but coalesced with stale flags. Helix would then *silently* miss every subsequent save to that file. RepoMap currently doesn't watch (it lazy-extracts on demand), so this is a brand-new failure surface for v1.10.

**Why it happens:** Default fsnotify per-file watch attaches to inode, not pathname. SPEC §16.1 lists `ChangeFileModified/Renamed/Deleted` events but does not say whether the watcher *re-attaches* after rename, or how a missed event is detected.

**Warning sign:**
- Live-update goldens passing in CI but agents reporting stale results in field — extremely hard to detect from inside the daemon.
- `helix_live_watcher_dropped_events_total` metric (must exist) > 0 outside of explicit bulk-update events.
- Periodic scrub: every N minutes, hash a deterministic file sample and compare against `files.content_hash` in the store; alert on mismatch. SPEC §27.2 mentions "watcher misses" in passing without a concrete recovery story.

**Prevention:**
- Watch **directories**, not files (Linux inotify best practice; aligns with how gopls and rust-analyzer LSPs do file-watching).
- After every `RENAME` or `REMOVE`, **eagerly re-add the watch** on the new inode if the path still exists; record a `re-attach` count metric.
- Periodic content-hash sweep (SPEC §27.2 promotes from "best-effort" to "required every `watcher_scrub_interval`, default 5 min"). Mismatch → enqueue a synthetic `ChangeFileModified`.
- Test fixture: simulate vim-style atomic rename + JetBrains safe-write (writes to `___jb_tmp___` then renames) + VS Code hot-exit; assert the watcher fires for every save.

**Phase to address:** P2 (live overlay). Watcher contract test must run in P2 and again in P8 (because compaction races with watcher).

---

### C3. PageRank weight explosion when LSP enrichment lands late

**What goes wrong:** SPEC §13 emits tree-sitter-derived edges at lower confidence; SPEC §14 promotes them to LSP-validated 1.0 confidence after enrichment. SPEC §18.4 does "incremental PageRank repair." If a hot symbol (e.g., a popular helper) gets all its incoming edges promoted from 0.4→1.0 over 30 seconds while other parts of the graph still have ts-only edges, that node's score temporarily dominates and **clusters drift visibly between tool calls**. Tool goldens that captured a `get_semantic_context` result in the unstable window break later.

**Why it happens:** Per-edge confidence is multiplied into edge weight (SPEC §12.4); LSP enrichment is asynchronous and prioritized; rank fusion (§18.5) blends across projections. There is no monotone "score frontier" guaranteeing that a tool call at time T returns scores derived from a consistent enrichment level.

**Warning sign:**
- Goldens for `get_semantic_context` / `find_related_symbols` flake when LSP cold-start latency varies.
- `helix_pagerank_score_delta_p99` (must exist) shoots high during enrichment storm even on a quiet repo.
- Manual: run `index_semantic_graph` cold, sample top-10 ranked symbols every 5s for 60s; expect monotonic stabilization, not oscillation.

**Prevention:**
- Tools that return ranked output must include the `graph_version` and `enrichment_level` (% of files LSP-validated) in their result envelope; goldens key on those.
- For agent-facing tools, default to the **last committed snapshot's** scores (deterministic) and overlay only edges, with a separate "live_unranked" hint when overlay-only matches exist. Recompute exact scores only at compaction (SPEC §22.2 already does this in `RecomputeExactScores`).
- For incremental repair (§18.4), bound updates to a small frontier (`max_repair_nodes` config, fail-closed if exceeded → mark scores stale and let next compaction recompute).
- Determinism test: same git SHA + same enrichment level must produce byte-identical PageRank vectors across runs (seeded tie-breaking on NodeID).

**Phase to address:** P4 (graph scores). Add `graph_version` to all tool envelopes in P6.

---

### C4. PageRank dangling nodes & disconnected components destabilize ranks

**What goes wrong:** Real codebases have leaf utilities (no outgoing edges from this repo's POV — they call only stdlib/third-party) and disconnected files (testdata, generated code, fixtures). Naive PageRank with these creates rank sinks; the textbook fix is to treat dangling nodes as uniformly-redistributing. Helix's existing RepoMap PageRank (~60 LOC, hand-rolled, Phase 46) has this baked in for ambiguity-weighted edges, but the **multi-projection** rank in §18.1 (`CALL_GRAPH_PAGERANK`, `REFERENCE_PAGERANK`) re-introduces the problem if any projection filters edges below a threshold.

**Why it happens:** Each projection induces a different sub-graph; dangling-node treatment must be re-applied per projection. Easy to forget in incremental repair (§18.4) which works on a delta.

**Warning sign:**
- `score_fusion_nan_count` > 0.
- Top-K results include a leaf utility ranked above the API endpoint that calls it.
- Property test: any node with zero out-edges in the projection contributes uniformly; assert `Σ pageranks ≈ 1.0` ± epsilon.

**Prevention:**
- Single, shared `applyDanglingMass` helper used by every projection.
- Determinism test: NodeID-sorted iteration in every for-loop; never iterate Go maps for rank computation (existing project rule, easy to slip in incremental repair).
- Score fusion (§18.5) must clamp / renormalize.

**Phase to address:** P4 (graph scores).

---

### C5. DuckDB cross-process file-lock collision (CLI tools, eval harness)

**What goes wrong:** DuckDB upstream is explicit: "Concurrent writes to the same DuckDB file from multiple processes are not supported … will return `IO Error: Could not set lock on file`." Helix runs as a daemon (one process — fine). But `helix status`, future `helix index --offline`, the eval harness (SPEC §37), and any CLI subcommand that wants to peek at the store will collide if they open it read-write. Even read-only opens can be sensitive to WAL state.

**Why it happens:** DuckDB is designed for single-process MVCC. Helix's existing CLI (`helix status`, `helix activate`) communicates with the daemon over gRPC — but a future contributor "just opens the DB" for a quick query and breaks the daemon.

**Warning sign:**
- CI flakes: `IO Error: Could not set lock on file`.
- Eval harness fails with lock errors when running while a dev daemon is active on the same workspace.

**Prevention:**
- **All store access goes through the daemon** — exposed via gRPC or admin HTTP, never by reopening the .duckdb file. Add a build-time vet rule (forbidden import: `marcboeker/go-duckdb` outside `internal/store/`).
- `helix status --json` / future `helix graph dump` must call into the daemon, not the file.
- Eval harness (P10) gets its own *isolated* workspace + daemon child, never shares DB with the dev daemon.
- Document this as a constraint in `internal/store/README.md` alongside the schema.

**Phase to address:** P0 (store). Lock the architectural rule before any other code touches the DB.

---

### C6. DuckDB JSON column performance / size growth without VACUUM

**What goes wrong:** SPEC §9 schema heavily uses JSON columns (evidence, type-evidence, edge-properties). DuckDB JSON is fine for small payloads but: (a) blob/JSON updates rewrite full rows, (b) `VACUUM` / `CHECKPOINT` are *not* automatic on the WAL — long-lived overlays bloat the WAL until forced checkpoint, (c) reading a JSON column for every row in a large query is order-of-magnitude slower than a typed column. On a 50k-symbol repo with 30 days of edits, the .duckdb file can grow >1 GB without VACUUM.

**Why it happens:** Schema convenience pulls developers toward JSON for "I'll add fields later." Compaction (§22) doesn't currently call `CHECKPOINT`.

**Warning sign:**
- `du -sh .helix/semantic.duckdb` grows monotonically across compactions.
- `get_semantic_context` p95 latency drifts upward as repo ages even when symbol count is flat.
- DuckDB `pragma_database_size()` reports `wal_size` > 100 MB.

**Prevention:**
- Compaction (§22.2) ends with `CHECKPOINT` and a periodic `VACUUM` (config-gated, defaults to weekly).
- Bench harness includes a "100-day repo" scenario (replay 1000 fake commits) — assert query p95 doesn't drift > 25%.
- Anything queried in a hot loop is a typed column; JSON only for evidence blobs that are read by ID, never scanned.
- Cap evidence size per row (drop oldest when limit hit) — SPEC says "evidence" but doesn't bound size.

**Phase to address:** P0 (schema), P8 (compaction).

---

### C7. Stable symbol ID drift on rename / generics / overloads

**What goes wrong:** SPEC §11 defines a stable symbol key. The hard cases:
- File rename `foo.go` → `foo_test.go`: package-private `helper` keeps logical identity but path-based key changes.
- Go generic instantiation `List[int]` vs `List[string]`: same source decl, different runtime types.
- TypeScript overloaded function declarations (`function f(x: number): number; function f(x: string): string;`): same name, two declarations.
- Anonymous types / closures (Go `func() {}`, JS arrow functions): line+column based keys flip on whitespace changes.
- Two packages exporting `New` (extremely common in Go): qualified-name collision if the package path is dropped.

If keys drift on a non-semantic edit, every tool that compares "before vs after" (blast radius, change impact graph) reports false changes; if keys collide, two distinct symbols share scores, breaking `find_related_symbols`.

**Why it happens:** Stable-key design is one of the hardest problems in code intelligence. SPEC §11.2 has merge rules but does not enumerate these edge cases.

**Warning sign:**
- `helix_symbol_id_drift_per_commit` metric > 0 on whitespace-only commits.
- Goldens for `get_change_impact_graph` flake on no-op refactors (rename a parameter).
- Test: clone HEAD twice, run extraction on both, diff symbol keys → must be empty.

**Prevention:**
- Stable key components (SPEC §11.1): `repo_id || package_path || enclosing_chain || name || arity || receiver_type` — explicitly NOT line/column, NOT signature_hash (sensitive to whitespace), NOT generic type-args. `signature_hash` lives in a *separate* column for "did the contract change" checks.
- For overloads: include parameter types (canonicalized, not raw text) in the key.
- For anonymous symbols: hash the AST shape, not the source range.
- Test matrix: 30+ pairs of (before, after) where the human-meaning is "same symbol" or "different symbol"; key equality must match human intent.
- Cross-language test: same test matrix re-run for Go, TS+JS, Python.

**Phase to address:** P1 (extraction). Lock the key contract before any consumer (overlay, graph, MCP tools) depends on it.

---

### C8. LSP enrichment death spiral (jdtls cold-start, gopls memory, rust-analyzer status)

**What goes wrong:** The new LSP revalidation queue (SPEC §21) calls into the same shared LS workers that serve interactive MCP tools. If 1000 files are queued for revalidation right after a `git checkout main`, jdtls (just stabilized in Phase 56!) gets buried under workspace/symbol calls; gopls hits its memory budget and starts evicting working-set packages; rust-analyzer's `experimental/serverStatus` gate (Phase 47) sits waiting forever because indexing keeps re-triggering. Interactive `find_references` calls timeout. Users see Helix get *slower* the more it knows.

**Why it happens:** SPEC §14.2 has "LSP Budget" and §21 has prioritization — but the budget is a token bucket per-LS, not a global *fairness* constraint between revalidation traffic and live MCP traffic. Helix already learned this: pre-Phase 56, jdtls notification dispatch was silently dropped and `JdtlsAdapter.WaitUntilJavaReady` was racy.

**Warning sign:**
- `helix_lspool_queue_depth{kind="revalidation"}` and `{kind="interactive"}` both rise; interactive p95 climbs.
- Circuit breakers (`ErrCircuitOpen`) fire on tools that worked fine before the index was added.
- jdtls memory > GOMEMLIMIT × 0.8.

**Prevention:**
- Revalidation queue is **strictly lower priority** than interactive MCP traffic; use a separate token bucket *and* a hard concurrency cap (default 1 worker).
- Interactive deadline (§14.2) preempts in-flight revalidation requests when LS is saturated.
- After `git_checkout` / `bulk_update`, do **NOT** flood the queue — mark all affected files `pending` and let lazy-on-tool-call drive enrichment (SPEC §29.5 says "pause PageRank local repair" but says nothing about pausing LSP revalidation).
- Reuse Phase 56's `WaitUntilJavaReady` for jdtls, `experimental/serverStatus` for rust-analyzer; gate revalidation behind LS-readiness before enqueueing.
- Soak test: 10k-file checkout + interactive tool calls every 1s; assert interactive p95 stays under 2s.

**Phase to address:** P3 (LSP enrichment). Cross-cuts existing v1.9 worker pool — must not regress the BUG-02 (rust-analyzer) and Phase 56 (jdtls) fixes.

---

## Moderate Pitfalls

### M1. Overlay rows lost during compaction crash

**What goes wrong:** Compaction (§22.2) writes a new snapshot then `Store.ClearOverlay`. If the daemon crashes between `CommitSnapshot` and `ClearOverlay`, on restart the overlay still has rows that are now also in the snapshot — effective-read sees doubled / contradictory facts. If the crash is between `WriteSnapshotFacts` and `CommitSnapshot`, the half-written snapshot needs cleanup.

**Why it happens:** Two-step commit without a journal. SPEC §29.6 says "Abort new snapshot. Keep previous committed snapshot. Keep live overlay if it remains valid" — "if it remains valid" is doing a lot of work.

**Prevention:**
- Single DuckDB transaction wraps `CommitSnapshot` + `ClearOverlay`. Yes, the transaction is large; that's correct.
- If single-tx not feasible, use a `compaction_journal` table: write `(snapshot_id, overlay_epoch_cleared)` *atomically* with the snapshot commit; on restart, replay/reconcile.
- Crash test: kill daemon mid-compaction (SIGKILL), restart, assert effective-read returns the pre-compaction state OR the post-compaction state, never a mix.

**Warning sign:** Effective-read returns more symbols than `count(*) from symbols where snapshot_id = current`.

**Phase to address:** P8 (compaction).

---

### M2. Bulk-update threshold tuned wrong → death spiral

**What goes wrong:** SPEC §16.2 collapses "many changes above threshold" into a single `bulk_update`. Set too low, every `npm install` fires a full re-extraction; set too high, a `git checkout` of a 500-file branch becomes 500 individual updates that never coalesce because they trickle in over multiple debounce windows.

**Prevention:**
- Threshold is `min(N_absolute, fraction_of_workspace)` — default 50 files OR 5% of workspace, whichever is smaller. Re-evaluate on benchmark suite.
- Different thresholds per source: a `helix_edit` event from our own tool is never bulk-coalesced (it's already one logical edit); a watcher-source flood is.
- Telemetry: record `coalesce_ratio = output_events / input_events`.

**Warning sign:** `coalesce_ratio` < 0.1 (everything getting coalesced) or > 0.95 (nothing getting coalesced).

**Phase to address:** P2 (live overlay).

---

### M3. Type-resolution fixpoint non-convergence

**What goes wrong:** SPEC §38 runs a fixpoint over file facts, promoting type-evidence rounds. Pathological cases: mutually-recursive type aliases, generic constraints that depend on themselves, Python duck-typed functions where every argument is `Any`. The loop runs the iteration cap with no progress, declares "did not converge," and the spec doesn't say what edges get *emitted* in that case.

**Prevention:**
- Hard iteration cap (`max_fixpoint_iters`, default 8) — beyond cap, **emit only edges with confidence ≥ 0.9** at last stable iteration. Never emit "validated" status for non-converged.
- Per-iteration progress metric: if `Δ resolved_types` is 0 for 2 iterations, exit early.
- Test: synthetic mutually-recursive generic from rust-analyzer's test corpus; assert termination + correct conservative output.

**Warning sign:** `helix_type_resolution_non_converged_total` > 0; agent gets surprising "validated" claims that are actually heuristic.

**Phase to address:** P11 (type resolution).

---

### M4. Receipts don't survive context compaction

**What goes wrong:** SPEC §36.4 introduces "safety receipts" that an agent should check before destructive edits. Coding agents (Claude Code, Codex) auto-compact context when token budget exceeds threshold; a receipt produced 30 turns ago is summarized away and the next destructive edit happens *without* the receipt being cited. The guardrail is bypassed without anyone noticing.

**Prevention:**
- Receipts are **server-side state**, not just a tool result. Daemon stores receipt ID → claim mapping with TTL (5 min).
- Destructive tools (rename_symbol, replace_symbol_body, etc.) require `receipt_id` parameter and look up the claim themselves; agent only forwards an ID, not the content.
- Without a valid receipt → tool returns `Unsupported` / `InvalidArgs` with explicit "call `validate_graph_edge` first" hint.
- Test: integration scenario where agent's context is artificially truncated; destructive tool must refuse without server-side receipt match.

**Warning sign:** `helix_receipt_expired_total` ≪ `helix_receipt_issued_total` BUT destructive tool call rate is high → agents are bypassing.

**Phase to address:** P9 (guardrails).

---

### M5. Eval harness leaks task prompts into model training

**What goes wrong:** Eval tasks (SPEC §37.4) sent to commercial LLM APIs (Anthropic, OpenAI). Per provider TOS, prompts may be retained for abuse review; some providers historically used API traffic for training (now opt-out by default, but the default flipped within the last 18 months and may flip back). If an eval task contains proprietary repo code, that code leaves the building.

**Prevention:**
- Eval tasks committed to repo: synthetic only, no proprietary code. README states this explicitly.
- For "real repo" evals: use the OSS Helix repo itself, or a public OSS project.
- Provider config must include `data_retention=zero` flag where supported (Anthropic: "no retention" tier requires enrollment; OpenAI: `store=false` on Chat Completions).
- Document the threat model in `eval/README.md`.

**Warning sign:** N/A — this is a process pitfall.

**Phase to address:** P10 (eval).

---

### M6. Cardinality explosion in metrics

**What goes wrong:** Per-symbol or per-file labels on Prometheus metrics blow up the time-series count. v1.2 already established a bounded-label allowlist (`tool_name, profile, mode, language, outcome`). New v1.10 metrics tempt with `symbol_kind`, `edge_kind`, `projection`, `language` — fine. But `repo_id`, `file_path`, `cluster_id`, or anything per-symbol is forbidden.

**Prevention:**
- Add `language` only if values come from the closed 52-language registry (already enforced).
- New labels: `edge_kind`, `projection`, `confidence_bucket` (low/med/high), `freshness` (snapshot/overlay/pending). Each ≤ 10 cardinality.
- Bounded-label CI lint already exists (v1.2 Phase 11) — extend it to the new metric families.

**Warning sign:** Prometheus scrape size grows > 10× current.

**Phase to address:** P0 (alongside store metrics).

---

### M7. Existing-tool fallback path drift

**What goes wrong:** SPEC §24 says `get_repo_map`, `get_context`, `analyze_blast_radius` integrate with the live graph. If the index is **disabled** (config flag, or DuckDB unavailable, or first-run before index build), these tools fall back to v1.9 behavior. Two failure modes: (a) the fallback path is rarely tested → silently rots, (b) tool reports "validated" when actually using fallback heuristic.

**Prevention:**
- Every existing tool keeps a **`source` field** in the result envelope: `"snapshot" | "overlay" | "live_graph" | "fallback_repomap" | "fallback_lsp_only"`.
- Test matrix per tool: index enabled & populated, index enabled & cold, index disabled. All three must pass. Index-disabled path is the v1.9 golden.
- `analyze_blast_radius` confidence drops to ≤ 0.6 in fallback mode and explicitly says so in result.

**Warning sign:** Goldens for `get_repo_map`/`get_context` only run with index-on; v1.9 path passes only because no one runs it.

**Phase to address:** P7 (existing-tool integration).

---

### M8. Token-budgeted retrieval non-deterministic across runs

**What goes wrong:** SPEC §20.5 token-budgeted selection picks symbols up to a budget. If selection iterates a Go map (or any unordered set), top-K is non-deterministic when scores tie or are within float epsilon. Goldens flake; agents see different context for the same query.

**Prevention:**
- Sort by `(score DESC, stable_symbol_key ASC)` before any cut-off — stable_symbol_key is a deterministic string (SPEC §11.1).
- Existing RepoMap renderer (v1.6) already does this for binary-search budget fitting; reuse the same helper.
- Goldens for `get_semantic_context` capture exact symbol order at multiple budgets.

**Warning sign:** Same query, same SHA, different output across runs.

**Phase to address:** P6 (MCP tools).

---

## Minor Pitfalls

### m1. Pipeline DAG half-migrated bootstrap ordering

**What goes wrong:** SPEC §39 introduces a typed pipeline DAG. Migrating the existing daemon bootstrap (`internal/daemon/daemon.go`, currently 14+ steps with strict ordering — middleware install order is critical, see CLAUDE.md) leaves some phases declared and others ad-hoc. Cycle / missing-dep validation lies because half the dependencies aren't expressed.

**Prevention:** Migrate atomically per area; each area's DAG check passes before merging. Maintain CLAUDE.md's middleware LIFO invariant in the DAG (declare it explicitly).

**Phase to address:** P12 (pipeline DAG).

---

### m2. Shutdown order inversion deadlock

**What goes wrong:** Existing daemon shutdown is "kernel-first, then everything else." Adding overlay tx + LSP queue + compaction goroutine + watcher + eval-harness child = 4 new shutdown participants. If watcher is shut down *before* the in-flight overlay tx commits, the tx commit succeeds but a downstream "scrub" check (which compares store state to filesystem state) sees a phantom mismatch and flags corruption.

**Prevention:**
- Document shutdown order in `daemon.go` header comment: watcher (stop accepting new events) → coalescer drain (apply remaining) → LSP queue drain → compaction *if pending* → overlay close → store close → kernel.
- Each subsystem has a `Drain(ctx)` method with a hard deadline; shutdown returns OK if all drain or escalates after 5s.

**Phase to address:** P12 (pipeline DAG) — encode in DAG.

---

### m3. JSDoc/PHPDoc/YARD heuristic emits "validated" type edges

**What goes wrong:** SPEC §38.6 allows comment-based fallbacks. If the resolver promotes comment-derived types to the same edge confidence as LSP-derived types, downstream tools claim verification they don't have.

**Prevention:** Comment-derived evidence caps at confidence 0.7 (`heuristic_validated`), not 1.0. Edge `evidence_kind` field surfaces the source.

**Phase to address:** P11 (type resolution).

---

### m4. Judge-LLM bias in eval

**What goes wrong:** Same model family scoring its own output (Anthropic judge scoring Claude agent) inflates scores. Previous Helix v1.4 judge work already noted this — kept "informational only, never blocks merge." Don't change that contract.

**Prevention:** Cross-model judging (Anthropic-judge for DeepSeek-agent, vice versa). Aggregate reports show per-judge score; large divergence flags suspect tasks for human review. Never make judge output a CI gate.

**Phase to address:** P10 (eval).

---

### m5. Daemon-config drift between project / user / CLI for new index settings

**What goes wrong:** v1.10 introduces ~20 new config knobs (compaction interval, LSP budget, bulk threshold, eval modes, etc.). 4-layer precedence (CLI > project > user > profile) already exists. New knobs added to one layer's defaults but not documented in USAGE.md → silent surprises.

**Prevention:** All new keys in `internal/config/` get a `config_test.go` "every key has a default + every key documented in USAGE.md" assertion (existing pattern).

**Phase to address:** P0 (alongside store config).

---

## Phase-Specific Warnings

| Phase | Likely Pitfall(s) | Mitigation Required |
|---|---|---|
| P0 Store | C5 cross-process lock, C6 JSON growth, M6 cardinality, m5 config drift | Architectural lock rule, CHECKPOINT in compaction, label allowlist extension, config-coverage test |
| P1 Extraction | C7 stable-key drift | Symbol-key contract test matrix, lock before consumers depend on it |
| P2 Live overlay | C1 race, C2 watcher misses, M2 bulk threshold | Epoch-CAS contract, directory-watch + scrub, telemetry on coalesce ratio |
| P3 LSP enrichment | C8 death spiral | Strict priority queue, do-not-flood after bulk_update, reuse v1.9 readiness gates |
| P4 Scores | C3 score instability, C4 dangling nodes | `graph_version` in tool envelope, monotone-frontier rule, shared dangling-mass helper |
| P5 Clusters | C3 (cluster drift mirrors score drift) | Same as P4; clusters key on snapshot scores by default |
| P6 MCP tools | M8 non-determinism | Stable-key tiebreak in all selection paths |
| P7 Existing-tool integration | M7 fallback drift | `source` field in envelope; index-disabled goldens |
| P8 Compaction | C1 race, M1 crash recovery, C6 VACUUM | Single-tx commit OR journal table; long-repo bench |
| P9 Guardrails | M4 receipt loss | Server-side receipt store, ID-only forwarding |
| P10 Eval | M5 prompt leakage, m4 judge bias | Public-data only, retention-zero, cross-model judging, never CI gate |
| P11 Type resolution | M3 fixpoint non-convergence, m3 comment heuristic confidence cap | Hard iteration cap + conservative emission; cap comment-derived confidence |
| P12 Pipeline DAG | m1 half-migration, m2 shutdown order | Atomic per-area migration; declare existing middleware LIFO invariant in DAG |

---

## Sources

**Helix shipped bugs (anti-pattern source):**
- v1.9 Phase 47 BUG-02: rust-analyzer rename quirk — LS readiness must be structurally signaled (`experimental/serverStatus`), not timed
- v1.9 Phase 48 + 56 BUG-03 / LSDISP-01..04b: jdtls notification dispatch was silently dropped pre-v1.9 (`jsonrpc.Conn.OnNotification` not set in `Worker.Start`); `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate
- v1.9 Phase 46 BUG-01: PageRank ambiguity-weighting + Lua-fixture leak — confirms PageRank determinism + edge-weight footguns are real on this codebase
- v1.9 Phase 49 BUG-04: GrammarRegistry duplication — confirms shared-registry pitfalls in concurrent extractors; single canonical registry now injected from daemon bootstrap
- v1.1 Phase 8: SessionInfo data race (RWMutex + Snapshot pattern) — established the pattern reused for `LiveGraphCache` here
- v1.9 metric rename `serena_* → helix_*` and bounded-label CI lint (v1.2 Phase 11) — cardinality discipline pre-existing

**SPEC-DRAFT.md weak points identified above (sections under-specified for the failure they describe):**
- §16.3 commit ordering vs §22 compaction — race C1 (no epoch CAS)
- §27.2 watcher-misses described as "best-effort" — pitfall C2 (no scrub contract)
- §29.5 rapid edit storm only mentions PageRank pause, not LSP queue pause — pitfall C8
- §38 fixpoint without explicit non-convergence emission rule — pitfall M3
- §29.6 snapshot-commit failure says "keep overlay if valid" without defining valid — pitfall M1
- §36.4 receipts treated as agent-side artifact — pitfall M4

**External war stories (MEDIUM confidence):**
- fsnotify upstream: [#17 atomic saves](https://github.com/fsnotify/fsnotify/issues/17), [#80 mv overwrite misses](https://github.com/fsnotify/fsnotify/issues/80), [#214 keep watching on rename](https://github.com/fsnotify/fsnotify/issues/214), [#254 watcher quits after one change](https://github.com/fsnotify/fsnotify/issues/254), [#372 watching single file is highly nontrivial](https://github.com/fsnotify/fsnotify/issues/372)
- DuckDB concurrency: [DuckDB Concurrency docs](https://duckdb.org/docs/current/connect/concurrency), [Issue #77 multiple instances on same file](https://github.com/duckdb/duckdb/issues/77), [Discussion #4899 concurrent writes fail](https://github.com/duckdb/duckdb/discussions/4899)
- PageRank dangling nodes: [Langville & Meyer, "PageRank Computation, with Special Attention to Dangling Nodes," SIAM J. Matrix Anal. Appl.](https://epubs.siam.org/doi/10.1137/060664331)
- Sourcegraph zoekt and gopls have published incremental-index correctness post-mortems; the C1 epoch-CAS pattern is borrowed from gopls's overlay handling

**Confidence calibration:**
- HIGH: C1, C2, C5, C7, C8, M1, M4, M6, M7, M8 — anchored either in shipped Helix bugs or in well-documented upstream issues we can hit reproducibly
- MEDIUM: C3, C4, C6, M2, M3, M5, m1–m5 — well-known patterns in adjacent products, not yet observed in Helix because the subsystem isn't built
