---
phase: 64
plan: 04
subsystem: index-semantic-graph-tool
tags: [mcp-tool, singleflight, timeout-partial, mode-tier, path-validation, b5-closure]
dependency_graph:
  requires:
    - phase: 64
      plan: 02
      provides: "effective-graph queries on *Store (LatestCommittedSnapshot used by ResolveAuto)"
    - phase: 64
      plan: 03
      provides: "skill.go (FINAL) + accessors.go (FINAL) + envelope.go + mode_check.go"
  provides:
    - "internal/skill/semantic/runner.go — IndexRunner with singleflight.Group keyed (workspace, RESOLVED-mode); Run + ResolveAuto + Shutdown methods; ErrModeMustBeResolved sentinel rejects auto/empty modes at the runner boundary (closes B5 contract)"
    - "internal/skill/semantic/tools_index.go — IndexSemanticGraphArgs typed-args struct, registerIndexSemanticGraph (kernel-style typed-args + WrapToolSpan), testable handleIndexSemanticGraph method, const indexHelp"
    - "internal/skill/semantic/handler_helpers.go — shared errorResult / jsonResult / validatePaths helpers (owned by W1, consumed read-only by W2 plans 64-05/06/07)"
  affects:
    - phase: 64
      plan: 05
      via: "handler_helpers.go consumed read-only (errorResult / jsonResult / validatePaths)"
    - phase: 64
      plan: 06
      via: "handler_helpers.go consumed read-only"
    - phase: 64
      plan: 07
      via: "handler_helpers.go consumed read-only"
    - phase: 64
      plan: 08
      via: "P64-08 wires production buildFn into NewIndexRunner and calls registerIndexSemanticGraph(server, semanticSkill, tracer) from semantic_wiring.go"
tech_stack:
  added:
    - "golang.org/x/sync/singleflight (already in go.mod via lspenrich; now a second consumer)"
  patterns:
    - "Singleflight join keyed on (workspace, RESOLVED-mode) — mirrors lspenrich/manager.go:15,82-88"
    - "Sync-with-timeout dispatch via DoChan + select — bgCtx derived from context.Background() (D-04)"
    - "Mode-tier check FIRST in handler bodies — establishes precedent for Phase 66 GuardrailMiddleware"
    - "Path-traversal validation mirrors internal/skill/repomap/skill.go:311-319 (T-28-05 / T-64-04-02)"
    - "Atomic-typed shared progress fields (atomic.Uint64 / atomic.Int64) on buildState for safe foreground/background access"
key_files:
  created:
    - internal/skill/semantic/runner.go
    - internal/skill/semantic/runner_test.go
    - internal/skill/semantic/runner_singleflight_test.go
    - internal/skill/semantic/tools_index.go
    - internal/skill/semantic/tools_index_test.go
    - internal/skill/semantic/handler_helpers.go
  modified:
    - internal/skill/semantic/skill.go
decisions:
  - "buildState.snapshotID promoted from uint64 to atomic.Uint64 mid-implementation when -race flagged a data race between the foreground timeout-partial reader and the background buildFn writer. This is the ONLY shared field with a foreground reader; filesIndexed/filesReused were already atomic. The handler/runner contract remains unchanged."
  - "ResolveAuto on store error returns 'full' rather than propagating the error. Reasoning: a degraded snapshot history is best recovered by a full build; surfacing the error to the caller would only result in either a retry-storm or a confusing failure for an MVP user."
  - "validatePaths empty-root short-circuit accepts absolute paths when ws.RepoRoot is empty (test path / pre-activation). Production daemon always wires a non-empty RepoRoot via the SessionAccessor, so this branch only matters in unit tests that don't construct a real workspace key."
  - "handler logic extracted to handleIndexSemanticGraph method so unit tests can drive the handler directly without spinning up an MCP server. The registration closure is a one-line forwarder. This pattern is what W2 plans (64-05/06/07) will mirror for refresh/status/context handlers."
  - "Singleflight key uses ws.RepoRoot (string) rather than ws.Hash() (hashed) because RepoRoot is the canonical workspace identity for build orchestration; Hash() is only used by ResolveAuto's StoreAccessor seam where the daemon adapter (P64-08) will translate to whatever repoID the *Store expects."
