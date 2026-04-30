---
phase: 53-obs-metrics-gaps
plan: 05
subsystem: observability
tags: [observability, daemon, session, http, stdio, metrics, helix]
requires:
  - 53-01 (SessionLifecycleInc helper, helix_session_lifecycle_total vector)
provides:
  - forwarder stdio session-lifecycle emission (started|ended|error, stdio)
  - http session-lifecycle middleware (started|ended|error, http) — best-effort ended
  - sessionRunner test seam on forwarderServiceHandler for unit-testing StreamMCP without an MCP runtime
affects:
  - Plan 53-06 (USAGE.md): must document the best-effort `ended` semantic for transport=http
tech-stack:
  added:
    - net/http handler middleware pattern (FIRST in this codebase — no in-tree precedent)
  patterns:
    - drop-unknown closed-enum discipline at emission (inherited from RenameStrategyInc)
    - sync.Map.LoadOrStore for concurrent first-seen idempotency
    - statusRecorder with effectiveStatus() default-to-200 accessor
    - function-pointer test seam (sessionRunner) on a handler struct to bypass heavyweight deps in unit tests
key-files:
  created:
    - internal/daemon/http_session_middleware.go (89 lines)
    - internal/daemon/forwarder_test.go (240 lines, NEW file — no existing forwarder test in daemon pkg)
    - internal/daemon/http_session_middleware_test.go (189 lines)
  modified:
    - internal/daemon/daemon.go (+69, -11): metrics + serveSession fields on forwarderServiceHandler; StreamMCP rewritten with 3 lifecycle emission sites; defaultSessionRunner extracted; listenHTTP install site wraps HTTPHandler with httpSessionMiddleware
decisions:
  - "Daemon obs Provider field name on the daemon struct is `obs *obs.Provider` (line 106); accessed in handler construction as `d.obs.Metrics()`"
  - "StreamMCP refactored to use a sessionRunner function-pointer seam — sessionRunner is the production type, defaultSessionRunner is the production builder, and forwarderServiceHandler.serveSession is the test-only override. nil seam falls back to defaultSessionRunner — production code path is unchanged"
  - "The plan's task 5.1 sub-test `started_error_on_connect_fail` and `started_error_on_session_wait_fail` were collapsed under the seam — both surface as runner err != nil and emit (error, stdio). The third SessionLifecycleInc call site count (3 = started + ended + error) matches the plan's acceptance criterion"
  - "`ended` emission for transport=http fires BEFORE the inner handler runs, so a panic mid-request still leaves the metric consistent with the client's expressed intent (DELETE /mcp is a clean termination signal per MCP spec)"
metrics:
  duration: "~22 minutes"
  completed: "2026-04-30"
  tasks: 2
  files_created: 3
  files_modified: 1
  lines_added: 587
---

# Phase 53 Plan 05: Session Lifecycle Emission Summary

**One-liner:** Wired `helix_session_lifecycle_total{phase, transport}` for both transports — direct calls in `forwarderServiceHandler.StreamMCP` for `transport=stdio`, and a NEW `internal/daemon/http_session_middleware.go` http.Handler wrapper (Q-1 Option 2; first http.Handler middleware in this codebase) for `transport=http`.

## What Landed

### Task 5.1 — stdio forwarder lifecycle

**`internal/daemon/daemon.go`:**

- Added `metrics *obs.Metrics` field on `forwarderServiceHandler` (Phase 53 D-17 — direct call discipline because daemon already imports `internal/obs`).
- Added `serveSession sessionRunner` field as a test seam.
- Wired `metrics: d.obs.Metrics()` at the handler construction site in `listenSocket` (line 552). `d.obs.Metrics()` is never nil per the Noop-default invariant.
- Refactored `StreamMCP` body to emit at 3 boundaries:
  - `SessionLifecycleInc("started", "stdio")` AFTER `firstMsg` is received (we know we have a session).
  - `SessionLifecycleInc("error", "stdio")` if the runner returns non-nil (covers both Connect failure and Wait failure — collapsed via the seam).
  - `SessionLifecycleInc("ended", "stdio")` on clean runner return.
  - **No emission** if `stream.Recv()` fails before `firstMsg` (regression guard against count drift).
