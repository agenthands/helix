---
phase: 78-internal-toolbench-go-first-languagerunner-interface
plan: 04
subsystem: bench/datasets internal-toolbench store-ON fixture + bench/runtime parallel store-isolation proof
tags: [fixture, store-on, incremental-update, refresh-semantic-graph, parallel-isolation, D-01, D-02, D-03, T-78-04, tdd]
requires:
  - bench/datasets/internal-toolbench/go/IT-go-patch-apply-1 (Plan 02 seed; the 6-file shape)
  - bench/runtime CellConfig.StoreOptIn -> writeCellConfig(semantic_index.enabled) + WithWorkingDir(repoDir) wiring (Plan 02 D-01/D-02/D-03)
  - bench/runtime.deriveStoreOptIn (capability==incremental_update -> store ON) (Plan 02)
  - bench/runtime.RunMatrix --parallel dispatch + bench/runtime.CellResult.{RejectedForeignPid,ScratchDir} (Phase 77 / Plan 02)
  - bench/languages/go.GoRunner.RunTests (go test ./... grading) + CapIncrementalUpdate (Plan 01)
  - internal/skill/semantic refresh_semantic_graph {paths,wait_for_lsp,max_wait_ms} + get_semantic_context (Phase 64/70)
provides:
  - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/ (the 10th capability; the ONE store-ON fixture)
  - bench/runtime/store_isolation_test.go (the D-03 --parallel store-isolation integration proof; security T-78-04)
  - 10/10 capability corpus complete (patch_apply=P02, 8 store-off=P03, incremental_update=this plan)
affects:
  - Phase 79 (evaluators consume the full 10-capability corpus incl. the store-ON cell)
  - the phase C1/C2 success criteria (10th capability) and D-03 load-bearing security assertion
tech-stack:
  added: []
  patterns:
    - "Store-ON solve-then-`go test` fixture: registered-capability token wired to a handler; edit flips the token + refresh_semantic_graph overlay-drain + a semantic query; go test pins the post-refresh behavior"
    - "RED-by-fix-removal TDD for an integration test guarding a pre-existing fix: temporarily disable cell.go WithWorkingDir to demonstrate the shared-CWD DuckDB deadlock (1/2 succeeded), then GREEN against the real fix (2/2)"
    - "Per-cell store isolation asserted via distinct CellResult.ScratchDir roots + Succeeded==2 + RejectedForeignPid==0 under --parallel=2"
key-files:
  created:
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/go.mod
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/registry.go
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/registry_test.go
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/scripted_agent.yaml
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/task.json
    - bench/datasets/internal-toolbench/go/IT-go-incremental-update-1/verify.sh
    - bench/runtime/store_isolation_test.go
  modified: []
decisions:
  - "Two COPIES of the incremental_update cell (the plan's 'two copies' option) at --parallel=2 — only your_agent_full has a MODE.md, and each RunCell builds its own ephemeral sandbox so the per-cell .helix/semantic.duckdb is distinct regardless of shared (task,mode) identity"
  - "Distinct-store-path assertion derives from each outcome's CellResult.ScratchDir (the ephemeral sandbox root) + the cwd-relative store default — NOT from the durable OutDir (the store lives in scratch, cleaned on success); Succeeded==2 is the operative no-deadlock proof"
  - "RED demonstrated by temporarily commenting out cell.go's WithWorkingDir append (proving the shared-CWD deadlock drops the matrix to 1/2 succeeded) — the production D-03 fix already landed in Plan 02, so the test gate is committed as test(78-04)"
metrics:
  duration: ~50min
  completed: 2026-06-17
---

# Phase 78 Plan 04: Store-ON incremental_update Fixture + Parallel Store-Isolation Proof Summary

The 10-capability ToolBench Go corpus is complete: this plan adds the ONE
store-ON cell (`IT-go-incremental-update-1`, the 10th capability) that drives the
real Phase 70 overlay-drain refresh, and the `bench/runtime/store_isolation_test.go`
integration test that proves the D-03 per-cell `WithWorkingDir(repoDir)` fix
prevents the DuckDB shared-lock deadlock under `--parallel=2` (the phase's
load-bearing security assertion, T-78-04). The fixture runs 1/1 green end-to-end
store-ON; the isolation test runs 2/2 green with the fix and was RED-proven to
drop to 1/2 with the fix disabled.

