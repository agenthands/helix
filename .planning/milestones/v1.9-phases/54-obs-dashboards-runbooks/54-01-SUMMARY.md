---
phase: 54-obs-dashboards-runbooks
plan: 01
subsystem: observability
tags: [observability, prometheus, promql, grafana, validation-test, wave-0]
requires:
  - phase 53 v1.2 metric inventory (12 helix_* families on internal/obs/metrics.go)
provides:
  - registry-driven PromQL validation gate for Wave 1+ dashboard/runbook content
  - .gitkeep tracking for deploy/grafana/, docs/runbooks/, docs/images/
  - prometheus/prometheus@v0.311.3 module dep (parser-only import surface)
affects:
  - go.mod / go.sum (+1 direct require, +97 go.sum lines)
  - internal/obs/ test package (3 new tests; existing tests unaffected)
tech-stack:
  added:
    - github.com/prometheus/prometheus v0.311.3 (promql/parser only)
  patterns:
    - registry-driven validation (mirrors metrics_labels_test.go::lintLabels)
    - pure-function problemsForExpr + thin t.Errorf wrapper (drift test asserts on slice length, not on a corrupted *testing.T)
    - env-var gated empty-dir fail-closed (Wave 0 escape; Wave 1 removes)
key-files:
  created:
    - internal/obs/dashboards_test.go (3 tests, ~330 LOC including header)
    - deploy/grafana/.gitkeep
    - docs/runbooks/.gitkeep
    - docs/images/.gitkeep
  modified:
    - go.mod (added prometheus/prometheus direct require)
    - go.sum (+97 lines for parser transitives)
decisions:
  - "D-01-A: Pin prometheus/prometheus at v0.311.3 (the latest stable Go-module-path tag) instead of the v3.11.0 named in the plan. Plan/RESEARCH assumed module-path migration to /v3 that has not happened upstream."
  - "D-01-B: Use parser.NewParser(parser.Options{}).ParseExpr instead of the package-level parser.ParseExpr documented in RESEARCH §C.2 — that function does not exist in v0.311.x. Default Options{} is sufficient for vanilla PromQL."
  - "D-01-C: Refactor to a pure-function problemsForExpr helper rather than calling t.Errorf inside a t.Run subtest in _catchesDrift. The plan-sketched `subT.Failed()` pattern fails because t.Run propagates child failure to the parent — the cleaner pattern is the metrics_labels_test.go::lintLabels approach."
  - "D-01-D: projectRoot uses ../../.. relative to filepath.Dir(testfile) (i.e., 2 ascents — `internal/obs` → repo root). Plan/RESEARCH wrote `../../..` which would walk one level above the repo root."
metrics:
  duration: ~25 min
  completed: 2026-05-01
  tasks_completed: 2
  files_created: 4
  files_modified: 2
  commits: 2
---

# Phase 54 Plan 01: Wave 0 PromQL Validation Gate Summary

**One-liner:** Wave 0 lands the Phase 54 contract — `prometheus/prometheus`@v0.311.3 dep added, `internal/obs/dashboards_test.go` shipped with three tests (validator, drift companion, empty-dir-fail-closed proof), three `.gitkeep` stubs track the new top-level dirs Wave 1 fills.

## What was done

- **Task 1 (commit `b96df92e`)**: added `github.com/prometheus/prometheus@v0.311.3` as a Go module dep. `go.sum` grew by +3 lines at this stage (the dep was indirect because nothing imported it yet). `make verify-embed-pubkey` confirmed unaffected (the target is a `cmp` of two pubkey files — module deps cannot affect it). `go vet ./internal/... ./cmd/...` green; `go build ./cmd/helix` green.
- **Task 2 (commit `ad271e81`)**: wrote `internal/obs/dashboards_test.go` (in-package, 3 tests + helpers), the three `.gitkeep` stubs, and ran `go mod tidy` which promoted prometheus/prometheus to a direct require (no `// indirect`) and pulled in the parser's transitive deps (`grafana/regexp`, `model/labels`, `util/features`, `promql/parser/posrange`, plus several `prometheus/*` siblings). Final go.sum delta vs. baseline (Task 1 start): +97 lines — under the 100-line flag threshold from RESEARCH §B.1.

## Verification results

