---
phase: 69
plan: 06
subsystem: semantic-graph/status-accessors
tags: [semantic-graph, status, e2e, integration, test]
requires:
  - "69-01: *Store.ClusterStatusForGraphVersion + ClusterStatusRow"
  - "69-02: retrieval.MetaKey* exported constants + *Engine.DocCount"
  - "69-03: compactBundle BleveMeta wiring (last_compact_at stamping)"
  - "69-04: RetrievalStatus envelope field + RetrievalAccessor.RetrievalStatus interface method"
  - "69-05: daemon.NewSchedulerAccessorForStore + daemon.NewRetrievalAccessorForStore factory constructors"
provides:
  - "TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval — STATUS-03 closure"
  - "Production status handler now stamps RetrievalStatus into the envelope (was silently zero-valued post-69-04)"
  - "Test-only HandleGetSemanticGraphStatusForTest / HandleIndexSemanticGraphForTest exports (export_status_test.go)"
affects:
  - "internal/skill/semantic/tools_status.go"
  - "internal/skill/semantic/export_status_test.go (NEW, test-only)"
  - "internal/skill/semantic/status_e2e_external_test.go (NEW, package semantic_test)"
tech-stack:
  patterns:
    - "Black-box test package (`package semantic_test`) to break daemon→semantic import cycle"
    - "export_test.go pattern (uppercase shim in package semantic exposes unexported handler to external test)"
    - "Inlined harness (mirrors integration_test.go::newE2EHarness but standalone since the in-package harness is unexported)"
    - "BumpGraphVersion → UpsertClusters → UpsertClusterMembers seeding pattern"
    - "Recoverer.Probe + poll-RetrievalPending pattern for deterministic bleve corpus priming via production rebuild path"
key-files:
  created:
    - "internal/skill/semantic/export_status_test.go"
    - "internal/skill/semantic/status_e2e_external_test.go"
  modified:
    - "internal/skill/semantic/tools_status.go"
decisions:
  - "Lived in `package semantic_test` (new file `status_e2e_external_test.go`), not the existing in-package `integration_test.go` — the existing file is `package semantic` and adding an `internal/daemon` import there would create a Go-level cycle (daemon already imports `internal/skill/semantic`). The `package semantic_test` pattern was already established by `production_adapter_e2e_test.go` (Phase 65 65-12 Task 4)."
  - "Used `Recoverer.Probe` (production async path) + poll-until-not-pending, NOT direct `engine.SetMeta(MetaKeyCorpusVersion, ...)`. The production rebuildBlocking is the only writer that stamps MetaKeyCorpusVersion + MetaKeyIndexedFiles in lockstep; the test exercises that exact code path. No `primeBleveCorpus` extension was needed."
  - "Wired `RetrievalStatus` field into the `get_semantic_graph_status` handler envelope (deviation Rule 2 — auto-add missing critical functionality). Plan 69-04 added the `RetrievalStatus` field to `StatusResult` and the `RetrievalAccessor.RetrievalStatus(ws)` interface method, and Plan 69-05 added the daemon adapter delegation, but no plan in the wave wired the handler to actually populate the field. Without this wiring the test could not meaningfully assert non-placeholder values."
  - "Inlined a minimal harness instead of exporting `newE2EHarness` from `package semantic`. The in-package harness has 12+ unexported helper types; exposing them all just for one external test would balloon the API surface. Inlining is ~150 lines and self-contained."
metrics:
  duration: "~30 min"
  completed: "2026-05-14"
---

# Phase 69 Plan 06: STATUS-03 E2E — Cluster + Retrieval Status Round-Trip Summary

Closes STATUS-03 and Phase 69 ROADMAP success criterion #4: a single integration test exercises the full status surface on a populated workspace and asserts non-placeholder values across both `cluster_status` and `retrieval_status`, going through the SAME production factory code path the daemon's `semSchedulerAdapter` / `semRetrievalAdapter` invoke at runtime.

