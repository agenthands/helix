---
phase: 53-obs-metrics-gaps
plan: 03
subsystem: observability
tags: [observability, repomap, metrics, prometheus, sink-interface, helix]

# Dependency graph
requires:
  - phase: 53-obs-metrics-gaps
    plan: 01
    provides: "*obs.Metrics.RepoMapLookup and *obs.Metrics.RepoMapExtractObserve helper methods + helix_repomap_lookups_total + helix_repomap_extract_duration_seconds vectors"
  - phase: 11
    provides: "lspool.MetricsSink template (interface + NoopSink + closed-enum constants); the verbatim shape this plan mirrors"
provides:
  - "internal/repomap/metrics.go declares MetricsSink (RepoMapLookup, RepoMapExtractObserve) + LookupHit/Miss + ExtractorTreesitter/LSP/Fallback constants + NoopSink"
  - "internal/repomap/cache.go: TagCache.metrics field + SetMetricsSink setter + hit/miss emission at GetOrExtract D-03 boundaries"
  - "internal/skill/repomap/skill.go: per-extractor latency observation around all three GetOrExtract dispatcher branches (Q-2 Option 2)"
  - "internal/repomap/render.go: documentation block pinning the Q-2 Option 2 invariant (no-op extractFn does NOT muddy the histogram)"
  - "internal/daemon/daemon.go 12d post-init wiring block: rs.SetMetricsSink + rs.Cache().SetMetricsSink"
  - "internal/daemon/wiring_test.go: var _ repomap.MetricsSink = (*obs.Metrics)(nil) compile-time assertion + TestObsMetricsIsRepoMapSink runtime companion"
affects:
  - "Phase 54 (OBS-01/OBS-02 dashboards): repomap hit-rate and per-extractor latency PromQL queries are now backed by emitted vectors"
  - "Phase 55 (OBS-04 trace audit): repomap dispatcher timing is observable; trace coverage can correlate with histogram tail"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Setter-pattern post-init wiring (12d block) — chosen over a NewTagCache constructor argument; mirrors 12a/12b/12c"
    - "Sink-interface + NoopSink + nil-normalization in setter (preserves D-15 — repomap does not import internal/obs)"
    - "Q-2 Option 2: cache.go owns lookup hit/miss; skill.go dispatcher owns per-extractor latency (extractor type known only at dispatch layer)"
    - "metricsSink() helper on *RepoMapSkill normalizes nil to NoopSink{} so test-constructed skills bypassing Init never panic"

key-files:
  created:
    - internal/repomap/metrics.go
    - internal/repomap/metrics_test.go
    - internal/skill/repomap/metrics_emit_test.go
  modified:
    - internal/repomap/cache.go
    - internal/skill/repomap/skill.go
    - internal/repomap/render.go
    - internal/daemon/daemon.go
    - internal/daemon/wiring_test.go

key-decisions:
  - "Setter pattern (SetMetricsSink) chosen for both *TagCache and *RepoMapSkill instead of widening NewTagCache's constructor — minimizes call-site churn and matches the existing 12a/12b/12c daemon post-init shape"
  - "Q-2 Option 2 locked: cache.go emits lookup-only (hit/miss); skill.go dispatcher emits per-extractor latency. The extractor type is known only at the dispatcher; cache.go knows only that extractFn ran"
  - "*RepoMapSkill.Cache() accessor was already present (line 97 of skill.go) — no new accessor needed for the daemon 12d wiring block"
  - "render.go's no-op extractFn is left unchanged (documentation-only edit). Cache lookup hit/miss WILL fire from cache.go on its calls, but the histogram stays clean because no real extractor ran"
  - "Failed extractions (acquire error, read error, extract error) still record their latency observation per D-07 — the sink call sits AFTER the inner work in every branch"

patterns-established:
  - "Plan 53-03 setter pattern: when wiring a sink into an existing struct constructed via NewXxx(), prefer SetMetricsSink(nil-normalized to NoopSink{}) over widening NewXxx — keeps pre-existing callers untouched and matches the post-init wiring block in daemon.go"
  - "Sink resolution at dispatch entry: capture s.metricsSink() once outside the inner GetOrExtract closure so SetMetricsSink races during walkAndExtract see a stable reference"

requirements-completed: [OBS-03]

# Metrics
duration: 7min
completed: 2026-04-30
---

# Phase 53 Plan 03: RepoMap MetricsSink wiring Summary

