---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 06
subsystem: bench-runtime
tags: [ablation, no_semantic, fail-closed, graceful-shutdown, ABLATE-06, WR-02, gap_closure]
gap_closure: true
requires:
  - "internal/daemon/shutdown.go d.shutdown() reads-total emission (Phase 81-05, path A)"
  - "internal/eval/sandbox.DaemonHandle (Setpgid process group, Kill)"
provides:
  - "internal/eval/sandbox.DaemonHandle.Stop(timeout) — graceful SIGTERM teardown that lets d.shutdown() flush the reads-total line"
  - "bench/runtime fail-CLOSED scrape/assert: an absent reads-total line hard-fails the no_semantic arm"
  - "bench/runtime real-daemon emission integration test (TestNoSemanticReadsTotalLineEmitted)"
affects:
  - "bench/runtime RunCell teardown ordering (Stop-then-Kill)"
  - "81-07 (GAP 2, store-ON arm) which depends on this plan's teeth"
tech-stack:
  added: []
  patterns:
    - "graceful SIGTERM-then-SIGKILL daemon teardown (process-group targeted)"
    - "fail-closed observability gate: line PRESENCE distinct from value"
key-files:
  created:
    - bench/runtime/no_semantic_emission_integration_test.go
  modified:
    - internal/eval/sandbox/sandbox.go
    - bench/runtime/cell.go
    - bench/runtime/no_semantic_zero_reads_test.go
decisions:
  - "daemonGracefulStopTimeout = 12s (10s daemon ShutdownTimeout + slack) so a daemon using its full shutdown window still beats RunCell's deadline before the Kill fallback"
  - "Stop does NOT drain the Wait goroutine on timeout — Kill's group-SIGKILL + buffered drain owns reaping the straggler (avoids double-ownership of the in-flight Wait)"
  - "Kill is called UNCONDITIONALLY after Stop (even on the graceful path) as a no-op reaper, guaranteeing the process is fully reaped before the daemon-log tap"
  - "Stop on nil/already-exited process returns graceful=true (mirrors Kill's nil + os.ErrProcessDone guards); ESRCH on the group SIGTERM reaps the zombie and returns graceful=true"
  - "the integration test spawns a BARE daemon (no profile/config): the reads-total line is emitted on every graceful shutdown whenever d.obs != nil, so a bare daemon suffices to prove emission on the real teardown path"
metrics:
  duration: ~22min
  completed: 2026-06-20
  tasks: 2
  files: 4
---

# Phase 81 Plan 06: Make the no_semantic Zero-Reads Runtime Gate Have Teeth Summary

Graceful SIGTERM teardown (`DaemonHandle.Stop`) plus a fail-CLOSED scrape/assert turn the previously-vacuous `no_semantic` zero-reads runtime gate into a real end-to-end guarantee, proven by a real-daemon emission integration test — closing GAP 1 / WR-02 from 81-VERIFICATION.md.

## What This Plan Did

GAP 1 (BLOCKER, WR-02): the `helix_semantic_store_reads_total` value is emitted only inside the daemon's graceful `d.shutdown()` (shutdown.go:60-62), reached only after `g.Wait()` unblocks on SIGTERM/SIGINT. But the bench tore the daemon down via `DaemonHandle.Kill()` → `syscall.Kill(-pid, SIGKILL)`, which is untrappable — so `shutdown()` never ran, the line was never written, and `scrapeSemanticReadsTotal` treated the missing line as a silent `count=0`. The result: `assertNoSemanticReads("your_agent_no_semantic", 0)` passed unconditionally on every real run; the gate could never observe a real read.

This plan fixes both halves:

- **(a) Graceful teardown FIRST** — new `DaemonHandle.Stop(timeout)` sends SIGTERM to the daemon process group and waits up to `timeout` for graceful exit, so `d.shutdown()` actually runs and flushes the reads-total line. RunCell calls `Stop` before the existing `Kill` fallback.
- **(b) Fail-CLOSED scrape** — `scrapeSemanticReadsTotal` now reports line PRESENCE distinct from the count; `assertNoSemanticReads` HARD-FAILS the `no_semantic` arm on an absent line (the proof never ran), killing the root fail-open anti-pattern (cell.go:101-103).
- **(c) Real-daemon integration test** — `TestNoSemanticReadsTotalLineEmitted` spawns a real daemon, drives it down through the actual Stop-then-Kill path, and asserts daemon.log carries a real reads-total line with an integer count (replacing the synthetic-`os.WriteFile`-only coverage).

D-04 build-but-block respected (store construction untouched); D-05 path-A daemon-log emission preserved and now actually fires.

