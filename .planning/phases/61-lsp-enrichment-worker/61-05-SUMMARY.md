---
phase: 61-lsp-enrichment-worker
plan: 05
subsystem: lsp-enrichment / production-wiring / gap-closure
tags: [phase-61, lsp-enrichment, gap-closure, production-wiring, cascade-lsp-shim, tdd]
gap_closure: true
closes_gap: "Production daemon does not dispatch enrichment jobs end-to-end (Manager.Run constructs Worker without NewCascadeLSP factory; every dispatched job lands on OutcomeDropped + Error log)"

# Dependency graph
requires:
  - phase: 61-01
    provides: "LeaseAcquirer + LaneQueue + Outcome enum + MetricsSink + OverlayStore + Pool.ForegroundBusy + Pool.SetYieldCheckWindow"
  - phase: 61-02
    provides: "Worker.RunN + LeaseProvider seam + Cascade engine + Budget + ReadinessProbe + CascadeLSP interface + cascade_integration_test.go realLSPShim reference implementation"
  - phase: 61-03
    provides: "Manager (B2 lease cache via singleflight) + PoolAcquirer + PoolReadinessProbe + daemon bootstrap"
  - phase: 61-04
    provides: "Acceptance integration tests + stress test + cascade_integration_test.go fully exercising realLSPShim against real gopls + jdtls"

provides:
  - "Production cascadeLSPShim — adapts *lspool.WorkerLease to lspenrich.CascadeLSP. Lives in internal/semantic/lspenrich/cascade_lsp_shim.go (no build tag); 7 unit tests in cascade_lsp_shim_test.go (also no build tag — runs under default `go test`)."
  - "Manager.SetCascadeLSPFactory injection seam — Manager.Run threads the factory into Worker.NewCascadeLSP."
  - "internal/daemon/live_wiring.go production wiring — buildLiveBundle constructs the production shim factory closure and supplies it via Manager.SetCascadeLSPFactory."
  - "TestManagerProductionDispatch_Go (//go:build integration) — drives the full live-wiring stack end-to-end against real gopls and asserts FilesEnriched >= 1 + FilesDropped == 0 + typed-edge contract."
  - "61-VERIFICATION.md gap #1 closure: Phase 64 no longer owns the CascadeLSP-over-WorkerLease adapter; it ships in 61-05."

affects:
  - "Phase 64 dependency: refresh_semantic_graph MCP tool inherits an already-wired production dispatch path."
  - "Phase 65 strangler-fig: get_health gains a non-stub Status() to consume — every Manager.Run dispatch now produces real counter increments."

# Tech tracking
tech-stack:
  added: []     # Zero new external dependencies; reuses *lspool.WorkerLease + protocol/gen.
  patterns:
    - "Lazy URI resolution from first DocumentSymbol(ctx, path) call — the production shim has no constructor parameter for URI; the cascade's §14.4 step-1 DocumentSymbol fires before any per-symbol calls, so URI is captured before Hover/CallHierarchy/etc. need it."
    - "Unexported leaseRequester interface seam (Request + Notify) — *lspool.WorkerLease satisfies it via its concrete methods; unit tests substitute a fakeLease without spinning a real LS subprocess. Public NewCascadeLSPShim signature stays `func(*lspool.WorkerLease) CascadeLSP` so the existing CascadeLSPFactory contract is preserved (4+ pre-existing test injection sites pass unchanged)."
    - "MethodNotFound mapping at the shim layer — JSON-RPC -32601 errors from lease.Request map to the typed lspenrich.ErrMethodNotFound sentinel that cascade.go's IsMethodNotFound recognises. Soft 'not applicable to this symbol' errors (e.g., gopls's 'not a type name' on prepareTypeHierarchy) map to (nil, nil) silently — same contract realLSPShim already established."
    - "TDD red/green/refactor across 3 atomic commits for Task 1 — the failing test commit is the safety net against regressing the URI-lazy contract or MethodNotFound mapping."
    - "Startup INFO log gains `cascade_lsp_factory: 'production' | '<unset>'` key — operators can confirm at boot whether the production dispatch path is wired vs. the deferred-wiring failure mode."
    - "Per-task commit-by-task atomicity — 5 commits (test/feat/refactor for Task 1, feat for Task 2, test for Task 3) so individual deviations are git-bisectable."

