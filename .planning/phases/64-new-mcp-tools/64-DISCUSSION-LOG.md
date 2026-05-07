# Phase 64: New MCP Tools (P0 set of 4) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-07
**Phase:** 64-new-mcp-tools
**Areas discussed:** index_semantic_graph dispatch model, get_semantic_context retrieval strategy, refresh vs index --mode=incremental boundary, Default profile visibility

---

## index_semantic_graph dispatch model

### Q1: When an agent calls index_semantic_graph (especially mode=full on a large repo), how should the tool respond?

| Option | Description | Selected |
|--------|-------------|----------|
| Sync up to max_duration_ms; partial=true on timeout | Block in-process up to the request's max_duration_ms (default 120s, capped by TelemetryMiddleware budget). On timeout, return partial=true with files_indexed so far + snapshot_id of the in-progress build. Single round-trip for the common case; reuses existing budget semantics. | ✓ |
| Always async — return immediately, agent polls | Tool returns within ~100ms with snapshot_id + status=building. Agent polls get_semantic_graph_status until status=committed. Worse ergonomics for the 90% incremental/refresh case. | |
| Hybrid — sync for incremental/refresh, async for full | incremental + refresh modes block sync; full + auto return async with status=building. Forces agents to handle both response shapes. | |

**User's choice:** Sync up to max_duration_ms; partial=true on timeout
**Notes:** Selected the recommended option directly.

### Q2: If a second agent calls index_semantic_graph while the first is still building, how do we coordinate?

| Option | Description | Selected |
|--------|-------------|----------|
| Single-flight join — second call attaches to in-flight build | golang.org/x/sync/singleflight.Group keyed by (workspace, mode). Both calls return the same snapshot_id when the build commits or both time out together. Pattern matches Phase 61 LeaseAcquirer + Phase 62 RankScheduler doctrine. | ✓ |
| Reject second call with ErrInProgress | Second call returns immediately with structured error: build_already_in_progress. Agent decides whether to retry. | |
| Queue — second call waits for first to finish, then runs again | Second call runs sequentially after the first. Doubles work, no signal that the second build is even necessary. | |

**User's choice:** Single-flight join — second call attaches to in-flight build

### Q3: What does mode=auto resolve to when the agent calls index_semantic_graph?

| Option | Description | Selected |
|--------|-------------|----------|
| auto = (full if no snapshot) else incremental | First-ever call builds full; subsequent calls do incremental. Matches lazy-init doctrine (semantic_index.indexing.mode=lazy from SPEC §25). | ✓ |
| auto = always incremental; full requires explicit mode=full | Safer (no surprise multi-minute call) but breaks first-call UX on a fresh workspace. | |
| auto = decide per change-ratio threshold | Compute change_ratio = files_changed_since_last_snapshot / total_files. Most adaptive but expensive (worktree scan before dispatch). | |

**User's choice:** auto = (full if no snapshot) else incremental

### Q4: On sync timeout (max_duration_ms exceeded mid-build), what does the response carry?

| Option | Description | Selected |
|--------|-------------|----------|
| partial=true + counts so far + status=building | Return snapshot_id (in-progress), files_indexed, files_reused, partial=true, freshness=stale, duration_ms=elapsed, status=building. Build keeps running in background under singleflight lease; commits when done. | ✓ |
| Hard error — abort the build, return failure envelope | Cancel ctx, roll back the snapshot, return an error. Cleaner state machine but throws away significant work. | |
| Return partial=true but cancel the background build | Same response shape as option A, but the daemon stops the build at timeout. Worst of both worlds. | |

**User's choice:** partial=true + counts so far + status=building

---

## get_semantic_context retrieval strategy

### Q1: How should get_semantic_context rank candidates for the `task` string?

