---
phase: 50-toolchain-go1.25-bench-local
verified: 2026-05-02T00:00:00Z
status: passed
score: 5/5 success criteria verified
overrides_applied: 0
re_verification:
  previous_status: never_verified
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
deferred: []
human_verification: []
---

# Phase 50: toolchain-go1.25-bench-local Verification Report

**Phase Goal:** Helix builds/vets/tests green on `ubuntu-latest` with Go 1.25 + a compatible gopls, AND the benchmark harness is converted to a local-only flow with all hosted-CI bench plumbing removed.

**Requirements covered:** TOOL-01, TOOL-02

**Verified:** 2026-05-02
**Status:** passed
**Re-verification:** No — initial verification (this report formalises the audit-time soft gap from `.planning/v1.9-MILESTONE-AUDIT.md`).

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| #   | Truth                                                                                                              | Status     | Evidence |
| --- | ------------------------------------------------------------------------------------------------------------------ | ---------- | -------- |
| SC-1 | A CI job on `ubuntu-latest` with Go 1.25 completes `go build`, `go vet`, `go test` green; benchmarks NOT in CI    | ✓ VERIFIED | `.github/workflows/go-test.yml:14` `runs-on: ubuntu-latest`; `:28` `go-version: '1.25.x'`; `:93-97` `go vet ./...` then `go test ./... -count=1`. Three most recent runs on `main` all `success` (run IDs 25068895178, 25068848930, 25068609876 via `gh run list --workflow=go-test.yml`). No `bench` step exists in any workflow. |
| SC-2 | gopls v0.17.1 linux/amd64 incompatibility resolved with documented strategy (upgrade/patch/replacement) in CONTRIBUTING.md | ✓ VERIFIED | `CONTRIBUTING.md:240-244` "gopls Compatibility" subsection: documents floor `>=v0.21`, "not pinned in `go.sum` or `go.mod`", `go install golang.org/x/tools/gopls@latest` after Go upgrades. Strategy = upgrade (rely on gopls@latest with a documented floor). Same content cross-referenced in `USAGE.md:511-523` "gopls version compatibility". |
| SC-3 | `bench.yml` and `capture-baseline.yml` removed; `*-github-hosted.txt` baselines removed/relocated                    | ✓ VERIFIED | `ls .github/workflows/` returns: `codeql.yml codespell.yml docker.yml docs.yaml go-test.yml junie.yml pytest.yml release.yml` — neither `bench.yml` nor `capture-baseline.yml` exists. `ls test/bench/baselines/` returns only `README.md`; no `v1.*-github-hosted.txt` files. Original artifacts archived under `_superseded/` (8 files including original 50-01-PLAN.md, 50-02-PLAN.md). |
| SC-4 | Bench harness runs locally via documented commands (`make bench`, `make bench-baseline`); local-only flow documented in CONTRIBUTING.md and `test/bench/baselines/README.md` | ✓ VERIFIED | `Makefile:1` declares `.PHONY: ... bench bench-baseline`; `Makefile:44-48` define both targets (bench → `go test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...`; bench-baseline → same with `tee test/bench/baselines/local.txt`). `.gitignore:280` lists `test/bench/baselines/local.txt`. `CONTRIBUTING.md:130-157` "Running Benchmarks" — documents local-only with rationale. `test/bench/baselines/README.md:1-36` rewritten with local-only framing, `make bench-baseline` workflow, and benchstat usage. |
| SC-5 | Benchmarks tech-debt note in PROJECT.md removed/rewritten to reflect local-only stance                              | ✓ VERIFIED | `.planning/PROJECT.md:139` "Known tech debt" line now lists ONLY `rust-analyzer` rename quirk and `jdtls cold-start` indexing — bench tech debt removed. Lines 48-50 retain historical v1.2 ✓ entries (Phase 9/15) which are accurate v1.2-shipped facts. Line 127 ("Resolve Go 1.25 / gopls v0.17.1 ... unblocks CI bench gate") sits in the v1.9 milestone-goals subsection (Phase 50 itself was the resolution); not a current tech-debt claim. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact                                                  | Expected                                                       | Status     | Details |
| --------------------------------------------------------- | -------------------------------------------------------------- | ---------- | ------- |
| `.github/workflows/go-test.yml`                           | Sole Go CI gate; ubuntu-latest; Go 1.25.x; vet+test            | ✓ VERIFIED | 97 lines; lines 14, 28, 93-97 confirm. |
| `.github/workflows/bench.yml`                             | MUST NOT EXIST                                                 | ✓ VERIFIED | Absent — `ls .github/workflows/` confirms. |
| `.github/workflows/capture-baseline.yml`                  | MUST NOT EXIST                                                 | ✓ VERIFIED | Absent — same listing. |
| `test/bench/baselines/v1.*-github-hosted.txt`             | MUST NOT EXIST                                                 | ✓ VERIFIED | `ls test/bench/baselines/` returns only `README.md`. |
| `test/bench/cmd/benchgate/`                               | MUST NOT EXIST (per Plan 50-01 invariants)                     | ✓ VERIFIED | `find` for `benchgate*` returns no results. |
| `Makefile` `bench` + `bench-baseline` targets             | Both defined with `## ` self-doc                               | ✓ VERIFIED | Lines 1, 44-48; `make -n bench` and `make -n bench-baseline` both resolve per Plan 04 grep matrix. |
| `test/bench/baselines/README.md`                          | Local-only framing; mentions `local`, `gitignored`, `make bench` | ✓ VERIFIED | 36 lines; explicit "Benchmarks run locally only", `make bench-baseline`, `local.txt`, `benchstat@latest`. |
| `CONTRIBUTING.md` "Running Benchmarks" section            | Local-only with rationale; `make bench`; `>=v0.21` gopls floor | ✓ VERIFIED | Lines 130-157 (benchmarks) + 240-244 (gopls). |
| `CONTRIBUTING.md` "gopls Compatibility" subsection        | Documents resolution strategy                                  | ✓ VERIFIED | Lines 240-244. |
| `USAGE.md` "gopls version compatibility" subsection       | Cross-doc mirror of resolution strategy                        | ✓ VERIFIED | Lines 511-523. |
| `.planning/PROJECT.md` tech-debt line                     | Bench/CI tech debt removed; rust-analyzer + jdtls retained     | ✓ VERIFIED | Line 139 trimmed; Plan 50-04 grep matrix gates `! grep 'GrammarRegistry instances'` and `! grep 'CI ubuntu-latest'` both PASS. |
| `go.mod` Go version pin                                   | `go 1.25.x`                                                    | ✓ VERIFIED | Line 3: `go 1.25.1`. |
| `.gitignore` for local baseline                           | `test/bench/baselines/local.txt` ignored                       | ✓ VERIFIED | Line 280. |
| `_superseded/` archive of original Phase 50 design        | Holds the abandoned CI-bench-gate artifacts                    | ✓ VERIFIED | Contains 50-01-PLAN, 50-02-PLAN, 50-CONTEXT, 50-DISCUSSION-LOG, 50-PATTERNS, 50-RESEARCH, 50-VALIDATION, README — confirms ROADMAP.md note (lines 119-121). |

