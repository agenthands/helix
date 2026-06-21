---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - bench/evaluators/swebench/verified.go
  - bench/evaluators/swebench/rescore.go
  - bench/evaluators/swebench/differential.go
  - bench/evaluators/swebench/harness.go
  - bench/evaluators/swebench/ingest.go
  - bench/evaluators/swebench/predictions.go
  - bench/evaluators/swebench/report.go
  - bench/datasets/swebench-utboost/fetch.go
  - bench/datasets/swebench-utboost/pin.go
  - bench/aggregator/aggregate.go
  - bench/aggregator/report.go
  - bench/runtime/result.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 87: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Reviewed the SWE-bench Verified adapter: the subprocess harness wrapper, predictions
producer, report parser, ingest, the 3-condition `verified_correctness` gate, the UTBoost
rescorer + `ApplyToRow`, differential overlap, the additive `result.v2` open keys, the
aggregator raw-vs-rescored column, and the UTBoost dataset fetch/pin.

The headline integrity claim — that the gate is computed independently of `task_success`,
never aliases pointers, and fails closed to an explicit `&false` on a missing augmented
oracle — holds for the abstain path and the pointer-independence discipline, and the SC#2
buggy-patch divergence is proven **hermetically** (`TestVerified_BuggyPatch`, no `t.Skip`,
committed fixtures). The harness wrapper's argv allowlisting, env allowlist, and SSRF/path
guards on the UTBoost fetch are solid.

**However, the gate has one BLOCKER false-positive path the phase exists to prevent:** the
`passed()` predicate treats an *empty* test bucket as a pass, so an instance whose patch
failed to apply (or whose tests never ran) — yielding all-empty `failure` lists — produces
`verified_correctness=true` with zero tests executed. This is the exact spurious-true the
milestone's headline integrity claim forbids. It is currently latent (no production caller
wires `Grade` yet — Phase 89 owns that), but it is a defect in the gate's contract and must
be fixed before the gate is wired live.

## Critical Issues

### CR-01: Vacuous-pass — empty/non-ran test buckets yield `verified_correctness=true`

**File:** `bench/evaluators/swebench/verified.go:67,100-104`

**Issue:** `passed(l TestList)` returns `len(l.Failure) == 0`. It is satisfied by an
*empty* bucket as readily as by an all-success bucket. `Grade` composes:

```go
canonicalPass := passed(canonical.TestsStatus.FailToPass)   // true if FAIL_TO_PASS is empty
augmentedPass := passed(augmented.TestsStatus.FailToPass)    // true if augmented FAIL_TO_PASS is empty
noRegress     := passed(...PassToPass) && passed(...PassToFail)
ok := canonicalPass && augmentedPass && noRegress
```

When a harness report is for an instance whose patch did **not** apply, errored, or
otherwise never ran its tests, `tests_status` is empty or absent. `encoding/json` leaves
`TestsStatus` at its zero value: every `TestList.Failure` is an empty slice. All three
conditions then evaluate `true` and the gate returns `verified_correctness=&true` for an
instance that executed **zero** verification tests. (Confirmed reachable: `ParseInstanceReport`
tolerates a missing `tests_status` key — no `DisallowUnknownFields`, no required-field check —
and `Grade` consults neither `PatchSuccessfullyApplied`, `PatchExists`, `PatchIsNone`, nor
`Resolved`.) A no-op or failed-to-apply patch thus reads as verified-correct — the single
worst benchmark failure the phase is chartered to prevent.

This is distinct from, and stricter than, the abstain branch (which only guards a *nil*
augmented report). A *present* augmented report with an empty `FAIL_TO_PASS` is not an
abstain today but should not count as a pass either.

**Fix:** Require each gating bucket to have actually run at least one test, and require a
clean application, before crediting a pass. Treat an empty `FAIL_TO_PASS` (canonical or
augmented) as a non-pass, not a pass:

```go
// A bucket passes only when it ran ≥1 test AND none failed. An empty bucket is
// NOT a pass — it means no verification tests executed (patch didn't apply, instance
// errored, or the augmented suite was absent), which must never read as verified.
func passed(l TestList) bool {
	return len(l.Failure) == 0 && len(l.Success) > 0
}
```

And gate the whole verdict on the canonical patch having actually applied (the harness
records this), so a non-applied/no-op patch can never be verified:

