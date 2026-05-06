---
phase: 61-lsp-enrichment-worker
verified: 2026-05-06T10:35:00Z
status: human_needed
score: 4/5 must-haves verified (1 with caveat)
overrides_applied: 0
gaps:
  - truth: "Production daemon dispatches enrichment jobs end-to-end (Manager.Run → Worker.processOne → Cascade.Run)"
    status: partial
    reason: "Manager.Run constructs Worker WITHOUT NewCascadeLSP factory (manager.go:198-207). Worker.processOne (worker.go:225-229) checks `if w.NewCascadeLSP == nil` and emits OutcomeDropped + Error log unconditionally. Result: every job dispatched through the production wiring (live_wiring.go) is dropped before the cascade engine runs. Acceptance integration tests (TestACC4/6/10) compensate by injecting NewCascadeLSP at Worker layer directly. The cascade engine itself is correct and exercised end-to-end against real gopls + jdtls in TestCascade_GoIntegration / TestCascade_JavaIntegration. Executor SUMMARY explicitly defers NewCascadeLSP production wiring to Phase 64+."
    artifacts:
      - path: "internal/semantic/lspenrich/manager.go"
        issue: "NewManager + Manager.Run construct Worker without NewCascadeLSP — production wiring gap"
      - path: "internal/daemon/live_wiring.go"
        issue: "buildLiveBundle does not supply a CascadeLSPFactory to Manager"
    missing:
      - "Production wiring of Worker.NewCascadeLSP — needs adapter from CascadeLSP-over-WorkerLease to be constructed in live_wiring.go and threaded through Manager → Worker"
human_verification:
  - test: "Verify the deferred NewCascadeLSP wiring gap is acceptable scope for Phase 61"
    expected: "Phase 64 (`refresh_semantic_graph` MCP tool) explicitly accepts ownership of the production CascadeLSP wiring; OR Phase 61 ships a follow-up plan that wires NewCascadeLSP without depending on the Phase 64 MCP tool surface"
    why_human: "The executor explicitly flagged this in 61-04-SUMMARY decisions section as a deliberate deferral. ROADMAP Phase 61 Success Criteria 1-4 are all satisfied at the unit/integration test level — production end-to-end dispatch is not a SC. Whether deferring the wiring is acceptable for milestone v1.10 phase boundary requires architect judgement."
  - test: "Confirm cascade_test.go cascadeNow var-init ordering issue is acceptable"
    expected: "Either the test fixture is patched to recompute cascadeNow per-test (matches its own doc-comment claim), OR the order-dependent failure is documented as a known-flaky integration-tag-only issue"
    why_human: "Under `go test -tags integration`, when TestCascade_JavaIntegration (9.3s) runs before TestCascade_C1/C3/C4/C5/C6/C8 in sort order, the package-level `var cascadeNow = time.Now()` falls outside the 5s per-file budget window and 6 cascade tests fail with 'partial_budget' instead of their expected outcomes. Tests pass in isolation; default `go test -short` is unaffected. This is a test-fixture ordering bug, not a production code bug."
deferred:
  - truth: "Phase 61 production worker dispatches jobs end-to-end (cascade engine called from live daemon)"
    addressed_in: "Phase 64"
    evidence: "ROADMAP Phase 64 goal: 'New MCP Tools (P0 set of 4) — index_semantic_graph, refresh_semantic_graph, get_semantic_graph_status, get_semantic_context'. The CascadeLSP factory shim ships with Phase 64's refresh_semantic_graph wiring per executor decision in 61-04-SUMMARY: 'production wiring (live_wiring.go) does NOT yet supply Worker.NewCascadeLSP — that adapter ships in Phase 64+'. Phase 65 (`get_health` strangler-fig) wires the Status() accessor. Phase 61 ROADMAP SCs 1-4 are achievable at the test surface without production cascade dispatch."
---

# Phase 61: LSP Enrichment Worker Verification Report

**Phase Goal:** Async LSP enrichment promotes tree-sitter facts to LSP-confirmed evidence without burying foreground tool calls or regressing v1.9 readiness invariants.

