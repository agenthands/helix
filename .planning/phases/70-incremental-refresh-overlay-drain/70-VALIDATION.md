---
phase: 70
phase-slug: incremental-refresh-overlay-drain
status: approved
nyquist_compliant: true
wave_0_complete: true
framework: go test
created: 2026-05-15
revised: 2026-05-15
---

# Phase 70 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Regenerated 2026-05-15 from the per-plan automated test commands of all 7 plans.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib `testing`; no external test framework) |
| **Config file** | none (table-driven; per-package harness helpers) |
| **Quick run command** | `go test ./internal/semantic/store/ ./internal/skill/semantic/ ./internal/daemon/ ./internal/obs/ ./internal/semantic/live/coalescer/ ./internal/semantic/live/service/ -race -count=1` |
| **Full suite command** | `make test` (≡ `go test ./...`) |
| **Estimated runtime (quick)** | ~25 s |
| **Estimated runtime (full)** | ~70 s |

---

## Sampling Rate

- **After every task commit:** Run the quick command (scoped to packages touched
  by the task is also acceptable; the full quick command above bounds the latency
  ceiling).
- **After every plan wave:** Run `go test ./... -race -count=1` plus
  `make vet-nokernel2semantic` and `go vet ./...`.
- **Before `/gsd-verify-work`:** Full suite green AND one local bench run of
  `TestBench_RefreshIncremental_10kSymbols_P95Under200ms` with p95 logged
  (CI does not run the bench by project rule).
- **Max feedback latency:** 70 s (full suite).

---

## Per-Task Verification Map

Each row pulls the exact `<verify><automated>` command from the plan's task
definition. "Dim Coverage" column flags which Nyquist Dimension 8 sub-rule the
task satisfies (see Validation Sign-Off below).

