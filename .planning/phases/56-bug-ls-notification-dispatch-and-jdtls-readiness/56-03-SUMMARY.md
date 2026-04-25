---
phase: 56
plan: 03
subsystem: lspool
tags:
  - lspool
  - jdtls
  - readiness
  - quirks
requires:
  - QuirkAdapter interface (existing)
  - RustAnalyzerAdapter Phase 47 prior art (mirror)
provides:
  - JdtlsAdapter.NotificationHandlers["language/status"]
  - JdtlsAdapter.WaitUntilJavaReady(ctx) error
  - javaReadinessTimeout constant (90s)
  - D-06 doc-comment contract on QuirkAdapter.NotificationHandlers
affects:
  - Plan 04 (consumes WaitUntilJavaReady from the Java integration gate)
tech-stack:
  patterns:
    - "Mirror-of-prior-art (RustAnalyzerAdapter readiness machinery)"
    - "Two-channel sequential gate (ServiceReady + ProjectStatus=OK)"
key-files:
  modified:
    - internal/kernel/lspool/quirks.go
    - internal/kernel/lspool/quirks_test.go
decisions:
  - "Mirror RustAnalyzerAdapter ensure/signal helper shape verbatim — twice, one channel per gate — rather than abstracting a shared helper, so the readiness pattern stays grep-able per adapter."
  - "javaReadinessTimeout = 90s (vs. legacy's 20s hotfix in eclipse_jdtls.py:921-927) keeps the strict both-gates contract while raising the ceiling for cold-cache jdtls startups."
  - "D-06 contract documented on the QuirkAdapter interface declaration itself (not just on RustAnalyzerAdapter / JdtlsAdapter implementations) so future adapter authors see the synchronous-non-blocking expectation at the source."
metrics:
  duration: ~10m
  completed: 2026-04-25
---

# Phase 56 Plan 03: jdtls language/status readiness gate Summary

Mirror of RustAnalyzerAdapter's readiness machinery applied to JdtlsAdapter, observing `language/status` notifications and exposing `WaitUntilJavaReady(ctx) error` that blocks until BOTH `ServiceReady` AND `ProjectStatus=OK` have been seen. Also locks the D-06 handler-concurrency contract into the `QuirkAdapter.NotificationHandlers` interface declaration so every future adapter author inherits the synchronous, non-blocking expectation.

## Tasks Completed

| # | Task | Commit |
|---|------|--------|
| 1 | RED: extend quirks_test.go with four jdtls readiness tests | `babb398a` |
| 2 | GREEN: implement JdtlsAdapter readiness + WaitUntilJavaReady + D-06 interface doc | `68ee70bf` |
| 3 | Run go vet + targeted lspool test gate | (verification only — no commit) |

## Code Locations

### `internal/kernel/lspool/quirks.go`
- D-06 doc-comment on `QuirkAdapter.NotificationHandlers`: lines 45-60
- Extended `JdtlsAdapter` struct (with `readyMu`, `serviceReady`, `projectReady`): lines 318-330
- `NotificationHandlers["language/status"]` handler: lines 339-363
- `ensureServiceReadyCh` + `signalServiceReady`: lines 365-389
- `ensureProjectReadyCh` + `signalProjectReady`: lines 391-415
- `javaReadinessTimeout` constant (90s): line 421
- `WaitUntilJavaReady(ctx) error`: lines 427-449

### `internal/kernel/lspool/quirks_test.go`
- `TestJdtlsAdapter_ImplementsQuirkAdapter`: appended after existing jdtls ExtraArgs tests
- `TestJdtlsAdapter_LanguageStatusReadiness`: covers D-08 (both gates required)
- `TestJdtlsAdapter_LanguageStatusMalformed`: T-56-07
- `TestJdtlsAdapter_WaitUntilJavaReady_ContextCancel`: pre-cancel returns within 100ms

## Imports

`fmt` was NOT previously imported in `quirks.go` and was added (alongside the existing `context`, `encoding/json`, `os`, `path/filepath`, `strings`, `sync`, `sync/atomic`, `time`, `langregistry`, `gen`). Required for `fmt.Errorf` in WaitUntilJavaReady timeout error returns.

## Verification

- `go vet ./internal/kernel/lspool/...` exits 0
- `go vet ./...` exits 0 (only pre-existing C macro-redefinition warning from swift tree-sitter binding, unrelated)
- `go test ./internal/kernel/lspool/... -count=1` exits 0
- `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter|TestRustAnalyzerAdapter' -count=1` exits 0 (all four new jdtls tests + existing rust-analyzer tests pass)

## Acceptance Criteria — Status

- [x] D-06 interface doc-comment present (`Phase 56 D-06`, `non-blocking`, `synchronously on the`, `MUST NOT do I/O` all grep-confirmed)
- [x] `serviceReady chan struct{}` + `projectReady chan struct{}` fields present
- [x] `"language/status"` handler registered
- [x] All four readiness helpers present (ensure/signal × Service/Project)
- [x] `WaitUntilJavaReady` present
- [x] `javaReadinessTimeout = 90` present
- [x] Both signal branches reachable from handler
- [x] `go vet` clean, targeted tests green
- [x] `JdtlsAdapter.NotificationHandlers()` returns non-nil map with `language/status`
- [x] Handler closes `serviceReady` on `{type:ServiceReady, message:ServiceReady}` and `projectReady` on `{type:ProjectStatus, message:OK}`
- [x] `WaitUntilJavaReady(ctx)` returns nil iff both gates close; honors ctx cancel; bounded by 90s
- [x] Signal helpers idempotent + mutex-protected (mirror of Phase 47 prior art)

Requirements satisfied: JDTLS-RDY-01a, JDTLS-RDY-01b, JDTLS-RDY-01c.

## Deviations from Plan

None — plan executed exactly as written. The shape, helper names, channel layout, timeout constant, and D-06 interface doc-comment all match the planned text verbatim. `fmt` was added per the plan's contingency note ("ADD fmt if not present").

## Threat Mitigations Applied

- T-56-07 (Tampering / malformed payloads): `json.Unmarshal` errors return early; wrong-type fields leave zero values which fail equality checks. Verified by `TestJdtlsAdapter_LanguageStatusMalformed`.
- T-56-08 (DoS / unbounded wait): `javaReadinessTimeout = 90s` ceiling enforced via `time.NewTimer` in both select stages; caller ctx deadline takes precedence if shorter.
- T-56-09 (DoS / signal helper concurrency): mutex-guarded; idempotent close pattern verified by mirror of Phase 47 RustAnalyzerAdapter prior art.
- T-56-12 (Tampering / handler concurrency contract): D-06 doc-comment locks the synchronous non-blocking expectation into the interface declaration itself; every future adapter author sees it at the source.

## Self-Check: PASSED

- internal/kernel/lspool/quirks.go: FOUND
- internal/kernel/lspool/quirks_test.go: FOUND
- commit babb398a: FOUND
- commit 68ee70bf: FOUND
