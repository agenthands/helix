---
phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcem
plan: 02
subsystem: security
tags: [mcp, middleware, authz, profile, mode, permission-denied, lifo, sec-01]

# Dependency graph
requires:
  - phase: 91-01
    provides: code-generated verb surface (verbs always invocable, removing the tools/list-only filter as the sole gate)
  - phase: 66
    provides: GuardrailMiddleware (the near-verbatim template) + serr typed-error taxonomy
provides:
  - "ProfileEnforcementMiddleware: server-side tools/call authz against the session's resolved AllowedTools"
  - "InstallProfileEnforcementMiddleware: daemon wiring placed between Guardrail (14b.5) and LazyInit (14c)"
  - "Typed serr.PermissionDenied refusal that errors.Is round-trips to the CLI"
  - "LIFO-order + refuse-after-activate regression tests guarding the LazyInit-first invariant"
affects: [91-03, 91-04, profile, mode, cli verb hiding]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "tools/call enforcement middleware cloned from GuardrailMiddleware: method early-out → *CallToolRequest extract → session Snapshot() → membership scan → typed-error refusal"
    - "Authz enforced server-side in the daemon (authoritative session); CLI checks are UX-only and bypassable"
    - "nil session / nil whitelist == allow (ProfileFilterMiddleware nil-semantics parity)"

key-files:
  created:
    - internal/mcp/profile_enforce.go
    - internal/mcp/profile_enforce_test.go
  modified:
    - internal/daemon/daemon.go

key-decisions:
  - "Refuse via returned error (nil, serr.New(PermissionDenied).WithTool) NOT an IsError CallToolResult, so errors.Is(err, serr.ErrPermissionDenied) round-trips CLI-side (T-91-09)"
  - "Reused the single shared getSessionFn closure rather than creating a second session accessor (single source of truth)"
  - "Installed at step 14b.6 between Guardrail and LazyInit so LIFO exec keeps LazyInit first and ProfileEnforce before Guardrail"

patterns-established:
  - "Profile/mode authz middleware: early-out on non-tools/call, read AllowedTools snapshot, linear membership scan, typed PermissionDenied on miss"
  - "Order-recording middleware chain test asserts install/exec LIFO so a future reorder fails CI"

requirements-completed: [SEC-01]

# Metrics
duration: 3min
completed: 2026-06-21
status: complete
---

# Phase 91 Plan 02: tools/call Profile/Mode Enforcement Middleware Summary

**Server-side `tools/call` authz (ProfileEnforcementMiddleware) that refuses any tool outside the session's resolved AllowedTools with a typed `serr.PermissionDenied`, installed between Guardrail and LazyInit so LazyInit-first LIFO is preserved.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-06-21T14:16:38Z
- **Completed:** 2026-06-21T14:19:14Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Closed the SEC-01 privilege-escalation regression: once Plan 91-01 makes every verb always-invocable, a read-mode/ci-bot agent can no longer reach a destructive edit verb — the daemon refuses it server-side.
- `ProfileEnforcementMiddleware` is a near-verbatim clone of the verified `GuardrailMiddleware`: tools/call early-out, `*CallToolRequest` extract guard, session `Snapshot()` read, AllowedTools membership scan, typed-error refusal.
- Refusal returns `nil, serr.New(serr.PermissionDenied, …).WithTool(name)` (an error, not an IsError result) so `errors.Is(err, serr.ErrPermissionDenied)` round-trips to the CLI; the message names only the requested tool + profile/mode (no enumeration of other tools, T-91-08).
- Installed at daemon step 14b.6 between Guardrail (14b.5) and LazyInit (14c) reusing the shared `getSessionFn`; LIFO exec order = LazyInit → ProfileEnforce → Guardrail → … → handler.
- Added LIFO-order and refuse-after-activate regression tests so any future reorder that breaks the LazyInit-first / ProfileEnforce-before-Guardrail invariant fails CI.