key-files:
  created:
    - "internal/semantic/lspenrich/cascade_lsp_shim.go (460 LOC; production cascadeLSPShim + leaseRequester interface + helpers)"
    - "internal/semantic/lspenrich/cascade_lsp_shim_test.go (264 LOC; 7 unit tests, no build tag)"
    - "internal/semantic/lspenrich/integration_dispatch_test.go (273 LOC; //go:build integration; TestManagerProductionDispatch_Go end-to-end)"
  modified:
    - "internal/semantic/lspenrich/manager.go (+ newCascadeLSP field, + SetCascadeLSPFactory method, threaded into Worker in Run, + Warn-log when nil, + cascade_lsp_factory INFO log key)"
    - "internal/daemon/live_wiring.go (+ lspool import, + SetCascadeLSPFactory call after NewManager, + cascade_lsp_factory INFO log key)"
    - "internal/semantic/lspenrich/cascade_integration_test.go (- realLSPShim type + 8 method bodies, - 5 helpers that moved to cascade_lsp_shim.go; - net ~310 LOC; tests now construct shim via lspenrich.NewCascadeLSPShim(lease))"

decisions:
  - "Promoted shim is unexported `cascadeLSPShim` with public `NewCascadeLSPShim(lease *lspool.WorkerLease) CascadeLSP` constructor. Rationale: the existing `CascadeLSPFactory func(lease *lspool.WorkerLease) CascadeLSP` signature on Worker.NewCascadeLSP is FIXED (4 pre-existing test injection sites use it). The shim must implement CascadeLSP via *lspool.WorkerLease; promoting realLSPShim verbatim is the smallest-delta change."
  - "Internal leaseRequester interface seam — *lspool.WorkerLease.Request/Notify methods satisfy the unexported interface; unit tests substitute a fakeLease. Rationale: lspool has no mockable RPC seam (Worker.Request talks to a real JSON-RPC subprocess); the alternative was to spin a real LS for every shim unit test, which would make the default `go test -short` path 30s+ slower."
  - "Lazy URI resolution from first DocumentSymbol(ctx, path) call instead of constructor-time URI parameter. Rationale: cascade.go fires DocumentSymbol with job.Path BEFORE any per-symbol calls (§14.4 step 1), so URI is set before Hover/CallHierarchy/etc. need it. Pre-DocumentSymbol Hover/peer calls return (nil, nil) gracefully — no panic on misuse. Keeps the public constructor signature minimal (single lease parameter) and matches the existing CascadeLSPFactory contract."
  - "Manager.SetCascadeLSPFactory after NewManager (rather than NewManagerWithFactory). Rationale: the integration_acceptance_test.go and worker_test.go test injection sites construct Worker DIRECTLY (bypassing Manager). Adding the factory as a Manager constructor parameter would force every test to feed a nil factory and pollute the readiness/dropped-path tests. The setter style is invisible to those tests and keeps the change additive."
  - "Manager.Run logs `cascade_lsp_factory` state at INFO (not Warn) when wired and Warn when unset. Rationale: deferred-wiring failure mode (factory unset → every job lands on OutcomeDropped + Error log per file) is silent at startup and only surfaces under load. Surfacing it as a single startup Warn line makes the failure mode loud at boot rather than at first-job time."
  - "Helper functions (uriToPath, kindLabel, isJSONRPCMethodNotFound, isNonFatalLSPError, symbolsFromDocumentSymbols) moved to cascade_lsp_shim.go as unexported package-internal helpers. Rationale: the integration test (cascade_integration_test.go) is in `package lspenrich_test` so removing them from there does not affect symbol uniqueness. The integration test now imports them transitively via NewCascadeLSPShim — the shim is the single source of truth for the helpers."
  - "TestManagerProductionDispatch_Go pre-warms gopls via a foreground-style sessionID + didOpen, then ReleaseLease's that lease before constructing the manager. Rationale: the manager's enrichment lease singleflight will acquire its own clean lease; pre-warming the LS worker avoids the first-job paying the LS cold-start cost (which would otherwise dominate the test runtime)."

