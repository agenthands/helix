# Phase 54: obs-inbinary-page - Research

**Researched:** 2026-04-28
**Domain:** In-binary HTML observability page on the existing admin listener (`html/template` + `prometheus.Gatherer` walk)
**Confidence:** HIGH

## Summary

Phase 54 adds a single GET `/` HTML handler to the existing admin listener (`internal/daemon/telemetry.go`). The handler walks the same `*prometheus.Registry` already exposed at `/metrics`, projects metric families into a small in-memory view-model, and renders an inline-CSS HTML page via `html/template`. Refresh = re-render — no polling endpoint, no second registry, no caching.

The four operator runbooks already exist (`docs/runbooks/`, commit `780a3eda`) and match success criterion #3 verbatim: log-line symptoms, local-only triage commands (`serena status`, `curl /metrics`, `vm_stat`/`/proc/meminfo`), config-knob remediation, no Grafana, no PromQL fences. Verification is a read-only gate — no rewrite needed. The README cross-links the four runbooks correctly.

USAGE.md needs a small addition: an "In-binary metrics page" subsection under `## Observability Quickstart` (line 606) pointing at `http://127.0.0.1:9100/` and `docs/runbooks/`, with no third-party software prerequisites. The existing PromQL/Prometheus scrape config already there can stay (it's an optional remote-monitoring path), but the new page must be presented as the default operator surface.

**Primary recommendation:** Add `internal/daemon/admin_page.go` with `(d *Daemon).handleAdminIndex(w, r)` mounted on `mux.Handle("/", ...)` in `listenAdmin` (telemetry.go:58, just before `mux.HandleFunc("/healthz", ...)`). The handler calls `d.obs.Metrics().Registry().Gather()` to get `[]*dto.MetricFamily`, projects into a typed view-model in a separate pure function (testable without HTTP), and executes a single `html/template.Template` parsed once at package init. Zero new go.mod entries — `prometheus/client_model` (which provides `dto.MetricFamily`) is already a direct dependency at v0.6.2.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP route registration | `internal/daemon` (admin listener) | — | Mount point already exists in `telemetry.go`; the listener owns routing on the admin port. |
| Metric family snapshot | `internal/obs` (registry owner) | `internal/daemon` (caller) | `*obs.Metrics.Registry()` already exposes `prometheus.Gatherer`; daemon calls `Gather()` per request. |
| View-model projection (MetricFamily → page model) | `internal/daemon/admin_page.go` (new file) | — | Pure function over `[]*dto.MetricFamily`; no IPC, no kernel access — keep it next to the handler so nothing else in the codebase needs to know about `dto.MetricFamily`. |
| HTML rendering | `internal/daemon/admin_page.go` | — | `html/template` execution; template embedded with `//go:embed` next to handler. |
| Runbook content | `docs/runbooks/` (already shipped) | — | Static markdown; no code touches it. |
| USAGE.md prose update | `USAGE.md ## Observability Quickstart` | — | Single doc edit. |

**Key boundary:** `internal/obs/` does NOT learn about HTML rendering. It already publishes the registry; the daemon-side handler is the only consumer of `Gather()`. This preserves the "prometheus/client_golang stays confined to internal/obs" rule from `metrics.go:3-6`, with a single carve-out for `dto.MetricFamily` which the daemon needs to walk.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | (Stale text in REQUIREMENTS.md says "JSON Grafana dashboards in `deploy/grafana/`"; per ROADMAP.md Phase 54 retitle on 2026-04-28, the Grafana scope is reverted and OBS-01 is now satisfied by the in-binary HTML page on the admin listener.) | Admin listener at `internal/daemon/telemetry.go:48-103` already binds loopback, mounts `/metrics`. Adding `/` HTML index is a one-handler change. |
| OBS-02 | Written runbooks in `docs/runbooks/` for ErrCircuitOpen, deadline timeouts, LS crash/restart, memory-pressure eviction. | All four files plus `README.md` index already shipped in commit `780a3eda` and verified below (Runbook Verification section). Phase 54 only needs to confirm content matches success criterion #3, not rewrite. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

| Directive | Source | Phase 54 Implication |
|-----------|--------|----------------------|
| Single Go binary, no external runtime deps | "Project" section | Hard constraint — this phase exists *because* Grafana violated it. New approach must add zero go.mod entries. |
| `go vet ./...` and `go test ./...` before completing any Go task | "Go Development Commands" | Validation gate. |
| `gofmt -w .` to format | "Go Development Commands" | Run before commit. |
| Go-native, MCP-first, LSP only — no proprietary backends | "Constraints" | Confirms HTML page is internal observability, not a new product surface. |
| `prometheus/client_golang` confined to `internal/obs/` | `internal/obs/metrics.go:3-6` (frozen design rule) | Daemon-side handler may import `prometheus/client_model` (`dto.*`) since the registry already exposes `Gatherer`, but MUST NOT call `prometheus.NewRegistry`, `MustRegister`, or any `*prometheus.*` constructor. Stay on the read side of the boundary. |
| Edit, Write only via GSD workflow | "GSD Workflow Enforcement" | This is a `/gsd-plan-phase 54` phase — execution will go through `/gsd:execute-phase`. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `html/template` (stdlib) | Go 1.25 | HTML rendering with auto-escaping | Stdlib; auto-escapes `{{.}}` interpolation against XSS; no external dep. [VERIFIED: stdlib] |
| `github.com/prometheus/client_golang/prometheus` | v1.23.2 | Already in go.mod; `*Registry` implements `Gatherer`. | Already used to construct the registry at `internal/obs/metrics.go:73`. [VERIFIED: go.mod direct dep] |
| `github.com/prometheus/client_model/go` | v0.6.2 | Provides `dto.MetricFamily`, `dto.Metric`, `dto.MetricType` for walking the gather result. | Already a direct dep in go.mod (transitive of client_golang but listed explicitly). [VERIFIED: `go list -m` returned v0.6.2] |
| `net/http` (stdlib) | Go 1.25 | `mux.Handle("/", ...)`, `httptest.NewRecorder` for tests. | Stdlib. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `embed` (stdlib) | Go 1.25 | `//go:embed page.html.tmpl` to ship the template inside the binary | When the template grows beyond a backtick-string literal; either choice is fine. [VERIFIED: stdlib] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `html/template` | `text/template` | `text/template` does NOT auto-escape — would re-introduce XSS surface for any metric label that ever escapes its closed enum. Reject. |
| Single template | Template per section | Single template is simpler and the page is small (one screen). Keep it single. |
| Auto-refresh `<meta http-equiv="refresh" content="5">` | No auto-refresh | Success criterion #2 says "Refresh on the page = re-render from current registry state." Reading literally: the operator presses F5. A 5-second meta refresh is a friendly default but is **not required** by the criterion. Recommend: include it commented out, or behind a query string `?refresh=5`, so default behavior is "no auto-refresh" (matches the literal criterion) and operators who want it can opt in. Plan-checker should flag if a plan adds unconditional auto-refresh — it's a deviation from criterion #2. |

**Installation:** None. All dependencies already in `go.mod`.

**Version verification:**
- `prometheus/client_golang v1.23.2` — confirmed in `go.mod` [VERIFIED: grep on go.mod]
- `prometheus/client_model v0.6.2` — confirmed via `go list -m` [VERIFIED]

## Architecture Patterns

### System Architecture Diagram

```
   Operator browser ──GET / ──────────────────►  admin listener (loopback)
                                                  │  (telemetry.go:48 net.Listen)
                                                  │
                                                  ▼
                                        mux.Handle("/", handleAdminIndex)   ◄── NEW
                                                  │
                                                  ▼
                                  d.obs.Metrics().Registry().Gather()
                                                  │
                                                  ▼
                                           []*dto.MetricFamily
                                                  │
                                                  ▼
                                       buildPageModel(families) ──► PageModel{
                                                                       Tools: []ToolRow,
                                                                       Edits: []EditRow,
                                                                       Workers: []WorkerRow,
                                                                       Circuits: []CircuitRow,
                                                                       Evictions: []EvictionRow,
                                                                       RepoMapHitRate: float64,
                                                                       Process: ProcessInfo,
                                                                       GeneratedAt: time.Time,
                                                                    }
                                                  │
                                                  ▼
                                       indexTmpl.Execute(w, model)
                                                  │
                                                  ▼
                                          inline-CSS HTML page

   Same registry also serves: GET /metrics ──► promhttp.HandlerFor(registry, ...)
                                                  (telemetry.go:71 — unchanged)
```

The diagram is single-process. Both `/` and `/metrics` read the same `*prometheus.Registry` instance owned by `*obs.Metrics`. No second registry, no scraper, no caching layer (criterion #2).

### Recommended Project Structure

```
internal/daemon/
├── telemetry.go            # existing — listenAdmin, handleHealthz, handleReadyz
├── admin_page.go           # NEW — handleAdminIndex + buildPageModel + indexTmpl init
├── admin_page_test.go      # NEW — table-driven tests on buildPageModel + httptest on handler
└── admin_page.html.tmpl    # NEW — //go:embed'ed inline-CSS template
```

Keeping the new file separate from `telemetry.go` makes the diff small and the new code's blast radius obvious. `listenAdmin` only changes to add one line: `mux.Handle("/", http.HandlerFunc(d.handleAdminIndex))` before the `/healthz` registration (so `/healthz`, `/readyz`, `/metrics`, `/debug/pprof/*` continue to win on exact match — `ServeMux` longest-prefix match rules apply, and exact paths beat `/`).

### Pattern 1: Pure projection function for testability

**What:** Separate `buildPageModel([]*dto.MetricFamily) PageModel` from the HTTP handler. The handler is a thin shim: gather → project → execute template.

**When to use:** Always for this kind of read-side observability handler. Lets the bulk of the logic be tested with table-driven tests over hand-built `dto.MetricFamily` fixtures, no HTTP, no template, no registry.

**Example:**
```go
// Source: pattern derived from telemetry.go's existing handleHealthz/handleReadyz separation
// (handlers are thin; logic is in tested helpers like validateAdminAddr).

func (d *Daemon) handleAdminIndex(w http.ResponseWriter, _ *http.Request) {
    families, err := d.obs.Metrics().Registry().Gather()
    if err != nil {
        // Same posture as promhttp.ContinueOnError (telemetry.go:74) — partial data is OK.
        d.logger.Warn("admin index: gather error", "err", err)
    }
    model := buildPageModel(families, time.Now())
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := indexTmpl.Execute(w, model); err != nil {
        d.logger.Warn("admin index: template execute", "err", err)
    }
}
```

### Pattern 2: Walking `dto.MetricFamily` for typed extraction

**What:** Switch on `mf.GetType()` (`COUNTER`, `GAUGE`, `HISTOGRAM`) and read the appropriate sub-message. Labels are a flat `[]*dto.LabelPair`.

**When to use:** Inside `buildPageModel`, once per metric family of interest.

**Example:**
```go
// Source: prometheus/client_model/go/metrics.pb.go (dto package)
// [CITED: github.com/prometheus/client_model README + auto-generated proto types]

import dto "github.com/prometheus/client_model/go"

func extractLabelValue(m *dto.Metric, name string) string {
    for _, lp := range m.GetLabel() {
        if lp.GetName() == name {
            return lp.GetValue()
        }
    }
    return ""
}

func sumCounterByLabel(mf *dto.MetricFamily, label string) map[string]float64 {
    out := map[string]float64{}
    for _, m := range mf.GetMetric() {
        v := extractLabelValue(m, label)
        out[v] += m.GetCounter().GetValue()
    }
    return out
}

// p95 from a histogram: Prometheus client model exposes Bucket{} entries with
// CumulativeCount and UpperBound. p95 = first bucket where cumulative >= 0.95 * total.
// (See "p95 over a single dto.Histogram" in Pitfalls below — this is approximate.)
func p95FromHistogram(h *dto.Histogram) float64 {
    total := h.GetSampleCount()
    if total == 0 { return 0 }
    target := 0.95 * float64(total)
    for _, b := range h.GetBucket() {
        if float64(b.GetCumulativeCount()) >= target {
            return b.GetUpperBound()
        }
    }
    return math.Inf(+1)
}
```

### Pattern 3: Embed the template, parse once

```go
import (
    _ "embed"
    "html/template"
)

//go:embed admin_page.html.tmpl
var indexTmplSrc string

var indexTmpl = template.Must(template.New("admin-index").Parse(indexTmplSrc))
```

`template.Must` panics at init time if the template is malformed — fail-fast is exactly right for an embedded resource.

### Anti-Patterns to Avoid
- **Reading metric values via `prometheus.Counter.Write(&dto.Metric{})` on the live vector.** This bypasses the registry's `Gather()` ordering and label semantics. Always go through `Registry().Gather()` per criterion #2.
- **Building a second `prometheus.Registry`.** Criterion #2 explicitly forbids it. The handler MUST read the same registry that serves `/metrics`.
- **Accepting any user input.** The page is GET-only with no query parameters that affect rendering. Don't add filters, don't accept POST. (Admin listener is loopback-only by `validateAdminAddr` at telemetry.go:108, but defense-in-depth: still reject non-GET with 405.)
- **Using `text/template` instead of `html/template`.** Prometheus label values are bounded enums today, but auto-escaping is the right default for any HTML output. Future label additions must not silently become an XSS vector.
- **Querying every request and re-parsing the template.** Template lives at package scope, parsed once at import.
- **Adding a /metrics-page-data JSON endpoint and rendering with JS.** Criterion #1 says "single self-contained HTML page (inline CSS, no JS frameworks)". Server-side render only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTML escaping | Custom string-builder | `html/template` | Auto-escapes by context (attribute, text, URL); avoids XSS class entirely. |
| Metric snapshot | Custom counter cache | `prometheus.Gatherer.Gather()` | The registry already serializes a coherent snapshot per call. |
| Histogram quantile estimation | Linear interpolation | Bucket-edge readout (see Pitfall #1) | True quantiles need the original samples; we only have buckets. Document the approximation, don't pretend it's exact. |
| HTTP routing | Custom multiplexer | `http.ServeMux` already in use | `mux.Handle("/", ...)` + exact-match for `/healthz` etc. is the standard pattern. |
| CSS framework | Tailwind / Bootstrap | Inline `<style>` block | Criterion #1: "inline CSS, no JS frameworks." Single binary, single GET. |

**Key insight:** Every external dep added to this page becomes a perpetual upgrade burden in a binary that markets itself as zero-dep. The discipline is: stdlib + already-present deps only.

## Common Pitfalls

### Pitfall 1: Histogram p95 is bucket-quantized, not exact
**What goes wrong:** `dto.Histogram` exposes only cumulative bucket counts, not raw samples. Computing p95 by scanning buckets gives the upper bound of the bucket where the 95th percentile falls — which can be off by a full bucket width. With `prometheus.DefBuckets` (5ms, 10ms, 25ms, ...), p95 of "8ms typical" data reports 10ms.
**Why it happens:** Prometheus is designed for server-side `histogram_quantile()` over scraped buckets across time. Single-snapshot client-side quantile is necessarily approximate.
**How to avoid:** Document this on the page itself (a small `(approx, bucket-quantized)` annotation next to the p95 column). Don't try to interpolate within a bucket — the result is no more accurate and looks falsely precise.
**Warning signs:** A reviewer asks "why does p95 jump by 5ms suddenly?" — answer is bucket boundary crossing, not a real latency change.

### Pitfall 2: `/` route conflicts with `/metrics`, `/healthz`, etc.
**What goes wrong:** `http.ServeMux` treats patterns ending in `/` as prefix matches. Registering `mux.Handle("/", h)` makes `/` match every path that doesn't have a more specific handler. Naively this is fine — `/metrics` already has an exact-match registration so it wins. But adding `/` AFTER the others can still be wrong if a future maintainer flips the order or adds a path-prefixed route.
**Why it happens:** `ServeMux` longest-match wins; registering `/` is the catch-all "fallback" handler.
**How to avoid:** Inside `handleAdminIndex`, check `r.URL.Path != "/"` and return 404 for anything else. That way `/foo` returns a clean 404 instead of rendering the index page on a typo.
**Warning signs:** Test must include a request to `/notapage` returning 404, not 200.

### Pitfall 3: `Gather()` partial-failure handling
**What goes wrong:** `prometheus.Gatherer.Gather()` can return both `families` and `err` together (per Gatherer interface contract — partial gather is valid). Code that does `if err != nil { return }` will discard valid metric data on a single bad collector.
**Why it happens:** Prometheus design: degrade, don't fail. Mirrors the `promhttp.ContinueOnError` choice already made at `telemetry.go:74`.
**How to avoid:** Treat err as a log-and-continue signal. Always render the page from whatever families we got, even partial.
**Warning signs:** A bad custom collector somewhere makes the entire page 500. Should be log-warning + degraded-but-rendered page.

### Pitfall 4: Concurrent counter mutation during `Gather()`
**What goes wrong:** Counters are atomic but the snapshot returned by `Gather()` is a point-in-time read. A counter incremented mid-gather may or may not appear in the result. Sums computed from labels (e.g., total tool calls = sum over `outcome` labels) can differ by 1-2 from a hypothetical "true" total because some label combinations were read before increment and others after.
**Why it happens:** No global lock across collectors.
**How to avoid:** Don't display "totals" computed from sub-labels — display the families as Prometheus structures them. If a "total" is needed, expose it as a separate metric (e.g., a non-labeled counter), not a derived sum.
**Warning signs:** Page shows "tool_calls_total: 1003" while sum of per-outcome rows is 1002. This is benign in production but will alarm reviewers.

### Pitfall 5: Template execution after WriteHeader writes a malformed page
**What goes wrong:** If `indexTmpl.Execute(w, model)` fails mid-write, headers are already sent (write to `http.ResponseWriter` flushes them) and the response body is truncated HTML. Browser shows a half-rendered page.
**Why it happens:** `html/template` errors out at first encoding problem; partial output already on the wire cannot be retracted.
**How to avoid:** Render to `bytes.Buffer` first, only `w.Write(buf.Bytes())` on success. For a small page (~10KB) this is cheap. On error, return 500 with a plain-text body.
**Warning signs:** Test that intentionally feeds a malformed model and asserts 500 with a clean error body.

### Pitfall 6: Process metrics may be absent on Windows
**What goes wrong:** `collectors.NewProcessCollector()` (registered at `internal/obs/metrics.go:184`) populates `process_resident_memory_bytes` and `process_open_fds` on Linux/macOS but yields fewer metrics on Windows.
**Why it happens:** `process_collector_other.go` in client_golang stubs out unsupported metrics on Windows.
**How to avoid:** When extracting `process_resident_memory_bytes`, treat absence as "—" not as a zero. Same for `process_open_fds`.
**Warning signs:** Page shows "RSS: 0 B" on Windows. Should be "—".

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — page is read-only over the live registry; no on-disk artifacts. | None. |
| Live service config | None — admin listener already runs at `127.0.0.1:9100` per existing config. | None. |
| OS-registered state | None — the new route is in-process. | None. |
| Secrets/env vars | None. | None. |
| Build artifacts | The new template file (`admin_page.html.tmpl`) is `//go:embed`'ed into the binary; no separate artifact. | None. |

**Nothing found in any category — verified by:** (1) no new files outside `internal/daemon/` and `USAGE.md`, (2) registry already exists, (3) admin port already bound by existing code.

## Code Examples

### Reading every metric family of interest

```go
// Source: pattern derived from internal/obs/metrics.go families list (lines 36-66)
// and prometheus/client_model proto definitions [CITED: client_model/go/metrics.pb.go]

func buildPageModel(families []*dto.MetricFamily, now time.Time) PageModel {
    m := PageModel{GeneratedAt: now}
    for _, f := range families {
        switch f.GetName() {
        case "serena_tool_calls_total":
            m.Tools = mergeToolCalls(m.Tools, f) // sum by tool_name, group outcomes
        case "serena_tool_duration_seconds":
            m.Tools = mergeToolDurations(m.Tools, f) // p95 per tool_name from histogram buckets
        case "serena_edit_outcome_total":
            m.Edits = collectEditOutcomes(f)
        case "serena_lspool_workers":
            m.Workers = collectGaugeByLanguage(f)
        case "serena_lspool_circuit_state":
            m.Circuits = collectCircuitStates(f) // map 0/1/2 → closed/half-open/open
        case "serena_lspool_evictions_total":
            m.Evictions = collectEvictionsByReason(f)
        case "serena_lspool_cache_total", "serena_repomap_cache_total":
            m.Caches = appendCacheRow(m.Caches, f) // hit/miss → hit-rate
        case "process_resident_memory_bytes":
            m.Process.RSS = readSingleGauge(f)
        case "go_goroutines":
            m.Process.Goroutines = int(readSingleGauge(f))
        }
    }
    return m
}
```

### HTTP handler with bytes.Buffer for atomic write

```go
func (d *Daemon) handleAdminIndex(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.Header().Set("Allow", "GET")
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    families, err := d.obs.Metrics().Registry().Gather()
    if err != nil {
        d.logger.Warn("admin index: partial gather", "err", err)
    }
    model := buildPageModel(families, time.Now())
    var buf bytes.Buffer
    if err := indexTmpl.Execute(&buf, model); err != nil {
        d.logger.Error("admin index: template execute", "err", err)
        http.Error(w, "template error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("Cache-Control", "no-store")
    _, _ = w.Write(buf.Bytes())
}
```

### Test pattern (mirrors `telemetry_test.go` `httptest.NewRecorder` style)

```go
func TestHandleAdminIndex_RendersFromRegistry(t *testing.T) {
    p := obs.Noop(slog.NewTextHandler(io.Discard, nil)) // gives a working *Provider with real registry
    p.Metrics().ToolCalls.WithLabelValues("find_symbols", "claude-code", "edit", "go", "success").Inc()
    d := &Daemon{
        config: &config.SerenaConfig{Observability: config.ObservabilityConfig{}},
        logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
        obs:    p,
    }
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    rec := httptest.NewRecorder()
    d.handleAdminIndex(rec, req)
    if rec.Code != http.StatusOK {
        t.Fatalf("status = %d, want 200", rec.Code)
    }
    body := rec.Body.String()
    if !strings.Contains(body, "find_symbols") {
        t.Fatalf("expected tool name in body; got %q", body[:min(200, len(body))])
    }
    if !strings.Contains(body, "<style>") {
        t.Fatal("expected inline <style> block (criterion #1: inline CSS)")
    }
}
```

The `obs.Noop` constructor returns a Provider whose `Metrics()` is real and registered (verified at `internal/obs/obs.go:50`), so tests can mutate counters and read them back via `Registry().Gather()` without any production wiring.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Grafana dashboards + PromQL-deep-linked runbooks | In-binary HTML page on existing admin listener | 2026-04-28 (Phase 54 retitle) | Removes Prometheus + Grafana + Podman network requirements; aligns with "single Go binary, no Docker" product principle. |
| Bespoke metrics page library (e.g., `expvar` HTML view) | `html/template` over `prometheus.Gatherer.Gather()` | This phase | Reuses existing registry; zero new deps. |

**Deprecated/outdated:**
- Anything in the abandoned `54-obs-dashboards-runbooks/` plan tree (already replaced by the rename to `54-obs-inbinary-page` per commit `82d39f87`).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Bucket-quantized p95 is acceptable for the operator triage use case (no SLO-grade quantile required). | Pitfall 1 | If reviewers want exact quantile, plan must add a separate non-bucketed summary or omit p95 entirely. Low risk — runbooks already triage on counters and rates, not p95. |
| A2 | `<meta http-equiv="refresh">` is opt-in (off by default). | Standard Stack alternatives | If user wants auto-refresh by default, add it unconditionally — trivial change. |
| A3 | The page renders English-only labels (no i18n). | Implicit throughout | Serena CLI is English-only today (`internal/cli/status_output.go` strings are English); no risk. |
| A4 | Loopback-only access is sufficient security; no auth, no CSRF. | Anti-Patterns | The admin listener is gated by `validateAdminAddr` (`telemetry.go:108`) to loopback. Same posture as `/healthz`, `/metrics`, pprof. |

## Open Questions

1. **Should the page show `serena_session_lifecycle_total` (Phase 53 metric) as well?**
   - What we know: It's in the registry, it's in the criterion #1 list as "lspool worker count + circuit state by language" but session lifecycle is a different family.
   - What's unclear: The criterion list reads "tool call counts + p95 latency by tool, edit outcomes, lspool worker count + circuit state by language, recent eviction reasons, repomap cache hit rate, and process RSS / goroutines." Session lifecycle is NOT explicitly listed.
   - Recommendation: Out of scope for v1 of the page. Plan-checker should verify the page renders exactly the seven categories named in criterion #1; session lifecycle is metadata for a future iteration.

2. **"Recent eviction reasons" — is this a counter snapshot or a log tail?**
   - What we know: Criterion #1 says "recent eviction reasons" (plural). The metric `serena_lspool_evictions_total{language, reason}` is a counter — it can show *which* (language, reason) tuples have non-zero evictions, but not when they happened.
   - What's unclear: Does "recent" mean "any non-zero counter" or "since process start" or "in the last N minutes"?
   - Recommendation: Render as "evictions since process start, grouped by (language, reason)" — that's what the counter natively provides. Don't try to derive a sliding window without a delta-from-prior-snapshot machinery (out of scope, would require state).

3. **Page on `/` vs. `/dashboard` vs. `/ui`?**
   - What we know: Criterion #1 says "opening `http://127.0.0.1:9100/`" — explicitly `/`.
   - Recommendation: Mount on `/` as the criterion specifies. No ambiguity.

4. **Should `serena status` CLI link to the page in its output?**
   - What we know: `internal/cli/status_output.go` already prints worker/circuit summaries to terminal.
   - Recommendation: Out of scope for this phase. A CLI footer like "See http://127.0.0.1:9100/ for full metrics" is a one-line addition, but the criteria don't require it. Defer.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build | ✓ (assumed; project requires Go 1.25 per Phase 50) | 1.25 | — |
| `prometheus/client_golang` | Registry | ✓ (already in go.mod) | v1.23.2 | — |
| `prometheus/client_model` | `dto.MetricFamily` walk | ✓ (already in go.mod) | v0.6.2 | — |
| `html/template`, `embed`, `net/http`, `net/http/httptest` | Handler + tests | ✓ (stdlib) | Go 1.25 | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

This phase is the textbook "no new external deps" phase.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `net/http/httptest` |
| Config file | None (Go convention) |
| Quick run command | `go test ./internal/daemon/ -run TestHandleAdminIndex -count=1` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-01 (criterion #1) | GET / returns 200 with HTML containing the 7 named sections | unit (httptest) | `go test ./internal/daemon -run TestHandleAdminIndex_RendersFromRegistry -count=1` | ❌ Wave 0 |
| OBS-01 (criterion #1) | Inline `<style>` block present, no `<script src>` | unit | `go test ./internal/daemon -run TestHandleAdminIndex_NoExternalAssets` | ❌ Wave 0 |
| OBS-01 (criterion #2) | Page reads registry via `Gather()`, not a clone | unit | `go test ./internal/daemon -run TestBuildPageModel_FromGatheredFamilies` | ❌ Wave 0 |
| OBS-01 (criterion #2) | Counter incremented between two GETs reflects in second response | unit | `go test ./internal/daemon -run TestHandleAdminIndex_RefreshSeesNewIncrements` | ❌ Wave 0 |
| OBS-01 (criterion #5) | `go.mod` direct dep count unchanged before/after phase | shell + go list | `go list -m all \| wc -l` (manual snapshot before/after) | manual |
| OBS-01 | Non-GET returns 405 with Allow header | unit | `go test ./internal/daemon -run TestHandleAdminIndex_MethodNotAllowed` | ❌ Wave 0 |
| OBS-01 | Path other than `/` returns 404 | unit | `go test ./internal/daemon -run TestHandleAdminIndex_NotFoundOnOtherPaths` | ❌ Wave 0 |
| OBS-01 | Bucket-quantized p95 documented on page | unit (string assert) | `go test ./internal/daemon -run TestPageRendersApproxAnnotation` | ❌ Wave 0 |
| OBS-02 (criterion #3) | All 4 runbooks exist with required sections (Symptoms, Inspect/Triage, Remediate, Verify, Escalate) | unit (file existence + grep) | `go test ./internal/daemon -run TestRunbooksPresent` OR shell `for f in ErrCircuitOpen.md deadline-timeouts.md ls-crash-restart.md memory-pressure-eviction.md; do test -f docs/runbooks/$f; done` | manual / Wave 0 |
| OBS-02 (criterion #3) | No "grafana" string in any runbook | shell | `! grep -ri grafana docs/runbooks/` | manual |
| OBS-02 (criterion #3) | No PromQL fences (```promql) in any runbook | shell | `! grep -r '\`\`\`promql' docs/runbooks/` | manual |
| OBS-01 (criterion #4) | USAGE.md ## Observability Quickstart mentions `http://127.0.0.1:9100/` and `docs/runbooks/` | shell | `grep -q 'http://127.0.0.1:9100/' USAGE.md && grep -q 'docs/runbooks' USAGE.md` | manual |

### Sampling Rate
- **Per task commit:** `go test ./internal/daemon -count=1` (~5s)
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go vet ./...` clean; `go test ./...` green; `gofmt -l .` empty.

### Wave 0 Gaps
- [ ] `internal/daemon/admin_page_test.go` — covers OBS-01 criteria 1, 2, 5 + 405/404 cases
- [ ] No fixture files needed (use `obs.Noop()` for working registry; populate counters in-test)
- [ ] No new framework install; stdlib `testing` + `httptest` only.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Listener is loopback-only (`validateAdminAddr` enforces it). v1.3 will add auth for non-loopback per existing comment at `telemetry.go:120`. |
| V3 Session Management | no | Stateless GET. |
| V4 Access Control | no | Loopback gating IS the access control. |
| V5 Input Validation | yes | Reject non-GET (405); reject path != `/` (404); no query params consumed for rendering decisions. |
| V6 Cryptography | no | No secrets, no signing. |
| V14 Configuration | yes | `Cache-Control: no-store`, `X-Content-Type-Options: nosniff` to prevent MIME sniffing of metric label values that might look HTML-ish. |

### Known Threat Patterns for Go HTTP + html/template

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via metric label values | Tampering | `html/template` auto-escapes `{{.}}` interpolation. Never use `{{.X | safehtml}}` for any field that originates from a label. Closed-enum allowlists at metric-write time (see `internal/obs/metrics.go:228, 240, 251, ...`) are defense in depth, not the primary control. |
| Response splitting | Tampering | Do not echo any header value from the request. Headers we set are static literals. |
| Open-redirect / SSRF | Tampering / Information Disclosure | No URL-construction from input; the page is static GET on `/`. |
| MIME-sniff XSS on text-y label values | Tampering | `X-Content-Type-Options: nosniff`. |
| DoS by repeated /  refresh | Denial of Service | Loopback-only. `Gather()` is O(metrics); a malicious actor on loopback already has unrestricted access. Out of scope. |

## Runbook Verification (against Success Criterion #3)

Read all 4 runbooks plus `README.md` at `docs/runbooks/` (commit `780a3eda`). Findings:

| Required element (criterion #3) | ErrCircuitOpen | deadline-timeouts | ls-crash-restart | memory-pressure-eviction |
|---------------------------------|----------------|-------------------|------------------|-------------------------|
| Symptoms with specific log lines | ✓ (`level=WARN msg="lspool circuit opened" language=java consecutive_failures=5`) | ✓ (`level=WARN msg="tool call timeout" tool=find_symbols duration=10.0s deadline=10s`) | ✓ (`worker spawned` / `worker exited reason=crash` pair) | ✓ (`level=WARN msg="evicting worker due to memory pressure" language=java rss_pct=82`) |
| Triage as local commands | ✓ (`serena status --verbose`, `curl /metrics`, fd/RSS check) | ✓ (`serena status`, grep on metrics, session-lifecycle metric check) | ✓ (`pkill`, config inspection, restart counter snapshot) | ✓ (`vm_stat`, `/proc/meminfo`, eviction counter, workers gauge) |
| Remediation as config knobs / restart procedure | ✓ (`consecutive_failures` threshold, `pkill -USR1 serena`) | ✓ (`mcp.tool_deadlines`, `kernel.activation_timeout`) | ✓ (`languages.java.ls_binary_override`, `lspool.max_workers_per_language`, `pkill -USR1`) | ✓ (`lspool.eviction.memory_pressure_threshold_pct`, `max_workers_per_language`) |
| No Grafana panel deep-links | ✓ (none) | ✓ (none) | ✓ (none) | ✓ (none) |
| No PromQL fences | ✓ (only `bash` fences with `curl ... \| grep`) | ✓ | ✓ | ✓ |
| Cross-links between runbooks | ✓ → memory-pressure | ✓ → ErrCircuitOpen | ✓ → memory-pressure, ErrCircuitOpen | ✓ → ErrCircuitOpen |

`docs/runbooks/README.md` correctly indexes all four with one-line "when to use" descriptions, states "no Prometheus, no Grafana, no Docker", and provides the prerequisite `curl http://127.0.0.1:9100/metrics | head -5` smoke test.

**Verdict:** Runbooks already satisfy criterion #3 in full. **No rewrite needed.** Phase 54's runbook task collapses to a verification gate: a CI/test step asserting (a) all 4 files exist, (b) no `grafana` string, (c) no `promql` fences, (d) README links to all 4. This is a 5-line shell test or a `TestRunbooksPresent` Go test reading them off disk.

## USAGE.md Update Plan (Criterion #4)

**Current state at line 606** (`## Observability Quickstart`):
- "Enable the Admin Listener" subsection ✓
- "Health Checks" subsection ✓
- "Prometheus Metrics" subsection — has `curl http://127.0.0.1:9100/metrics`, scrape config, PromQL examples
- Phase 53 metric family deep-dive ✓

**Needed addition:** New subsection between "Health Checks" and "Prometheus Metrics", titled `### In-Binary Metrics Page`. Content (per criterion #4 "one short paragraph", "no third-party software as prerequisite"):

```markdown
### In-Binary Metrics Page

Once the admin listener is enabled, point a browser at `http://127.0.0.1:9100/`
to see a live, self-contained HTML page summarising tool RED metrics, lspool
worker state, circuit breakers, recent eviction reasons, repomap cache
hit-rate, and process RSS / goroutines. The page reads the same registry
exposed at `/metrics` -- press F5 to refresh. No Prometheus, Grafana, or
Docker required. For triage walkthroughs, see [`docs/runbooks/`](./docs/runbooks/).
```

The existing "Prometheus Metrics" + scrape config + PromQL section can stay as-is — it's an optional remote-monitoring path for users who want central aggregation. Criterion #4 says "no third-party software is mentioned as a prerequisite" — the new paragraph satisfies that; the later Prometheus section is clearly an optional add-on, not a prerequisite for the page.

## Sources

### Primary (HIGH confidence)
- `internal/daemon/telemetry.go` (lines 19-103) — admin listener implementation, exact mount point for new `/` route
- `internal/obs/metrics.go` (lines 1-298) — registry construction, all metric families, helper signatures
- `internal/obs/obs.go` (lines 1-95) — `Provider.Metrics().Registry()` accessor chain, `Noop()` constructor for tests
- `internal/daemon/telemetry_test.go` (lines 1-243) — `httptest` test pattern this phase will mirror
- `docs/runbooks/*.md` (5 files) — verified content
- `USAGE.md` (lines 606-700) — current Observability Quickstart section
- `go.mod` — confirmed `client_golang v1.23.2` and `client_model v0.6.2` already direct deps
- `internal/cli/status_output.go` — existing CLI styling convention (no HTML conventions exist; the page sets the precedent)

### Secondary (MEDIUM confidence)
- prometheus/client_model README — `dto.MetricFamily` shape and `GetType()`/`GetMetric()`/`GetCounter()`/`GetHistogram()` API [CITED]
- `prometheus.Gatherer` interface contract — partial gather is valid, returns `(families, err)` [CITED: prometheus/client_golang godoc]

### Tertiary (LOW confidence)
- None — every claim is backed by code in this repo or by directly cited Prometheus types.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dep is already in go.mod and `Gather()` is a stable Prometheus API since v1.0.
- Architecture: HIGH — mount point and registry accessor both confirmed in code.
- Pitfalls: HIGH — bucket-quantized p95, `ServeMux` `/` semantics, partial-gather, header-after-write, Windows process metrics are all well-known Go/Prometheus traps.
- Runbook verification: HIGH — read all 5 files end-to-end, confirmed compliance with criterion #3.

**Research date:** 2026-04-28
**Valid until:** 2026-05-28 (stable APIs, internal codebase pinned to specific commits)
