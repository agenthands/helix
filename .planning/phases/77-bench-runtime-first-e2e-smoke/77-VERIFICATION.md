---
phase: 77-bench-runtime-first-e2e-smoke
verified: 2026-06-17T09:40:00Z
status: passed
score: 4/4 roadmap success criteria verified (16/16 plan must-have truths)
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
---

# Phase 77: Bench Runtime & First E2E Smoke — Verification Report

**Phase Goal:** Stand up the bench orchestrator end-to-end on a single Go ToolBench task in a single mode (`your_agent_full`) — no container, no per-language sprawl — so the runtime shape (subprocess daemon spawn → agent → schema-valid result.v2.json → merged OTel trace) is forced into existence and proven before evaluators (Phase 79) and ablation runners (Phase 80) land on top.
**Verified:** 2026-06-17T09:40:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

This phase was verified for REAL, not by SKIP. Both binaries were built
(`/tmp/hxv/helix`, `/tmp/hxv/helix-bench`) and the integration tests + E2E smoke
were executed with `HELIX_BIN` resolvable, so `TestDaemonTap` and `TestCrossCell`
ran their full spawn→drive→tap→merge path rather than skipping.

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| - | ----- | ------ | -------- |
| 1 | `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one>` ≤30s, writes schema-valid `result.v2.json`; one daemon subprocess per (task×mode); no port collisions on `--parallel=4` | ✓ VERIFIED | Real run completed in **0.41s** (`/usr/bin/time -v` wall clock 0:00.41). Emitted `result.v2.json` carries `schema_version:"v2"`, `outcome:"success"`, `fairness`, `trace_ref`, `model_id`. `TestResultV2*` schema tests pass. One Unix socket per cell, HTTP disabled (`subprocess/daemon.go:28-37`), no TCP port → no collisions by construction. `TestCrossCell` (parallel N≥2 cells) PASS in 0.34s. |
| 2 | `make bench-quick` exits 0 with ≥1 task succeeding ≤90s; `make bench`/`make bench-<suite>` invoke `cmd/helix-bench run --benchmarks=…` | ✓ VERIFIED | `make bench-quick` → **EXIT_CODE=0, ELAPSED=4s**, "1/1 cells succeeded". `make -n bench` = `go run ./cmd/helix-bench run --benchmarks=toolbench-go`. `make -n bench SUITE=toolbench-go` resolves the suite. `make -n bench-micro` is the ORIGINAL Go microbench (`go test -bench=.`) — collision reconciled (Pitfall 1). |
| 3 | `bench/runtime/sandbox/` thin-wraps `internal/eval/sandbox/`; `bench/runtime/subprocess/` spawns `helix daemon` with PID-gated per-cell JSONL paths | ✓ VERIFIED | `sandbox/sandbox.go:32-33` embeds `*evalsandbox.Sandbox` (struct embed, no fork — D-07). `subprocess/daemon.go:37-41` delegates to `sb.StartDaemon` over per-cell Unix socket. PID-gated tap: `DaemonTapResult.RejectedForeignPid==0` asserted per cell; merged trace shows single daemon PID (1390955) with `source:daemon`/`source:cc` legs. |
| 4 | First E2E smoke produces a merged OTel trace (daemon-tap + agent-tap) with zero orphan spans and zero PID cross-talk | ✓ VERIFIED | Emitted `trace.json`: `tool_call_summary.total=2` (`activate_project` daemon leg + `replace_in_file` cc leg), both `source:daemon` and `source:cc` events present, single daemon PID, no foreign tools. `TestCrossCell` asserts `RejectedForeignPid==0` + no foreign tool names + `ToolCallTotal>=1` + `CCLegPresent` + `ResultValid`. |

