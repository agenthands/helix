---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 07
subsystem: bench-cli
tags: [cli, cobra, aggregator, leaderboard, cost-quality, fail-closed]
requires:
  - bench/aggregator.Aggregate (Plan 82-06 pure orchestrator)
  - bench/runtime.BuildResult / Validate (fixture synthesis)
provides:
  - "helix-bench aggregate <run_dir> subcommand (D-02)"
  - "operator-invocable STATS-01 / COST-03 surface"
affects:
  - cmd/helix-bench/main.go (sixth subcommand registration)
tech-stack:
  added: []
  patterns:
    - "thin cobra RunE wrapping a pure orchestrator; main() is the only os.Exit site"
    - "injected today = time.Now().UTC() for the freshness gate (never time.Now() inside pure code)"
    - "fail-closed: RunE returns the Aggregate error verbatim -> non-zero exit, no reports"
key-files:
  created:
    - cmd/helix-bench/aggregate.go
    - cmd/helix-bench/aggregate_test.go
  modified:
    - cmd/helix-bench/main.go
    - cmd/helix-bench/main_test.go
decisions:
  - "Default --seed is a fixed constant (42) so a bare `aggregate` is byte-deterministic (D-08); operators override for alternate replicates."
  - "`aggregate` is the always-intended sixth helix-bench subcommand; the Phase 75 BENCH-02 'exactly five' --help assertion was updated to six (not bypassed via the verify-tos direct-dispatch hack)."
  - "The in-process subcommand tests are deliberately HELIX_BIN-free (pure synthetic fixtures); the real-daemon E2E is the manual/auto-approved human-verify path via the built binary."
metrics:
  duration: ~7m
  completed: 2026-06-21
  tasks: 2
  files: 4
---

# Phase 82 Plan 07: helix-bench aggregate Subcommand Summary

The `helix-bench aggregate <run_dir>` cobra subcommand (D-02) — a thin RunE wrapper that builds an `aggregator.Config` from `--runs`/`--seed`/`--iterations`/`--ci-level`, injects `today = time.Now().UTC()` for the cost freshness gate, calls the pure `aggregator.Aggregate`, and propagates the error so a deficient run exits non-zero with no reports (fail-closed, D-05). This closes the operator-facing surface for STATS-01/COST-03 and renders the first `leaderboard.md` + `cost_quality.md`.

## What Was Built

- **`cmd/helix-bench/aggregate.go`** — `newAggregateCmd()`: `Use: "aggregate <run_dir>"`, `Args: cobra.ExactArgs(1)`, flags `--runs` (IntVar, default 3 — the EXPECTED N for the fail-closed gate, never the disk count, D-05/Pitfall 3), `--seed` (Uint64Var, default `42`, D-08), `--iterations` (IntVar, default 10000), `--ci-level` (Float64Var, default 0.95). `RunE` builds `aggregator.Config{ExpectedN, Seed, Iterations, CILevel, CostTablePath: "bench/datasets/cost-table.yaml", Today: time.Now().UTC()}`, calls `aggregator.Aggregate(args[0], cfg)`, and `return err` on failure (no reports written; Aggregate already writes nothing on a deficiency). On success it prints both report paths to `cmd.OutOrStdout()`.
- **`cmd/helix-bench/main.go`** — `root.AddCommand(newAggregateCmd())` in `newRootCmd` (the sixth subcommand); package docstring + root `Long` help updated to list `aggregate`.
- **`cmd/helix-bench/main_test.go`** — the BENCH-02 "exactly five subcommands" assertion updated to six, with `aggregate` added to the help-mention list (renamed `TestHelixBenchHelpListsFiveSubcommands` -> `TestHelixBenchHelpListsSubcommands`).
- **`cmd/helix-bench/aggregate_test.go`** — in-process tests driving the real `newRootCmd()` over PURELY synthetic `result.v2.json` fixtures (built via `runtime.BuildResult` + `Validate`), NO HELIX_BIN: `TestAggregateCmdFailClosed` (a cell with 2 of 3 rows -> `Execute()` non-nil error AND no `leaderboard.md`/`cost_quality.md`) and `TestAggregateCmdSufficient` (N=3 -> nil error AND both reports exist and are non-empty).

## Tasks & Commits

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Implement aggregate subcommand + register + in-process test | `4ed87ea7` | aggregate.go, aggregate_test.go, main.go, main_test.go |
| 2 | Human-verify (auto-approved) — real multi-run E2E | (no code change; verification only) | — |

## Verification Evidence

**Build + unit (Task 1 gate):**
```
$ go build ./cmd/helix-bench                                    -> BUILD OK
$ go vet ./cmd/helix-bench/...                                  -> VET OK
$ HELIX_BIN=$(...)/helix go test ./cmd/helix-bench/ -run 'Aggregate' -count=1 -v
    === RUN   TestAggregateCmdFailClosed
    --- PASS: TestAggregateCmdFailClosed (0.01s)
    === RUN   TestAggregateCmdSufficient
    --- PASS: TestAggregateCmdSufficient (0.03s)
    PASS
```
Both new tests **RAN (not SKIP)** and PASSED — no false-green.

**Standard gate (bare env, CI polarity):**
```
$ go vet ./...     -> exit 0
$ make vet         -> exit 0 (go vet + 5 custom vettools all clean)
$ go test ./...    -> exit 0 (ok across the tree)
```