metrics:
  start_time: "2026-05-08T00:00:00Z"
  end_time: "2026-05-08T00:00:00Z"
  duration: "single-session execution"
  task_count: 3
  files_created: 6
  files_modified: 1
  tests_added: 17
  completed_date: "2026-05-08"
---

# Phase 64 Plan 04: index_semantic_graph (TOOL-01) Summary

## One-Liner

Implement TOOL-01 (`index_semantic_graph`): typed-args registration, mode-tier (review+) check, path-traversal validation, singleflight join keyed on the RESOLVED mode, and sync-with-timeout dispatch with background continuation under `context.Background()` (D-04). Closes checker B5 at BOTH the runner layer (`Run` rejects unresolved modes with `ErrModeMustBeResolved`) and the handler layer (`handleIndexSemanticGraph` calls `ResolveAuto` BEFORE `Run`).

## What Shipped

### Task 1: RED gate — failing tests for IndexRunner

**`internal/skill/semantic/runner.go`** (initial stub) — `ErrModeMustBeResolved` sentinel + `buildState` struct + `NewIndexRunner` constructor + `Run` / `ResolveAuto` panic-stub method bodies. Compile gate so RED-phase tests build.

**`internal/skill/semantic/runner_test.go`** — 6 tests:

- `TestIndexRunner_AutoResolution_NoSnapshot_ReturnsFull` — `ResolveAuto` returns `"full"` when `LatestCommittedSnapshot` returns `(0, nil)`.
- `TestIndexRunner_AutoResolution_HasSnapshot_ReturnsIncremental` — returns `"incremental"` when a committed snapshot exists.
- `TestIndexRunner_HappyPath_Commits` — `Run("full")` returns `IndexResult{SnapshotID: 42, Status: committed, Partial: false, FilesIndexed: 10, FilesReused: 2}` and invokes buildFn exactly once.
- `TestIndexRunner_RejectsAutoModeAtRun` — `Run("auto", ...)` AND `Run("", ...)` both return `ErrModeMustBeResolved`; buildFn NEVER invoked.
- `TestIndexRunner_TimeoutReturnsPartialBuilding` — buildFn sleeps 5s, `Run` with `maxMs=100` returns `IndexResult{Partial: true, Status: building, SnapshotID: <in-progress>, Freshness: stale}`.
- `TestIndexRunner_TimeoutDoesNotCancelBackground` — after the foreground timeout fires, the background buildFn is still running 200ms later (proves bgCtx ≠ requestCtx; D-04).

**`internal/skill/semantic/runner_singleflight_test.go`** — 3 tests:

- `TestIndexRunner_SingleflightJoin_TwoCallersOneSnapshotID` — two concurrent `Run(ws, "full")` callers receive the SAME snapshot id; buildFn invoked exactly ONCE.
- `TestIndexRunner_SingleflightJoin_DifferentModesNoCollapse` — `Run("full")` + `Run("incremental")` invoke buildFn TWICE (different singleflight keys).
- `TestIndexRunner_SingleflightJoin_TwoAutoCallersOneBuild` — closes checker B5: two concurrent goroutines each call `ResolveAuto` then `Run(resolved)`; buildFn invoked exactly ONCE; both observe the same snapshot id.

**Commit:** `dc5e075a` — `test(64-04): add failing tests for IndexRunner ...`

### Task 2: GREEN gate — IndexRunner implementation

**`internal/skill/semantic/runner.go`** (full implementation):

