---
phase: 69
plan: 05
subsystem: semantic-graph/status-accessors
tags: [adapter-wiring, factory-extraction, placeholder-removal, status-accessor]
requires:
  - "69-01: *Store.ClusterStatusForGraphVersion + ClusterStatusRow (IsCurrent / ActualGraphVersion)"
  - "69-02: retrieval.MetaKey* exported constants + *Engine.DocCount"
  - "69-03: compactBundle.SetBleveMetaFn setter + bleveMetaFn field"
  - "69-04: semantic.RetrievalAccessor.RetrievalStatus interface method + RetrievalStatus envelope fields"
provides:
  - "internal/daemon.NewSchedulerAccessorForStore — exported factory for cluster-status derivation"
  - "internal/daemon.NewRetrievalAccessorForStore — exported factory for retrieval-status derivation"
  - "semSchedulerAdapter.ClusterStatus — delegates to factory"
  - "semRetrievalAdapter.RetrievalStatus — delegates to factory"
  - "compactBundle.bleveMetaFn — closure resolving semanticBundle.engines[ws.RepoRoot]"
affects:
  - "internal/daemon/semantic_wiring.go"
  - "internal/daemon/semantic_accessor_factories.go (NEW)"
  - "internal/daemon/daemon.go"
  - "internal/skill/semantic/tools_status.go"
  - "internal/skill/semantic/accessors.go"
  - "internal/skill/semantic/tools_status_test.go"
  - "internal/skill/semantic/envelope_test.go"
  - "internal/skill/semantic/integration_test.go"
tech-stack:
  patterns:
    - "Exported factory constructor (single source-of-truth shared by daemon adapter + future E2E test)"
    - "Closure-based post-init wiring (SetBleveMetaFn binding pattern, mirrors SetEnrichFn)"
    - "Closed-enum reason priority order (bleve-unavailable > corpus_version-uninitialized > corpus_version-lag > compactor-never-ran)"
key-files:
  created:
    - "internal/daemon/semantic_accessor_factories.go"
  modified:
    - "internal/daemon/semantic_wiring.go"
    - "internal/daemon/daemon.go"
    - "internal/skill/semantic/tools_status.go"
    - "internal/skill/semantic/accessors.go"
    - "internal/skill/semantic/tools_status_test.go"
    - "internal/skill/semantic/envelope_test.go"
    - "internal/skill/semantic/integration_test.go"
decisions:
  - "Sweep the W1 sentinel string `phase-62-clustering-no-status-accessor` across internal/ rather than only the daemon adapter — the in-package `productionDefaultClusterStatus()` helper now mirrors the factory's nil-store branch (`no-store`) so tests track real production output."
  - "Test-path fallback in tools_status.go uses `no-scheduler` rather than reusing one of the factory's closed-enum reasons — the scheduler-missing condition is observably distinct from store-missing."
metrics:
  duration: "~25 min"
  completed: "2026-05-14"
---

# Phase 69 Plan 05: Production Status Accessors — Adapter Wiring Summary

Plan 69-05 closes both Phase 64 W1 production placeholders (`ClusterStatus` returning the `phase-62-clustering-no-status-accessor` sentinel; `RetrievalStatus` interface method not implemented) by extracting the derivation logic into two exported factory constructors that the daemon's adapters AND the upcoming Plan 69-06 E2E test will share. It also binds the `compactBundle.bleveMetaFn` closure left as a wiring slot by Plan 69-03, closing the last open thread between the compactor and the bleve corpus-state meta surface.

## Final factory file layout (`internal/daemon/semantic_accessor_factories.go`)

```
package daemon

type schedulerAccessorImpl struct { store *semanticstore.Store }
  IsQuiescent   — returns true (status surface; rank-bundle-backed IsQuiescent stays on semSchedulerAdapter)
  ScoreStatus   — returns graph.ScoreStatusMissing (RESEARCH Q9 pending Phase 65/67)
  ClusterStatus — full CONTEXT D1 derivation (see reason ladder below)

type retrievalAccessorImpl struct { store *semanticstore.Store; engine *retrieval.Engine }
  QueryBleve / PersonalizedPageRank / RetrievalPending / TopEdgesFor — zero-value stubs (status-only carve-out)
  RetrievalStatus — full priority-order derivation (see reason ladder below)

NewSchedulerAccessorForStore(store) semantic.SchedulerAccessor
NewRetrievalAccessorForStore(store, eng) semantic.RetrievalAccessor

Compile-time guards:
  _ semantic.SchedulerAccessor = (*schedulerAccessorImpl)(nil)
  _ semantic.RetrievalAccessor = (*retrievalAccessorImpl)(nil)
  _ compact.BleveMeta          = (*retrieval.Engine)(nil)
```

