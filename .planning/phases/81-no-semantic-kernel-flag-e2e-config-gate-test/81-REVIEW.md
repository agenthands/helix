---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
reviewed: 2026-06-20T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - bench/runtime/cell.go
  - bench/runtime/no_semantic_emission_integration_test.go
  - bench/runtime/no_semantic_store_on_test.go
  - bench/runtime/no_semantic_zero_reads_test.go
  - internal/daemon/daemon.go
  - internal/daemon/daemon_test_export_test.go
  - internal/daemon/semantic_gate.go
  - internal/daemon/semantic_gate_test.go
  - internal/eval/sandbox/sandbox.go
  - internal/semantic/live/handler/handler.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: resolved
resolved: 2026-06-20T00:00:00Z
---

# Phase 81: Code Review Report

**Reviewed:** 2026-06-20
**Depth:** standard
**Files Reviewed:** 10
**Status:** resolved

> **Resolution (2026-06-20):** All in-scope findings fixed and verified
> (race-detector clean, `make vet` green, HELIX_BIN-gated bench tests run+pass).
> - CR-01 — fixed in `63b4a6b0` (single owned daemon Wait goroutine; Stop/Kill select on shared `exited` channel).
> - WR-01/WR-02/WR-03/IN-03 — fixed in `58c057d0` (graceful-stop budget 12s→20s; strict `graceful` assertion downgraded to logged expectation; malformed reads-total `count` now fail-closed via `*int`; scraper/test parsers kept in lockstep).
> - WR-04 — addressed (doc-only) in `cc61320f` (counter guarantee scoped to `effective_graph.go` counting wrappers; no reads rerouted).
> - IN-01 (harmless, documented) and IN-02 (pre-existing keyword-surfacing no-op tied to `vet-ablation-leakage`) intentionally left untouched.

## Summary

Phase 81 gap-closure (plans 81-06 / 81-07) for ABLATE-06. The two-part design is
sound and the intent is well realized:

- **D-04 build-but-block invariant holds.** Verified the store-Open guard
  (`daemon.go:330 if cfg.SemanticIndex.Enabled`), the live-bundle build
  (`daemon.go:393 if !effDisableLSP`), and the `newSemanticBundle` build path
  (`daemon.go:532`) are all free of `effSemanticDisabled` as a *construction*
  predicate; `effSemanticDisabled` is passed into the bundle only to skip the
  accessor block, not to skip building. Gating consistently un-wires
  READ-DRIVERS only (`SetFileFactStore` at daemon.go:437, the five
  `SetActivateCallback` drivers at daemon.go:995-1037, the lazy-activate
  `ScheduleInitialExtraction` at daemon.go:930). `TestSemanticBackgroundPipelinesGated`
  asserts `semanticBundleForTest() != nil` and `SemanticStore() != nil` under the
  gate. The zero-reads proof is therefore non-vacuous. Good.
- **Fail-closed scrape is correct in shape.** `scrapeSemanticReadsTotal` returns
  `(count, present)`; `assertNoSemanticReads` hard-fails on `!present` on the
  no_semantic arm only; the daemon emits the line on every graceful shutdown
  whenever `d.obs != nil` (shutdown.go:60-62); the msg constant matches.
- **handler.go** change is the additive nil-safe `HasFactStore()` accessor only —
  consistent with the gating intent, no defect.

However, the graceful-teardown sequence in `sandbox.go` has a real concurrency
defect: `DaemonHandle.Stop` and `DaemonHandle.Kill` can call `(*exec.Cmd).Wait()`
concurrently on the same `Cmd`, which the stdlib forbids. There are also
robustness gaps around the 12s graceful-stop budget vs. the daemon's worst-case
shutdown wall-time, a strict `graceful=true` assertion in the emission test, and a
malformed-count blind spot in the scraper.

## Critical Issues

### CR-01: Concurrent `(*exec.Cmd).Wait()` from `Stop` then `Kill` on the non-graceful path

**File:** `internal/eval/sandbox/sandbox.go:230-240` (Stop) and `:264-277` (Kill)
**Issue:**
`os/exec` documents `Cmd.Wait()` as call-once and not safe for concurrent use; a
second call returns `exec: Wait was already called` instead of the real exit
status, and two concurrent calls race on `Cmd.ProcessState` and the internal
bookkeeping goroutine.

On the **timeout / non-graceful path** the contract is violated:

1. `Stop` launches goroutine **G1** = `go func() { done <- h.cmd.Wait() }()`
   (line 231) and on `<-time.After(timeout)` returns `graceful=false` WITHOUT
   draining `done`. G1 is still blocked in `Wait()` because the process has not
   exited (that is *why* the timeout fired).
2. `RunCell` (cell.go:593) then unconditionally calls `h.Kill()`.
3. `Kill` sends the group SIGKILL (line 254), calls `cmd.Process.Kill()` (returns
   nil while the process is still live, so the `ErrProcessDone` branch is
   skipped), then launches goroutine **G2** = `go func() { done <- h.cmd.Wait() }()`
   (line 266).

