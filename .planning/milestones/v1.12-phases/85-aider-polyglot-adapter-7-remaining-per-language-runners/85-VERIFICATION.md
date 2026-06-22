---
phase: 85-aider-polyglot-adapter-7-remaining-per-language-runners
verified: 2026-06-21T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: "Initial verification — no prior VERIFICATION.md existed."
gaps: []
deferred: []
notes:
  - "REQUIREMENTS.md bottom traceability table rows for TOOLBENCH-03/04/05/06/07 still read 'Pending'/'TBD' even though the requirement-definition checkboxes (lines 34-39) are [x] and the runners are fully implemented, registered, tested, and threshold-met. This is a STALE BOOKKEEPING ROW, not a functional gap — the code is the source of truth and is complete. Recommend updating those five rows to 'Complete / 85-03|85-05' for traceability hygiene (non-blocking)."
---

# Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners — Verification Report

**Phase Goal:** Aider Polyglot dataset-loader-only adapter (225 Exercism tasks, pinned-sha shallow clone, 2-attempt stderr-reprompt) + the 7 remaining ToolBench language runners (Python/TS/JS/Java/C#/C++/Rust) each implementing the Phase 78 LanguageRunner; per-language toolchain images + `--network=none`; Exercism license audit + `make verify-licenses`.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + verification-context must-haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC#1 | Aider Polyglot dataset-loader-only adapter: pinned-sha shallow clone (validated), config.json→tasks mapping, 2-attempt stderr-reprompt, per-language pass-rate sliceable; live full run honestly gated | ✓ VERIFIED | `bench/datasets/aider-polyglot/{loader,clone,pin}.go` build + all hermetic tests green (`TestLoadExercise`, `TestTwoAttemptReprompt/PassFirst/FailBoth`, `TestCloneArgsFixedArgv/FailClosed`, `TestRestorePristine`). Plan-01 language axis sliceable: `TestAggregateByLanguagePassRate` (python=1.0/rust=0.0), `TestLanguageEmittedWhenSet/OmittedWhenEmpty/BackwardCompatValidate` green. `PinnedSHA = 7e0611e77b54...` (40-hex, `isHexSHA1`-validated). Live full-run honestly gated: `TestLiveCloneAtPinnedSha` SKIPs offline, asserts HEAD==pin when run. |
| SC#2 | All 7 languages have `bench/languages/<L>/runner.go` implementing LanguageRunner; per-language thresholds met; HERMETIC golden-fixture parser is SOLE authoritative proof; `Passed = exitCode==0` | ✓ VERIFIED | All 7 (+go) carry `var _ languages.LanguageRunner = (*XRunner)(nil)`, register `(internal-toolbench, <lang>)`, and gate `Passed: exitCode == 0`. All 7 golden parser tests run hermetically (no SKIP) and pass. Thresholds: C++≥6, Rust≥8, Java≥8, C#≥6, Python≥8, TS≥8, JS≥8 — all PASS via `coverage_*_test.go`. Java fixture has real provenance (apache/maven-surefire@5ee132b4, `TestJavaFixtureProvenance` gate). Rust parses libtest TEXT, `TestRustDoesNotUseMessageFormatJSON` forbids `--message-format json`. |
| SC#3 | `container.Run(--network=none)` seam exists + argv-asserted hermetically; non-hermetic tasks flagged; toolchain-image live runs honestly gated | ✓ VERIFIED | `bench/container/run.go` `runArgs(...netNone)` emits `--network=none` IFF netNone; `TestRunArgsNetworkNoneFixedArgv` + `TestRunArgsNetworkFlagIsExactlyThePolicy` green (no live engine). `flagNonHermetic` flags rust/java/javascript + config `non_hermetic`; `TestFlagNonHermetic` green. Pre-baked toolchain-image live runs honestly recorded as gated in 85-07-SUMMARY. |
| SC#4 | `LICENSE-AUDIT.md` per-track sha256 + redistribution clause; `make verify-licenses` is a green hard-fail gate | ✓ VERIFIED | 6 tracks (cpp/go/java/javascript/python/rust), each with `license_sha256` + `redistribution_clause_excerpt`. `make verify-licenses` exits 0. Hard-fail proven LIVE: blanking a sha256 → exit 1 (`track "cpp" ... has an empty license_sha256 field`); restored → exit 0. Validator uses `KnownFields(true)` strict decode + `(count, error)` signature; negative-path tests green (empty license, empty sha256, unknown key, zero tracks). |

**Score:** 4/4 truths verified

### Code-Review Warning Fixes (regression-checked against SCs)

| Warning | Fix in live code | Regression test | Status |
|---------|------------------|-----------------|--------|
| WR-01: restorePristine never wired into driver loop (anti-tamper gap) | `loader.go:243-249` — `restorePristineTests` called AFTER agent edit, BEFORE each `runTests`, gated on `ex.SrcDir != ""` | `TestRunExerciseRestoresTamperedTest` — tampering agent writes `# TAMPERED` into test file; grader asserts on-disk content is pristine at grade time (`sawTamperAtGrade` fails if restore absent); also asserts solution edit preserved | ✓ FIXED (substantive, not vacuous) |
| WR-02: Rust acceptance argv must run `#[ignore]`-gated tests | `loader.go:292` — `nativeTestCommand("rust")` = `cargo test -- --include-ignored` (flag after `--` so libtest receives it) | `TestRustAcceptanceArgvRunsIgnoredTests` — asserts `--include-ignored` present AND positioned after the `--` terminator | ✓ FIXED |
| WR-03: cloneArgs dest-leaf guard silently accepted hostile leaves | `clone.go:91-93` — now `return nil, err` propagating `validatePathSegment` directly (no narrower swallowing re-check); leading-dot leaf `.git`/`.ssh` rejected | `TestCloneArgsRejectsHostileLeaf` — covers `.git`, `.ssh`, `.`, `..` leaves | ✓ FIXED |

No SC regressed: all four SCs verified above with the fixes in place.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/datasets/aider-polyglot/loader.go` | loader + 2-attempt protocol + restorePristineTests wired | ✓ VERIFIED | restorePristineTests wired into loop; native commands; flagNonHermetic |
| `bench/datasets/aider-polyglot/clone.go` | pinned-sha `--depth 1` clone + hostile-leaf guard | ✓ VERIFIED | fixed argv, sha pin, gitEnv allowlist, WR-03 guard |
| `bench/datasets/aider-polyglot/pin.go` | pinned sha constant + repo URL | ✓ VERIFIED | `PinnedSHA = 7e0611e7...`, `RepoURL = Aider-AI/polyglot-benchmark`, `isHexSHA1` |
| `bench/container/run.go` | runArgs + Run(--network=none) seam | ✓ VERIFIED | netNone IFF flag; fail-close validation; allowlistEnv; `--` terminator |
| `bench/languages/{python,typescript,javascript,java,csharp,cpp,rust}/runner.go` | 7 LanguageRunners | ✓ VERIFIED | all conform, register, `Passed=exitCode==0`, hermetic golden fixtures |
| `cmd/helix-bench/verify_licenses.go` | strict-decode hard-fail validator | ✓ VERIFIED | KnownFields(true), (count,error), fail-closed |
| `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` | per-track sha256 + redistribution clause | ✓ VERIFIED | 6 tracks complete |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `RunExercise` loop | `restorePristineTests` | called AFTER edit, BEFORE grade (WR-01) | ✓ WIRED |
| `nativeTestCommand("rust")` | libtest `--include-ignored` | flag after `--` (WR-02) | ✓ WIRED |
| `cloneArgs` dest-leaf | `validatePathSegment` rejection | `return nil, err` propagated (WR-03) | ✓ WIRED |
| `runArgs(netNone=true)` | `--network=none` argv | append IFF netNone | ✓ WIRED |
| Makefile `verify-licenses:` | `cmd/helix-bench verify-licenses` | `go run ./cmd/helix-bench verify-licenses LICENSE-AUDIT.md` | ✓ WIRED |
| each runner `init()` | `languages.Register` | `(internal-toolbench, <lang>)` | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full module builds | `go build ./...` | exit 0 | ✓ PASS |
| All bench tests green | `go test -count=1 ./bench/...` | all packages ok | ✓ PASS |
| Full vet gate (incl. 6 leakage vettools) | `make vet` | exit 0 | ✓ PASS |
| License gate green | `make verify-licenses` | `... OK`, exit 0 | ✓ PASS |
| License gate hard-fails on tamper | blank a `license_sha256` → `go run ./cmd/helix-bench verify-licenses ...` | exit 1 ("empty license_sha256") | ✓ PASS |
| 3 WR regression tests | `go test -run 'TestRunExerciseRestoresTamperedTest|TestRustAcceptanceArgvRunsIgnoredTests|TestCloneArgsRejectsHostileLeaf'` | all PASS | ✓ PASS |
| 7 hermetic golden parsers | `go test -run Golden` (per language) | 7/7 PASS, no SKIP | ✓ PASS |
| Capability thresholds | `go test -run 'Capabilities'` | 7/7 PASS | ✓ PASS |
| `--network=none` argv | `go test -run NetworkNone` | PASS (no live engine) | ✓ PASS |
| Live clone honestly gated | `go test -run TestLiveCloneAtPinnedSha` (offline) | SKIP (clean) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ADAPTER-AIDER-01 | 85-01, 85-02, 85-06, 85-07 | Aider Polyglot dataset-loader-only adapter | ✓ SATISFIED | loader/clone/pin + language axis + container seam + license gate; hermetic proof complete, live run gated |
| TOOLBENCH-03 | 85-03 | Python `pytest --json-report` ≥8/10 | ✓ SATISFIED | PyRunner + golden + Python≥8 coverage PASS |
| TOOLBENCH-04 | 85-05 | TypeScript `vitest --reporter=json` ≥8/10 | ✓ SATISFIED | TSRunner + golden + TS≥8 coverage PASS |
| TOOLBENCH-05 | 85-05 | JavaScript `jest --json` ≥8/10 | ✓ SATISFIED | JSRunner + golden + JS≥8 coverage PASS |
| TOOLBENCH-06 | 85-03 | Java `mvn test` surefire ≥8/10 | ✓ SATISFIED | JavaRunner + real-provenance golden + Java≥8 coverage PASS |
| TOOLBENCH-07 | 85-03 | C# `dotnet test --logger trx` ≥6/10 | ✓ SATISFIED | CSharpRunner + TRX golden + C#≥6 coverage PASS |
| TOOLBENCH-08 | 85-04 | C++ `cmake/ctest` ≥6/10 | ✓ SATISFIED | CppRunner + ctest JUnit golden + C++≥6 coverage PASS |
| TOOLBENCH-09 | 85-04 | Rust `cargo test` libtest TEXT ≥8/10 | ✓ SATISFIED | RustRunner + libtest TEXT golden + message-format guard + Rust≥8 coverage PASS |

Every requirement ID in plan frontmatter (`ADAPTER-AIDER-01`, `TOOLBENCH-03..09`) is accounted for and verified in the codebase. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No unreferenced TBD/FIXME/XXX debt markers in phase source; no stub `return nil,nil`/`panic()` in runners; the Java test's "TODO" string is a placeholder the provenance gate guards AGAINST, not a debt marker | ℹ️ Info | None |

### Documentation Hygiene Note (non-blocking)

REQUIREMENTS.md's bottom traceability table still lists TOOLBENCH-03/04/05/06/07 as `Pending` / Plan `TBD`, while:
- their requirement-definition checkboxes (lines 34-39) are `[x]`,
- the runners are fully implemented, registered, tested, and threshold-met (verified above),
- plan frontmatter claims them (85-03: 03/06/07; 85-05: 04/05).

This is a stale bookkeeping row, NOT a functional gap — the code is complete and the source of truth. Recommend updating those five rows to `Complete / 85-03|85-05`. It does not affect goal achievement and is not a blocker.

### Human Verification Required

None. All four SCs are hermetically provable in-tree; the live full-run / per-language sanity comparison (SC#1) and pre-baked toolchain-image runs (SC#3) are ROADMAP-sanctioned toolchain/network-gated boundaries that are honestly recorded as gated (not falsely claimed complete), and the decision logic for both is hermetically proven. Per the verification context, `passed` is appropriate here.

### Gaps Summary

No gaps. All 4 must-haves verified against the live codebase. The 3 code-review warnings (WR-01 restorePristineTests wired, WR-02 ignored-tests argv, WR-03 hostile-leaf guard) are fixed in live code, each backed by a substantive (non-vacuous) regression test, with no SC regression. `go build`, `go test ./bench/...`, `make vet`, and `make verify-licenses` all green; license gate proven to hard-fail on tamper. The only finding is a stale REQUIREMENTS.md traceability row (documentation hygiene, non-blocking).

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