**Verified:** 2026-05-06T10:35:00Z

**Status:** human_needed

**Re-verification:** No — initial verification.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + ENRICH-01..05)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | ENRICH-01: semantic does not import internal/kernel directly; LeaseAcquirer interface is the seam | VERIFIED | `internal/semantic/lspenrich/acquirer.go:6-7` imports only `internal/kernel/lspool` + `internal/workspace`. `grep -rn '"github.com/agenthands/helix/internal/kernel' internal/semantic/lspenrich/*.go \| grep -v lspool \| grep -v _test.go` returns no output. The `nosemantic2kernel` vet analyzer (cmd/vet-nosemantic2kernel + internal/lint/nosemantic2kernel) is wired into `make vet` (Makefile:22, 28, 40-41) and runs clean. LeaseAcquirer interface (acquirer.go:37-40) has exactly two methods: AcquireLease + ForegroundBusy. Transitive `kernel/jsonrpc` reachable through `kernel/lspool` is allowed by analyzer (DIRECT-import enforcement only) and matches plan-stated boundary discipline. |
| 2 | ENRICH-02: priority queue with foreground preemption + concurrency cap default 1 | VERIFIED | `internal/semantic/lspenrich/queue.go` ships LaneQueue with strict-priority Drain (queue_test.go L1-L6 pass). `Pool.ForegroundBusy` (pool.go:322) + `Pool.SetYieldCheckWindow` (pool.go:343) implement the foreground-busy stamp gate with `lsp-enrichment:` session prefix exclusion. Cascade.Run calls foregroundBusy() between every step (cascade.go:251-262 — `checkBoundary` closure invoked before each LSP call). Default `max_concurrent_workers=1` confirmed in `internal/config/defaults.go:112`. Manager.Run honors cap via `w.RunN(gctx, n)` where `n=cfg.MaxConcurrentWorkers` (manager.go:208-218). Acceptance integration tests TestACC4_Cap1 + TestACC4_Cap4 + TestACC6_VoluntaryYield all pass under `go test -tags integration`. |
| 3 | ENRICH-03: Java + Rust readiness gates honored, never bypassed | VERIFIED | `internal/semantic/lspenrich/readiness.go:241-251` implements WaitForLanguageReady dispatching to JavaReady/RustQuiescent. PoolReadinessProbe (`readiness_probe.go:34, 47-58`) calls real `Pool.JdtlsAdapter(wsKey).WaitUntilJavaReady(ctx)` and `Pool.RustAnalyzerAdapter(wsKey).QuiescentChan()`. Worker.processOne (worker.go) calls `WaitForLanguageReady` BEFORE `LeaseProvider.AcquireFor`. No "best-effort bypass" `if err != nil { return nil }` patterns found. Acceptance tests TestACC10_JavaReadiness + TestACC10_JavaReadinessTimeout PASS under `-tags integration`. Real-LS test TestCascade_JavaIntegration PASSES (jdtls cold-start observed; cascade waits then proceeds; CALLS edges produced). |
| 4 | ENRICH-04: per-file budget enforced; partial:true / partial_reason="budget exhausted" remains queryable | VERIFIED | `internal/semantic/lspenrich/budget.go` ships Budget value type with TimeoutPerFile + TimeoutTotal parsing, ConsumeSymbol/ConsumeReferences guards, HasRemainingTime check. Cascade.Run boundary check (cascade.go:251-271) returns OutcomePartialBudget and stamps `MarkFileSemanticPending(path, "budget exhausted")` on expiry (cascade.go:264, 329, 408). `OverlayTx.MarkFileSemanticPending` validates against closed enum (preempted/bulk_update_pending/lsp_unavailable/budget exhausted). Files remain queryable — partial_reason is metadata, not a deletion. Test C2 (BudgetExhaustion) and C9 (ConsumeReferencesPerSymbolGranularity) pass; B1-B6 budget unit tests pass. |
| 5 | ENRICH-05: 100-file edit burst / git-checkout storm does not bury foreground tool calls; interactive p95 < 5s | VERIFIED (with caveat) | `internal/semantic/lspenrich/stress_test.go` exists with `//go:build stress` build tag (line 1). `TestStress_ENRICH05_Go` + `TestStress_ENRICH05_Java` defined (stress_test.go:403, 422). Per 61-04-SUMMARY local run results: 149 foreground samples / p95=877µs / mean=583.834µs / 100 enrichment txs committed — foreground p95 is ~5700x under the 5s budget. Stress test gated correctly: `go test -short ./internal/semantic/lspenrich/...` does not pull in stress test (verified via run output). Caveat: stress test exercises Worker directly (with NewCascadeLSP injected), not through production daemon dispatch — same caveat as ACC6 below. The `git checkout storm` half is implicitly covered by `live/handler/handler.go` ChangeBulkUpdate suppression path (P01-T3) which is unit-tested but the stress test focuses on per-file edit-burst — acceptable per executor's plan choice. |

