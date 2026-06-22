---
phase: 89-reports-ci-policy-contamination-canary
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - bench/aggregator/aggregate.go
  - bench/aggregator/report.go
  - bench/aggregator/load.go
  - bench/runtime/prompt_inject.go
  - cmd/helix-bench/report.go
  - .github/workflows/bench.yml
  - bench/canary/canary.go
findings:
  critical: 1
  warning: 3
  info: 3
  total: 7
status: issues_found
---

# Phase 89: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 89 is the v1.12 publication capstone: aggregate-time verified_correctness /
cost_per_solved leaderboard columns, per-language + ablations renderers, the
cost/quality ASCII scatter, the contamination-canary aggregate-time exclusion +
footnote, the `report --run-id` operator command, the shared `renderAll` byte path,
and the CI cost-policy workflow.

The path-traversal mitigation (`isValidRunID`), the BCa ordering guard, the
pass@k k>n domain guard, the fail-closed Load/cost-table gates, the atomic
writeReport, the deterministic sort-before-emit renderers, and the pure FNV-keyed
canary selector are all sound and well-guarded. The CI workflow is least-privilege,
hard-capped, secret-free on the PR path, and event-gated on the expensive path.

There is, however, **one BLOCKER on the single highest-risk concern named in the
review brief**: the contamination-canary exclusion is INCOMPLETE. The
`verified_correctness` headline leaderboard column — and the cost/quality scatter
and ablation deltas that derive from the same raw rows — do NOT route through
`cleanRows`, so a contaminated row's verdict silently inflates published headline
numbers. The exclusion was wired only into `reduceLeaderRow` and `reduceCostRow`;
the Phase 89 additive reduces that landed alongside it bypass it. The existing
exclusion test passes only because its fixture gives clean and dirty rows the same
verdict, masking the leak.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: `verified_correctness` headline column counts contaminated rows (canary exclusion bypassed)

**File:** `bench/aggregator/aggregate.go:543-558` (reduce) and `:139` (call site); rendered at `bench/aggregator/report.go:323,328`

**Issue:**
`reduceVerifiedCorrectness` reads `loaded.Rows(task, mode)` directly and pools over
EVERY row, with NO `cleanRows` split:

```go
func reduceVerifiedCorrectness(loaded *Loaded, tasks []string, mode string) ciValue {
	var trueCount, total int
	for _, task := range tasks {
		for _, r := range loaded.Rows(task, mode) { // <-- raw rows, no cleanRows
			v := r.Metrics.VerifiedCorrectness
			...
```

`verified_correctness` IS a rendered headline leaderboard column (`report.go:323`
header, `:328` cell). The review brief names it explicitly as one of the three
headline reduces (`pass@1, verified_correctness, cost`) that MUST exclude
contamination — and the code's own contamination footnote asserts it does:

```
report.go:368: "from the headline (pass@1, verified_correctness, cost_per_solved). "
```

But only `reduceLeaderRow` (aggregate.go:262) and `reduceCostRow` (aggregate.go:659)
call `cleanRows`. A fully contaminated task whose runs are stamped
`verified_correctness=true` is EXCLUDED from pass@1 and cost (correct) yet COUNTED in
the published verified_correctness column — the precise "contaminated data inflating
the headline" failure the canary exists to prevent.

The same raw value propagates into the REPORT-04 scatter via
`buildScatterPoints` (aggregate.go:623 reads `l.VerifiedCorrectness`), so the
published cost-vs-quality scatter inherits the contaminated y-axis too.

**Why the existing test misses it:** `TestCanaryExclusionFromHeadline`
(aggregate_canary_test.go:130-175) gives BOTH the clean task and the dirty task
`metricVC(true, true, ...)` — verified_correctness=true on every row. Pooled clean-only
= 1.0 and pooled all-rows = 1.0, so the assertion at line 157
(`InDelta(1.0, full.VerifiedCorrectness.Point)`) passes whether or not the dirty rows
are excluded. The test's own comment (lines 149-153) flags this masking for pass@1 but
the same defect silently neuters the verified_correctness assertion. A discriminating
fixture (clean task `verified_correctness=false`, dirty task `verified_correctness=true`)
would expose 0.0-correct vs 0.5-contaminated.

**Fix:** Route the verified_correctness reduce through the same `cleanRows` split the
other headline reduces use, and add a per-cell empty-clean fail-safe (mirroring
reduceLeaderRow:263). Sketch:

```go
func reduceVerifiedCorrectness(loaded *Loaded, tasks []string, mode string) ciValue {
	var trueCount, total int
	for _, task := range tasks {
		rows := loaded.Rows(task, mode)
		if len(rows) == 0 {
			continue
		}
		rows, _ = cleanRows(rows) // exclude contaminated rows from the headline
		for _, r := range rows {
			v := r.Metrics.VerifiedCorrectness
			if v == nil {
				continue
			}
			total++
			if *v {
				trueCount++
			}
		}
	}
	return pooledRate(trueCount, total)
}
```

Then strengthen `TestCanaryExclusionFromHeadline` with a discriminating verdict so the
exclusion is actually asserted (clean=false / dirty=true → headline must be 0.0, not 0.5).

## Warnings

### WR-01: Ablation `task_success` deltas count contaminated rows

**File:** `bench/aggregator/aggregate.go:565-578` (`successVectorForMode`), consumed by `reduceAblations` (`:588-602`)

**Issue:** `successVectorForMode` builds the per-task success vector from
`loaded.Rows(task, mode)` via `successCount` with NO `cleanRows` filter. `reduceAblations`
feeds those vectors into the published `ablations.md` `full_task_success` /
`other_task_success` CIs and their delta. A contaminated cell therefore inflates the
ablation table even though the same task_success metric IS excluded from the leaderboard
(reduceLeaderRow:262). This is the same class of integrity leak as CR-01 but on a
secondary published artifact, so it is a WARNING rather than a BLOCKER — fix it in the
same pass.