| Option | Description | Selected |
|--------|-------------|----------|
| Graph-only — PageRank + N-hop proximity from anchors | Seed from files + symbols anchors. Score = personalized PageRank. Task string preserved for evidence/explanation but does NOT influence ranking. | |
| Hybrid — graph + text retrieval over symbol/comment text | Parse task into terms, run BM25 over symbol names + docstrings + nearby comments to get a candidate seed set, then expand via graph proximity and re-rank with PageRank. | ✓ |
| Anchors-required — fail closed if no files/symbols seed given | Forces agents to use SMTC-style anchors. Cleanest semantics but extra round trip when agent has a task description but no anchors yet. | |

**User's choice:** Hybrid — graph + text retrieval over symbol/comment text

### Q2: Hybrid picked. What's the text retrieval substrate?

| Option | Description | Selected |
|--------|-------------|----------|
| DuckDB FTS extension on the existing semantic store | Built-in FTS, lives inside semantic.duckdb alongside facts; same lifecycle as snapshots, atomically committed/rolled-back with compaction tx. | |
| modernc.org/sqlite FTS5 (already in stack for memory search) | Reuse the FTS5 path from internal/skill/memory/. Two stores per workspace; not transactionally bound to snapshots. | |
| In-memory inverted index rebuilt per snapshot commit | Build a Go map[term]→[]symbol_id at snapshot commit time. RAM cost grows; daemon restart pays rebuild cost. | |
| **bleve (user-suggested via "Other")** | Pure-Go full-text search library (Couchbase ecosystem). BM25, scorch backend, no CGO, on-disk persistent index. | ✓ |

**User's choice:** bleve (https://github.com/blevesearch/bleve)
**Notes:** User pointed at bleve via Other freeform input. Reflected back the tradeoffs (pro: pure Go, mature BM25, scorch persistence; con: meaningful new dep, dual-store crash recovery story for snapshot↔bleve). Confirmed in follow-up question with rerouting fallback to DuckDB FTS only if researcher surfaces a hard blocker.

### Q3 (confirm): Confirming text retrieval substrate = bleve. Move on, or refine?

| Option | Description | Selected |
|--------|-------------|----------|
| Confirmed — bleve, researcher digs into version/recovery | Lock bleve. Researcher investigates: backend (scorch vs upside-down), version + transitive size, mapping/analyzer config, dual-store crash-recovery story. | ✓ |
| Bleve-conditional — fall back to DuckDB FTS if researcher surfaces a blocker | Lock 'bleve preferred' but instruct the researcher to gate on dependency size, license, and a small benchmark. | |
| Step back — I want to think about this more | Hold the decision. | |

**User's choice:** Confirmed — bleve, researcher digs into version/recovery

### Q4: What text fields get indexed in bleve for each symbol fact?

| Option | Description | Selected |
|--------|-------------|----------|
| Name + docstring + path tokens | Index symbol name (camelCase/snake_case split), docstring, tokenized file path. ~1.5-2x of name-only index. | |
| Name only — minimal index | Smallest, fastest. Misses task strings that mention concepts not in identifiers. | |
| Name + docstring + path + nearby comment context | Adds 5-line window of comments above/below the symbol declaration. Catches inline TODO comments. ~3-4x index size. | ✓ |

**User's choice:** Name + docstring + path + nearby comment context

### Q5: How are bleve text scores fused with Phase 62 PageRank scores in the final ranking?

| Option | Description | Selected |
|--------|-------------|----------|
| Reciprocal Rank Fusion (RRF) | score = 1/(k + rank_text) + 1/(k + rank_graph), with k=60. Robust to score scale differences. Deterministic. | |
| Linear blend — alpha * bleve + (1-alpha) * pagerank | Pick a blend weight. Tunable but the right alpha drifts with task type. | |
| Stage gating — bleve picks top-K candidates, PageRank re-ranks | Bleve returns 200-500 candidates, then PageRank scores order them. Bleve's score is ignored after the cut. | |
| **Weighted RRF with equal-weight defaults (user-suggested via "Other")** | Implement the formula `score = w_text/(K + rank_text) + w_graph/(K + rank_graph)` from day one. RRFConfig{K=60, WText=1.0, WGraph=1.0} as Go-internal constants. Don't expose weights as config keys until Phase 67 eval data justifies. | ✓ |

