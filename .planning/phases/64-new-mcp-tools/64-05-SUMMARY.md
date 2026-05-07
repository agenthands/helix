---
phase: 64
plan: 05
subsystem: refresh-semantic-graph-tool
tags: [mcp-tool, read-tier, snapshot-invariant, no-compaction, lsp-wait, b3-closure, b1-closure, b4-closure]
dependency_graph:
  requires:
    - phase: 64
      plan: 03
      provides: "skill.go (FINAL) + accessors.go (FINAL — including CompactorAccessor) + envelope.go (RefreshResult) + mode_check.go"
    - phase: 64
      plan: 04
      provides: "handler_helpers.go (errorResult / jsonResult / validatePaths) consumed read-only"
  provides:
    - "internal/skill/semantic/tools_refresh.go — RefreshSemanticGraphArgs typed-args struct, registerRefreshSemanticGraph (kernel-style typed-args + WrapToolSpan), testable handleRefreshSemanticGraph method, const refreshHelp"
  affects:
    - phase: 64
      plan: 08
      via: "P64-08 calls registerRefreshSemanticGraph(server, semanticSkill, tracer) from semantic_wiring.go after wiring LiveAccessor / QueueAccessor / StoreAccessor"
tech_stack:
  added: []
  patterns:
    - "Recorder-style mock interfaces for invariant tests — recorderStoreAccessor adds canary Begin/Commit/Abort/Write snapshot methods that t.Fatal on invocation; recorderCompactorAccessor t.Fatals on OnFlush. Three layers of D-09/D-13 enforcement (compile / grep / test)."
    - "Closed-loop ticker polling for LSP-wait — 50ms tick interval, hard cap at max_wait_ms (default 3000)."
    - "Strict-subset path filter (D-11) — args.Paths flows verbatim to live.OnWorkspaceChanged; remainder remains queued for the next refresh or natural coalescer flush."
key_files:
  created:
    - internal/skill/semantic/tools_refresh.go
    - internal/skill/semantic/tools_refresh_test.go
  modified:
    - internal/skill/semantic/skill.go
decisions:
  - "files_updated reports len(args.Paths) when paths supplied, 0 when empty — the LiveAccessor.OnWorkspaceChanged signature returns only an error (no count), and the strict-subset semantics under D-11 mean the count IS the requested-paths cardinality. Documented in refreshHelp Return Shape section."
  - "Forbidden-token comment paraphrasing — comments referring to the snapshot-write surface use 'Begin/Commit/Abort/Write methods on snapshots' rather than the literal identifier strings, because the CI grep gate (! grep -E 'BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts|OnFlush') treats comment hits as violations. Test-side recorder methods retain the literal names because tests are not subject to the same gate (the gate scans only tools_refresh.go)."
  - "LSP-wait poll uses an unconditional DepthAll read at the top of the loop body so a queue that's already drained at entry exits the wait immediately (zero ticks). Without this, a queue at depth 0 with WaitForLSP=true would wait at least one 50ms tick before returning."
  - "ctx-cancellation in the LSP-wait poll uses a select-case-break-then-recheck pattern; Go's break inside a select doesn't exit the for loop, so the loop body explicitly re-checks ctx.Err() to break out. Documented in code comments."
metrics:
  start_time: "2026-05-08T00:00:00Z"
  end_time: "2026-05-08T00:00:00Z"
  duration: "single-session execution"
  task_count: 2
  files_created: 2
  files_modified: 1
  tests_added: 8
  completed_date: "2026-05-08"
---

# Phase 64 Plan 05: refresh_semantic_graph (TOOL-02) Summary

## One-Liner

Implement TOOL-02 (`refresh_semantic_graph`): typed-args registration, mode-tier (read+) check, path-traversal validation, live drain via `LiveAccessor.OnWorkspaceChanged` (D-11 strict-subset), and `wait_for_lsp` polling capped at `max_wait_ms` (D-12). Hard invariant: refresh NEVER calls the snapshot-write surface (D-09) and NEVER triggers the per-workspace compactor (D-13). Three-layer enforcement (compile / grep / test) closes CONTEXT.md acceptance test #2.

## What Shipped

### Task 1: RED gate — failing tests for refresh handler (commit `cbaf0aab`)

**`internal/skill/semantic/tools_refresh.go`** (initial stub) — `RefreshSemanticGraphArgs` typed struct + `handleRefreshSemanticGraph` method body that `panic("not implemented")`. Compile gate so RED-phase tests build.

**`internal/skill/semantic/tools_refresh_test.go`** — 8 tests:

