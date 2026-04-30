---
phase: 53
slug: obs-metrics-gaps
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-30
---

# Phase 53 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package + `stretchr/testify/assert` |
| **Config file** | none (Go convention) |
| **Quick run command** | `go test ./internal/obs/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | quick ~5s, full ~60s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/obs/...`
- **After every plan wave:** Run `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/repomap/... ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/... ./internal/daemon/...`
- **Before `/gsd-verify-work`:** Full suite (`go vet ./... && go test ./...`) must be green
- **Max feedback latency:** 30 seconds at wave granularity; 5 seconds per task

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-01 | TBD | 1 | OBS-03 | — | All 5 new metric families register on the obs.Metrics owned registry | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -v` | ✅ extend `want[]` | ⬜ pending |
| TBD-02 | TBD | 1 | OBS-03 | — | New label carve-outs (`result`, `extractor`, `phase`, `transport`) accepted by lint | unit | `go test ./internal/obs -run TestMetricsLabelsAllowlist -v` | ✅ extend primed vectors + carveOuts | ⬜ pending |
| TBD-03 | TBD | 1 | OBS-03 | — | Lint still rejects unknown label names | unit | `go test ./internal/obs -run TestMetricsLabelsAllowlist_catchesDrift -v` | ✅ no change | ⬜ pending |
| 53-02-2.2 | 02 | 2 | OBS-03 | — | lspool emits `result=hit` on share path, `result=miss` on spawn path | unit | `go test ./internal/kernel/lspool -run TestPool_AcquireLease_LookupEmission -v` | ✅ extended in 53-02-2.1 (RED) → 53-02-2.2 (GREEN) | ✅ green |
| TBD-05 | TBD | 1 | OBS-03 | — | repomap cache emits `result=hit` on mtime match, `result=miss` on extractFn path | unit | `go test ./internal/repomap -run TestTagCache_GetOrExtract_LookupEmission -v` | ❌ W0 | ⬜ pending |
| TBD-06 | TBD | 1 | OBS-03 | — | repomap extract histogram observes seconds with `extractor∈{treesitter,lsp,fallback}`, drops unknown | unit | `go test ./internal/repomap -run TestRepoMapExtractObserve -v` | ❌ W0 | ⬜ pending |
| TBD-07 | TBD | 1 | OBS-03 | — | session lifecycle counter emits `phase∈{started,ended,error}` × `transport∈{stdio,http}` from forwarder + http handler | unit | `go test ./internal/daemon -run TestForwarderHandler_SessionLifecycle -v` | ❌ W0 | ⬜ pending |
| TBD-08 | TBD | 1 | OBS-03 | — | edit outcome counter emits per-tool outcome with `strategy=none` for non-fuzzy tools, fuzzy.Strategy values for fuzzy tools | unit | `go test ./internal/kernel/edit -run TestEditTools_OutcomeEmission -v && go test ./internal/kernel/fileops -run TestFileopsTools_OutcomeEmission -v` | ❌ W0 | ⬜ pending |
| TBD-09 | TBD | 1 | OBS-03 | — | mcp.RecordEditOutcome roundtrips through `setEditOutcomeSink` and increments the obs.Metrics vector | unit | `go test ./internal/mcp -run TestRecordEditOutcome -v` | ❌ W0 | ⬜ pending |
| TBD-10 | TBD | 1 | OBS-03 | — | Cardinality bound: lspool_lookups ≤ 2 × N_languages | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_LSPoolLookups -v` | ❌ W0 | ⬜ pending |
| TBD-11 | TBD | 1 | OBS-03 | — | Cardinality bound: repomap_lookups ≤ 2 × N_languages | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_RepoMapLookups -v` | ❌ W0 | ⬜ pending |
| TBD-12 | TBD | 1 | OBS-03 | — | Cardinality bound: repomap_extract_duration label-combo count ≤ 3 × N_languages | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_RepoMapExtract -v` | ❌ W0 | ⬜ pending |
| TBD-13 | TBD | 1 | OBS-03 | — | Cardinality bound: session_lifecycle ≤ 3 × 2 = 6 | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_SessionLifecycle -v` | ❌ W0 | ⬜ pending |
| TBD-14 | TBD | 1 | OBS-03 | — | Cardinality bound: edit_outcome ≤ 7 × 6 × strategy_count = 168 | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_EditOutcome -v` | ❌ W0 | ⬜ pending |
| TBD-15 | TBD | 1 | OBS-03 | — | Noop-default invariant preserved: `obs.Noop(...)` Metrics() exposes all new helpers without panic | unit | `go test ./internal/obs -run TestMetrics_NoopProviderReturnsUsableSink -v` | ✅ extend helper-call list | ⬜ pending |
| TBD-16 | TBD | 1 | OBS-03 | — | Compile-time assertion: `*obs.Metrics` satisfies new `repomap.MetricsSink` | unit | `go test ./internal/daemon -run TestObsMetricsIsRepoMapSink -v` | ❌ W0 | ⬜ pending |
| 53-02-2.2 | 02 | 2 | OBS-03 | — | Compile-time assertion: `*obs.Metrics` satisfies extended `lspool.MetricsSink` (with new LSPoolLookup) | unit | `go test ./internal/daemon -run TestObsMetricsIsLSPoolSink -v` | ✅ existing wiring_test.go assertion auto-validates | ✅ green |
| TBD-18 | TBD | 1 | OBS-03 | — | ROADMAP.md success-criterion-1 uses `helix_*` not `serena_*` | shell | `! grep -q 'serena_lspool_cache_hits_total\|serena_repomap_cache_hits_total\|serena_repomap_extract_duration_seconds\|serena_session_lifecycle_total\|serena_edit_outcome_total' .planning/ROADMAP.md` | manual W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs (TBD-NN) above are placeholders; the planner replaces them with concrete `<phase>-<plan>-<task>` IDs in PLAN.md `<verifications>` blocks.*

---

## Wave 0 Requirements

- [ ] `internal/repomap/metrics.go` — NEW MetricsSink interface + NoopSink + extractor/result constants
- [ ] `internal/repomap/metrics_test.go` — NEW emission + interface satisfaction tests
- [ ] `internal/kernel/lspool/metrics_test.go` — EXTEND `recordingSink` with `lookups []lookupEvent`; ADD `TestPool_AcquireLease_LookupEmission`
- [ ] `internal/kernel/edit/tools_test.go` — NEW; assert outcome+strategy emission for each of 5 handlers (`replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`)
- [ ] `internal/kernel/fileops/tools_test.go` — NEW; assert outcome+strategy for `replace_in_file` + `fuzzy_edit`
- [ ] `internal/mcp/middleware_test.go` — EXTEND with `TestRecordEditOutcome` parallel to existing rename tests
- [ ] `internal/daemon/forwarder_test.go` (or extend `wiring_test.go`) — `TestForwarderHandler_SessionLifecycle` covering stdio AND http transports
- [ ] `internal/obs/metrics_labels_test.go` — EXTEND primed vectors in `TestMetricsLabelsAllowlist` + ADD 5 cardinality bound tests
- [ ] `internal/daemon/wiring_test.go` — ADD `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` line + companion runtime test

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| USAGE.md documents all 5 new metrics with labels and semantics | OBS-03 | Documentation prose review beyond grep | Review §"Prometheus Metrics" lines after edit; confirm each metric appears with its label set, label value enum, and one-line semantics; confirm new PromQL example for hit-ratio is present |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
