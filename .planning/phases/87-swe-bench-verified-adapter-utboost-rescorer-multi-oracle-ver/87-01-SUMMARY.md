---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
plan: 01
subsystem: testing
tags: [swe-bench, utboost, subprocess, result-schema, dataset-pin, bench, go]

# Dependency graph
requires:
  - phase: 83-85
    provides: "EmbedderID/Language additive-minor open-key discipline on result.v2 (mirrored for container_id/exit_code)"
  - phase: 84-85
    provides: "bench/container/engine.go + run.go fixed-argv/strict-env/procgroup os/exec discipline (mirrored by harness.go)"
  - phase: 85-86
    provides: "bench/datasets/aider-polyglot pin.go + bench/datasets/crosscodeeval fetch.go pinned-fetch discipline (mirrored by swebench-utboost)"
provides:
  - "result.v2 additive open keys container_id (string) + exit_code (*int) on ResultInput/resultDoc/BuildResult + schema"
  - "shared SwebenchRawResolvedKey/SwebenchRescoredVerifiedKey rescore key-name consts (single home for Plan 03 producer + Plan 04 reader)"
  - "bench/evaluators/swebench harness.go fixed-argv subprocess wrapper for python -m swebench.harness.run_evaluation"
  - "bench/datasets/swebench-utboost pin.go + fetch.go (pinned, SSRF-safe, LimitReader-bounded, HELIX_BENCH_NETWORK-gated UTBoost fetch)"