**Score:** 4/4 ROADMAP success criteria verified; 16/16 plan must-have truths verified.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/runtime/sandbox/sandbox.go` | Embeds eval sandbox + durable paths | ✓ VERIFIED | `*evalsandbox.Sandbox` embedded (line 33); `ResultPath`/`MergedTracePath` added |
| `bench/runtime/subprocess/daemon.go` | StartDaemon/Kill lifecycle | ✓ VERIFIED | Delegates to `sb.StartDaemon`; per-cell socket, HTTP disabled |
| `bench/runtime/subprocess/claude.go` | Wired-not-gating claude branch (D-01) | ✓ VERIFIED | Delegates to `internal/eval/agent.Agent`; `ErrClaudeNotFound` surfaced; `--agent=claude` reachable, not CI-gated |
| `bench/runners/mode_resolver.go` | mode→profile via MODE.md frontmatter | ✓ VERIFIED | Parses YAML frontmatter (yaml.v3); not a hard-coded map (D-05) |
| `bench/runners/your_agent_full/MODE.md` | `profile: bench-full` | ✓ VERIFIED | Frontmatter declares `mode: your_agent_full` / `profile: bench-full` |
| `bench/runtime/result.go` | result.v2 builder + schema-validate | ✓ VERIFIED | jsonschema/v6 validate-on-write; `TestResultV2*` green |
| `bench/runtime/cctap.go` | CCTapResult synth from StepResults | ✓ VERIFIED | Synthesizes 2-leg CC events; zero CC tokens (not fabricated) |
| `bench/runtime/cell.go` | E2E single-cell orchestrator | ✓ VERIFIED | spawn→drive→kill→tap→merge→write; preserve-on-failure |
| `bench/runtime/drive.go` | Forwarder drive over per-cell socket | ✓ VERIFIED | `--mode=stdio --socket=`; drives scripted frames |
| `bench/runtime/matrix.go` | (bench×mode×task) expander, --parallel bounded | ✓ VERIFIED | Path-traversal IDs rejected; semaphore-bounded |
| `cmd/helix-bench/main.go` | run subcommand (replaces notYetImplemented) | ✓ VERIFIED | `RunE` wired; single os.Exit site; exit 0 iff ≥1 succeeds |
| `bench/datasets/toolbench-go/sum-doubler/` | Seed task, fails before edit | ✓ VERIFIED | `sum.go` returns `x` (deliberately wrong); scripted edit → `return x * 2`; verify.sh runs `go test` |
| `Makefile` | bench/bench-quick/bench-micro reconciled | ✓ VERIFIED | bench-micro = original microbench; bench/bench-quick → helix-bench run |
| `bench/BENCH.md` | result.v2 provenance key contract | ✓ VERIFIED | Documents schema_version, outcome, trace_ref, model_id |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| sandbox.go | internal/eval/sandbox.Sandbox | struct embed `*evalsandbox.Sandbox` | ✓ WIRED |
| mode_resolver.go | your_agent_full/MODE.md | yaml frontmatter parse | ✓ WIRED |
| subprocess/daemon.go | Sandbox.StartDaemon | subprocess spawn | ✓ WIRED |
| result.go | bench/schema/result.v2.schema.json | jsonschema/v6 validate-on-write | ✓ WIRED |
| cctap.go | internal/eval/trace.CCTapResult | event synthesis | ✓ WIRED |
| cell.go | internal/eval/trace.TapDaemonLog | PID-gated tap after Kill | ✓ WIRED |
| cell.go | internal/eval/trace.Merge | 2-leg merge | ✓ WIRED |
| drive.go | helix forwarder | exec `--mode=stdio --socket=` | ✓ WIRED |
| main.go | bench/runtime.RunCell | run subcommand drives matrix | ✓ WIRED |
| Makefile bench-quick | cmd/helix-bench run | go build helix then helix-bench run | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| result.v2.json | outcome/tool_call_summary | real daemon tap of `daemon.log` (activate_project) + forwarder-driven `replace_in_file` | Yes — `total=2`, real tool names, real PID | ✓ FLOWING |
| trace.json (merged) | events[] | daemon leg (tapped) + cc leg (synth from StepResults) | Yes — both legs present, real edit result text "1 replacement(s) made in sum.go" | ✓ FLOWING |
| seed sum.go | Double() body | edited in scratch clone, not source | Yes — source unmodified post-run (git clean), verify.sh `go test` exits 0 | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Binaries build | `go build ./cmd/helix && ./cmd/helix-bench` | both OK | ✓ PASS |
| Integration taps run for real | `HELIX_BIN=… go test -run 'TestDaemonTap\|TestCrossCell'` | both PASS (not SKIP) | ✓ PASS |
| E2E smoke timing + schema | timed `helix-bench run …` | 0.41s, schema-valid result.v2 | ✓ PASS |
| make bench-quick gate | `make bench-quick` | exit 0, 4s, 1/1 | ✓ PASS |
| bench-micro collision | `make -n bench-micro` | original `go test -bench=.` | ✓ PASS |
| Full bench suite | `go test ./bench/... ./cmd/helix-bench/...` | all ok | ✓ PASS |
| go vet | `go vet ./bench/... ./cmd/helix-bench/...` | clean | ✓ PASS |
| gofmt | `gofmt -l bench/ cmd/helix-bench/` | clean (empty) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` declared or implied for this phase; verification used the phase's own Go integration tests, the `helix-bench run` E2E smoke, and the `make bench-quick` CI gate — all executed in-process by the verifier with binaries built (not via SUMMARY claims). N/A.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| BENCH-04 | 77-01,02,03,04 | Bench runtime reuses eval sandbox+subprocess; one daemon per (task×mode); ≤30s smallest task; no port collisions | ✓ SATISFIED | sandbox embed, per-cell Unix socket (no TCP), 0.41s smoke, TestCrossCell parallel PASS |
| BENCH-05 | 77-04,05 | make bench/bench-quick/bench-<suite> invoke `cmd/helix-bench run`; bench-quick exit 0 ≥1 task ≤90s | ✓ SATISFIED | make bench-quick exit 0 in 4s; bench/bench SUITE= → helix-bench run --benchmarks |

