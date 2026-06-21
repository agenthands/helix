---
phase: 85
plan: 07
subsystem: bench
tags: [bench, aider-polyglot, dataset-loader, 2-attempt-protocol, pinned-sha-clone, hermetic-fixture, tdd, leaf-package]
requires:
  - "85-01: result.v2 `language` provenance field + aggregator Report.ByLanguage per-language pass-rate slice"
  - "85-02: bench/container.Run(--network=none) seam (the hermetic-task execution path the loader records intent for)"
provides:
  - "bench/datasets/aider-polyglot dataset-loader-only adapter (ADAPTER-AIDER-01 complete): pinned-sha shallow clone + .meta/config.json mapping + 2-attempt stderr-reprompt protocol"
  - "Per-exercise Exercise carrying Language (feeds the 85-01 result.v2 `language` field → aggregator ByLanguage slice for SC#1)"
  - "SC#3 non-hermetic task flagging (config marker + per-language rule)"
affects: [ADAPTER-AIDER-01, SC#1-per-language-pass-rate, SC#3-network-isolation, phase-89-reports]
tech-stack:
  added: []
  patterns:
    - "Leaf package (stdlib only) mirroring bench/ragindex + bench/container leaf discipline"
    - "cacheDir() three-step precedence cloned verbatim from bench/ragindex/cache.go (HELIX_CACHE_DIR → UserCacheDir/helix → ~/.helix/cache)"
    - "Fixed-argv + env-allowlist + sha-pin discipline cloned from bench/container/engine.go for the git child"
    - "validatePathSegment (V5/T-85-07-02) cloned from bench/runtime/validate.go into the leaf package"
    - "Injected TestFn/AgentFn function-type seams so the hermetic fixture test drives the exact 2-attempt protocol with NO toolchain"
key-files:
  created:
    - bench/datasets/aider-polyglot/loader.go
    - bench/datasets/aider-polyglot/pin.go
    - bench/datasets/aider-polyglot/clone.go
    - bench/datasets/aider-polyglot/loader_test.go
    - bench/datasets/aider-polyglot/clone_test.go
    - bench/datasets/aider-polyglot/fixtures/ (python wordy hermetic + rust leap non_hermetic, 8 files)
  modified: []
decisions:
  - "PinnedSHA = 7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f — the REAL refs/heads/main HEAD of Aider-AI/polyglot-benchmark resolved live at plan time (2026-06-21), NOT a placeholder; clone pins this exact commit by sha (never a branch/tag) per T-85-07-01"
  - "Loader uses the dataset's OWN native per-language command table (pytest / cargo test -- --include-ignored / go test ./... / ./gradlew test / ./npm-test.sh / ./cpp-test.sh), NOT the TOOLBENCH runner argv (RESEARCH §Pattern 4 / Open Q1)"
  - "Hermetic fixture-set test is the SOLE authoritative proof of the config.json mapping + 2-attempt protocol; the live pinned-sha clone is network-gated (HELIX_BENCH_NETWORK + github probe) and never the sole proof"
  - "git checkout step omits the -- terminator because `git checkout --detach` rejects a -- <pathspec> form; flag-smuggling on the sha is closed by isHexSHA1 (40-hex, no leading '-'), not the terminator"
metrics:
  duration: ~35min
  tasks: 2
  files: 14
  completed: 2026-06-21
---

# Phase 85 Plan 07: Aider Polyglot Dataset-Loader-Only Adapter (ADAPTER-AIDER-01) Summary

**Landed the dataset-loader-only Aider Polyglot adapter — a pinned-sha `git clone --depth 1` of `Aider-AI/polyglot-benchmark`, per-exercise `.meta/config.json` file mapping (`files.solution`/`test`/`example`), and the upstream 2-attempt + stderr-reprompt protocol (tries=2, 180s) driven by the dataset's OWN native per-language test commands — proven HERMETICALLY over a committed fixture task set with NO upstream Python harness, with the live clone + full 225-task run honestly recorded as network/toolchain-gated.**

## What Was Built

A new LEAF package `bench/datasets/aider-polyglot/` (stdlib-only, mirroring the `bench/ragindex` + `bench/container` leaf invariant):

- **`loader.go`** — `Config` decoding `.meta/config.json` `files.solution`/`test`/`example` + optional `non_hermetic`; `loadExercise(dir, lang)` reading + V5-validating the exercise name & language before any Join; `restorePristine` restoring solution + test stubs into the work dir each attempt (the benchmark.py restore step); `RunExercise` driving the exact `tries=2` loop (agent edits → run native tests under a 180s child context → break on pass → re-prompt attempt 2 with the captured failure output); `nativeTestCommand` returning aider's OWN per-language argv (NOT TOOLBENCH); `flagNonHermetic` recording SC#3 network-at-test-time tasks.
- **`pin.go`** — `RepoURL` + `PinnedSHA` (the real main HEAD) + `isHexSHA1` 40-hex validator (mirroring `bench/container.isHexSHA256`).
- **`clone.go`** — `cacheDir()`/`clonePath()` rooting the clone under `<cacheDir>/aider-polyglot/<sha>/` (precedence cloned from `bench/ragindex/cache.go`); `cloneArgs` building the fixed `--depth 1` clone/fetch/checkout argv pinned by sha, fail-closing on a non-hex sha / flag-shaped URL / unclean dest BEFORE os/exec; `Clone` running it with a strict `gitEnv` allowlist (PATH/HOME/HELIX_CACHE_DIR + `GIT_TERMINAL_PROMPT=0`); `headSHA` for the live HEAD==pin assertion.
- **`fixtures/`** — a committed task set: a hermetic Python `wordy` exercise (mirroring the real `.meta/config.json` shape exactly) + a `non_hermetic`-marked Rust `leap` exercise (exercises SC#3 flagging). The hermetic test drives real file mapping + pristine restore + the full 2-attempt loop with NO clone.

## Consumes Plan 01 + Plan 02

- **Plan 01:** each `Exercise` carries `Language`, the field the runner persists onto result.v2's `language` provenance key so the aggregator's `Report.ByLanguage` slices per-language pass-rate (SC#1 substrate).
- **Plan 02:** `flagNonHermetic` records which tasks must NOT reach the network at test time; the intended hermetic-task execution path is the Plan 02 `container.Run(--network=none)` seam (the loader records the intent; the live container run is toolchain-gated).

## Tasks & Commits

1. **Task 1 (RED): failing config.json mapping + 2-attempt protocol over the committed fixture set** — `34d99368` (test)
2. **Task 1 (GREEN): loadExercise + restorePristine + RunExercise + nativeTestCommand + flagNonHermetic** — `b5be7915` (feat)
3. **Task 2 (RED): failing pinned-sha clone argv + non-hermetic flagging + network-gated live leg** — `e471741f` (test)
4. **Task 2 (GREEN): pin.go + clone.go (cloneArgs/Clone/headSHA/clonePath)** — `a6724905` (feat)

## Verification

- `go build ./...` clean; `go vet ./bench/...` clean; `go test ./bench/...` green (hermetic).
- `go test ./bench/datasets/aider-polyglot/ -count=1` green: `TestLoadExercise`, `TestLoadExercisePathSegmentRejected`, `TestRestorePristine`, `TestTwoAttemptReprompt` / `PassFirst` / `FailBoth`, `TestNativeTestCommand`, `TestTwoAttemptTimeoutBudget`, `TestPinIsRealHexSHA`, `TestIsHexSHA1`, `TestCloneArgsFixedArgv`, `TestCloneArgsFailClosed`, `TestClonePathUnderCache`, `TestFlagNonHermetic`.
- **The hermetic fixture-set test is the SOLE authoritative proof** of the config.json mapping + 2-attempt protocol; the live clone (`TestLiveCloneAtPinnedSha`) `t.Skip`s offline / without `HELIX_BENCH_NETWORK`.
- TDD gates present: `test(85-07)` (2 commits) + `feat(85-07)` (2 commits).

## Live Verification (run once during execution, then re-gated)

The network-gated live leg was run ONCE with `HELIX_BENCH_NETWORK=1` to confirm the clone path works end-to-end:
- `TestLiveCloneAtPinnedSha` **PASS** — the resolved checkout HEAD == `PinnedSHA` (7e0611e7...).
- The real dataset structure was confirmed: **6 tracks** (cpp/go/java/javascript/python/rust) totalling **225 practice exercises** (cpp 26 + go 39 + java 47 + javascript 49 + python 34 + rust 30 = 225 — exactly the ADAPTER-AIDER-01 acceptance count), and the real `python/.../wordy/.meta/config.json` `files` block (`solution:[wordy.py]`, `test:[wordy_test.py]`, `example:[.meta/example.py]`) matches the loader's mapping byte-for-byte.

## Toolchain/Network-Gated (NOT falsely claimed complete)

Per the Validation Strategy "Toolchain-Gated Verifications" + Open Q3, the following are recorded as gated, NOT asserted here (like Phase 84's deferred live-mirror):
- **SC#1 full 225-task run + per-language sanity pass-rate comparison.** This needs the live clone + the pinned model + all 6 native toolchains (pytest, cargo, go, gradle/JDK, node/jest, cmake/ctest) installed, plus per-task containerized execution via the Plan 02 `--network=none` seam. The decision logic (config.json mapping, 2-attempt protocol, language slicing, clone-by-sha) is built and proven hermetically; the live full run + the per-language comparison against the published aggregate Sonnet-direct number within a tolerance band (Open Q3), plus a committed one-time per-language baseline captured on first full run, are deferred to a toolchain-equipped host. The committed fixtures + live clone leg prove every decision boundary; only the toolchain execution of the 225 real tasks is gated.
- **Per-language toolchain images** with pre-resolved offline dep caches (so rust/java/javascript tasks run hermetically) are a separate build-artifact effort — until they exist, `flagNonHermetic` conservatively flags rust/java/javascript tracks (and any config `non_hermetic` marker) so the runner can route only hermetic tasks through `--network=none` and defer the flagged ones.

## ADAPTER-AIDER-01 Reconciliation

This plan completes ADAPTER-AIDER-01. Earlier plans touched it partially (85-02 landed the SC#3 `--network=none` container Run seam). The full dataset-loader adapter — pinned-sha clone, `.meta/config.json` mapping, and the 2-attempt + stderr-reprompt protocol — lands here. The REQUIREMENTS traceability row is updated from "In progress" to complete (the SC#1 full live run remains honestly toolchain-gated as above; the adapter contract is delivered and hermetically proven).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `git checkout --detach -- <sha>` is invalid git syntax**
- **Found during:** Task 2 (the network-gated live leg, run once to verify end-to-end).
- **Issue:** The initial `cloneArgs` checkout step emitted `git checkout --detach -- <sha>`, mirroring the `--` terminator used on the clone/fetch steps. git rejects this with `fatal: git checkout: --detach does not take a path argument` — after `--`, git treats the sha as a pathspec, not a commit-ish.
- **Fix:** Dropped the `--` from the checkout step only (`git checkout --detach <sha>`). Flag-smuggling on the sha remains closed because `isHexSHA1` already guarantees 40 hex chars with no leading `-`; the terminator was redundant there. The clone (URL/dest) and fetch (refspec) steps keep their `--` terminators.
- **Files modified:** bench/datasets/aider-polyglot/clone.go
- **Commit:** a6724905

## Known Stubs

None. The loader is fully wired (real config.json parsing, real file restore, real 2-attempt loop, real clone argv). The only deferred surface is the toolchain-gated live full run, documented above as honestly gated — not a stub.

## Threat Surface

All five threat-register dispositions are mitigated and proven:
- **T-85-07-01** (mutable-ref tampering) — clone pinned by sha via `cloneArgs`; `isHexSHA1` fail-closes a branch/tag before os/exec.
- **T-85-07-02** (exercise-name path traversal) — `validatePathSegment` (cloned from cell.go) on every exercise name + language + clone leaf; `clonePath` sha-validates the cache sub-segment.
- **T-85-07-03** (untrusted test code → network) — `flagNonHermetic` flags network-at-test-time tasks (SC#3); hermetic tasks route through the Plan 02 `--network=none` seam.
- **T-85-07-04** (env leak to git child) — `gitEnv` forwards only PATH/HOME/HELIX_CACHE_DIR (+ `GIT_TERMINAL_PROMPT=0`), never os.Environ().
- **T-85-07-05** (argv flag-smuggling) — fixed argv + `--` terminator on clone/fetch + `isValidGitURL` leading-'-' rejection + `isHexSHA1` on the sha.

No NEW security surface beyond the threat model was introduced.

## TDD Gate Compliance

Both tasks followed RED → GREEN. `test(85-07)` commits (34d99368, e471741f) precede their `feat(85-07)` counterparts (b5be7915, a6724905). No REFACTOR commits were needed.

## Self-Check: PASSED

All 6 created files exist on disk; all 4 per-task commit hashes (34d99368, b5be7915, e471741f, a6724905) are present in git history.
