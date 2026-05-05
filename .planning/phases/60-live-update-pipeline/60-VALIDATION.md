---
phase: 60
slug: live-update-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-05
---

# Phase 60 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source of truth for the per-criterion test map: `60-RESEARCH.md` §Validation Architecture (lines 702–776).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (+ `go test -race` for concurrency tests) |
| **Config file** | None — Go convention |
| **Quick run command** | `go test ./internal/semantic/live/... ./internal/semantic/store/... ./internal/kernel/edit/... ./internal/kernel/fileops/... -count=1` |
| **Full suite command** | `go test ./... -race -count=1` |
| **Estimated runtime** | ~5s quick · 30–90s full (including editor fixtures) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/semantic/live/... ./internal/semantic/store/... -count=1` (~5s subset).
- **After every plan wave:** Run `go test ./... -race -count=1` (full suite, 30–90s including editor fixtures).
- **Before `/gsd-verify-work`:** Full suite must be green AND `go vet ./...` AND `cmd/vet-noduckdb` AND new `cmd/vet-nokernel2semantic` must all be clean.
- **Max feedback latency:** 5 seconds for per-task subset, 90 seconds for full suite.

---

## Per-Task Verification Map

> The planner MUST extend this table with one row per task in each PLAN.md as the plans are written.
> Test Type / Automated Command / File-Exists columns inherit from the per-criterion mapping below.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | (planner fills) | (planner fills) | LIVE-0X | T-60-XX / — | (planner fills) | unit/integration/stress | (planner fills) | ❌ W0 | ⬜ pending |

### Acceptance-Criterion Map (CONTEXT.md #1–#13 → tests)

| Crit | Behavior | Requirement | Test Type | Automated Command | File Exists |
|------|----------|-------------|-----------|-------------------|-------------|
| #1 | `internal/kernel/` does not import `internal/semantic/...` | LIVE-07 | unit (vet analyzer) | `go test ./internal/lint/nokernel2semantic/...` | ❌ W0 — new analyzer + `cmd/vet-nokernel2semantic/` |
| #2 | `EditNotifier.OnEdit` returns within O(microseconds) | LIVE-07 | integration | `go test ./internal/kernel/edit/... -run TestOnEditNonBlocking` | ❌ W0 — `internal/kernel/edit/notifier_integration_test.go` |
| #3 | Editor-fixture suite (Vim, JetBrains, VS Code) catches every save | LIVE-02 | integration (editor fixtures) | `go test ./internal/semantic/live/watcher/... -run TestEditorFixtures -tags editor` | ❌ W0 — `internal/semantic/live/testdata/editors/{vim,jetbrains,vscode}/` |
| #4 | `CoalesceEvents` 6 merge rules + bulk_update threshold | LIVE-04 | unit (table-driven) | `go test ./internal/semantic/live/coalescer/... -run TestCoalesceEvents` | ❌ W0 — `coalesce_test.go` |
| #5 | bulk_update collapse fires when > 200 events at flush time | LIVE-06 | integration | `go test ./internal/semantic/live/coalescer/... -run TestBulkUpdateCollapse` | ❌ W0 — `bulk_test.go` |
| #6 | Schema migration v2→v3 succeeds on clean / Phase-57 / Phase-59 stores | LIVE-05 | unit (migration round-trip) | `go test ./internal/semantic/store/... -run TestMigration003` | ❌ W0 — extends `migrations_test.go` |
| #7 | Per-tx `overlay_epoch` advancement under N concurrent BeginOverlayTx | LIVE-05 | stress / property | `go test ./internal/semantic/store/... -run TestOverlayEpochConcurrent -race` | ❌ W0 — `overlay_concurrent_test.go` |
| #8 | No-op coalesced batches do not advance epoch | LIVE-05 | unit | `go test ./internal/semantic/live/... -run TestNoOpDoesNotAdvanceEpoch` | ❌ W0 — `noop_test.go` |
| #9 | ENOSPC simulation: one slog.Warn, Status() returns inotify_enospc | LIVE-03 | unit (mock fsnotify) | `go test ./internal/semantic/live/watcher/... -run TestENOSPCFallback` | ❌ W0 — `enospc_test.go` |
| #10 | Manifest scan detects watcher misses within 2× interval | LIVE-03 | integration | `go test ./internal/semantic/live/scanner/... -run TestScannerCatchesWatcherMisses -timeout 30s` | ❌ W0 — `scanner_integration_test.go` |
| #11 | `helix_semantic_live_updates_total{kind, outcome}` bounded labels | LIVE-04 | unit (label allowlist) | `go test ./internal/obs/... -run TestMetricsLabelsAllowlist_LiveUpdates` | ❌ W0 — extends `metrics_labels_test.go` |
| #12 | 8 kernel edit/fileops tools each emit OnEdit on success | LIVE-07 | integration (mock notifier) | `go test ./internal/kernel/edit/... ./internal/kernel/fileops/... -run TestOnEditCalledOnSuccess` | ❌ W0 — per-tool integration test |
| #13 | LIVE-01..LIVE-07 marked Done in REQUIREMENTS.md | (close-out) | manual | `grep -E '^- \[x\] \*\*LIVE-0[1-7]\*\*' .planning/REQUIREMENTS.md \| wc -l` | manual-only |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/lint/nokernel2semantic/analyzer.go` + `cmd/vet-nokernel2semantic/main.go` — modeled on `internal/lint/noduckdb/` (acceptance #1)
- [ ] `internal/semantic/live/coalescer/coalesce_test.go` + `bulk_test.go` — table-driven coalesce + flush-time bulk collapse (acceptance #4, #5)
- [ ] `internal/semantic/store/overlay_concurrent_test.go` — concurrent `BeginOverlayTx` race-tagged (acceptance #7)
- [ ] `internal/semantic/store/migrations_test.go` extension — round-trip on clean / Phase-57 / Phase-59 stores (acceptance #6)
- [ ] `internal/semantic/live/watcher/enospc_test.go` — uses fake fsnotify backend (acceptance #9)
- [ ] `internal/semantic/live/scanner/scanner_integration_test.go` — real tmpfs (acceptance #10)
- [ ] `internal/semantic/live/testdata/editors/{vim,jetbrains,vscode}/` — fixture dirs + golden event sequences (acceptance #3)
- [ ] `internal/semantic/live/handler/noop_test.go` — no-op flush epoch invariance (acceptance #8)
- [ ] Per-tool integration tests for the 8 hook entry points — extends existing `internal/kernel/edit/outcome_emission_test.go` and `internal/kernel/fileops/outcome_emission_test.go` (acceptance #12)
- [ ] `internal/obs/metrics_labels_test.go` extension — `helix_semantic_live_updates_total{kind, outcome}` allowlist (acceptance #11)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| LIVE-01..LIVE-07 marked `[x]` in `.planning/REQUIREMENTS.md` | LIVE-01..07 (close-out) | REQUIREMENTS.md is human-edited at phase close | At phase close, run `grep -E '^- \[x\] \*\*LIVE-0[1-7]\*\*' .planning/REQUIREMENTS.md \| wc -l` and confirm output is `7`. |
| Vim editor-fixture skip on Windows CI | LIVE-02 (#3) | No Vim binary on Windows runners — skip predicate, not a failure | Confirm `t.Skip` fires on `runtime.GOOS == "windows"` and the JetBrains + VS Code Go-program fixtures still run. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies recorded above
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (10 items above)
- [ ] No watch-mode flags in test invocations
- [ ] Feedback latency < 5s (subset) / < 90s (full)
- [ ] `nyquist_compliant: true` set in frontmatter once planner fills the per-task table

**Approval:** pending
