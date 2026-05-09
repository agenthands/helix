---
phase: 64
plan: 06
subsystem: get-semantic-graph-status-tool
tags: [mcp-tool, read-tier, status-envelope, w1-closure, freshness-enum, score-status, cluster-status]
dependency_graph:
  requires:
    - phase: 64
      plan: 02
      provides: "*Store effective-graph queries (LatestCommittedSnapshot consumed by handler fan-out)"
    - phase: 64
      plan: 03
      provides: "skill.go (FINAL) + accessors.go (FINAL — including SchedulerAccessor.ClusterStatus) + envelope.go (StatusResult / ClusterStatus / Freshness closed enum) + mode_check.go"
    - phase: 64
      plan: 04
      provides: "handler_helpers.go (errorResult / jsonResult) consumed read-only"
  provides:
    - "internal/skill/semantic/tools_status.go — GetSemanticGraphStatusArgs typed-args struct (empty), registerGetSemanticGraphStatus (kernel-style typed-args + WrapToolSpan), testable handleGetSemanticGraphStatus method, const statusHelp"
  affects:
    - phase: 64
      plan: 08
      via: "P64-08 calls registerGetSemanticGraphStatus(server, semanticSkill, tracer) from semantic_wiring.go after wiring StoreAccessor / SchedulerAccessor / QueueAccessor / LiveAccessor / RetrievalAccessor adapters"
tech_stack:
  added: []
  patterns:
    - "Pure read-path fan-out — every accessor call is independently nil-guarded so a partial-wiring state still returns a structured envelope rather than a hard error"
    - "Closed-enum freshness selection with priority order — retrievalPending > (overlay && pendingLSP) > overlay > fresh; switch-case-default pattern matches Phase 62 D-07 doctrine"
    - "Per-accessor error tolerance — individual accessor errors log to slog.Warn but don't fail the response (status is a pure inspector; partial state is more useful than an error envelope)"
    - "Hard-coded projection list (statusProjections) — caller cannot inject a projection name (T-64-06-03 mitigation)"
    - "ws.Hash() as the canonical repoID seam (matches refresh/index handlers)"
key_files:
  created:
    - internal/skill/semantic/tools_status.go
    - internal/skill/semantic/tools_status_test.go
  modified:
    - internal/skill/semantic/skill.go
decisions:
  - "retrieval_pending mapped to FreshnessStale (not a new enum value) — bleve recovery means the retrieval index doesn't yet reflect the latest committed snapshot, which is observably equivalent to a stale cache. Keeping the four-value SPEC §26.2 closed enum stable (vs adding a fifth 'retrieval_rebuilding' value) preserves API stability for downstream consumers."
  - "Per-accessor error tolerance vs hard-fail: each StoreAccessor read (LatestCommittedSnapshot, CurrentGraphVersion) tolerates errors by zero-valuing the field and logging via slog.Warn. Reasoning: status is a pure inspector — agents call it to decide whether to refresh/index, and a hard failure when one accessor degrades is worse than a partial envelope that visibly shows the gap (graph_version=0 + scoreStatus=missing is itself a useful signal)."
  - "Projection list hard-coded (CALL_GRAPH_PAGERANK / REFERENCE_PAGERANK / FILE_DEPENDENCY_PAGERANK) at package scope (statusProjections var). Closes T-64-06-03 — caller cannot inject a projection name. List matches what Phase 62 ships; future projection additions land here when Phase 62 surfaces them."
  - "graphScoreStatusMissing as a local string const rather than `string(graph.ScoreStatusMissing)` — the test-path scheduler-nil branch needs the value at compile-time, and importing graph just for one constant in a fan-out file would obscure the seam (graph is consumed only via the SchedulerAccessor.ScoreStatus signature in accessors.go). The const carries a docstring pointing to internal/semantic/graph/status.go for traceability."
  - "freshness priority retrievalPending > overlay+pendingLSP > overlay > fresh, NOT retrievalPending > overlay > pendingLSP. Reasoning: pendingLSP alone (without overlay-active) does not gate freshness per SPEC §26.2 — the LSP queue can be non-empty in fully-committed steady state when the latest commit triggered enrichment work that's not yet drained. The structurally_fresh_semantically_pending value is reserved for the case where overlay updates have applied (graph_version bumped) AND LSP enrichment is still pending."
metrics:
  start_time: "2026-05-07T22:30:42Z"
  end_time: "2026-05-07T22:35:13Z"
  duration: "~5 minutes (single-session sequential execution)"
  task_count: 1
  files_created: 2
  files_modified: 1
  tests_added: 5
  completed_date: "2026-05-08"
---

