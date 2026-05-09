---
phase: 63
slug: compaction-retention
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-07
updated: 2026-05-07
---

# Phase 63 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.22+) |
| **Config file** | none — Go's built-in test discovery |
| **Quick run command** | `go test ./internal/semantic/store/... ./internal/semantic/compact/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~60 seconds (quick) / ~5 minutes (full) |

---

## Sampling Rate

- **After every task commit:** Run quick run command (package-scoped)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

> Each task in PLAN.md gets a row mapping task_id → requirement → test type → automated command. Populated 2026-05-07 from the `<verify><automated>` blocks in each task across 63-01-PLAN.md and 63-02-PLAN.md.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 63-01-T1 | 01 | 1 | COMPACT-01, COMPACT-04 | T-63-01-01, T-63-01-02 | parameterized SQL via `?` placeholders; double-commit / commit-after-abort guards; ClearOverlayLE encapsulates *sql.Tx | TDD RED — failing tests | `go test ./internal/semantic/store/... -run 'TestBeginSnapshot\|TestSnapshot_\|TestFakeCompactor_' 2>&1 \| grep -E 'FAIL\|undefined\|undeclared' && echo RED-GATE-OK` | ❌ W0 | ⬜ pending |
| 63-01-T2 | 01 | 1 | COMPACT-01, COMPACT-04 | T-63-01-01, T-63-01-02, T-63-01-04 | snapshot tx isolation (DuckDB ACID); Snapshot.ClearOverlayLE owns *sql.Tx — no Tx() accessor; vet-noduckdb clean | TDD GREEN — implementation passes | `go test ./internal/semantic/store/... -run 'TestBeginSnapshot\|TestSnapshot_\|TestFakeCompactor_' -count=1 -timeout 60s && go vet ./internal/semantic/store/...` | ❌ W0 | ⬜ pending |
| 63-02-T1 | 02 | 2 | COMPACT-01, COMPACT-02, COMPACT-03 | T-63-02-01, T-63-02-03, T-63-02-04 | Phase 60 D-04 CAS contract maintained; in-memory atomic-counter proxy (OverlayHasPendingRows) backs zero-I/O gate; pre-flight size guard caps tx; store.Vacuum encapsulates SQL | unit (additive accessors) + migration | `go test ./internal/semantic/live/coalescer/... ./internal/semantic/store/... ./internal/semantic/lspenrich/... ./internal/semantic/graph/... ./internal/kernel/... -count=1 -timeout 120s && go vet ./...` | ❌ W0 | ⬜ pending |
| 63-02-T2 | 02 | 2 | COMPACT-01, COMPACT-02, COMPACT-04, COMPACT-05 | T-63-02-01, T-63-02-02, T-63-02-04, T-63-02-06 | gate.IsReady is zero-I/O (CONTEXT.md D-04); single-tx atomicity (snap.ClearOverlayLE + snap.DeleteSnapshotsBeyond on the snapshot's own tx); kill-mid-compact subprocess test confirms rollback; CAS interleave property test under -race; bounded-label metric outcomes | unit + property + subprocess + bench | `go test ./internal/semantic/compact/... -run 'TestGate_\|TestCompactor_\|TestCAS_\|TestCompactor_KillMidCompact' -count=1 -race -timeout 180s && go vet ./internal/semantic/compact/...` | ❌ W0 | ⬜ pending |
| 63-02-T3 | 02 | 2 | COMPACT-01, COMPACT-03 | T-63-02-05, T-63-02-06, T-63-02-07, T-63-02-08 | per-workspace ownership (no global compactor); closed-enum bounded-label metrics (drop-on-unknown helpers); vet-compact-uses-store enforces compact→store boundary; koanf duration parse validation | integration (daemon wiring) + config + obs + vet analyzer | `go test ./internal/daemon/... ./internal/config/... ./internal/obs/... ./internal/semantic/compact/... -count=1 -timeout 240s && go vet ./... && go run ./cmd/vet-noduckdb ./... && go run ./cmd/vet-compact-uses-store ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*File exists column: ❌ W0 = test file is created in this task's RED step (Wave 0). Once the test file lands, flip to ✅. Subprocess fixture (`testdata/cmd/compact_one/main.go`) is also a Wave-0 artifact created inside 63-02-T2.*