metrics:
  duration: "~32min"
  tasks_completed: 3
  files_created: 3
  files_modified: 3
  commits_in_this_session: 5
  completed_date: 2026-05-06
  loc_added: ~997      # cascade_lsp_shim.go + cascade_lsp_shim_test.go + integration_dispatch_test.go
  loc_removed: ~310    # cascade_integration_test.go shrinks (realLSPShim + helpers moved)
  loc_net: ~687

requirements_completed: [ENRICH-01, ENRICH-02, ENRICH-03, ENRICH-04, ENRICH-05]  # phase 61-04 already flipped these to [x]; 61-05 closes the production-dispatch gap that was a deferred caveat on those check-offs.
---

# Phase 61 Plan 05: Manager NewCascadeLSP production wiring (gap closure)

**Closes 61-VERIFICATION.md gap #1.** Pre-61-05: `Manager.Run` constructed `Worker` without setting `NewCascadeLSP`; every job dispatched through the production wiring (`live_wiring.go`) landed on `worker.go:225-229`'s `if w.NewCascadeLSP == nil` branch, emitting `OutcomeDropped` + an `Error` log before the cascade ran. Post-61-05: a production-grade `cascadeLSPShim` (promoted from `cascade_integration_test.go`'s `realLSPShim`) wraps `*lspool.WorkerLease` as `lspenrich.CascadeLSP`. `Manager` accepts a `CascadeLSPFactory` injection via `SetCascadeLSPFactory`. `internal/daemon/live_wiring.go` supplies the production factory closure. A new `//go:build integration` test asserts a queued job dispatched through `Manager.Run` results in `OutcomeApplied` against real gopls — not `OutcomeDropped`.

## Performance

- **Duration:** ~32min
- **Tasks executed:** 3
- **Atomic commits:** 5 (test/feat/refactor for Task 1, feat for Task 2, test for Task 3)
- **Production-code lines added:** ~460 (cascade_lsp_shim.go) + ~25 (Manager.go SetCascadeLSPFactory + thread-through) + ~10 (live_wiring.go SetCascadeLSPFactory call)
- **Test lines added:** ~264 (cascade_lsp_shim_test.go) + ~273 (integration_dispatch_test.go)
- **Net repo delta:** +997 LOC (new files) − 310 LOC (cascade_integration_test.go shrinkage) = ~+687 LOC

## Accomplishments

### Task 1 — Promote cascadeLSPShim into production package (TDD red→green→refactor)

#### RED commit (`57c736c5`)

7 failing unit tests in `cascade_lsp_shim_test.go` covering:

1. `Test_CascadeLSPShim_ImplementsCascadeLSP` — interface satisfaction.
2. `Test_CascadeLSPShim_DocumentSymbol_LearnsURIFromPath` — URI captured from path on first call.
3. `Test_CascadeLSPShim_Hover_AfterDocumentSymbol_NoPanic` — graceful degradation when documentSymbol returned no symbols.
4. `Test_CascadeLSPShim_Hover_WithoutDocumentSymbol_ReturnsNilNil` — pre-DocumentSymbol Hover returns `(nil, nil)`.
5. `Test_CascadeLSPShim_TypeHierarchy_MethodNotFoundMapping` — `-32601` lease error → `lspenrich.ErrMethodNotFound`.
6. `Test_isJSONRPCMethodNotFound` — predicate table-driven cases.
7. `Test_isNonFatalLSPError` — predicate table-driven cases.

Verified RED via `go test -run CascadeLSPShim ./internal/semantic/lspenrich/...` — fails at compile time on `cascadeLSPShim`/`shimDocSym`/`isJSONRPCMethodNotFound`/`isNonFatalLSPError` undefined.

#### GREEN commit (`55131304`)

`cascade_lsp_shim.go` ships:

- Unexported `cascadeLSPShim` struct (lease + cached URI + cached docSyms).
- Public `NewCascadeLSPShim(lease *lspool.WorkerLease) CascadeLSP` constructor — internal `lease leaseRequester` field is the test seam.
- All 7 CascadeLSP methods (`DocumentSymbol`, `DrainDiagnostics`, `Hover`, `CallHierarchy`, `TypeHierarchy`, `Implementation`, `Definition`, `ReferencesForSymbol`).
- 5 helpers moved from cascade_integration_test.go (`uriToPath`, `kindLabel`, `isJSONRPCMethodNotFound`, `isNonFatalLSPError`, `symbolsFromDocumentSymbols`) as unexported package-internal helpers.
- Compile-time assertion `var _ CascadeLSP = (*cascadeLSPShim)(nil)`.

