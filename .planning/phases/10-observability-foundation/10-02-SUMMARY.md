---
phase: 10-observability-foundation
plan: 02
subsystem: daemon/observability
tags: [observability, admin-listener, pprof, healthz, readyz, stdlib-only]
requires:
  - 10-01 (ObservabilityConfig schema, --admin-addr CLI flag, ContextHandler)
provides:
  - "internal/daemon/telemetry.go: listenAdmin, validateAdminAddr, handleHealthz, handleReadyz, registerPprof, ready atomic"
  - "Daemon.Run admin errgroup goroutine (non-fatal per D-04)"
  - "package-level ready atomic flipped after setup; reset on g.Wait() return"
affects:
  - internal/daemon/daemon.go (Run sequence)
tech_stack:
  added: []
  patterns:
    - "Non-fatal errgroup wrapper for degraded-optional subsystems (D-04)"
    - "Loopback-only bind enforcement with v1.3 auth roadmap hint (Pitfall 6)"
    - "Graceful HTTP server shutdown via serveDone + ctx select + 5s Shutdown (mirror of listenHTTP)"
    - "Package-level atomic test hook (adminListenerAddr) for lifecycle probes"
key_files:
  created:
    - internal/daemon/telemetry.go
    - internal/daemon/telemetry_test.go
  modified:
    - internal/daemon/daemon.go
key_decisions:
  - "ready atomic lives at package level (var ready atomic.Uint32) not a Daemon struct field: matches testability of handler tests (resetReady helper) and avoids Daemon signature churn"
  - "adminListenerAddr atomic.Pointer[string] is documented as a TEST-ONLY hook; set after net.Listen, cleared via defer; enables race-free /healthz lifecycle probe without exposing Daemon internals"
  - "registerPprof extracted into a helper so conditional gating is a single-line branch — the import of net/http/pprof is always compiled in but DefaultServeMux side effect is harmless since daemon uses its own mux (RESEARCH Pattern 5)"
  - "ready.Store(0) placed after g.Wait() (not in d.shutdown()) so repeated Run invocations in the same process (tests) start from ready=0 deterministically"
metrics:
  duration: "~18 min"
  completed: "2026-04-08"
  tasks_completed: 2
  files_created: 2
  files_modified: 1
  lines_added: ~404
  go_mod_delta: 0
---

# Phase 10 Plan 02: Admin Listener Wiring Summary

