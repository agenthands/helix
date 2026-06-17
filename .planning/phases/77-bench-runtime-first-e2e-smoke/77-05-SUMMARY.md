---
phase: 77-bench-runtime-first-e2e-smoke
plan: 05
subsystem: bench-make-orchestration
tags: [bench, makefile, BENCH-05, e2e-smoke, ci-gate, collision-reconciliation, phase-close]
requires:
  - cmd/helix-bench run subcommand (Plan 04 — the surface the targets invoke)
  - bench/runtime.RunCell / RunMatrix (Plan 03/04 — the spine the smoke exercises)
  - bench/datasets/toolbench-go/sum-doubler (Plan 01 — the seed smoke task)
  - bench/schema/result.v2.schema.json (Phase 75 — the open-additionalProperties result contract)
provides:
  - Makefile bench-micro (renamed Go microbench, recipe preserved verbatim)
  - Makefile bench (go run ./cmd/helix-bench run --benchmarks=$(SUITE))
  - Makefile bench-quick (build helix -> hermetic scripted your_agent_full smoke, <=90s CI gate)
  - Makefile SUITE var (the bench-<suite> parameterization, default toolbench-go)
  - bench/BENCH.md result.v2 provenance key-name contract (outcome, trace_ref, model_id, fairness, tokens_*, schema_version) for Phase 79
affects:
  - Phase 79 (aggregator/scorers read the pinned result.v2 provenance keys, never rename them)
  - CI (bench-quick is the hermetic <=90s bench smoke gate; bench/bench-micro stay local/nightly)
