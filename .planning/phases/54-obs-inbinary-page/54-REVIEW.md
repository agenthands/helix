---
phase: 54-obs-inbinary-page
reviewed: 2026-04-28T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/daemon/status_page.go
  - internal/daemon/status_page.html.tmpl
  - internal/daemon/status_page_test.go
  - internal/daemon/telemetry.go
  - docs/runbooks/runbooks_test.go
  - docs/usage_test.go
  - USAGE.md
findings:
  critical: 1
  warning: 4
  info: 4
  total: 9
status: issues_found
---

# Phase 54: Code Review Report

**Reviewed:** 2026-04-28
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 54 ships an in-binary HTML metrics page on the admin listener plus two CI guard tests. The implementation is generally tight: `html/template` provides automatic context-aware escaping, the handler buffers output before writing (clean 500 on template error), routing narrows to `/` properly, and the partial-gather case is handled.

However standard-depth review surfaces one BLOCKER (out-of-bounds panic in `humanBytes` for very large RSS values), three correctness WARNINGs in `buildPageModel` aggregation (last-write-wins on multi-label tool histograms, RSS, and goroutines), one test that is effectively non-discriminating (`TestStatusPageRegistryShared` checks for substring `"3"`), plus a few INFO items. None of the findings are security vulnerabilities — the template-safety surface is clean.

## Critical Issues

### CR-01: `humanBytes` panics on petabyte-scale input (index out of range)

**File:** `internal/daemon/status_page.go:348-360`
**Issue:** The `suffix` slice has four entries (`KiB`, `MiB`, `GiB`, `TiB`). The loop terminates when `v < unit`, but for `n >= 1024^5` (1 PiB) the loop body increments `exp` to 4, then the indexing `[]string{"KiB","MiB","GiB","TiB"}[exp]` panics with "index out of range [4] with length 4". For `n >= 1024^6` it grows further. While 1 PiB of RSS is unrealistic for a real process, `process_resident_memory_bytes` is read from a gauge that could be malformed (test fixtures, future bugs, or a corrupted gather result). The handler does NOT recover panics, so this propagates to `net/http`'s per-request recover — but the user sees a 500 with an opaque connection drop instead of the page.

The current code also silently caps at TiB even within the supported range without indicating overflow.

**Fix:**
```go
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	suffix := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < len(suffix)-1; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), suffix[exp])
}
```
Add `&& exp < len(suffix)-1` to the loop guard so the index can never exceed the slice. Adding `PiB`/`EiB` is optional but cheap.

## Warnings

### WR-01: Tool p95 aggregation is last-write-wins across label dimensions

**File:** `internal/daemon/status_page.go:157-166`
**Issue:** `serena_tool_duration_seconds` carries four labels (`tool_name`, `profile`, `mode`, `language`). The page keys p95 by `tool_name` only:

```go
p95 := p95FromHistogram(m.GetHistogram())
toolP95[name] = p95
toolHasP95[name] = !math.IsInf(p95, 0) && m.GetHistogram().GetSampleCount() > 0
```

When the same `tool_name` appears with different `(profile, mode, language)` combinations — which is the *normal* case once a daemon serves multiple workspaces / profiles — the loop overwrites the previous entry. The displayed p95 is therefore the p95 of whichever metric the gather happens to return last (gather order is non-deterministic across collectors). This is silently wrong and gets worse as the daemon sees more diverse traffic.

The same shape of bug affects `toolCalls[name] += ...` on line 155, but there summation across label dimensions is at least a defensible semantic (total calls regardless of profile/mode); for a histogram quantile there is no defensible single-line aggregation.

**Fix:** Either (a) merge bucket counts across all metrics for the same `tool_name` before computing p95, or (b) row per `(tool_name, profile, mode, language)` so the page reflects what the registry actually holds. Suggested (a):

```go
type histAcc struct {
	buckets map[float64]uint64 // upperBound -> cumulative
	sum     uint64             // sample count
}
toolHist := map[string]*histAcc{}
// ... in case "serena_tool_duration_seconds":
h := m.GetHistogram()
acc, ok := toolHist[name]
if !ok {
	acc = &histAcc{buckets: map[float64]uint64{}}
	toolHist[name] = acc
}
acc.sum += h.GetSampleCount()
for _, b := range h.GetBucket() {
	acc.buckets[b.GetUpperBound()] += b.GetCumulativeCount()
}
// then compute p95 from merged acc after the gather loop.
```

### WR-02: RSS and goroutines silently take the last metric in their family

**File:** `internal/daemon/status_page.go:203-213`
**Issue:** `process_resident_memory_bytes` and `go_goroutines` are emitted as a single metric per process by `prometheus/client_golang`'s process collector, so in practice the `for _, m := range f.GetMetric()` loop runs exactly once. But the code writes:

```go
v := int64(m.GetGauge().GetValue())
model.Process.RSSBytes = v
```

If a future change registers a second collector (or a test passes a fixture with two metrics in the family — exactly what `TestBuildPageModel_FromGatheredFamilies` could end up doing), the loop silently keeps the last one. Equivalent issue for `go_goroutines`. The intent is "exactly one metric"; the code does not assert that, and there is no log when the assumption is violated.

**Fix:** Either guard with a "first-only" pattern (`if model.Process.HasRSS { continue }`) and log on duplicate, or document that last-wins is intentional. Cheapest fix:
```go
case "process_resident_memory_bytes":
	if len(f.GetMetric()) > 0 {
		v := int64(f.GetMetric()[0].GetGauge().GetValue())
		model.Process.RSSBytes = v
		model.Process.HasRSS = true
		model.Process.RSSHuman = humanBytes(v)
	}
```

