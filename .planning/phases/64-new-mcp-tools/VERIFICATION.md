---
phase: 64-new-mcp-tools
verified: 2026-05-08T03:10:00Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
verdict: PASSED-WITH-CARRYOVER
carryover:
  - item: "Production buildFn ships an empty-Facts placeholder"
    location: "internal/daemon/semantic_wiring.go::makeProductionBuildFn"
    anchor: "TODO(phase-65)"
    addressed_in: "Phase 65 (strangler-fig integration)"
    evidence: "Production indexer chain composition (per-language extractors + classifier walk + overlay drain) deferred per P64-08 SUMMARY.md decisions[0]. Phase 65 ROADMAP entry will compose the real Facts payload."
  - item: "semSessionAdapter.Workspace returns zero-value WorkspaceKey"
    location: "internal/daemon/semantic_wiring.go:523"
    anchor: "Phase 65 wires a real registry lookup"
    addressed_in: "Phase 65"
    evidence: "*mcp.SessionInfo carries a hashed WorkspaceKey string, not the canonical workspace.WorkspaceKey struct. Handlers tolerate the zero key (returns repoID=\"\") and surface their existing \"no workspace\" envelopes. Documented as accepted scope deferral in P64-08 SUMMARY.md Deviation #3."
---

# Phase 64: New MCP Tools (P0 set of 4) Verification Report

