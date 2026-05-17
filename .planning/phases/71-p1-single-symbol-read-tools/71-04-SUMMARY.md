---
phase: 71-p1-single-symbol-read-tools
plan: 04
subsystem: skill-semantic
tags:
  - semantic
  - mcp-tool
  - find-related-symbols
  - retrieval-fusion
  - read-only
  - tdd
  - phase-71
  - wave-2
dependency-graph:
  requires:
    - internal/skill/semantic.SeedInput / resolveSeed (Phase 71-01)
    - internal/skill/semantic.SymbolByNameAccessor / ClusterMembershipAccessor (Phase 71-01)
    - internal/skill/semantic.buildPopulatedGraphFixture (Phase 71-01)
    - internal/skill/semantic.FreshnessV2 + FreshnessStatus + FreshnessSource (Phase 71-02)
    - internal/skill/semantic.SemanticSkill.assembleFreshness (Phase 71-03)
    - internal/semantic/retrieval.Fuse + DefaultRRFConfig (Phase 62 / 64-07)
    - RetrievalAccessor.PersonalizedPageRank + QueryBleve
  provides:
    - "find_related_symbols MCP tool registered through RegisterAll"
    - "FindRelatedSymbolsArgs / FindRelatedSymbolsResult / RelatedResult response shapes"
    - "pathFromSymbolID + pathsAllow package-private helpers (consumed by future tools that need strict-subset filtering over results)"
  affects:
    - .planning/phases/71-p1-single-symbol-read-tools/71-05-PLAN.md (validate_graph_edge — cross-tool integration test can now reference both 71-03 and 71-04 tools live)
tech-stack:
  added: []
  patterns:
    - "Closed-enum gated handler (Pattern from tools_context.go / tools_explain_symbol.go)"
    - "RRF fusion via retrieval.Fuse with single-symbol anchor list (mirrors tools_context.go:226-264)"
    - "Single-goroutine post-fuse cluster co-membership boost (Pitfall 6)"
    - "graph_version captured BEFORE PageRank, reused by cluster lookup (Pitfall 3)"
    - "Recorder StoreAccessor canary for D-09 / D-13 (mirrors tools_explain_symbol_test.go)"
    - "Concurrent invocation idempotency under -race (Pitfall 6 regression guard)"
key-files:
  created:
    - path: internal/skill/semantic/tools_find_related.go
      role: "handler + register + types + help const + 2 package-private helpers"
    - path: internal/skill/semantic/tools_find_related_test.go
      role: "12 behavior tests + recorder canaries + retrieval/cluster accessor fakes"
  modified:
    - path: internal/skill/semantic/register.go
      role: "added registerFindRelatedSymbols(server, s, tracer) call"
decisions:
  - "assembleFreshness was NOT re-extracted into a shared helpers file because 71-03 already placed it on *SemanticSkill (handler-method). Both 71-03 and 71-04 call s.assembleFreshness directly — zero duplication. The plan's REFACTOR step is therefore a no-op (documented under Deviations)."
  - "Cluster boost multiplier is a fixed constant (1.25), not a config knob, per 71-CONTEXT.md decision to ship RRF-style weight-free ranking in v1.11 (no tuning surface)."
  - "pathFromSymbolID extracts the defining file from the stable SymbolID `<path>::<name>` convention. When the convention does not hold (no `::` separator), pathsAllow refuses the candidate rather than leak — strict-subset is fail-closed (T-71-04-01 mitigation)."
  - "Per-candidate ClusterIDOf errors are tolerated without poisoning the whole pass — that candidate is left un-boosted but other candidates can still receive the boost. Only the seed's ClusterIDOf error triggers fallback_reason=cluster_boost_unavailable (the boost cannot be computed without a reference cluster)."
metrics:
  duration: "~35 minutes execution"
  completed: "2026-05-17"
  tasks_completed: 1
  files_created: 2
  files_modified: 1
  commits: 2
---

# Phase 71 Plan 04: `find_related_symbols` MCP Tool Summary

Ship the second of three Phase 71 P1 single-symbol read tools. Given one seed symbol, return up to `k` ranked sibling symbols via `PersonalizedPageRank(seed)` + RRF fusion (`retrieval.Fuse`) + single-goroutine post-fuse cluster co-membership boost. Honors `paths` as a strict-subset filter on results. Read-only, mode-gated, and instrumented with D-09 / D-13 canaries + a concurrent-invocation idempotency guard.

## What Landed

| # | Commit  | Type | Description                                                  |
| - | ------- | ---- | ------------------------------------------------------------ |
| 1 | 39f10c4d | test | RED — 12 failing behavior tests for find_related_symbols     |
| 2 | 74641495 | feat | GREEN — handler + register + 2 helpers; full suite stays green |

## Behavior Tests (all green under `-race -count=1`)

