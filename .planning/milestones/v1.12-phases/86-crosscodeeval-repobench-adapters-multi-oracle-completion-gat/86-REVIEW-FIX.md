---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat/86-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 86: Code Review Fix Report

**Source review:** 86-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (1 BLOCKER + 4 Warnings; the 5 Info findings were out of scope)
- Fixed: 5
- Skipped: 0

All fixes were applied in an isolated git worktree, each committed atomically
with a `fix(86):` prefix, and verified against the full project gate.

## Fixed Issues

### CR-01 (BLOCKER): Documented `DefaultESThreshold` never applied — zero-value `GateConfig{}` failed OPEN

**Files modified:** `bench/evaluators/completion_gate/gate.go`, `bench/evaluators/completion_gate/gate_test.go`
**Commit:** 6460be4e
**Applied fix:** `Grade` now floors a non-positive `cfg.ESThreshold` to the
documented `DefaultESThreshold` (0.9) before composing the oracles, mirroring
the aggregator's `withDefaults` discipline. This makes the edit-similarity
oracle genuinely enforced under a zero-value `GateConfig{}` (previously
`es >= 0.0` was always true, silently collapsing the three-oracle AND to
EM AND identifier-match). The constant is now live, not dead code.

Added a hermetic regression test (`TestGrade_ZeroValueConfig_ESOracleStillBites`)
that proves an ES-below-0.9 pair yields `verified_correctness=false` under
`GateConfig{}` (the third oracle bites), and that an EM-exact pair (ES==1.0)
still verifies under the same zero-value config (the 0.9 floor is not over-strict).
The abstain explicit-`&false` invariant is untouched.

**Logic note:** This fix changes a verification-gate condition. The new behavior
is covered by the added test and the existing abstain/threshold-knob suite (all
pass), so it is confirmed by hermetic proof rather than left as
"requires human verification."

### WR-01: RepoBench `gold_snippet_index` length-skew/absence hard-failed the whole decode

**Files modified:** `bench/datasets/repobench/loader.go`
**Commit:** 08a0f355
**Applied fix:** Track `goldPresent` (column present and aligned for this row).
A `TaskRetrieval` row whose gold index is absent (column missing or shorter than
tasks) now returns a precise per-row error naming the missing index, instead of
the default `-1` masquerading as a generic `out of range [0,N)` value.
Completion/pipeline rows never consult the gold index, so a missing column no
longer threatens those rows. Documented the requirement on `decodeParquet`.

### WR-02: `int(golds[i])` truncated untrusted int64→int with no explicit bound

**Files modified:** `bench/datasets/repobench/loader.go`
**Commit:** 73e8f324
**Applied fix:** Bound-check the parquet int64 against `math.MinInt`/`math.MaxInt`
in int64 space BEFORE narrowing to a platform `int`, and report the ORIGINAL
int64 value in the error (previously the error reported the wrapped value on a
32-bit build). No new dependency (`math` is stdlib).

### WR-03: Per-row `language` mismatch silently dropped all rows as generic "zero tasks"

**Files modified:** `bench/datasets/crosscodeeval/loader.go`, `bench/datasets/repobench/loader.go`
**Commit:** 501c192e
**Applied fix:** Both loaders now track skipped-vs-kept counts. When
`kept == 0 && skipped > 0`, they return a distinct error naming the
tag-semantics drift (count of mismatched rows + an example non-matching
language value) instead of the misleading generic "parquet decoded zero
tasks". A genuinely empty decode keeps the original message.

### WR-04: `Fetch` cache-hit served partial/corrupt bytes via non-atomic write

**Files modified:** `bench/datasets/crosscodeeval/fetch.go`, `bench/datasets/repobench/fetch.go`
**Commit:** 00d0f2ee
**Applied fix:** Replaced the non-atomic `os.WriteFile` with a temp-file +
atomic `os.Rename` (mirroring the aggregator's `writeReport` pattern,
report.go:346-370). A partial write can no longer become a cache hit — the
rename either lands the complete file or leaves the prior cache state
untouched. The optional `--force` flag was deemed out of the minimum scope
(the reviewer explicitly preferred the atomic-write fix as the minimum); the
atomic write removes the "corrupt cache served forever" failure mode that
motivated `--force`.

## Skipped Issues

None — all 5 in-scope findings were fixed.

## Verification

All required gates pass on the post-fix tree:

- `go build ./...` — clean
- `go vet ./bench/... ./cmd/helix-bench/...` — clean
- `go test -count=1 ./bench/... ./cmd/helix-bench/...` — all packages ok
  (including `bench/aggregator`, confirming the leaderboard/cost goldens are
  byte-identical)
- `make vet` (full gate, all 6 custom vettools) — clean
- `make verify-verified-md` — all required VERIFIED.md sections present

`git diff` over the two `loader.go` files (and all changed files) touches no
golden / testdata / fixture / parquet files; the aggregator golden tests pass
unchanged. No new dependencies were added; arrow-go remains the only parquet
dep and no gomlx was introduced.

---

_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