- `TestRefreshHandler_HappyPath_DrainsLive` — 3 paths, store returns graph_version=42, queue depth 0; asserts `RefreshResult{FilesUpdated:3, GraphVersion:42, PendingLSP:false, Freshness: fresh}`.
- `TestRefreshHandler_NoSnapshotCommit_Invariant` (D-09) — recorderStoreAccessor with canary `BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts` methods that `t.Fatal` on invocation. Drives the happy path; asserts the canary counters all sit at zero (belt-and-braces beyond the t.Fatal trip).
- `TestRefreshHandler_NoCompactorCall_Invariant` (D-13) — recorderCompactorAccessor with `OnFlush` that `t.Fatal`s. Drives the happy path; asserts the canary counter sits at zero.
- `TestRefreshHandler_PathsStrictSubset` (D-11) — `args.Paths=["a.go"]`; asserts `live.OnWorkspaceChanged` invoked exactly once with paths=["a.go"] (verbatim).
- `TestRefreshHandler_WaitForLSP_TimeoutReportsPending` (D-12) — queue depth stuck at 5; `WaitForLSP=true, MaxWaitMs=200`; asserts response is `RefreshResult{PendingLSP:true, PendingLSPFiles:5, Freshness: structurally_fresh_semantically_pending}` and elapsed is ~200ms (100..350ms slack).
- `TestRefreshHandler_WaitForLSP_DrainsBeforeTimeout` — queue depth drops to 0 after 50ms; `WaitForLSP=true, MaxWaitMs=2000`; asserts `PendingLSP:false, Freshness: fresh` and elapsed <500ms.
- `TestRefreshHandler_PathTraversalRejected` (T-64-05-01) — `args.Paths=["../../etc"]`; asserts IsError + "path traversal" message + `live.OnWorkspaceChanged` NOT called.
- `TestRefreshHandler_ModeReadAccepted` (T-64-05-02) — session.Mode="read"; asserts handler proceeds (read+ accepts everyone).

**Recorder canary types defined locally in `tools_refresh_test.go`:**
- `recorderStoreAccessor` — implements StoreAccessor (read surface) PLUS extra `BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts` methods that `t.Fatal` on call.
- `recorderCompactorAccessor` — implements CompactorAccessor; `OnFlush` t.Fatals on call.
- `mockLiveAccessor` — records calls with locked slice for race-clean inspection.
- `mockQueueAccessor` — `depthFn` callback so tests can model "drains over time" behavior.

**Verification (RED gate):**
- `go vet ./internal/skill/semantic/... ` PASS
- `go test ./internal/skill/semantic/ -run TestRefreshHandler_` exits NON-ZERO (panic from stub).
- `grep -c "BeginSnapshot\|CommitSnapshot" tools_refresh_test.go` returns **6** (≥4 required).
- `grep -c "CompactorAccessor" tools_refresh_test.go` returns **15** (≥1 required).

### Task 2: GREEN gate — implement handler (commit `d587672b`)

**`internal/skill/semantic/tools_refresh.go`** (full implementation):

- Top-of-file `INVARIANT (D-09 / D-13)` block paraphrases the forbidden surface (Begin/Commit/Abort/Write snapshot methods, compactor flush trigger) without using the literal identifier tokens — the CI grep gate (`! grep -E '(BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts|OnFlush)'`) treats any token hit as a violation, including comment hits.
- `const refreshHelp` — verbose multi-line help text (Usage Examples, Parameters, Return Shape, Mode Tier, Boundary section explicitly stating D-09/D-13 invariant).
- `RefreshSemanticGraphArgs` typed struct with json + jsonschema tags on `Paths`, `WaitForLSP`, `MaxWaitMs`.
- `registerRefreshSemanticGraph(server, s, tracer)` — kernel-style `mcpsdk.AddTool` + `kernel.WrapToolSpan` + `server.Registry().Register(&mcp.ToolDef{..., HelpText: refreshHelp})`.
- `(s *SemanticSkill) handleRefreshSemanticGraph(ctx, args) *mcpsdk.CallToolResult` — testable handler body. Order is load-bearing:
  1. `checkMode(s.sessionSnapshot(ctx), modeTierRead)` — every session passes; retained for code-review visibility and the Phase 66 GuardrailMiddleware precedent.
  2. `validatePaths(args.Paths, ws.RepoRoot)` — rejects `..` and absolute paths outside root.
  3. Default `MaxWaitMs` to 3000 when ≤0.
  4. `s.live.OnWorkspaceChanged(ws, args.Paths)` — drain (D-11 strict subset).
  5. `s.store.CurrentGraphVersion(ctx, ws.Hash())` — read graph_version post-drain.
  6. Initial `pendingLSPFiles = s.queue.DepthAll(ws)`.
  7. If `args.WaitForLSP`: 50ms-ticker poll loop, hard deadline at `time.Now().Add(maxWaitMs * ms)`. Loop body checks depth FIRST so an already-drained queue exits without ticking; on timeout or ctx cancel, breaks out. `pendingLSP = pendingLSPFiles > 0` after the loop.
  8. `overlayActive := s.store.OverlayHasPendingRows(ws.Hash())`.
  9. Freshness selection: `overlay_active` if overlay has pending rows; `structurally_fresh_semantically_pending` if pendingLSP; otherwise `fresh`.
  10. `jsonResult(RefreshResult{...})` — envelope marshal.