In the window between G2's launch and the process being reaped, **G1 and G2 both
execute `h.cmd.Wait()` on the same `*exec.Cmd` concurrently.** The buffered-channel
comment addresses goroutine *leakage*, not the concurrent-`Wait` invariant — both
goroutines run `Wait` simultaneously regardless of buffer depth. One call returns
the "already called" error rather than reaping, so the process may be left unreaped
and the exit status is lost.

A secondary, benign instance is on the **graceful path**: G1 in Stop has already
returned before `Stop` returns, then `Kill` calls `h.cmd.Wait()` again at line 258
(via the `ErrProcessDone` branch). That call is serial (not racing) and only
returns a discarded "already called" error.

**Impact:** flakiness under `-race`, a possibly-leaked unreaped daemon when the
losing `Wait` returns the error instead of reaping, and — because the reads-total
line is scraped only *after* `Kill` returns — an intermittently-incomplete
`daemon.log` that the fail-closed gate then turns into a spurious HARD cell
failure. This is precisely the teardown path 81-06 made load-bearing.

**Fix:** own a single `Wait` goroutine in the handle (started in `StartDaemon`)
and have both `Stop` and `Kill` select on a shared `exited <-chan error`:

```go
type DaemonHandle struct {
	cmd    *exec.Cmd
	taskID string
	mode   string
	exited chan error // the ONLY Wait() lives here, started in StartDaemon
}

// after cmd.Start() in StartDaemon:
h.exited = make(chan error, 1)
go func() { h.exited <- h.cmd.Wait() }()

func (h *DaemonHandle) Stop(timeout time.Duration) (bool, error) {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return true, nil
	}
	pid := h.cmd.Process.Pid
	if kerr := syscall.Kill(-pid, syscall.SIGTERM); kerr != nil && !errors.Is(kerr, syscall.ESRCH) {
		return false, fmt.Errorf("sandbox: SIGTERM daemon %s/%s: %w", h.taskID, h.mode, kerr)
	}
	select {
	case <-h.exited:
		return true, nil
	case <-time.After(timeout):
		return false, nil
	}
}

func (h *DaemonHandle) Kill() error {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	pid := h.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = h.cmd.Process.Kill()
	select {
	case <-h.exited:
		return nil
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-h.exited
		return fmt.Errorf("sandbox: daemon %s/%s did not exit within 5s after kill", h.taskID, h.mode)
	}
}
```

This guarantees exactly one `Wait()` for the process lifetime; `Stop` and `Kill`
both observe the same exit signal without ever racing.

## Warnings

### WR-01: 12s graceful-stop budget is shorter than the daemon's worst-case shutdown wall-time

**File:** `bench/runtime/cell.go:71-78` (`daemonGracefulStopTimeout = 12 * time.Second`); cross-ref `internal/daemon/shutdown.go:12-62`
**Issue:**
The comment claims the 12s budget is "aligned with the daemon's default
`Daemon.ShutdownTimeout` (10s) ... so a daemon that is genuinely draining ... has
the full shutdown window." But `shutdown()` is not bounded by `ShutdownTimeout`
alone: the kernel `Shutdown(ctx)` uses the `ShutdownTimeout` context (up to 10s),
and the trace-exporter flush at `shutdown.go:43` uses a **separate 5s context**
that runs *after* the kernel shutdown. Worst-case `shutdown()` wall-time is
therefore ≈ 10s + 5s ≈ 15s, plus the post-flush reads-total emit and listener
close. A daemon that uses most of its kernel-drain window followed by a slow trace
flush exceeds 12s.

When that happens `Stop` times out, returns `graceful=false`, and `RunCell`
escalates to `Kill` — which SIGKILLs the daemon *mid-shutdown*, potentially before
`shutdown.go:60-62` emits the reads-total line. The fail-closed gate
(`!present`) then turns a slow-but-correct shutdown into a spurious HARD cell
failure, converting a latency hiccup into a false ABLATE-06 violation.

**Fix:** size the budget off the true worst case (kernel `ShutdownTimeout` + the
5s trace flush + slack), e.g.:

```go
// kernel drain (ShutdownTimeout, default 10s) + trace flush (5s, shutdown.go:43) + slack
const daemonGracefulStopTimeout = 20 * time.Second
```

and correct the comment to account for the additive 5s flush window.

### WR-02: Emission integration test's strict `require.True(graceful)` will flake on slow shutdown