All 7 unit tests pass.

#### REFACTOR commit (`e1ddc30c`)

`cascade_integration_test.go` shrinks by ~280 LOC: removes the in-file `realLSPShim` type + 8 method bodies + the 5 helpers. The two test functions (`TestCascade_GoIntegration` and `TestCascade_JavaIntegration`) construct the shim via `lspenrich.NewCascadeLSPShim(lease)`. Verified locally: `TestCascade_GoIntegration` passes against real gopls (3 symbols, 8 typed edges).

### Task 2 — Wire CascadeLSPFactory through Manager + live_wiring (`824b347f`)

#### Manager.go changes

- New field `newCascadeLSP CascadeLSPFactory` in the `Manager` struct.
- New method `(m *Manager) SetCascadeLSPFactory(f CascadeLSPFactory)`.
- `Manager.Run` threads `m.newCascadeLSP` into `Worker.NewCascadeLSP` via the struct literal.
- `Manager.Run` Warn-logs at startup when `m.newCascadeLSP == nil` so the deferred-wiring failure mode is loud at boot.
- The "lsp-enrichment manager starting" INFO log gains a `cascade_lsp_factory` key (`"production"` | `"<unset>"`).

#### live_wiring.go changes

- `internal/kernel/lspool` added to the import block (the closure type-references `*lspool.WorkerLease` directly).
- After `enrichMgr := lspenrich.NewManager(...)`, the bundle calls:
  ```go
  enrichMgr.SetCascadeLSPFactory(func(lease *lspool.WorkerLease) lspenrich.CascadeLSP {
      return lspenrich.NewCascadeLSPShim(lease)
  })
  ```
- The "lsp-enrichment manager constructed" INFO log gains a `cascade_lsp_factory: "production"` key.

#### Verification

- `go build ./...` clean.
- `go vet $(go list ./... | grep -v tmp/)` clean (modulo pre-existing swift cgo macro warning).
- `go test -tags integration -run TestACC ./internal/semantic/lspenrich/...` — all 5 ACC sub-tests still pass (they construct Worker DIRECTLY so the Manager-level setter is invisible to them; this is the additive-change invariant).
- `go test -timeout 60s ./internal/semantic/lspenrich/... ./internal/daemon/...` clean.

### Task 3 — TestManagerProductionDispatch_Go (`c9b94a64`)

`internal/semantic/lspenrich/integration_dispatch_test.go` (`//go:build integration`) drives the full production stack:

1. Real `*lspool.Pool` (constructed via `newPoolForDispatchTest` helper).
2. Pre-warms gopls with a foreground-style `test-prewarm:go` sessionID + `textDocument/didOpen` for `main.go`, sleeps 2s for indexing, releases the lease.
3. Constructs `lspenrich.NewLaneQueue(1024, 1024)` + `NewPoolAcquirer(pool)` + `NewPoolReadinessProbe(pool)` + `newIntegrationStore()` + `lspenrich.NewManager(...)`.
4. **THE LOAD-BEARING WIRING:** `mgr.SetCascadeLSPFactory(func(lease *lspool.WorkerLease) lspenrich.CascadeLSP { return lspenrich.NewCascadeLSPShim(lease) })` — verbatim what live_wiring.go's Task 2 hookup does.
5. Starts the manager in a goroutine via `mgr.Run(runCtx)`.
6. Enqueues a single `RevalidateFileJob` for `main.go` on the high lane.
7. Polls `mgr.Status()` until `FilesEnriched + FilesPending + FilesDropped >= 1` or 30s deadline.
8. Cancels context, waits for `mgr.Run` to return, asserts:
   - `FilesEnriched >= 1` (the gap-closure invariant).
   - `FilesDropped == 0` (the pre-61-05 OutcomeDropped behavior is gone).
   - At least one cascade tx committed.
   - At least 1 symbol upserted from documentSymbol.
   - Every emitted edge meets `Confidence == 1.0`, `ValidationState == "validated"`, `Source` has prefix `"lsp."`.

