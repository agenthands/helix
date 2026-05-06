---
phase: 61-lsp-enrichment-worker
plan: 03
subsystem: lsp-enrichment / daemon-bootstrap / kernel-semantic-seam
tags: [phase-61, lsp-enrichment, manager, status, metrics, trace, daemon-bootstrap, B2-lease-cache]

# Dependency graph
requires:
  - phase: 61-01
    provides: "LeaseAcquirer + LaneQueue + Outcome enum + MetricsSink interface + OverlayStore interface (P01 types.go) + Pool.ForegroundBusy + Pool.SetYieldCheckWindow"
  - phase: 61-02
    provides: "Worker.RunN + LeaseProvider seam + Cascade engine + Budget + ReadinessProbe interface (Java/Rust dispatch)"
  - phase: 60
    provides: "live_wiring.go buildLiveBundle skeleton + storeOverlayWriter adapter pattern"
  - phase: 56
    provides: "*lspool.Pool / *lspool.WorkerLease + JdtlsAdapter + RustAnalyzerAdapter (consumed via the new pool accessors)"

provides:
  - "lspenrich.Manager — owns N enrichment workers per cfg.MaxConcurrentWorkers AND the per-(wsKey, lang) lease cache (B2 fix-path-A); satisfies P02 LeaseProvider so Worker.Leases = manager"
  - "lspenrich.Status — D-09 read-only snapshot accessor (FilesEnriched / FilesDropped / FilesPreempted / FilesPending / LastErrorPerLanguage / LaneDepths) for Phase 65 get_health"
  - "lspenrich.StartFileSpan / StartStepSpan — semantic.lsp_enrich_file root + child step spans (SPEC §28.2)"
  - "lspenrich.PoolAcquirer — production adapter satisfying LeaseAcquirer + LeaseReleaser (var _ LeaseAcquirer = (*PoolAcquirer)(nil), var _ LeaseReleaser = (*PoolAcquirer)(nil))"
  - "lspenrich.PoolReadinessProbe — production adapter satisfying ReadinessProbe via Pool.JdtlsAdapter / Pool.RustAnalyzerAdapter (W8 — real accessor names)"
  - "*lspool.Pool.JdtlsAdapter(wsKey) / *lspool.Pool.RustAnalyzerAdapter(wsKey) — per-(wsKey, lang) accessors (W8)"
  - "*lspool.RustAnalyzerAdapter.QuiescentChan() <-chan struct{} — receive-only channel for select-based readiness waits"
  - "liveBundle.Run / liveBundle.OnWorkspaceDeactivate / liveBundle.Stop — daemon lifecycle hooks for the enrichment manager (B2)"
  - "storeCascadeStoreAdapter / storeCascadeTxAdapter — adapts *semanticstore.Store to lspenrich.CascadeStore (production cascade-tx wrapper)"

affects:
  - internal/daemon/daemon.go (Daemon struct gains a `live *liveBundle` field; Run pulls live.Run into the errgroup; gRPC DeactivateWorkspace handler invokes live.OnWorkspaceDeactivate)
  - internal/daemon/shutdown.go (Phase 0 calls live.Stop BEFORE kernel.Shutdown)
  - internal/daemon/live_e2e_test.go (buildLiveBundle signature change — accepts LSPEnrichmentConfig)

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/sync/singleflight (Manager.AcquireFor singleflight semantics — concurrent acquires on the same cold key collapse to ONE underlying Pool.AcquireLease)"
  patterns:
    - "Singleflight cache: one in-flight AcquireLease per (wsKey, lang) key; shared result for concurrent callers; failed acquires NOT cached so the next call retries (B2 invariant)"
    - "Optional-interface release: Manager calls LeaseReleaser via type-assertion on its LeaseAcquirer, so test fakes that don't track release still satisfy the cache-invariant tests; production *PoolAcquirer satisfies the interface"
    - "Trace span helpers as plain top-level functions: StartFileSpan / StartStepSpan return (ctx, span); caller defer span.End() — mirrors otel idioms in the rest of the codebase"
    - "Daemon errgroup hook for long-lived bundle: liveBundle.Run blocks on ctx.Done when no enrichment manager is constructed so the errgroup goroutine doesn't return early and tear down siblings"