**File:** `bench/runtime/no_semantic_emission_integration_test.go:71-76`
**Issue:**
`require.True(t, graceful, ...)` hard-fails whenever the real daemon does not exit
within `daemonGracefulStopTimeout`. Given WR-01 (the budget can be shorter than the
daemon's worst-case shutdown) and that this is a real-daemon test subject to host /
CI load, the assertion is a flake source. The load-bearing proof here is the
*presence of the reads-total line emitted by the graceful path* (lines 80-86), not
strictly that the first `Stop` won the race within 12s.

**Fix:** either raise the budget per WR-01 so the graceful window comfortably
exceeds the daemon's worst case, or keep the presence assertion as load-bearing and
downgrade the `graceful` flag to a logged expectation:

```go
graceful, stopErr := h.Stop(daemonGracefulStopTimeout)
require.NoError(t, stopErr, "graceful Stop must not error")
if !graceful {
	t.Logf("daemon did not exit within %s; relying on line-presence assertion", daemonGracefulStopTimeout)
}
require.NoError(t, h.Kill())
present, count := scanReadsTotalLine(t, daemonLog)
require.True(t, present, "...")
```

### WR-03: `scrapeSemanticReadsTotal` treats a `msg`-present-but-`count`-malformed line as a clean count=0

**File:** `bench/runtime/cell.go:114-121`
**Issue:**
The scraper unmarshals into `readsTotalLine{Msg string; Count int}`. A line whose
`msg` matches but whose `count` is JSON-null or wrong-typed unmarshals with
`raw.Count` silently at its zero value while `present` is set true — recording
`(0, true)`, a clean pass on the no_semantic arm even though the real count was
never parsed. The phase's own stated concern ("line-present-but-malformed") is only
half-closed: presence is validated, but a garbled count masquerades as 0. (A
line that fails to unmarshal *entirely* is correctly skipped → `present=false` →
fail-closed; this gap is specifically the partially-decodable line.)

**Fix:** decode `count` as `*int` and treat a `msg`-matching line whose `count` is
absent/non-integer as malformed → do not set `present=true` for it:

```go
type readsTotalLine struct {
	Msg   string `json:"msg"`
	Count *int   `json:"count"`
}
...
if raw.Msg == daemonReadsTotalMsg {
	if raw.Count == nil {
		continue // msg matched but count missing/garbled — not a valid proof line
	}
	count = *raw.Count
	present = true
}
```

### WR-04: Uncounted store reads bypass the `helix_semantic_store_reads_total` chokepoint

**File:** cross-module — `internal/semantic/store/effective_graph.go:61-74` (counted wrappers) vs. raw `s.db.QueryRowContext` / `t.tx.QueryRowContext` / `snap.tx.QueryContext` callers (e.g. `overlay.go:694,967`, `snapshot.go:558`)
**Issue:**
The zero-reads gate assumes every back-channel read funnels through
`s.queryContext` / `s.queryRowContext`, which increment the counter. Several
tx-scoped read paths call the raw `*sql.Tx` / `*sql.DB` methods directly and do
NOT increment the counter (`snapshot.go:558 snap.tx.QueryContext` is a read inside
a snapshot tx; `overlay.go:694,967 t.tx.QueryRowContext` are tx-scoped reads). If a
gated-but-not-fully-un-wired background path ever reaches one of these uncounted
reads on the no_semantic arm, `SemanticStoreReadsValue()` stays 0 and the gate
passes vacuously — the counter cannot witness a read it does not instrument.
(`overlay.go:129` is an `UPDATE ... RETURNING` RMW, correctly out of a *reads*
counter's scope.) This is largely pre-existing, but the phase elevates the counter
to a correctness gate, so the scope of what the gate can prove should be tightened
or documented.

**Fix:** route the tx-scoped *read* paths (`snapshot.go:558`, `overlay.go:694,967`)
through a counting wrapper, or add a comment at the gate (cell.go:602 /
semantic_gate.go) explicitly scoping the guarantee to reads through
`effective_graph.go`'s wrappers so a future maintainer does not over-trust the
counter.

## Info

### IN-01: `prePatchSnapshot` and the post-patch path both run `r.Setup`, double-running setup

**File:** `bench/runtime/cell.go:327-339` and `:644-646`
**Issue:** `prePatchSnapshot` calls `r.Setup` then `r.RunTests`; the post-patch
block (644-646) calls `r.Setup` again before `RunTests`. The comment acknowledges
this is "harmless for the hermetic Go runner," but a runner with side-effecting
Setup (module fetch, build-cache mutation) would run setup twice per cell.
**Fix:** document `LanguageRunner.Setup` as idempotent, or track a `setupDone`
flag in the cell.

### IN-02: `_ = rank.engine.SetVersionNotifier` evaluates a method value purely to "surface the keyword"

**File:** `internal/daemon/daemon.go:464`
**Issue:** Takes a method value and discards it solely so the identifier appears in
daemon wiring (per the inline comment). Dead code that conveys no behavior and can
mislead readers into thinking a notifier is wired here. Not introduced by this
phase, but adjacent to the gated region under review.
**Fix:** remove the line and reference the real channel-based notify path in a
comment instead of an executable no-op.

### IN-03: Duplicated JSONL reads-total parsing in test and production can drift

**File:** `bench/runtime/no_semantic_emission_integration_test.go:95-123` (`scanReadsTotalLine`) duplicates `bench/runtime/cell.go:95-127` (`scrapeSemanticReadsTotal`)
**Issue:** `scanReadsTotalLine` reimplements the production scraper's buffer size,
struct, last-line-wins, and skip-on-error logic. The local copy is justified (the
test asserts against the file the daemon wrote, not via the scraper), but the two
can drift — e.g. the WR-03 `*int` fix would need applying in both.
**Fix:** annotate both sites that they must stay in lockstep, or factor the parse
into a shared exported helper and have the test assert raw-file presence
separately.

---

_Reviewed: 2026-06-20_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