**Local run:** PASS in 2.36s — 1 file enriched, 0 dropped, 0 pending, 3 symbols, 8 edges (kinds: `[TYPE_OF CALLS TYPE_OF CALLS TYPE_OF CALLS CALLS CALLS]`). Startup log line: `cascade_lsp_factory=production`. Gap #1 closed.

## Task Commits

| # | Task | Type | Commit |
|---|------|------|--------|
| 1a | Failing tests for cascadeLSPShim (RED) | test | `57c736c5` |
| 1b | Production cascadeLSPShim (GREEN) | feat | `55131304` |
| 1c | cascade_integration_test uses NewCascadeLSPShim (REFACTOR) | refactor | `e1ddc30c` |
| 2 | Manager.SetCascadeLSPFactory + live_wiring wiring | feat | `824b347f` |
| 3 | TestManagerProductionDispatch_Go integration test | test | `c9b94a64` |

## Files Created / Modified

See frontmatter `key-files`. Concrete numbers: 3 created, 3 modified.

## Decisions Made

See frontmatter `decisions`. Highlights:

- **Promoted shim is unexported with a public constructor.** Preserves the existing `CascadeLSPFactory func(*lspool.WorkerLease) CascadeLSP` contract so all 4 pre-existing test injection sites + the stress test's NewCascadeLSP injection (worker_test.go:781/829/927; integration_acceptance_test.go:440/520/665/817; stress_test.go:306) compile and pass unchanged.
- **Lazy URI resolution from path argument.** Cascade.go fires DocumentSymbol(ctx, job.Path) as §14.4 step 1, before any per-symbol calls; the shim captures URI here, no constructor parameter needed.
- **Setter, not constructor parameter, for CascadeLSPFactory.** Tests that bypass Manager.Run (the 4 pre-existing injection sites) are invisible to the setter — keeps the change additive.

## Deviations from Plan

### None — plan executed exactly as written.