### Key Link Verification

| From                                | To                                          | Via                                       | Status     | Details |
| ----------------------------------- | ------------------------------------------- | ----------------------------------------- | ---------- | ------- |
| `Makefile bench-baseline`           | `test/bench/baselines/local.txt`           | `tee test/bench/baselines/local.txt`      | ✓ WIRED    | Line 48; gitignored at `.gitignore:280`. |
| `test/bench/baselines/README.md`    | `make bench-baseline` workflow             | Documented invocation                     | ✓ WIRED    | README.md lines 12-22. |
| `CONTRIBUTING.md gopls subsection`  | Resolution strategy                         | Documented `>=v0.21` floor + reinstall    | ✓ WIRED    | CONTRIBUTING.md:240-244. |
| `USAGE.md gopls subsection`         | Cross-doc consistency                       | Same `>=v0.21` floor                      | ✓ WIRED    | USAGE.md:511-523. |
| `go-test.yml`                       | Go 1.25 toolchain                           | `actions/setup-go@v5` go-version: 1.25.x | ✓ WIRED    | go-test.yml:26-28. |

### Behavioral Spot-Checks

| Behavior                                                      | Command                                                                                              | Result                                       | Status |
| ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | -------------------------------------------- | ------ |
| go-test.yml workflow has run green on ubuntu-latest recently  | `gh run list --workflow=go-test.yml --limit=3 --json conclusion,headBranch,url`                      | 3/3 most-recent runs on `main` = `success` (run IDs 25068895178, 25068848930, 25068609876) | ✓ PASS |
| `go vet` clean on production tree (ignoring untracked tmp/)   | `go vet ./internal/... ./cmd/... ./test/...`                                                         | Only one pre-existing CGO `TOKEN_COUNT` macro-redefined warning in `internal/treesitter/bindings/swift/src/scanner.c:5` (documented in CLAUDE.md as a vendored-binding artefact; not a vet error) | ✓ PASS |
| `make -n bench` resolves                                       | `make -n bench`                                                                                      | Resolves per Plan 04 grep matrix gate        | ✓ PASS |
| `make -n bench-baseline` resolves                              | `make -n bench-baseline`                                                                             | Resolves per Plan 04 grep matrix gate        | ✓ PASS |
| `local.txt` ignored, `README.md` not ignored                   | `git check-ignore test/bench/baselines/local.txt` / same for README.md                              | Plan 04 matrix: ignore exit 0; README exit 1 | ✓ PASS |
| Workflow inventory: bench.yml / capture-baseline.yml absent    | `ls .github/workflows/`                                                                              | Returns 8 files; neither bench.yml nor capture-baseline.yml present | ✓ PASS |
| No `benchgate` references in active source/docs/CI             | `grep -rn benchgate --include='*.go' --include='*.md' --include='*.yml' --include='Makefile' --exclude-dir=_superseded --exclude-dir=.planning --exclude-dir=legacy .` | Plan 04 matrix: no matches                   | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s)        | Description                                                                                                  | Status     | Evidence                                          |
| ----------- | --------------------- | ------------------------------------------------------------------------------------------------------------ | ---------- | ------------------------------------------------- |
| TOOL-01     | 50-03, 50-04          | Helix builds/vets/tests green on ubuntu-latest with Go 1.25 + compatible gopls; gopls v0.17.1 root-cause documented; benches NOT on CI | ✓ SATISFIED | go-test.yml run 25068609876 green; CONTRIBUTING.md:240-244 + USAGE.md:511-523 document resolution; no bench job exists. |
| TOOL-02     | 50-01, 50-02, 50-03   | Bench harness local-only — bench.yml/capture-baseline.yml/`*-github-hosted.txt` removed; local `make bench`/`make bench-baseline`; CONTRIBUTING.md + baselines/README.md document local-only stance; PROJECT.md tech-debt rewritten | ✓ SATISFIED | All artefact rows above pass; Plan 04 grep matrix 28/28 PASS; PROJECT.md line 139 trimmed. |

