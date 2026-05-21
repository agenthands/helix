# Requirements: Helix v1.11 — Semantic Index Completion & P1 MCP Tools

**Defined:** 2026-05-12
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Source of truth:** `SPEC-DRAFT.md` (P1 tool contracts) and `.planning/milestones/v1.10-ROADMAP.md` (completion-item anchors). Research synthesis: `.planning/research/SUMMARY.md` (to be (re)generated for v1.11 P1-tool details if needed).

---

## Status legend

- `- [ ]` pending
- `- [x]` complete
- `- [~]` won't-do (rejected with rationale appended after em-dash)

---

## v1.11 Requirements

Each requirement is testable from an agent/user perspective and maps to one roadmap phase.

### DIFF — Precise FileFactDiff Populator (closes DEF-67-F01-FULL-DIFF)

- [ ] **DIFF-01**: `internal/semantic/live/handler/difffacts.go` Tier-1 (full diff old↔new FileFact) populator active in production. When a live edit fires, the post-commit hook computes per-symbol additions/removals/changes and per-edge additions/removals against the pre-edit FileFact and calls `FileFactDiffRecorder.RecordSymbol*` / `RecordEdge*` accordingly. Tier-3 synthetic-marker fallback retained only for the cold-start / missing-pre-edit-FileFact case.
- [ ] **DIFF-02**: Pre-edit FileFact accessor lands on `*Store` or via a new live-handler-local cache (whichever design wins under research/discussion). Whatever the seam, it must be readable from the post-commit hook without violating the kernel↔semantic boundary (vet-nokernel2semantic stays green).
- [ ] **DIFF-03**: `graph_version` advances on every live edit, AND ApplyRepair receives non-empty per-symbol deltas in production. Verified by `internal/semantic/live/handler/handler_diff_e2e_test.go` (new): seed a workspace, edit a single symbol, assert recorder receives exactly N RecordSymbolChanged calls + ApplyRepair fires with non-empty diff set. Race-clean.
- [ ] **DIFF-04**: Tier-2 (added-only) fallback exercised when Tier-1 diffing is infeasible (e.g., extractor returned partial extraction). Behavior is graceful degrade, not error.

### STATUS — Production Status Accessors (closes TOOL-03 / TOOL-04 placeholder annotations)

- [ ] **STATUS-01**: `semSchedulerAdapter.ClusterStatus` at `internal/daemon/semantic_wiring.go:408` returns real `{state, reason, computed_at, member_count, ...}` from the Phase 62 cluster engine (`internal/semantic/graph/cluster*.go`) instead of `{state:"unknown", reason:"phase-62-clustering-no-status-accessor"}`. New `*Store` accessor `ClusterStatusForGraphVersion(ctx, graphVersion)` or equivalent.
- [ ] **STATUS-02**: Retrieval status accessors at `semantic_wiring.go:441,450` return real `{corpus_version, indexed_files, indexed_symbols, last_compact_at}` from the bleve engine + corpus mapper. Surfaces in `get_semantic_graph_status` response.
- [ ] **STATUS-03**: `get_semantic_graph_status` integration test (`internal/mcp/integ/.../graph_status_e2e_test.go` or similar) asserts non-placeholder values across cluster_status + retrieval_status on a populated workspace.

### REFRESH — Incremental refresh_semantic_graph (closes refresh-degraded annotation)

- [x] **REFRESH-01**: `collectCandidatePaths` in `internal/daemon/semantic_wiring.go:1466-1475` (or the path it migrates to) consults the overlay-drain seam in `internal/semantic/live/` for incremental mode instead of full-walk. Full-walk remains as fallback when overlay-drain returns empty (e.g., overlay was rotated).
- [x] **REFRESH-02**: Performance: `refresh_semantic_graph` with `mode:"incremental"` against a 10k-symbol workspace where exactly 1 file changed completes in ≤ 200ms p95 on the project bench harness. (Benchmark is local-only per project rule.)
- [x] **REFRESH-03**: Incremental fallback behavior verified by `internal/eval/runner/refresh_incremental_test.go` (or eval-side) — when overlay-drain is empty, refresh falls back to full-walk and logs the reason with bounded label.

### P1TOOL — P1 MCP Tools (6 new tools)