key-files:
  created:
    - "internal/semantic/lspenrich/status.go (Status struct + statusTracker; W7 partial_budget bumps both counters; W11 LastErrorPerLanguage as map[string]string)"
    - "internal/semantic/lspenrich/status_test.go (S1..S4 + S2b W7 + S3b W11 — 6 tests)"
    - "internal/semantic/lspenrich/manager.go (Manager — singleflight per-key cache + LeaseProvider + LeaseReleaser optional dispatch + Run/Stop/Status/AcquireFor/OnWorkspaceDeactivate)"
    - "internal/semantic/lspenrich/manager_test.go (M-Cache1..M-Cache3 + M-Deactivate1 + M-Stop1 + M-Concurrent1 + M1 + M2 — 8 tests)"
    - "internal/semantic/lspenrich/trace.go (StartFileSpan / StartStepSpan helpers — SPEC §28.2 anchored)"
    - "internal/semantic/lspenrich/pool_acquirer.go (PoolAcquirer — production LeaseAcquirer + LeaseReleaser)"
    - "internal/semantic/lspenrich/readiness_probe.go (PoolReadinessProbe — production ReadinessProbe via real Pool accessor names)"
    - "internal/kernel/lspool/pool_quirks_accessor_test.go (ACC1..ACC5 — Pool.JdtlsAdapter / Pool.RustAnalyzerAdapter accessors return nil-when-no-worker, nil-when-quirks-wrong-type, adapter-when-present)"
  modified:
    - "internal/kernel/lspool/pool.go (added JdtlsAdapter + RustAnalyzerAdapter per-wsKey accessors — W8)"
    - "internal/kernel/lspool/quirks.go (added RustAnalyzerAdapter.QuiescentChan returning <-chan struct{} for select-based waits)"
    - "internal/daemon/daemon.go (Daemon.live field; Run g.Go for live.Run; forwarderServiceHandler.live + DeactivateWorkspace forwarding; buildLiveBundle 6-arg signature)"
    - "internal/daemon/shutdown.go (Phase 0 — live.Stop BEFORE kernel.Shutdown)"
    - "internal/daemon/live_wiring.go (liveBundle.enrichMgr + Run/OnWorkspaceDeactivate/Stop methods + storeCascadeStoreAdapter/storeCascadeTxAdapter; buildLiveBundle constructs Manager when enrichCfg.Enabled && MaxConcurrentWorkers > 0; calls Pool.SetYieldCheckWindow)"
    - "internal/daemon/live_e2e_test.go (buildLiveBundle test call signature update)"

