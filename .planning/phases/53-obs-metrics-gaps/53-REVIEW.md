---
phase: 53-obs-metrics-gaps
reviewed: 2026-04-30T23:59:00Z
depth: standard
files_reviewed: 26
files_reviewed_list:
  - internal/daemon/daemon.go
  - internal/daemon/forwarder_test.go
  - internal/daemon/http_session_middleware_test.go
  - internal/daemon/http_session_middleware.go
  - internal/daemon/wiring_test.go
  - internal/fuzzy/fuzzy_test.go
  - internal/fuzzy/match.go
  - internal/kernel/edit/outcome_emission_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/outcome_emission_test.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/lspool/metrics_test.go
  - internal/kernel/lspool/metrics.go
  - internal/kernel/lspool/pool.go
  - internal/mcp/middleware.go
  - internal/mcp/record_edit_outcome_test.go
  - internal/obs/metrics_labels_test.go
  - internal/obs/metrics_test.go
  - internal/obs/metrics.go
  - internal/repomap/cache.go
  - internal/repomap/metrics_test.go
  - internal/repomap/metrics.go
  - internal/repomap/render.go
  - internal/skill/repomap/metrics_emit_test.go
  - internal/skill/repomap/skill.go
  - USAGE.md
findings:
  critical: 1
  warning: 6
  info: 4
  total: 11
status: issues_found
---

# Phase 53: Code Review Report

**Reviewed:** 2026-04-30T23:59:00Z
**Depth:** standard
**Files Reviewed:** 26
**Status:** issues_found

## Summary

Phase 53 introduces 5 new `helix_*` Prometheus metric families with closed-enum drop-unknown discipline at the `*obs.Metrics` helper layer, plus matching `MetricsSink` interfaces in `lspool` and `repomap` and an `atomic.Pointer`-based recorder pattern in `internal/mcp` for the new edit-outcome family. Cardinality is verifiably bounded (168 series for `helix_edit_outcome_total`, 6 for `helix_session_lifecycle_total`) and the closed-enum carve-outs are wired through `metrics_labels_test.go`.

