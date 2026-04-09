---
phase: 09-benchmark-harness-v1-1-baseline
verified: 2026-04-08T00:00:00Z
status: human_needed
score: 4/6 must-haves verified
overrides_applied: 0
gaps:
  - truth: "The v1.1 baseline numbers are committed under test/bench/baselines/ and serve as the reference point for all v1.2 delta gates"
    status: partial
    reason: "Baseline file exists but is an intentional PLACEHOLDER with no real numbers. The plan's documented three-step rollout defers capture to the first ubuntu-latest CI run because benchmark numbers are architecture-specific and capturing on dev darwin would produce misleading values. Until the baseline candidate is committed via a follow-up re-baseline PR, no v1.2 delta gate can produce meaningful deltas."
    artifacts:
      - path: "test/bench/baselines/v1.1-github-hosted.txt"
        issue: "Contains only header stanza (goos/goarch/pkg) and a PLACEHOLDER comment block — zero Benchmark lines"
    missing:
      - "Real v1.1 baseline numbers captured from a first ubuntu-latest CI run, committed via a dedicated re-baseline PR"
  - truth: "A CI job runs benchstat against the committed baseline and fails any PR that regresses >10% time or >20% allocs at p<0.05"
    status: partial
    reason: "The CI workflow, benchgate binary, and thresholds are all implemented and unit-tested. However, bench.yml currently invokes benchgate in --warn-only mode, which always exits 0 regardless of breaches. The gate will not actually fail a PR until a follow-up PR removes --warn-only. Additionally, roadmap SC3 cites 10%/20% thresholds (release tier), but bench.yml defaults to PR tier (15%/25%); --release-tier switching is implemented but not wired into the PR workflow."
    artifacts:
      - path: ".github/workflows/bench.yml"
        issue: "Invokes `go run ./test/bench/cmd/benchgate --warn-only ...` — never blocks PRs in current state"
      - path: "test/bench/cmd/benchgate/main.go"
        issue: "Default thresholds in bench.yml use PR tier 15%/25%, not the 10%/20% release tier cited by roadmap SC3 (tier selection is implemented but release-tier is not the PR default by design per CONTEXT D-01)"
    missing:
      - "Follow-up PR removing --warn-only from bench.yml once the baseline is seeded"
      - "Clarification whether roadmap SC3 should cite the PR tier (15%/25%) or whether the workflow should be split by tier"
human_verification:
  - test: "Accept the two-step baseline rollout deviation from roadmap SC2/SC3"
    expected: "Human confirms that committing a PLACEHOLDER baseline + running benchgate in --warn-only mode satisfies Phase 9's 'one-way door' intent, with the understanding that the real baseline and blocking enforcement land in two follow-up PRs (not additional Phase 9 plans). This is explicitly documented in test/bench/baselines/README.md and the Plan 09-06 rollout section."
    why_human: "The deviation is deliberate and well-reasoned (architecture-specific baselines must be captured on the same runner that enforces the gate). It is documented in plan, summary, README, and bench.yml comments. But roadmap SC2/SC3 as literally worded are not satisfied today, so a human must accept the deferred completion path."
  - test: "Confirm roadmap SC3 threshold wording (10%/20%) vs implemented PR tier (15%/25%)"
    expected: "Either roadmap SC3 is updated to reference the tiered strategy (PR 15%/25%, release 10%/20%), or bench.yml is reconfigured to invoke benchgate with --release-tier on PR runs. CONTEXT.md D-01 locks the tiered approach, so the roadmap phrasing appears to be the drift."
    why_human: "Cannot decide programmatically whether the roadmap or the implementation is authoritative. D-01 says tiered; SC3 says 10%/20%."
  - test: "Run the full bench suite end-to-end on ubuntu-latest via workflow_dispatch and confirm all benchmarks execute and produce valid benchfmt output"
    expected: "BenchmarkTools produces 38 sub-benchmark lines; BenchmarkLSPIndex_Cold/Warm produce 2 lines; BenchmarkMemory produces its custom metric lines; heap snapshots upload as artifacts; benchgate warn-only report prints without error."
    why_human: "No ubuntu-latest runner available from this verifier session; only darwin-local compile/vet/unit-test smoke was run."