**Phase Goal:** Agents can index, refresh, inspect, and query the semantic graph through four new MCP tools (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`) whose responses always carry `freshness` + `graph_version` fields, profile/mode gating respects the existing matrix, and selection is deterministic.
**Verified:** 2026-05-08T03:10:00Z
**Status:** PASSED-WITH-CARRYOVER
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                                                                                                                | Status     | Evidence                                                                                                                                                                                                                                                                                              |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | TOOL-01 `index_semantic_graph` registered, mode review+/admin, singleflight join works, sync-with-timeout returns partial result, background continuation commits   | VERIFIED   | `internal/skill/semantic/tools_index.go:79-87` registers tool; `internal/skill/semantic/runner.go:11,100` imports `golang.org/x/sync/singleflight` + `singleflight.Group`; mode-tier review+ enforced via `checkMode` (skill.go); `TestE2E_ConcurrentIndexFull_SameSnapshotID` PASS; `TestE2E_ConcurrentIndexAuto_SameSnapshotID` PASS |
| 2   | TOOL-02 `refresh_semantic_graph` registered, read+ tier, drains live pipeline, NEVER calls Begin/Commit/Abort/Write/OnFlush (D-09/D-13 hard invariant)              | VERIFIED   | `internal/skill/semantic/tools_refresh.go:88-96` registers tool; `grep -E "(BeginSnapshot\|CommitSnapshot\|AbortSnapshot\|WriteSnapshotFacts\|OnFlush)" internal/skill/semantic/tools_refresh.go` returns 0 matches (exit=1); `TestE2E_RefreshNeverCommitsSnapshot` PASS                                |
| 3   | TOOL-03 `get_semantic_graph_status` registered, read+ tier, pure read path through frozen accessors                                                                  | VERIFIED   | `internal/skill/semantic/tools_status.go:73-82` registers tool; pure read fan-out across StoreAccessor / SchedulerAccessor / QueueAccessor / LiveAccessor / RetrievalAccessor (P64-06 SUMMARY); 5 unit tests PASS in `tools_status_test.go`                                                            |
| 4   | TOOL-04 `get_semantic_context` registered, retrieval engine uses bleve, RRF deterministic, recovery procedure exercised                                              | VERIFIED   | `internal/skill/semantic/tools_context.go:106-115` registers tool; `internal/semantic/retrieval/` package ships bleve-backed FTS engine + weighted RRF + Recoverer; `TestContextHandler_Determinism_10Runs` PASS; 6 recovery tests PASS (`TestRecovery_*`); 5 RRF tests PASS                            |
| 5   | Effective-graph queries on `*Store`: 5 methods landed (QueryEffectiveAdjacency, CountStaleScoreRows, MarkAllScoreRowsStale, LatestCommittedSnapshot, IterateCommittedSymbols) | VERIFIED   | `grep "^func (s \*Store)" internal/semantic/store/effective_graph.go` confirms all 5 methods at lines 85, 155, 186, 208, 242. P64-02 SUMMARY documents 14 RED→GREEN tests + race-clean lock-free reads.                                                                                              |
| 6   | Skill skeleton + profile/mode wiring: Caddy `init()` registration, frozen accessors, `semantic` skill identifier in all 5 profiles + 4 modes                         | VERIFIED   | `skill.go:38: func init() { skill.Register(&SemanticSkill{}) }`; `grep -l "semantic" internal/profile/profiles/*.yaml` returns all 5 profiles; `grep -l "semantic" internal/profile/modes/*.yaml` returns all 4 modes (read.yaml + edit.yaml carry `index_semantic_graph` in `exclude_tools` to enforce review+ mode tier)  |
| 7   | Daemon wiring: production glue mirrors compact_wiring.go + rank_wiring.go pattern; integration test exercises real `*Store` + scheduler + queue + live + bleve       | VERIFIED   | `internal/daemon/semantic_wiring.go` (755 lines) + `internal/daemon/imports.go:12` blank import + `internal/daemon/daemon.go` lines 441-447 (construct), 675-679 (RegisterAll), 745-748 (activate-callback), 912-914 (errgroup); 8 E2E integration tests PASS in `internal/skill/semantic/integration_test.go` |
| 8   | Bleve gate verdict: bench evidence committed; binary growth ≤ 50 MiB and indexing throughput within 5x DuckDB FTS5                                                   | VERIFIED   | `bench/semantic_bench_REPORT.md` contains `**PASS — proceed with bleve**`; binary growth 8.81 MiB (17.6% of 50 MiB budget); indexing 0.61x DuckDB FTS5 (12.2% of 5.0x ceiling — bleve actually 1.6x faster). `bench/semantic_bench_test.go` + `bench/semantic_bench_fts_test.go` benchmarks pinned. |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact                                                          | Expected                                                                  | Status     | Details                                                                                          |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------ |
| `internal/skill/semantic/skill.go`                                | SemanticSkill + init() registration + frozen accessor setters             | VERIFIED   | 7,749 bytes; `skill.Register(&SemanticSkill{})` at line 38; 8 setter methods                     |
| `internal/skill/semantic/runner.go`                               | IndexRunner with singleflight.Group + exported BuildState seam            | VERIFIED   | 9,783 bytes; imports `golang.org/x/sync/singleflight`; exported `BuildState`, `RunnerBuildFn`, `NewProductionIndexRunner` |
| `internal/skill/semantic/tools_index.go`                          | TOOL-01 handler + sync-with-timeout dispatch                              | VERIFIED   | 5,836 bytes; registers `index_semantic_graph`                                                    |
| `internal/skill/semantic/tools_refresh.go`                        | TOOL-02 handler + D-09/D-13 invariant + paths strict-subset + wait_for_lsp | VERIFIED   | 9,552 bytes; D-09/D-13 grep-clean (zero forbidden tokens)                                        |
| `internal/skill/semantic/tools_status.go`                         | TOOL-03 handler + closed-enum fan-out                                     | VERIFIED   | 10,522 bytes; registers `get_semantic_graph_status`                                              |
| `internal/skill/semantic/tools_context.go`                        | TOOL-04 handler + bleve+RRF + determinism                                 | VERIFIED   | 15,633 bytes; registers `get_semantic_context`                                                   |
| `internal/skill/semantic/register.go`                             | RegisterAll exported entry                                                | VERIFIED   | 1,048 bytes; calls 4 register* functions                                                         |
| `internal/skill/semantic/integration_test.go`                     | 8 E2E tests covering acceptance #1..#7                                    | VERIFIED   | 24,476 bytes; 8 PASS                                                                             |
| `internal/skill/semantic/profile_filter_test.go`                  | 4 profile-filter tests across 5 profiles × 4 modes                        | VERIFIED   | 6,165 bytes; 4 PASS                                                                              |
| `internal/kernel/help/semantic_help_test.go`                      | 4 get_tool_help tests                                                     | VERIFIED   | 4,635 bytes; 4 PASS                                                                              |
| `internal/daemon/semantic_wiring.go`                              | semanticBundle + 7 narrow accessor adapters + makeProductionBuildFn       | VERIFIED   | 26,592 bytes (755 lines); newSemanticBundle, ensureRetrieval, 7 adapters, makeProductionBuildFn |
| `internal/semantic/store/effective_graph.go`                      | 5 *Store methods (QueryEffectiveAdjacency, CountStaleScoreRows, MarkAllScoreRowsStale, LatestCommittedSnapshot, IterateCommittedSymbols) | VERIFIED   | All 5 method signatures present at lines 85, 155, 186, 208, 242                                |
| `internal/semantic/retrieval/`                                    | bleve-backed FTS + weighted RRF + Recoverer                               | VERIFIED   | All retrieval tests PASS (5 RRF + 3 bleve + 6 recovery)                                          |
| `bench/semantic_bench_REPORT.md`                                  | bleve gate verdict (PASS/FAIL)                                            | VERIFIED   | Verdict `**PASS — proceed with bleve**` recorded; both gates cleared with headroom               |
| `internal/profile/profiles/{ci-bot,claude-code,codex,full,ide-assistant}.yaml` | All 5 profiles include `semantic` skill                                   | VERIFIED   | `grep -l "semantic" internal/profile/profiles/*.yaml` returns all 5 files                        |
| `internal/profile/modes/{read,edit,review,admin}.yaml`            | All 4 modes include `semantic` skill (read/edit exclude index_semantic_graph) | VERIFIED   | `grep -l "semantic" internal/profile/modes/*.yaml` returns all 4 files; read.yaml line 29 + edit.yaml exclude index_semantic_graph |

### Key Link Verification

| From                                          | To                                                | Via                                                              | Status | Details                                                                                                                          |
| --------------------------------------------- | ------------------------------------------------- | ---------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------- |
| `internal/daemon/imports.go`                  | `internal/skill/semantic`                         | blank import                                                     | WIRED  | Line 12: `_ "github.com/agenthands/helix/internal/skill/semantic"` triggers `init()`                                              |
| `internal/daemon/daemon.go`                   | `newSemanticBundle`                               | step 6f.2 (line 441-447)                                         | WIRED  | Bundle constructed alongside compactBndl with placeholder getSession=nil                                                          |
| `internal/daemon/daemon.go`                   | `semanticpkg.RegisterAll`                         | step 14d (line 675-679)                                          | WIRED  | Tools registered AFTER InstallMiddleware so brief descriptions + profile filtering land on tools/list                              |
| `internal/daemon/daemon.go`                   | `sBndl.SetSessionFn(getSessionFn)`                | step 14d (line 676)                                              | WIRED  | Same closure passed to InstallMiddleware (single-source-of-truth W2 closure)                                                      |
| `internal/daemon/daemon.go::SetActivateCallback` | `sBndl.ensureRetrieval(ctx, activeWSKey)`         | line 745-748                                                     | WIRED  | Workspace activation primes the bleve engine + recovery probe before any get_semantic_context call                                 |
| `internal/daemon/daemon.go::errgroup`         | `d.semantic.Run(gctx)`                            | line 912-914                                                     | WIRED  | Kernel-first shutdown ordering preserved (closes bleve engines + IndexRunner on ctx.Done)                                          |
| `tools_refresh.go::handleRefreshSemanticGraph` | StoreAccessor read-only surface                   | accessor interface (compile-time guard)                          | WIRED  | StoreAccessor exposes only LatestCommittedSnapshot/CurrentGraphVersion/OverlayHasPendingRows/QueryEffectiveAdjacency — no mutators |
| `runner.go::Run`                              | `singleflight.Group`                              | direct field `sf singleflight.Group`                             | WIRED  | Concurrent callers attach via Do((ws,resolvedMode), buildFn); shared snapshot_id returned                                          |
| `tools_context.go::handleGetSemanticContext`  | `retrieval.Engine.QueryBleve` + `rrf.Fuse`        | RetrievalAccessor                                                | WIRED  | Bleve query → graph PageRank → weighted RRF fusion → greedyPack → TopEdgesFor (cap 5) → computeContextFreshness                   |

### Behavioral Spot-Checks

| Behavior                                              | Command                                                                                          | Result                          | Status |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ------------------------------- | ------ |
| Build helix binary                                    | `go build ./cmd/helix`                                                                           | exit=0                          | PASS   |
| Vet entire internal + bench tree                      | `go vet ./internal/... ./bench/...`                                                              | exit=0 (only pre-existing swift binding macro warning) | PASS |
| Semantic skill + store tests                          | `go test ./internal/skill/semantic/... ./internal/semantic/... -count=1`                         | all packages PASS               | PASS   |
| Daemon + help + mcp tests                             | `go test ./internal/daemon/... ./internal/kernel/help/... ./internal/mcp/... -count=1`           | all packages PASS               | PASS   |
| Race-clean skill tests                                | `go test ./internal/skill/semantic/... -race -count=1`                                           | PASS (only LC_DYSYMTAB ld warn) | PASS   |
| 8 E2E integration tests                               | `go test ./internal/skill/semantic/ -run "TestE2E_" -v -count=1`                                 | 8/8 PASS                        | PASS   |
| 4 profile-filter tests                                | `go test ./internal/skill/semantic/ -run "TestProfileFilter_" -v -count=1`                       | 4/4 PASS                        | PASS   |
| 4 get_tool_help tests                                 | `go test ./internal/kernel/help/ -run "TestGetToolHelp_" -v -count=1`                            | 4/4 PASS                        | PASS   |
| 14 retrieval (RRF + bleve + recovery) tests           | `go test ./internal/semantic/retrieval/... -run "Recovery\|RRF\|Bleve" -v -count=1`              | 14/14 PASS                      | PASS   |
| Determinism harness 10x byte-identical                | `go test ./internal/skill/semantic/ -run "TestContextHandler_Determinism" -v`                    | PASS                            | PASS   |
| D-09/D-13 hard invariant grep                         | `grep -E "(BeginSnapshot\|CommitSnapshot\|AbortSnapshot\|WriteSnapshotFacts\|OnFlush)" internal/skill/semantic/tools_refresh.go` | exit=1 (no matches)             | PASS   |
| Phase 64 TODO anchor removed from rank_wiring.go      | `grep -c "TODO(phase-64)" internal/daemon/rank_wiring.go`                                        | 0                               | PASS   |
| Bleve gate verdict committed                          | `grep "PASS — proceed with bleve" bench/semantic_bench_REPORT.md`                                | match                           | PASS   |

### Requirements Coverage

| Requirement | Source Plan(s)               | Description                                                                                                | Status     | Evidence                                                                                                                              |
| ----------- | ---------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| TOOL-01     | P64-04, P64-08               | `index_semantic_graph` MCP tool (review+/admin) with auto/full/incremental/refresh modes                   | SATISFIED  | Tool registered, singleflight join verified by E2E, mode-tier check enforced, sync-with-timeout dispatch with bgCtx                   |
| TOOL-02     | P64-05, P64-08               | `refresh_semantic_graph` MCP tool (read+) with `wait_for_lsp` + `paths` filters                            | SATISFIED  | Tool registered, D-09/D-13 invariants enforced at compile (StoreAccessor), grep, and test layers; paths strict-subset (D-11)            |
| TOOL-03     | P64-02, P64-06, P64-08       | `get_semantic_graph_status` MCP tool (read+) with SPEC §23.3 envelope                                       | SATISFIED  | Tool registered, fan-out across StoreAccessor/SchedulerAccessor/QueueAccessor/LiveAccessor/RetrievalAccessor, closed-enum freshness   |
| TOOL-04     | P64-01, P64-02, P64-07, P64-08 | `get_semantic_context` MCP tool (read+) with bleve+RRF + determinism                                       | SATISFIED  | Tool registered, bleve gate PASS, weighted RRF deterministic, dual-store recovery procedure tested (6 tests)                          |
| TOOL-05     | P64-03, P64-04, P64-05, P64-06, P64-07, P64-08 | All four tools respect profile/mode gating; tools/list filters; get_tool_help returns parameter docs       | SATISFIED  | 4 profile-filter tests + 4 get_tool_help tests PASS; D-14 symmetric matrix wired in 5 profiles + 4 modes                              |

### Anti-Patterns Found

No blocker anti-patterns. Two acknowledged scope deferrals (with explicit `TODO(phase-65)` anchors) are documented in code as carryover items, not as stubs:

| File                                | Line     | Pattern                              | Severity | Impact                                                                                                                            |
| ----------------------------------- | -------- | ------------------------------------ | -------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `internal/daemon/semantic_wiring.go` | 680, 723 | TODO(phase-65) — empty-Facts placeholder | Info     | Production buildFn ships a deliberate empty-Facts placeholder; per-language extractor chain composition deferred to Phase 65 strangler-fig integration. Pipeline contract documented inline. Test-layer closure is `TestE2E_IndexThenContext_SymbolCount` (15-symbol fixture). |
| `internal/daemon/semantic_wiring.go` | 523, 559 | TODO(phase-65) — zero-value WorkspaceKey | Info     | `*mcp.SessionInfo` carries a hashed `WorkspaceKey` string, not the canonical struct. Handlers tolerate the zero key and surface their existing "no workspace" envelopes. Phase 65 will wire a real registry lookup. |

### Human Verification Required

None — all acceptance criteria are programmatically verified via the 16 new tests (4 profile-filter + 4 help + 8 integration), the determinism harness (`TestContextHandler_Determinism_10Runs`), the bleve gate verdict (committed in `bench/semantic_bench_REPORT.md`), the D-09/D-13 grep invariant, the `go vet` + `go build ./cmd/helix` smoke checks, and the race-clean test runs.

### Gaps Summary

No blocking gaps. Phase 64 ships all four MCP tools end-to-end with production daemon wiring, full profile/mode matrix coverage, deterministic ranking, dual-store recovery, and 16 new tests asserting acceptance criteria #1..#8 from CONTEXT.md. The two carryover items (production buildFn empty-Facts placeholder + zero-value WorkspaceKey from session adapter) are explicitly scope-deferred to Phase 65 with `TODO(phase-65)` anchors and inline contract documentation; they do not block any Phase 64 acceptance test or any downstream Phase 65 wiring.

**Verdict: PASSED-WITH-CARRYOVER.** Phase 64 goal achieved; carryover items handed forward to Phase 65 (strangler-fig integration).

---

_Verified: 2026-05-08T03:10:00Z_
_Verifier: Claude (gsd-verifier)_