**User's choice:** Implement weighted RRF capability, ship unweighted RRF default, expose weights only after eval data proves useful profiles
**Notes:** User wrote: "Implement the API so weighted RRF is possible, but set defaults to equal weights ... So my recommendation is: Implement weighted RRF capability. Ship unweighted RRF default. Only expose weights after eval data proves useful profiles. That keeps the design flexible without creating an early tuning knob that agents or users will misuse." Reflected back as D-07 with K=60, WText=1.0, WGraph=1.0 defaults; weights stay Go-internal constants this phase; revisit Phase 67.

---

## refresh vs index --mode=incremental boundary

### Q1: What's the semantic split between the two tools that both apply pending live changes?

| Option | Description | Selected |
|--------|-------------|----------|
| refresh = overlay-only; index = commits snapshot | refresh (read+) drains live queue into overlay, optionally waits for LSP, bumps graph_version. Never commits a snapshot. index --mode=incremental (review+/admin) writes a new snapshot via Phase 63 snapshot-write API. Cleanest interpretation of mode-gating. | ✓ |
| refresh = paths-targeted; index = whole-workspace | refresh re-processes ONLY the files in paths filter; index processes whole workspace. Doesn't explain read+/review+ split. | |
| refresh = poll/no-op; index = the only writer | refresh just reports pending state, doesn't drive any work. wait_for_lsp:true contract doesn't fit. | |

**User's choice:** refresh = overlay-only; index = commits snapshot

### Q2: When refresh_semantic_graph is called with `paths` filter, what does it process?

| Option | Description | Selected |
|--------|-------------|----------|
| Strict subset — only the listed paths, even if other queued changes exist | Process ONLY listed paths; leave others pending. Agent asks about file X, gets exactly file X. | ✓ |
| Inclusive — process listed paths plus drain rest of queue | Listed paths first (priority), then drain everything else. Unbounded extra work. | |
| Best-match — listed paths only IF they're queued; no-op for others | Cheaper but confusing (files_updated=0 ambiguous). | |

**User's choice:** Strict subset — only the listed paths, even if other queued changes exist

### Q3: When refresh_semantic_graph is called with wait_for_lsp:true, how long does it block?

| Option | Description | Selected |
|--------|-------------|----------|
| Up to max_wait_ms (default 3000ms); return pending_lsp:true on timeout | Block on Phase 61 LSPQueue draining for the requested paths up to max_wait_ms. Matches SPEC §23.2 input shape. | ✓ |
| Block until queue drains, ignore max_wait_ms ceiling | Stalled LSP holds the MCP connection arbitrarily long. Unacceptable for read+ tier responsiveness. | |
| Trigger LSP request fire-and-forget; report pending count | Schedule LSP revalidation but don't wait at all. Ignores wait_for_lsp:true contract. | |

**User's choice:** Up to max_wait_ms (default 3000ms); return pending_lsp:true on timeout

### Q4: Does refresh_semantic_graph trigger a Phase 63 compaction cycle as a side effect?

| Option | Description | Selected |
|--------|-------------|----------|
| No — refresh never triggers compaction | Compaction is owned by the Phase 63 per-workspace compactor goroutine. Read+ stays read-only with respect to committed state. | ✓ |
| Yes — refresh resets the compactor's idle timer | Layering refresh on top is redundant; complicates BlockedReason story. | |
| Yes, and synchronously — refresh runs compaction inline if gate is ready | A read+ tool now does write-side work; review+/admin gating exists specifically because committing snapshots is privileged. | |

**User's choice:** No — refresh never triggers compaction

---

## Default profile visibility

### Q1: Which profiles get all four tools by default?