---

# Phase 9: Benchmark Harness & v1.1 Baseline Verification Report

**Phase Goal:** Capture the pre-instrumentation performance baseline so every subsequent phase can publish a measurable delta
**Verified:** 2026-04-08
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Developer can run `go test -bench=. ./test/bench/...` and get reproducible p50/p95/p99 numbers for all 38 tools, cold/warm LSP indexing, memory profiles | VERIFIED | BenchmarkTools with 38 sub-benches via `for _, tc := range benchTools`; BenchmarkLSPIndex_Cold + BenchmarkLSPIndex_Warm; BenchmarkMemory with 4 scenarios; benchstat (-alpha 0.05) consumption wired via `make bench-stat` and bench.yml; p99 is reported by benchstat (phase Q5 says reported but not gated) |
| 2 | v1.1 baseline numbers committed under test/bench/baselines/ as reference for v1.2 delta gates | PARTIAL (placeholder) | `test/bench/baselines/v1.1-github-hosted.txt` exists but contains only header + PLACEHOLDER comment block; no Benchmark lines. Three-step rollout documented in README and comment; real numbers deferred to first CI run. |
| 3 | CI job runs benchstat against committed baseline and fails any PR regressing >10% time / >20% allocs at p<0.05 | PARTIAL (warn-only) | `.github/workflows/bench.yml` runs benchstat and benchgate on every PR, with GOMAXPROCS=4, pinned gopls, `-short` to skip full-repo, `-count=10`. But benchgate is invoked with `--warn-only` which always exits 0. Also thresholds default to PR tier 15%/25%, not the 10%/20% release tier cited in SC3. |
| 4 | All benchmarks use `testing.B.Loop` (compiler cannot elide hot path) | VERIFIED | `for b.Loop()` present in tools_bench_test.go, lsp_index_bench_test.go, memory_bench_test.go, fullrepo_smoke_test.go (4 files); `for i := 0; i < b.N` returns 0 matches across test/bench |

**Score:** 2/4 roadmap SCs fully verified; 2/4 partial (intentional deferrals).

### PLAN Frontmatter Truths

