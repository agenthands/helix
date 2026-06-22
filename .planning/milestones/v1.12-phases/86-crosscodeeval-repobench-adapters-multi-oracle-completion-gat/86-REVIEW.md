---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
reviewed: 2026-06-21T04:53:50Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - bench/evaluators/exactmatch/exactmatch.go
  - bench/evaluators/editsim/editsim.go
  - bench/evaluators/identmatch/identmatch.go
  - bench/evaluators/completion_gate/gate.go
  - bench/datasets/crosscodeeval/fetch.go
  - bench/datasets/crosscodeeval/loader.go
  - bench/datasets/crosscodeeval/pin.go
  - bench/datasets/repobench/fetch.go
  - bench/datasets/repobench/loader.go
  - bench/datasets/repobench/pin.go
  - bench/datasets/repobench/score.go
  - bench/canary/canary.go
  - bench/aggregator/aggregate.go
  - bench/aggregator/report.go
  - cmd/helix-bench/main.go
findings:
  critical: 1
  warning: 4
  info: 5
  total: 10
status: issues_found
---

# Phase 86: Code Review Report

**Reviewed:** 2026-06-21T04:53:50Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Reviewed the Phase 86 completion-benchmark surface: the EM / edit-similarity /
identifier-match leaf scorers, the multi-oracle completion gate, the
CrossCodeEval + RepoBench dataset-loader adapters (with their SSRF-hardened HF
fetchers and arrow-go parquet decode), the contamination canary, the additive
aggregator `CanaryPassRate` column, and the `fetch-datasets` CLI wiring.

The SSRF posture of the fetchers is genuinely solid: pinned `Host`/`Repo`
constants, `isHexSHA1` rev shape-validation, `isValidHTTPSHost`, the
`validatePathSegment` gate before every `filepath.Join`, and an `io.LimitReader`
body cap with a one-byte-past-cap overflow probe. The scorers and canary carry
real hermetic fixture proof. `go vet` is clean across all packages.

One BLOCKER undermines the headline invariant the gate exists to protect: the
documented `DefaultESThreshold` is never applied, so the zero-value
`GateConfig{}` config silently disables the edit-similarity oracle (it passes on
any input), converting a documented fail-closed default into a fail-open one.
There is no test for the zero-value config path, so the hole is unproven.
Several WARNINGs concern robustness gaps in the loaders and an
overflow/length-skew edge in the RepoBench int column.

## Critical Issues

### CR-01: Documented `DefaultESThreshold` is never applied — zero-value `GateConfig{}` fails OPEN, silently disabling the edit-similarity oracle

**File:** `bench/evaluators/completion_gate/gate.go:103-114` (const at `:55`)
**Issue:**
`DefaultESThreshold = 0.9` is documented (gate.go:52-55, :64-66) as the default
the gate uses "when a caller does not override GateConfig.ESThreshold" and "when
zero-valued callers want the documented default." But `Grade` never consults it:

```go
ok := em && es >= cfg.ESThreshold && id
```

When a caller constructs `GateConfig{}` (the Go zero value, `ESThreshold == 0.0`),
the comparison becomes `es >= 0.0`, which is **always true** for every input
(ES is in [0,1]). The edit-similarity oracle is silently removed from the
all-three-required AND, collapsing a three-oracle gate into a two-oracle gate
(EM AND identifier-match) on the default config.

This is the worst-class benchmark defect the gate's own package doc warns
against: it is a path where `verified_correctness=true` can be produced more
easily than intended, because a real oracle was silently neutralized. It is not
a false `true` on the *abstain* path (that path is correct and well-tested), but
it is a fail-OPEN on the oracle path under the documented default config. The
`DefaultESThreshold` constant is currently dead code (no caller references it —
confirmed by grep), which is itself the symptom: the "default" was written but
never wired.

No test exercises `Grade(... , GateConfig{}, false)`; every gate_test.go case
passes an explicit `ESThreshold`, so the fail-open default path is entirely
unproven.