The instrumentation discipline is generally strong — closed enums are enforced at emission, drop-unknown is exhaustively tested, and the atomic-pointer sink pattern preserves the noop-default invariant. However, adversarial review surfaced one BLOCKER (a data race on `TagCache.metrics` between `SetMetricsSink` and `GetOrExtract`'s lookup-hit emission) and several WARNINGs covering count-drift cases in the HTTP session middleware (orphan DELETE, race window after Recv before emission), an outcome misclassification on the `replace_in_file` fuzzy-fallback file-read error path, an outdated `outcome="error"` example in USAGE.md that doesn't match any enum value, and the Q-3 documented-but-still-present under-classification of validation errors as `internal`.

The fuzzy sentinel additions (`ErrNoMatch`, `ErrAmbiguous`) are correctly threaded through `serr.Wrap` and preserve the `errors.Is(err, serr.ErrInvalidArgs)` chain. The `sessionRunner` test seam in `daemon.go` is transparent in production — the nil-fallback at handler invocation time is correct.

## Critical Issues

### CR-01: Data race on TagCache.metrics between SetMetricsSink and GetOrExtract hit emission

**File:** `internal/repomap/cache.go:101-110`

**Issue:** `GetOrExtract` reads `c.metrics` at line 109 (`c.metrics.RepoMapLookup(...)`) AFTER releasing `c.mu` at line 104. `SetMetricsSink` (cache.go:74-77) writes `c.metrics` while holding `c.mu`. There is no other synchronization on the field. Concurrent callers — for example, the daemon post-init wiring at `daemon.go:333-335` racing with a `GetOrExtract` from `walkAndExtract` — can produce a Go data race detectable under `-race`.

The race is real because:
1. `daemon.go:333` calls `cache.SetMetricsSink(observability.Metrics())` post-init.
2. `walkAndExtract` (skill.go:412) calls `cache.GetOrExtract` concurrently as soon as the first tool call invokes `ensureCache`.
3. `GetOrExtract` reads `c.metrics` outside the mutex.

The miss-branch read at line 121 has the same problem — `c.mu` was released at line 114, then `c.metrics.RepoMapLookup` is called at line 121 without re-acquiring.

**Fix:**

```go
// Capture the sink under the lock so the field read is synchronized.
c.mu.Lock()
sink := c.metrics
var cachedMtime int64
err = c.db.QueryRow(
    "SELECT mtime_ns FROM file_tags WHERE file_path = ? LIMIT 1",
    filePath,
).Scan(&cachedMtime)

if err == nil && cachedMtime == mtime {
    tags, loadErr := c.loadTags(filePath)
    c.mu.Unlock()
    if loadErr != nil {
        return nil, fmt.Errorf("loading cached tags: %w", loadErr)
    }
    sink.RepoMapLookup(LangFromExt(filePath), LookupHit)
    return tags, nil
}
c.mu.Unlock()
sink.RepoMapLookup(LangFromExt(filePath), LookupMiss)
// ...
```

Alternatively, replace `c.metrics MetricsSink` with `atomic.Pointer[MetricsSink]` so reads are lock-free and writes don't need the cache mutex (mirrors the `editOutcomeSink` pattern in `internal/mcp/middleware.go:56`). The skill-side `s.metricsSink()` helper at `skill/repomap/skill.go:149-157` ALREADY captures-then-uses under the lock; `cache.go` is the outlier.

## Warnings

### WR-01: Orphan DELETE emits ended without prior started

**File:** `internal/daemon/http_session_middleware.go:43-51`

**Issue:** When a client sends `DELETE /mcp` with an `Mcp-Session-Id` header that was never seen by this middleware instance (e.g., the daemon restarted, the session was issued by a different process, or a malicious client spams DELETE for arbitrary IDs), the middleware emits `helix_session_lifecycle_total{phase="ended", transport="http"}` even though no matching `started` was emitted. The `if r.Method == http.MethodDelete && sessionID != ""` check does NOT consult the `seen` map. This produces count drift that operators are explicitly trying to track via the started−ended gap (USAGE.md L697).

The `started` emission at line 39 correctly uses `LoadOrStore` to gate one emission per first-seen ID. The DELETE path should mirror that: only emit `ended` if the ID was previously stored (i.e., we actually saw a `started` for it).

Compare with the regression-guard test `no_session_id_no_emit` at http_session_middleware_test.go:97-120 — it covers no-header DELETE, but NOT unseen-id DELETE. The contract gap is not tested.

**Fix:**

```go
if r.Method == http.MethodDelete && sessionID != "" {
    if _, loaded := seen.LoadAndDelete(sessionID); loaded {
        // Only emit `ended` for sessions we actually saw start.
        metrics.SessionLifecycleInc("ended", "http")
    }
}
```

`LoadAndDelete` (Go 1.20+) collapses the existence check and the delete into one atomic op. Add a regression test that POSTs id="seen", DELETEs id="unseen", and asserts `ended|http == 0`.

### WR-02: replace_in_file fuzzy-fallback ReadFile error reports outcome=success

**File:** `internal/kernel/fileops/tools.go:395-402`

**Issue:** On the fuzzy-fallback path, when `ReadFile(root, args.Path)` fails (e.g., file vanished between `ReplaceInFile`'s 0-hit literal pass and this read, permission flipped, transient EIO), the handler returns `textResult(...)` with the message "0 replacement(s) made in %s" and the deferred `RecordEditOutcome` fires with `outcome="success"`. There was a real error and zero replacements were made — observability should reflect that.

The comment at line 397-401 calls this "existing behavior", but Phase 53 instrumentation is the first time this path emits a metric. Reporting `success` for a silently-swallowed I/O error muddies the success rate.

**Fix:**

```go
if count == 0 && !args.IsRegex {
    content, readErr := ReadFile(root, args.Path)
    if readErr != nil {
        outcome = "internal"
        return textResult(fmt.Sprintf("0 replacement(s) made in %s", args.Path)), nil, nil
    }
    // ...
}
```

If preserving the textResult shape is non-negotiable (caller backwards-compat), at minimum classify the outcome as `internal` so the gauge doesn't lie. Equivalently: return `errorResult(readErr.Error())` and let the existing `outcome = "internal"` fallthrough trigger.

### WR-03: USAGE.md PromQL example references nonexistent outcome value "error"

**File:** `USAGE.md:670-673`

**Issue:** The example query uses `helix_tool_calls_total{outcome="error"}` but the closed enum (middleware.go:147-155) is `{success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal}`. There is no `error` value. A user copy-pasting this query gets zero results.

```promql
# Error rate per tool (last 5 minutes)
rate(helix_tool_calls_total{outcome="error"}[5m])
```

This is a longstanding bug surfaced by Phase 53's expansion of the metrics surface (the documentation is now under closer scrutiny). Three of the seven enum values are TODO-gated for v1.3 (invalid_args, not_found, ls_crash) per the same file's comment at line 149-152, so the practical "error" set is `{circuit_open, timeout, internal}`.

**Fix:**

```promql
# Error rate per tool (last 5 minutes; unionize the failure outcomes).
rate(helix_tool_calls_total{outcome=~"timeout|circuit_open|internal"}[5m])
  / rate(helix_tool_calls_total[5m])
```

Or, equivalently, the inverse:

```promql
1 - rate(helix_tool_calls_total{outcome="success"}[5m])
  / rate(helix_tool_calls_total[5m])
```

### WR-04: 5xx response on DELETE double-counts the request

**File:** `internal/daemon/http_session_middleware.go:43-56`

**Issue:** A DELETE that triggers a 5xx upstream response emits BOTH `ended` (line 49) AND `error` (line 55). The `ended` fires unconditionally on DELETE before `next.ServeHTTP`, so even when the inner handler fails with 500 the wrapper has already counted it as a clean termination. From an operator's view, the gauge for "sessions cleanly terminated" includes failed DELETEs.

The CAVEAT comment at line 44-48 justifies the pre-handler emission as protection against a panic in the inner handler, but the inverse is: a non-panic 5xx still emits ended.

**Fix:** Consider gating the ended emission on the response status:

```go
isDelete := r.Method == http.MethodDelete && sessionID != ""
rw := &statusRecorder{ResponseWriter: w}
next.ServeHTTP(rw, r)
status := rw.effectiveStatus()
if isDelete && status < 500 {
    if _, loaded := seen.LoadAndDelete(sessionID); loaded {
        metrics.SessionLifecycleInc("ended", "http")
    }
}
if status >= 500 {
    metrics.SessionLifecycleInc("error", "http")
}
```

This also subsumes WR-01. If the panic-protection semantics matter more than the double-count (operator decision), document explicitly that `ended + error` can both fire on the same DELETE — currently neither USAGE.md nor the test suite captures the overlap.

### WR-05: Concurrent SetEditOutcomeSink is racy on read side without sync

**File:** `internal/mcp/middleware.go:56,79-85,396-398`

**Issue:** `editOutcomeSink atomic.Pointer[func(...)]` is correct for the hot path. However, `SetEditOutcomeSinkForTest` (line 396) is exposed for cross-package tests, and `TestRecordEditOutcome/noop_without_sink` at record_edit_outcome_test.go:30 calls `mcp.SetEditOutcomeSinkForTest(nil)`. Storing `nil` via `setEditOutcomeSink(nil)` at middleware.go:62-64 stores a non-nil `*func` pointing to a nil function; `RecordEditOutcome` at line 81 correctly handles this (`if p == nil || *p == nil`).

But `setEditOutcomeSink(fn)` at line 62 stores `&fn` — taking the address of the parameter `fn`. The compiler may or may not alias this with the caller's address. The cross-package test calls `mcp.SetEditOutcomeSinkForTest(nil)` — the parameter `fn` becomes a typed nil `func`, then `setEditOutcomeSink` stores `&fn` (a non-nil pointer to a nil func). The `*p == nil` check on read (line 81) handles this correctly. So `RecordEditOutcome` is safe.

The actual concern is that consecutive tests can leak sinks across test boundaries — `noop_without_sink` runs first, then `roundtrip_through_installed_sink` calls `SetEditOutcomeSinkForTest(...)`, then `install_middleware_routes_to_obs_metrics` resets it again. If a future test runs in `t.Parallel()`, the package-level pointer leaks across goroutines. Currently no test uses `t.Parallel()` here, but the global state is a footgun.

**Fix:** Add a comment on `SetEditOutcomeSinkForTest` explicitly forbidding `t.Parallel()` use, OR add a `t.Cleanup(func() { setEditOutcomeSink(nil) })` to each sub-test that mutates the sink. Same pattern applies to `setRenameStrategySink` (Phase 47). Mention this in the recorder docstring at line 57-65.

### WR-06: Q-3 under-classification of validation errors as "internal" misleads operators

**File:** `internal/kernel/edit/tools.go:65-71`, `internal/mcp/middleware.go:181-182`

**Issue:** The Q-3 resolution buckets `serr.InvalidArgs` results that aren't fuzzy ambiguity / no-match (e.g., "missing required field: path", "missing required field: new_body") as `outcome="internal"`. This is preserved across all 7 instrumented handlers. The doc comment explicitly notes this is "scheduled for v1.3 typed-error work."

The pragmatic concern: every missing-field error from agent calls (which DO happen — agents ship malformed args routinely) increments the same counter as genuine internal kernel errors. An on-call operator looking at `rate(helix_edit_outcome_total{outcome="internal"})` cannot distinguish "agent sent garbage" from "kernel imploded." This is a production-relevant signal degradation.

The plan acknowledges this is intentional. But it's worth flagging as a known gap so the v1.3 work tracks it explicitly: when the typed-error layer lands, every `outcome="internal"` site needs review for correct re-classification (likely to a new `invalid_args` bucket — which would then need to be added to the closed enum, breaking the 168-cardinality bound that `TestMetrics_CardinalityBounds_EditOutcome` enforces). Adding a 7th outcome moves the bound to 7×7×4=196.

**Fix:** Add a TODO comment at edit/tools.go:65-71 referencing the v1.3 typed-error plan ID (when one exists) and the cardinality-bound test that must be updated:

```go
// TODO(v1.3): When typed validation errors land, route missing-field /
// invalid-arg errors to a new outcome="invalid_args" bucket. Updating this
// requires:
//   1. Add invalid_args to editOutcomeEnum (middleware.go:192)
//   2. Update obs/metrics.go:295 EditOutcomeInc allowlist
//   3. Update TestMetrics_CardinalityBounds_EditOutcome bound from 168 to 196
//   4. Update USAGE.md table row for helix_edit_outcome_total
```

This makes the v1.3 migration mechanical instead of an archaeological dig.

## Info

### IN-01: stale carve-out comment claim "internal/obs never imported by lspool/repomap"

**File:** `internal/kernel/lspool/metrics.go:6`, `internal/repomap/metrics.go:5-7`

**Issue:** Both packages document "never imports internal/obs" as a structural rule. Confirmed via the file imports — neither package imports `internal/obs` directly. The compile-time `var _ MetricsSink = (*obs.Metrics)(nil)` lives in `internal/daemon/wiring_test.go:18,27` where the cycle is broken by daemon owning both. This is the correct pattern.

The note is accurate. Flagging only because the same comment also says "ad-hoc by *obs.Metrics" — which understates how strongly this is enforced (compile-time assertion in wiring_test.go, runtime smoke in TestObsMetricsIsRepoMapSink). Consider strengthening the wording to "compile-time-checked" so future contributors don't loosen the contract.

### IN-02: HTTP session middleware unbounded sync.Map (T-53-13)

**File:** `internal/daemon/http_session_middleware.go:35`

**Issue:** Documented at line 30-33: an attacker-supplied stream of distinct `Mcp-Session-Id` values can grow `seen` without bound. The doc punts to "a v1.10 follow-up if scrape data shows growth." Worth tracking in the issue tracker rather than only in code comments — code-comment TODOs are easy to lose.

If a `LoadAndDelete` fix lands for WR-01, the eviction surface is at least the DELETE path: legitimate clients that signal session termination shrink the map. But malicious clients that never DELETE still grow it.

**Fix:** Add an explicit issue reference (e.g., `// see issue #NNN`) in the comment. Optional v1.5 hardening: bound the map to `MaxSessions` with FIFO eviction, OR move the first-seen tracking entirely into the MCP SDK's session lifecycle if/when it exposes a hook (per the same caveat).

### IN-03: defaultSessionRunner allocates per-session closure even in production

**File:** `internal/daemon/daemon.go:660-662`

**Issue:** Every `StreamMCP` invocation in production calls `defaultSessionRunner(h.mcpServer)` which constructs a fresh closure capturing `mcpServer`. Allocation per-session, not per-request. The cost is negligible (one alloc per connect) but the test-seam pattern leaks an allocation that could be elided.

**Fix:** Cache the production runner once at handler construction. Cleaner refactor:

```go
func (h *forwarderServiceHandler) defaultRunner() sessionRunner {
    if h.cachedRunner == nil {
        h.cachedRunner = defaultSessionRunner(h.mcpServer)
    }
    return h.cachedRunner
}
```

Or equivalently, set `h.serveSession = defaultSessionRunner(mcpServer)` at handler-construction time in `listenSocket` (daemon.go:545-553). Production then uses the same field tests inject; the test seam becomes "field, not nil-fallback." Net: no nil-check, one allocation total. This is a micro-optimization; flagged as INFO because allocation per session is not a hot path. Skip if the current pattern is preferred for clarity.

### IN-04: ambiguityError WithDetail formats hits count but not strategy enum bound

**File:** `internal/fuzzy/match.go:299-301`

**Issue:** `WithDetail(fmt.Sprintf("strategy=%s count=%d", strategy, len(hits)))` formats the strategy as a Strategy string. Strategy is a defined string type with values `{StrategyExact, StrategyWhitespace, StrategyIndentationFlex, StrategyFailed}`. The cascade only ever calls `ambiguityError` from the first three strategies (matchSingle exits via `failureError` for the failed branch) — so in practice `strategy=failed` never appears in a detail string. Good defense.

But the test `TestAmbiguity_DoesNotCascade` at fuzzy_test.go:170-179 asserts `strategy=exact` substring matching, which couples the test to the formatted-string detail. If a future refactor changes the format (e.g., adds JSON encoding), the test breaks silently — `assert.Contains` on a free-text format. Consider switching to a structured detail (map) accessor, or pin the format with a fixture.

Strictly informational; the current code is correct.

---

_Reviewed: 2026-04-30T23:59:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