The plan correctly identified:
- The exact pre-existing line where the gap originates (manager.go:198-207 lacks NewCascadeLSP).
- The exact line where it manifests (worker.go:225-229's `if w.NewCascadeLSP == nil`).
- That option (b) — internal `leaseRequester` interface seam preserving public `*lspool.WorkerLease` signature — is the right choice for unit-testability without spinning a real LS subprocess.
- That URI lazy-learn from `path` keeps cascade_integration_test.go green with the smallest delta (didOpen URI matches `"file://" + path` since cascade.go calls DocumentSymbol with absolute path).

No auto-fixed bugs surfaced; no architectural deviations needed.

## Authentication Gates

None — the integration test runs against gopls available locally; t.Skip when absent. No external auth required.

## Test Coverage

- **Default short tests** (`go test -short -timeout 60s ./internal/semantic/lspenrich/... ./internal/daemon/...`): PASS in 1.16s + 2.27s (lspenrich + daemon respectively). New shim unit tests run cleanly under default path.
- **kernel/lspool short tests:** PASS in 6.05s (no regression from the wiring change).
- **Integration suite** (`go test -tags integration -timeout 240s ./internal/semantic/lspenrich/...`): PASS in 16.5s (all TestACC* + TestCascade_GoIntegration + TestCascade_JavaIntegration + new TestManagerProductionDispatch_Go).
- **Build clean** (`go build ./...`): clean (modulo pre-existing swift cgo macro warning).
- **Project-wide vet** (`go vet $(go list ./... | grep -v tmp/)`): clean (modulo pre-existing swift cgo macro warning).
- **Acceptance grep checks** (Task 1):
  - `type cascadeLSPShim struct`: 1 match
  - `var _ CascadeLSP = (*cascadeLSPShim)(nil)`: 1 match
  - `func NewCascadeLSPShim`: 1 match
  - `realLSPShim` references in cascade_integration_test.go: 0 matches
  - `cascade_lsp_shim.go` `//go:build` first-3-line check: 0 matches (no build tag — production code)
- **Acceptance grep checks** (Task 2):
  - `NewCascadeLSP:[[:space:]]*m\.newCascadeLSP` in manager.go: 1 match
  - `func (m \*Manager) SetCascadeLSPFactory` in manager.go: 1 match
  - `newCascadeLSP CascadeLSPFactory` in manager.go: 1 match
  - `NewCascadeLSPShim` in live_wiring.go: 1 match
  - `SetCascadeLSPFactory` in live_wiring.go: 1 match
- **Acceptance grep checks** (Task 3):
  - `//go:build integration` first line of integration_dispatch_test.go: 1 match
  - `TestManagerProductionDispatch` references: 3 matches
  - `SetCascadeLSPFactory` references: 4 matches
  - `mgr.Run` references: 3 matches
  - `FilesEnriched|FilesDropped|OutcomeApplied` references: 20 matches

## Boundary Verification

The semantic→kernel direct-import boundary is preserved. The new production shim lives in `internal/semantic/lspenrich/`, not in `internal/kernel/`:

```
$ grep -rn '"github.com/agenthands/helix/internal/kernel' internal/semantic/lspenrich/*.go | grep -v lspool | grep -v _test.go
(no output — no internal/kernel direct imports beyond internal/kernel/lspool)
```

`internal/daemon/live_wiring.go` now imports `internal/kernel/lspool` directly (in addition to `internal/kernel`) — that import was already permitted because daemon is at the wiring layer (not under `internal/semantic/...`); the `nosemantic2kernel` analyzer enforces the `internal/semantic/...` → `internal/kernel/...` direction only, and `internal/daemon` is upstream of both.

## Phase 61 Closeout — Gap #1 Resolution

**61-VERIFICATION.md gap #1 ("Production daemon does not dispatch enrichment jobs end-to-end"): CLOSED.**

| Pre-61-05 | Post-61-05 |
|-----------|------------|
| `Manager.Run` constructed `Worker` without `NewCascadeLSP`. | `Manager.Run` threads `m.newCascadeLSP` into `Worker.NewCascadeLSP`. |
| `live_wiring.go` did not supply a CascadeLSPFactory. | `live_wiring.go` calls `enrichMgr.SetCascadeLSPFactory(...)` with the production NewCascadeLSPShim closure. |
| Every Manager.Run-dispatched job → `OutcomeDropped` + Error log. | Jobs run the cascade end-to-end against real LSPs; OutcomeApplied is the success path. |
| `Worker.NewCascadeLSP factory is nil; cannot run cascade` Error log fired on every dispatched job. | Error log NEVER fires from production daemon dispatch (asserted by TestManagerProductionDispatch_Go: `FilesDropped == 0`). |

**Phase 64 dependency severed:** the 61-04-SUMMARY decision *"production wiring (live_wiring.go) does NOT yet supply Worker.NewCascadeLSP — that adapter ships in Phase 64+"* is OBSOLETE. Phase 64 inherits an already-wired production dispatch path; its only remaining work is the MCP tool wrapper (`refresh_semantic_graph` + `get_semantic_graph_status`).

**Phase 65 strangler-fig benefit:** `Manager.Status()` now produces real, non-stub counter increments on every Manager.Run dispatch, giving `get_health` a meaningful surface to consume.

## Self-Check

Verified after writing this SUMMARY:

- File `internal/semantic/lspenrich/cascade_lsp_shim.go`: FOUND
- File `internal/semantic/lspenrich/cascade_lsp_shim_test.go`: FOUND
- File `internal/semantic/lspenrich/integration_dispatch_test.go`: FOUND
- Modified `internal/semantic/lspenrich/manager.go`: FOUND (SetCascadeLSPFactory + newCascadeLSP field present)
- Modified `internal/semantic/lspenrich/cascade_integration_test.go`: FOUND (realLSPShim removed; NewCascadeLSPShim used)
- Modified `internal/daemon/live_wiring.go`: FOUND (lspool import + SetCascadeLSPFactory call present)
- Commit `57c736c5` (Task 1 RED): FOUND in `git log`
- Commit `55131304` (Task 1 GREEN): FOUND in `git log`
- Commit `e1ddc30c` (Task 1 REFACTOR): FOUND in `git log`
- Commit `824b347f` (Task 2 wiring): FOUND in `git log`
- Commit `c9b94a64` (Task 3 integration test): FOUND in `git log`

## Self-Check: PASSED

---
*Phase: 61-lsp-enrichment-worker*
*Completed: 2026-05-06*