**Fix:** Apply the `cleanRows` split inside `successVectorForMode` before `successCount`:

```go
rows := loaded.Rows(task, mode)
if len(rows) == 0 {
	continue
}
present = true
rows, _ = cleanRows(rows)
c, n := successCount(rows)
if n > 0 {
	vec = append(vec, float64(c)/float64(n))
}
```

Note `present` must still be set from the pre-filter row existence so a fully-contaminated
mode does not vanish from the comparison (it should render an em-dash, not disappear).

### WR-02: Per-language pass-rate counts contaminated rows

**File:** `bench/aggregator/aggregate.go:327-368` (`reduceLanguageRows`)

**Issue:** `reduceLanguageRows` pools `successCount` over `loaded.Rows(task, mode)` with no
`cleanRows` split, so the published `per_language.md` pass_rate / n include contaminated
runs. Lower blast radius than CR-01 (per-language is a coarse pooled slice, not a headline
ranking column), but it is still a published number that a contaminated run can inflate,
and it is inconsistent with the leaderboard's exclusion. Classified WARNING.

**Fix:** Split per cell before bucketing by language:

```go
rows := loaded.Rows(task, mode)
rows, _ = cleanRows(rows)
perLang := map[string][]Row{}
for _, r := range rows {
	...
```

(Keep `reduceCanaryRate` deliberately reading ALL rows — it is the MEASUREMENT, correctly
excluded from this fix.)

### WR-03: `report --run-id` resolves `--out` with zero validation — traversal/abs-path vector survives via `--out`

**File:** `cmd/helix-bench/report.go:95`

**Issue:** `isValidRunID` correctly hardens `runID` against `..`/absolute/leading-`-`
(report.go:41-57, :92-94), but `runDir = filepath.Join(out, runID)` (line 95) takes `out`
straight from the `--out` flag with NO validation. A caller can pass
`--out ../../etc --run-id passwd` (or `--out /` ) and `Aggregate` will `MkdirAll` and write
the four report files anywhere the process can write (writeReport → os.MkdirAll + os.Rename,
report.go:682-705). The doc comment (report.go:26-29) advertises traversal protection but
only the `run-id` half is covered.

This is operator-supplied (lower severity than an untrusted-input traversal) and `--out`
defaulting to `bench/reports` is benign in normal use, so it is a WARNING — but the file is
explicitly the "path-traversal mitigation" surface, and an unvalidated `--out` undercuts the
stated V5 guarantee. At minimum the doc comment should not claim a value "can never escape
the report tree" while `--out` can relocate the whole tree.

**Fix:** Either (a) clean and constrain `out` (e.g. reject absolute paths / `..` segments, or
resolve against a fixed root), or (b) narrow the doc comment to state that only `run-id` is
validated and `--out` is a trusted operator root. If write-anywhere is intended for operators,
say so explicitly; do not advertise containment the code does not enforce.

## Info

### IN-01: bench.yml uses tag-pinned actions, not SHA-pinned

**File:** `.github/workflows/bench.yml:46,49,65,68`

**Issue:** `actions/checkout@v4` and `actions/setup-go@v5` are tag-pinned. The repo's
release/security-sensitive workflows (`release.yml`, `bench-mirror.yml`) SHA-pin
(`actions/checkout@11bd71901...`), and CLAUDE.md cites SHA-pinning as a supply-chain
discipline for the release pipeline. The brief asks specifically about "pinned action SHAs."
Within this repo, however, the non-release CI lane (`go-test.yml`, `codeql.yml`, `codespell.yml`)
all tag-pin, so bench.yml follows the established convention for its lane. Informational, with a
recommendation to SHA-pin for consistency with the security narrative.

**Fix:** Optional — SHA-pin both actions (with `# vN.N.N` trailing comment) to match
release.yml, or document that tag-pinning is acceptable for the hermetic, secret-free CI lane.

### IN-02: `reduceSwebenchScores` (raw/rescored) also bypasses cleanRows

**File:** `bench/aggregator/aggregate.go:508-531`

**Issue:** Like CR-01/WR-01/WR-02, `reduceSwebenchScores` pools over raw `loaded.Rows`. These
columns (RawScore/RescoredScore) are NOT currently rendered into leaderboard.md/cost_quality.md
(report.go renders neither), so there is no published-number leak today — hence INFO not WARNING.
But the moment a future phase renders them (the comments anticipate a "downstream SWE-bench render"),
the same contamination leak appears. Apply the `cleanRows` split here in the same sweep for
consistency, so the discipline is uniform across every score-time reduce except the deliberate
`reduceCanaryRate` measurement.

**Fix:** Same `cleanRows` split pattern as CR-01, applied per cell before counting.

### IN-03: Contamination footnote text over-claims relative to current behavior

**File:** `bench/aggregator/report.go:367-369`

**Issue:** The rendered footnote states excluded cells were removed
"from the headline (pass@1, **verified_correctness**, cost_per_solved)." Given CR-01, that
sentence is false for verified_correctness today: the cell is footnoted as excluded while its
verdict still feeds the published verified_correctness column. Once CR-01 is fixed the text
becomes accurate; if CR-01 is (incorrectly) deferred, this footnote is actively misleading and
should be corrected to match actual behavior. Tracked as INFO because it resolves automatically
when CR-01 lands.

**Fix:** Land CR-01 (preferred). If deferred, drop `verified_correctness` from the footnote
enumeration so the published claim is not false.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
