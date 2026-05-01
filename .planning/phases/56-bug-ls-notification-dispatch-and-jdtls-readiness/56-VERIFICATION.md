---
phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness
verified: 2026-04-25T00:00:00Z
reverified: 2026-05-01T00:00:00Z
status: verified
score: 10/10 must-haves verified
overrides_applied: 0
human_verification_resolved:
  - test: "CI run of full suite including TestSymbols_JavaFixture and TestEdit_JavaFixture"
    resolution: "CI confirmed green — gh run 25068895178 (workflow go-test.yml, SHA c1a6cf55, 2026-04-28T17:50Z) conclusion=success. SHA c1a6cf55 is downstream of all Phase 56 commits (56-01 through 56-04). Java 21 + fresh jdtls install via Eclipse JDT.LS milestones; full `go test ./...` (no skip) green means TestSymbols_JavaFixture + TestEdit_JavaFixture passed with the dispatcher fix and waitJavaReady gate active. Two more recent runs (25068848930, 25068609876) also green. JDTLS-RDY-02 satisfied. The previously reported local warm-cache flakiness is no longer reproducing on this host (TestSymbols_JavaFixture PASS in 2.74s, TestEdit_JavaFixture PASS in 16.87s — 2026-05-01)."
    resolved_by: "/gsd-verify-work 56 (UAT Test 2 + Test 3 + Test 5)"
---

# Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness Verification Report

**Phase Goal:** Fix the silent-drop LS notification dispatch race and add jdtls readiness API so Java integration tests can deterministically wait for the language server to be project-ready before issuing LSP queries.

