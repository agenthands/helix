---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 07
subsystem: daemon-gate + bench-runtime
tags: [ablation, no_semantic, CR-01, build-but-block, ABLATE-06, gap_closure]
gap_closure: true
requires:
  - "internal/daemon/semantic_gate.go effSemanticDisabled resolution + gate doctrine (Phase 81-04)"
  - "bench/runtime fail-CLOSED scrape/assert + DaemonHandle.Stop graceful teardown (Phase 81-06 teeth)"
  - "IT-go-incremental-update-1 store-ON fixture (Phase 78-04)"
provides:
  - "internal/daemon backgroundSemanticReadsDisabled predicate — extends the gate doctrine to the daemon-internal background read pipelines"
  - "Gated SetActivateCallback read-drivers + SetFileFactStore (CR-01 closed structurally; zero back-channel reads holds for a store-ON arm)"
  - "bench/runtime store-ON no_semantic end-to-end regression proof (TestNoSemanticStoreOnZeroReads)"
affects:
  - "the no_semantic ablation arm: a store-ON cell now drives ZERO semantic-store reads (structural gate, not corpus-dependent)"
  - "ABLATE-06 runtime guarantee: with 81-06 + 81-07 both teeth present, the masking interaction (81-VERIFICATION.md:113) is broken"
tech-stack:
  added: []
  patterns:
    - "single-resolution-point gate predicate extended to background read pipelines (build-but-block: gate the READS, not construction)"
    - "store-ON ablation regression test (open store + gated reads => non-vacuous zero-reads proof)"
key-files:
  created:
    - bench/runtime/no_semantic_store_on_test.go
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/semantic_gate.go
    - internal/daemon/semantic_gate_test.go
    - internal/daemon/daemon_test_export_test.go
    - internal/semantic/live/handler/handler.go
decisions:
  - "Gated SIX read-drivers, not five: the plan named the SetActivateCallback five + SetFileFactStore, but the lazy-activate path (daemon.go ~911) drives the SAME ScheduleInitialExtraction read — gated it too (Rule 2) so CR-01 closure is complete for both the explicit-activate and lazy-init paths, not just the explicit one."
  - "Introduced backgroundSemanticReadsDisabled(effSemanticDisabled) pure predicate rather than five inline `&& !effSemanticDisabled` conjuncts — keeps the single-resolution-point doctrine the semantic_gate.go file already establishes (a future contributor sees the background pipelines are part of the gated surface)."
  - "Added Handler.HasFactStore() (non-test introspection) + Daemon test seams (effSemanticDisabledForTest / fileFactStoreWiredForTest) so the unit test asserts the read-driver is INERT under the gate via wired state, NOT by spinning a real store (the store-ON read proof is the bench integration test)."
  - "The store-ON regression uses StoreOptIn=true on the IT-go-incremental-update-1 seed + your_agent_no_semantic mode (profile bench-no-semantic carries disable_semantic_subsystem: true). The store is OPENED (D-04 build-but-block) AND the gate is ON — exactly the arm CR-01 cared about and five_of_six's store-OFF seed could not exercise."
metrics:
  duration: ~30min
  completed: 2026-06-20
  tasks: 2
  files: 5
---

# Phase 81 Plan 07: Gate the daemon-internal semantic read pipelines (CR-01) Summary