## What Was Built

### Task 1 — store-ON incremental_update fixture (commit `2dd54cf9`)
`IT-go-incremental-update-1/` (6-file seed shape): `task.json`
`capability=incremental_update` (which `deriveStoreOptIn` maps to `StoreOptIn=true`
→ `semantic_index.enabled=true` + `WithWorkingDir(repoDir)`, D-01/D-02/D-03 — no
separate flag). `registry.go` registers behavior under a DELIBERATELY-WRONG token
(`registeredCapability()` returns `"legacy"`, which is not a `registry` key, so
`Update` resolves the `zeroHandler` and returns 0). `scripted_agent.yaml` drives
the real incremental-update workflow:

1. `replace_in_file` — flip the token `"legacy"` → `"incremental"`, wiring
   `Update` to the `incrementalHandler` (n+1).
2. `refresh_semantic_graph{paths:["registry.go"], wait_for_lsp:true,
   max_wait_ms:5000}` — drive the Phase 70 overlay-drain so the live semantic
   graph reflects the edit (the capability's named tool, D-05).
3. `get_semantic_context{task, files, symbols}` — a semantic query anchored on
   the edited symbol whose result reflects the post-refresh state.

`registry_test.go` (`go test`, the GoRunner grade) pins the post-refresh behavior:
`Update(n)==n+1` and `registeredCapability()=="incremental"` — both FAIL pre-edit
and PASS post-edit. Hermetic dependency-free `go.mod`; documented as OUT of
`bench-quick` (RESEARCH Pitfall 3 — the ≤90s budget stays pinned to the store-OFF
seed).

### Task 2 — D-03 --parallel store-isolation integration test (commit `8af26c9e`, TDD)
`bench/runtime/store_isolation_test.go`: runs two COPIES of the
incremental_update store-ON cell through `RunMatrix(ctx, cells, 2, cfg)` at
`--parallel=2` and asserts the D-03 contract (T-78-04):

1. **Succeeded==2** — neither daemon hangs on a shared DuckDB lock (the Pitfall-1
   deadlock would manifest as `socket did not appear within 10s` → 0–1 successes).
2. **Distinct `.helix/semantic.duckdb` paths** — derived from each outcome's
   distinct `CellResult.ScratchDir` (each `RunCell` builds its own ephemeral
   sandbox; `WithWorkingDir(repoDir)` lands the cwd-relative store per cell).
3. **`RejectedForeignPid==0`** per cell — no cross-cell PID leakage in the
   PID-gated daemon tap.

Integration-gated like `daemon_tap_integration_test.go`: SKIPs cleanly without a
resolvable helix binary, runs green with `HELIX_BIN` set.

## Verification Results

- **End-to-end store-ON (the fixture):** `helix-bench run
  --benchmarks=internal-toolbench --languages=go
  --tasks=IT-go-incremental-update-1 --agent=scripted` → `1/1 cells succeeded`
  (the cell graded its outcome through `GoRunner.RunTests` `go test ./... -json`
  after the edit + `refresh_semantic_graph` overlay-drain + `get_semantic_context`).
- **Pre-edit FAILS / post-(manual)-edit PASSES** validated for the fixture: pre
  `go test` exit 1 (`Update(0)=0, want 1`; `registeredCapability()="legacy"`),
  post exit 0; reverted to the deliberately-wrong state.
- **store_isolation_test (GREEN):** `HELIX_BIN=<bin> go test ./bench/runtime/
  -run TestStoreIsolationParallel` → PASS (2/2 succeeded, distinct store roots,
  RejectedForeignPid==0).
- **store_isolation_test (RED proof):** with cell.go's `WithWorkingDir` append
  temporarily disabled, the same test FAILED — matrix dropped to 1/2 succeeded
  and left a shared `bench/runtime/.helix/semantic.duckdb` (the harness-cwd
  deadlock artifact); cell.go restored byte-identical afterward.
- **`go vet ./...`** — clean (exit 0).
- **`go test ./bench/... ./internal/eval/... -count=1`** (HELIX_BIN set) — PASS
  (bench/runtime ran the real `--parallel=2` store-ON path in ~11s).
- **`make bench-quick`** (store-OFF seed) — `1/1 cells succeeded` (Pitfall-3
  budget intact; the store-ON fixture is NOT in the quick gate).
- **Strict-parse:** `scripted_agent.yaml` loads under `runner.LoadScript`
  KnownFields (3 steps: replace_in_file, refresh_semantic_graph,
  get_semantic_context).
- **Source assertions:** `grep -q 'semantic.duckdb'` and `grep -q
  'RejectedForeignPid'` in `store_isolation_test.go` both pass; `task.json`
  carries `"capability": "incremental_update"`; `scripted_agent.yaml` names
  `refresh_semantic_graph` with `wait_for_lsp`.

## Success Criteria Met

- **C1/C2 (10th capability):** `incremental_update` has a deterministic Go
  fixture; the corpus is now 10/10 (patch_apply=P02, 8 store-off=P03,
  incremental_update=this plan).
- **D-01/D-02 (targeted store enablement):** ONLY this fixture enables the store;
  `bench-quick` stays pinned to the store-OFF seed.
- **D-03 (per-cell store isolation under --parallel):** proven — 2 store-ON cells
  run parallel-safe with distinct store files, both succeed, zero foreign-PID
  rejection.

## Deviations from Plan

None — plan executed exactly as written. The plan's Task-2 acceptance note
("the incremental_update fixture, or two copies") was resolved to the **two
copies** path because only `your_agent_full` has a MODE.md in `bench/runners`
(distinct modes would require a second MODE.md the phase does not add); each
`RunCell` still builds its own ephemeral sandbox, so the per-cell store isolation
the test proves is unaffected by the shared `(task, mode)` cell identity.

## TDD Gate Compliance

The production D-03 fix (`cell.go` `WithWorkingDir(repoDir)` for store-ON cells)
landed in Plan 02's GREEN gate (`feat(78-02)` commit `34c928ca`). This plan adds
the integration test that PROVES it. RED was demonstrated by temporarily removing
the `WithWorkingDir` append (matrix drops to 1/2 succeeded — the shared-CWD
deadlock), then GREEN against the restored real fix (2/2). The test gate is
committed as `test(78-04)` (`8af26c9e`); cell.go was restored byte-identical (no
source change in this plan). Task 1's fixture is a corpus/data task graded by its
own `go test`, validated FAIL-pre-edit / PASS-post-edit.

## Known Stubs

The fixture's `registeredCapability()` returning `"legacy"` and the `zeroHandler`
(`Apply -> 0`) are the DELIBERATE "deliberately-incomplete module" premise (D-04),
resolved by the fixture's own `scripted_agent.yaml` during a bench run and
validated FAIL-pre-edit / PASS-post-edit. No accidental stubs.

## Threat Surface

- **T-78-04 (parallel store-lock deadlock)** — mitigated and PROVEN: the D-03
  `WithWorkingDir(repoDir)` per-cell working dir makes `.helix/semantic.duckdb`
  per-cell; `store_isolation_test.go` asserts distinct store files + both cells
  succeed + `RejectedForeignPid==0` under `--parallel=2` (the phase's load-bearing
  security assertion), and the RED demonstration confirmed the deadlock manifests
  without the fix.
- **T-78-09 (path traversal under cmd.Dir)** — accept/verified: the cwd-relative
  `.helix/semantic.duckdb` lands inside the V5-validated per-cell repo dir; the
  store rejects absolute paths. No escape.
- **T-78-10 (store-ON fixture network/host access)** — accept/verified: the
  fixture `go.mod` is dependency-free and the refresh is in-process (Phase 70
  overlay-drain); the end-to-end run is hermetic and offline.

No new external-input paths introduced (a data-only fixture + a test).

## Self-Check: PASSED

All 6 fixture files + `store_isolation_test.go` present on disk; commits
`2dd54cf9` and `8af26c9e` present in git log.