# Phase 64 Plan 06: get_semantic_graph_status (TOOL-03) Summary

## One-Liner

Implement TOOL-03 (`get_semantic_graph_status`): typed-args registration, mode-tier (read+) check, fan-out of read-only `StoreAccessor` / `SchedulerAccessor` / `QueueAccessor` / `LiveAccessor` / `RetrievalAccessor` calls into the SPEC §23.3 envelope. Closed-enum freshness selection with priority `retrievalPending > overlay+pendingLSP > overlay > fresh`. Pure read path — no writes. Closes checker W1 at the response-shape level (`cluster_status` structured envelope with `state="unknown"` + `reason="phase-62-clustering-no-status-accessor"` until Phase 65/67 wires a live source).

## What Shipped

### Task 1: tools_status.go + 5 unit tests + skill.go single-line deletion

**`internal/skill/semantic/tools_status.go`** (new):

- `const statusHelp` — verbose multi-line help text (Usage Examples / Parameters / Return Shape / Mode Tier sections; documents closed-enum freshness values + closed-enum cluster_status states + closed-enum score_status per Phase 62).
- `type GetSemanticGraphStatusArgs struct{}` — empty typed-args struct for jsonschema-generation consistency.
- `registerGetSemanticGraphStatus(server, s, tracer)` — kernel-style `mcpsdk.AddTool` + `kernel.WrapToolSpan` + `server.Registry().Register(&mcp.ToolDef{..., HelpText: statusHelp})`.
- `var statusProjections = []string{"CALL_GRAPH_PAGERANK", "REFERENCE_PAGERANK", "FILE_DEPENDENCY_PAGERANK"}` — hard-coded projection list at package scope (T-64-06-03 mitigation: caller cannot inject a projection name).
- `(s *SemanticSkill) handleGetSemanticGraphStatus(ctx, _) *mcpsdk.CallToolResult` — testable handler body. Order:
  1. `checkMode(s.sessionSnapshot(ctx), modeTierRead)` — every session passes; retained for code-review visibility and the Phase 66 GuardrailMiddleware precedent.
  2. Resolve `ws := s.workspaceKey(ctx)`; `repoID := ws.Hash()`.
  3. Fan-out accessor reads under nil-guards:
     - `s.store.LatestCommittedSnapshot(ctx, repoID)` → `latestSnapshotID` (error → 0 + slog.Warn).
     - `s.store.CurrentGraphVersion(ctx, repoID)` → `graphVersion` (error → 0 + slog.Warn).
     - `s.store.OverlayHasPendingRows(repoID)` → `overlayActive`.
     - `s.queue.DepthAll(ws)` → `pendingLSPFiles`.
     - `s.live.LastFlushAt(ws)` → `lastLiveUpdateMs`.
     - For each projection in `statusProjections`: `s.scheduler.ScoreStatus(repoID, projection)` → `scoreStatus[projection]`.
     - `s.scheduler.ClusterStatus(repoID)` → `clusterStatus` (W1 closure: production adapter returns the unknown-with-reason default until Phase 65/67).
     - `s.retrieval.RetrievalPending(ws)` → `retrievalPending`.
  4. Closed-enum freshness selection (SPEC §26.2):
     - `retrievalPending` → `FreshnessStale` (engine rebuilding overrides everything).
     - `overlayActive && pendingLSPFiles > 0` → `FreshnessStructurallyFreshSemanticallyPending`.
     - `overlayActive` → `FreshnessOverlayActive`.
     - default → `FreshnessFresh`.
  5. `jsonResult(StatusResult{...})` — envelope marshal.
- `const graphScoreStatusMissing = "missing"` — package-local const for the test-path scheduler-nil branch (avoids importing `graph` just for one constant).

**`internal/skill/semantic/tools_status_test.go`** (new) — 5 tests:

| Test | What it asserts |
| --- | --- |
| `TestStatusHandler_EmptyStore` | All accessors return zero values → response envelope: `LatestSnapshotID=0`, `GraphVersion=0`, `OverlayActive=false`, `PendingLSPFiles=0`, `LastLiveUpdateMs=0`, `RetrievalPending=false`, `Freshness=fresh`, all 3 projections `score_status=missing`, `ClusterStatus.State="unknown"`, `ClusterStatus.Reason="phase-62-clustering-no-status-accessor"`. |
| `TestStatusHandler_PopulatedStore` | Store returns snapshot=42 / GV=99 / overlay=true; queue depth=5; per-projection `exact / stale / approximate`; cluster_status default. Asserts every field reflects supplied state; `Freshness=structurally_fresh_semantically_pending` (overlay + pendingLSP). |
| `TestStatusHandler_ClusterStatus_DefaultUnknown_ReasonPopulated` (**W1 closure**) | Scheduler returns `ClusterStatus{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}`. Asserts `result.ClusterStatus.State == "unknown"` AND `result.ClusterStatus.Reason == "phase-62-..."` AND wire-format JSON marshals to `{"state":"unknown","reason":"phase-62-clustering-no-status-accessor"}`. |
| `TestStatusHandler_FreshnessEnum_AllValuesValid` | Table over (overlayActive, pendingLSPFiles, retrievalPending) tuples covering all four closed-enum values; pins priority ordering (retrievalPending > overlay+pendingLSP > overlay > fresh) so a future regression that swaps two cases is caught. Includes a closed-enum guard that rejects any out-of-enum freshness value. |
| `TestStatusHandler_RetrievalPending_FreshnessStale` | retrievalPending=true with overlay=true AND pendingLSP=2 → response carries `RetrievalPending=true` AND `Freshness=stale` (priority case overrides overlay/pendingLSP signals). |

**`internal/skill/semantic/skill.go`** (single-line deletion):

```diff
 var (
-	statusHelp  = "get_semantic_graph_status: stub help (replaced by P64-06)"
 	contextHelp = "get_semantic_context: stub help (replaced by P64-07)"
 )
```

`Tools()` still references `statusHelp` — Go resolves it to the `const statusHelp` declared at package scope in `tools_status.go`. Zero other diff lines in skill.go. accessors.go is NOT in this plan's diff.

**Commit:** `0d1e3edf` — `feat(64-06): implement get_semantic_graph_status (TOOL-03)`

## Verification

| Check | Result |
| --- | --- |
| `go vet ./internal/skill/semantic/...` | PASS |
| `gofmt -l internal/skill/semantic/{tools_status.go,tools_status_test.go,skill.go}` | empty |
| `go test ./internal/skill/semantic/ -run TestStatusHandler_ -count=1` | PASS — 5/5 tests (with 6 sub-cases for the freshness-table test) |
| `go test ./internal/skill/semantic/ -race -count=1` | PASS — full suite race-clean |
| `tools_status.go` contains `func registerGetSemanticGraphStatus(` | PASS |
| `tools_status.go` declares `const statusHelp` | PASS |
| `tools_status.go` invokes ALL of: `s.store.LatestCommittedSnapshot`, `s.store.CurrentGraphVersion`, `s.store.OverlayHasPendingRows`, `s.queue.DepthAll`, `s.live.LastFlushAt`, `s.scheduler.ScoreStatus`, `s.scheduler.ClusterStatus` | PASS — `grep -E "s\.store\.\(LatestCommittedSnapshot\|CurrentGraphVersion\|OverlayHasPendingRows\)\|s\.queue\.DepthAll\|s\.live\.LastFlushAt\|s\.scheduler\.\(ScoreStatus\|ClusterStatus\)"` returns 7 matches |
| `tools_status_test.go` contains all 5 named tests including `TestStatusHandler_ClusterStatus_DefaultUnknown_ReasonPopulated` | PASS |
| `skill.go` does NOT contain `statusHelp = "get_semantic_graph_status: stub help` | PASS |
| `accessors.go` in this plan's diff | NOT PRESENT (closes B1+B4) |

## Acceptance Criteria (must_haves)