## ClusterStatus reason ladder (factory output, CONTEXT D1)

| Condition                                                          | State     | Reason                  | ComputedAt | MemberCount |
| ------------------------------------------------------------------ | --------- | ----------------------- | :--------: | :---------: |
| `store == nil`                                                     | `unknown` | `no-store`              |    `0`     |    `0`      |
| `CurrentGraphVersion err OR gv == 0`                               | `unknown` | `no-graph-version`      |    `0`     |    `0`      |
| `ClusterStatusForGraphVersion err`                                 | `unknown` | `accessor-error`        |    `0`     |    `0`      |
| `row.ClusterCount == 0`                                            | `unknown` | `no-cluster-rows`       |    `0`     |    `0`      |
| `row.ClusterCount > 0 && row.IsCurrent`                            | `current` | _(omitempty)_           | `row.CA`   | `row.MC`    |
| `row.ClusterCount > 0 && !row.IsCurrent`                           | `stale`   | `graph_version-lag`     | `row.CA`   | `row.MC`    |

The `building` variant is reserved for a future phase (no signal source today; see Plan 69-01 deferred block).

## RetrievalStatus reason priority order (factory output)

1. `engine == nil` → `{Reason: "bleve-unavailable"}` (all numeric fields zero)
2. `GetMeta(MetaKeyCorpusVersion)` empty/missing/unparseable → `{Reason: "corpus_version-uninitialized"}`
3. parsed `corpus_version < store.CurrentGraphVersion(ws.Hash())` → fields populated AND `Reason: "corpus_version-lag"`
4. `GetMeta(MetaKeyLastCompactAt)` empty/missing or zero → fields populated AND `Reason: "compactor-never-ran"`
5. all checks pass → fields populated, `Reason: ""` (omitempty)

`IndexedSymbols` is sourced from `*Engine.DocCount()` (Plan 69-02); `IndexedFiles` from `MetaKeyIndexedFiles` meta; `LastCompactAt` from `MetaKeyLastCompactAt` meta. No string literals — all keys are the exported `retrieval.MetaKey*` constants.

## SetBleveMetaFn bootstrap insertion point

Bound inside the `semanticBundle` construction block in `internal/daemon/daemon.go`, immediately after `sBndl = newSemanticBundle(…)`. This is BEFORE step 14d (SetSessionFn) and BEFORE `SetActivateCallback` is installed — critical because `ensureCompactor` captures the resolved `BleveMeta` value at compactor construction time; bindings after the first activation do NOT retroactively rewire.

```go
if compactBndl != nil && sBndl != nil {
    compactBndl.SetBleveMetaFn(func(ws workspace.WorkspaceKey) compact.BleveMeta {
        sBndl.mu.Lock()
        defer sBndl.mu.Unlock()
        eng, ok := sBndl.engines[ws.RepoRoot]
        if !ok || eng == nil {
            return nil
        }
        return eng
    })
}
```

The closure is nil-safe on the consumer side: `compactor.Deps.BleveMeta == nil` is a legal no-op per `internal/semantic/compact/accessors.go`.

## Carve-out comment for the retrievalAccessorImpl non-status methods

The factory file's package-level doc block documents the carve-out explicitly:

> Carve-out — retrievalAccessorImpl is STATUS-ONLY. Non-status methods (QueryBleve / PersonalizedPageRank / RetrievalPending / TopEdgesFor) return zero values and MUST NOT be used for production query paths; the daemon's semRetrievalAdapter retains the full production implementations for those. This impl exists so daemon AND the Plan 69-06 E2E test share a single ClusterStatus / RetrievalStatus code path.

Each non-status method also carries an inline reference back to the file-level carve-out and the production owner (`semRetrievalAdapter`).

## Deviations from plan