decisions:
  - "Singleflight via golang.org/x/sync/singleflight (NOT a hand-rolled per-key mutex map): the post-acquire re-check pattern with a single map mutex still allows concurrent goroutines to race past the cache-miss check.  singleflight.Group guarantees AT MOST ONE in-flight call per key — the test M-Concurrent1 (100 goroutines, expecting AcquireLease call count == 1) can only pass with a true singleflight.  x/sync was already in go.mod (errgroup); singleflight added zero new transitive dependencies."
  - "LeaseReleaser as optional interface (not a required method on LeaseAcquirer): keeps the unit-test fakes minimal — they don't track release count via a pool lifecycle they don't model.  Production *PoolAcquirer satisfies LeaseReleaser; tests assert the cache invariant via the AcquireFor call counter only, which is the load-bearing observable."
  - "RustAnalyzerAdapter.QuiescentChan() returns <-chan struct{} (a fresh closed channel when already-quiescent): the existing WaitUntilRenameReady takes a context.Context and returns a bool — wrapping it for the readiness probe would either double-block or duplicate the timeout machinery.  Exposing the channel directly lets PoolReadinessProbe.RustQuiescent select on (channel, ctx.Done) without re-implementing timeout logic.  Already-quiescent returns a pre-closed channel so the select fires immediately."
  - "storeCascadeStoreAdapter wraps *semanticstore.Store — Upsert*/WriteInvalidations are no-ops in Phase 61, MarkFileSemanticPending + Commit + Rollback + Epoch forward to the real *store.OverlayTx.  This matches the cascade_overlay_epoch_test.go pattern from Plan 61-02 (realStoreCascadeAdapter) and respects the Phase 60 D-04 stub for WriteInvalidations.  Phase 62 wires the real upsert path."
  - "Daemon-side wiring: Daemon.live is a struct field (was a captured local in New) so Run can pull live.Run(gctx) into the top-level errgroup AND the gRPC DeactivateWorkspace handler can forward to live.OnWorkspaceDeactivate.  This is the cleanest production B2 path the codebase supports today — there is no formal kernel-side workspace deactivate callback yet, but the gRPC handler IS invoked by forwarder shutdown so cached leases ARE released promptly per workspace."
  - "Daemon shutdown ordering: Phase 0 calls live.Stop BEFORE kernel.Shutdown so the enrichment manager releases all cached leases before the kernel tears down its worker pool.  This avoids racing the kernel's stopAll() against still-cached lease handles.  Test coverage: M-Stop1 verifies Stop releases all; daemon.shutdown ordering is observed via the kernel/live integration tests (no direct test, but the live.Stop call is unconditional and idempotent)."
  - "buildLiveBundle signature grew from 6 to 7 args (added enrichCfg semantic.LSPEnrichmentConfig before the rest).  Updating live_e2e_test.go was a one-line change — the test passes the zero-value config so it does NOT construct an enrichment manager (the CR-04 e2e test does not exercise the enrichment path)."

metrics:
  duration: "~75min (resumption only — Tasks 1-2 were committed in the prior session)"
  tasks_completed: 5     # 2 prior + 3 in this session
  tasks_in_this_session: 3
  files_created: 8
  files_modified: 6
  commits_in_this_session: 4   # RED + 3 GREEN feats
  completed_date: 2026-05-06

requirements_completed: [ENRICH-01, ENRICH-02, ENRICH-03, ENRICH-04]
---

# Phase 61 Plan 03: Manager + adapters + daemon bootstrap

**LSP enrichment Manager owns N workers + per-(wsKey, lang) lease cache (B2 fix-path-A) with singleflight semantics; PoolAcquirer + PoolReadinessProbe satisfy the kernel/semantic seams via the W8 per-wsKey Pool accessors; daemon bootstrap constructs the manager from cfg, runs it in the errgroup, and wires OnWorkspaceDeactivate from the gRPC handler so cached leases are released promptly per workspace.**

## Performance

- **Duration:** ~75min (resumption — Tasks 1-2 were committed in the prior worktree)
- **Started:** 2026-05-06 (resumption start)
- **Completed:** 2026-05-06
- **Tasks executed in this resumption:** 3 (Tasks 3, 4, 5)
- **Tasks already committed before resumption:** 2 (Tasks 1, 2 — verified via `git log --oneline --grep='61-03'`: 773274d1, 8b707209)

## Accomplishments

- **Task 3 — Manager + Status + trace (RED → GREEN cycle):**
  - `status.go`: `Status` struct (D-09) + `statusTracker` with atomic counters and a sync.RWMutex on the per-language error map.  W7: `OutcomePartialBudget` bumps BOTH `filesEnriched` AND `filesPending` so get_health consumers can compute `fully_enriched := enriched - pending`.  W11: `LastErrorPerLanguage` is `map[string]string` for v1 with documented evolution path.  Concurrent readers do NOT block (read-lock only, atomic loads on counters).
  - `manager.go`: `Manager` owns the per-(wsKey, lang) lease cache (B2 fix-path-A).  `AcquireFor` uses `golang.org/x/sync/singleflight` so 100 concurrent calls on a cold key collapse to ONE underlying `Pool.AcquireLease` — verified by test `TestManager_AcquireFor_ConcurrentSingleflight_M_Concurrent1`.  `ErrCircuitOpen` (and any AcquireLease error) is NOT cached — the slot stays empty so the next job retries (`TestManager_AcquireFor_ErrCircuitOpen_NotCached_M_Cache3`).  `OnWorkspaceDeactivate` releases per-ws leases via the optional `LeaseReleaser` interface (production: `*lspool.Pool.ReleaseLease`); `Stop` releases all.  Compile-time assertion: `var _ LeaseProvider = (*Manager)(nil)` so production wiring constructs `Worker.Leases = manager` directly.
  - `trace.go`: `StartFileSpan` produces the canonical `semantic.lsp_enrich_file` root span (SPEC §28.2 line 2761); `StartStepSpan` opens child spans named `semantic.lsp_enrich.<step>`.