| # | Truth | Status |
|---|-------|--------|
| Plan 01 / 1 | `testing.TB`-typed StartTestDaemon/PrepareFixture/callTool/callToolExpectError/listSessionTools/requireGopls callable from benchmarks | VERIFIED (`grep tb testing.TB` returns 5 matches in harness.go, 4 in helpers.go) |
| Plan 01 / 2 | Existing integration tests still compile unchanged | VERIFIED per 09-01 SUMMARY |
| Plan 01 / 3 | TestDaemon struct stores `tb testing.TB` | VERIFIED |
| Plan 02 / 1 | `go test -bench=. ./test/bench/...` compiles with no benchmarks needed | VERIFIED (`go vet ./test/bench/...` clean) |
| Plan 02 / 2 | TestMain starts one daemon, goroutine leak assertion present | VERIFIED (main_test.go contains leak check with grace loop) |
| Plan 02 / 3 | `rss.CurrentRSS()` returns non-zero uint64 on linux/darwin, compiles elsewhere | VERIFIED (rss_{linux,darwin,other}.go present with build tags) |
| Plan 02 / 4 | tools_manifest benchTools matches live registry | VERIFIED (TestBenchToolsManifestMatchesRegistry exists and passes per summary; manifest has exactly 38 entries — note the file is `tools_manifest_test.go`, not the plan's literal `tools_manifest.go`, a deliberate package-layout deviation documented in 09-02-SUMMARY) |
| Plan 03 / 1 | BenchmarkTools produces 38 sub-benchmark lines | VERIFIED (table-driven over benchTools) |
| Plan 03 / 2 | 3 warmup calls before b.Loop | VERIFIED (`for i := 0; i < 3` present) |
| Plan 03 / 3 | Every iteration uses `testing.B.Loop` | VERIFIED |
| Plan 03 / 4 | Edit tool sub-benchmarks use per-iteration mutable copy with StopTimer/StartTimer fences | VERIFIED (isEditTool map + needsCopy dispatch + prepareGoFixtureCopyB + StopTimer/StartTimer present) |
| Plan 04 / 1 | BenchmarkLSPIndex_Cold spins fresh daemon in loop with StopTimer/StartTimer, runs -benchtime=1x | VERIFIED |
| Plan 04 / 2 | BenchmarkLSPIndex_Warm reuses single daemon | VERIFIED |
| Plan 04 / 3 | BenchmarkFullRepoSmoke skips under testing.Short() | VERIFIED |
| Plan 04 / 4 | Cold-start metric labeled/reported separately from warm | VERIFIED (separate functions) |
| Plan 05 / 1 | BenchmarkMemory reports RSS at s1_idle/s2_workspace/s3_first_call/s4_post_100_calls | VERIFIED (grep returns all 4 labels) |
| Plan 05 / 2 | Each scenario writes pprof heap snapshot with scenario-sha naming | VERIFIED (4 heapSnapshot calls; 4 .pb.gz files present in test/bench/pprof/) |
| Plan 05 / 3 | RSS reported from both runtime.ReadMemStats and rss.CurrentRSS | VERIFIED (reportMemory helper dual-reports) |
| Plan 05 / 4 | .gitignore excludes *.pb.gz except baseline-*.pb.gz | VERIFIED |
| Plan 06 / 1 | `make bench` + `make bench-stat` targets exist | VERIFIED (Makefile lines 34, 42) |
| Plan 06 / 2 | CI workflow runs the suite and invokes benchgate with tiered thresholds at p<0.05 | PARTIAL (warn-only; see SC3 gap) |
| Plan 06 / 3 | v1.1 baseline file exists as placeholder; first CI run populates real numbers via re-baseline PR | VERIFIED-BY-DESIGN (placeholder present, README documents the rollout) |
| Plan 06 / 4 | benchgate parses benchfmt-native output | VERIFIED (imports golang.org/x/perf/benchfmt + benchmath) |
| Plan 06 / 5 | p99 reported but not gated | VERIFIED (benchgate only gates sec/op and allocs/op) |

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `test/integration/harness.go` | VERIFIED | `tb testing.TB` on 5 functions |
| `test/integration/helpers.go` | VERIFIED | `tb testing.TB` on 4 functions |
| `test/bench/bench_helpers_test.go` | VERIFIED | 341 lines; startBenchDaemon, prepareGoFixtureB, prepareGoFixtureCopyB, activateWorkspaceB, callToolB, requireGoplsB present (renamed from plan's `bench_helpers.go` — deliberate XTestGoFiles layout fix) |
| `test/bench/main_test.go` | VERIFIED | 140 lines; TestMain + TestBenchToolsManifestMatchesRegistry |
| `test/bench/tools_manifest_test.go` | VERIFIED | 230 lines; benchTools slice with 38 entries (confirmed via line-level read + SUMMARY 09-02 parity test passing) |
| `test/bench/rss/rss.go` + rss_linux.go + rss_darwin.go + rss_other.go | VERIFIED | All 4 files present with correct build tags; rss_test.go passes on darwin |
| `test/bench/tools_bench_test.go` | VERIFIED | 221 lines; BenchmarkTools with table-drive, warmup, b.Loop, isEditTool dispatch |
| `test/bench/lsp_index_bench_test.go` | VERIFIED | 73 lines; BenchmarkLSPIndex_Cold + BenchmarkLSPIndex_Warm |
| `test/bench/fullrepo_smoke_test.go` | VERIFIED | 74 lines; BenchmarkFullRepoSmoke with testing.Short gate |
| `test/bench/memory_bench_test.go` | VERIFIED | 120 lines; 4-scenario BenchmarkMemory with dual RSS |
| `test/bench/heap_snapshot_test.go` | VERIFIED | 74 lines (renamed from plan's `heap_snapshot.go` — same package-layout reason; exports heapSnapshot + gitShortSHA) |
| `test/bench/pprof/.gitkeep` + `.gitignore` | VERIFIED | .gitignore excludes *.pb.gz except baseline-*.pb.gz |
| `test/bench/pprof/s[1-4]-*-*.pb.gz` | VERIFIED | 4 snapshot files present on disk (d265c017 commit sha) |
| `test/bench/cmd/benchgate/main.go` | VERIFIED | 375 lines; benchfmt + benchmath imports, all flags (time/allocs/alpha/release-tier/warn-only), Welch's t-test via AssumeNormal.Compare |
| `test/bench/cmd/benchgate/main_test.go` | VERIFIED | 278 lines; 12+ test cases; `go test -count=1 ./test/bench/cmd/benchgate/...` passes (verified this session) |
| `test/bench/baselines/v1.1-github-hosted.txt` | PARTIAL | Placeholder only — no benchmark lines |
| `test/bench/baselines/README.md` | VERIFIED | Documents rollout + capture procedure + GOMAXPROCS + tiered thresholds + re-baseline policy |
| `.github/workflows/bench.yml` | VERIFIED (with warn-only caveat) | ubuntu-latest, GOMAXPROCS=4, gopls@v0.17.1 pinned, -short, -count=10, benchgate --warn-only, upload-artifact, timeout-minutes=30, permissions:contents:read |
| `Makefile` bench + bench-stat targets | VERIFIED | Both present, existing targets untouched |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| test/bench/main_test.go TestMain | integration harness (TB-typed) | TB-generic bench helpers re-implement daemon startup (plan's documented deviation — not an import) | WIRED |
| tools_manifest_test.go benchTools | live MCP tool registry | `TestBenchToolsManifestMatchesRegistry` via `bd.RegistryNames()` (bypass ProfileFilterMiddleware) | WIRED (SUMMARY 09-02 documents the `Registry().Names()` fix) |
| tools_bench_test.go | bench_helpers_test.go prepareGoFixtureCopyB | isEditTool/needsCopy dispatch | WIRED |
| lsp_index_bench_test.go | startBenchDaemon + Stop | per-iteration daemon lifecycle in Cold | WIRED |
| memory_bench_test.go | rss.CurrentRSS + runtime.ReadMemStats | reportMemory dual-labels | WIRED |
| bench.yml | benchgate | `go run ./test/bench/cmd/benchgate --warn-only ...` | WIRED (warn-only mode by design) |
| benchgate | golang.org/x/perf/benchfmt + benchmath | Welch's t-test via AssumeNormal.Compare | WIRED |
| baselines/v1.1-github-hosted.txt | BenchmarkTools, LSPIndex_Warm, Memory | expected to contain benchfmt lines | NOT YET WIRED (placeholder only) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go vet ./test/bench/...` | clean | exit 0 | PASS |
| `go build ./test/bench/cmd/benchgate/...` | compiles | exit 0 | PASS |
| `go test -count=1 ./test/bench/cmd/benchgate/...` | 12 unit tests | `ok ... 0.316s` | PASS |
| `for b.Loop()` present in 4 bench files | grep | 4 files | PASS |
| `for i := 0; i < b.N` absence | grep | 0 matches | PASS |
| 4 pprof snapshots on disk | ls test/bench/pprof/s*.pb.gz | 4 files present | PASS |
| Running full bench suite on ubuntu-latest | skipped | n/a | SKIP (no ubuntu runner; routed to human) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| BENCH-01 | 09-01, 09-02 | Benchmark harness in test/bench using testing.B.Loop (Go 1.25) | SATISFIED | test/bench package compiles; all benches use b.Loop; scaffold + TestMain + manifest in place |
| BENCH-02 | 09-03 | Tool response time benchmarks for all 38 tools (p50/p95/p99) | SATISFIED | BenchmarkTools with 38-sub-bench table-drive; p-quantiles come from benchstat over -count=10 |
| BENCH-03 | 09-04 | LSP indexing throughput (cold/warm) for Go fixture | SATISFIED | BenchmarkLSPIndex_Cold + BenchmarkLSPIndex_Warm + BenchmarkFullRepoSmoke |
| BENCH-04 | 09-05 | Memory profile benchmarks (baseline, per-workspace, per-LS-worker) | SATISFIED | BenchmarkMemory 4 D-06 scenarios + dual RSS + pprof snapshots |
| BENCH-05 | 09-06 | CI benchstat regression gate with tiered thresholds at p<0.05 | PARTIAL | Gate code + workflow shipped and tested; currently runs in --warn-only mode pending baseline seed. Enforcement flips with follow-up PR. Roadmap SC3 threshold wording (10%/20%) diverges from D-01 PR tier (15%/25%) — needs clarification. |
| BENCH-06 | 09-06 | v1.1 baselines committed | PARTIAL | File committed as placeholder; real numbers deferred to first CI capture per documented three-step rollout. |

REQUIREMENTS.md currently marks BENCH-02 and BENCH-04 as complete; BENCH-01/03/05/06 still pending. The Phase 9 implementation actually satisfies BENCH-01 and BENCH-03 as well — REQUIREMENTS.md should be updated to reflect this (documentation drift, not an implementation gap).

### Anti-Patterns Found

None. Scan of test/bench files found no TODO/FIXME/placeholder markers in benchmark logic. The PLACEHOLDER marker in `v1.1-github-hosted.txt` is intentional and documented; it is not a stub in the bug-smell sense — it is a load-bearing artifact of the two-step rollout.

No hardcoded-empty data flowing into user-visible paths. `for i := 0; i < 3` warmup in tools_bench_test.go is the intentional 3-call priming window, not a stub.

No `for i := 0; i < b.N; i++` patterns (plan forbids them; grep confirms 0 matches).

### Human Verification Required

See frontmatter `human_verification` section. Three items:

1. **Accept the two-step baseline rollout deviation from roadmap SC2/SC3.** The deviation is deliberate, documented in README + bench.yml comments + plans + summaries, and well-reasoned (architecture-specific baselines must come from the runner that enforces the gate). Phase 9's "one-way door" commitment — gate code enforced + baseline file committed + rollout codified — is met under the documented interpretation, but roadmap SC2/SC3 as literally worded are not satisfied in the current snapshot.
2. **Reconcile roadmap SC3 wording with D-01 tiered thresholds.** SC3 says 10%/20% (release tier); D-01 and the implemented default say 15%/25% (PR tier). Either the roadmap or the workflow wording needs to change to match the other.
3. **End-to-end ubuntu-latest dry run.** Compile/vet/unit-test smoke was run on darwin this session, but the full `go test -bench=. -count=10` suite on ubuntu-latest has not been exercised — that happens on the first bench.yml run.

### Gaps Summary

Phase 9 ships the entire benchmark harness, all four benchmark classes (tools, LSP index, memory, full-repo smoke), the benchgate CLI with unit tests, the CI workflow, the Make targets, and the baselines README. The two partial items (SC2 baseline and SC3 enforcement) are by design and deliberately deferred to follow-up PRs per the three-step rollout documented in `test/bench/baselines/README.md` and Plan 09-06. The documented rollout is sound and addresses a real environmental concern (benchmark numbers are architecture-specific), but it trades away the literal reading of the roadmap SCs in exchange for correctness. A human must decide whether to accept this interpretation.

No blocking anti-patterns were found. All plans compile and pass the targeted unit tests. REQUIREMENTS.md traceability marks need a minor refresh (BENCH-01 and BENCH-03 are actually satisfied but still marked Pending).

---

*Verified: 2026-04-08*
*Verifier: Claude (gsd-verifier)*