**`internal/skill/semantic/skill.go`** (single-line deletion):

```diff
 var (
-	refreshHelp = "refresh_semantic_graph: stub help (replaced by P64-05)"
 	statusHelp  = "get_semantic_graph_status: stub help (replaced by P64-06)"
 	contextHelp = "get_semantic_context: stub help (replaced by P64-07)"
 )
```

`Tools()` still references `refreshHelp` — Go resolves it to the `const refreshHelp` declared at package scope in `tools_refresh.go`. Zero other diff lines in skill.go.

## Verification

| Check                                                                                                 | Result                                          |
| ----------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `go vet ./internal/skill/semantic/...`                                                                | PASS                                            |
| `gofmt -l internal/skill/semantic/{tools_refresh.go,skill.go,tools_refresh_test.go}`                  | empty                                           |
| `go test ./internal/skill/semantic/ -run TestRefreshHandler_ -count=1`                                | PASS — 8/8 tests                                |
| `go test ./internal/skill/semantic/ -race -count=1`                                                   | PASS — full suite race-clean                    |
| `grep -E '(BeginSnapshot\|CommitSnapshot\|AbortSnapshot\|WriteSnapshotFacts\|OnFlush)' tools_refresh.go` | NO MATCHES (D-09 / D-13 grep gate)              |
| `grep -q 'const refreshHelp' tools_refresh.go`                                                        | PASS                                            |
| `grep -q 'refreshHelp = "refresh_semantic_graph: stub help' skill.go`                                 | NO MATCH (stub deleted, PASS)                   |
| `tools_refresh.go` contains `func registerRefreshSemanticGraph(`                                      | PASS                                            |
| `tools_refresh.go` contains `type RefreshSemanticGraphArgs struct`                                    | PASS                                            |
| `tools_refresh.go` calls `checkMode(`                                                                 | PASS                                            |
| `tools_refresh.go` calls `s.workspaceKey(`                                                            | PASS                                            |
| `accessors.go` in this plan's diff                                                                    | NOT PRESENT (B3 closure preserved)              |

## Acceptance Criteria (must_haves)