```go
func Grade(canonical InstanceEval, augmented *InstanceEval) GateResult {
	if augmented == nil {
		f := false
		return GateResult{VerifiedCorrectness: &f}
	}
	// Fail closed if the patch never applied: an un-run instance is not verified.
	if !canonical.PatchSuccessfullyApplied {
		f := false
		return GateResult{VerifiedCorrectness: &f}
	}
	canonicalPass := passed(canonical.TestsStatus.FailToPass)
	augmentedPass := passed(augmented.TestsStatus.FailToPass)
	// PASS_TO_PASS regress check stays "no failures" — but a present non-empty
	// success list is the expected shape; keep noRegress as failure-only since an
	// empty PASS_TO_PASS legitimately means "no pre-existing tests to regress".
	noRegress := len(canonical.TestsStatus.PassToPass.Failure) == 0 &&
		len(canonical.TestsStatus.PassToFail.Failure) == 0
	ok := canonicalPass && augmentedPass && noRegress
	cv, av, nv, okv := canonicalPass, augmentedPass, noRegress, ok
	return GateResult{VerifiedCorrectness: &okv, CanonicalPass: &cv, AugmentedPass: &av, NoRegress: &nv}
}
```

Add a hermetic fixture/test proving an empty-`tests_status` (failed-to-apply) instance
yields `verified_correctness=false`, alongside the existing buggy-patch fixture. Note that
`noRegress` keeps its failure-only semantics (an empty `PASS_TO_PASS` legitimately means
there were no pre-existing tests), but `canonicalPass`/`augmentedPass` must require a real
run.

## Warnings

### WR-01: `PinnedSHA` is self-admittedly unconfirmed against live upstream

**File:** `bench/datasets/swebench-utboost/pin.go:32-40`

**Issue:** The whole SSRF/tamper-resistance story for the UTBoost fetch rests on
`PinnedSHA` being the real immutable commit currently serving the augmented suite. The
constant's own doc comment states its "EXACT LIVE confirmation ... is DEFERRED" and the
offline environment "cannot reach HF to confirm it." `isHexSHA1` only proves the *shape*
of the SHA, not that it resolves to the audited content. If this 40-hex is wrong, the
fetcher silently downloads whatever (if anything) HF serves at that ref — and the cached
result is then served "forever as a hit." This is a supply-chain integrity gap that the
pinning machinery cannot itself close.

**Fix:** Before this dataset is fetched in any scored run, confirm the SHA via
`https://huggingface.co/api/datasets/Bertsekas/SWE-Bench_Verified_UTBoost/refs` on a
networked host and update the constant, OR add a content-digest check (e.g. assert a
pinned sha256 of the downloaded payload) so a wrong/moved commit fails closed rather than
caching unverified bytes. Track the deferral as a release-blocking checklist item, not a
code comment.

### WR-02: Cache hit short-circuits the size cap — a poisoned/oversized cache file is served unbounded

**File:** `bench/datasets/swebench-utboost/fetch.go:116-118`

**Issue:** On a cache hit `Fetch` returns `os.ReadFile(dst)` directly. The
`maxDatasetBytes` `io.LimitReader` cap (T-87-04) is applied only on the **network** leg.
A cache file that grew (cosmic ray, a future writer bug, or a manually planted file under
the predictable `<cacheDir>/swebench-utboost/<rev>/<file>` path) is read into memory with
no bound, defeating the OOM defense the network path carefully enforces. The cache path is
not a trust boundary today, but the asymmetry means the documented "256 MiB cap" is not
actually a total invariant.

**Fix:** Enforce the same cap on the cache read, e.g. `os.Stat` the file and reject when
`size > maxDatasetBytes`, or read via `io.LimitReader(f, maxDatasetBytes+1)` and apply the
same overflow check used on the network body.

### WR-03: Harness `Run` does not set `cmd.Dir`, leaving the harness CWD as the (uncontrolled) parent CWD

**File:** `bench/evaluators/swebench/harness.go:262-264`

**Issue:** `exec.CommandContext` inherits the parent process's working directory when
`cmd.Dir` is left empty. The swebench harness writes its `logs/run_evaluation/<run_id>/...`
tree relative to CWD, so the output location (and any report the ingest path later reads)
depends on wherever the daemon happened to be launched — not on a value validated here.
The env allowlist and fixed argv are carefully controlled; the unpinned CWD is the one
uncontrolled input that influences where artifacts land. This is a robustness/predictability
gap (the run_id is validated, but the directory it roots under is not).

