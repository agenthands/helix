---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver/87-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 87: Code Review Fix Report

**Source review:** 87-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (1 critical + 4 warnings; the 3 Info findings are out of scope)
- Fixed: 5
- Skipped: 0

All in-scope findings were fixed with hermetic regression coverage. Every fix was
committed atomically with a `fix(87):` prefix. The full gate is green:
`go build ./...`, `go vet ./bench/... ./cmd/helix-bench/...`,
`go test -count=1 ./bench/...`, the SC#2 `TestVerified_BuggyPatch`, `make vet`
(incl. verify-no-docker-sdk), `make verify-verified-md`, and the aggregator
leaderboard/cost goldens all pass. No new dependencies were added.

## Fixed Issues

### CR-01: Vacuous-pass — empty/non-ran test buckets yield `verified_correctness=true`

**Files modified:** `bench/evaluators/swebench/verified.go`,
`bench/evaluators/swebench/verified_test.go`,
`bench/evaluators/swebench/testdata/report.notapplied_canonical.json` (new),
`bench/evaluators/swebench/testdata/report.notests_canonical.json` (new)
**Commit:** e9d226bb
**Applied fix:**
- `passed()` now requires a gating bucket to have run `>=1` test AND have zero
  failures (`len(l.Failure)==0 && len(l.Success)>0`), so an empty `FAIL_TO_PASS`
  bucket is no longer credited as a pass.
- A dedicated `noRegressBucket()` preserves the failure-only semantics for the
  regress check (`PASS_TO_PASS` / `PASS_TO_FAIL`), where an empty bucket
  legitimately means "no pre-existing tests to regress" — so the stricter
  non-empty-success rule is applied ONLY to the canonical/augmented gate buckets,
  per the review's explicit guidance.
- `Grade` now fails closed (`verified_correctness=&false`, oracles not consulted)
  when `canonical.PatchSuccessfullyApplied` is false, so a no-op / failed-to-apply
  patch can never read as verified.
- Two hermetic regression fixtures + `TestVerified_VacuousPass` prove both vacuous
  shapes (patch-not-applied with empty buckets; patch-applied but no tests ran)
  yield `verified_correctness=false`.
- The existing `TestVerified_Gate` / `TestVerified_ApplyToMetrics` helpers were
  updated to set `PatchSuccessfullyApplied: true` so the all-pass case still
  reaches the oracles under the new patch gate. The load-bearing SC#2
  `TestVerified_BuggyPatch` is unchanged and still passes (its committed
  `report.buggy_canonical.json` already had `patch_successfully_applied: true` and
  a non-empty `FAIL_TO_PASS.success`).

**Requires human verification:** this is a gate-logic change. The two new tiers
and the existing matrix tests assert the intended truth table, but a reviewer
should confirm the `noRegress` failure-only carve-out matches the intended
contract before the Phase 89 live wiring lands (IN-03).

### WR-01: `PinnedSHA` unconfirmed — add a content-digest assertion

**Files modified:** `bench/datasets/swebench-utboost/pin.go`,
`bench/datasets/swebench-utboost/fetch.go`,
`bench/datasets/swebench-utboost/fetch_test.go`
**Commit:** 9b8a67fe
**Applied fix:** Added `PinnedContentDigests` (a per-file `sha256` map, currently
empty pending live confirmation) plus `assertContentDigest`. `Fetch` now asserts
the downloaded bytes (before caching) AND the cache-hit bytes against any recorded
digest, failing closed on mismatch — closing the supply-chain gap the rev-pin
alone cannot. The deferral is now enforceable code (a release-blocking checklist
item) rather than a comment: once a Docker+swebench+network host records the
audited digests, the assertion is automatic. An unlisted file remains allowed on
the rev-pin alone (the documented, reviewed residual). This is the review's
preferred "add the content-digest assert" option, scoped so the empty-digest
offline state stays correct.

### WR-02: Cache hit bypasses the `maxDatasetBytes` cap

**Files modified:** `bench/datasets/swebench-utboost/fetch.go`,
`bench/datasets/swebench-utboost/fetch_test.go`
**Commit:** b49e7d8d
**Applied fix:** `readCacheCapped` replaces the bare `os.ReadFile` on the cache-hit
path: it `os.Stat`-checks the size and reads via `io.LimitReader(f,
maxDatasetBytes+1)` with the same `+1` overflow probe the network leg uses, so a
poisoned/oversized cache file is refused symmetric with the network path. Only a
genuine miss (`os.IsNotExist`) falls through to the network fetch; an oversized or
otherwise unreadable cache file surfaces the error instead of silently re-fetching.
`TestFetchCacheHitHonorsSizeCap` proves miss/in-cap/oversized behavior using a
sparse oversized file (no multi-GiB fixture needed).

### WR-03: Harness `Run` does not set `cmd.Dir`

**Files modified:** `bench/evaluators/swebench/harness.go`,
`bench/evaluators/swebench/harness_test.go`
**Commit:** c9aeef33 (committed together with WR-04 — both edit `Run`)
**Applied fix:** Added a validated `HarnessRun.WorkDir` field. `Run` calls
`resolveWorkDir`, which enforces the same `isValidPredictionsPath` discipline
(clean, absolute, no `..`/`:`) for an explicit dir and defaults an empty one to
`<HELIX_CACHE_DIR>/swebench-runs` (created if absent), then sets `cmd.Dir`. The
harness output tree is now rooted deterministically, never under the uncontrolled
parent CWD. `TestResolveWorkDir` covers the default, explicit-valid, and
fail-closed (relative/traversal/colon) cases.

### WR-04: `allowlistEnv` can hand the harness an empty PATH / no DOCKER_HOST

**Files modified:** `bench/evaluators/swebench/harness.go`,
`bench/evaluators/swebench/harness_test.go`
**Commit:** c9aeef33 (committed together with WR-03 — both edit `Run`)
**Applied fix:** `allowlistEnv` now returns `([]string, error)` and fails closed
with `errHarnessEnv` when `PATH` is empty (the harness locates `docker` via PATH).
The allowlist was extended to forward `DOCKER_HOST` / `DOCKER_TLS_VERIFY` /
`DOCKER_CERT_PATH` WHEN SET, so a non-default Docker daemon is reachable — still an
explicit allowlist, never the inherited env. `TestAllowlistEnvFailsOnEmptyPath`
and `TestAllowlistEnvForwardsDocker` cover the new behavior;
`TestAllowlistEnvStrict` was updated to clear DOCKER_* so it still asserts the base
3-key forward.

## Skipped Issues

None. (The 3 Info findings — IN-01 `Detect` python-only probe, IN-02 git
quoted-path header parsing, IN-03 latent-until-Phase-89 — were out of scope per
the fix instructions and were not modified. CR-01's fix already satisfies IN-03's
"fix CR-01 before the Phase 89 wiring" dependency.)

---

_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
