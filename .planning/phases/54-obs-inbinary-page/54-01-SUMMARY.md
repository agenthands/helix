---
phase: 54
plan: 01
subsystem: daemon-admin-listener
tags: [observability, admin-listener, html-template, prometheus-gatherer]
dependency_graph:
  requires:
    - "internal/obs.Provider.Metrics().Registry() (already shipped, plan 11-01)"
    - "internal/daemon admin listener at telemetry.go (already shipped, plan 10-OBS-03)"
  provides:
    - "GET / on the admin listener returning a self-contained HTML metrics page"
    - "buildPageModel: pure []*dto.MetricFamily → PageModel projection (testable)"
  affects:
    - "internal/daemon/telemetry.go (one mux.Handle line added)"
tech_stack:
  added: []
  patterns:
    - "html/template + //go:embed for inline-CSS template parsed once at init"
    - "bytes.Buffer pre-render so a template error returns 500 instead of half-written body"
    - "r.URL.Path != \"/\" narrowing to keep \"/\" from acting as a catch-all"
    - "p95 from histogram buckets (bucket-quantized; annotated on the page, not interpolated)"
key_files:
  created:
    - "internal/daemon/status_page.go"
    - "internal/daemon/status_page.html.tmpl"
    - "internal/daemon/status_page_test.go"
  modified:
    - "internal/daemon/telemetry.go"
decisions:
  - "Mounted the new route at \"/\" via mux.Handle (catch-all), then narrowed inside the handler with r.URL.Path != \"/\" — exact-match routes (/healthz, /readyz, /metrics, /debug/pprof/*) still win regardless of registration order, so the catch-all is harmless and keeps the handler self-contained."
  - "p95 is bucket-quantized (DefBuckets edges). Annotated on the page as '(approx, bucket-quantized)' rather than interpolating across buckets — interpolation would not be more accurate and would falsely imply precision."
  - "Auto-refresh is opt-out by default (no <meta http-equiv='refresh'>) per the literal reading of criterion #2 ('Refresh on the page = re-render')."
  - "Used google.golang.org/protobuf/proto for the test fixture builders since dto types are protobuf-generated and the proto helper package is already a transitive dep — no new go.mod entries."
metrics:
  duration_minutes: 22
  completed: 2026-04-28
  tasks_completed: 2
  files_changed: 4
requirements: [OBS-01]
---

# Phase 54 Plan 01: In-Binary Status Page Summary

Self-contained HTML metrics page on `GET /` of the existing admin listener; reads the same `*prometheus.Registry` that already serves `/metrics`, renders seven RED-style sections, zero new external dependencies.

## What Shipped

A new admin route `GET /` at the existing loopback admin listener (`127.0.0.1:9100` by default) returns a single inline-CSS HTML page summarising:

1. Tool RED — call counts and bucket-quantized p95 latency per tool
2. Edit outcomes by tool + outcome
3. LSP pool workers per language
4. Circuit state per language (closed / half-open / open)
5. Recent eviction reasons (only non-zero counters since process start)
6. Cache hit rate per (family, language) for `serena_lspool_cache_total` and `serena_repomap_cache_total`
7. Process RSS (human-formatted; `—` on Windows) and goroutine count

The handler, the pure projection function, and the embedded template all live in `internal/daemon/status_page.go` + `internal/daemon/status_page.html.tmpl`. The route is wired in `internal/daemon/telemetry.go` with one new line. The whole feature ships in 474 inserted lines across three files (handler, template, telemetry edit) with no `go.mod` or `go.sum` change.

## Tasks

| # | Type | Description | Commit |
|---|------|-------------|--------|
| 1 | test (RED) | Seven failing tests covering render, routing (404, 405), shared registry, refresh visibility, p95 annotation, empty-registry safety, and pure projection | `d9eda90c` |
| 2 | feat (GREEN) | `handleStatusPage` + `buildPageModel` + embedded template + `mux.Handle("/")` wiring | `6469ca12` |

## Verification

- `go test ./internal/daemon/ -run 'TestStatusPage|TestBuildPageModel' -count=1` — 7/7 pass
- `go test ./internal/daemon/ -count=1` — full daemon suite green (no regression in existing healthz/readyz/pprof/listener tests)
- `go test ./... -count=1 -short` — all packages green, no regressions
- `gofmt -l` on changed files — clean
- `go vet ./internal/daemon/...` — clean
- `git diff go.mod go.sum` — empty (criterion #5 satisfied)

## Acceptance Gate Results

| Gate | Result |
|------|--------|
| `! grep -E '(prometheus\.NewRegistry\|MustRegister)' internal/daemon/status_page.go` | PASS — only the read side of the registry; constructors stay in `internal/obs/` |
| `grep -q 'mux.Handle("/"' internal/daemon/telemetry.go` | PASS |
| `grep -q 'r.URL.Path != "/"' internal/daemon/status_page.go` | PASS — Pitfall #2 narrowing |
| `grep -q 'approx, bucket-quantized' internal/daemon/status_page.html.tmpl` | PASS — Pitfall #1 annotation |
| `! grep -E '(<script src=\|<link rel=\"stylesheet\" href=)' internal/daemon/status_page.html.tmpl` | PASS — criterion #1 satisfied |
| `git diff go.mod go.sum` empty | PASS — criterion #5 satisfied |

## Threat Mitigations Applied

- T-54-01 (XSS via label values) — `html/template` auto-escapes; no `template.HTML` use anywhere in the template.
- T-54-02 (MIME-sniff XSS) — `X-Content-Type-Options: nosniff` set on the response.
- T-54-03 (non-loopback access) — reuses existing `validateAdminAddr` gate at `telemetry.go:108`; no new bind, no new listener.
- T-54-05 (response splitting) — all response headers are static literals.
- T-54-06 (path traversal) — `r.URL.Path != "/"` returns 404.

## Deviations from Plan

None. The plan executed exactly as written, with two tiny notes for the record:

1. The plan's task 1 example construction snippet referred to `m.LspoolWorkers`/`m.EditOutcomes`; the actual exported field names on `*obs.Metrics` are `LSPoolWorkers` and `EditOutcome`. The implementation uses the real names. This is a plan-text typo, not a behavioural deviation.
2. The plan's `<interfaces>` block listed the `serena_tool_calls_total` label as `tool`; the actual registered label is `tool_name`. The implementation reads `tool_name`. The test fixture in `status_page_test.go` already uses the correct label, so no rework was needed.

Both notes are documentation-level mismatches that do not change the contract or the tests.

## Self-Check: PASSED

- File `internal/daemon/status_page.go` exists — FOUND
- File `internal/daemon/status_page.html.tmpl` exists — FOUND
- File `internal/daemon/status_page_test.go` exists — FOUND
- Commit `d9eda90c` exists — FOUND (RED, test fixtures + 7 failing tests)
- Commit `6469ca12` exists — FOUND (GREEN, handler + template + mux wiring)
- All 7 tests pass — VERIFIED
- `go test ./...` green — VERIFIED
- Zero go.mod / go.sum delta — VERIFIED