**Auto-approved human-verify equivalent (real multi-run matrix, HELIX_BIN):**
```
$ go build -o helix ./cmd/helix && go build -o helix-bench ./cmd/helix-bench   -> BUILD OK
$ HELIX_BIN=$(pwd)/helix ./helix-bench run --modes your_agent_full --modes baseline_plain \
      --runs 3 --tasks IT-go-semantic-view-1 --helix-bin $(pwd)/helix --out /tmp/agg-e2e
    helix-bench run complete: 6/6 cells succeeded
    Reports written to: /tmp/agg-e2e/20260620T222737Z
$ ./helix-bench aggregate /tmp/agg-e2e/20260620T222737Z --runs 3       -> EXIT 0, wrote both reports
$ ./helix-bench aggregate /tmp/agg-e2e/20260620T222737Z --runs 3       -> EXIT 0 (re-run)
$ diff (run1 vs run2) leaderboard.md  -> LEADERBOARD IDENTICAL   (D-08 byte-stable)
$ diff (run1 vs run2) cost_quality.md -> COST IDENTICAL          (D-08 byte-stable)
$ ./helix-bench aggregate <run_dir> --runs 5  -> Error: aggregate: insufficient runs: [...] ; EXIT 1, no reports (fail-closed)
```

**Rendered `leaderboard.md` (real run, excerpt):**
```
# Leaderboard

| mode | benchmark | task_success | pass@1 | pass@N | tokens_input | ... | tool_calls | files_read | edit_locality |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| baseline_plain  | internal-toolbench | 1.0000 [1.0000, 1.0000] | 1.0000 [1.0000, 1.0000] | 1.0000 [1.0000, 1.0000] | — | — | 6.0000 [6.0000, 6.0000] | 0.0000 [0.0000, 0.0000] | — |
| your_agent_full | internal-toolbench | 1.0000 [1.0000, 1.0000] | 1.0000 [1.0000, 1.0000] | 1.0000 [1.0000, 1.0000] | — | — | 6.0000 [6.0000, 6.0000] | 0.0000 [0.0000, 0.0000] | — |

## CI overlap warnings (STATS-04)
- baseline_plain vs your_agent_full: task_success CI overlap 1.0000 [1.0000, 1.0000] vs 1.0000 [1.0000, 1.0000] — no X>Y claim

---
seed: 42
bootstrap_iterations: 10000
ci_level: 0.95
runs: 3
cost_table_valid_until: 2027-01-28
```
Rows are (mode × benchmark) with `value [lo,hi]` CIs; the STATS-04 overlap warning fires; the footer records seed/iterations/ci_level/runs and the cost-table `valid_until`. Token + cost columns correctly render em-dashes (the scripted corpus has null token metrics — expected, not a bug).

## Checkpoint Handling

Task 2 is a `checkpoint:human-verify` (`gate="blocking"`). The phase is running under `--auto` with auto-mode ACTIVE, and this is NOT a package-legitimacy / `blocking-human` gate, so per GSD auto-mode checkpoint handling it is **AUTO-APPROVED**. Rather than pausing, the executor performed the AUTOMATED equivalent (build both binaries, run a real N=3 matrix with HELIX_BIN, aggregate it, confirm both reports render + are byte-identical across re-runs, confirm fail-closed on `--runs 5`) — evidence captured above. No code changes were needed at Task 2; the E2E surfaced no defect in the subcommand or the aggregator.

## Deviations from Plan

None to the planned implementation — the subcommand, registration, flag wiring, and in-process test landed exactly as specified, and the prior subcommand-count assertion was updated for the legitimate sixth subcommand (as the plan instructed).

## Deferred Issues (out of scope — logged, NOT fixed)

A pre-existing test failure was discovered while running `go test ./...` with a resolvable `helix` binary:

- **`TestRunSubcommandWiresDeltaPass`** (`cmd/helix-bench/run_cmd_test.go:144`) fails: real-mode rows are "missing ablation_deltas (delta pass not wired into runBench)". The run reports `12/12 cells succeeded`; only the post-run `ablation_deltas` write-back assertion fails.
- This is the **Phase 80-05 "RED"** delta-pass wiring test (commit `05db8792`) whose GREEN never landed — it concerns the `bench/runtime` delta write-back, NOT the Phase 82 aggregator.
- **Proven pre-existing:** it reproduces identically on `4ed87ea7~1` (the commit BEFORE this plan's first commit) in a clean worktree. Plan 82-07 touches only the four aggregate files; it never touches `runBench`, the daemon, or the delta pass.
- It SKIPs without a `helix` binary (so the bare `go test ./...` CI gate is green), and is HELIX_BIN-gated of the opposite polarity (red only once `helix` is on PATH).
- Logged to `82-.../deferred-items.md`; **DEFER to a Phase-80 delta-pass GREEN follow-up.** Not fixed here per the SCOPE BOUNDARY rule.

## Known Stubs

None. The subcommand wires the real `aggregator.Aggregate` orchestrator end-to-end; no placeholder data paths.

## Threat Model Coverage

- **T-82-07-01** (deficient run silently producing a leaderboard) — MITIGATED: `RunE` returns the `Aggregate` error -> non-zero exit, no reports (verified: `--runs 5` over a 3-run tree exits 1 with no reports; `TestAggregateCmdFailClosed`).
- **T-82-07-02** (non-deterministic CLI output) — MITIGATED: `--seed` (default fixed 42) threaded into `Config`; `today` injected as `time.Now().UTC()`; the second real aggregate run is byte-identical to the first (`diff` empty).
- **T-82-07-SC** (npm/pip/cargo installs) — N/A: no installs; stdlib + existing cobra only.

No new threat surface beyond the plan's `<threat_model>` was introduced.

## Self-Check: PASSED

- `cmd/helix-bench/aggregate.go` — FOUND
- `cmd/helix-bench/aggregate_test.go` — FOUND
- `cmd/helix-bench/main.go` — FOUND (modified)
- `cmd/helix-bench/main_test.go` — FOUND (modified)
- Commit `4ed87ea7` — FOUND in git log