| Test                                                  | Asserts                                                                                                                                                |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `TestFindRelatedSymbols_RankingDefaultK`              | 25 neighbors → results ordered by Score descending, length ≤ 20 (default k); FreshnessV2 has non-zero graph_version.                                   |
| `TestFindRelatedSymbols_KClamp` (4 sub-cases)         | k=0→20, k=500→100, k=1→1, k=50→50.                                                                                                                     |
| `TestFindRelatedSymbols_PathsStrictSubset_ResultsFiltered` | `paths=["pkg/sibling/"]` filters results to those under `pkg/sibling/`; out-of-prefix neighbors are removed.                                          |
| `TestFindRelatedSymbols_PathsRejectsTraversal`        | `paths=["../../../etc"]` → error envelope via validatePaths.                                                                                          |
| `TestFindRelatedSymbols_EmptyResult`                  | Isolated seed (no PageRank reachables) → `{results: [], total_count: 0}` — NOT an error.                                                              |
| `TestFindRelatedSymbols_ClusterBoostApplied`          | A in seed's cluster, B not → A ranks above B post-boost; `ClusterBoostApplied=true` surfaces on A.                                                    |
| `TestFindRelatedSymbols_ClusterBoostSameGraphVersion` | Pitfall 3 regression: `CurrentGraphVersion` is invoked, cluster reads observe `gv=42` (the PageRank-time graph_version).                              |
| `TestFindRelatedSymbols_ClusterBoostUnavailable`      | Seed cluster lookup errors → ranked results still returned, no `ClusterBoostApplied` flag, envelope's `fallback_reason=cluster_boost_unavailable`. |
| `TestFindRelatedSymbols_ReadOnly`                     | Recorder StoreAccessor canaries (Begin/Commit/Abort/Write) never invoked across full happy-path traversal.                                            |
| `TestFindRelatedSymbols_ModeRejected`                 | Resolver-unwired path produces error envelope; no PageRank invocation, no canary fires.                                                                |
| `TestFindRelatedSymbols_SeedNotFound`                 | `(file, name)` tuple that doesn't resolve → `resolution=not_found` + `fallback_reason=symbol_not_found`; PageRank never invoked.                       |
| `TestFindRelatedSymbols_ConcurrentSameSeed`           | Pitfall 6 regression: 16 goroutines with identical input produce byte-identical responses (idempotent); race detector clean.                          |

## Deviations from Plan

### Skipped REFACTOR step (no-op extraction)

The plan's REFACTOR step proposed extracting `assembleFreshness` from 71-03's `tools_explain_symbol.go` into a shared `freshness_helpers.go` file. Inspection during GREEN revealed that 71-03 already placed `assembleFreshness` on `*SemanticSkill` as a method on the receiver — both 71-03's `handleExplainSymbolDeep` and 71-04's `handleFindRelatedSymbols` call `s.assembleFreshness(ctx, repoID)` directly. There is zero duplication, so extracting the method into a separate file would be a pure file rename without behavior change.

This matches the same call recorded in 71-03's SUMMARY: "shapeEdges helper extracted in GREEN (not a separate REFACTOR commit)". 71-04 follows the same minimal-refactor doctrine.

### Pragmatic `TestFindRelatedSymbols_ModeRejected` (carried over from 71-03)

`checkMode(snap, modeTierRead)` returns `nil` for all sessions by design — read+ tier is the lowest tier; every session passes. True mode rejection is impossible for this tool. The test asserts the *spirit* of the plan's "no accessor calls on reject" invariant by driving an alternate rejection path (unwired `SymbolByName` accessor) and verifying neither the recorder StoreAccessor canaries nor `PersonalizedPageRank` are invoked. Identical handling as 71-03.