1. **In-scope sweep widened beyond the daemon adapter** — The plan's `<verification>` block (line 254) targets `grep -rn 'phase-62-clustering-no-status-accessor' internal/`. The user prompt expanded this to "across the entire repo." Three additional sites inside `internal/skill/semantic/` referenced the W1 sentinel string in test fixtures and doc comments (tools_status.go fallback, tools_status_test.go assertions, envelope_test.go JSON-shape assertions, integration_test.go e2eSchedAcc) — all swept. Archived `.planning/phases/64-*` SUMMARY/PLAN documents are immutable history and left untouched.

2. **Test-fixture default reason updated** — `productionDefaultClusterStatus()` (test helper) previously returned the W1 sentinel; now returns `{State: "unknown", Reason: "no-store"}` to mirror what the factory's nil-store branch produces. Test assertions that pinned the W1 string were updated to assert the new closed-enum reason instead.

3. **Test-path fallback reason in tools_status.go** — When `s.scheduler == nil` (only reachable in test paths that exercise the handler without wiring a scheduler), the handler returns `{State: "unknown", Reason: "no-scheduler"}` rather than reusing one of the factory's reasons. This keeps the test-path condition observably distinct from any real production state.

4. **`e2eRetrievalAcc.RetrievalStatus` TODO anchor preserved** — Plan 69-04 already added the stub method with a `TODO(plan-69-05)` anchor; per the spawn-time instruction (see prompt: "Otherwise leave it for 69-06"), the anchor is left intact. Plan 69-06 will swap the e2e adapter for the real `daemon.NewSchedulerAccessorForStore` / `NewRetrievalAccessorForStore` factories.

## Threat-model verification

| Threat ID | Status      | Evidence                                                                                                                                                                                                                            |
| --------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| T-69-01   | mitigated   | Both factories' status methods return only counts + monotonic versions + closed-enum reasons. No file paths, no symbol IDs traverse the adapter.                                                                                    |
| T-69-02   | mitigated   | `retrievalAccessorImpl` takes an explicit `*retrieval.Engine` argument scoped by `ws.RepoRoot`; the daemon adapter resolves the engine via `engineFor(ws)` which keys on `ws.RepoRoot`. No cross-workspace meta reads possible.    |
| T-69-03   | mitigated   | `grep -nE 'Begin(Snapshot\|OverlayTx)\|Commit(Snapshot)?\|WriteSnapshotFacts' internal/daemon/semantic_wiring.go internal/daemon/semantic_accessor_factories.go` returns zero matches.                                              |

## Verification

| Command                                                                                                  | Result    |
| -------------------------------------------------------------------------------------------------------- | --------- |
| `go build ./...`                                                                                         | clean     |
| `go vet ./...`                                                                                           | clean     |
| `go test ./internal/daemon/... ./internal/semantic/... ./internal/skill/semantic/... -race -count=1`    | PASS      |
| `grep -rn 'phase-62-clustering-no-status-accessor' internal/`                                            | 0 matches |
| `grep -nE 'W1 placeholder\|W1 production-layer placeholder\|deliberate companion' internal/daemon/semantic_wiring.go` | 0 matches |
| `grep -n 'NewSchedulerAccessorForStore\|NewRetrievalAccessorForStore' internal/daemon/semantic_accessor_factories.go`  | both defs present |
| `grep -rn 'GetMeta("corpus_version"\|GetMeta("indexed_files"\|GetMeta("last_compact_at"' internal/daemon/`              | 0 matches (constants only) |

## Commits

| Hash       | Message                                                                                             |
| ---------- | --------------------------------------------------------------------------------------------------- |
| `f2e09707` | feat(69-05): extract status-accessor factories + migrate daemon adapters + remove W1 placeholders   |
| `ce3df76c` | feat(69-05): bind bleveMetaFn + sweep phase-62-clustering-no-status-accessor                        |

## Self-Check: PASSED

- `internal/daemon/semantic_accessor_factories.go`: FOUND
- `internal/daemon/semantic_wiring.go` (modified): FOUND
- `internal/daemon/daemon.go` (modified): FOUND
- `internal/skill/semantic/tools_status.go` (modified): FOUND
- `internal/skill/semantic/accessors.go` (modified): FOUND
- `internal/skill/semantic/tools_status_test.go` (modified): FOUND
- `internal/skill/semantic/envelope_test.go` (modified): FOUND
- `internal/skill/semantic/integration_test.go` (modified): FOUND
- Commit `f2e09707`: FOUND
- Commit `ce3df76c`: FOUND
