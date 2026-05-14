---
phase: 69
slug: production-status-accessors
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-05-14
---

# Phase 69 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — `go.mod` at repo root |
| **Quick run command** | `go test ./internal/semantic/cluster/... ./internal/semantic/retrieval/... ./internal/semantic/compact/... ./internal/daemon/... ./internal/skill/semantic/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60s quick / ~5min full |

---

## Sampling Rate

- **After every task commit:** Run quick command on the touched package
- **After every plan wave:** Run quick command across all Phase 69 packages
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds for the quick command set

---

## Wave 0 Status

The RED tasks across Plans 69-01, 69-02, 69-03, and 69-04 ARE the Wave 0 test stubs for this phase. They are not a separate Wave 0 — they are tasks `*-T1` in each TDD plan, each landing a failing test scaffold that the subsequent GREEN task makes pass. The frontmatter `wave_0_complete: true` reflects the fact that the RED-task work is planned end-to-end (every required scaffold has a concrete commit-bearing task).

Specifically:
- `69-01-T1` lands `internal/semantic/store/effective_graph_test.go` stubs for `ClusterStatusForGraphVersion` including race-clean read-path test (STATUS-01).
- `69-02-T1` lands `internal/semantic/retrieval/recovery_test.go` stubs for `corpus_version` / `last_compact_at` / `indexed_files` derivations + exported MetaKey* constant compile assertion (STATUS-02).
- `69-03-T1` lands `internal/semantic/compact/compactor_test.go` stubs for `BleveMeta` Deps injection + `last_compact_at` write (STATUS-02).
- `69-04-T1` lands `internal/skill/semantic/envelope_test.go` stubs verifying additive `ClusterStatus.{ComputedAt,MemberCount}` and new nested `RetrievalStatus` JSON shape + closed-enum Reason coverage (STATUS-01, STATUS-02).

The integration test for STATUS-03 (`69-06-T1`) is authored in a non-TDD execute plan (Wave 4) because the underlying production code paths must exist first; STATUS-03 is an end-to-end behavior test, not a unit-test scaffold.

---

## Per-Task Verification Map

> Every task in PLAN.md has a row here with a concrete `automated_command`. RED tasks (69-0[1-4]-T1) ARE the Wave 0 stubs — they land the test scaffolds that subsequent GREEN tasks make pass.

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 69-01-T1 | 69-01 | 1 | STATUS-01 | D-09 read-path | unit-race | `go test ./internal/semantic/store/ -run "TestClusterStatusForGraphVersion" -count=1 2>&1 \| grep -E "undefined\|no field\|cannot\|undeclared"` | ❌ W0 (this task creates it) | ⬜ pending |
| 69-01-T2 | 69-01 | 1 | STATUS-01 | D-09 read-path | unit-race | `go test ./internal/semantic/store/ -run "TestClusterStatusForGraphVersion" -count=1 -race -timeout 30s` | ✅ existing (after T1) | ⬜ pending |
| 69-02-T1 | 69-02 | 1 | STATUS-02 | N/A | unit | `go test ./internal/semantic/retrieval/ -run "TestEngine_DocCount\|TestRebuild_WritesCorpusVersionAndFileCount" -count=1 2>&1 \| grep -E "undefined\|cannot\|undeclared\|FAIL"` | ❌ W0 (this task creates it) | ⬜ pending |
| 69-02-T2 | 69-02 | 1 | STATUS-02 | N/A | unit-race | `go test ./internal/semantic/retrieval/ -count=1 -race -timeout 60s` | ✅ existing (after T1) | ⬜ pending |
| 69-03-T1 | 69-03 | 2 | STATUS-02 | N/A | unit | `go test ./internal/semantic/compact/ -run "TestRunCompaction_WritesLastCompactAt" -count=1 2>&1 \| grep -E "undefined\|cannot\|undeclared"` | ❌ W0 (this task creates it) | ⬜ pending |
| 69-03-T2 | 69-03 | 2 | STATUS-02 | N/A | unit-race | `go test ./internal/semantic/compact/ -count=1 -race -timeout 60s && grep -rn '"github.com/agenthands/helix/internal/semantic/compact"' internal/semantic/retrieval/ \| grep -v '^$'` | ✅ existing (after T1) | ⬜ pending |
| 69-03-T3 | 69-03 | 2 | STATUS-02 | N/A | integration | `go build ./internal/daemon/... && go test ./internal/daemon/... -run "TestCompact" -count=1` | ✅ existing | ⬜ pending |
| 69-04-T1 | 69-04 | 1 | STATUS-01, STATUS-02 | N/A | unit | `go test ./internal/skill/semantic/ -run "TestClusterStatus\|TestRetrievalStatus\|TestStatusResult" -count=1 2>&1 \| grep -E "undefined\|no field\|cannot\|undeclared"` | ❌ W0 (this task creates it) | ⬜ pending |
| 69-04-T2 | 69-04 | 1 | STATUS-01, STATUS-02 | N/A | unit | `go test ./internal/skill/semantic/ -run "TestClusterStatus\|TestRetrievalStatus\|TestStatusResult" -count=1` | ✅ existing (after T1; **note: package-wide build intentionally RED until 69-05-T2 — see 69-04 `wave_build_state: intentionally_broken_until_05`**) | ⬜ pending |
| 69-05-T1 | 69-05 | 3 | STATUS-01, STATUS-02 | D-09 read-path | integration | `go build ./internal/daemon/... && grep -nE 'phase-62-clustering-no-status-accessor\|W1 placeholder\|W1 production-layer placeholder\|deliberate companion' internal/daemon/semantic_wiring.go` | ✅ existing | ⬜ pending |
| 69-05-T2 | 69-05 | 3 | STATUS-01, STATUS-02 | D-09 read-path | integration | `go build ./... && go test ./internal/skill/semantic/ ./internal/daemon/ -count=1 -race -timeout 90s` | ✅ existing | ⬜ pending |
| 69-06-T1 | 69-06 | 4 | STATUS-03 | D-09 read-path | integration | `go test ./internal/skill/semantic/ -run "TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval\|TestE2E_IndexThenContext_SymbolCount" -count=1 -race -timeout 120s` | ❌ W0 (this task creates the new E2E test) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|

*None — all Phase 69 success criteria have automated verification (unit + integration).*

---

## D-09 Read-Path Invariant Check

The new `*Store.ClusterStatusForGraphVersion` accessor MUST NOT call `Begin/Commit/Abort/Write`. Enforce via:

- [ ] `go vet ./...` (covers project-local `vet-nokernel2semantic` analyzer)
- [ ] Read-path grep audit pattern (Phase 64 reference): `! grep -nE 'Begin|Commit|Abort|Exec.*INSERT|Exec.*UPDATE' internal/semantic/cluster/persist.go | grep ClusterStatusForGraphVersion`
- [ ] Race test mirrors `TestCountStaleScoreRows_LockFree_RaceSafe` (effective_graph_test.go:242-293) — concurrent reads under `go test -race`
- [ ] Two-query stale-fallback path remains pure SELECTs (no DML)

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (12/12)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING test files listed above (RED tasks are the W0 stubs)
- [x] No watch-mode flags
- [x] Feedback latency < 90s for quick command
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-05-14