No Rule 1/2/3 fixes required during execution — the seam (71-01) and shared helper (71-03's `assembleFreshness`) gave the handler everything it needed.

## Threat Model — Mitigations Observed

| Threat ID    | Mitigation in code                                                                                                                                                                            |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T-71-04-01   | `validatePaths` rejects `..` and out-of-root absolute paths; `pathsAllow` is fail-closed (returns false when defining path cannot be determined).                                            |
| T-71-04-02   | k clamped to [1, 100]; `findRelatedDefaultK = 20`, `findRelatedMinK = 1`, `findRelatedMaxK = 100`.                                                                                            |
| T-71-04-03   | Boost is a deterministic additive multiplier on candidates already returned by PageRank — no new visibility surface beyond what PageRank already exposes.                                     |
| T-71-04-04   | Pitfall 3: `CurrentGraphVersion` is captured BEFORE the fan-out to `PersonalizedPageRank` (step 6). The same captured `gv` pins the cluster lookup. Verified by `TestFindRelatedSymbols_ClusterBoostSameGraphVersion`. |
| T-71-04-05   | Pitfall 6: cluster boost is a single-goroutine pass over `fused`. Verified by `TestFindRelatedSymbols_ConcurrentSameSeed` under `-race`.                                                     |
| T-71-04-06   | `checkMode(snap, modeTierRead)` at handler entry; canary tests confirm forbidden write-side methods are never invoked on either success or error paths.                                       |

## must_haves Observance

- ✅ `find_related_symbols` is registered through `RegisterAll` (`grep -c "registerFindRelatedSymbols(server, s, tracer)" internal/skill/semantic/register.go` → 1)
- ✅ Handler enforces read+ mode tier at entry via `checkMode(snap, modeTierRead)`
- ✅ Ranking pipeline = `PersonalizedPageRank(seed)` + RRF fusion via `retrieval.Fuse` + post-fuse cluster co-membership boost
- ✅ Default k=20, clamped to [1, 100]
- ✅ `paths` filter is strict-subset on RESULTS; the seed itself is NOT constrained (D2 + Phase 70 D2). Seed is not in `fused` because PageRank excludes the anchor from its own neighbor set.
- ✅ Empty result (isolated seed) returns empty list, not an error
- ✅ Cluster boost reads against the SAME `graph_version` that PageRank ran on (Pitfall 3 — verified by `TestFindRelatedSymbols_ClusterBoostSameGraphVersion`)
- ✅ Handler is read-only — `grep -E "BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts" internal/skill/semantic/tools_find_related.go` returns no matches

## Verification

```
go vet ./internal/skill/semantic/                                              # clean (only unrelated Swift binding warning)
go test -race -count=1 ./internal/skill/semantic/                              # ok 6.093s
go test -run TestFindRelatedSymbols ./internal/skill/semantic/ -race -count=1  # 12/12 pass
grep -c "func (s \*SemanticSkill) handleFindRelatedSymbols" internal/skill/semantic/tools_find_related.go  # 1
grep -c "func registerFindRelatedSymbols" internal/skill/semantic/tools_find_related.go                    # 1
grep -c "registerFindRelatedSymbols(server, s, tracer)" internal/skill/semantic/register.go                # 1
grep -E "BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts" internal/skill/semantic/tools_find_related.go  # no match
grep -c "retrieval.Fuse" internal/skill/semantic/tools_find_related.go        # 5 (≥ 1)
grep -c "PersonalizedPageRank" internal/skill/semantic/tools_find_related.go  # 4 (≥ 1)
grep -c "validatePaths" internal/skill/semantic/tools_find_related.go         # 2 (≥ 1)
```

## Known Stubs

**Production wiring of `RetrievalAccessor` + `ClusterMembershipAccessor` for `find_related_symbols`:** the seams already exist (Phase 64-07 wired `RetrievalAccessor`; 71-01 declared `ClusterMembershipAccessor`). Until the production daemon adapter wraps `ClusterMembershipAccessor` against the real cluster engine, live calls to `find_related_symbols` will degrade gracefully via `fallback_reason=cluster_boost_unavailable` and still return ranked PageRank+RRF results.

This is **intentional** per the seam-first wave-2 design and is consistent with the 71-03 SUMMARY's "production wiring deferred" note. The wiring plan (likely absorbed into 71-05 or a follow-up) will activate the boost without further handler changes.

## Threat Flags

None new — `find_related_symbols` reads exclusively through the existing read-only narrow accessors. No new endpoints, schema changes, or auth paths. The threat surface is identical to `get_semantic_context` (anchor-driven ranking with bounded result size).

## Consumer Hooks

- **71-05 validate_graph_edge** can now reference both `explain_symbol_deep` (71-03) and `find_related_symbols` (71-04) as live tools in the cross-tool integration test. The recorder StoreAccessor canary pattern is established by both 71-03 and 71-04; 71-05 should reuse the same `recorderStoreAccessorFor<Tool>` shape with a fresh `t.Fatal` message.
- The CI grep gate planned for 71-05 will cover `tools_find_related.go` automatically (file name pattern matches the `tools_*.go` read-tool glob).
- `pathFromSymbolID` + `pathsAllow` package-private helpers are available for any future tool that needs strict-subset filtering over `<path>::<name>` stable IDs (notably `validate_graph_edge` if it grows a paths filter).

## TDD Gate Compliance

- ✅ `test(71-04)` commit `39f10c4d` precedes feat (RED gate)
- ✅ `feat(71-04)` commit `74641495` follows test (GREEN gate)
- N/A separate `refactor(71-04)` commit — REFACTOR step was a documented no-op (see Deviations)

## Self-Check: PASSED

- ✅ `internal/skill/semantic/tools_find_related.go` exists (handler + register + types + help const + INVARIANT header + 2 helpers)
- ✅ `internal/skill/semantic/tools_find_related_test.go` exists (12 behavior tests + recorder canaries + retrieval/cluster fakes)
- ✅ `internal/skill/semantic/register.go` carries `registerFindRelatedSymbols(server, s, tracer)`
- ✅ Commits `39f10c4d` (test) + `74641495` (feat) present on `worktree-agent-a1f52559602ae5aa8`
- ✅ `go vet ./internal/skill/semantic/` clean
- ✅ `go test -race -count=1 ./internal/skill/semantic/` green (6.093s)
- ✅ 12/12 `TestFindRelatedSymbols_*` cases pass under `-race -count=1`