- Extracted the MCP-runtime portion into `defaultSessionRunner(mcpServer)` so unit tests can swap it without spinning up a full MCP server. Production code path is unchanged when the seam is nil.

**`internal/daemon/forwarder_test.go` (NEW):**

- `TestForwarderHandler_SessionLifecycle` with 4 sub-tests:
  - `started_ended_on_clean_exit` — runner returns nil → assert (started, ended).
  - `started_error_on_connect_fail` — runner returns connect error → assert (started, error).
  - `started_error_on_session_wait_fail` — runner returns wait error → assert (started, error).
  - `no_emit_when_recv_fails_before_firstmsg` — Recv() fails first → assert NO emissions, runner not invoked.
- `fakeForwarderStream` is a minimal `serenav1.ForwarderService_StreamMCPServer` (= `grpc.BidiStreamingServer[MCPMessage, MCPMessage]`) — only `Recv()` and `Context()` are exercised; the rest of the embedded `grpc.ServerStream` interface is no-op.
- `gatherSessionLifecycle(t, m)` is a shared helper that walks `obs.Metrics.Registry().Gather()` and returns a map of `phase|transport` → count, keyed for both stdio and http tests (reused in Task 5.2's tests).

### Task 5.2 — http session middleware

**`internal/daemon/http_session_middleware.go` (NEW):**

- `httpSessionMiddleware(next http.Handler, metrics *obs.Metrics) http.Handler` per Q-1 Option 2.
- `sync.Map` tracks first-seen `Mcp-Session-Id`. `LoadOrStore(id, struct{}{})` returns `loaded=false` only for the winning goroutine when N concurrent requests share the same new id, so `started` is emitted exactly once.
- `DELETE /mcp` with a session id emits `(ended, http)` BEFORE the inner handler runs. The id is then removed from the seen set so re-use after a DELETE counts as a new session.
- `statusRecorder` wraps the `http.ResponseWriter` to capture `WriteHeader(code)`. `effectiveStatus()` defaults to 200 when the inner handler never calls WriteHeader (mirrors `net/http`'s implicit-200-on-Write behavior); the `>= 500` check uses this accessor so a never-WriteHeader handler is correctly classified.
- Best-effort `ended` semantic is documented in the file's package doc and in the inline DELETE-handler comment.

**`internal/daemon/daemon.go`:**

- One-line change at the install site: `mux.Handle("/mcp", httpSessionMiddleware(d.mcpServer.HTTPHandler(), d.obs.Metrics()))` in `listenHTTP`.

**`internal/daemon/http_session_middleware_test.go` (NEW):**

- `TestHTTPSessionMiddleware` with all 6 sub-tests required by the plan:
  - `first_seen_session_id_emits_started` — second request with same id does NOT re-emit.
  - `delete_emits_ended` — POST then DELETE → exactly one `ended`.
  - `5xx_response_emits_error` — inner handler responds 500 → both `started` and `error` fire.
  - `no_session_id_no_emit` — POST and DELETE without `Mcp-Session-Id` header → zero emissions.
  - `concurrent_first_seen_idempotent` — 100 goroutines, same new id → exactly 1 `started` emission (sync.Map.LoadOrStore semantics held under load).
  - `post_then_delete_then_post_re_emits_started` — DELETE forgets the id; subsequent POST with same id counts as a new session (`started=2`, `ended=1`).

## Best-effort `ended` for transport=http (deferred to Plan 06)

The MCP SDK v1.5.0 (go.mod:14) does not expose a per-session lifecycle hook; the SDK owns session timeout and cleanup internally. This wrapper observes only the externally-visible signals — `DELETE /mcp` from the client and 5xx responses from the upstream. Sessions that disappear due to server-side timeout will NOT register an `ended` event. Plan 53-06 will document this caveat in `USAGE.md` for operators reading scraped metrics.

## Key Decisions Made

1. **`d.obs` is the field name** for the obs Provider on the Daemon struct (line 106 of daemon.go). `observability` is the local var name in the constructor — this plan accesses metrics via `d.obs.Metrics()` from within the handler-installation code paths.

2. **`forwarder_test.go` was a NEW file.** No prior test in the daemon package exercised StreamMCP — the closest existing test (`internal/forwarder/forwarder_test.go::fakeStream`) is client-side and doesn't satisfy the server-side gRPC interface. The new file rolls its own minimal `fakeForwarderStream` satisfying `grpc.BidiStreamingServer[MCPMessage, MCPMessage]`.

3. **No-emit-on-failed-recv guard CONFIRMED.** Sub-test `no_emit_when_recv_fails_before_firstmsg` proves that when `stream.Recv()` returns an error before yielding the first message, `forwarderServiceHandler.StreamMCP` returns BEFORE any `SessionLifecycleInc` call AND before the `serveSession` runner is invoked. This is the regression guard against count drift (inflated `started` counter without matching `ended`/`error`).

4. **Concurrent-first-seen test PASSED under N=100 goroutines.** The metric showed exactly 1 `started|http` emission after 100 concurrent requests with the same `Mcp-Session-Id` value. `sync.Map.LoadOrStore` guarantees that only the winning goroutine sees `loaded=false`.

5. **sessionRunner seam introduced** to keep tests free of MCP runtime dependencies. Production-time `serveSession` is nil and the handler falls back to `defaultSessionRunner(mcpServer)`, which is the verbatim original `Connect → session.Wait` logic. This preserves exact production behavior; the seam is observable only by tests that explicitly set the field.

6. **Three SessionLifecycleInc sites in StreamMCP, not four** — the plan's "started + error-on-connect + ended/error-on-wait" count maps to (started, error, ended) under the seam refactor. `error` collapses both Connect-fail and Wait-fail because the runner surfaces them the same way. Acceptance criterion `grep -c 'h.metrics.SessionLifecycleInc' = 3` passes.

## Files Modified / Created

| File | Lines | Change |
|---|---|---|
| `internal/daemon/daemon.go` | +69 / -11 | added metrics+serveSession fields, refactored StreamMCP, defaultSessionRunner, wrapped listenHTTP |
| `internal/daemon/http_session_middleware.go` | +89 (NEW) | the wrapper itself |
| `internal/daemon/forwarder_test.go` | +240 (NEW) | TestForwarderHandler_SessionLifecycle + helpers |
| `internal/daemon/http_session_middleware_test.go` | +189 (NEW) | TestHTTPSessionMiddleware (6 sub-tests) |
| **Total** | **+587 / -11** | 1 modified, 3 created |

## Commits

| Task | Type | Hash | Message |
|---|---|---|---|
| 5.1 | test | `bc372931` | add failing forwarder stdio session lifecycle tests (RED) |
| 5.1 | feat | `5f37a46c` | emit forwarder stdio session lifecycle (Phase 53 D-08) (GREEN) |
| 5.2 | test | `a5b00432` | add failing http session middleware tests (RED) |
| 5.2 | feat | `d1ccc1c6` | wrap MCP HTTP handler with session-lifecycle middleware (GREEN) |

## Verification

```text
$ go vet ./internal/daemon/...
ok

$ go test ./internal/daemon/... ./internal/obs/...
ok      github.com/agenthands/helix/internal/daemon   1.518s
ok      github.com/agenthands/helix/internal/obs      10.621s

$ go build ./cmd/helix
ok

$ go test ./internal/daemon/... -run TestForwarderHandler_SessionLifecycle -v
=== RUN   TestForwarderHandler_SessionLifecycle
=== RUN   TestForwarderHandler_SessionLifecycle/started_ended_on_clean_exit
=== RUN   TestForwarderHandler_SessionLifecycle/started_error_on_connect_fail
=== RUN   TestForwarderHandler_SessionLifecycle/started_error_on_session_wait_fail
=== RUN   TestForwarderHandler_SessionLifecycle/no_emit_when_recv_fails_before_firstmsg
--- PASS: TestForwarderHandler_SessionLifecycle (0.00s)
PASS

$ go test ./internal/daemon/... -run TestHTTPSessionMiddleware -v
=== RUN   TestHTTPSessionMiddleware
=== RUN   TestHTTPSessionMiddleware/first_seen_session_id_emits_started
=== RUN   TestHTTPSessionMiddleware/delete_emits_ended
=== RUN   TestHTTPSessionMiddleware/5xx_response_emits_error
=== RUN   TestHTTPSessionMiddleware/no_session_id_no_emit
=== RUN   TestHTTPSessionMiddleware/concurrent_first_seen_idempotent
=== RUN   TestHTTPSessionMiddleware/post_then_delete_then_post_re_emits_started
--- PASS: TestHTTPSessionMiddleware (0.00s)
PASS
```

All success criteria met:

- [x] forwarderServiceHandler emits 3 SessionLifecycleInc call sites (started + error + ended)
- [x] httpSessionMiddleware emits started (first-seen), ended (DELETE), error (5xx)
- [x] listenHTTP install site wraps mcpServer.HTTPHandler() with httpSessionMiddleware
- [x] Concurrent-first-seen idempotency proven (N=100 goroutines, started=1)
- [x] Best-effort `ended` semantic documented in code (USAGE.md docs deferred to Plan 06)
- [x] Regression guard verified: no emission on failed Recv() before firstMsg
- [x] `go vet ./internal/daemon/...` ok
- [x] `go test ./internal/daemon/... ./internal/obs/...` ok
- [x] `go build ./cmd/helix` ok

## Deviations from Plan

### Auto-fixed Issues

None — plan executed as written, with two consciously-tracked refinements that improved testability without changing the contract:

1. **[Refinement] Introduced `sessionRunner` function-pointer seam** on `forwarderServiceHandler`. The plan's `<action>` block in Task 5.1 listed inline `Connect()` and `session.Wait()` calls verbatim from the existing code. Implementing the 4 required sub-tests (especially `started_error_on_connect_fail` and `started_error_on_session_wait_fail`) without spinning up a full MCP runtime required a test seam. The seam preserves exact production behavior when the field is nil (the production path falls back to `defaultSessionRunner(mcpServer)`, which is the verbatim original logic). This is a Rule-3 fix (blocking issue: tests cannot be written without it) but was applied as a transparent refactor that the plan's success criteria still validates exactly (3 call sites, all acceptance greps pass).

2. **[Process note] RED→GREEN per task** committed as separate atomic commits per executor protocol, even though the test-then-impl idiom would benefit from a single combined commit. Net effect on the merged feature branch is identical; the per-task atomic commits are cleaner for `git bisect`.

## Threat surface scan

No new threat surface introduced beyond what the plan's `<threat_model>` covers.

- **T-53-13** (sync.Map memory growth from attacker-supplied session ids) — mitigated: DELETE /mcp clears the entry; the SDK's own session lifecycle bounds the live-session population. v1.10 hardening (max session count + LRU eviction) is documented inline in `httpSessionMiddleware` doc comment if scrape data shows growth.
- **T-53-14** (cardinality DoS on `helix_session_lifecycle_total`) — mitigated: closed enums (3 phases × 2 transports = 6 series max). Plan 53-01's `TestMetrics_CardinalityBounds_SessionLifecycle` enforces `≤ 6` at build time.
- **T-53-15** (info disclosure via session_id labels) — accept: `session_id` is NOT a label on the metric (only `phase`, `transport`). The id is used for the in-memory seen-set only.
- **T-53-16** (tampering with sync.Map state) — accept: in-process state, no persistence; daemon restart resets it.

## Self-Check: PASSED

- internal/daemon/http_session_middleware.go exists at 89 lines
- internal/daemon/forwarder_test.go exists at 240 lines
- internal/daemon/http_session_middleware_test.go exists at 189 lines
- internal/daemon/daemon.go modified (3 SessionLifecycleInc sites + middleware wrap)
- Commit bc372931 exists: test(53-05) RED for forwarder
- Commit 5f37a46c exists: feat(53-05) GREEN for forwarder
- Commit a5b00432 exists: test(53-05) RED for http middleware
- Commit d1ccc1c6 exists: feat(53-05) GREEN for http middleware
- All 4 forwarder sub-tests pass
- All 6 http middleware sub-tests pass
- go vet ./internal/daemon/... ok
- go test ./internal/daemon/... ./internal/obs/... ok
- go build ./cmd/helix ok
- 0 known stubs in modified files