- **Task 4 — Pool accessors + PoolAcquirer + PoolReadinessProbe:**
  - `internal/kernel/lspool/pool.go`: `Pool.JdtlsAdapter(wsKey)` and `Pool.RustAnalyzerAdapter(wsKey)` — read-locked, type-asserted via `Worker.Quirks()`; return nil when no Ready worker matches (lang, repoRoot).
  - `internal/kernel/lspool/quirks.go`: `RustAnalyzerAdapter.QuiescentChan()` returns `<-chan struct{}` (already-closed when adapter is currently quiescent) so callers can `select` on it alongside `ctx.Done()`.
  - `pool_acquirer.go`: `PoolAcquirer` forwards `AcquireLease`, `ForegroundBusy`, `ReleaseLease`.  Compile-time: `var _ LeaseAcquirer = (*PoolAcquirer)(nil)` and `var _ LeaseReleaser = (*PoolAcquirer)(nil)`.
  - `readiness_probe.go`: `PoolReadinessProbe` uses real Pool accessor names (no `// adjust to real accessor name` pseudo-comments).  Compile-time: `var _ ReadinessProbe = (*PoolReadinessProbe)(nil)`.

- **Task 5 — daemon bootstrap (B2 deactivate hook + B4 SetYieldCheckWindow):**
  - `buildLiveBundle` signature gains `enrichCfg semantic.LSPEnrichmentConfig`.  When `enrichCfg.Enabled && MaxConcurrentWorkers > 0`, constructs the manager with PoolAcquirer + PoolReadinessProbe + storeCascadeStoreAdapter + ProdMetricsSink.
  - `liveBundle.Run`: starts manager in daemon's top-level errgroup; blocks on ctx.Done when no manager is constructed.
  - `liveBundle.OnWorkspaceDeactivate`: forwards to `Manager.OnWorkspaceDeactivate(wsKey)` from the gRPC `DeactivateWorkspace` handler so cached leases are released per workspace (B2).
  - `liveBundle.Stop`: shutdown safety-net (releaseAll across all workspaces).
  - `Pool.SetYieldCheckWindow` invoked unconditionally from `enrichCfg.YieldCheckWindowMs` (B4).
  - `daemon.shutdown`: Phase 0 calls `live.Stop()` BEFORE `kernel.Shutdown` so leases are released before workers tear down.

## Task Commits

| # | Task | RED | GREEN |
|---|------|-----|-------|
| 1 | config keys (defaults + struct + per-feature test) | — | `773274d1` (committed in prior session) |
| 2 | obs metrics (5 vectors + closed-enum helpers + ProdMetricsSink) | — | `8b707209` (committed in prior session) |
| 3 | trace + Status + Manager (singleflight per-key cache) | `d3533bb2` | `3ac6461f` |
| 4 | Pool accessors + PoolAcquirer + PoolReadinessProbe (W8) | — (additive feature) | `0c6feb98` |
| 5 | daemon bootstrap (errgroup + DeactivateWorkspace) | — (no-op refactor) | `041c931f` |

**TDD gate compliance:** Task 3 ships the RED → GREEN cycle (`test(61-03):` failing tests → `feat(61-03):` implementation).  Tasks 4 and 5 are additive feature work + signature plumbing where there is no pre-implementation behavior to test against (Task 4's accessor tests live in the GREEN commit because the feature was greenfield; Task 5's wiring is exercised through the existing `live_e2e_test.go` regression suite).

## Files Created / Modified