## Task Commits

1. **Task 1 (RED): failing tests for ProfileEnforcementMiddleware** - `ba917a66` (test)
2. **Task 1 (GREEN): ProfileEnforcementMiddleware implementation** - `4972655a` (feat)
3. **Task 2: install wiring between Guardrail and LazyInit** - `25fc42ed` (feat)

_Note: the LIFO-order and refuse-after-activate tests for Task 2 were authored in the Task 1 RED commit (single test file) and went GREEN once the install wiring landed._

## Files Created/Modified
- `internal/mcp/profile_enforce.go` - `ProfileEnforcementMiddleware` + `InstallProfileEnforcementMiddleware`; tools/call AllowedTools authz with typed PermissionDenied refusal and the AFTER-Guardrail/BEFORE-LazyInit install-order doc.
- `internal/mcp/profile_enforce_test.go` - 9 tests: refuse (typed, next-not-called), allow, non-tools/call passthrough, nil session, nil whitelist, typed message+WithTool via errors.As, edit-mode allow, LIFO execution order, refuse-after-activate propagation.
- `internal/daemon/daemon.go` - step 14b.6 calls `InstallProfileEnforcementMiddleware(mcpServer.SDK(), getSessionFn, logger)` between the Guardrail (line 883) and LazyInit (line 955) installs, with an info log.

## Decisions Made
- Return the deny as an error (not an `IsError` CallToolResult) so the typed kind survives the wire and `errors.Is` works CLI-side — directly mitigates T-91-09 and matches the Guardrail Block path.
- Reuse the single `getSessionFn` closure already wired to Telemetry/ProfileFilter/Guardrail rather than creating a second session accessor (single source of truth, D-01 thread-safety invariant).
- `nil` session and `nil` AllowedTools both mean "allow" to match `ProfileFilterMiddleware` nil-as-all semantics, avoiding a fail-closed regression for unconfigured/admin sessions.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. RED failed to compile as expected (undefined `mcp.ProfileEnforcementMiddleware`), GREEN passed all 9 tests on first implementation; build/vet/proto/package suites all clean.

## User Setup Required
None - no external service configuration required.

## Threat Mitigations Verified
- **T-91-05** (EoP, read/ci-bot invokes destructive verb): mitigated — every tools/call is membership-checked against AllowedTools; out-of-set tools refused.
- **T-91-06** (mis-ordered enforcement): mitigated — installed between Guardrail and LazyInit; LIFO-order test asserts LazyInit-first and ProfileEnforce-before-Guardrail.
- **T-91-07** (CLI-only filter bypass): mitigated — authz is server-side in the daemon middleware.
- **T-91-09** (typed error returned as IsError breaks errors.Is): mitigated — returned as an error; round-trip asserted by `errors.As`/`errors.Is` tests.

## Verification Results
- `go test ./internal/mcp/ -run ProfileEnforce -count=1` — GREEN (all 9 behaviors incl. LIFO order).
- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./internal/mcp/... ./internal/daemon/...` — PASS (mcp 8.3s, daemon 9.8s).
- `git diff --exit-code api/proto/` — empty (zero-proto invariant holds).
- `grep -n InstallProfileEnforcementMiddleware internal/daemon/daemon.go` — line 910, between Guardrail (883) and LazyInit (955).

## Next Phase Readiness
- SEC-01 server-side authz is the load-bearing fix; Plan 91-04's CLI verb hiding (SEC-02) is now safely UX-only because the daemon is authoritative.
- No blockers for 91-03 / 91-04.

## Self-Check: PASSED
- FOUND: internal/mcp/profile_enforce.go
- FOUND: internal/mcp/profile_enforce_test.go
- FOUND commit: ba917a66 (RED test)
- FOUND commit: 4972655a (GREEN impl)
- FOUND commit: 25fc42ed (daemon wiring)

---
*Phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcem*
*Completed: 2026-06-21*