**RepoMap cache hit-rate counter and per-extractor latency histogram emit from production code paths via a Phase 11-style sink interface; *obs.Metrics is bound at compile time to repomap.MetricsSink and operationally wired in daemon post-init 12d.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-30T17:10:08Z
- **Completed:** 2026-04-30T17:17:07Z
- **Tasks:** 4
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments

- New `internal/repomap/metrics.go` declares `MetricsSink` (closed-enum surface) + `NoopSink` + `LookupHit/Miss` and `ExtractorTreesitter/LSP/Fallback` constants. D-15 lock preserved: zero `internal/obs` imports in repomap.
- `TagCache.GetOrExtract` emits exactly two `c.metrics.RepoMapLookup` call sites — `LookupHit` at the mtime-match branch (cache.go:109) and `LookupMiss` at the extractFn-invocation branch (cache.go:121). Closes the cache-hit-rate half of OBS-03.
- `internal/skill/repomap/skill.go` dispatcher wraps each of the three `GetOrExtract` extractor branches with `time.Now/time.Since` and emits `RepoMapExtractObserve(lang, extractor, seconds)` per branch (treesitter / lsp / fallback) — closes the extraction-latency half of OBS-03 at the layer where the extractor type is known (Q-2 Option 2).
- `internal/repomap/render.go` documentation comment block pins the rule that the no-op extractFn caller does NOT trigger an extract observation — important so future changes don't muddy the histogram with 0s.
- `internal/daemon/daemon.go` 12d block wires both seams (`rs.SetMetricsSink` and `rs.Cache().SetMetricsSink`) using the post-init pattern; `internal/daemon/wiring_test.go` adds the compile-time assertion `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` and runtime companion `TestObsMetricsIsRepoMapSink`.

## Architectural Decision (locked at execution start)

**Setter pattern (NOT constructor change).** Per the worktree pre-decision and Wave 2 anti-pattern guidance, this plan uses `SetMetricsSink(sink)` setters on both `*TagCache` and `*RepoMapSkill`. Rationale:

- Adding a sink argument to `NewTagCache(dbPath string)` would force every existing caller (`internal/skill/repomap/skill.go` Init at line 79, every test using `repomap.NewTagCache`, future callers in fallback/render) to thread the dependency through. The setter route changes only the daemon post-init wire-up.
- Matches the existing `SetEnrichFn` / `SetFallbackDeps` / `SetRegistry` shape on `*RepoMapSkill`, keeping the 12a/12b/12c/12d wiring block uniform.
- Allows the never-wired tests (e.g. `cache_test.go::TestTagCache_GetOrExtract_CacheMiss`) to continue functioning unchanged thanks to the nil → `NoopSink{}` normalization in both `NewTagCache` (initialized at construction) and `SetMetricsSink` (re-normalized on every call).

The decision was recorded **before writing code**, then implemented as such. Both `SetMetricsSink` methods normalize a nil argument to `NoopSink{}` to prevent test-constructed structs (e.g. `&RepoMapSkill{}` bypassing `Init`) from panicking on emission.

## Cache() accessor