**Score:** 5/5 ROADMAP Success Criteria + ENRICH-01..05 verified at the unit/integration-test surface.

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|--------------|----------|
| 1 | Production daemon end-to-end cascade dispatch (Worker.NewCascadeLSP factory wired in live_wiring.go) | Phase 64 | Executor 61-04-SUMMARY decisions section: "production wiring (live_wiring.go) does NOT yet supply Worker.NewCascadeLSP — that adapter ships in Phase 64+. Without NewCascadeLSP every Manager.Run dispatch lands on OutcomeDropped before the cascade is constructed (worker.go step 4)." ROADMAP Phase 64 goal lists `refresh_semantic_graph` and `get_semantic_graph_status` MCP tools; the CascadeLSP-over-WorkerLease adapter is the load-bearing dependency for those tools. |
| 2 | Phase 65 `get_health` strangler-fig wires `Manager.Status()` accessor | Phase 65 | ROADMAP Phase 65: "`get_repo_map`, `get_context`, `analyze_blast_radius`, `get_health` consult semantic when available". 61-CONTEXT.md "Out of scope (deferred)" line 132-133: "`get_health` watcher/enrichment status integration — Phase 65 strangler-fig. Phase 61 ships a `Status()` accessor on the worker manager (queue depth per lane, last error per language, files enriched/dropped/preempted counters); Phase 65 wires it into `get_health`." Status() accessor exists at `manager.go:Status()`. |

### Required Artifacts (must_haves from PLAN frontmatter)