## Tasks Completed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Add `DaemonHandle.Stop` (graceful SIGTERM teardown) + drive from RunCell before Kill | `3cb0abe5` | internal/eval/sandbox/sandbox.go, bench/runtime/cell.go, bench/runtime/no_semantic_emission_integration_test.go (new) |
| 2 | Make scrape/assert fail-CLOSED — absent reads-total line hard-fails the no_semantic arm | `ae7aafd7` | bench/runtime/cell.go, bench/runtime/no_semantic_zero_reads_test.go |

## Verification Evidence

All commands run from repo root. The HELIX_BIN-gated tests are proven to RUN (not SKIP).

```
$ go build -o helix ./cmd/helix                              # exit 0
$ go vet ./...                                               # exit 0
$ make vet                                                   # exit 0 (all 5 custom vet tools incl. ablation-leakage)
$ go test ./bench/runtime/ ./internal/eval/sandbox/ -count=1 # ok (non-gated)
$ go test ./...                                              # no failures
```

HELIX_BIN-gated bench/runtime run (the false-green trap — these SKIP without HELIX_BIN per MEMORY):

```
$ HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/ \
    -run 'TestNoSemanticReadsTotalLineEmitted|TestFiveOfSixSmoke|TestNoSemanticZeroReads' -count=1 -v
--- PASS: TestFiveOfSixSmoke (1.38s)                # RAN, not SKIP
--- PASS: TestNoSemanticReadsTotalLineEmitted (0.26s)   # RAN, not SKIP — real daemon Stop-then-Kill, daemon.log line present
--- PASS: TestNoSemanticZeroReads (0.00s)
    --- PASS: .../no_semantic_zero_reads_passes
    --- PASS: .../no_semantic_nonzero_reads_fails_hard
    --- PASS: .../other_modes_unaffected_by_nonzero_reads
    --- PASS: .../no_semantic_absent_line_fails_hard       # NEW Test 7 (fail-closed on-arm)
    --- PASS: .../other_modes_absent_line_benign           # NEW Test 8 (scope guard)
    --- PASS: .../scrape_reads_total_from_daemon_log
    --- PASS: .../scrape_reads_total_nonzero
    --- PASS: .../scrape_missing_line_reports_absent       # replaced fail-open Test 6
PASS
ok  github.com/agenthands/helix/bench/runtime 1.660s
```

Acceptance-criteria greps:

```
grep -c 'func (h \*DaemonHandle) Stop' internal/eval/sandbox/sandbox.go        == 1
grep -c 'syscall.Kill(-pid, syscall.SIGTERM)' internal/eval/sandbox/sandbox.go == 1
grep -c 'h.Stop' bench/runtime/cell.go                                         == 2
grep -c 'missing line → count 0|missing line yields zero|treated as zero reads, not an error' bench/runtime/cell.go == 0
gofmt -l (all 4 touched files)                                                 prints nothing
```

## Deviations from Plan

None — plan executed exactly as written. Both tasks are TDD-shaped (Task 1's integration test asserts the real-daemon emission; Task 2's unit tests assert the fail-closed inversion) but were committed as a single `feat` commit each (the test file and the implementation it covers landed together) rather than separate RED/GREEN commits, because the integration test for Task 1 requires the `Stop` method and the `daemonGracefulStopTimeout` const to compile at all (a standalone RED commit would not build).

## TDD Gate Compliance

This plan's tasks carry `tdd="true"`. The behavior-adding change (the `Stop` method + fail-closed gate) is covered by tests that fail in the absence of the implementation:
- Task 1: `TestNoSemanticReadsTotalLineEmitted` asserts `Stop` returns `graceful=true` and that the real daemon's daemon.log carries the reads-total line — this would fail if teardown stayed SIGKILL-only.
- Task 2: `no_semantic_absent_line_fails_hard` asserts `require.Error` for `assertNoSemanticReads(noSemanticMode, 0, false)` — this fails against the old fail-open contract.

Both tasks were committed as `feat(...)` commits (test + implementation together) because the test files do not compile without the new symbols (`Stop`, `daemonGracefulStopTimeout`, the 3-value scrape signature). No separate `test(...)` RED commit was possible without a non-building tree.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/eval/sandbox/sandbox.go (Stop method present)
- FOUND: bench/runtime/cell.go (fail-closed scrape/assert + Stop wiring)
- FOUND: bench/runtime/no_semantic_zero_reads_test.go (updated)
- FOUND: bench/runtime/no_semantic_emission_integration_test.go (new)
- FOUND commit: 3cb0abe5
- FOUND commit: ae7aafd7