tech-stack:
  added: []
  patterns:
    - target-name collision reconciled by RENAME (bench -> bench-micro), recipe preserved verbatim — never silently clobbered
    - SUITE ?= make var as the bench-<suite> parameterization (avoids a `bench-%:` pattern rule shadowing bench-micro/bench-quick/bench-baseline)
    - bench-quick mirrors eval-quick local-only/no-network/no-API-key discipline (scripted agent, D-01)
    - absolute --helix-bin=$(CURDIR)/helix so the daemon resolves from the per-cell ephemeral scratch cwd
    - generated per-run reports gitignored (/bench/reports/*) with the tracked .gitkeep negated, mirroring /eval/reports/
key-files:
  created:
    - .planning/phases/77-bench-runtime-first-e2e-smoke/77-05-SUMMARY.md
  modified:
    - Makefile
    - bench/BENCH.md
    - .gitignore
decisions:
  - "Collision reconciled by RENAMING the Go microbench bench: -> bench-micro: (recipe byte-preserved), NOT by clobbering — make bench-micro still runs go test -bench, make bench now runs helix-bench run (T-77-13 mitigated; verified via make -n recipe identity)"
  - "bench-<suite> is implemented as `make bench SUITE=<suite>` (a make var), not a `bench-%:` pattern rule — a pattern rule would shadow bench-micro/bench-quick/bench-baseline and silently re-route them through helix-bench"
  - "bench-quick passes an ABSOLUTE --helix-bin=$(CURDIR)/helix because the per-cell sandbox runs the forwarder/daemon from an ephemeral scratch cwd where a relative ./helix would not resolve"
  - "result.v2 provenance keys pinned in BENCH.md as snake_case (outcome, trace_ref, model_id, fairness, tokens_input/output, schema_version) — the schema leaves additionalProperties open so the contract lives in BENCH.md for Phase 79 (Open Question 3)"
metrics:
  duration_seconds: 540
  completed: 2026-06-17
  tasks: 2
  files: 3
---

# Phase 77 Plan 05: make bench Reconciliation + Timed E2E Smoke Gate Summary

Closes Phase 77 (BENCH-05). The `make bench` target-name collision (Pitfall 1) is
reconciled by renaming the Phase 64 Go microbench `bench:` -> `bench-micro:` (recipe
preserved verbatim); the reclaimed `bench`, plus a new `bench-quick` and the
`make bench SUITE=<suite>` parameterization, now invoke `cmd/helix-bench run`. Both
timed smoke gates pass within budget producing a schema-valid 2-leg result with zero
PID cross-talk, and `bench/BENCH.md` pins the result.v2 provenance key names so Phase
79 consumes a stable contract.

## What Was Built

### Task 1 — Makefile reconciliation + BENCH.md key-name contract (commit 9aa1fa49)
- **`.PHONY` updated** — dropped the old `bench` (microbench) meaning; added
  `bench-micro`, `bench-quick`; kept `bench` (now the milestone driver) and
  `bench-baseline`.
- **`bench-micro:`** — the original Phase 64 microbench recipe
  (`go test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...`) preserved
  **verbatim** under the unambiguous name, with a migration comment. `bench-baseline`
  unchanged (still tees the same microbench recipe).
- **`bench:`** — `go run ./cmd/helix-bench run --benchmarks=$(SUITE)` (BENCH-05). A
  `SUITE ?= toolbench-go` var is the `bench-<suite>` parameterization (`make bench
  SUITE=<suite>`).
- **`bench-quick:`** — `go build -o helix ./cmd/helix` FIRST (RESEARCH build-sequencing
  note: the subprocess-daemon/forwarder-drive path SKIPs if `helix` is absent), THEN
  the hermetic scripted `your_agent_full` smoke on the single `sum-doubler` seed task
  with `--agent=scripted` and an absolute `--helix-bin=$(CURDIR)/helix`. Local-only,
  no-network, no-API-key (D-01) — comments mirror `eval-quick`.
- **`bench/BENCH.md`** — the deferred collision section rewritten as **RESOLVED**
  (rename table + migration note), plus a new **result.v2 provenance key-name
  contract** section pinning `schema_version`, `outcome`, `fairness`, `tokens_input`/
  `tokens_output`, `trace_ref`, `model_id` as the stable snake_case keys Phase 79
  reads (Open Question 3). Eval targets / EVAL.md untouched.

### Task 2 — Timed E2E smoke gate (operator/auto-run verification, no source changes)
The phase gate. Run under AUTO MODE (auto-advance chain): the deterministic timed
gate was executed and reported objectively; a passing gate = approved.

## Timed Gate Results (objective, AUTO MODE)

**Collision reconciliation (T-77-13, `make -n` recipe identity):**
- `make bench-micro` -> `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` (the OLD microbench)
- `make bench` -> `go run ./cmd/helix-bench run --benchmarks=toolbench-go` (NOT go test -bench)
- Reconciled, not clobbered.

**Criterion #1 — single-task smoke (<=30s budget):**
```
go build -o helix ./cmd/helix      # 175,346,928-byte daemon binary
go run ./cmd/helix-bench run --benchmarks=toolbench-go --modes=your_agent_full \
  --tasks=sum-doubler --helix-bin=$(pwd)/helix --out /tmp/bench-c1-out
→ "helix-bench run complete: 1/1 cells succeeded"
EXIT=0  ELAPSED=2s   (<=30s)
```
`result.v2.json` inspected: `schema_version=v2`, `outcome=success`, `trace_ref` present,
`model_id=claude-sonnet-4-5-20260128`, `fairness` present.

**Criterion #4 — merged 2-leg trace, zero PID cross-talk:**
`trace.json`: 6 events, sources `{cc, daemon}` (both legs present), tool-call summary
`total=2` (`activate_project`×1, `replace_in_file`×1), `by_outcome={success:2}` — 2-leg
merge with no foreign tool names.

**Criterion #2 — `make bench-quick` (<=90s, >=1 task):**
```
make bench-quick
→ "helix-bench run complete: 1/1 cells succeeded"
EXIT=0  ELAPSED=~0s  (binary cached from the build step; cold ~2s per criterion #1)  (<=90s)
```

All gates PASS within budget. Treated as approved (deterministic gate, AUTO MODE).

## How to Verify

- Recipe identity: `make -n bench-micro` (go test -bench) vs `make -n bench` (helix-bench run).
- `make bench-micro` still runs the Go microbenchmark.
- `time go run ./cmd/helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=sum-doubler --helix-bin=$(pwd)/helix --out /tmp/out` -> exit 0 <=30s, schema-valid result.v2.json.
- `time make bench-quick` -> exit 0 <=90s, 1/1 cells succeeded.
- `grep -q trace_ref bench/BENCH.md`; `go vet ./...` clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] gitignore generated bench/reports run dirs**
- **Found during:** Task 2 — `make bench-quick` writes per-run reports under
  `bench/reports/<run_id>/`, which surfaced as untracked files (task_commit_protocol
  forbids leaving generated output untracked). `eval/reports/` is already gitignored;
  `bench/reports/` was not.
- **Fix:** Added `/bench/reports/*` + `!/bench/reports/.gitkeep` to `.gitignore`
  (mirrors `/eval/reports/`, keeps the tracked `.gitkeep`).
- **Files modified:** .gitignore
- **Commit:** ca19c728

No bugs (Rule 1), no missing critical functionality (Rule 2), no architectural changes
(Rule 4). The only Makefile design divergence — using a `SUITE` make var instead of a
`bench-%:` pattern rule for `bench-<suite>` — is a deliberate decision (a pattern rule
would shadow `bench-micro`/`bench-quick`/`bench-baseline`); the plan explicitly permits
"documented `make bench SUITE=...`".

## Scope Boundary Notes

- **Makefile + BENCH.md + .gitignore only** — no `bench/runtime` or `cmd/` source touched
  (the smoke exercises the Plan 01-04 spine unchanged).
- **Eval untouched** — eval targets and `eval/EVAL.md` are not modified.
- **No new packages** (T-77-SC) — zero npm/pip/cargo installs; the Makefile adds no
  install steps.

## Known Stubs

None. The targets invoke the fully-wired `cmd/helix-bench run`; the timed smoke ran a
real cell end-to-end to a schema-valid 2-leg result.

## Self-Check: PASSED

- Makefile (bench-micro / bench / bench-quick / SUITE) — FOUND
- bench/BENCH.md (trace_ref contract + collision RESOLVED) — FOUND
- .gitignore (/bench/reports/* + .gitkeep negation) — FOUND
- .planning/phases/77-bench-runtime-first-e2e-smoke/77-05-SUMMARY.md — FOUND
- Commit 9aa1fa49 (feat, Task 1) — FOUND
- Commit ca19c728 (chore, Rule 3 gitignore) — FOUND