| Gate | Result |
|------|--------|
| `go vet ./internal/... ./cmd/...` | ✅ green (only pre-existing `tmp/` fixture warnings unrelated to this plan) |
| `go build ./cmd/helix` | ✅ green (only the pre-existing benign `TOKEN_COUNT macro redefined` warning from the swift tree-sitter binding) |
| `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` | ✅ green — all three new tests + all pre-existing tests pass |
| `go test ./internal/obs/... -count=1 -run "^TestDashboardsAndRunbooksReferenceRegisteredMetrics$"` (no env var) | ✅ FAILS as required with stderr `no dashboards found at .../deploy/grafana/*.json — Wave 1 must populate deploy/grafana/. Set HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 only during Phase 54 Wave 0.` |
| `make verify-embed-pubkey` | ✅ green — Phase 52 audit-embed invariant preserved |
| `.gitkeep` stubs | ✅ all three present (`deploy/grafana/.gitkeep`, `docs/runbooks/.gitkeep`, `docs/images/.gitkeep`) |

## Validator behavior reference

The validator is a pure function `problemsForExpr(expr, families, source) []string` with a thin t.Errorf wrapper. For each VectorSelector in the parsed PromQL AST it:

- Strips histogram suffixes `_bucket`/`_count`/`_sum` to recover the base family name (so `helix_repomap_extract_duration_seconds_bucket` resolves to the registered base `helix_repomap_extract_duration_seconds`).
- Allowlists runtime-emitted families: `up` (Prometheus scrape-status), `go_*` (NewGoCollector), `process_*` (NewProcessCollector). The dashboards' `$instance` template variable queries `up{job=~".*helix.*"}`, so `up` MUST be allowlisted.
- For each label matcher: skips PromQL-internal label names `{__name__, le, job, instance}` (last one for the `$instance` template variable), then checks the name against `AllowedLabels` (metrics.go) and `carveOuts[base]` (metrics_labels_test.go). All other label names are reported.

## Test invocation modes (CRITICAL for Wave 1)

**Wave 0 mode (passes today):**
```bash
HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1
```

**Wave 1+ mode (intentionally fails until content lands; must be the default after Wave 1):**
```bash
go test ./internal/obs/... -count=1
```

> ⚠️  **WAVE 1 EXECUTORS — READ THIS FIRST.** The first action when shipping ANY dashboard JSON or runbook Markdown content is to **remove** the `os.Getenv("HELIX_DASHBOARDS_TEST_ALLOW_EMPTY") != "1"` clause from BOTH fatal checks in `internal/obs/dashboards_test.go::TestDashboardsAndRunbooksReferenceRegisteredMetrics`. Leaving the gate in place silently weakens the fail-closed contract — an accidentally-empty `deploy/grafana/` would no longer trip CI if the env var ever leaks into a workflow. The package-level comment in `dashboards_test.go` repeats this instruction loudly. The `TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty` test (in this file, independent of the env-var) pins the precondition that Wave 1 inherits a working contract.

## Deviations from Plan

### Auto-fixed issues

**1. [Rule 3 — Blocking] prometheus/prometheus version pin**

- **Found during:** Task 1 (first `go get`)
- **Issue:** Plan and RESEARCH §B.1 specified `github.com/prometheus/prometheus@v3.11.0`. The Go module proxy refused this version with `module path must match major version ("github.com/prometheus/prometheus/v3")`. Investigation: Prometheus has not migrated its `go.mod` module path to `/v3`; the v3.x marketing line ships under legacy `v0.3xx.x` Go module tags. RESEARCH §B.1 Assumption A4 was wrong about the module-path semantics.
- **Fix:** Pinned at `v0.311.3` — the latest stable Prometheus release on the legacy module path (released 2026-04 line). Same Apache-2.0 license, same parser-only import surface as the planned version. No semantic difference for this phase's use case.
- **Files modified:** `go.mod`, `go.sum`
- **Commit:** `b96df92e`
- **Wave 1 implication:** none — the parser API is identical at v0.311.x.

**2. [Rule 3 — Blocking] parser.ParseExpr does not exist in v0.311.x**