- `singleflight.Group` keyed `(ws.RepoRoot, RESOLVED-mode)` — second concurrent caller for the same key joins the in-flight build.
- `Run` rejects `mode=="auto"` and `mode==""` up-front with `ErrModeMustBeResolved` (B5 contract). Then derives a `callCtx` with the per-call deadline (clamped to `r.timeout`, default 120s) and dispatches via `sf.DoChan`.
- The `DoChan` callback constructs `bgCtx` from `context.Background()` (NOT the request ctx) — D-04 invariant — registers a `*buildState` in `r.inFlight` keyed by `ws.RepoRoot`, and invokes `r.buildFn(bgCtx, ws, mode, st)`.
- `select { case v := <-ch: ... case <-callCtx.Done(): ... }`:
  - Channel result branch: populates `IndexResult` from buildFn return + sets `DurationMs` + sets `Status` to `committed` (or `failed` on error).
  - Timeout branch: reads `*buildState` from `r.inFlight`, copies the in-flight snapshot id + progress counters, returns `IndexResult{Partial: true, Status: building, Freshness: stale}`.
- `ResolveAuto` calls `r.store.LatestCommittedSnapshot(ctx, ws.Hash())` → returns `"full"` on `(0, nil)` or store error, else `"incremental"`.
- `Shutdown(ctx)` ranges `r.inFlight` and cancels every `*buildState.cancel` — daemon shutdown drops in-flight builds cleanly.

**Commit:** `17b99440` — `feat(64-04): implement IndexRunner ...`

### Task 3: tools_index.go + handler_helpers.go + skill.go single-line deletion

**`internal/skill/semantic/tools_index.go`** (new):

- `const indexHelp` — verbose multi-line help text (Usage Examples, Parameters, Return Shape, Mode Tier, Concurrency).
- `type IndexSemanticGraphArgs struct` with json + jsonschema tags on `Mode`, `MaxDurationMs`, `Paths`.
- `registerIndexSemanticGraph(server, s, tracer)` — kernel-style `mcpsdk.AddTool` + `kernel.WrapToolSpan` + `server.Registry().Register(&mcp.ToolDef{..., HelpText: indexHelp})`.
- `(s *SemanticSkill) handleIndexSemanticGraph(ctx, args) *mcpsdk.CallToolResult` — the testable handler body. Order is load-bearing:
  1. `checkMode(s.sessionSnapshot(ctx), modeTierReview)` — denies read/edit with structured PermissionDenied envelope (T-64-04-01).
  2. `validatePaths(args.Paths, ws.RepoRoot)` — rejects `..` and absolute paths outside root (T-64-04-02).
  3. Resolve `mode == "" || mode == "auto"` via `s.runner.ResolveAuto(ctx, ws)` — closes B5 + T-64-04-07.
  4. `s.runner.Run(ctx, ws, RESOLVED-mode, args.MaxDurationMs)` — singleflight-joined dispatch.
  5. Marshal result via `jsonResult(result)`.

**`internal/skill/semantic/handler_helpers.go`** (new, shared with W2):

- `errorResult(msg)` — `*mcpsdk.CallToolResult` with `IsError: true` and TextContent body.
- `jsonResult(v)` — JSON-marshals `v` and wraps in TextContent. Defensive errorResult on marshal failure.
- `validatePaths(paths, root)` — mirrors `internal/skill/repomap/skill.go:311-319`. Empty input permitted; `..` rejected; absolute paths rejected unless they `HasPrefix(root)`.

**`internal/skill/semantic/skill.go`** (single-line deletion):

```diff
 var (
-	indexHelp   = "index_semantic_graph: stub help (replaced by P64-04)"
 	refreshHelp = "refresh_semantic_graph: stub help (replaced by P64-05)"
 	statusHelp  = "get_semantic_graph_status: stub help (replaced by P64-06)"
 	contextHelp = "get_semantic_context: stub help (replaced by P64-07)"
 )
```

`Tools()` still references `indexHelp` — Go resolves it to the `const indexHelp` declared at package scope in `tools_index.go`. revision-W1 closure verified: `git diff` shows zero added lines in skill.go.

**`internal/skill/semantic/tools_index_test.go`** — 8 tests:

