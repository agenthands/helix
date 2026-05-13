---
phase: 68
slug: precise-filefactdiff-populator
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-13
---

# Phase 68 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `68-RESEARCH.md` → "Validation Architecture" section.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` stdlib + `go test -race` |
| **Config file** | none — standard Go convention |
| **Quick run command** | `go test ./internal/semantic/live/handler/... ./internal/semantic/store/... ./internal/semantic/extract/... -count=1` |
| **Full suite command** | `go test ./... -race -count=1` (matches CLAUDE.md mandate) |
| **Estimated runtime** | ~30s quick / ~3-5min full suite |

---

## Sampling Rate

- **After every task commit:** Run the quick command (handler + store + extract packages, `-race`).
- **After every plan wave:** `go test -race ./internal/semantic/... ./internal/obs/... -count=1 && make vet`
- **Before `/gsd-verify-work`:** Full suite + `make vet` must be green.
- **Max feedback latency:** ~30s for the quick path.

---

## Per-Task Verification Map

> Plan-level mapping (task IDs are placeholders pending planner output 68-01..68-05).

| Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists |
|------|------|-------------|-----------------|-----------|-------------------|-------------|
| 68-01 | 1 | DIFF-02 | `GetLatestFileFact` returns overlay row on hit | unit | `go test -race -run TestGetLatestFileFact_OverlayHit ./internal/semantic/store/...` | ❌ W0 |
| 68-01 | 1 | DIFF-02 | snapshot fallback when overlay absent | unit | `go test -race -run TestGetLatestFileFact_SnapshotFallback ./internal/semantic/store/...` | ❌ W0 |
| 68-01 | 1 | DIFF-02 | cold-start returns `(_, false, nil)` | unit | `go test -race -run TestGetLatestFileFact_ColdStart ./internal/semantic/store/...` | ❌ W0 |
| 68-01 | 1 | DIFF-02 | `vet-nokernel2semantic` invariant intact | smoke | `make vet` | ✅ existing |
| 68-02 | 1 | DIFF-01 | Go `ExtractFile` happy path | unit | `go test -race ./internal/semantic/extract/golang/...` | ❌ W0 |
| 68-02 | 1 | DIFF-01 | TS `ExtractFile` happy path | unit | `go test -race ./internal/semantic/extract/typescript/...` | ❌ W0 |
| 68-03 | 1 | DIFF-04 | outcome metric closed-enum drop on unknown tier | unit | `go test -race ./internal/obs/...` | ❌ W0 |
| 68-04 | 2 | DIFF-01 | `diffSymbols` flag computation | unit | `go test -race -run TestDiffSymbols ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-04 | 2 | DIFF-01 | body-only edits do not flag graph-changing | unit | `go test -race -run TestDiffSymbols_BodyOnlyNotGraphChanging ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-04 | 2 | DIFF-04 | Tier-2 fires on `ExtractionStatusPartial` | unit | `go test -race -run TestTryAddedOnlyDiff_Partial ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-04 | 2 | success crit 5 | Tier-3 bounded-reason metric (cold/failed/unsupported) | unit | `go test -race -run TestTier3_BoundedReasonMetric ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-04 | 2 | DIFF-04 | outcome metric records `tier="added-only"` | unit | `go test -race -run TestFileFactDiffOutcomeMetric ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-05 | 3 | DIFF-01,03 | end-to-end live edit → precise diff → non-empty repair | e2e | `go test -race -run TestE2E_LiveEditFiresPreciseDiff ./internal/semantic/live/handler/...` | ❌ W0 |
| 68-05 | 3 | DIFF-03 | E2E race-clean | e2e | same as above with `-race` (default) | ❌ W0 |
| all | all | regression | existing handler tests pass | unit | `go test -race ./internal/semantic/live/handler/... -count=1` | ✅ existing |
| all | all | regression | existing graph tests pass | unit | `go test -race ./internal/semantic/graph/... -count=1` | ✅ existing |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · ❌ W0 = file does not exist yet, Wave 0 must create it*

---

## Wave 0 Requirements

- [ ] `internal/semantic/store/filefact_accessor_test.go` — DIFF-02 (overlay hit, snapshot fallback, cold-start, error propagation).
- [ ] `internal/semantic/extract/golang/provider_extract_file_test.go` — Go `ExtractFile` (happy / file-not-found / large-file → partial).
- [ ] `internal/semantic/extract/typescript/provider_extract_file_test.go` — TS `ExtractFile` happy path.
- [ ] `internal/semantic/live/handler/difffacts_test.go` — `diffSymbols`, `tryFullDiff`, `tryAddedOnlyDiff`, Tier-3 dispatch.
- [ ] `internal/semantic/live/handler/handler_diff_e2e_test.go` — DIFF-03 end-to-end against real `*Store` + real `extract.Registry`.
- [ ] Extend `internal/semantic/live/handler/export_test.go` — add `LastRecorderSnapshotForTest` accessor.
- [ ] No framework install needed — Go stdlib covers everything.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Metric label cardinality observed in real Prometheus scrape under load | success crit 5 | Requires live daemon + scraping infra; out of scope for unit/E2E | Optional post-merge: run daemon, drive HelixEdit traffic, scrape `/metrics`, confirm only 3 tier values and bounded reason values appear |

*All phase requirements have automated verification; the table above is observational only.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies.
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify.
- [ ] Wave 0 covers all MISSING references (six files / one accessor extension).
- [ ] No watch-mode flags (Go testing has none in use).
- [ ] Feedback latency < 60s for quick path.
- [ ] `nyquist_compliant: true` set in frontmatter once Wave 0 is scoped into PLAN.md.

**Approval:** pending