- **Found during:** Task 2 first test compile
- **Issue:** RESEARCH §C.2 and the plan's `<interfaces>` block documented `parser.ParseExpr(input string) (Expr, error)` as a package-level function. v0.311.x exports only the `Parser` interface and `NewParser(opts Options) Parser`. The package-level convenience function was removed during the v3 marketing rework. RESEARCH Pitfall #5 anticipated this need but called it out only as conditional ("Some Prometheus extensions ... require parser.NewParser").
- **Fix:** Use `parser.NewParser(parser.Options{}).ParseExpr(expr)`. Default `Options{}` is sufficient for the vanilla PromQL produced by Wave 1 (rate, sum, histogram_quantile, etc.). Comment in `validateExpr` documents the rationale and the experimental-features escape hatch.
- **Files modified:** `internal/obs/dashboards_test.go`
- **Commit:** `ad271e81`

**3. [Rule 1 — Bug] `_catchesDrift` subtest pattern propagates failure to parent**

- **Found during:** First Task 2 test run
- **Issue:** The plan sketch used `t.Run("driftSubtest", func(s *testing.T) { ...; subT = s })` then asserted `subT.Failed()`. This works to detect the failure but Go's `t.Run` propagates the child's `Failed()` state to the parent — the parent test reported FAIL even when the validator correctly flagged the drift.
- **Fix:** Refactored to mirror the in-repo precedent (`metrics_labels_test.go::lintLabels`): extracted the validator core to `problemsForExpr(expr, families, source) []string` (pure function, no `*testing.T`). `validateExpr` is now a thin `t.Helper`+`t.Error` wrapper. `_catchesDrift` calls `problemsForExpr` directly and asserts on `len(problems) > 0`. This also makes `_catchesDrift` more robust — it now also asserts the problem string mentions the offending metric name, mirroring `TestMetricsLabelsAllowlist_catchesDrift:155-163`.
- **Files modified:** `internal/obs/dashboards_test.go`
- **Commit:** `ad271e81`

**4. [Rule 1 — Bug] projectRoot path-ascent count**

- **Found during:** Sanity check before first test run
- **Issue:** Plan and RESEARCH §C.3 prescribed `filepath.Join(filepath.Dir(file), "..", "..", "..")` (three ascents). `filepath.Dir(internal/obs/dashboards_test.go)` = `internal/obs` — two segments deep. Three ascents would walk one directory ABOVE the repo root, breaking the `filepath.Glob` of `deploy/grafana/*.json`.
- **Fix:** Use two ascents (`..`, `..`) — `internal/obs` → `internal` → repo root. Verified by running the validator and confirming it finds (or correctly reports the absence of) `deploy/grafana/*.json`. RESEARCH §C.3's arithmetic was off by one.
- **Files modified:** `internal/obs/dashboards_test.go`
- **Commit:** `ad271e81`

### Auth gates

None.

## Dependency notes

- `go.sum` line growth (Task 1+Task 2 combined): +97 lines (259 → 356). Under the 100-line "document if greater than" threshold from RESEARCH §B.1 / Assumption A5.
- `go list -m all | grep prometheus/prometheus` returns `github.com/prometheus/prometheus v0.311.3` (no `// indirect`).
- No `tsdb`, `scrape`, `rules`, `web`, or `storage` subpackages were pulled in. The only new top-level Prometheus-org modules in the graph are the parser's direct transitives (`alertmanager`, `client_golang/exp`, `common/assets`, `exporter-toolkit`, `otlptranslator`, `sigv4`) — these are listed by `go list -m all` because `prometheus/prometheus`'s top-level `go.mod` requires them, but they are NOT linked into the helix binary because no helix code imports them. Tree-shaking at link time keeps the actual binary impact bounded.

## Self-Check: PASSED

- ✅ `internal/obs/dashboards_test.go` exists (verified)
- ✅ `deploy/grafana/.gitkeep`, `docs/runbooks/.gitkeep`, `docs/images/.gitkeep` exist (verified)
- ✅ commit `b96df92e` exists (`build(54-01): add github.com/prometheus/prometheus@v0.311.3 dep`)
- ✅ commit `ad271e81` exists (`test(54-01): add registry-driven PromQL validation test (Wave 0 gate)`)
- ✅ `go.mod` contains `github.com/prometheus/prometheus v0.311.3` as a direct require
- ✅ `go.sum` contains `github.com/prometheus/prometheus` (2 lines: `.h1:` hash + `/go.mod` hash)
- ✅ `make verify-embed-pubkey` exits 0
- ✅ `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` exits 0
- ✅ `go test ./internal/obs/... -count=1 -run "^TestDashboardsAndRunbooksReferenceRegisteredMetrics$"` (no env var) exits non-zero with `no dashboards found at`