No orphaned requirements: REQUIREMENTS.md maps BENCH-04 → Phase 77 plans 01-03 and BENCH-05 → Phase 77 plan 05; both claimed by plan frontmatter and both verified.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | No unreferenced TBD/FIXME/XXX in bench/ or cmd/helix-bench/ | — | Clean |

### Documented Deviations (verified honored, NOT failed)

| Deviation | Status | Evidence |
| --------- | ------ | -------- |
| D-01 scripted agent gates CI; real claude wired not gating | ✓ HONORED | subprocess/claude.go reachable via `--agent=claude`, not in CI gate |
| D-04 rich metrics (edit_locality, regression_rate, pass@k) absent | ✓ HONORED | grep of emitted result.v2.json: RICH_METRICS_ABSENT_OK |
| D-03 one seed task sum-doubler only | ✓ HONORED | single dataset dir; corpus deferred to Phase 78 |
| Per-cell semantic_index disabled via --config (Wave-3 fix) | ✓ HONORED | cell.go:414-416 writes `semantic_index:\n enabled: false`; documented rationale at cell.go:394-402 |
| bench-<suite> as `make bench SUITE=<suite>` var-form | ✓ HONORED | `make -n bench SUITE=toolbench-go` → helix-bench run --benchmarks=toolbench-go |

### Code-Review Warnings Assessment (6 advisory)

All 6 review warnings (WR-01..WR-06) were assessed against the phase GOAL. The
reviewer explicitly classified all as non-BLOCKER because the hermetic
single-cell CI path does not trigger their conditions. Confirmed here:

- WR-01 (verify timeout→failure conflation), WR-02 (reader goroutine leak),
  WR-03 (out-of-order response drop), WR-05 (process-group reaping without
  Setpgid), WR-06 (start-clock after spawn) — all fire only under timeout /
  out-of-order / parallel-scale / multi-child conditions that the proven
  single-cell scripted E2E path avoids. They are robustness refinements for the
  scale Phases 78+ introduce, NOT a failure of "force the runtime shape into
  existence and prove it." The runtime shape exists and is proven (4/4 criteria).
- WR-04 (dotfile in dataset dir fails whole run) — a robustness hardening; the
  current single-task corpus has no stray dotfiles, smoke runs clean.

None represents the phase goal not being met. Recommend tracking WR-01/WR-05 for
Phase 78 (they bite under the corpus + parallel scale that phase introduces).

### Human Verification Required

None. All four ROADMAP criteria were verified programmatically by building the
binaries and executing the real spawn→drive→tap→merge path, the timed E2E smoke,
and the `make bench-quick` CI gate. The single manual-only item in 77-VALIDATION.md
(real `claude` CLI agent path, D-01) is explicitly out of CI scope and not part
of the phase goal — it is wired-not-gating by design and does not block.

### Gaps Summary

No gaps. The bench orchestrator stands up end-to-end on the single Go ToolBench
seed task in `your_agent_full` mode with no container and no per-language sprawl.
The full runtime shape is forced into existence and proven by executed evidence:
subprocess daemon spawn (per-cell Unix socket, HTTP off) → forwarder-driven
scripted agent → schema-valid `result.v2.json` (v2, outcome, fairness, trace_ref,
model_id; rich metrics correctly absent) → merged 2-leg OTel trace
(`tool_call_summary.total=2`, daemon + cc legs, single PID, zero cross-talk).
All gates pass: E2E smoke 0.41s (≤30s), make bench-quick exit 0 in 4s (≤90s),
TestDaemonTap + TestCrossCell PASS for real, go vet + gofmt clean, full bench
suite green, requirements BENCH-04/BENCH-05 satisfied.

---

_Verified: 2026-06-17T09:40:00Z_
_Verifier: Claude (gsd-verifier)_