### WR-03: `TestStatusPageRegistryShared` substring check `"3"` is non-discriminating

**File:** `internal/daemon/status_page_test.go:125`
**Issue:** After incrementing the counter to 3, the test asserts:
```go
if !strings.Contains(body2, "3") {
```
The HTML page contains many literal `3`s independent of the counter: CSS values like `0.3rem`, `0.05rem` (no — but `0.30`/`0.3rem`), border-radius/padding tokens, the `2026` year in the timestamp footer (`...-04-28...`), and even the `state-3` class fallback. As written, the test passes regardless of whether the counter rendered correctly. It does NOT actually verify the registry-shared invariant it claims to.

This also masks regressions in the very behavior the test was added to prove (criterion: "refresh re-renders with updated counter").

**Fix:** Anchor the assertion to the row that should contain the count. For example, look for the literal table cell `<td>3</td>` (build-time renderer uses `{{.Calls}}` directly inside `<td>`):
```go
if !strings.Contains(body2, "<td>3</td>") {
	t.Fatalf("expected count 3 to render in a <td> cell; body=%q", body2[:min(2000,len(body2))])
}
```
Or assert the body2 count of `find_symbols` is followed by `<td>3</td>` via a regex:
```go
re := regexp.MustCompile(`find_symbols</td>\s*<td>3</td>`)
if !re.MatchString(body2) { t.Fatalf(...) }
```

### WR-04: Counter `float64 → uint64` cast does not handle negative / NaN values

**File:** `internal/daemon/status_page.go:155, 172, 191`
**Issue:** Three sites perform `uint64(m.GetCounter().GetValue())`. Counters are always non-decreasing in well-behaved producers, but a malformed gather result (e.g., partial-gather error, stale dto) could produce `NaN` or negative float values. `uint64(NaN)` and `uint64(-1.0)` are implementation-defined in Go and on amd64 produce `0x8000000000000000` (a huge positive number). That value would render as `9223372036854775808` in the page — confusing to operators and visually breaks tables.

The handler already accepts partial gather (line 117), making this scenario not purely hypothetical.

**Fix:** Sanitize once in a small helper:
```go
func counterToUint64(v float64) uint64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return uint64(v)
}
```
Apply at lines 155, 172, 191. Same hardening for the gauge int64 casts at lines 179, 205, 212 if NaN-safety is desired.

## Info

### IN-01: `circuitStateName` for non-{0,1,2} values renders as a CSS class with raw integer

**File:** `internal/daemon/status_page.go:300-311`, `internal/daemon/status_page.html.tmpl:69`
**Issue:** Unknown circuit gauge values fall through to `fmt.Sprintf("%d", int(v))`, which the template then injects into `class="state-{{.State}}"`. `html/template` correctly escapes attribute context (no XSS), but the resulting class name (e.g., `state-7`) has no matching CSS rule, so the operator sees an unstyled cell with no visual indication that the value is anomalous. Also, `int(v)` for `v=NaN` is implementation-defined and typically renders as `-2147483648` on amd64 — visually jarring.
**Fix:** Map unknown values to a sentinel like `"unknown"` and add `.state-unknown { color: #b03030; font-style: italic; }` so anomalous states are visually flagged:
```go
default:
	return "unknown"
```

### IN-02: `p95FromHistogram` assumes sorted buckets

**File:** `internal/daemon/status_page.go:281-296`
**Issue:** The function returns the upper bound of the *first* bucket whose cumulative count crosses 95%. Prometheus dto buckets are conventionally sorted ascending, but neither the code nor a comment asserts this. A misordered fixture (or a future client_model change) silently produces wrong quantiles.
**Fix:** Add a one-line precondition or a `sort.SliceStable` defense:
```go
buckets := h.GetBucket()
sort.SliceStable(buckets, func(i, j int) bool {
	return buckets[i].GetUpperBound() < buckets[j].GetUpperBound()
})
```
Or just document the assumption with a comment referencing the dto contract.

### IN-03: `handleStatusPage` does not defend against `d.obs == nil`

**File:** `internal/daemon/status_page.go:113`
**Issue:** `d.obs.Metrics().Registry().Gather()` panics if `d.obs` is nil. The current call site (`telemetry.go:75`) only mounts the metrics endpoint after a nil check, but mounts `/` (the status page) unconditionally at line 63. If a future reorg makes `obs` optional, every GET / panics until the per-request recover catches it.
**Fix:** Either move the `d.obs != nil && d.obs.Metrics() != nil` check ahead of the `mux.Handle("/", ...)` registration, or add a defensive nil check inside `handleStatusPage`:
```go
if d.obs == nil || d.obs.Metrics() == nil {
	http.Error(w, "metrics not configured", http.StatusServiceUnavailable)
	return
}
```

### IN-04: USAGE regression test does not verify the page paragraph mentions refresh / 127.0.0.1 binding nor browse-with-curl, only forbids brand names

**File:** `docs/usage_test.go:35-39`
**Issue:** The body-of-paragraph check only enforces ABSENCE of `prometheus | grafana | docker | podman`. Criterion #4 forbids prerequisites — but the test does not assert any positive content (e.g., that the paragraph mentions "refresh" or "browser"). A future edit that strips the paragraph down to a single sentence would still pass the test even if it loses the operator-facing invariants.
**Fix:** Add positive assertions for at least one stable phrase the runbook contract depends on:
```go
for _, want := range []string{"browser", "refresh"} {
	if !strings.Contains(body, want) {
		t.Errorf("In-Binary Metrics Page paragraph missing required hint %q", want)
	}
}
```
This is judgment — the current weak-form gate is defensible; flagging as Info, not a blocker.

---

_Reviewed: 2026-04-28_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