No orphaned requirements. REQUIREMENTS.md maps TOOL-01 and TOOL-02 to Phase 50; both are claimed by the plans listed above.

### Anti-Patterns Found

| File                                  | Line | Pattern                                                                                          | Severity | Impact |
| ------------------------------------- | ---- | ------------------------------------------------------------------------------------------------ | -------- | ------ |
| `test/bench/memory_bench_test.go`     | 16   | Stale comment references `.github/workflows/bench.yml (added by Plan 09-06)` — workflow was deleted in Plan 50-01 | ℹ️ Info  | Doc-only drift; comment is "load-bearing documentation" but now misleading. Plan 50-04 SUMMARY explicitly flagged this as an out-of-scope discovery (matrix only greps `*.md`, not `*.go`) and recommended cleanup in a future polish phase. Does NOT affect runtime, build, vet, test, or any TOOL-01/02 success criterion. |
| `internal/treesitter/bindings/swift/src/scanner.c` | 5 | `TOKEN_COUNT` macro redefined under CGO compile | ℹ️ Info | Pre-existing across the v1.9 milestone; documented in CLAUDE.md "locally vendored Swift" line; not a Phase 50 regression and not a vet error. |

No blocker or warning anti-patterns introduced by Phase 50.

### Human Verification Required

None. All five Roadmap Success Criteria are observable in the codebase or in the recorded `gh run` history (run 25068609876 captured in 50-04-SUMMARY.md "Task 3 — CI run captured" subsection, supplemented by two further green runs 25068848930 and 25068895178 visible via `gh run list`).

### Gaps Summary

No gaps blocking Phase 50 goal achievement. The phase delivered both required outcomes:

1. **TOOL-01 — Go 1.25 / gopls unblock on ubuntu-latest:** `go-test.yml` is the sole Go CI gate, runs on `ubuntu-latest` with `actions/setup-go@v5` pinned at `1.25.x`, and the three most recent runs on `main` are all green. The gopls v0.17.1 root-cause is documented as an upgrade strategy (`>=v0.21` floor, no version pin) in both `CONTRIBUTING.md:240-244` and `USAGE.md:511-523`. Three live defects surfaced by Plan 50-04's verification gate (fresh-repo Actions trigger pipeline, Java 17→21 mismatch, symbol-position semantics + jdtls method-name lookup) were fixed in commits `1673e9de`, cherry-picks `91fd8bd7` + `38d925c1`, and `a8db8c17` — all captured in 50-04-SUMMARY.md.
2. **TOOL-02 — Local-only bench harness:** `bench.yml` and `capture-baseline.yml` are deleted (confirmed by directory listing); no `*-github-hosted.txt` baselines exist; `Makefile bench` and `bench-baseline` targets shipped; `.gitignore:280` excludes `local.txt`; `test/bench/baselines/README.md` rewritten; CONTRIBUTING.md "Running Benchmarks" section spells out the local-only stance and rationale; `.planning/PROJECT.md:139` tech-debt line trimmed.

The user-memory hard rule "benchmarks are local-only, never on CI" is honored — no current workflow runs benchmarks, and the project documentation explicitly forbids it.

The single info-level finding is the stale `bench.yml` comment in `test/bench/memory_bench_test.go:16`, already known to the team (recorded as a follow-up at the bottom of 50-04-SUMMARY.md). It is doc-only drift in a comment block and does not affect any TOOL-01 or TOOL-02 success criterion, build, vet, or test outcome — surfacing here for transparency, not as a blocker.

---

_Verified: 2026-05-02_
_Verifier: Claude (gsd-verifier)_
