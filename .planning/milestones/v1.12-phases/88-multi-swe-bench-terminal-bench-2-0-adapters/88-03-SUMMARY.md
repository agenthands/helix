---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
plan: "03"
subsystem: bench/longwall
tags: [bench, checkpoint, resume, state-machine, sc3, hermetic, tdd]
requires: []
provides:
  - "bench/longwall checkpoint store (atomic write/read, injected clock)"
  - "bench/longwall resume-aware Scheduler.Run (skip-done, idempotent re-entry)"
affects:
  - "Phase 88 SC#3 (>24h long-horizon resume after harness restart)"
tech-stack:
  added: []
  patterns:
    - "temp+rename atomic write (writeCacheAtomic primitive copied locally — no bench/runtime import)"
    - "injected now func() time.Time (mirrors internal/semantic/compact.NewCompactionGate)"
    - "validated path-safe cellKey composition (validatePathSegment discipline)"
key-files:
  created:
    - bench/longwall/checkpoint.go
    - bench/longwall/checkpoint_test.go
    - bench/longwall/scheduler.go
    - bench/longwall/scheduler_test.go
  modified:
    - .planning/STATE.md
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
decisions:
  - "longwall is a pure stdlib leaf: copied the atomic temp+rename primitive locally rather than importing bench/runtime (writeDurable is unexported there)"
  - "only a clean Outcome.Success persists StatusDone; failed/partial(running) checkpoints always re-run — a failure is never a resume skip"
  - "an unreadable/torn checkpoint re-runs (conservative), never silently skipped as done"
  - "scheduler is sequential+pure: long-wall scope is resume-correctness, not parallelism (matrix dispatch owns bounded parallelism)"
metrics:
  duration: ~9m
  tasks: 2
  completed: 2026-06-21
---

# Phase 88 Plan 03: Long-Wall Checkpoint/Resume State Machine Summary

Net-new pure-Go `bench/longwall/` package implementing per-cell crash-safe checkpointing (atomic temp+rename) and a resume-aware scheduler, proving the Phase 88 SC#3 >24h long-horizon resume guarantee hermetically via an injected clock with NO real 24h run and NO Docker.

## What Was Built

**`bench/longwall/checkpoint.go`** — `CellState{cell_key,status,result_ref,updated_at}`, a `Store` carrying `dir` + injected `now func() time.Time` (nil→time.Now), `writeCheckpoint` (local temp+rename primitive copied from `swebench-utboost/fetch.go:232` `writeCacheAtomic` — crash-safe, no `bench/runtime` import), `readCheckpoint` (ENOENT→not-found so the cell runs; parse error surfaces — never a fabricated `done`), and a validated path-safe `cellKey(benchmark,lang,mode,task,runIndex)`.

**`bench/longwall/scheduler.go`** — `CellID(.Key)`/`Outcome`/`Summary` types and `Scheduler.Run(ctx, cells, run)`: a per-cell resume gate skips any cell already checkpointed `done`, otherwise runs the cell and atomically checkpoints its outcome. `statusFor` maps only a clean success → `done`; a failure → `failed` (re-runs next pass). Sequential and pure — no Docker, no `bench/runtime`/`bench/container`.

## Three Hermetic Invariants (the sole SC#3 proof)

1. **Round-trip** (`TestCheckpointRoundTrip`): a `done` write reads back `done` with the injected-clock `UpdatedAt` (golden-stable, not wall time).
2. **Resume-skips-done** (`TestResumeSkipsDone`): a pre-seeded `done` cell's counting runner is invoked **exactly 0 times**; siblings run once.
3. **Idempotent re-entry** (`TestIdempotentReentry` + `TestFullRestart`): a partial `running` checkpoint re-runs then converges to `done`; a FRESH scheduler over the same ckptDir (clock advanced +48h) runs **0 cells** — proving a >24h gap is irrelevant. `TestFailedCellReruns` confirms a `failed` cell is never marked done.

`TestCheckpointAtomic` asserts no leftover `.tmp-*` after a successful write and clean unmarshal (no torn state).

## Deviations from Plan

None — plan executed exactly as written. (Added one extra test, `TestFailedCellReruns`, covering the statusFor failure path described in the plan's `<action>`; strengthens rather than deviates.)

## Verification

- `go test ./bench/longwall/` — 8/8 pass (hermetic: no Docker, no tb, no 24h, injected clock)
- `go vet ./bench/longwall/` — clean
- `go build ./...` — pass
- `make vet` (all custom vettools) — pass
- Leaf invariant: `go list -deps ./bench/longwall/ | grep bench/(runtime|container)` — empty (no forbidden import)
- TDD gate sequence in git log: `test(88-03)` → `feat(88-03)` for each task

## Threat Mitigations Applied

| Threat ID | Mitigation |
|-----------|------------|
| T-88-03-01 (torn checkpoint) | atomic temp+rename; TestCheckpointAtomic asserts no leftover `.tmp-*` |
| T-88-03-02 (cellKey path escape) | `validateSegment` rejects empty / separator / traversal segments before the join |
| T-88-03-03 (spoofed done) | parse error surfaces; only clean success writes `done`; failed/partial re-runs |

## Commits

- `010de0c8` test(88-03): failing tests for atomic checkpoint store (RED)
- `4d70ec79` feat(88-03): atomic per-cell checkpoint store with injected clock (GREEN)
- `d2a55f34` test(88-03): failing tests for resume-aware scheduler (RED)
- `d89c9108` feat(88-03): resume-aware long-wall scheduler (GREEN)

## Self-Check: PASSED

- FOUND: bench/longwall/checkpoint.go
- FOUND: bench/longwall/scheduler.go
- FOUND: bench/longwall/checkpoint_test.go
- FOUND: bench/longwall/scheduler_test.go
- FOUND commits: 010de0c8, 4d70ec79, d2a55f34, d89c9108
