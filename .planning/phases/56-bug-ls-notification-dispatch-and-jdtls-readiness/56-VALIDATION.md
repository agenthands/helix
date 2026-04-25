---
phase: 56
slug: bug-ls-notification-dispatch-and-jdtls-readiness
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-25
---

# Phase 56 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `56-RESEARCH.md` §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.25 stdlib) |
| **Config file** | none (`go test` discovers via package layout) |
| **Quick run command** | `go test ./internal/kernel/jsonrpc/... ./internal/kernel/lspool/... -run 'Notification|Jdtls|Dispatcher' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Integration regression command** | `go test -tags=integration ./test/integration/... -run 'TestRustAnalyzer_NotificationDispatchEndToEnd|TestSymbols_JavaFixture|TestEdit_JavaFixture' -count=1` |
| **Estimated runtime** | ~3s quick, ~60s full (excludes integration tag), ~3-5min with integration tag (jdtls cold start dominant) |

---

## Sampling Rate

- **After every task commit:** Run quick command (≤3s)
- **After every plan wave:** Run full suite + `go vet ./...` (≤60s without integration tag)
- **Before `/gsd-verify-work`:** Full suite + integration tag full suite, both green
- **Max feedback latency:** 60s during execution; ~5min at phase gate

---

## Per-Task Verification Map

> Populated by planner. Each task in PLAN.md must map to a row here. Req IDs from research:
> LSDISP-01..04b, JDTLS-RDY-01a..01c, JDTLS-RDY-02, LSDISP-REG-01.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | LSDISP-01 | — | OnNotification set when handlers registered | unit | `go test ./internal/kernel/lspool/... -run 'TestWorker_DispatcherWired' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | LSDISP-02 | — | ProcessHandle.Start does NOT auto-launch Listen | unit | `go test ./internal/kernel/lspool/... -run 'TestProcessHandle_StartListenSeparate' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | LSDISP-03 | — | Dispatcher recovers from handler panic and continues | unit | `go test ./internal/kernel/jsonrpc/... -run 'TestConn_NotificationHandlerPanic' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | LSDISP-04 | — | Worker.Start errors if dispatcher unwired with non-empty handlers (D-11) | unit | `go test ./internal/kernel/lspool/... -run 'TestWorker_DispatcherWiringRegression' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | LSDISP-04b | — | Unhandled method debug-logs and drops | unit | `go test ./internal/kernel/jsonrpc/... -run 'TestConn_NotificationUnknownMethod' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | JDTLS-RDY-01a | — | JdtlsAdapter registers `language/status` handler | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_LanguageStatusReadiness' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | JDTLS-RDY-01b | — | Malformed `language/status` payload no-ops | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_LanguageStatusMalformed' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | JDTLS-RDY-01c | — | `WaitUntilJavaReady` blocks until both events; honors ctx cancel | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_WaitUntilJavaReady' -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | JDTLS-RDY-02 | — | Java fixture: hover, cross-file refs, replace_symbol_body green (D-15) | integration | `go test ./test/integration/... -run 'TestSymbols_JavaFixture\|TestEdit_JavaFixture' -count=1` | ✅ exists, needs Wait gate | ⬜ pending |
| TBD | TBD | TBD | LSDISP-REG-01 | — | Real rust-analyzer `quiescent` flips after worker start (D-14) | integration (build-tag) | `go test -tags=integration ./test/integration/... -run 'TestRustAnalyzer_NotificationDispatchEndToEnd' -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/kernel/jsonrpc/codec_test.go` — extend with `TestConn_NotificationUnknownMethod`, `TestConn_NotificationHandlerPanic` (D-12)
- [ ] `internal/kernel/lspool/quirks_test.go` — extend with `TestJdtlsAdapter_LanguageStatusReadiness`, `TestJdtlsAdapter_LanguageStatusMalformed`, `TestJdtlsAdapter_WaitUntilJavaReady` (D-13), `TestJdtlsAdapter_ImplementsQuirkAdapter`
- [ ] `internal/kernel/lspool/worker_test.go` — scaffold if missing; add `TestWorker_DispatcherWired`, `TestWorker_DispatcherWiringRegression` (D-11)
- [ ] `internal/kernel/lspool/process_test.go` — scaffold if missing; add `TestProcessHandle_StartListenSeparate` (D-02 contract)
- [ ] `test/integration/rust_test.go` — extend with `TestRustAnalyzer_NotificationDispatchEndToEnd` (D-14) under `//go:build integration`
- [ ] `test/integration/java_test.go` — modify `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` to call `waitJavaReady` after `StartTestDaemon` (D-10)
- [ ] Verify `td.Daemon.LSPool().GetWorker(...)` accessor exists; if not, add a test-only helper in `test/integration/harness.go` (do NOT widen public Pool API)
- [ ] Verify `.github/workflows/go-test.yml` installs `jdtls` before `go test ./...` — otherwise D-15 silently skips in CI (A4)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none) | — | All phase behaviors have automated verification. | — |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