Plan 69-04 added the envelope field. Plan 69-05 added the factory constructors and wired the daemon adapter to delegate. Plan 69-06 (this plan) (1) wires the handler to actually stamp `RetrievalStatus` into the envelope, (2) authors the E2E test that swaps placeholder adapters for `daemon.NewSchedulerAccessorForStore` / `daemon.NewRetrievalAccessorForStore` and asserts the values cross the wire intact.

## Assertions (TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval)

ClusterStatus (production factory: ClusterStatusForGraphVersion → ClusterStatusRow → semantic.ClusterStatus):
- `ClusterStatus.State == "current"` (factory reads `ClusterCount > 0 && IsCurrent`)
- `ClusterStatus.MemberCount == 5` (sum of seeded member rows: 3 + 2)
- `ClusterStatus.ComputedAt > 0` (UpsertClusters stamps `now()` into the `computed_at` column)

RetrievalStatus (production factory: GetMeta(MetaKeyCorpusVersion/MetaKeyIndexedFiles/MetaKeyLastCompactAt) + engine.DocCount):
- `RetrievalStatus.CorpusVersion == graph_version` (Recoverer.rebuildBlocking stamps `MetaKeyCorpusVersion` to the same gv the test seeded)
- `RetrievalStatus.IndexedSymbols > 0` (engine.DocCount > 0 after rebuildBlocking upserted 15 SymbolDocs)
- `RetrievalStatus.IndexedFiles >= 1` (Recoverer.rebuildBlocking stamps `MetaKeyIndexedFiles` from the distinct FileID set walked across the snapshot)
- `RetrievalStatus.Reason ∈ {"", "compactor-never-ran"}` (no compactor runs in fixture; higher-priority degradation reasons MUST not trigger)

Regression guard: `TestE2E_IndexThenContext_SymbolCount` continues to PASS (the existing fixture was not modified — Plan 69-06 only added a new external file and one handler-wiring line).

## Deviations from Plan

### Auto-fixed (Rule 2 — auto-add missing critical functionality)

**1. [Rule 2] Wire RetrievalStatus into the get_semantic_graph_status envelope**
- **Found during:** Task 1 (test authoring) — the new test asserts `sr.RetrievalStatus.CorpusVersion == gv`; reading `tools_status.go` showed the handler never invoked `s.retrieval.RetrievalStatus(ws)` and never wrote to `StatusResult.RetrievalStatus`. The field was silently zero-valued in production.
- **Issue:** Plan 69-04 added the envelope field + interface method, Plan 69-05 added the daemon factory + adapter delegation, but no plan in the wave wired the handler to actually populate the field. Without this fix Plan 69-06's test could not meaningfully assert non-placeholder values — and worse, the production daemon would also never have surfaced the corpus state to MCP clients.
- **Fix:** Added 4 lines to `handleGetSemanticGraphStatus` to call `s.retrieval.RetrievalStatus(ws)` (guarded by the same `s.retrieval != nil` nil-check that already gated `RetrievalPending`) and stamped the result into the response struct.
- **Files modified:** `internal/skill/semantic/tools_status.go`
- **Commit:** included in `test(69-06): …` (single commit for atomicity since the test cannot pass without the wiring).

## Implementation Notes

### Package-cycle resolution

The new test imports `internal/daemon` for the factory constructors. The existing `integration_test.go` is `package semantic` (in-package test), and `internal/daemon` itself imports `internal/skill/semantic` — adding the test to `package semantic` would create a Go-level import cycle.

The fix follows the same pattern Phase 65 65-12 Task 4 established for `production_adapter_e2e_test.go`: place the new test in a separate file (`status_e2e_external_test.go`) declared `package semantic_test` (black-box test package). Black-box test packages can co-exist with internal `*_test.go` files in the same directory; the cycle does not apply because `package semantic_test` is a distinct package even though it lives next to `package semantic` sources.

The handler entry point (`handleGetSemanticGraphStatus`) is unexported. Rather than promoting it to a public API surface, an `export_status_test.go` shim file (in `package semantic`, suffix `_test.go` so it compiles only for tests) declares:

```go
func HandleGetSemanticGraphStatusForTest(s *SemanticSkill, ctx context.Context) *mcpsdk.CallToolResult { … }
func HandleIndexSemanticGraphForTest(s *SemanticSkill, ctx context.Context, mode string) *mcpsdk.CallToolResult { … }
```