| Artifact | Status | Details |
|----------|--------|---------|
| internal/semantic/lspenrich/acquirer.go | VERIFIED | LeaseAcquirer interface, exactly 2 methods, imports lspool + workspace only |
| internal/semantic/lspenrich/types.go | VERIFIED | Outcome typed string + 5 closed-enum constants + MetricsSink + OverlayStore interfaces (B3 single source of truth) |
| internal/semantic/lspenrich/queue.go | VERIFIED | LaneQueue with strict-priority Drain, lane enum (high/background) |
| internal/semantic/lspenrich/budget.go | VERIFIED | Budget value type, NewBudget(now, workerStart, cfg), HasRemainingTime, ConsumeSymbol/ConsumeReferences |
| internal/semantic/lspenrich/readiness.go | VERIFIED | WaitForLanguageReady + ReadinessProbe interface |
| internal/semantic/lspenrich/cascade.go | VERIFIED | Cascade.Run with §14.4 6-step orchestration, checkBoundary closure, partial_reason stamping |
| internal/semantic/lspenrich/worker.go | VERIFIED | Worker.RunN drain loop, LeaseProvider seam (B2: no per-job Release), explicit Capabilities (W3) |
| internal/semantic/lspenrich/manager.go | VERIFIED with caveat | NewManager + Run + Stop + Status + AcquireFor + OnWorkspaceDeactivate. Lease cache via singleflight (line 74). ErrCircuitOpen NOT cached. ⚠️ Caveat: constructs Worker without NewCascadeLSP factory (line 198-207) — production dispatch is a no-op. |
| internal/semantic/lspenrich/status.go | VERIFIED | Status struct + statusTracker; W7: OutcomePartialBudget bumps both filesEnriched + filesPending; W11: LastErrorPerLanguage map[string]string |
| internal/semantic/lspenrich/metrics.go | VERIFIED | ProdMetricsSink wraps obs.Metrics; B3 compile-time assertion `var _ MetricsSink = ProdMetricsSink{}` |
| internal/semantic/lspenrich/trace.go | VERIFIED | semantic.lsp_enrich_file root span + per-step children |
| internal/semantic/lspenrich/pool_acquirer.go | VERIFIED | PoolAcquirer adapter with `var _ LeaseAcquirer = (*PoolAcquirer)(nil)` |
| internal/semantic/lspenrich/readiness_probe.go | VERIFIED | PoolReadinessProbe wraps real `Pool.JdtlsAdapter(wsKey)` + `Pool.RustAnalyzerAdapter(wsKey).QuiescentChan()` accessors |
| internal/semantic/lspenrich/stress_test.go | VERIFIED | `//go:build stress`, TestStress_ENRICH05_Go/Java, p95 assertion |
| internal/semantic/lspenrich/integration_acceptance_test.go | VERIFIED | `//go:build integration`, ACC4/Cap1/Cap4 + ACC6_VoluntaryYield + ACC10/JavaReadiness/Timeout |
| internal/semantic/lspenrich/cascade_integration_test.go | VERIFIED | `//go:build integration`, real gopls + jdtls; TestCascade_GoIntegration / JavaIntegration both PASS |
| internal/semantic/lspenrich/cascade_overlay_epoch_test.go | VERIFIED (test fixture caveat) | Real-store epoch advancement test exists; ⚠️ Subject to cascadeNow ordering issue (see human verification #2) |
| internal/lint/nosemantic2kernel/analyzer.go | VERIFIED | Vet analyzer; testdata covers internal/kernel forbidden, internal/kernel/lspool allowed, internal/workspace allowed |
| cmd/vet-nosemantic2kernel/main.go | VERIFIED | singlechecker.Main wiring |
| internal/kernel/lspool/pool.go | VERIFIED | ForegroundBusy(wsKey) + SetYieldCheckWindow(d) added; lsp-enrichment: prefix filter in AcquireLease (line 154); JdtlsAdapter(wsKey) + RustAnalyzerAdapter(wsKey) per-workspace accessors (W8) |
| internal/semantic/store/overlay.go | VERIFIED | MarkFileSemanticPending with closed-enum partialReasonClosedEnum validator (4 values: preempted, bulk_update_pending, lsp_unavailable, budget exhausted) |
| internal/semantic/live/handler/handler.go | VERIFIED | LSPRevalidationEnqueuer renamed in-place to LSPLaneEnqueuer (B5); selectLane + ChangeBulkUpdate suppression + markBulkPending |
| internal/daemon/live_wiring.go | VERIFIED with caveat | Constructs Manager when cfg.LSPEnrichment.Enabled && MaxConcurrentWorkers > 0; Stop() safety net; OnWorkspaceDeactivate forwarded from gRPC handler. ⚠️ Does NOT supply NewCascadeLSP factory (deferred to Phase 64+). |
| internal/config/defaults.go | VERIFIED | max_concurrent_workers=1, yield_check_window_ms=200 (lines 112-113) |
| internal/semantic/config.go | VERIFIED | MaxConcurrentWorkers + YieldCheckWindowMs koanf-tagged fields (lines 183, 187) |
| internal/obs/metrics.go | VERIFIED | 5 bounded-label metrics with closed-enum drop-on-unknown helpers |
| .planning/REQUIREMENTS.md | VERIFIED | 5 ENRICH-* boxes flipped to [x] (verified via grep) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| internal/semantic/lspenrich → internal/kernel | (forbidden, except lspool) | nosemantic2kernel analyzer | VERIFIED | Direct grep returns 0; analyzer clean via `make vet` |
| internal/semantic/live/handler → lspenrich.Lane | EnqueueLane | LSPLaneEnqueuer interface | VERIFIED | Single source of truth (B5); LSPRevalidationEnqueuer fully renamed |
| internal/kernel/lspool/pool.go → "lsp-enrichment:" prefix | session-id stamp filter | strings.HasPrefix in AcquireLease | VERIFIED | pool.go:154 — only non-`lsp-enrichment:` sessions stamp lastForegroundLease |
| Cascade.Run → tx.Upsert/Commit/MarkFileSemanticPending | overlay tx commit | per-file 1 tx | VERIFIED | cascade.go calls UpsertSymbols/UpsertReferences/UpsertEdges/UpsertDiagnostics + Commit per file; partial outcomes stamp partial_reason |
| readiness_probe.go → JdtlsAdapter.WaitUntilJavaReady | per-(wsKey, lang) accessor | Pool.JdtlsAdapter(wsKey) | VERIFIED | Real accessor wired (no pseudo-comments); test PR1/PR2/ACC1 pass |
| Manager.AcquireFor → singleflight.Group | concurrent acquire collapse | golang.org/x/sync/singleflight | VERIFIED | manager.go:74; TestManager_AcquireFor_ConcurrentSingleflight_M_Concurrent1 asserts 100 concurrent → 1 underlying AcquireLease call (manager_test.go:283-321) |
| daemon gRPC DeactivateWorkspace → live.OnWorkspaceDeactivate | Manager.OnWorkspaceDeactivate | per-wsKey lease release | VERIFIED | daemon.go:917 → live_wiring.go:90-95 → manager.go:173 (releases all (wsKey,*) cached leases) |
| daemon shutdown → live.Stop() | Manager.Stop → releaseAll | safety-net fallback | PARTIAL | live_wiring.go:101-106 ships `liveBundle.Stop()` method but it is NOT explicitly invoked from daemon.go shutdown sequence. However: (a) live.Run is in errgroup, exits on ctx cancel; (b) Manager.Run exits the errgroup and calls releaseAll() (manager.go:221) on its way out. So leases ARE released on daemon shutdown via the errgroup-cancel path; the explicit Stop() method is currently a defense-in-depth surface that the daemon does not call. Acceptable. |
| Manager.Run → Worker (with NewCascadeLSP) | Worker constructed in production | factory injection seam | NOT_WIRED (deferred) | manager.go:198-207 constructs Worker without setting NewCascadeLSP. worker.go:225-229 emits OutcomeDropped + Error log when factory is nil. Production dispatch is a no-op until Phase 64+ wires the factory. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|----|
| Cascade.Run upserted symbols/edges | tx.UpsertSymbols/Edges output | gopls/jdtls real LSP responses (TestCascade_GoIntegration) | YES — 3 symbols, 8 edges with kinds [TYPE_OF CALLS TYPE_OF CALLS TYPE_OF CALLS CALLS CALLS] for Go fixture | FLOWING |
| Cascade.Run on jdtls real fixture | tx.UpsertEdges (CALLS, EXTENDS) | jdtls real LSP responses (TestCascade_JavaIntegration) | YES — 3 symbols, 8 edges incl EXTENDS for Java fixture | FLOWING |
| Worker.processOne → Cascade.Run via production daemon | cascade outcome metric | Manager.Run constructs Worker w/o NewCascadeLSP | NO — every job lands on OutcomeDropped + Error log before cascade runs | DISCONNECTED (deferred to Phase 64+) |
| Manager.Status() | LaneDepths/FilesEnriched/etc | statusTracker counters incremented by trackedMetrics decorator on every LSPEnrichmentTotal/Errors call | YES — counters increment in unit/integration tests | FLOWING |
| ForegroundBusy(wsKey) | lastForegroundLease[wsKey] timestamp | pool.go:154 stamps in AcquireLease for non-enrichment sessions | YES — verified by F1-F5 + SY1-SY3 in pool_foreground_busy_test.go | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Default short tests pass | `go test -short -timeout 120s ./internal/semantic/lspenrich/...` | `ok ... (cached)` | PASS |
| Project-wide vet (tracked packages) | `go vet $(go list ./... \| grep -v tmp/)` | no output (clean) | PASS |
| nosemantic2kernel analyzer clean | `go vet -vettool=$(go env GOPATH)/bin/vet-nosemantic2kernel ./internal/...` | no output (only swift cgo macro warning, unrelated) | PASS |
| nokernel2semantic analyzer clean | `go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./internal/kernel/...` | no output (only swift cgo macro warning) | PASS |
| Direct kernel-import boundary | `grep -rn '"github.com/agenthands/helix/internal/kernel' internal/semantic/lspenrich/*.go \| grep -v lspool \| grep -v _test.go` | no output | PASS |
| Transitive kernel-import audit | `go list -deps ./internal/semantic/lspenrich/... \| grep -v 'kernel/lspool' \| grep 'kernel'` | `kernel/jsonrpc` (1 leak through lspool — analyzer enforces DIRECT only, transitive allowed per plan) | PASS (acceptable per plan) |
| ENRICH-01..05 REQUIREMENTS check-off | `grep -c '^- \[x\] \*\*ENRICH-' .planning/REQUIREMENTS.md` | 5 | PASS |
| Build clean | `go build ./...` | (orchestrator confirmed clean) | PASS |
| Integration tests (single-test) | `go test -tags integration -run TestCascade_C1 -count=1 ./internal/semantic/lspenrich/...` | PASS | PASS |
| Integration tests (full suite) | `go test -tags integration -timeout 120s ./internal/semantic/lspenrich/...` | 6 cascade tests FAIL when ordered after TestCascade_JavaIntegration (9.3s) due to stale `cascadeNow` package var | FAIL (test fixture bug — not production code) |
| ACC integration tests | `go test -tags integration -run TestACC ./internal/semantic/lspenrich/...` | All 5 PASS (per executor SUMMARY: 2.67s total) | PASS |
| Stress test (local) | `go test -tags stress -run TestStress_ENRICH05_Go ./internal/semantic/lspenrich/...` | Per executor SUMMARY: 32.06s, 149 foreground samples, p95=877µs (under 5s budget), 100 enrichment txs committed | PASS |
| Stress test gating | `go test -run TestStress -short ./internal/semantic/lspenrich/...` | "no tests to run" (correctly excluded by `//go:build stress`) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ENRICH-01 | 61-01, 61-04 | LeaseAcquirer interface; semantic does not import internal/kernel | SATISFIED | nosemantic2kernel analyzer wired into make vet, runs clean. acquirer.go imports only lspool + workspace. |
| ENRICH-02 | 61-01, 61-02, 61-04 | Priority queue + foreground preempt + cap=1 default | SATISFIED | LaneQueue strict-priority Drain (queue.go); cascade ForegroundBusy boundary; default cfg max_concurrent_workers=1; ACC4/Cap1/Cap4 + ACC6 PASS |
| ENRICH-03 | 61-02, 61-04 | Java + Rust readiness gates honored | SATISFIED | readiness.go + readiness_probe.go + Pool.JdtlsAdapter/RustAnalyzerAdapter accessors; ACC10 + JavaIntegration PASS |
| ENRICH-04 | 61-02, 61-04 | Per-file budget; partial_reason="budget exhausted" remains queryable | SATISFIED | Budget value type + cascade boundary check + MarkFileSemanticPending closed-enum; C2 + C9 PASS |
| ENRICH-05 | 61-04 | Stress test asserts foreground p95 < 5s | SATISFIED | TestStress_ENRICH05_Go locally PASS (p95=877µs); `//go:build stress` gating preserved |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/semantic/lspenrich/manager.go | 198-207 | NewManager constructs Worker without NewCascadeLSP factory | Warning | Production dispatch is a no-op (every job → OutcomeDropped). Acknowledged by executor as deferred to Phase 64+. |
| internal/semantic/lspenrich/cascade_test.go | 317 | `var cascadeNow = time.Now()` (package-level init, not per-test) | Warning | 6 cascade tests fail in `-tags integration` mode when ordered after long-running TestCascade_JavaIntegration; doc-comment claims "recomputed at test-call time" but the code is a `var` initializer. Tests pass in isolation and under `-short`. |
| internal/daemon/daemon.go | shutdown sequence | `live.Stop()` method exists but not explicitly invoked at daemon shutdown | Info | Acceptable: errgroup ctx-cancel triggers Manager.Run to call releaseAll() on exit (manager.go:221). The Stop() method is defense-in-depth. |

### Human Verification Required

#### 1. Confirm NewCascadeLSP production-wiring deferral is acceptable

**Test:** Review whether the gap "Manager.Run constructs Worker without NewCascadeLSP factory" — meaning every production-dispatched enrichment job lands on OutcomeDropped + Error log before the cascade engine runs — is acceptable scope for closing Phase 61.

**Expected:** Phase 64 explicitly accepts the CascadeLSP-over-WorkerLease adapter wiring as part of `refresh_semantic_graph` MCP tool. ROADMAP confirms Phase 64 owns `refresh_semantic_graph` + `get_semantic_graph_status`. Executor 61-04-SUMMARY decisions section documents this deferral. Acceptance integration tests TestACC4_Cap1/Cap4 + TestACC6_VoluntaryYield exercise the load-bearing cascade → ForegroundBusy seam end-to-end via direct Worker construction (with NewCascadeLSP injected).

**Why human:** Phase 61 ROADMAP Success Criteria 1-4 are all "what must be TRUE" statements that are achievable at the test surface (interface + analyzer + integration test) without requiring production end-to-end dispatch. The executor's deferral is plausible but reduces the "live daemon enriches files" capability to "Phase 61 ships the components; Phase 64 wires them into the live dispatch path". Architect judgement needed on whether this milestone phase can close.

#### 2. Confirm cascadeNow package-var test fixture is acceptable

**Test:** Inspect `internal/semantic/lspenrich/cascade_test.go:317` — the `var cascadeNow = time.Now()` package-level initializer that the doc-comment claims is "recomputed at test-call time" but isn't.

**Expected:** Either (a) patch cascadeNow to be a `func cascadeNow() time.Time { return time.Now() }` so it actually recomputes per test, or (b) document this as a known-flaky integration-tag-only ordering issue. Default `go test -short` is unaffected; `go test -tags integration` only fails 6 cascade tests when TestCascade_JavaIntegration runs first (which it does in alphabetical order).

**Why human:** This is a 1-line test-fixture fix but introducing it post-merge requires a follow-up commit. The bug does NOT affect production code or default CI.

### Gaps Summary

Phase 61 ships all 5 ROADMAP Success Criteria + ENRICH-01..05 at the unit/integration-test surface. The cascade engine, budget enforcement, voluntary yield, readiness gates, and concurrency cap are all correctly implemented and exercised end-to-end against real gopls + jdtls.

**Two notable items requiring human disposition:**

1. **Production wiring gap (deferred to Phase 64+):** Manager.Run constructs Worker without supplying `NewCascadeLSP` factory. The cascade engine therefore does not run from the live daemon today — every dispatched job emits OutcomeDropped + an Error log. The executor explicitly deferred the `CascadeLSP` adapter to Phase 64 (`refresh_semantic_graph` MCP tool). Acceptance/integration tests cover the load-bearing invariants by constructing Worker directly with the factory injected. This is documented and consistent with the ROADMAP's Phase 64 dependency, but the milestone effect is: "Phase 61 ships the building blocks; Phase 64 turns them on in production."

2. **Test fixture flakiness under `-tags integration`:** Six cascade unit tests (C1, C3, C4, C5, C6, C8) fail when run after the 9-second jdtls integration test, because `var cascadeNow = time.Now()` falls outside the 5-second per-file budget window. Tests pass in isolation and under `-short`. Production code is correct.

Otherwise: build clean, default tests green, both vet analyzers clean, semantic→kernel boundary preserved, REQUIREMENTS.md ENRICH-01..05 all checked off, cosign-style observability metrics + trace spans in place, B2 lease-cache singleflight invariant tested (100 concurrent → 1 acquire), B2 deactivate hook wired through gRPC + errgroup-cancel paths.

---

*Verified: 2026-05-06T10:35:00Z*
*Verifier: Claude (gsd-verifier)*
