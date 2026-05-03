---
phase: 54-obs-dashboards-runbooks
reviewed: 2026-05-01T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - USAGE.md
  - deploy/grafana/helix-engine.json
  - deploy/grafana/helix-overview.json
  - docs/runbooks/ErrCircuitOpen.md
  - docs/runbooks/deadline-timeouts.md
  - docs/runbooks/ls-crash-restart.md
  - docs/runbooks/memory-pressure-eviction.md
  - go.mod
  - internal/obs/dashboards_test.go
findings:
  critical: 0
  warning: 3
  info: 6
  total: 9
status: issues_found
---

# Phase 54: Code Review Report

**Reviewed:** 2026-05-01
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Phase 54 ships PromQL-validated dashboards, four runbooks, USAGE.md insertions, and a new direct dep on `prometheus/prometheus v0.311.3` for parser-only use. The validator test (`internal/obs/dashboards_test.go`) is solid: parser API choice, runtime-family allowlist, histogram suffix stripping, drift-detection self-test, and fail-closed empty-dir test are all correct. Anchor verification against the live tree confirms 19/20 cited file:line references in the runbooks land on the named symbol. One anchor in `deadline-timeouts.md` is wrong, and a few smaller issues with dashboard panel modeling, suffix-stripping edge cases, and template-variable consistency exist. No critical defects, no security issues, no data loss risks.

## Warnings

### WR-01: Stale USAGE.md anchor in deadline-timeouts runbook

**File:** `docs/runbooks/deadline-timeouts.md:60`
**Issue:** The runbook cites `USAGE.md:816-820 — operator-facing knob table mapping config keys to tool classes`. In the current USAGE.md, lines 816-820 are part of the `### Benchmarks` block ("Tips:" / `-short` / `GOMAXPROCS=4` / link to CONTRIBUTING.md). The actual Timeout Budgets table the runbook is describing lives at `USAGE.md:838-848` (heading `### Timeout Budgets` at line 838, table rows at 843-848). Operators following this code reference will land on the wrong section.
**Fix:** Update the anchor:
```markdown
- `USAGE.md:838-848` — operator-facing Timeout Budgets table mapping config keys to tool classes.
```

### WR-02: `$instance` template variable is wired but applied to only one panel

**File:** `deploy/grafana/helix-engine.json` (and `helix-overview.json`)
**Issue:** Both dashboards declare an `$instance` template variable backed by `label_values(up{job=~".*helix.*"}, instance)`. In `helix-engine.json` it is referenced on exactly one panel (`id: 1`, line 86: `helix_lspool_workers{language=~"$language", instance=~"$instance"}`). In `helix-overview.json` it is not referenced by any panel target. A picker that filters nothing on 8/9 engine panels and 0/6 overview panels will silently mislead operators in multi-instance deployments — selecting one instance still shows aggregated data from all instances.
**Fix:** Either add `, instance=~"$instance"` to every panel's `expr` (consistent), or remove the `$instance` variable from the overview dashboard and document that engine panel 1 is the only instance-filtered chart. The phase context (54-RESEARCH §A.4) explicitly calls out instance filtering as a feature, so the additive fix is preferable. Minimal patch for engine panel 2:
```json
"expr": "sum by (reason) (rate(helix_lspool_evictions_total{language=~\"$language\", instance=~\"$instance\"}[5m]))"
```
Repeat for panels 3-9 and all overview panels. Note that `helix_session_lifecycle_total` and `helix_edit_outcome_total` carry an `instance` label by virtue of Prometheus scrape attribution, so the filter is valid PromQL and the validator allowlists `instance` (`dashboards_test.go:194`).

### WR-03: `stripHistogramSuffix` over-trims any metric ending in `_count` / `_sum` / `_bucket`

**File:** `internal/obs/dashboards_test.go:54-61`
**Issue:** The function unconditionally strips `_bucket` / `_count` / `_sum` suffixes. Today the only Helix histogram is `helix_repomap_extract_duration_seconds`, so the function is correct for the current metric set. But a future Helix counter or gauge whose name happens to end in one of those suffixes (e.g., a hypothetical `helix_foo_sum_total` would be safe, but `helix_messages_count` — a plausible counter name — would silently be re-mapped to `helix_messages` and a missing-family bug would slip through). The validator's job is precisely to catch typos and missing registrations; a heuristic that mutates names before the registry check weakens that contract.
**Fix:** Only strip the suffix when it's a recognized histogram base. Cheapest implementation:
```go
func stripHistogramSuffix(name string, families map[string]bool) string {
    for _, sfx := range []string{"_bucket", "_count", "_sum"} {
        if strings.HasSuffix(name, sfx) {
            base := strings.TrimSuffix(name, sfx)
            if families[base] {
                return base
            }
        }
    }
    return name
}
```
Pass `families` from `problemsForExpr`. Names that look like histogram suffixes but aren't will then fail the registry check loudly, exactly as intended.