Already present at `internal/skill/repomap/skill.go:97` (existed for `SetEnrichFn`'s LSP enrichment seam). The daemon 12d block reuses it — **no new accessor needed**.

## render.go no-op extractFn → no histogram pollution

Confirmed: `internal/repomap/render.go:127-129` calls `r.cache.GetOrExtract(filePath, func() ([]Tag, error) { return nil, nil })`. With Plan 53-03 wiring:
- The cache's `LookupMiss` fires from `cache.go:121` because cache.go does not know about the no-op nature of the closure — that's correct, the cache *did* miss.
- Crucially, `RepoMapExtractObserve` does **not** fire because render.go's no-op closure runs inside `cache.GetOrExtract`, not in the `walkAndExtract` dispatcher in `skill.go`. The observation lives only in the dispatcher's instrumented closure.

This invariant is pinned by `TestDispatcher_RenderNoOpExtractFn_DoesNotEmitObserve` in `internal/skill/repomap/metrics_emit_test.go`, which exercises the exact same no-op extractFn shape and asserts `len(extracts) == 0` while `len(lookups) == 1` (the miss).

## Test results

| Task | Tests added | Suite result |
| ---- | ----------- | ------------ |
| 3.1 | TestMetricsSink_NoopSinkSatisfiesInterface, TestNoopSink_safe, TestMetricsSink_LookupResultConstants, TestMetricsSink_ExtractorConstants (4 new) | `go test ./internal/repomap/...` PASS |
| 3.2 | TestTagCache_GetOrExtract_LookupEmission/{hit_branch_emits_hit, miss_branch_emits_miss, nil_sink_default_is_safe} (3 sub-tests) | `go test ./internal/repomap/...` PASS |
| 3.3 | TestDispatcher_TreesitterBranch_ObservesTreesitter, TestDispatcher_LSPBranch_ObservesLSP, TestDispatcher_LSPBranch_ObservesLSPEvenOnAcquireError, TestDispatcher_FallbackBranch_ObservesFallback, TestDispatcher_NilSinkSafe, TestDispatcher_RenderNoOpExtractFn_DoesNotEmitObserve (6 new) | `go test ./internal/skill/repomap/...` PASS |
| 3.4 | TestObsMetricsIsRepoMapSink (1 new) + compile-time assertion | `go test ./internal/daemon/...` PASS; existing TestObsMetricsIsLSPoolSink regression PASS; `go build ./cmd/helix` clean |

Final regression: `go test ./internal/repomap/... ./internal/skill/repomap/... ./internal/obs/... ./internal/daemon/...` → all four packages PASS. `go vet` clean across all four packages.

## Task Commits

Each task committed atomically with `--no-verify` (parallel-execution rule):

1. **Task 3.1: repomap.MetricsSink interface + NoopSink + constants + recordingSink test fixture** — `a69cbe7a` (feat)
2. **Task 3.2: TagCache.metrics field + GetOrExtract hit/miss emission + SetMetricsSink setter** — `d9487b08` (feat)
3. **Task 3.3: dispatcher per-extractor latency observation + render.go Q-2 Option 2 doc + metrics_emit_test.go** — `7a87f286` (feat)
4. **Task 3.4: daemon 12d post-init wiring block + wiring_test.go compile-time assertion + runtime companion** — `5b498691` (feat)

A docs-level commit folding in this SUMMARY follows after self-check.

## Files Created/Modified

- `internal/repomap/metrics.go` (NEW) — MetricsSink interface, LookupHit/Miss, ExtractorTreesitter/LSP/Fallback constants, NoopSink with compile-time assertion.
- `internal/repomap/metrics_test.go` (NEW) — interface satisfaction, NoopSink safety, constants pinning, recordingSink/snapshot test fixture, hit/miss/nil-sink emission tests.
- `internal/repomap/cache.go` (MODIFIED) — added `metrics MetricsSink` field, `SetMetricsSink` setter with nil normalization, hit/miss emission at lines 109 and 121 (the canonical D-03 boundaries).
- `internal/repomap/render.go` (MODIFIED) — documentation block above the no-op extractFn caller pinning Q-2 Option 2 (no behavior change).
- `internal/skill/repomap/skill.go` (MODIFIED) — added `metrics repomap.MetricsSink` field, `SetMetricsSink` setter, `metricsSink()` nil-normalizing accessor, and `time.Now/time.Since` wrapping around all three dispatcher branches with `RepoMapExtractObserve` emission per branch.
- `internal/skill/repomap/metrics_emit_test.go` (NEW) — six dispatcher branch tests covering treesitter, LSP, LSP-on-acquire-error, fallback, nil-sink safety, and the render.go no-op invariant.
- `internal/daemon/daemon.go` (MODIFIED) — new 12d post-init wiring block binding `*obs.Metrics` to both repomap seams.
- `internal/daemon/wiring_test.go` (MODIFIED) — added `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` compile-time assertion and `TestObsMetricsIsRepoMapSink` runtime companion exercising every closed-enum constant.

## Decisions Made

1. **Setter pattern over constructor change** — see Architectural Decision section above. Both `TagCache.SetMetricsSink` and `RepoMapSkill.SetMetricsSink` exist; neither `NewTagCache`'s nor any constructor's signature changed.
2. **Default-to-NoopSink at three layers** — (a) `NewTagCache` initializes `&TagCache{db: db, metrics: NoopSink{}}`, (b) `Init()` sets `s.metrics = repomap.NoopSink{}`, (c) `metricsSink()` accessor normalizes nil to `NoopSink{}` for test-constructed skills bypassing `Init`. Defense in depth — any path can be safe regardless of construction route.
3. **Capture sink reference at walkAndExtract entry** — `sink := s.metricsSink()` is resolved once per file outside the inner `GetOrExtract` closure so a `SetMetricsSink` race during walkAndExtract sees a stable reference (no half-applied sinks across branches in the same file).
4. **Observe AFTER inner work in every branch** — every branch calls `sink.RepoMapExtractObserve(...)` at the END of its work (after extract / acquire / read), so failed extractions still record their latency. D-07 is preserved.
5. **render.go is documentation-only** — the no-op extractFn caller is intentionally left unchanged. Adding a `RepoMapExtractObserve` here would emit 0s into the histogram and ruin its diagnostic value. Pinned by `TestDispatcher_RenderNoOpExtractFn_DoesNotEmitObserve`.

## Deviations from Plan

None — plan executed exactly as written.

The plan's Task 3.3 action block flagged that the dispatcher branches "may already be factored into helper methods" and that the example shape was illustrative; in practice the three branches were inline inside the existing `walkAndExtract` closure, so the implementation followed the illustrative shape directly with no structural deviation. The `Cache()` accessor was already present (anticipated as possible by the plan), so no new accessor was added.

## Issues Encountered

- **Worktree base mismatch at startup** — initial `git merge-base` was `c1a6cf5526aa` (a different prior phase 53 commit). Resolved per the `<worktree_branch_check>` step by `git reset --hard 5136b391029a52da0598371862f63201d570574a` (the post-Plan 53-02 merge), then verifying HEAD matched. Hard-reset succeeded on the first attempt.
- **Pre-existing graphify cgo build error** — `go build ./...` reports a cgo error in `tmp/graphify/tests/fixtures/sample.c`. Unrelated to this plan; the four target packages (`./internal/repomap/...`, `./internal/skill/repomap/...`, `./internal/obs/...`, `./internal/daemon/...`) and `./cmd/helix` build cleanly. Not in scope of Plan 53-03.
- **Pre-existing Swift tree-sitter cgo warning** — `TOKEN_COUNT macro redefined`. Pre-existing on `main`; unaffected by this plan's changes.

## User Setup Required

None — observability vectors emit automatically once the daemon is started; operators can scrape `/metrics` to see the new `helix_repomap_lookups_total{language,result}` and `helix_repomap_extract_duration_seconds{language,extractor}` series after walkAndExtract runs once.

## Next Phase Readiness

- Wave 2 sibling Plan 53-05 (next) shares `internal/daemon/daemon.go` and will wire its session-lifecycle counter; the 12d block lives between 12c and the existing middleware install (line 326). No structural conflict expected — Plan 53-05 should append a 12e block or instrument the forwarder handler/HTTP transport directly.
- Phase 54 dashboards (OBS-01/OBS-02) can now consume:
  - Hit-rate: `sum(rate(helix_repomap_lookups_total{result="hit"}[5m])) / sum(rate(helix_repomap_lookups_total[5m]))` (per-language with `by (language)`).
  - Per-extractor p95: `histogram_quantile(0.95, sum(rate(helix_repomap_extract_duration_seconds_bucket[5m])) by (le, extractor))`.
- Phase 55 trace audit can correlate trace spans for repomap operations with these histogram emissions.

## Self-Check: PASSED

Verified post-write:
- `internal/repomap/metrics.go` — exists ✓
- `internal/repomap/metrics_test.go` — exists ✓
- `internal/skill/repomap/metrics_emit_test.go` — exists ✓
- Commits exist: `a69cbe7a`, `d9487b08`, `7a87f286`, `5b498691` — all in `git log` ✓
- `go build ./cmd/helix` — exits 0 ✓
- `go test ./internal/repomap/... ./internal/skill/repomap/... ./internal/obs/... ./internal/daemon/...` — PASS ✓
- `go vet` — clean (warnings limited to pre-existing Swift cgo macro redefinition) ✓
- D-15 lock — `! grep -E '^\s*"github.com/agenthands/helix/internal/obs"' internal/repomap/cache.go internal/repomap/metrics.go internal/repomap/render.go` ✓
- Compile-time assertion `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` present in `wiring_test.go` ✓
- Setter pattern (NOT constructor change) used — verified by grep `grep 'func NewTagCache' internal/repomap/cache.go` returns the pre-existing single-arg signature ✓

---
*Phase: 53-obs-metrics-gaps*
*Plan: 03*
*Completed: 2026-04-30*