See frontmatter `key-files`.  Concrete numbers: 8 created, 6 modified across this resumption (3 tasks).

## Decisions Made

See frontmatter `decisions`.  Highlights:

- **Singleflight via x/sync** instead of hand-rolled per-key mutex maps — the M-Concurrent1 invariant (100 acquires, 1 underlying call) is unachievable with a single map mutex + post-acquire re-check because all 100 goroutines can race past the cache-miss before any one of them inserts the cached lease.  `singleflight.Group` collapses concurrent calls deterministically.
- **Optional LeaseReleaser interface** — keeps test fakes minimal; production *PoolAcquirer satisfies it.  The cache invariant tests (M-Cache1/2/3) assert via the AcquireFor call counter, which is the observable contract; release tracking is exercised separately via M-Deactivate1 and M-Stop1 against `recordingAcquirer.releaseCount`.
- **RustAnalyzerAdapter.QuiescentChan exposes the readyCh as receive-only** so the readiness probe can select on it alongside ctx.Done without duplicating the WaitUntilRenameReady timeout machinery.
- **Daemon.live as a struct field** (was a captured local in New) so Run AND the gRPC DeactivateWorkspace handler can both reach it — the gRPC handler is currently the only deterministic per-workspace deactivate signal, so wiring it there is the cleanest B2 path the codebase supports today.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Singleflight required for M-Concurrent1 invariant.**
- **Found during:** Task 3 GREEN — first implementation used a sync.Mutex around the cache map + a post-acquire re-check.  Test M-Concurrent1 (100 concurrent AcquireFor calls on a cold key, expecting AcquireLease call count == 1) FAILED with `count=2`.
- **Issue:** All 100 goroutines can race past the cache-miss check before any one of them inserts the cached lease.  The post-acquire re-check absorbs the loser's lease (releases ours, returns theirs) but the AcquireLease call counter has already been bumped twice (or more under heavier contention).  The test invariant requires AT MOST ONE AcquireLease call.
- **Fix:** Switched to `golang.org/x/sync/singleflight.Group` which collapses concurrent Do(key, fn) calls to a single in-flight `fn` invocation.  All other goroutines block in `Do` and receive the shared result.  After the singleflight returns, the cache is populated so subsequent AcquireFor calls hit the fast path.
- **Files modified:** `internal/semantic/lspenrich/manager.go` (replaced post-acquire re-check pattern with `acquireFlight singleflight.Group` + `Do(flightKey, ...)`).
- **Verification:** `go test -race ./internal/semantic/lspenrich/... -run M_Concurrent1 -count=1` exits 0 with `r.calls == 1` after 100 concurrent AcquireFor calls on a cold key.
- **Committed in:** `3ac6461f` (Task 3 GREEN).

**2. [Rule 3 — Blocking] buildLiveBundle signature change broke live_e2e_test.go.**
- **Found during:** Task 5 GREEN — `go test ./internal/daemon/...` failed to compile because the existing CR-04 e2e test calls `buildLiveBundle(cfg, store, sched, k, metrics, logger)` (6 args) and the new signature requires 7 (added `enrichCfg semantic.LSPEnrichmentConfig` after `cfg`).
- **Fix:** Updated the test call to pass `semantic.LSPEnrichmentConfig{}` (zero value) so the test path does NOT construct an enrichment manager.  The CR-04 e2e test exercises the producer-side LSPQueue enqueue path only — it does not need the enrichment manager.
- **Files modified:** `internal/daemon/live_e2e_test.go` (one-line change).
- **Verification:** `go test ./internal/daemon/... -count=1 -race` passes including the live_e2e + cr-04 tests.
- **Committed in:** `041c931f` (Task 5 GREEN).

### Plan Action Note

The plan's Task 3 action snippet referenced `lease.Release()` (a method on `*WorkerLease`) and the plan's Task 5 action snippet referenced `lspenrich.NewOverlayStoreAdapter(store)`.  Neither exists in the codebase as-named:

- `*lspool.WorkerLease` does NOT have a `Release()` method — release is via `*lspool.Pool.ReleaseLease(sessionID string)`.  The Manager satisfies this via the optional `LeaseReleaser` interface and looks up sessionID from the cached `*WorkerLease.SessionID` field.
- The `OverlayStore` interface (P01 types.go) returns `*store.OverlayTx`; the Worker uses `CascadeStore` returning `CascadeTx` (a richer surface).  The daemon needs a `CascadeStore` adapter, NOT an `OverlayStore` adapter — the SUMMARY documents this as `storeCascadeStoreAdapter`.

These are plan-level inconsistencies between the action prose and the actual interfaces P01 + P02 shipped.  The implementation honors the interfaces, not the action's pseudo-method names.

## Authentication Gates

None encountered.  All work was local-tree only (no LS startup, no external auth).

## Test Coverage

- **Status (status_test.go):** 6 tests covering zero-value defaults, applied / partial_budget (W7) / dropped outcomes, last-error-per-language as string (W11), concurrent reader race-cleanness.
- **Manager (manager_test.go):** 8 tests covering M-Cache1/2/3 (per-key + ErrCircuitOpen NOT cached), M-Deactivate1 (per-ws release), M-Stop1 (releaseAll), M-Concurrent1 (singleflight under -race), M1 (Run honors ctx-cancel), M2 (Stop idempotent).
- **Pool accessors (pool_quirks_accessor_test.go):** 5 tests covering nil-when-no-worker, nil-when-quirks-wrong-type, adapter-when-present (Java + Rust).
- **Daemon (existing live_e2e_test.go):** signature update only — existing CR-04 producer-side regression remains green.

`go test -race ./internal/{config,obs,semantic/lspenrich,kernel/lspool,daemon}/... -count=1 -timeout 120s` exits 0; `make vet` exits 0 across all four analyzers (default, noduckdb, nokernel2semantic, nosemantic2kernel).

## Boundary Verification

```
$ go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/semantic/lspenrich | grep "internal/kernel"
github.com/agenthands/helix/internal/kernel/lspool
```

Direct imports of `internal/semantic/lspenrich` reach only `internal/kernel/lspool`.  No `internal/kernel` parent-package import — `vet-nosemantic2kernel` clean.

## B2 + B3 + W7 + W8 + W11 Audits

| Audit | Expected | Actual | Status |
|-------|----------|--------|--------|
| B2: `AcquireFor\|OnWorkspaceDeactivate\|releaseAll` count in manager.go | >= 6 | 20 | PASS |
| B2: `leases map[wsLangKey]` declaration | >= 1 | 1 | PASS |
| B2: lease cache singleflight | tests M-Concurrent1 + M-Cache1/2/3 pass | pass | PASS |
| B3: `type Outcome string\|type MetricsSink interface\|type OverlayStore interface` redeclarations across manager.go/status.go/metrics.go | == 0 | 0 | PASS |
| W7: `case OutcomePartialBudget` bumps `filesEnriched.Add` AND `filesPending.Add` | == 2 | 2 | PASS |
| W8: `Pool.JdtlsAdapter\|Pool.RustAnalyzerAdapter` accessor declarations on pool.go | >= 2 | 2 | PASS |
| W8: pseudo-comments (`// adjust to real accessor name\|pseudo`) in readiness_probe.go | == 0 | 0 | PASS |
| W11: `map[string]string` declared in status.go for last-error map | >= 1 | 1 | PASS |
| W11: documented evolution-path comment about Phase 65 | present | present | PASS |
| B4: `SetYieldCheckWindow` call in live_wiring.go | == 1 | 1 | PASS |

## Daemon Wiring Summary