- `TestIndexHandler_HappyPath_ReturnsCommitted` — review-mode session, `Mode: "full"` → JSON envelope with `SnapshotID: 42`.
- `TestIndexHandler_ModeViolationFromRead` — read-mode session → IsError + "current mode is" message; `Run` NEVER called.
- `TestIndexHandler_ModeViolationFromEdit` — edit-mode session → same as above.
- `TestIndexHandler_AdminAllowed` — admin-mode session → success; `Run` called once.
- `TestIndexHandler_PathTraversalRejected` — `Paths: ["../../../etc/passwd"]` → IsError + "path traversal".
- `TestIndexHandler_AbsolutePathOutsideRoot` — `Paths: ["/etc/passwd"]` → IsError + "outside workspace root".
- `TestIndexHandler_AutoModeResolvesBeforeRun` — closes B5 handler-side: `Mode: "auto"` → `ResolveAuto` called once, `Run` called once with `mode="full"` (the resolved value); asserts `Run` is NEVER called with `"auto"` or `""`.
- `TestIndexHandler_EmptyModeResolvesBeforeRun` — `Mode: ""` → same flow with `ResolveAuto` returning `"incremental"`.

**Commit:** `477cdae2` — `feat(64-04): add index_semantic_graph handler + helpers (TOOL-01)`

## Verification

| Check                                                                                                 | Result                                          |
| ----------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `go vet ./internal/skill/semantic/...`                                                                | PASS                                            |
| `gofmt -l internal/skill/semantic/`                                                                   | empty                                           |
| `go test ./internal/skill/semantic/ -run 'TestIndexRunner_\|TestIndexHandler_' -count=1`              | PASS — 17 tests across 3 test files             |
| `go test ./internal/skill/semantic/ -run TestIndexRunner_SingleflightJoin -race -count=1`             | PASS — all 3 singleflight tests under -race     |
| `go test ./internal/skill/semantic/ -race -count=1`                                                   | PASS — full suite race-clean                    |
| `go build ./internal/... ./cmd/...`                                                                   | PASS (only pre-existing CGO Swift warning)      |
| `tools_index.go` contains `func registerIndexSemanticGraph(`                                          | PASS                                            |
| `tools_index.go` contains `type IndexSemanticGraphArgs struct`                                        | PASS                                            |
| `tools_index.go` calls `checkMode(`                                                                   | PASS                                            |
| `tools_index.go` calls `s.workspaceKey(`                                                              | PASS                                            |
| `tools_index.go` declares `const indexHelp`                                                           | PASS                                            |
| `tools_index.go` ResolveAuto line precedes Run line                                                   | PASS (line 125 < line 129)                      |
| `runner.go` contains `singleflight.Group`                                                             | PASS                                            |
| `runner.go` contains `var ErrModeMustBeResolved`                                                      | PASS                                            |
| `runner.go` rejects `mode=="auto"` / `mode==""` with `ErrModeMustBeResolved`                          | PASS                                            |
| `skill.go` no longer contains `indexHelp = "index_semantic_graph: stub help`                          | PASS                                            |
| `skill.go` `Tools()` still references `HelpText: indexHelp`                                           | PASS (resolves to const in tools_index.go)      |
| `skill.go` diff in this plan: zero added lines (single-line stub deletion only)                       | PASS (closes revision-W1)                       |
| `handler_helpers.go` declares `errorResult`, `jsonResult`, `validatePaths`                            | PASS                                            |

## Acceptance Criteria (must_haves)