| Task ID | Plan | Wave | REQ-ID | Test Command | Dim Coverage |
|---------|------|------|--------|--------------|--------------|
| 70-01-01 | 01 | 1 | REFRESH-01 | `go test ./internal/semantic/store/ -race -run TestOverlayChangedPathsSince -count=1 2>&1 \| grep -E "undeclared name\|undefined: .*OverlayChangedPathsSince\|FAIL"` | 8a (RED test exists), 8d (compile-failure proves negative case) |
| 70-01-02 | 01 | 1 | REFRESH-01 | `go test ./internal/semantic/store/ -race -run TestOverlayChangedPathsSince -count=1` | 8a, 8b (CI-runnable), 8d (5 sub-tests cover cold_start / single_change / multi_epoch / empty_since / no_meta_row) |
| 70-02-01 | 02 | 1 | REFRESH-01 | `go test ./internal/semantic/store/ -race -run 'TestMigration006\|TestMigrationsRegistry\|TestCurrentSchemaVersion' -count=1` | 8a, 8b, 8d (schema delta + DEFAULT 0 backfill verified) |
| 70-02-02 | 02 | 1 | REFRESH-01 | `go test ./internal/semantic/store/ -race -run 'CommitSnapshot\|LatestCommittedSnapshotBaseEpoch' -count=1` | 8a, 8b, 8d (round-trip Begin → SetBaseOverlayEpoch → Commit → Latest accessor) |
| 70-03-01 | 03 | 1 | REFRESH-01 | `go test ./internal/semantic/live/coalescer/ ./internal/semantic/live/service/ -race -run 'FlushNow' -count=1 -timeout 30s` | 8a, 8b, 8d (DrainsPending + EmptyIsNoop + ConcurrentEnqueueSafe under -race) |
| 70-03-02 | 03 | 1 | REFRESH-01, REFRESH-03 | `go test ./internal/obs/ -race -run 'IncrementalRefreshFallback\|MetricsLabelsAllowlist' -count=1` | 8a, 8b, 8d (allowlist carve-out + drop-on-unknown reason) |
| 70-04-01 | 04 | 2 | REFRESH-01, REFRESH-03 | `go build ./... && go vet ./internal/skill/semantic/... ./internal/daemon/...` | 8a (interface extension compiles), 8b |
| 70-04-02 | 04 | 2 | REFRESH-01, REFRESH-03 | `go test ./internal/daemon/ -race -run 'TestCollectCandidatePaths' -count=1` | 8a, 8b, 8d (hot path + cold_start + overlay_rotated + empty_overlay + full-walk regression) |
| 70-05-01 | 05 | 3 | REFRESH-01 | `go test ./internal/skill/semantic/ -race -run 'TestRefresh_FilesUpdated_FromSeam\|TestRefresh_FilesUpdated_Seam_FlushNowErrorIsNonFatal\|TestRefresh_FilesUpdated_NoLive_StillReads' -count=1 2>&1 \| grep -E "FAIL\|files_updated"` | 8a (RED), 8b, 8d (FlushNow error tolerated, no_live path) |
| 70-05-02 | 05 | 3 | REFRESH-01 | `go test ./internal/skill/semantic/ -race -run 'TestRefresh' -count=1 && bash -c 'set -e; grep -nE "\b(BeginSnapshot\|CommitSnapshot\|AbortSnapshot\|WriteSnapshot)\b" internal/skill/semantic/tools_refresh.go && exit 1; exit 0'` | 8a, 8b, 8d (D-09 grep gate enforces read-only invariant) |
| 70-06-01 | 06 | 4 | REFRESH-03 | `go test ./internal/eval/runner/ -race -run 'TestRefreshHarness_Smoke' -count=1` | 8a, 8b (harness smoke) |
| 70-06-02 | 06 | 4 | REFRESH-03 | `go test ./internal/eval/runner/ -race -run TestRefreshIncremental -count=1 -timeout 60s` | 8a, 8b, 8d (hot path + all 3 fallback reasons end-to-end with real store + coalescer) |
| 70-07-01 | 07 | 4 | REFRESH-02 | `CI= go test ./internal/eval/runner/ -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms -count=1 -timeout 5m -v 2>&1 \| grep -E "p50=\|p95=\|p99=\|PASS\|FAIL"` | 8a, 8d (p95 budget); **8b EXEMPT** — bench is local-only per project rule `feedback_no_ci_benchmarks` (CI gates exercised via `CI=true …` skip-path assertion in the plan's secondary verify) |

Total: 13 tasks across 7 plans. Every task has an automated `<verify>` command.

---

## Wave 0 Requirements (RESOLVED in this revision)

The plan-checker revision (2026-05-15) consumed Wave 0 reads inline rather than
deferring them to a Wave-0 plan. The three RESEARCH.md Open Questions are now
RESOLVED in `70-RESEARCH.md` `## Open Questions (RESOLVED 2026-05-15)`:

- [x] **A1 — Overlay path form** RESOLVED: **absolute paths**. Source consulted:
      `internal/semantic/live/handler/handler.go:392-417`,
      `internal/semantic/live/watcher/watcher.go:225,289`,
      `internal/semantic/live/service/service.go:184`.
- [x] **A2 — Coalescer FlushNow re-entrancy** RESOLVED: **safe**. FlushNow body
      pattern locked in RESEARCH.md. Source consulted:
      `internal/semantic/live/coalescer/coalescer.go` (full file).
- [x] **A3 — `repo` label form** RESOLVED: **`string(repoID) == ws.Hash()`**,
      identical to Phase 68 `LiveFileFactDiffInc`. Source consulted:
      `internal/obs/metrics.go:80-81, 105-110, 768-778`,
      `internal/semantic/live/handler/difffacts.go:260`.

No additional Wave 0 test scaffolding is required — all `<verify><automated>`
commands listed in the Per-Task Verification Map reference tests that the task
itself creates (RED step) or that already exist (e.g., `TestMetricsLabelsAllowlist`,
`go build`, `go vet`).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Bench p95 ≤ 200 ms on developer workstation | REFRESH-02 | Project rule `feedback_no_ci_benchmarks` forbids CI benchmarks; hardware variance makes shared-runner numbers unreliable | Run `CI= go test ./internal/eval/runner/ -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms -count=1 -timeout 5m -v` locally before `/gsd-verify-work`; record p50/p95/p99 in the phase summary |

All other phase behaviors have automated verification.

---

## Dimension 8 (Validation) Coverage Notes

- **8a — Test exists before task is "done":** Every task creates or extends a
  test. RED-first tasks (`70-01-01`, `70-05-01`) commit a failing test before
  implementation. No `MISSING — Wave 0 must create` placeholders remain after
  the A1/A2/A3 resolutions above.
- **8b — Test runs in CI:** All test commands are pure `go test` invocations
  runnable on `ubuntu-22.04` and `macos-14` runners (the project's split CI
  matrix). Exception: `70-07-01` (bench) is explicitly CI-excluded per project
  rule; its skip-path is itself verified by the plan's acceptance criteria
  (`CI=true go test … -v 2>&1 | grep -E "SKIP"`).
- **8c — Sampling continuity:** No three consecutive tasks (by execution order
  Wave-1 → Wave-4, plan-by-plan) lack an automated verify. Wave 1 tasks
  (01-01, 01-02, 02-01, 02-02, 03-01, 03-02) all have `go test` commands. Wave 2
  (04-01, 04-02) likewise. Wave 3 (05-01, 05-02), Wave 4 (06-01, 06-02, 07-01)
  likewise. Maximum feedback latency between a task commit and its test run is
  bounded by the quick command (~25 s).
- **8d — Failure-mode coverage:** Each requirement has tests covering both
  happy and unhappy paths:
  - **REFRESH-01 (hot path):** `70-01-02` (5 store sub-tests including
    `empty_since` and `no_meta_row`), `70-02-02` (`LatestCommittedSnapshotBaseEpoch`
    returns `(0, false, nil)` on missing), `70-04-02` (`TestCollectCandidatePaths_Full_UsesWalker`
    regression), `70-05-02` (`FlushNowErrorIsNonFatal`, `NoLive_StillReads`).
  - **REFRESH-02 (bench):** `70-07-01` asserts `p95 ≤ 200ms` and `t.Errorf` on
    breach; manual gate before `/gsd-verify-work`.
  - **REFRESH-03 (fallback):** `70-04-02` covers all three reason labels
    against mocks; `70-06-02` covers the same three reasons end-to-end with a
    real store + coalescer; `70-03-02` covers `drop-on-unknown` for malformed
    reason values.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify (13/13)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (the three Open Questions resolved
      inline in this revision; no MISSING test commands remain)
- [x] No watch-mode flags
- [x] Feedback latency < 70 s (quick command ~25 s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-05-15 (revision).