**Verified:** 2026-04-25
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | ProcessHandle.Start spawns process + creates Conn but does NOT launch Listen | VERIFIED | `process.go:61-101` — only drainStderr, reap, watchContext goroutines spawned. D-02 comment at lines 90-92 explicitly documents the removal. `grep -c 'go p\.conn\.Listen' = 1` (only in StartListen) |
| 2 | ProcessHandle.StartListen(ctx) is the only call site for `go p.conn.Listen(ctx)` | VERIFIED | `process.go:108-114` — only place that launches the dispatch loop |
| 3 | Worker.Start sets OnNotification BEFORE process.StartListen(ctx) | VERIFIED | `worker.go:190` reads handlers, `:193` assigns `OnNotification = buildDispatcher(...)`, `:198` runs assertion, `:205` calls `w.process.StartListen(ctx)`. Order is correct and BEFORE `initialize` Call |
| 4 | Dispatcher debug-logs and drops unknown methods (D-04) | VERIFIED | `worker.go::buildDispatcher` logs "unhandled LS notification" at Debug level for missing routes. Covered by `TestBuildDispatcher_UnknownMethodLogsAndDrops` |
| 5 | Dispatcher recovers from handler panics (D-05) and continues | VERIFIED | `buildDispatcher` uses `defer/recover`, logs "LS notification handler panicked" at Error level. Covered by `TestBuildDispatcher_PanicRecovers` (passing) |
| 6 | Worker.Start errors if quirks declare handlers but OnNotification ends up nil (D-11) | VERIFIED | `assertDispatcherWired` returns error + Error-logs; Worker stops + state→Stopped on failure. Covered by `TestWorker_DispatcherWiringRegression` |
| 7 | JdtlsAdapter implements language/status handler with ServiceReady + ProjectStatus=OK branches | VERIFIED | `quirks.go:345-363` — handler unmarshals defensively and routes to signalServiceReady / signalProjectReady. Malformed payloads no-op |
| 8 | JdtlsAdapter.WaitUntilJavaReady(ctx) blocks until BOTH events; honors ctx.Done; bounded by 90s timeout | VERIFIED | `quirks.go:421` const `javaReadinessTimeout = 90 * time.Second`; `WaitUntilJavaReady` (lines 427-449) sequentially awaits svc then proj with timer + ctx select |
| 9 | Java integration tests gate on WaitUntilJavaReady before symbol queries (D-10) | VERIFIED | `java_test.go:25-34` defines waitJavaReady; called at lines 54, 146, 172 (3 call sites — one for TestSymbols_JavaFixture, two subtests in TestEdit_JavaFixture). Uses `require.NoError` to fail loudly |
| 10 | LSDISP-REG-01 regression test exists under `//go:build integration` | VERIFIED | `rust_test.go:29` `TestRustAnalyzer_NotificationDispatchEndToEnd` — type-asserts RustAnalyzerAdapter, asserts `WaitUntilRenameReady` returns true. Test PASSES in 5.5s |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/kernel/lspool/process.go` | Start without auto-Listen + StartListen(ctx) | VERIFIED | StartListen at lines 108-114; eager Listen removed; `grep -c 'go p\.conn\.Listen' = 1` |
| `internal/kernel/lspool/process_test.go` | TestProcessHandle_StartListenSeparate | VERIFIED | Test exists; package tests pass |
| `internal/kernel/lspool/worker.go` | Dispatcher wiring + buildDispatcher + assertDispatcherWired + StartListen call | VERIFIED | All present at expected locations; package compiles and tests pass |
| `internal/kernel/lspool/worker_test.go` | TestBuildDispatcher_*, TestWorker_DispatcherWired, TestWorker_DispatcherWiringRegression | VERIFIED | All five test functions present; tests pass |
| `internal/kernel/jsonrpc/codec_test.go` | TestConn_NotificationUnknownMethod | VERIFIED | Test present; jsonrpc package tests pass |
| `internal/kernel/lspool/quirks.go` | JdtlsAdapter readiness + WaitUntilJavaReady + D-06 contract on QuirkAdapter interface | VERIFIED | All present; D-06 doc-comment lines 45-60 includes "non-blocking", "synchronously on the", "MUST NOT do I/O", "Phase 56 D-06" |
| `internal/kernel/lspool/quirks_test.go` | TestJdtlsAdapter_LanguageStatusReadiness + 3 sibling tests | VERIFIED | All four test functions present; tests pass |
| `internal/kernel/lspool/pool.go` | Pool.WorkerForTests test-only accessor | VERIFIED | Lines 223-235; uses RLock (not Lock) — read-only walk |
| `test/integration/harness.go` | TestDaemon.LSPoolWorker accessor | VERIFIED | Lines 71-77; thin wrapper over KernelInstance().Pool().WorkerForTests |
| `test/integration/java_test.go` | waitJavaReady helper invoked from both Java tests | VERIFIED | Helper at lines 25-34; 3 call sites confirmed |
| `test/integration/rust_test.go` | TestRustAnalyzer_NotificationDispatchEndToEnd under integration tag | VERIFIED | Test at line 29; build tag preserved |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| Worker.Start | ProcessHandle.StartListen | `w.process.StartListen(ctx)` AFTER OnNotification assignment | WIRED | worker.go:205, after line 193 (OnNotification assignment) and line 198 (assertion) |
| Worker.Start | Conn().OnNotification | `OnNotification = buildDispatcher(...)` | WIRED | worker.go:193 |
| buildDispatcher | log/slog | `logger.Debug` D-04, `logger.Error` D-05 | WIRED | "unhandled LS notification" + "LS notification handler panicked" |
| JdtlsAdapter handler | signalServiceReady/signalProjectReady | json.Unmarshal + branch | WIRED | quirks.go:354-360 |
| WaitUntilJavaReady | serviceReady/projectReady | ensureServiceReadyCh + ensureProjectReadyCh + select | WIRED | quirks.go:428-449 |
| QuirkAdapter.NotificationHandlers | D-06 contract | doc comment | WIRED | quirks.go:45-60 includes all required phrases |
| waitJavaReady | JdtlsAdapter.WaitUntilJavaReady | type assertion on Worker.Quirks() | WIRED | java_test.go:29-33 |
| TestRustAnalyzer_NotificationDispatchEndToEnd | RustAnalyzerAdapter.WaitUntilRenameReady | type assertion | WIRED | rust_test.go:40-44 |
| TestDaemon.LSPoolWorker | Pool.WorkerForTests | KernelInstance().Pool().WorkerForTests | WIRED | harness.go:75-77 |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| go vet clean | `go vet ./...` | exits 0 (only pre-existing CGO warning in swift binding, unrelated) | PASS |
| lspool + jsonrpc unit tests | `go test ./internal/kernel/lspool/... ./internal/kernel/jsonrpc/...` | PASS in ~5s | PASS |
| Rust dispatch end-to-end regression | `go test -tags=integration ./test/integration/... -run TestRustAnalyzer_NotificationDispatchEndToEnd` | PASS in 5.5s | PASS |
| Full suite excluding pre-existing Java failures | `go test ./... -skip 'TestSymbols_JavaFixture\|TestEdit_JavaFixture'` | All packages pass | PASS |
| Full suite (including Java fixtures) | `go test ./...` | TestSymbols_JavaFixture + TestEdit_JavaFixture FAIL (pre-existing locally; documented out-of-scope) | SKIP (deferred to CI) |
| Process listen contract | `grep -c 'go p\.conn\.Listen' internal/kernel/lspool/process.go` | returns 1 (only StartListen) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| LSDISP-01 | 56-02 | OnNotification set when handlers registered | SATISFIED | TestWorker_DispatcherWired (Shape B); worker.go:193 |
| LSDISP-02 | 56-01 | ProcessHandle.Start does NOT auto-launch Listen | SATISFIED | TestProcessHandle_StartListenSeparate; process.go grep verified |
| LSDISP-03 | 56-02 | Dispatcher recovers from handler panic and continues | SATISFIED | TestBuildDispatcher_PanicRecovers; buildDispatcher defer/recover |
| LSDISP-04 | 56-02 | Worker.Start errors if dispatcher unwired (D-11) | SATISFIED | assertDispatcherWired + TestWorker_DispatcherWiringRegression |
| LSDISP-04b | 56-02 | Unhandled method debug-logs and drops | SATISFIED | TestConn_NotificationUnknownMethod + TestBuildDispatcher_UnknownMethodLogsAndDrops |
| JDTLS-RDY-01a | 56-03 | JdtlsAdapter registers `language/status` handler | SATISFIED | quirks.go:345-363; TestJdtlsAdapter_LanguageStatusReadiness |
| JDTLS-RDY-01b | 56-03 | Malformed payload no-ops | SATISFIED | TestJdtlsAdapter_LanguageStatusMalformed |
| JDTLS-RDY-01c | 56-03 | WaitUntilJavaReady blocks until both; honors ctx | SATISFIED | TestJdtlsAdapter_WaitUntilJavaReady_ContextCancel + Readiness |
| JDTLS-RDY-02 | 56-04 | Java fixture tests gated on WaitUntilJavaReady (D-15) | SATISFIED (gating) / NEEDS HUMAN (downstream pass) | waitJavaReady wired at 3 call sites; gate proven correct (gates close in ~2s); downstream symbol assertions flake locally — needs CI confirmation |
| LSDISP-REG-01 | 56-04 | Real rust-analyzer quiescent flips after worker start (D-14) | SATISFIED | TestRustAnalyzer_NotificationDispatchEndToEnd PASSES in 5.5s |

Note: The LSDISP-* and JDTLS-RDY-* IDs are not present in `.planning/REQUIREMENTS.md` (which tracks higher-level milestone requirements). They are defined in plan frontmatter and 56-RESEARCH.md and are considered phase-internal requirement IDs. No orphans found in REQUIREMENTS.md mapped to Phase 56.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | All file scans clean. Documented "Phase 56 D-XX" markers are intentional traceability comments. The pre-existing CGO warning in `internal/treesitter/bindings/swift/src/scanner.c` is unrelated to this phase. |

### Human Verification Required

#### 1. CI run of full suite including Java fixtures

**Test:** Trigger CI run for the merged Phase 56 branch and observe `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` outcomes.
**Expected:** Both tests pass deterministically in CI. CI uses fresh jdtls per run (vs. local warm cache), and the per-task brief explicitly states CI is authoritative.
**Why human:** Local verification confirms (a) waitJavaReady gates ServiceReady + ProjectStatus=OK in ~2s and (b) the failures pre-exist at base commit (verified via git stash). The downstream `get_hover_info` / `find_references_cross_file` / `replace_body` failures appear specific to this developer machine's warm-cache jdtls fixture and predate Phase 56. JDTLS-RDY-02 satisfaction depends on CI green confirmation.

### Gaps Summary

No code-level gaps. Phase 56 fully delivers:
- ProcessHandle/Worker lifecycle is restructured so `OnNotification` is wired before `Listen` starts.
- A defensive, panic-recovering, unknown-method-dropping dispatcher closure (`buildDispatcher`) is in place with a regression assertion (`assertDispatcherWired`).
- D-06 handler-concurrency contract is now baked into the `QuirkAdapter.NotificationHandlers` interface declaration.
- JdtlsAdapter mirrors RustAnalyzerAdapter's readiness machinery with two channels (ServiceReady + ProjectStatus=OK) and a public `WaitUntilJavaReady(ctx) error`.
- Java integration tests gate on the new readiness API at all 3 `StartTestDaemon` call sites.
- `TestRustAnalyzer_NotificationDispatchEndToEnd` proves the wired dispatch path end-to-end through real rust-analyzer (passes in 5.5s).

The single open item is CI confirmation of the Java fixture tests, which the task brief and 56-04-SUMMARY.md both explicitly mark as out-of-scope/pre-existing and route to CI as authoritative.

---

_Verified: 2026-04-25_
_Verifier: Claude (gsd-verifier)_