**Fix:** Add a validated `WorkDir` field to `HarnessRun` (clean, absolute, same
`isValidPredictionsPath` discipline) and set `cmd.Dir = r.WorkDir`, so the harness output
tree is deterministic and not a function of the parent CWD.

### WR-04: `allowlistEnv` can hand the harness an empty environment, breaking Docker resolution

**File:** `bench/evaluators/swebench/harness.go:274-282`

**Issue:** `allowlistEnv` forwards only `PATH`, `HOME`, `HELIX_CACHE_DIR` and skips any
that are unset/empty. If `PATH` is empty in the parent env the harness subprocess gets a
`cmd.Env` with no `PATH`, and `python -m swebench.harness.run_evaluation` will fail to
spawn Docker (the harness shells out to `docker`, which it locates via `PATH`). More
broadly, the swebench harness commonly needs `DOCKER_HOST` on non-default daemon sockets;
the strict 3-key allowlist will silently drop it, surfacing as an opaque harness failure
rather than a clear config error. This is intentional hardening, but the failure mode is a
confusing run-time crash rather than a validated refusal.

**Fix:** Fail closed with a clear error when `PATH` resolves empty (the subprocess cannot
run without it), and consider extending the allowlist to the documented set the harness
actually needs (`DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`) so a non-default
Docker daemon is reachable — still an explicit allowlist, never the inherited env.

## Info

### IN-01: `Detect()` reports the harness "available" when only python (not swebench) is present

**File:** `bench/evaluators/swebench/harness.go:236-243`

**Issue:** `Detect` resolves merely python on `PATH` and returns a `*Harness`; the
swebench package's presence is deferred to run-time import error. The doc comment
acknowledges this, but live tests `t.Skip` on `errHarnessUnavailable` from `Detect`, so an
environment with python-but-no-swebench will *not* skip — it will attempt a run and fail.
This is benign for the gated live smoke (it `t.Skip`s a second time at the Docker gate),
but the "cheap hermetic gate live tests skip on" framing slightly overstates what `Detect`
proves.

**Fix:** Either add a lightweight `python -c "import swebench"` probe to `Detect` (cached),
or rename the sentinel/doc to make explicit that `Detect` proves only the interpreter, not
the harness package.

### IN-02: `touchedFiles` does not handle git's quoted-path header form

**File:** `bench/evaluators/swebench/differential.go:56-67`

**Issue:** `strings.Fields(rest)` splits on whitespace, so a `diff --git` header for a path
containing a space — which git emits quoted, e.g. `diff --git "a/dir/my file.py" "b/dir/my file.py"`
— is parsed incorrectly: `fields[len(fields)-1]` becomes `file.py"` and the `b/` prefix
strip fails. Such files are silently dropped from the touched-set, skewing the
`diff_overlap` denominator/numerator. Rare in SWE-bench corpora but produces a wrong (not
merely null) overlap when it occurs.

**Fix:** Detect a leading `"` in the rest and parse the quoted token(s) per git's
core.quotePath rules, or fall back to the `+++ b/<path>` hunk header which is unquoted-
friendly, before defaulting to the whitespace split.

### IN-03: Gate/rescore/ingest/differential have no production caller yet (CR-01 is latent until Phase 89)

**File:** `bench/evaluators/swebench/verified.go:92`, `rescore.go:39`, `ingest.go:36`, `differential.go:78`

**Issue:** `Grade`, `Rescore`, `ApplyToRow`, `Ingest`, and `DiffOverlap` are exercised only
by `_test.go` callers; no non-test code wires them into a scored run (the cell-wiring is
documented as Phase 89). This means CR-01's false-positive cannot reach a published
`result.v2` row today — but it also means the gate's contract is being locked in before any
production path stresses the empty-bucket case. Fix CR-01 (and add the empty-`tests_status`
fixture) **before** the Phase 89 wiring lands, so the spurious-true can never reach a row.

**Fix:** No code change here beyond CR-01; flag the dependency so CR-01 is resolved prior to
the downstream wiring rather than after.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