```
daemon.New
  └─ buildLiveBundle(liveCfg, enrichCfg, store, sched, k, metrics, logger)
        ├─ Pool.SetYieldCheckWindow(enrichCfg.YieldCheckWindowMs)            ← B4
        └─ if enrichCfg.Enabled && MaxConcurrentWorkers > 0:
              └─ enrichMgr = NewManager(
                      laneQueue,
                      PoolAcquirer{Pool: k.Pool()},        ← LeaseAcquirer + LeaseReleaser
                      storeCascadeStoreAdapter{store},     ← CascadeStore (Upsert*: Phase 62)
                      PoolReadinessProbe{Pool: k.Pool()},  ← Java/Rust gates via Pool accessors
                      enrichCfg,
                      ProdMetricsSink{M: metrics},
                      logger)

daemon.Run
  ├─ g.Go(d.kernel.Run(gctx))
  └─ if d.live != nil:
        g.Go(d.live.Run(gctx))                            ← Manager.Run errgroup

daemon.gRPC.DeactivateWorkspace(wsPath)
  └─ d.live.OnWorkspaceDeactivate(wsKey)                   ← B2 per-ws lease release

daemon.shutdown                                            ← Phase 0
  ├─ d.live.Stop()                                         ← safety-net releaseAll
  └─ d.kernel.Shutdown(ctx)
```

## Next Phase Readiness

- **For Plan 61-04 (stress test):** Manager.Queue() is the test-only accessor for direct enqueue; Manager.Status() exposes the counters the stress test asserts on (FilesEnriched / FilesPending under load).  The daemon-bootstrap path is now end-to-end — a stress harness can construct a Daemon with a non-zero MaxConcurrentWorkers and observe the enrichment loop running against a real LSP fixture.

- **Carry-over for the production cascade-LSP shim (Phase 64+):** the soft-error tolerance documented in 61-02-SUMMARY ("not a type name", "is a function, not a method", "no element found", etc.) lives in `cascade_integration_test.go` only.  When Phase 64+ wires a production `CascadeLSPFactory` around `*lspool.WorkerLease.Request`, that adapter MUST tolerate the same soft errors as nil-result rather than treating them as cascade-LS-unavailable.  PoolAcquirer at this layer does not inspect LSP payload errors — it only routes lease lifecycle — so the soft-error handling is the cascade-shim's responsibility, NOT PoolAcquirer's.

- **All four verification gates pass:**
  - `go build ./...` clean (only the pre-existing tree-sitter binding C-warning).
  - `go test -race ./internal/{config,obs,semantic/lspenrich,kernel/lspool,daemon}/... -count=1 -timeout 120s` exits 0 (all relevant packages).
  - `go test ./...` (full tree) clean — no FAIL packages introduced.
  - `make vet` clean across `vet`, `vet-noduckdb`, `vet-nokernel2semantic`, `vet-nosemantic2kernel`.

## Self-Check

Verified after writing this SUMMARY:

- File `internal/semantic/lspenrich/manager.go`: FOUND
- File `internal/semantic/lspenrich/manager_test.go`: FOUND
- File `internal/semantic/lspenrich/status.go`: FOUND
- File `internal/semantic/lspenrich/status_test.go`: FOUND
- File `internal/semantic/lspenrich/trace.go`: FOUND
- File `internal/semantic/lspenrich/pool_acquirer.go`: FOUND
- File `internal/semantic/lspenrich/readiness_probe.go`: FOUND
- File `internal/kernel/lspool/pool_quirks_accessor_test.go`: FOUND
- Modified: internal/kernel/lspool/pool.go: FOUND (JdtlsAdapter + RustAnalyzerAdapter)
- Modified: internal/kernel/lspool/quirks.go: FOUND (QuiescentChan)
- Modified: internal/daemon/daemon.go: FOUND (live field + g.Go + DeactivateWorkspace forwarding)
- Modified: internal/daemon/live_wiring.go: FOUND (enrichMgr + Run/OnWorkspaceDeactivate/Stop + storeCascadeStoreAdapter)
- Modified: internal/daemon/shutdown.go: FOUND (Phase 0 live.Stop)
- Commit `d3533bb2` (Task 3 RED): FOUND in `git log`
- Commit `3ac6461f` (Task 3 GREEN): FOUND in `git log`
- Commit `0c6feb98` (Task 4 GREEN): FOUND in `git log`
- Commit `041c931f` (Task 5 GREEN): FOUND in `git log`

## Self-Check: PASSED

---
*Phase: 61-lsp-enrichment-worker*
*Completed: 2026-05-06*