affects: [87-02, 87-03, 87-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Additive-minor open result.v2 key (omitempty, schema_version stays v2, additionalProperties OPEN, not required) — *int for exit_code so a clean 0 is preserved"
    - "Fixed-argv subprocess seam (RunArgs) with total regexp-free validators fail-closing before os/exec + strict env allowlist + procgroup + runShim hermetic test seam"
    - "Leaf pinned-dataset fetch (isHexSHA1 mutable-ref refusal, pinned-host SSRF guard, io.LimitReader cap, HELIX_CACHE_DIR cache, network-gated live leg)"

key-files:
  created:
    - bench/evaluators/swebench/harness.go
    - bench/evaluators/swebench/harness_test.go
    - bench/evaluators/swebench/procgroup_unix.go
    - bench/evaluators/swebench/procgroup_windows.go
    - bench/datasets/swebench-utboost/pin.go
    - bench/datasets/swebench-utboost/fetch.go
    - bench/datasets/swebench-utboost/pin_test.go
    - bench/datasets/swebench-utboost/fetch_test.go
    - bench/runtime/swebench_keys_test.go
  modified:
    - bench/runtime/result.go
    - bench/schema/result.v2.schema.json

key-decisions:
  - "exit_code is *int (not int) so a clean harness exit 0 round-trips, never dropped by omitempty (Pitfall 2)"
  - "The two rescore open-key-name consts live ONCE in bench/runtime/result.go (mirrors canary.DocKeyCompletion) so Plan 03 producer + Plan 04 reader cannot drift on spelling"
  - "harness.go shells the python CLI directly (NOT container.Engine.Run) — the harness owns its own per-instance Docker (Open Q4)"
  - "A1-A5 UTBoost upstream details resolved APPROVED-WITH-DEFERRAL: documented values used, exact LIVE pin/path confirmation deferred to a Docker+swebench+network host"

patterns-established:
  - "Pattern: result.v2 additive open keys with *int for any field whose zero is meaningful"
  - "Pattern: swebench harness fixed-argv wrapper cloning container/engine.go discipline"
  - "Pattern: swebench-utboost leaf pinned fetch cloning crosscodeeval/fetch.go discipline"

requirements-completed: [ADAPTER-SWE-01, VERIFIED-02]

# Metrics
duration: 6min
completed: 2026-06-21
---

# Phase 87 Plan 01: SWE-bench Substrate Summary

**result.v2 additive container_id/exit_code open keys + shared rescore key-name consts, a fixed-argv `python -m swebench.harness.run_evaluation` subprocess wrapper (strict env, procgroup, hermetic golden argv), and a pinned SSRF-safe network-gated UTBoost dataset fetch.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-06-21T05:42:38Z
- **Completed:** 2026-06-21T05:48:37Z
- **Tasks:** 5 (4 auto + 1 checkpoint resolved-with-deferral by orchestrator)
- **Files modified:** 11 (9 created, 2 modified)

## Accomplishments
- `container_id` (string) + `exit_code` (`*int`) landed as additive-minor `omitempty` open keys on `ResultInput`/`resultDoc`/`BuildResult` and the schema, mirroring `EmbedderID`/`Language` exactly; schema stays `"v2"`, neither key added to `required`, `additionalProperties` left OPEN. A clean `exit_code: 0` round-trips (preserved, never dropped); a pre-SWE-bench row (both absent) still validates.
- The two PINNED snake_case rescore open-key-name consts `SwebenchRawResolvedKey` / `SwebenchRescoredVerifiedKey` declared ONCE in `bench/runtime/result.go` — the single shared home Plan 03's producer and Plan 04's reader both import.
- `bench/evaluators/swebench/harness.go`: `RunArgs` builds the fixed `python -m swebench.harness.run_evaluation` argv with every value total-validated (dataset-name 2-name allowlist, `run_id [A-Za-z0-9_-]+`, clean-absolute predictions path, `<owner>__<repo>-<num>` instance ids, cache-level enum, `max_workers >= 1`) BEFORE crossing os/exec; `Harness.Run` clones `container/engine.go` (strict env allowlist PATH/HOME/HELIX_CACHE_DIR, `procGroupAttr` group-kill, `runShim` hermetic seam, no shell). `errHarnessUnavailable` Detect gate; live smoke t.Skips on python absence.
- `bench/datasets/swebench-utboost/{pin.go,fetch.go}`: a leaf (stdlib only) pinned UTBoost fetch — `isHexSHA1` 40-hex mutable-ref refusal, pinned-host SSRF guard, `io.LimitReader` 256 MiB cap, `HELIX_CACHE_DIR` cache, atomic temp+rename, `HELIX_BENCH_NETWORK`-gated live leg.

## Task Commits

1. **Task 1: container_id + exit_code additive result.v2 keys + shared consts** — `36474afe` (feat)
2. **Task 2: backward-compat + clean-zero hermetic test** — `a7b9a662` (test)
3. **Task 3: harness fixed-argv subprocess wrapper** — `6575e70f` (feat)
4. **Task 4: A1-A5 upstream checkpoint** — resolved-with-deferral by orchestrator (no separate commit; allowlist value already landed in Task 3)
5. **Task 5: UTBoost dataset pin + network-gated fetch** — `2024c60f` (feat)

## Files Created/Modified
- `bench/runtime/result.go` — ContainerID/ExitCode fields + SwebenchRawResolvedKey/SwebenchRescoredVerifiedKey consts
- `bench/schema/result.v2.schema.json` — optional container_id (string) + exit_code (["integer","null"]) open keys
- `bench/runtime/swebench_keys_test.go` — backward-compat (absent keys validate) + clean-zero (exit_code 0 preserved) proofs
- `bench/evaluators/swebench/harness.go` — RunArgs fixed-argv builder + Harness.Run subprocess seam
- `bench/evaluators/swebench/harness_test.go` — golden argv + per-validator fail-close + runShim equivalence + env-strict + live-skip
- `bench/evaluators/swebench/procgroup_{unix,windows}.go` — build-tag-split procGroupAttr for windows cross-compile
- `bench/datasets/swebench-utboost/pin.go` — DatasetID/PinnedSHA/isHexSHA1/isValidHTTPSHost
- `bench/datasets/swebench-utboost/fetch.go` — cacheDir/cachePath/resolveURL/Fetch/writeCacheAtomic
- `bench/datasets/swebench-utboost/{pin_test.go,fetch_test.go}` — pin immutability + mutable-ref refusal + cache-escape refusal + network-gated live leg

## Decisions Made
- `exit_code` is `*int` (not `int`): a literal `0` is a real "ran clean" code that a value-type `omitempty` would wrongly drop; nil = "no exit code captured".
- Rescore key-name consts live in `bench/runtime/result.go` (one shared home) so the Plan 03 writer and Plan 04 reader cannot drift on the spelling (mirrors `canary.DocKeyCompletion`).
- The harness wrapper shells `python` directly rather than routing through `container.Engine.Run` — the swebench harness manages its own per-instance Docker (Open Q4).

## Deviations from Plan

None - plan executed exactly as written. (Task 4 was a `checkpoint:human-verify`; the orchestrator resolved it APPROVED-WITH-DEFERRAL — see "A1-A5 Checkpoint Resolution" below — which is the intended path, not a deviation.)

## A1-A5 Checkpoint Resolution (Task 4 — honest deferral record)

The Task 4 `checkpoint:human-verify` gated ONLY the live UTBoost fetch; all hermetic-fixture work proceeded regardless. The orchestrator resolved it **APPROVED-WITH-DEFERRAL** using the 87-RESEARCH-documented values:

- **A1 (dataset id + pin rev):** `Bertsekas/SWE-Bench_Verified_UTBoost` @ `4c21a4831d80b66e976f2a5ce946a0abded7a2aa`. The rev is a valid 40-lowercase-hex (`isHexSHA1`-accepted) immutable commit shape and is wired into both `bench/datasets/swebench-utboost/pin.go` `PinnedSHA`/`DatasetID` and the `harness.go` dataset-name allowlist.
- **A2 (drop-in vs fork):** treated as a complete drop-in for `--dataset_name` (no harness fork), per the UTBoost README.
- **A3 (report log-dir path) / A5 (run-all-tests mechanism):** documented values used; these affect only the LIVE ingestion path (Plan 02), not this plan's hermetic fixtures.
- **A4 (`report.resolved` authoritative gate):** noted for Plan 03's `task_success` wiring.

**HONESTLY RECORDED DEFERRAL:** the EXACT pin rev, the report log-dir path, and the run-all-tests mechanism are confirmed against the *documented* upstream, but their **LIVE** confirmation (reaching huggingface.co / a real Docker+swebench host) is **DEFERRED** until a network-enabled host runs the gated smoke. This offline environment cannot reach HF to confirm the rev. `TestLiveFetch` is `HELIX_BENCH_NETWORK`-gated and tolerates a 404 as a clean skip (never the sole proof); `pin.go` `PinnedSHA` carries a doc comment recording the deferral and the re-pin query URL. If a live host finds a different rev, update `PinnedSHA` (the harness allowlist is by *name*, unaffected).

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. (Live UTBoost fetch + live harness smoke need Docker + the `swebench` Python package + `HELIX_BENCH_NETWORK`; all SKIP cleanly offline.)

## Verification
- `go build ./...` green.
- `go vet ./bench/...` green; `make vet` green (all custom vettools pass).
- `go test ./bench/runtime/... ./bench/schema/... ./bench/evaluators/swebench/... ./bench/datasets/swebench-utboost/...` green offline; full `go test ./bench/...` = 33 packages ok, no failures (result.v2 goldens byte-identical — omitempty drops the two new keys on non-SWE-bench rows).
- `make verify-no-docker-sdk` green (no Docker Go SDK).
- swebench + swebench-utboost packages cross-compile for `GOOS=windows`.
- Live UTBoost fetch + live harness smoke SKIP cleanly (no HELIX_BENCH_NETWORK / no python+swebench+docker).

## Next Phase Readiness
- Substrate ready for Plan 02 (predictions.jsonl producer + harness report parser + ingestion populating container_id/exit_code), Plan 03 (3-condition verified_correctness gate + rescore.go stamping SwebenchRescoredVerifiedKey), Plan 04 (aggregator raw-vs-rescored column reading both rescore key-name consts).
- Open: the exact UTBoost live pin/report-path confirmation is deferred to a Docker+swebench+network host before the first live 5-task smoke.

## Self-Check: PASSED

All 12 created/modified files present on disk; all 4 task commits (`36474afe`, `a7b9a662`, `6575e70f`, `2024c60f`) present in git history.

---
*Phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver*
*Completed: 2026-06-21*