---

## Wave 0 Requirements

> Wave 0 = test files that must exist BEFORE the corresponding production code in each task's `<verify><automated>` is allowed to run. Each TDD task is self-contained: its RED step writes the test file, its GREEN step makes it pass.

- [ ] `internal/semantic/store/snapshot_test.go` — unit tests for Begin/Write/Commit/Abort + ClearOverlayLE atomic-with-commit + Tx-accessor-absent encapsulation (created in 63-01-T1)
- [ ] `internal/semantic/store/snapshot_fake_compactor_test.go` — fakeCompactor end-to-end fixture (created in 63-01-T1)
- [ ] `internal/semantic/store/overlay_test.go` extensions — `TestStore_OverlayTxOpenCount_IncDec`, `TestStore_OverlayRowCount_BoundedByCapturedEpoch`, `TestStore_OverlayHasPendingRows_AtomicProxy`, `TestStore_Vacuum_RoundTrips` (created in 63-02-T1)
- [ ] `internal/semantic/store/vacuum_test.go` (or extension of overlay_test.go) — Vacuum smoke test (63-02-T1)
- [ ] `internal/semantic/store/migrations_test.go` extension — `TestApplyMigration004_AddsLastVacuumAtColumn` (63-02-T1)
- [ ] `internal/semantic/live/coalescer/coalescer_test.go` extensions — `TestCoalescer_LastFlushAt_StampsOnFlush`, `TestCoalescer_SetOnFlush_Invoked` (63-02-T1)
- [ ] `internal/semantic/lspenrich/queue_test.go` extensions — `TestLaneQueue_LastEnqueueAt_StampsOnSuccess`, `TestLaneQueue_DepthAll` (63-02-T1)
- [ ] `internal/semantic/graph/scheduler_test.go` extensions — `TestRankScheduler_IsQuiescent_FalseDuringRepair`, `TestRankScheduler_IsQuiescent_TrueOnEmptyState` (63-02-T1)
- [ ] `internal/kernel/edit_tx_count_test.go` — `TestKernel_ActiveEditTxCount_IncDec` table-driven across all 8 edit tools (63-02-T1)
- [ ] `internal/semantic/compact/gate_test.go` — 7 BlockedReason-isolation tests + BlockedNone happy path (63-02-T2)
- [ ] `internal/semantic/compact/compactor_test.go` — OnFlush timer reset, runCompaction success/partial/rollback (63-02-T2)
- [ ] `internal/semantic/compact/cas_property_test.go` — `TestCAS_InterleaveOverlayWritesWithCompaction` under `-race` (COMPACT-02) (63-02-T2)
- [ ] `internal/semantic/compact/kill_test.go` — `TestCompactor_KillMidCompact` table-driven over tx-phase boundaries (COMPACT-05) (63-02-T2)
- [ ] `internal/semantic/compact/testdata/cmd/compact_one/main.go` — subprocess fixture binary writing sentinel files at each tx-phase (63-02-T2)
- [ ] `internal/semantic/compact/longrepo_bench_test.go` — 100-file × 1000-cycle bench asserting `growth_factor < 2.0x` (COMPACT-03; local-only per MEMORY.md) (63-02-T2)
- [ ] `internal/daemon/wiring_test.go` extension — `TestDaemon_CompactBundleSpawnsPerWorkspace` + `TestPhase63_EndToEndSmoke` (63-02-T3)
- [ ] `internal/config/config_test.go` extension — `TestLoad_MaintenanceDefaults` (63-02-T3)
- [ ] `internal/obs/metrics_labels_test.go` extension — `TestSemanticCompactionOutcomeCardinality`, `TestSemanticVacuumOutcomeCardinality` (63-02-T3)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none) | — | — | — |

*All phase behaviors targeted for automated verification — kill-mid-compact uses subprocess fixture; long-repo bench is `go test -bench=Long` (local-only per MEMORY.md "Benchmarks are local-only — never on CI").*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (every test file required by a `<verify>` block is enumerated above)
- [x] No watch-mode flags (no `-watch`, no `--watchAll`; bench runs single-shot under `-benchtime=1x`)
- [x] Feedback latency < 60s (quick-run command targets package scope)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready for execution