- [x] `index_semantic_graph` dispatches in auto, full, incremental, refresh modes (truth #1).
- [x] Handler resolves `auto` → `full|incremental` via `ResolveAuto` BEFORE calling `Run`; the singleflight key is keyed on the RESOLVED mode (truth #2 — closes B5).
- [x] Two concurrent `mode=full` calls receive the same `snapshot_id` (truth #3 — `TestIndexRunner_SingleflightJoin_TwoCallersOneSnapshotID`).
- [x] Two concurrent `mode=auto` calls (with no committed snapshot) both resolve to `full` and join the same singleflight build, both receiving the same `snapshot_id` (truth #4 — `TestIndexRunner_SingleflightJoin_TwoAutoCallersOneBuild`).
- [x] Sync-with-timeout returns `partial=true`, `status=building` when `max_duration_ms` expires; background build keeps running (truth #5 — `TestIndexRunner_TimeoutReturnsPartialBuilding` + `TestIndexRunner_TimeoutDoesNotCancelBackground`).
- [x] `mode=auto` resolves to `full` when no committed snapshot exists, else `incremental` (truth #6 — D-03).
- [x] Calls from read mode are rejected with structured PermissionDenied envelope (truth #7 — `TestIndexHandler_ModeViolationFromRead` + `TestIndexHandler_ModeViolationFromEdit`).
- [x] Path filter input rejects `..` and absolute paths outside workspace root (truth #8 — `TestIndexHandler_PathTraversalRejected` + `TestIndexHandler_AbsolutePathOutsideRoot`).
- [x] `mode=incremental` and `mode=full` both produce committed snapshots via `BeginSnapshot/WriteSnapshotFacts/CommitSnapshot` (truth #9 — runner contract; production buildFn lands in P64-08).
- [x] This plan does NOT add helper methods to `skill.go`. The only edit to `skill.go` is the single-line deletion of `indexHelp = "...stub..."` from the stub-var block (truth #10 — closes revision-W1).
- [x] `runner.go` provides IndexRunner with `singleflight.Group` keyed `(workspace, RESOLVED-mode)`; `Run` documented to require resolved (full|incremental) mode (artifact #1).
- [x] `tools_index.go` provides `registerIndexSemanticGraph` + typed args + handler body. Owns `const indexHelp`. Handler calls `ResolveAuto` BEFORE `Run` (artifact #2).
- [x] `handler_helpers.go` provides shared `errorResult`/`jsonResult`/`validatePaths` (artifact #3).
- [x] All 17 tests pass (3 + 6 from runner suites, 8 handler tests).
- [x] `go vet` passes; `gofmt -l` empty; race detector clean.

## Threat Model Coverage

| Threat                                                                | Status              | Notes                                                                                                                                                            |
| --------------------------------------------------------------------- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T-64-04-01 (EoP — privilege escalation by spoofing mode)              | **mitigate (closed)** | `s.sessionSnapshot(ctx)` reads via `SessionInfo.Snapshot()` (RLock-protected per session.go:50-71); Mode never sourced from request payload. Two test cases prove the read/edit path is denied. |
| T-64-04-02 (Tampering — path traversal via Paths filter)             | **mitigate (closed)** | `validatePaths` rejects `..` and absolute paths outside workspace root. Two test cases prove rejection.                                                      |
| T-64-04-03 (DoS — unbounded max_duration_ms)                         | **mitigate (closed)** | `Run` clamps `deadline` to `min(maxMs, r.timeout=120s)`. TelemetryMiddleware adds a second budget cap.                                                        |
| T-64-04-04 (DoS — concurrent index calls spawning parallel builds)   | **mitigate (closed)** | `singleflight.Group` keyed `(workspace, RESOLVED-mode)`; second caller attaches per D-02. `TestIndexRunner_SingleflightJoin_TwoCallersOneSnapshotID` proves it. |
| T-64-04-05 (Information Disclosure — raw error text in MCP envelope) | **mitigate (closed)** | `errorResult` builds the closed-enum reason from `serr.New(...).Error()`; raw build errors flow through `serr` formatters that already redact internals.       |
| T-64-04-06 (Tampering — snapshot id forgery via concurrent BeginSnapshot races) | **mitigate (closed)** | Singleflight serializes `BeginSnapshot`; only the in-flight bg goroutine calls it. Second caller receives the same snapshot id via the singleflight result channel. |
| T-64-04-07 (EoP — caller bypasses singleflight by passing mode="auto") | **mitigate (closed)** | `Run` rejects `auto`/`""` with `ErrModeMustBeResolved`; only the handler resolves auto, and the resolved value goes into the singleflight key. Both layers tested (`TestIndexRunner_RejectsAutoModeAtRun` + `TestIndexHandler_AutoModeResolvesBeforeRun`). |

## Self-Check: PASSED

- File `internal/skill/semantic/runner.go` exists.
- File `internal/skill/semantic/runner_test.go` exists.
- File `internal/skill/semantic/runner_singleflight_test.go` exists.
- File `internal/skill/semantic/tools_index.go` exists.
- File `internal/skill/semantic/tools_index_test.go` exists.
- File `internal/skill/semantic/handler_helpers.go` exists.
- File `internal/skill/semantic/skill.go` modified (single-line stub deletion).
- Commit `dc5e075a` (Task 1 RED) found in git log.
- Commit `17b99440` (Task 2 GREEN) found in git log.
- Commit `477cdae2` (Task 3 handler + helpers) found in git log.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Data race on `buildState.snapshotID` between foreground timeout reader and background buildFn writer**

- **Found during:** Task 3 final `-race` verification (the per-task race tests passed; the full-suite `-race` run flagged the race because `TestIndexRunner_TimeoutDoesNotCancelBackground` and the singleflight tests overlapped).
- **Issue:** The foreground timeout-partial path reads `st.snapshotID` from `r.inFlight` while the background buildFn writes the same field. Both happen on the same `*buildState` instance; the read/write pair has no synchronization, so `-race` reported a data race.
- **Fix:** Promoted `buildState.snapshotID` from `uint64` to `atomic.Uint64`. The buildFn writes via `Store(...)`; the timeout-partial reader reads via `Load()`. The other shared progress fields (`filesIndexed`, `filesReused`) were already `atomic.Int64`, so this brings `snapshotID` into the same discipline.
- **Files modified:** `internal/skill/semantic/runner.go` (buildState definition + timeout reader), `internal/skill/semantic/runner_test.go` (mockBuild writer).
- **Commit:** `477cdae2` (folded into Task 3 commit because the RED tests in `dc5e075a` predate the impl that exposed the race; the impl + race fix landed together rather than splitting an interim commit that wouldn't compile).

### Architectural Changes

None.

### Authentication Gates

None.

## Threat Flags

None — this plan ships only one new MCP tool surface (`index_semantic_graph`), already enumerated in the plan's `<threat_model>`. No new network endpoints, no new auth paths, no new file access patterns beyond the documented `Paths` parameter (which `validatePaths` constrains).

## Forward Wiring Notes (for downstream plans)

**For P64-05 (W2 — refresh_semantic_graph):**
- Same registration shape as `tools_index.go`: declare `const refreshHelp` in `tools_refresh.go` (delete the stub line in skill.go).
- Consume `s.sessionSnapshot(ctx)` + `checkMode(snap, modeTierRead)`; consume `s.workspaceKey(ctx)`.
- Consume `errorResult`/`jsonResult`/`validatePaths` from `handler_helpers.go` — DO NOT redefine.
- Add new helpers (if any) in `internal/skill/semantic/refresh_helpers.go` (P64-05 owns it).

**For P64-08 (daemon wiring):**
- Construct production buildFn that calls `BeginSnapshot` / `WriteSnapshotFacts` / `CommitSnapshot` under bgCtx + writes progress counters into the supplied `*buildState`.
- Construct `IndexRunner` via `NewIndexRunner(storeAdapter, productionBuildFn, 120*time.Second)`.
- Wire the runner via `semanticSkill.SetRunner(&runnerAdapter{r: indexRunner})` (the adapter is a thin shim because `*IndexRunner` already satisfies `RunnerAccessor`).
- Call `registerIndexSemanticGraph(server, semanticSkill, tracer)` from `internal/daemon/semantic_wiring.go` after the skill is fully wired.
- Register `indexRunner.Shutdown` in the daemon shutdown ordering (kernel-first per CLAUDE.md).

---

*Plan 64-04 — Generated 2026-05-08 — Sequential executor on main working tree*