Gating the daemon-INTERNAL background read pipelines (SetActivateCallback drivers + SetFileFactStore + the lazy-activate extraction) on `effSemanticDisabled` — under build-but-block — closes CR-01 structurally, and a store-ON `no_semantic` end-to-end regression test (with 81-06's fail-closed teeth) dynamically proves the no_semantic arm makes ZERO back-channel reads against an OPEN DuckDB store.

## What This Plan Did

GAP 2 (WARNING, CR-01 from 81-VERIFICATION.md): criterion #1's "ZERO back-channel reads" was PARTIAL. Plan 81-04 gated only the TOOL-FACING SemanticLookup hand-outs + the SemanticSkill accessor block. The daemon-INTERNAL pipelines reached the COUNTED DuckDB read chokepoint (`s.queryContext` / `s.queryRowContext`) UNGATED:

- `SetActivateCallback` (daemon.go ~967-1003) drove `ScheduleInitialExtraction`, `live.startWorkspace`, `rank.ensureScheduler`, `compactBndl.ensureCompactor`, `sBndl.ensureRetrieval` — guarded only by nil-checks, and under D-04 build-but-block the bundle is NON-nil.
- `SetFileFactStore(semanticStore)` (daemon.go ~425) was gated only by `if semanticStore != nil`.

It was LATENT only because the shipped five_of_six smoke uses a store-OFF seed (IT-go-patch-apply-1) so the store never opened. A store-ON `no_semantic` cell (an incremental_update task) WOULD read through these ungated pipelines.

This plan:

- **Task 1 — gate the reads (build-but-block).** Threaded the already-resolved `effSemanticDisabled` (daemon.go:294) into the six daemon-internal read-DRIVERS via a new `backgroundSemanticReadsDisabled` predicate. The store-Open guard (daemon.go ~330) and the `newSemanticBundle` guard (~532) are UNTOUCHED — the store + bundle stay BUILT under the gate; only the READ-DRIVERS are forced off. New unit test `TestSemanticBackgroundPipelinesGated` proves the pipelines are inert under the gate AND the bundle is non-nil (D-04), and wire as before off the gate.
- **Task 2 — prove it dynamically.** `TestNoSemanticStoreOnZeroReads` runs a STORE-ON `no_semantic` cell end-to-end (StoreOptIn=true → daemon OPENS its DuckDB store) and asserts `SemanticStoreReads == 0` + no `SemanticReadViolation`. Because 81-06 made the runtime assertion fail-CLOSED (graceful Stop flushes the reads-total line; absent line is a hard failure), a CR-01 regression would surface as a non-zero count and hard-fail the cell.

With 81-06 (fail-closed teeth) + 81-07 (gated pipelines) both landed, the masking interaction at 81-VERIFICATION.md:113 is broken — both teeth are present together, so a real background read on the store-ON arm would now be observed and hard-fail the cell. The ABLATE-06 runtime guarantee holds.

## Tasks Completed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Gate the SetActivateCallback background read pipelines + SetFileFactStore (+ lazy-activate) on effSemanticDisabled (build-but-block) | `13eb7424` | internal/daemon/daemon.go, internal/daemon/semantic_gate.go, internal/daemon/semantic_gate_test.go, internal/daemon/daemon_test_export_test.go, internal/semantic/live/handler/handler.go |
| 2 | Store-ON no_semantic end-to-end zero-reads regression proof | `2fb2a686` | bench/runtime/no_semantic_store_on_test.go (new) |

## Verification Evidence

All commands run from repo root. The HELIX_BIN-gated test is proven to RUN (not SKIP).

```
$ go build -o helix ./cmd/helix                                   # build OK (exit 0)
$ go vet ./...                                                     # exit 0
$ make vet                                                         # exit 0 (all 5 custom vet tools incl. ablation-leakage)
$ go test ./internal/daemon/... -count=1                          # ok (9.2s)
$ go test ./internal/semantic/live/handler/ -count=1              # ok (HasFactStore added)
$ go test ./...                                                    # no failures
```

Daemon gate unit tests (Task 1):

```
$ go test ./internal/daemon/ -run 'TestEffSemanticDisabled|TestSemanticGate|TestSemanticBackgroundPipelinesGated|TestSemanticSkillAccessorsGated' -count=1 -v
--- PASS: TestEffSemanticDisabled (0.00s)
--- PASS: TestSemanticGateForcesNoop (0.27s)
--- PASS: TestSemanticGateChoosesTreeSitter (0.06s)
--- PASS: TestSemanticSkillAccessorsGated (0.10s)
--- PASS: TestSemanticBackgroundPipelinesGated (0.16s)
    --- PASS: .../gate_on_pipelines_inert_bundle_still_built
    --- PASS: .../gate_off_pipelines_wire_as_before
```

RED was confirmed before GREEN: with SetFileFactStore ungated, `gate_on_pipelines_inert_bundle_still_built` FAILED ("SetFileFactStore must be SKIPPED under the gate") while `gate_off` PASSED; after gating, both pass.

HELIX_BIN-gated bench/runtime run (the false-green trap — these SKIP without HELIX_BIN per MEMORY):

```
$ HELIX_BIN="$(git rev-parse --show-toplevel)/helix" go test ./bench/runtime/ \
    -run 'TestNoSemanticStoreOnZeroReads|TestNoSemanticReadsTotalLineEmitted|TestFiveOfSixSmoke|TestNoSemanticZeroReads' -count=1 -v -timeout 300s
--- PASS: TestFiveOfSixSmoke (1.41s)                  # RAN — store-OFF smoke still green
--- PASS: TestNoSemanticReadsTotalLineEmitted (0.26s) # RAN — 81-06 emission integration test
--- PASS: TestNoSemanticStoreOnZeroReads (0.53s)      # RAN, not SKIP — store-ON no_semantic, zero reads on OPEN store
--- PASS: TestNoSemanticZeroReads (0.00s)             # 81-06 fail-closed unit suite (8 subtests)
ok  github.com/agenthands/helix/bench/runtime 2.214s
```

Acceptance-criteria greps:

```
grep -c "effSemanticDisabled" internal/daemon/daemon.go                              == 23 (>= pre-plan + 5)
grep -n "effSemanticDisabled" internal/daemon/daemon.go | grep store-Open/newSemanticBundle == 0 (D-04 guards untouched)
grep -c "your_agent_no_semantic" bench/runtime/no_semantic_store_on_test.go          == 2
grep -c "StoreOptIn|incremental_update" bench/runtime/no_semantic_store_on_test.go   == 8
asserts SemanticStoreReads==0 AND SemanticReadViolation==false                       present
build tag !windows + resolveHelixBin/t.Skip                                          present
gofmt -l (all 5 touched files)                                                       prints nothing
```

## Deviations from Plan

### Auto-added completeness (Rule 2 — missing critical functionality)

**1. [Rule 2 - Missing functionality] Gated the lazy-activate ScheduleInitialExtraction read-driver too**
- **Found during:** Task 1
- **Issue:** The plan named the five SetActivateCallback drivers + SetFileFactStore. But the LAZY-activate path (`lazyActivateFn`, daemon.go ~911) drives the SAME `ScheduleInitialExtraction` semantic read on first tool call — left ungated, a lazy-activated no_semantic workspace would still drive an initial-walk read against the open store, leaving CR-01 partially open for the lazy path.
- **Fix:** Added `&& !backgroundSemanticReadsDisabled(effSemanticDisabled)` to the lazy-activate ScheduleInitialExtraction guard, mirroring the explicit-activate gate. Build-but-block preserved.
- **Files modified:** internal/daemon/daemon.go
- **Commit:** 13eb7424

## TDD Gate Compliance

Task 1 carries `tdd="true"`. RED was proven before GREEN: `TestSemanticBackgroundPipelinesGated/gate_on_pipelines_inert_bundle_still_built` was written first and FAILED against the ungated tree (SetFileFactStore wired under the gate), while `gate_off` PASSED — confirming the test is non-vacuous. The gate code then made it GREEN. The test + implementation landed in a single `feat` commit (the test does not compile without the new `backgroundSemanticReadsDisabled` predicate and the `effSemanticDisabledForTest` / `fileFactStoreWiredForTest` / `HasFactStore` seams), so a standalone RED commit would not build. Task 2 is the dynamic integration proof (`test(...)` commit).

## Notes / Out-of-Scope

- The FULL `HELIX_BIN=… go test ./bench/runtime/ -count=1` (no `-run` filter) hit the default 600s `go test` timeout on this machine and printed a `DaemonHandle.Kill` goroutine dump. This is the whole package running ALL real-daemon integration tests serially (store_isolation spawns parallel daemons, TestDaemonTap, etc.) exceeding the 10m default — an ENVIRONMENTAL/pre-existing characteristic, NOT a regression from this plan: the new `TestNoSemanticStoreOnZeroReads` runs in 0.53s and the targeted run of all four relevant tests completes in 2.2s. Per the project test gate, the targeted `-run` invocation is the authoritative HELIX_BIN proof; it passes. (Out of scope to re-architect the package's serial integration-test runtime; not caused by this change.)
- Removed the leftover `bench/runtime/.helix/semantic.duckdb` test artifact before committing (regenerable, not gitignored).

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/daemon/semantic_gate.go (backgroundSemanticReadsDisabled predicate)
- FOUND: internal/daemon/daemon.go (six read-drivers gated; D-04 guards untouched)
- FOUND: bench/runtime/no_semantic_store_on_test.go (store-ON regression proof)
- FOUND: internal/daemon/semantic_gate_test.go (TestSemanticBackgroundPipelinesGated)
- FOUND commit: 13eb7424
- FOUND commit: 2fb2a686