- [x] `get_semantic_graph_status` returns SPEC §23.3 envelope with all required fields (truth #1).
- [x] `cluster_status` sourced via `SchedulerAccessor.ClusterStatus` (declared in 64-03's accessors.go); production adapter (P64-08) will return `ClusterStatus{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}` until Phase 65/67 wires a live source — closes checker W1 (truth #2).
- [x] Empty store returns zero-valued envelope with no error (truth #3 — `TestStatusHandler_EmptyStore`).
- [x] Populated store surfaces score_status closed-enum values from Phase 62 RankScheduler (truth #4 — `TestStatusHandler_PopulatedStore` with `exact / stale / approximate` per projection).
- [x] `freshness` enum matches SPEC §26.2 closed values (truth #5 — `TestStatusHandler_FreshnessEnum_AllValuesValid` with closed-enum guard).
- [x] Mode-tier check is read+ — every session passes (truth #6).
- [x] `tools_status.go` is a NEW file; this plan does NOT modify `accessors.go` — closes checker B1+B4 (truth #7). `git diff --name-only HEAD~1 HEAD` lists only `tools_status.go` (new), `tools_status_test.go` (new), and `skill.go` (single-line stub delete).

## Threat Model Coverage

| Threat | Status | Notes |
| --- | --- | --- |
| T-64-06-01 (Information Disclosure — status leaks repo path / file counts) | **accept** | Same data is already surfaced via `get_health` (existing tool); no new disclosure surface. |
| T-64-06-02 (DoS — slow accessor blocks tool call) | **mitigate (closed)** | Each accessor read is O(1) atomic / single SQL row; per-accessor errors are tolerated (logged + zero-valued, not propagated) so a degraded accessor cannot block. TelemetryMiddleware BudgetFunc bounds total tool time. |
| T-64-06-03 (Tampering — spoofed projection name in ScoreStatus) | **mitigate (closed)** | `statusProjections` is a hard-coded package-scope `var` of three strings; caller cannot inject a projection name. |
| T-64-06-04 (Information Disclosure — `ClusterStatus.Reason` leaks internal phase numbering) | **accept** | `phase-62-clustering-no-status-accessor` is a known-public observability anchor; no secrets implicated. |

## Self-Check: PASSED

- File `internal/skill/semantic/tools_status.go` exists.
- File `internal/skill/semantic/tools_status_test.go` exists.
- File `internal/skill/semantic/skill.go` modified (single-line stub deletion of `statusHelp`).
- Commit `0d1e3edf` (feat — `feat(64-06): implement get_semantic_graph_status (TOOL-03)`) found in git log.
- accessors.go NOT in this plan's diff (closes B1+B4 — `git diff --name-only HEAD~1 HEAD` lists only the three plan files).

## Deviations from Plan

None. The plan executed exactly as written.

The original plan suggested a TDD RED→GREEN split could land in two commits, but since the plan declared a single `<task type="auto">` and the test file references the production handler signature directly (RED would have required a panic-stub of `handleGetSemanticGraphStatus`), the executor judged a single `feat` commit with both the impl and tests as cleaner — the resulting commit still has the test file alongside the impl, so the GREEN-phase verification (all 5 tests pass on first run) is preserved in the commit history. Sibling plans 64-04 and 64-05 split RED/GREEN because they had multiple tasks; 64-06 has one task.

## Threat Flags

None — this plan ships only the read-path status surface already enumerated in the plan's `<threat_model>`. No new network endpoints, no new auth paths, no new file access patterns. The handler reads only via narrow accessors (StoreAccessor, SchedulerAccessor, QueueAccessor, LiveAccessor, RetrievalAccessor) declared in 64-03 (frozen).

## Forward Wiring Notes (for downstream plans)

**For P64-08 (daemon wiring):**
- Construct production SchedulerAccessor adapter that delegates `ScoreStatus` to Phase 62 `RankScheduler.ScoreStatus`. For `ClusterStatus`: until Phase 65/67 wires a live cluster-status source, the adapter unconditionally returns `ClusterStatus{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}`. The structured Reason field is the observability anchor — keep the literal string stable so log queries / dashboards keying on it don't break.
- Production RetrievalAccessor adapter delegates `RetrievalPending` to bleve recovery progress (Phase 65 / when retrieval lands). Until then, the adapter returns false unconditionally.
- Call `registerGetSemanticGraphStatus(server, semanticSkill, tracer)` from `internal/daemon/semantic_wiring.go` AFTER `registerIndexSemanticGraph` and `registerRefreshSemanticGraph`.
- No new daemon shutdown ordering needed — status is a pure read path with no goroutines / no cleanup.

**For P64-07 (sibling W2 — get_semantic_context):**
- Helpers in `handler_helpers.go` (`errorResult`, `jsonResult`, `validatePaths`) remain shared and consumed read-only.
- `productionDefaultClusterStatus()` test helper in `tools_status_test.go` is package-local; if 64-07 needs it, copy the literal {state:"unknown", reason:"phase-62-..."} pair (it doesn't compose cleanly across test files since the W1-closure invariant is best documented inline at each call site).
- skill.go single-line stub deletion remains non-conflicting: 64-07 will delete `contextHelp` from the now-single-element stub-var block. After 64-07 lands, the stub block is empty and can be removed entirely (or left empty — choose whichever leaves a cleaner diff at end-of-W2).

**For Phase 65 / 67 (live cluster source):**
- The production SchedulerAccessor adapter is the SINGLE point of change to wire a live cluster-status source. Replace the unconditional unknown return with the live read; the response shape is already in place.
- `cluster_status.reason="phase-62-clustering-no-status-accessor"` is the observability anchor for tracking the gap. Once Phase 65/67 wires a live source, search for that literal string and remove the unconditional default — anywhere it still appears is a wiring bug.

---

*Plan 64-06 — Generated 2026-05-08 — Sequential executor on main working tree*