- [x] `refresh_semantic_graph` drains live changes via `LiveAccessor.OnWorkspaceChanged` (truth #1).
- [x] Refresh NEVER calls the snapshot-write surface or compactor flush trigger (truth #2 — D-09 / D-13). Three-layer enforcement: compile (StoreAccessor omits the surface), grep (no forbidden tokens in this file), test (recorder mocks t.Fatal on invocation).
- [x] `paths` filter is strict subset (truth #3 — D-11). `TestRefreshHandler_PathsStrictSubset` asserts the live accessor receives exactly the requested paths.
- [x] `wait_for_lsp:true` blocks up to `max_wait_ms` via `LaneQueue.DepthAll` polling (truth #4 — D-12). Two tests: timeout case and drain-before-timeout case.
- [x] Mode-tier check is read+ (every session passes); path-traversal validation rejects `..` and out-of-root paths (truth #5).
- [x] Freshness reflects what completed: `fresh` on full LSP drain; `structurally_fresh_semantically_pending` on LSP timeout (truth #6).
- [x] `tools_refresh.go` is a NEW file; this plan does NOT modify `accessors.go` (truth #7 — B3 closure preserved). `git diff --name-only` lists only `tools_refresh.go` (new), `tools_refresh_test.go` (new), and `skill.go` (single-line stub delete).
- [x] All 8 tests pass under `-race`.

## Threat Model Coverage

| Threat                                                                | Status              | Notes                                                                                                                                                            |
| --------------------------------------------------------------------- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T-64-05-01 (Tampering — path traversal in args.Paths)                | **mitigate (closed)** | `validatePaths` rejects `..` and absolute paths outside workspace root. `TestRefreshHandler_PathTraversalRejected` proves rejection.                          |
| T-64-05-02 (EoP — caller bypasses read+ tier)                        | **accept**          | read+ is the floor; modeTierRead returns nil for any session. `TestRefreshHandler_ModeReadAccepted` confirms read mode passes.                                |
| T-64-05-03 (DoS — wait_for_lsp blocks daemon for unbounded time)    | **mitigate (closed)** | MaxWaitMs capped at 3000 default; TelemetryMiddleware BudgetFunc additionally caps total tool time. `TestRefreshHandler_WaitForLSP_TimeoutReportsPending` proves the 200ms cap holds within ±100ms slack. |
| T-64-05-04 (Tampering — refresh writes a snapshot, D-09 violation)  | **mitigate (closed)** | THREE LAYERS: compile-time (StoreAccessor interface omits snapshot-write surface — adding a call would require visibly extending the interface), grep-time (no Begin/Commit/Abort/Write snapshot identifiers in tools_refresh.go), test-time (recorder canary mocks t.Fatal on invocation). |
| T-64-05-05 (DoS — refresh forces compactor, D-13 violation)         | **mitigate (closed)** | Same three-layer story as T-64-05-04 covering the compactor flush trigger. `TestRefreshHandler_NoCompactorCall_Invariant` exercises the recorder canary.       |

## Self-Check: PASSED

- File `internal/skill/semantic/tools_refresh.go` exists.
- File `internal/skill/semantic/tools_refresh_test.go` exists.
- File `internal/skill/semantic/skill.go` modified (single-line stub deletion).
- Commit `cbaf0aab` (Task 1 RED) found in git log: `test(64-05): add failing tests for refresh handler`.
- Commit `d587672b` (Task 2 GREEN) found in git log: `feat(64-05): implement refresh_semantic_graph handler (TOOL-02)`.
- `accessors.go` NOT in this plan's diff (B3 closure preserved — `git diff --name-only HEAD~2 HEAD` lists only tools_refresh.go, tools_refresh_test.go, skill.go).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Forbidden-token strings in doc comments tripped the grep gate**

- **Found during:** Task 2 final verification (`! grep -E '(BeginSnapshot|...|OnFlush)' tools_refresh.go` matched on the INVARIANT comment block at the top of the file plus the in-function HARD INVARIANT comment).
- **Issue:** The plan's verify command treats any occurrence of the forbidden tokens as a gate failure, regardless of whether the occurrence is in code or in a comment. Initial doc comments named the forbidden methods literally for clarity ("MUST NOT call BeginSnapshot, CommitSnapshot, ..."), which the gate rejected.
- **Fix:** Paraphrased both comment blocks to reference "Begin/Commit/Abort/Write methods on snapshots" and "compactor flush trigger" instead of the literal identifier strings. Comment intent is preserved; the grep gate now passes.
- **Files modified:** `internal/skill/semantic/tools_refresh.go`.
- **Commit:** `d587672b` (folded into Task 2 — the comment paraphrasing landed with the impl since both touch the same file in the same gate run).

### Architectural Changes

None.

### Authentication Gates

None.

## Threat Flags

None — this plan ships the refresh-handler surface already enumerated in the plan's `<threat_model>`. The handler reads only via narrow accessors (StoreAccessor, LiveAccessor, QueueAccessor, CompactorAccessor) declared in 64-03 (frozen). No new network endpoints, no new auth paths, no new file access patterns beyond the documented `Paths` parameter (which `validatePaths` constrains).

## Forward Wiring Notes (for downstream plans)

**For P64-08 (daemon wiring):**
- Construct production LiveAccessor adapter that delegates to `live.Service.OnWorkspaceChanged` — daemon owns the translation from `(ws, paths)` to `live.WorkspaceChangeSignal{WorkspaceID: ws, Paths: paths, Source: live.ChangeSourceMCP, ObservedAt: time.Now()}`.
- Construct production QueueAccessor adapter delegating to `lspenrich.LaneQueue.DepthAll` (the production accessor takes no `ws` arg today; the adapter ignores the `ws` parameter from the interface signature).
- Construct production StoreAccessor adapter that exposes ONLY the read surface (`LatestCommittedSnapshot`, `CurrentGraphVersion`, `OverlayHasPendingRows`, `QueryEffectiveAdjacency`); deliberately do NOT add snapshot-write methods to this adapter (D-09 enforcement at the daemon-wiring layer).
- Construct production CompactorAccessor adapter that delegates `OnFlush` to `compact.Compactor.OnFlush`.
- Call `registerRefreshSemanticGraph(server, semanticSkill, tracer)` from `internal/daemon/semantic_wiring.go` after the skill is fully wired and AFTER `registerIndexSemanticGraph`.
- Wire `s.SetLive(...)`, `s.SetQueue(...)`, `s.SetStore(...)`, `s.SetCompactor(...)` via the daemon post-init bundle.

**For P64-06 / P64-07 (siblings in W2):**
- Helpers in `handler_helpers.go` (`errorResult`, `jsonResult`, `validatePaths`) are shared and consumed read-only — do NOT redefine.
- Mock pattern from `tools_refresh_test.go` (recorder canaries for forbidden-call assertions) generalizes; status/context handlers can copy the recorder layout if they need their own invariant assertions.
- skill.go single-line stub deletions remain non-conflicting (P64-06 deletes `statusHelp`, P64-07 deletes `contextHelp`).

---

*Plan 64-05 — Generated 2026-05-08 — Sequential executor on main working tree*