## Info

### IN-01: PromQL fenced-block comment-strip splits on `\n\n` before stripping

**File:** `internal/obs/dashboards_test.go:163-178`
**Issue:** `extractPromQLFromRunbook` splits each fenced block on blank lines first, then strips comment-only lines. A fenced block containing `# Phase 53 D-01: lspool cache hit-ratio (5-minute window).\n# Closer to 1.0 = ...\nsum(rate(...))` (pure comments above the query, no blank line between them) is correctly handled because the comments are on the same `\n\n`-delimited block and get stripped before `parser.ParseExpr`. But a query like `# header\n\n# more\nrate(foo[5m])` would split into three blocks (`# header`, `# more`, `rate(foo[5m])`), strip the first two to empty, parse the third — also fine. There is no bug, but the doc comment at line 154-155 says "comment-only lines are stripped before parsing" which is accurate yet glosses over the `\n\n` split contract. The current pass is a useful invariant to call out for future authors.
**Fix:** Add a sentence to the function-doc comment:
```go
// blank-line-separated queries; comment-only lines (lines whose trim starts
// with `#`) are stripped before parsing. Fenced blocks may freely mix
// `# comment` lines and queries — the `\n\n` block split runs FIRST and
// purely-comment blocks become empty after the strip and are skipped.
```

### IN-02: Engine `barchart` panel uses `range: true` against a `rate()` query

**File:** `deploy/grafana/helix-engine.json:319-326` (panel 9 "Rename strategy share")
**Issue:** Bar charts in Grafana traditionally render an instant snapshot. The current target sets `range: true, instant: false`, so the panel will render a stacked time-series rendered as bars — usable, but not idiomatic for a "share" view. The other stat panels correctly use `instant`-equivalent semantics via `reduceOptions.calcs=["lastNotNull"]`.
**Fix:** Change to `range: false, instant: true` for the rename strategy bar chart, and let Grafana render a single bar per strategy averaged over `[5m]`.

### IN-03: Text-panel `id: 99` is a non-sequential magic number

**File:** `deploy/grafana/helix-overview.json:238`
**Issue:** Panel ids 1-6 are sequential, then panel 99 jumps for the explanatory text panel. Grafana imports do not require sequential ids, but the gap makes it hard for reviewers to grep "what is panel 7?" in future edits, and Grafana's "Reload dashboard" UI sometimes assigns new ids on import that conflict with the gap.
**Fix:** Rename to `"id": 7`.

### IN-04: `parser.NewParser(parser.Options{})` is constructed per-expression

**File:** `internal/obs/dashboards_test.go:207`
**Issue:** A new `Parser` is allocated on every `problemsForExpr` invocation. Cost is trivial for ~50 expressions, but a `var pool = sync.Pool{New: ...}` or a single per-test `parser.NewParser` reused across calls would mirror the upstream Prometheus pattern. Not a correctness issue.
**Fix:** None required for v1. Future cleanup: reuse a `parser.Parser` per-test if the validator grows past hundreds of queries.

### IN-05: `dashFile.Type` field on `dashPanel` is read but never inspected

**File:** `internal/obs/dashboards_test.go:110`
**Issue:** `dashPanel.Type` is declared and decoded from JSON but never used by `walk` or downstream logic. It was presumably added for a "skip text panels" short-circuit that was never implemented (text panels are correctly handled by the `Targets` being nil/empty, so the field is dead).
**Fix:** Remove the field, or wire it into a `if p.Type == "text" { continue }` early-out for clarity:
```go
type dashPanel struct {
    Targets []target    `json:"targets,omitempty"`
    Panels  []dashPanel `json:"panels,omitempty"`
}
```

### IN-06: `prometheus/prometheus v0.311.3` brings a heavy module graph for parser-only use

**File:** `go.mod:18`
**Issue:** The new direct dep pulls `github.com/grafana/regexp`, `github.com/dennwc/varint`, `github.com/cenkalti/backoff/v5`, `github.com/grpc-ecosystem/grpc-gateway/v2`, and a handful of TSDB-adjacent indirect deps (visible in the `// indirect` block lines 58-108). The package is only used for `promql/parser` from a _test.go file. Module-graph weight matters for build determinism and supply-chain surface area.
**Fix:** None required for v1 — Prometheus does not publish a parser-only module, and the alternative (vendoring the parser or hand-rolling one) is worse. Track for v1.x as a "consider extracting parser into a build-tagged tooling module" item; document that all `prometheus/prometheus` import surface is confined to `_test.go` files via `go vet ./... | grep prometheus/prometheus` to stop production code from accidentally depending on it.

---

_Reviewed: 2026-05-01_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

## REVIEW COMPLETE

- Critical: 0
- Warning: 3
- Info: 6
- Total: 9