One-liner: Loopback admin listener serving /healthz, /readyz, and gated /debug/pprof/* as a non-fatal errgroup goroutine in daemon.Run, with a post-setup ready atomic gating /readyz.

## What Shipped

**OBS-03 (loopback admin listener)** — `internal/daemon/telemetry.go:listenAdmin` binds to `cfg.Observability.AdminAddr`, no-op when empty. Refuses non-loopback via `validateAdminAddr`. Wired into `Daemon.Run` as an errgroup goroutine that logs bind failures and returns nil (D-04: admin errors never tear down the daemon).

**OBS-04 (health probes)** — `/healthz` always returns 200 `{"status":"ok"}` when the listener is up (D-13). `/readyz` returns 503 `{"status":"starting"}` until the package-level `ready` atomic is flipped to 1 at the end of `Daemon.Run` setup (after kernel/socket/http/admin goroutines spawned and `daemon started` log fires). Mitigates Pitfall #3 (operators hitting /readyz before LS warm-up).

**OBS-05 (gated pprof)** — `/debug/pprof/{,cmdline,profile,symbol,trace}` registered only when `cfg.Observability.EnablePprof == true`. Default false. Verified by `TestListenAdmin_PprofGated` (disabled → 404, enabled → 200). T-10-04 mitigated.

**Non-loopback refusal (Pitfall 6 contract)** — `validateAdminAddr` returns `admin addr must be loopback, got %q (v1.3 will add auth for non-loopback)` for `0.0.0.0`, LAN IPs, and DNS names. Error substring "v1.3" asserted in `TestValidateAdminAddr` and `TestListenAdmin_NonLoopbackRejected`.

**Graceful shutdown (Pitfall 2)** — Mirrors `listenHTTP` at `daemon.go:362-390`: `serveDone` channel + `select` on `ctx.Done()` + `server.Shutdown(5s ctx)`. Lifecycle test `TestListenAdmin_LoopbackLifecycle` cancels ctx and asserts the goroutine exits within 5s, with `adminListenerAddr` cleared.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | failing telemetry tests | 4d3830f0 | internal/daemon/telemetry_test.go |
| 1 (GREEN) | telemetry.go implementation | 6476b28f | internal/daemon/telemetry.go, telemetry_test.go (vet fix) |
| 2 | wire listenAdmin + ready atomic into Daemon.Run | 104eba76 | internal/daemon/daemon.go |

## Verification

- `go test ./internal/daemon/... -count=1 -timeout 60s` — PASS (all 8 new tests + existing daemon tests)
- `go vet ./internal/daemon/... ./internal/obs/...` — clean
- `go build ./cmd/serena` — succeeds
- `go.mod` / `go.sum` diff vs plan-start — zero new entries (stdlib-only constraint honored)

### Tests added (all passing)

1. `TestValidateAdminAddr` — 9 table cases covering loopback acceptance and non-loopback refusal with "v1.3" error substring
2. `TestHandleHealthz` — bare ResponseRecorder, 200 + application/json + {"status":"ok"}
3. `TestHandleReadyz_NotReady` — ready=0 → 503 "starting"
4. `TestHandleReadyz_Ready` — ready=1 → 200 "ready"
5. `TestListenAdmin_Disabled` — empty AdminAddr returns nil immediately
6. `TestListenAdmin_NonLoopbackRejected` — 0.0.0.0:0 returns error containing "v1.3"
7. `TestListenAdmin_LoopbackLifecycle` — binds 127.0.0.1:0, probes /healthz via real HTTP, cancels ctx, asserts clean exit within 5s, asserts adminListenerAddr cleared
8. `TestListenAdmin_PprofGated` — two sequential listeners, disabled → 404, enabled → 200

## Deviations from Plan

Minimal deviations — plan executed essentially as written.

**[Rule 3 - Blocking] Vet fix after GREEN**
- Found during: Task 1 GREEN verification
- Issue: `go vet` flagged `var _ atomic.Pointer[string] = adminListenerAddr` as copying a noCopy lock
- Fix: Removed the unused type-assertion sanity check at the bottom of telemetry_test.go (dead line — the tests already exercise the hook) and dropped the `sync/atomic` import from the test file
- Commit: folded into 6476b28f (Task 1 GREEN)

**Minor plan departure** — plan text suggested "Add a matching `ready.Store(0)` in d.shutdown() or immediately after g.Wait() and before d.shutdown()". Chose the latter because `d.shutdown()` already exists in `shutdown.go` as a method with no package-level coupling; placing the reset in `daemon.Run` immediately after `g.Wait()` keeps the observability state lifecycle local to the Run function for readability. This matches plan alternative 2 explicitly.

## Threat Register Coverage

All Phase 10 Plan 02 STRIDE threats mitigated:

- **T-10-04** (pprof info disclosure) — gated on EnablePprof; default false; loopback-only defense-in-depth
- **T-10-05** (non-loopback spoofing) — validateAdminAddr refuses with v1.3 hint
- **T-10-06** (bind failure DoS) — non-fatal errgroup wrapper; errors logged and swallowed
- **T-10-07** (shutdown hang) — 5s Shutdown timeout; integration test asserts sub-5s exit
- **T-10-08** (/readyz lies) — ready atomic flipped only after setup complete
- **T-10-09** (healthz body leakage) — literal `{"status":"ok"}`, no metadata (accepted)

## Success Criteria

- [x] OBS-03 delivered: loopback admin listener, default disabled, configurable port, non-fatal bind
- [x] OBS-04 delivered: /healthz always 200 when listener up, /readyz gated on ready atomic
- [x] OBS-05 delivered: /debug/pprof/* conditionally registered on EnablePprof only
- [x] Non-loopback bind attempts produce clear v1.3 error
- [x] Admin listener goroutine survives bind failure (D-04)
- [x] Admin listener drains on ctx cancel within 5s (Pitfall 2)
- [x] ready atomic flipped after daemon setup complete (Pitfall 3, Q3)
- [x] Existing daemon tests unchanged and still passing
- [x] Zero new go.mod entries (stdlib-only)

## Self-Check: PASSED

Files verified:
- FOUND: internal/daemon/telemetry.go (141 lines)
- FOUND: internal/daemon/telemetry_test.go (242 lines)
- FOUND: internal/daemon/daemon.go (modified, +21 lines)

Commits verified:
- FOUND: 4d3830f0 (test RED)
- FOUND: 6476b28f (feat GREEN)
- FOUND: 104eba76 (feat wire-in)