| Option | Description | Selected |
|--------|-------------|----------|
| All four tools in claude-code, codex, ide-assistant, full; only get_semantic_graph_status in ci-bot | Agent profiles see the full surface; ci-bot is intentionally narrow. | |
| **All four in every profile (full surface to everyone)** | Symmetric matrix. Mode-gating handles privilege. | ✓ |
| Status + refresh in all profiles; index + context only in claude-code/codex/ide-assistant/full | More granular; ci-bot gets read-only inspect plus cheap drainer. | |
| Conservative — only claude-code + full get all four; everyone else gets get_semantic_graph_status only | Tightest default. | |

**User's choice (Other freeform):** "All for claude,codex,gemini,opencode,ide-assistant / ci-bot / full all get the same four tools. Simpler matrix, no per-profile reasoning needed. WTF is ci-bot ???"
**Notes:** User's freeform answer named clients (gemini, opencode) rather than profiles. I disambiguated: clients are MCP integration targets (`helix setup <client>` covers Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, OpenCode, generic); profiles are tool-set bundles (claude-code, codex, ide-assistant, ci-bot, full). Each client picks one of the five profiles. ci-bot is the profile for headless CI/automation contexts. Confirmed in follow-up.

### Q2 (confirm): Which axis did you mean?

| Option | Description | Selected |
|--------|-------------|----------|
| All five PROFILES get all four tools (claude-code, codex, ide-assistant, ci-bot, full) | Symmetric matrix. SPEC §30.2 mode-gating handles privilege. | ✓ |
| All four tools in claude-code, codex, ide-assistant, full — ci-bot remains narrow | Phase 64 is conservative on the CI surface. | |
| I want to revisit the profile system itself | Hold the decision. | |

**User's choice:** All five PROFILES get all four tools (claude-code, codex, ide-assistant, ci-bot, full)

---

## Claude's Discretion

These were not explicitly discussed; they fall under "do what's consistent with established patterns":

- Package layout for the four skill packages (`internal/skill/semantic/...`); kernel adapters (`internal/kernel/...`) only if a tool needs kernel-private state.
- Stable-key tiebreak ordering: score → graph_version → symbol_id (matches Phase 62 sort-before-iterate doctrine).
- Effective-graph query implementation (`QueryEffectiveAdjacency`, `CountStaleScoreRows`, `MarkAllScoreRowsStale`) lands on `*Store` in `internal/semantic/store/`.
- `evidence` field shape per candidate: source ranks, matched terms, top edges (capped at ~5).
- Token-budget packing: greedy-by-fused-score (default); switch to RepoMap-style binary search if testing reveals pathological underfill.
- bleve recovery procedure exact mechanism: detect via comparing bleve segment metadata vs `semantic_meta.latest_snapshot_id`; rebuild bleve from snapshot in goroutine on mismatch; gate `get_semantic_context` on rebuild completion.
- `status=building` field name in partial-timeout response (planner may rename as long as agents can disambiguate).
- Bench fixture for determinism: synthetic 50k-symbol Go workspace with fixed seed; reusable by Phase 67 eval harness.

## Deferred Ideas

- Tools 5-10 from SPEC §23.5-23.10 (`explain_symbol_deep`, `find_related_symbols`, etc.) — v1.10.x.
- Multi-projection PageRank surface in `get_semantic_context`.
- RRF weight tuning as exposed config keys — Phase 67 eval-driven.
- MCP push notifications for graph state changes.
- Per-profile tool subset narrowing — wait for Phase 67 eval data.
- Hybrid-mode flip to DuckDB FTS as a runtime fallback (binary-time decision instead).
- `paths`-filter inclusive mode for refresh.
- `get_semantic_context` evidence shape v2 (e.g., per-candidate trace ID).
- Phase 65 strangler-fig integration (existing tools consult semantic).
- Phase 66 guardrails (`GuardrailMiddleware`).
- Phase 67 evaluation harness.
- Audit/redesign of profile semantics for v1.11 (surfaced by user's "WTF is ci-bot" question).
