---
phase: 74
slug: close-gap-wire-p1-tool-accessors-in-production-daemon
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-29
---

# Phase 74 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go stdlib, table-driven + black-box `package semantic_test` per Phase 73 D-04a) |
| **Config file** | none — existing repo conventions (`go.mod`, `Makefile`) |
| **Quick run command** | `go test -race -count=1 ./internal/daemon/... ./internal/skill/semantic/...` |
| **Full suite command** | `go vet ./... && go test -race -count=1 ./...` |
| **Estimated runtime** | quick ~30s; full ~3–5 min |

---

## Sampling Rate

- **After every task commit:** Run quick `go test -race -count=1 ./internal/daemon/... ./internal/skill/semantic/...`
- **After every plan wave:** Run quick + `go vet ./...` and the two repo-specific build tags `vet-nokernel2semantic` / `vet-noduckdb`
- **Before `/gsd:verify-work`:** Full suite green AND both bootstrap (D-03) and production-path E2E (D-03a) pass
- **Max feedback latency:** 30 seconds (quick); 5 min (full)

---

## Per-Task Verification Map

> Filled by planner. Skeleton enumerates the audit-driven assertions the wiring tests
> MUST cover. Planner expands per task with task IDs.

| Assertion ID | Tied to Decision | Test Type | Automated Command | Notes |
|--------------|------------------|-----------|-------------------|-------|
| A-01 | D-03 | unit (bootstrap) | `go test -race ./internal/daemon -run TestSemanticBundleWiresP1Accessors` | After `newSemanticBundle`, `WiredAccessorsForTest()` reports the 8 folded accessors non-nil. |
| A-02 | D-03 | unit (bootstrap) | same | Reports `TypeChain=false` and `EdgeEvidence=false` (research-confirmed deferral to Phase 75). |
| A-03 | D-04 | unit | `grep -E '"setters", 14' internal/daemon/semantic_wiring.go` (or assertion in bootstrap test) | Setters log line updated from 8 → 14. |
| A-04 | D-01 / D-03a | E2E | `go test -race ./internal/skill/semantic -run TestP1E2EProductionPath` | `explain_symbol_deep` produces non-empty envelope, `fallback_reason == ""` (SymbolByName + SymbolEdges wired). |
| A-05 | D-01 / D-03a | E2E | same | `find_related_symbols` envelope populated, `fallback_reason == ""`. |
| A-06 | D-01 / D-03a | E2E | same | `validate_graph_edge` envelope populated, `fallback_reason == ""` (SymbolByName + SymbolEdges). |
| A-07 | D-01 / D-03a | E2E | same | `get_cluster_map` envelope populated, `fallback_reason == ""` (ClusterMap + ClusterPageRank wired). |
| A-08 | D-01 / D-03a | E2E | same | `explain_cluster` envelope populated, `fallback_reason == ""` (ClusterMap + ClusterMember + ClusterMembership wired). |
| A-09 | D-01 / D-03a | E2E | same | `get_change_impact_graph` envelope populated, `fallback_reason == ""` (ImpactLookup wired). |
| A-10 | D-03b | E2E | same | For each handler that depends on `TypeChainAccessor` or `EdgeEvidenceAccessor`, when the accessor is nil the envelope carries the documented `fallback_reason: *_unavailable` value. |
| A-11 | "Specific Ideas" — SymbolEdges direction coverage | E2E | same | At least one handler-test pair exercises each `SymbolEdgesAccessor` direction (callers / incoming / outgoing). |
| A-12 | D-02 / D-02a | E2E | same | FreshnessV2 envelope reports non-empty `extractor_run_id` and status `current` after `SetExtractorRun` is wired. |
| A-13 | repo build invariants | static | `go vet -tags vet-nokernel2semantic ./...` and `go vet -tags vet-noduckdb ./...` | New adapter files must not violate either import boundary. |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/skill/semantic/export_p1_test.go` — research-recommended seam exporting `WiredAccessorsForTest()` returning a typed bool struct (package `semantic`, NOT `semantic_test`, to access unexported skill fields). Required by A-01 / A-02.
- [ ] No new framework install — Go stdlib + existing `package semantic_test` black-box pattern from Phase 73.

*Existing infrastructure covers all phase requirements aside from the test export above.*

---

## Manual-Only Verifications

| Behavior | Decision | Why Manual | Test Instructions |
|----------|----------|------------|-------------------|
| (none) | — | All Phase 74 assertions can be automated against `*semanticBundle` and `Handle*ForTest`. | — |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers `WiredAccessorsForTest()` export prerequisite
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s (quick) / < 5min (full)
- [ ] `nyquist_compliant: true` set in frontmatter once planner expands per-task table

**Approval:** pending