This is the canonical Go `export_test.go` pattern: the names are visible to `package semantic_test` but never leak into the production API.

### Bleve priming via Recoverer.Probe (NOT direct SetMeta)

The plan asked: "Prime bleve: ensure bleve docs exist AND `MetaKeyCorpusVersion` + `MetaKeyIndexedFiles` are populated. Prefer calling the real `recoverer.RebuildBlocking(...)` path".

`rebuildBlocking` is unexported, so the test drives it through `Recoverer.Probe` (the public single-writer entrypoint) and polls `RetrievalPending(ws)` until the background rebuild finishes. This exercises the EXACT production rebuild code path — including the meta-stamping at the commit point of recovery.go:310-328 — so the test would catch any future regression in either the rebuild logic or the meta-stamping. No extension to the existing `primeBleveCorpus` helper was needed (it lives in package `daemon` and is unexported anyway).

### Graph-version bump

`BeginOverlayTx` only advances `current_epoch`, not `graph_version`. Production advances `graph_version` only through `Engine.ApplyRepair` → `OverlayTx.BumpGraphVersion` (D-06 invariant). The test stands in for that path by calling `BumpGraphVersion` directly on the tx so the seeded cluster rows land at a non-zero graph_version (required for `ClusterStatusRow.IsCurrent` to be `true` against the same `CurrentGraphVersion` read by the factory).

### Why MemberCount == 5 and not something else

The factory reads `MemberCount` from `ClusterStatusForGraphVersion`, which aggregates `semantic_cluster_members` rows for a given (repo_id, graph_version). The test inserts 5 member rows (cluster 1 with 3 members; cluster 2 with 2 members) so the expected aggregate is exactly 5.

## Files Touched

| File | Status | Lines | Purpose |
|------|--------|-------|---------|
| `internal/skill/semantic/tools_status.go` | modified | +6 | Wire RetrievalStatus into handler envelope (Rule 2) |
| `internal/skill/semantic/export_status_test.go` | NEW (test-only) | 35 | Test export shim for unexported handlers |
| `internal/skill/semantic/status_e2e_external_test.go` | NEW (test-only) | ~390 | TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval + harness |

## Verification

- `go vet ./...` — PASS (only unrelated swift binding warning, pre-existing)
- `go build ./...` — PASS
- `go test ./internal/skill/semantic/ -run "TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval|TestE2E_IndexThenContext_SymbolCount" -count=1 -race -timeout 120s` — PASS (~2.7s)
- `go test ./internal/skill/semantic/... ./internal/daemon/... -race -count=1 -timeout 300s` — PASS
- `go test ./... -count=1 -timeout 600s` — PASS repo-wide
- `gofmt -l internal/skill/semantic/status_e2e_external_test.go internal/skill/semantic/export_status_test.go internal/skill/semantic/tools_status.go` — empty (clean)
- `grep -rn 'phase-62-clustering-no-status-accessor' internal/` — zero (placeholder still swept, 69-05 closure preserved)
- `grep -rn 'daemon.NewSchedulerAccessorForStore\|daemon.NewRetrievalAccessorForStore' internal/skill/semantic/` — new file call sites surface (proves factory usage, not re-implementation)

## Success Criteria

- [x] STATUS-03 closed: single integration test exercises the full status surface on a populated workspace and asserts non-placeholder values across both cluster_status and retrieval_status
- [x] Test instantiates `daemon.NewSchedulerAccessorForStore` and `daemon.NewRetrievalAccessorForStore` directly — exercises the SAME factory the daemon adapter uses (revision Warning 3 closed)
- [x] Phase 64 regression test (`TestE2E_IndexThenContext_SymbolCount`) still PASSES

## Self-Check: PASSED

- File `internal/skill/semantic/tools_status.go` exists (modified): FOUND
- File `internal/skill/semantic/export_status_test.go` exists (new): FOUND
- File `internal/skill/semantic/status_e2e_external_test.go` exists (new): FOUND
- Commit hash recorded below in commit message