- [ ] **P1TOOL-01**: `explain_symbol_deep` MCP tool — deep symbol explanation including: type chain (resolved confidence + ladder tier per Phase 62 TYPES-01..04), callers (effective-graph BFS), edges (incoming + outgoing, classified by kind), cluster membership, freshness envelope. Mode tier: `read+`. Response: closed-enum envelope mirroring v1.10 conventions.
- [ ] **P1TOOL-02**: `find_related_symbols` MCP tool — given a seed symbol, returns ranked related symbols via PageRank-from-seeds + cluster co-membership + RRF fusion. Mode tier: `read+`. Honors paths filter (strict-subset like refresh).
- [ ] **P1TOOL-03**: `get_cluster_map` MCP tool — workspace-level overview of weak-component clusters: count, size distribution, top-N labeled clusters with member counts + representative symbols + dominant edge kinds. Mode tier: `read+`.
- [ ] **P1TOOL-04**: `explain_cluster` MCP tool — per-cluster (by cluster_id from get_cluster_map) detailed view: full member list, ranking within cluster, cohesion/separation scores, dominant entry-point symbols. Mode tier: `read+`.
- [ ] **P1TOOL-05**: `get_change_impact_graph` MCP tool — pre-edit blast-radius via the semantic graph (different shape from `analyze_blast_radius`: returns the **graph subgraph** rather than file-level summary). Mode tier: `review+`. Confidence cap on fallback.
- [ ] **P1TOOL-06**: `validate_graph_edge` MCP tool — given an edge claim `(from_symbol, to_symbol, edge_kind)`, returns confidence + evidence path (LSP citations, AST citations, type-resolver derivation) explaining whether the edge exists. Mode tier: `read+`.
- [x] **P1TOOL-07**: All 6 P1 tools profile/mode gated via the existing `internal/profile/` matrix; `tools/list` filters per active profile; `get_tool_help` returns parameter docs for each.
- [ ] **P1TOOL-08**: All 6 P1 tools wrapped in the semantic skill (`internal/skill/semantic/`) following the v1.10 pattern (envelope.go + mode_check.go + accessors.go); zero new daemon-bootstrap special-casing.
- [ ] **P1TOOL-09**: All 6 P1 tools verified by E2E integration tests against a real workspace + populated graph; closed-enum freshness/source/confidence envelope fields present and correct.

---

## v2 Requirements (Deferred to v1.12+)

Tracked but not in v1.11 roadmap. Anchors so future planning can find them.

### v1.10 Carryover (release-pipeline platform debt)

- **REL-NOTARIZE-01**: Apple Developer ID notarization for darwin archives (DEF-59-NOTARIZE)
- **REL-DARWIN-CANARY-01**: Post-publish darwin smoke test on macos runner (DEF-59-DARWIN-CANARY)
- **REL-WIN-ARM64-01**: Restore windows/arm64 archive (DEF-59-WIN-ARM64-RESTORE)
- **REL-INSTALL-01**: INSTALL.md tarball perms (chmod +x) follow-up

### Production Eval Program (the v1.10 harness data flywheel)

- **EVALPROG-01**: Run real `claude-code` agent comparisons (baseline vs +helix) on a hundreds-of-task corpus
- **EVALPROG-02**: Iterate on profiles/guardrails based on calibrated data
- **EVALPROG-03**: Smart sampling for the production flywheel (weight toward concerning-signal interactions per ai-evals.md)

---

## Out of Scope (won't-do, project rules)

| Feature | Reason |
|---------|--------|
| Package-manager distribution (Homebrew, Scoop, deb/rpm/AUR, Chocolatey, winget, nix) | **Hard project rule.** Helix ships signed multi-arch binary archives only — `helix upgrade` verifies cosign keyless bundles + atomic-swaps from the archives. See Phase 58 D-01 + memory `feedback_no_packages.md`. |
| CI bench gates | **Hard project rule.** Benchmarks are local-only — never on hosted CI runners. See memory `feedback_no_ci_benchmarks.md`. |
| LLM-judge results gating CI | **Phase 67 EVAL-07.** Judge is informational; CI grep-gate forbids any reference to `tool_behavior_judge.json` in workflow files. |

---

## Traceability

Which phases cover which requirements. Updated during roadmap creation. Phase numbers continue from v1.10 (last was Phase 67; v1.11 starts at Phase 68).

| Requirement | Phase | Status |
|-------------|-------|--------|
| DIFF-01 | Phase 68 | Pending |
| DIFF-02 | Phase 68 | Pending |
| DIFF-03 | Phase 68 | Pending |
| DIFF-04 | Phase 68 | Pending |
| STATUS-01 | Phase 69 | Pending |
| STATUS-02 | Phase 69 | Pending |
| STATUS-03 | Phase 69 | Pending |
| REFRESH-01 | Phase 70 | Complete |
| REFRESH-02 | Phase 70 | Complete |
| REFRESH-03 | Phase 70 | Complete |
| P1TOOL-01 | Phase 71 | Pending |
| P1TOOL-02 | Phase 71 | Pending |
| P1TOOL-03 | Phase 72 | Pending |
| P1TOOL-04 | Phase 72 | Pending |
| P1TOOL-05 | Phase 72 | Pending |
| P1TOOL-06 | Phase 71 | Pending |
| P1TOOL-07 | Phase 73 | Complete |
| P1TOOL-08 | Phase 73 | Pending |
| P1TOOL-09 | Phase 73 | Pending |