**Fix:** Apply the documented default at the top of `Grade` (mirroring the
aggregator's `Config.withDefaults`), and add a hermetic test that
`Grade(pred, gold, GateConfig{}, false)` rejects an ES-below-0.9 pair:

```go
func Grade(pred, gold string, cfg GateConfig, abstain bool) GateResult {
	if abstain {
		f := false
		return GateResult{VerifiedCorrectness: &f}
	}
	// Apply the documented default so a zero-value GateConfig{} does NOT
	// silently disable the ES oracle (es >= 0.0 is always true).
	if cfg.ESThreshold <= 0 {
		cfg.ESThreshold = DefaultESThreshold
	}
	em := exactmatch.EM(pred, gold)
	es := editsim.ES(pred, gold)
	id, _ := identmatch.Match(pred, gold)
	ok := em && es >= cfg.ESThreshold && id
	// ...
}
```

(If a caller legitimately wants threshold 0, that is indistinguishable from "not
set" with a bare `float64`; if that case must be supported, make `ESThreshold`
a `*float64` so "unset" and "explicitly 0" are distinct. Today no caller wants
0, so the floor above is the correct fail-closed behavior.)

## Warnings

### WR-01: RepoBench `gold_snippet_index` length-skew silently degrades a retrieval row to an invalid `-1`, hard-failing the whole decode

**File:** `bench/datasets/repobench/loader.go:212-215, 236-238`
**Issue:**
`golds` is read as an *optional* column via `int64Column(tbl, "gold_snippet_index")`
with the error discarded. If the column is absent (`golds == nil`) or merely
shorter than `tasks` (a chunked/truncated column), `gold` defaults to `-1` for
those rows. For a `TaskRetrieval` row that then trips the range check at :236
and returns a hard error aborting the **entire** parquet decode — one short/absent
int column kills all tasks for the language, including valid completion rows.
The string columns are tolerant of length skew via `stringAt`, but the int
column has no equivalent, so the two are asymmetric. A real per-language
RepoBench parquet that omits the column for completion-only rows would fail the
whole load rather than the offending row.

**Fix:** Either treat a missing/short `gold_snippet_index` on a *retrieval* row
as a per-row skip-or-error decision consistent with the string-column tolerance,
or — if a retrieval row genuinely cannot exist without a gold index — make the
column REQUIRED (`int64Column` error not discarded) so the failure mode is "this
parquet is malformed" rather than a silent per-row `-1`. At minimum, document
that `gold_snippet_index` must be present and aligned for any retrieval row, and
surface the `int64Column` error when the table claims to carry retrieval tasks.

### WR-02: `int(golds[i])` truncates int64→int on 32-bit platforms with no bounds note

**File:** `bench/datasets/repobench/loader.go:214`
**Issue:**
`gold = int(golds[i])` converts an untrusted parquet `int64` to a platform `int`.
On a 32-bit build a hostile/corrupt parquet carrying a gold index > 2^31-1
wraps to a negative or unrelated value. The downstream range check
(`GoldSnippetIndex < 0 || >= len(Context)`) catches an out-of-`Context` value, so
this does not currently cause an OOB index — but the truncation makes the error
message misleading (it reports the wrapped value, not the real one) and relies
on the range check as the sole guard. Given the package's stated "untrusted HF
bytes" threat model, the conversion should be explicit about its bound.

**Fix:** Guard the conversion before narrowing, e.g.:

```go
if i < len(golds) {
	g := golds[i]
	if g < 0 || g > int64(maxReasonableContextLen) {
		return nil, fmt.Errorf("repobench: %s row %d gold_snippet_index %d out of plausible range", language, i, g)
	}
	gold = int(g)
}
```

or compare `golds[i]` against `int64(len(t.Context))` directly in int64 space
before narrowing.

### WR-03: CrossCodeEval per-row `language` mismatch silently DROPS rows with no floor check, so a wholly-mismatched parquet yields the generic "zero tasks" error instead of a precise one

**File:** `bench/datasets/crosscodeeval/loader.go:136-161` (and repobench/loader.go:190-252)
**Issue:**
When the optional `language` column is present, rows whose `langs[i] != language`
are `continue`-skipped (loader.go:140-143). This is the documented mixed-fixture
behavior, but there is no guard distinguishing "this parquet legitimately has no
rows for the requested language" from "every row was skipped due to a
language-tag typo / wrong column semantics." Both collapse to the generic
`"%s parquet decoded zero tasks"` at :159-161. For a real single-language
parquet that (incorrectly) carries a `language` column tagged with an unexpected
value (e.g. `"py"` vs `"python"`), the loader silently discards 100% of rows and
reports a misleading empty-decode error, masking a tag-mismatch bug.

**Fix:** Track skipped-vs-kept counts and, when `kept == 0 && skipped > 0`,
return a distinct error naming the mismatch (e.g. `"all N rows carried language
%q, none matched requested %q"`) so a tag-semantics drift surfaces explicitly
rather than as a generic empty decode.

### WR-04: `fetch-datasets` cache-hit path returns stale/corrupt cached bytes with no integrity check and no way to force a refresh

**File:** `bench/datasets/crosscodeeval/fetch.go:99-101`, `bench/datasets/repobench/fetch.go:102-104`, `cmd/helix-bench/main.go:367-417`
**Issue:**
`Fetch` returns the cached file verbatim on any successful `os.ReadFile`
(`if b, err := os.ReadFile(dst); err == nil { return b, nil }`) with no
validation that the cached bytes are a well-formed parquet, the expected size,
or match the pinned rev's content. A truncated/partial prior write (e.g. a
process killed mid-`os.WriteFile`, which is not atomic here — unlike the
aggregator's temp+rename in report.go:351-368) leaves a corrupt cache file that
is then served forever as a "hit." The `fetch-datasets` command labels such a
read `OK (%d bytes)` regardless. There is no `--force` / cache-bust flag.

This is a correctness/robustness gap rather than a security one (the rev is
pinned so the *source* is trusted), but a corrupt cache silently feeds the
decoder, and the only recovery is manual cache deletion.

**Fix:** Write the cache via temp-file + atomic rename (reuse the
report.go:writeReport pattern) so a partial write never lands as a hit; and/or
validate the cached bytes decode before returning them on the hit path. Add a
`--force` flag to `fetch-datasets` that bypasses the cache-hit short-circuit.

## Info

### IN-01: `DefaultESThreshold` is dead code until CR-01 is fixed

**File:** `bench/evaluators/completion_gate/gate.go:55`
**Issue:** No code path references `DefaultESThreshold` (grep-confirmed). It is a
documented constant with no wiring — the direct symptom of CR-01. Fixing CR-01
makes it live; absent that, it is misleading documentation of behavior that does
not exist.
**Fix:** Wire it in `Grade` (see CR-01). Until then it should not be presented in
the doc comment as the operative default.

### IN-02: `cacheDir`, `cachePath`, `resolveURL`, `validatePathSegment`, `isHexSHA1`, `isValidHTTPSHost`, `Host`, `maxParquetBytes`, etc. are duplicated verbatim across the two adapter packages

**File:** `bench/datasets/crosscodeeval/{fetch.go,pin.go,loader.go}` vs `bench/datasets/repobench/{fetch.go,pin.go,loader.go}`
**Issue:** The two fetchers and pin guards are near-identical copies (the code
comments even cross-reference each other as "mirrors ... EXACTLY"). The leaf-
isolation rationale is documented, but the cost is that a future fix to one
(e.g. the WR-04 atomic-write fix, or a SSRF guard tightening) must be applied to
both by hand, and drift will not be caught by the compiler. This is an accepted
trade-off per the package docs, but worth flagging as a maintenance hazard.
**Fix:** Consider a shared `bench/datasets/internal/hffetch` leaf helper (still
stdlib+arrow only) that both adapters embed, so the SSRF/cache/limit logic lives
once. If the leaf-purity constraint forbids this, add a cross-package test that
asserts the two `validatePathSegment` / `isHexSHA1` / `isValidHTTPSHost`
implementations agree on a shared adversarial corpus, so drift is caught.

### IN-03: `coefVariation` uses `mean == 0` exact float compare to guard divide-by-zero

**File:** `bench/aggregator/report.go:156-158`
**Issue:** `if mean == 0 { return 0 }` guards the `stddev/mean` divide. An exact
`== 0` float compare is correct for the literal-zero case but a mean of a tiny
non-zero magnitude (e.g. 1e-300 from near-zero USD) yields a huge CV without
tripping the guard. Not a crash (no div-by-zero), but the CV can spike
spuriously. Low impact given USD values are not sub-cent here.
**Fix:** Optionally guard `math.Abs(mean) < epsilon` instead of exact zero, or
document that near-zero means are intentionally allowed to produce large CVs.

### IN-04: `reduceCanaryRate` / `rowCanary` treat a JSON-unmarshal failure of the `completion` key as "absent," silently dropping a malformed row from the canary denominator

**File:** `bench/aggregator/aggregate.go:295-305`
**Issue:** When `r.Doc[completion]` is present but not a JSON string,
`json.Unmarshal` errors and `rowCanary` returns `(false, false)` — i.e. "not
present" — so the row is excluded from both numerator and denominator. A row that
carries a malformed completion value is thus silently treated as "no signal"
rather than surfaced. This matches the documented Pitfall-4 null discipline for
*absent* keys but conflates "absent" with "present-but-corrupt." Low impact
(no false canary verdict), but a corrupt completion field becomes invisible.
**Fix:** Distinguish the unmarshal-error case (e.g. count it for operator
visibility, or treat a present-but-unparseable completion as a data error rather
than silent absence).

### IN-05: `fetch-datasets` reports `OK (%d bytes)` for a cache hit identically to a fresh download, hiding whether the network was actually exercised

**File:** `cmd/helix-bench/main.go:382-408`
**Issue:** The command prints the same `OK (N bytes)` line whether `Fetch`
performed a live download or returned a cache hit (see WR-04). An operator
running `fetch-datasets` to *refresh* cannot tell from the output whether the
pinned rev was re-fetched or a stale cache was served. Minor observability gap.
**Fix:** Have `Fetch` (or a sibling) report cache-hit vs network-fetch and label
the line accordingly (`OK (cached, N bytes)` vs `OK (fetched, N bytes)`).

---

_Reviewed: 2026-06-21T04:53:50Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
