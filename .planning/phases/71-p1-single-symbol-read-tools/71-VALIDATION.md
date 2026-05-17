---
phase: 71
slug: p1-single-symbol-read-tools
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-17
---

# Phase 71 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `71-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `-race` |
| **Config file** | None — Go convention (`*_test.go` colocated) |
| **Quick run command** | `go test ./internal/skill/semantic/ -race -count=1` |
| **Full suite command** | `go test ./internal/skill/semantic/... ./internal/semantic/... -race -count=1` |
| **Estimated runtime** | ~5s quick, ~30s full |

---

## Sampling Rate

- **After every task commit:** `go vet ./internal/skill/semantic/ && go test ./internal/skill/semantic/ -race -count=1`
- **After every plan wave:** `go test ./internal/skill/semantic/... ./internal/semantic/... -race -count=1`
- **Before `/gsd:verify-work`:** `go test ./... -race -count=1` green; `go vet ./...` clean; CI grep-gate green for the three new handler files.
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Task IDs are placeholders pending planner assignment; columns track the requirement-to-test mapping derived from CONTEXT.md D1–D7 and the five Phase 71 success criteria.

| Behavior | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|----------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| Seed resolution: exact / ambiguous / not_found (D1) | 71-01 | 1 | P1TOOL-01/02/06 | — | Closed-enum `resolution`; capped `ambiguous_candidates` ≤5 | unit | `go test ./internal/skill/semantic/ -run TestSeedResolve -race` | ❌ W0 | ⬜ pending |
| `QuerySymbolByName` accessor on `*Store` (D1) | 71-01 | 1 | P1TOOL-01/02/06 | — | Read-only; no overlay write | unit | `go test ./internal/semantic/store/ -run TestQuerySymbolByName -race` | ❌ W0 | ⬜ pending |
| `LatestExtractorRunID` accessor (D5) | 71-01 | 1 | Succ.crit. #5 | — | Returns monotonic id; nil-safe | unit | `go test ./internal/semantic/store/ -run TestLatestExtractorRunID -race` | ❌ W0 | ⬜ pending |
| Edge-kind surface round-trip (D3) | 71-02 | 1 | Succ.crit. #1, #5 | — | All known internal → surface; unmapped → `other` with `internal_kind` populated | unit | `go test ./internal/skill/semantic/ -run TestEdgeKindSurface_RoundTrip` | ❌ W0 | ⬜ pending |
| Freshness envelope V2 shape (D5) | 71-02 | 1 | Succ.crit. #5 | — | Closed-enum `status ∈ {current,stale,unknown}`; carries `graph_version`, `snapshot_id`, `extractor_run_id`, `as_of_unix_ms` | unit | `go test ./internal/skill/semantic/ -run TestEnvelope_V2_Shape` | ❌ W0 | ⬜ pending |
| `explain_symbol_deep` happy path (D2, D5, D6) | 71-03 | 2 | P1TOOL-01 | — | Type chain + edges + callers + cluster ref + envelope | unit | `go test ./internal/skill/semantic/ -run TestExplainSymbolDeep -race` | ❌ W0 | ⬜ pending |
| `explain_symbol_deep` truncation (D2) | 71-03 | 2 | P1TOOL-01 | — | `*_count_total` vs `*_count_returned` reported; caps at 50 callers / 100 edges per direction | unit | `go test ./internal/skill/semantic/ -run TestExplainSymbolDeep_Truncation` | ❌ W0 | ⬜ pending |
| `explain_symbol_deep` degraded path (D6) | 71-03 | 2 | P1TOOL-01 | — | TYPES-04 cap ≤0.6; `fallback_reason` populated | unit | `go test ./internal/skill/semantic/ -run TestExplainSymbolDeep_Degraded` | ❌ W0 | ⬜ pending |
| `find_related_symbols` ranking (D2) | 71-04 | 2 | P1TOOL-02 | — | PageRank-from-seeds + cluster co-membership + RRF; default k=20, clamp [1,100] | unit | `go test ./internal/skill/semantic/ -run TestFindRelatedSymbols -race` | ❌ W0 | ⬜ pending |
| `find_related_symbols` paths strict-subset (D2) | 71-04 | 2 | P1TOOL-02 | — | Results filtered by `paths`; seed itself unconstrained | unit | `go test ./internal/skill/semantic/ -run TestFindRelatedSymbols_PathsFilter` | ❌ W0 | ⬜ pending |
| `find_related_symbols` empty result | 71-04 | 2 | P1TOOL-02 | — | Isolated seed → empty list, not error | unit | `go test ./internal/skill/semantic/ -run TestFindRelatedSymbols_Empty` | ❌ W0 | ⬜ pending |
| Multi-language populated-graph fixture (Succ.crit. #1) | 71-04 | 2 | All P1TOOL | — | At least one Go + one TS + one Java symbol with CALLS / RESOLVES_TO / USES_TYPE edges | fixture | `go test ./internal/skill/semantic/ -run TestPopulatedGraphFixture -race` | ❌ W0 | ⬜ pending |
| `validate_graph_edge` happy path (D4) | 71-05 | 3 | P1TOOL-06 | — | Confidence ∈ [0,1]; `evidence_status: complete`; all three sources cited | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge -race` | ❌ W0 | ⬜ pending |
| `validate_graph_edge` lenient empty evidence (D4) | 71-05 | 3 | P1TOOL-06 | — | Graph-attested edge with no citations → `evidence_status: none`, capped confidence | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge_LenientNone` | ❌ W0 | ⬜ pending |
| `validate_graph_edge` TYPES-04 cap (D4, D6) | 71-05 | 3 | P1TOOL-06 | — | Degraded type-resolver tier 6/7 → top-level `confidence` ≤0.6 via `types.CapCommentConfidence`; per-source `confidence_contribution` unclamped | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge_TYPES04Cap` | ❌ W0 | ⬜ pending |
| Edge absent (D4) | 71-05 | 3 | P1TOOL-06 | — | Low confidence + `fallback_reason: "edge_not_found"` | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge_Absent` | ❌ W0 | ⬜ pending |
| Three-tool cross-consistency (D7 #4) | 71-05 | 3 | Succ.crit. #1, #5 | — | `explain_symbol_deep` incoming-edges agree with `validate_graph_edge` confirmations for sampled edges | integration | `go test ./internal/skill/semantic/ -run TestThreeTools_CrossConsistency -race` | ❌ W0 | ⬜ pending |
| Race-clean under concurrent invocation (D7 #5, Succ.crit. #5) | 71-05 | 3 | All P1TOOL | — | N goroutines per tool, same seed; identical responses, no race-detector hits | integration | `go test ./internal/skill/semantic/ -run TestThreeTools_Concurrent -race -count=10` | ❌ W0 | ⬜ pending |
| `read+` mode tier enforced at handler entry (D6, Succ.crit. #4) | 71-03/04/05 | 2–3 | Succ.crit. #4 | — | `read` mode rejects; `read+`/`review`/`admin` accept | unit | `go test ./internal/skill/semantic/ -run TestThreeTools_ModeEnforcement` | ❌ W0 | ⬜ pending |
| Envelope shape parity across three tools (Succ.crit. #5) | 71-05 | 3 | Succ.crit. #5 | — | Closed-enum `freshness`, `source`, `fallback_reason` present on all three tools | unit + integration | `go test ./internal/skill/semantic/ -run TestThreeTools_EnvelopeShape -race` | ❌ W0 | ⬜ pending |
| CI grep-gate extension (D6) | 71-05 | 3 | Succ.crit. #4 | — | `Begin/Commit/Abort/Write` tokens absent from `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go` | CI gate | grep runner (see Open Question 1) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/skill/semantic/seed_resolve_test.go` — D1 resolution variants
- [ ] `internal/skill/semantic/edge_kind_surface_test.go` — D3 mapping round-trip
- [ ] `internal/skill/semantic/envelope_v2_test.go` — D5 envelope shape
- [ ] `internal/skill/semantic/tools_explain_symbol_test.go` — P1TOOL-01
- [ ] `internal/skill/semantic/tools_find_related_test.go` — P1TOOL-02
- [ ] `internal/skill/semantic/tools_validate_edge_test.go` — P1TOOL-06
- [ ] `internal/skill/semantic/populated_graph_fixture_test.go` — multi-language Go/TS/Java fixture
- [ ] `internal/skill/semantic/integration_test.go` — extend with `TestThreeTools_*` (cross-consistency, concurrent, envelope shape, mode enforcement)
- [ ] `internal/semantic/store/effective_graph_test.go` — `TestQuerySymbolByName`, `TestLatestExtractorRunID`

No framework install needed — Go `testing` ships with the toolchain.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live MCP `tools/list` shows the three new tools under `read+` profile filter | Succ.crit. #4 | Profile/mode 5×4 matrix tests deferred to Phase 73; Phase 71 only enforces handler-entry tier | Run `helix daemon` against a populated workspace; invoke `tools/list` via forwarder under a `read+` profile; observe `explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge` are present |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency <30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
