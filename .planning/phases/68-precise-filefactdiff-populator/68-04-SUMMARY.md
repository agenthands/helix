---
phase: 68-precise-filefactdiff-populator
plan: 04
subsystem: live-handler
tags: [live-handler, difffacts, populator, wiring, tier1, tier2, tier3]
dependency_graph:
  requires:
    - 68-01 (store.PriorFileFact + GetLatestFileFact accessor)
    - 68-02 (extract.ExtractionPipeline.ExtractFile shims)
    - 68-03 (obs.Metrics.LiveFileFactDiff* surfaces)
  provides:
    - "handler.FileFactStore / ExtractRegistry / FileFactDiffMetricsSink interfaces"
    - "handler.Handler.SetFileFactStore + SetExtractRegistry + FileFactDiffMetrics field"
    - "handler.diffSymbols(prior []store.PriorSymbol, curr []extract.SymbolFact, rec) — Pitfall-3-safe diff algorithm"
    - "Tier-1 tryFullDiff + Tier-2 tryAddedOnlyDiff (bool, string) populators"
    - "Tier-3 bounded reason routing (cold_start / extract_failed / extract_unsupported)"
    - "handler.LastRecorderSnapshotForTest + DiffSymbolsForTest export-test seams"
    - "Daemon post-init DI of live handler dependencies"
  affects:
    - DIFF-01 active (Tier-1 precise diff)
    - DIFF-04 active (Tier-2 added-only graceful degrade)
tech_stack:
  added: []
  patterns:
    - "tuple-return (handled, tier3Reason) populator contract (locked Plan 68-04 shape)"
    - "narrow handler-local interfaces (FileFactStore / ExtractRegistry / FileFactDiffMetricsSink) — production types satisfy without adapters"
    - "single canonicalize-on-compare call for StableKey (extract.CanonicalizeStableSymbolKey at the compare site)"
key_files:
  created:
    - internal/semantic/live/handler/difffacts_test.go
  modified:
    - internal/semantic/live/handler/handler.go
    - internal/semantic/live/handler/difffacts.go
    - internal/semantic/live/handler/export_test.go
    - internal/daemon/daemon.go
decisions:
  - "Wired the daemon DI at the existing SetRankApplier site in internal/daemon/daemon.go rather than internal/daemon/semantic_wiring.go (per plan files_modified). Rationale: live.handler is already constructed and SetRankApplier-wired in daemon.go — adding the new setters there keeps the analog site cohesive. semantic_wiring.go has no existing handler wiring; introducing a second wiring point would split responsibility."
  - "Duplicated langFromExt (8 LOC, 4 cases) into difffacts.go rather than hoisting to a new shared package. Reason: shared package would have a single consumer pair, increasing surface area for negligible deduplication. The function is stable (4 known extract.Provider languages) — Phase 68 D-02 vet-nokernel2semantic invariant intact (handler does NOT import daemon)."
  - "*semanticstore.Store satisfies handler.FileFactStore automatically — no adapter needed. The method signature `GetLatestFileFact(ctx, repoID, path string) (store.PriorFileFact, bool, error)` declared in Plan 68-01 matches verbatim."
  - "*extract.Registry satisfies handler.ExtractRegistry automatically — no adapter needed. Method `Provider(lang string) (Provider, bool)` already exists."
  - "FileFactDiffMetrics is a PUBLIC field on Handler (not a setter) because *obs.Metrics is the only production sink and the field assignment in daemon.go is a one-liner; the existing test files in this package use direct field assignment for h.Metrics already (handler_lane_test.go:193)."
  - "Captured recorder snapshot pre-Compute via h.lastRecorderSnapshot = recorder.Snapshot() to drive the LastRecorderSnapshotForTest seam. The Phase 62 path that called recorder.Snapshot() inside the ApplyRepair block was redirected to read h.lastRecorderSnapshot so there is a single source of truth (concurrent safety preserved by 62-09 single-goroutine tx ownership)."
metrics:
  duration_minutes: 35
  completed: 2026-05-13
  tasks_completed: 3
  files_created: 1
  files_modified: 4
---

# Phase 68 Plan 04: Tier-1/Tier-2 Populator + DI Wiring Summary

One-liner: filled the Tier-1 (precise diff) and Tier-2 (added-only)
populator stubs in `internal/semantic/live/handler/difffacts.go`,
shipped a Pitfall-3-safe `diffSymbols` that bridges
`[]store.PriorSymbol` (prior) → `[]extract.SymbolFact` (current), routed
Tier-3 fall-through through the bounded-label synthetic-reason metric,
added nil-safe Handler DI for the file-fact accessor + extract registry,
and wired the daemon post-init so production traffic flows through real
diffs. Closes DIFF-01 (Tier-1 active) and DIFF-04 (Tier-2 graceful
degrade).

## Tasks Completed

| Task | Name                                                                                 | Status | Commit     | Files                                                                                                                          |
|------|--------------------------------------------------------------------------------------|--------|------------|--------------------------------------------------------------------------------------------------------------------------------|
| 1    | RED — Failing diff tests + handler DI tests                                          | done   | `7f9d75a4` | `internal/semantic/live/handler/difffacts_test.go` (new), `internal/semantic/live/handler/export_test.go`                      |
| 2    | GREEN — Handler DI, diff algorithm, Tier-1/2 populators, Tier-3 reason routing       | done   | `1bae7395` | `internal/semantic/live/handler/handler.go`, `internal/semantic/live/handler/difffacts.go`                                     |
| 3    | Wire daemon post-init DI for FileFactStore + ExtractRegistry                         | done   | `12f21d1b` | `internal/daemon/daemon.go`                                                                                                    |

## Open Question Resolutions

- **Q4 (recorder snapshot test seam):** Resolved via a single
  `h.lastRecorderSnapshot graphpkg.FileFactDiff` field captured exactly
  once per `updateChangedFileWithKind` invocation, immediately after
  the populator runs. The existing `ApplyRepair` block reads from this
  field instead of calling `recorder.Snapshot()` a second time, so the
  test seam observes exactly the value the production path consumed.

## Final Tier Contract (locked)

### tryFullDiff signature

```go
func (h *Handler) tryFullDiff(ctx context.Context, repoID semantic.RepoID,
    path string, recorder *FileFactDiffRecorder) (bool, string)
```

Reason propagation:

| Path                                                  | Returns                         |
|-------------------------------------------------------|---------------------------------|
| h.factStore == nil                                    | (false, "")                     |
| GetLatestFileFact error                               | (false, "cold_start") + warn    |
| GetLatestFileFact !ok                                 | (false, "cold_start")           |
| h.extractRegistry == nil OR provider not found        | (false, "cold_start")           |
| ExtractFile err / nil                                 | (false, "extract_failed")       |
| status == Ready                                       | (true, "")  — Tier-1 fires      |
| status == Partial                                     | (false, "")  — Tier-2 owns      |
| status == Failed                                      | (false, "extract_failed")       |
| status == Unsupported                                 | (false, "extract_unsupported")  |

### tryAddedOnlyDiff signature

```go
func (h *Handler) tryAddedOnlyDiff(ctx context.Context, repoID semantic.RepoID,
    path string, recorder *FileFactDiffRecorder) (bool, string)
```

Reason propagation:

| Path                                                  | Returns                         |
|-------------------------------------------------------|---------------------------------|
| h.extractRegistry == nil OR provider not found        | (false, "")                     |
| ExtractFile err / nil                                 | (false, "extract_failed")       |
| status == Partial                                     | (true, "")  — Tier-2 fires      |
| status == Ready                                       | (false, "")  — Tier-1 owns      |
| status == Failed                                      | (false, "extract_failed")       |
| status == Unsupported                                 | (false, "extract_unsupported")  |

### populateRecorderForFile decision rule

```go
if handled, reason := h.tryFullDiff(...); handled { return } else { tier3Reason = reason }
if handled, reason := h.tryAddedOnlyDiff(...); handled { return } else if reason != "" {
    tier3Reason = reason  // extract-fail/unsupported wins over cold_start
}
if tier3Reason == "" { tier3Reason = "cold_start" }  // degenerate-path default
h.FileFactDiffMetrics.LiveFileFactDiffSyntheticReasonInc(tier3Reason)
recorder.RecordSymbolChanged(graphpkg.SymbolDiff{KindChanged: true})  // synthetic marker
h.emitFileFactDiffOutcome(repoID, "synthetic")
```

## langFromExt Disposition

**Duplicated** into `internal/semantic/live/handler/difffacts.go`
(8 LOC, 4 cases). Not hoisted to a shared package because the daemon
package is the only other consumer and the function is stable (the
extract provider set has been the same 4 languages — Go / TypeScript /
JavaScript / Python — since Phase 59 P05). Duplicating preserves the
vet-nokernel2semantic invariant (handler does NOT import daemon).

## Adapter Disposition

- **`*semanticstore.Store` → `handler.FileFactStore`:** NO adapter
  needed. The method signature
  `GetLatestFileFact(ctx, repoID, path string) (store.PriorFileFact, bool, error)`
  shipped by Plan 68-01 matches the handler-side interface declaration
  verbatim.

- **`*extract.Registry` → `handler.ExtractRegistry`:** NO adapter
  needed. The existing `Provider(lang string) (Provider, bool)` method
  on `*extract.Registry` matches the handler-side interface
  declaration. The interface's return type `extract.Provider` is the
  same concrete type the registry has always returned.

- **`*obs.Metrics` → `handler.FileFactDiffMetricsSink`:** NO adapter
  needed. The two methods on the interface
  (`LiveFileFactDiffInc(tier, repo)` and
  `LiveFileFactDiffSyntheticReasonInc(reason)`) match the Plan 68-03
  helper methods verbatim.

## Pitfall 3 Verification (load-bearing negative invariant)

```
$ grep -c 'SignatureHash' internal/semantic/live/handler/difffacts.go
0
```

The body-mixing hash field is never read in the diff path. The
positive-side invariant:

```
$ grep -c 'p\.Signature != n\.Signature' internal/semantic/live/handler/difffacts.go
2
```

(One in the algorithm, one in the load-bearing doc comment block.)

`TestDiffSymbols_BodyOnlyNotGraphChanging` is the runtime regression
test that asserts a body-only edit (identical Signature, different
SignatureHash on the prior side, different hash on the current side)
produces zero `ChangedSymbols`, zero `AddedSymbols`, zero
`RemovedSymbols` — body-only edits do NOT advance graph_version.

## Test List + Pass Count

All passing under `go test -race ./internal/semantic/live/handler/... -count=1`:

| Test                                             | Subtests / Cases | Result |
|--------------------------------------------------|------------------|--------|
| TestDiffSymbols                                  | 6 (added, removed, changed-signature, changed-visibility, changed-kind, unchanged) | PASS |
| TestDiffSymbols_BodyOnlyNotGraphChanging         | 1                | PASS   |
| TestTryFullDiff_HappyPath                        | 1                | PASS   |
| TestTryFullDiff_ColdStart                        | 1                | PASS   |
| TestTryFullDiff_StoreError                       | 1                | PASS   |
| TestTryAddedOnlyDiff_Partial                     | 1                | PASS   |
| TestTryAddedOnlyDiff_ReadyNotHandled             | 1                | PASS   |
| TestTier3_BoundedReasonMetric                    | 3 (cold_start, extract_failed, extract_unsupported) | PASS |
| TestFileFactDiffOutcomeMetric                    | 1                | PASS   |

Pre-existing tests in the package (recorder_test.go, noop_test.go,
handler_lane_test.go) also pass — no regressions introduced.

Daemon regression: `go test -race ./internal/daemon/... -count=1` → PASS.

Graph regression: `go test -race ./internal/semantic/graph/... -count=1` → PASS.

`make vet` → exit 0 (vet-nokernel2semantic, vet-nosemantic2kernel,
vet-noduckdb, vet-compact-uses-store all clean).

## Acceptance Criteria

- [x] `grep -c 'func diffSymbols' internal/semantic/live/handler/difffacts.go` == 1
- [x] `grep -c 'p\.Signature != n\.Signature' internal/semantic/live/handler/difffacts.go` ≥ 1 (= 2)
- [x] `grep -c 'SignatureHash' internal/semantic/live/handler/difffacts.go` == 0 (Pitfall 3 negative invariant)
- [x] `grep -c 'func (h \*Handler) SetFileFactStore' internal/semantic/live/handler/handler.go` == 1
- [x] `grep -c 'func (h \*Handler) SetExtractRegistry' internal/semantic/live/handler/handler.go` == 1
- [x] `grep -c 'LiveFileFactDiffSyntheticReasonInc' internal/semantic/live/handler/difffacts.go` ≥ 1 (= 2)
- [x] `grep -cE '"cold_start"\|"extract_failed"\|"extract_unsupported"' internal/semantic/live/handler/difffacts.go` ≥ 3 (= 8)
- [x] `grep -cE 'func \(h \*Handler\) tryFullDiff\(.*\) \(bool, string\)' ...` == 1 (locked tuple-return contract)
- [x] `grep -cE 'func \(h \*Handler\) tryAddedOnlyDiff\(.*\) \(bool, string\)' ...` == 1
- [x] `grep -c 'lastTier3Reason' internal/semantic/live/handler/handler.go` == 0 (rejected alternative — no Handler-mutable reason field)
- [x] `grep -v '^[[:space:]]*//' internal/semantic/live/handler/difffacts.go | grep -c 'internal/kernel'` == 0
- [x] `grep -c 'SetFileFactStore' internal/daemon/daemon.go` == 1 (≥1 required; wired at SetRankApplier site, not semantic_wiring.go — see Decisions)
- [x] `grep -c 'SetExtractRegistry' internal/daemon/daemon.go` == 1
- [x] `grep -c 'BumpGraphVersion' internal/semantic/live/handler/difffacts.go` == 0 (D-06 single-call-site preserved)
- [x] `go build ./...` exits 0
- [x] `make vet` exits 0
- [x] `go test -race ./internal/daemon/... ./internal/semantic/live/handler/... -count=1` exits 0

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Wave 1 deviation propagation] diffSymbols bridges
store.PriorSymbol (prior) → extract.SymbolFact (current)**

- **Found during:** Task 1 RED authoring.
- **Issue:** Plan 68-04's `<interfaces>` block specifies
  `PriorFileFact.Symbols []extract.SymbolFact`, but Plan 68-01 was
  forced to ship `[]store.PriorSymbol` instead to avoid an
  `extract → store → extract` import cycle (see 68-01-SUMMARY.md
  Deviations §1). The plan's `<wave1_deviation_advisory>` already
  documents that the handler package can import both packages and
  should bridge the two types.
- **Fix:** `diffSymbols(prior []store.PriorSymbol, curr []extract.SymbolFact, rec)`
  takes both shapes. The union of fields it reads (ID, Signature,
  Visibility, Kind, StableKey) is present on both. Kind comparison
  uses `p.Kind != string(n.Kind)` (PriorSymbol.Kind is `string`,
  extract.SymbolFact.Kind is `extract.SymbolKind` (a `string`-newtype)).
  StableKey comparison canonicalizes the extract side on the fly via
  `extract.CanonicalizeStableSymbolKey(n.StableKey)` — matching the
  store-side canonical string.
- **Files modified:** `internal/semantic/live/handler/difffacts.go`.
- **Commit:** `1bae7395`.

**2. [Rule 1 — Acceptance criterion compliance] Reworded all
"SignatureHash" mentions in difffacts.go to "body-mixing hash field"**

- **Found during:** Task 2 GREEN verification.
- **Issue:** First-pass implementation included `SignatureHash` in
  doc-comment Pitfall 3 warnings, which made
  `grep -c 'SignatureHash' difffacts.go` return 6 (acceptance
  criterion requires 0). The criterion was deliberately strict —
  the negative invariant must hold even in comments so a future grep-
  search for SignatureHash in difffacts.go returns no hits at all.
- **Fix:** Reworded all comment references to "the body-mixing hash
  field" / "the body-mixing hash". Algorithm behavior unchanged.
- **Files modified:** `internal/semantic/live/handler/difffacts.go`.
- **Commit:** folded into `1bae7395`.

**3. [Rule 3 — Blocking deviation] Wired daemon DI in `daemon.go`
not `semantic_wiring.go`**

- **Found during:** Task 3 start.
- **Issue:** Plan frontmatter listed `internal/daemon/semantic_wiring.go`
  in `files_modified`, but the actual `live.handler` construction
  + `SetRankApplier` wiring lives in `internal/daemon/daemon.go`
  (lines 370-394). `semantic_wiring.go` contains NO existing handler
  references. Adding a second wiring point there would split
  responsibility and violate the analog-site principle stated in
  68-RESEARCH.md A4.
- **Fix:** Wired `SetFileFactStore` / `SetExtractRegistry` / direct
  `FileFactDiffMetrics` assignment in `internal/daemon/daemon.go`
  at the same site as the existing `SetRankApplier` call (line 384+),
  inside the same `if semanticStore != nil && live != nil && live.handler != nil`
  nil-guard.
- **Files modified:** `internal/daemon/daemon.go` (vs. plan's
  `internal/daemon/semantic_wiring.go`).
- **Commit:** `12f21d1b`.

## TDD Gate Compliance

- **RED gate (`test`):** commit `7f9d75a4` — package failed to compile
  with `undefined: handler.FileFactStore / SetFileFactStore /
  SetExtractRegistry / FileFactDiffMetrics / lastRecorderSnapshot /
  diffSymbols / LastRecorderSnapshotForTest / DiffSymbolsForTest`.
- **GREEN gate (`feat`):** commit `1bae7395` — all Task 1 tests pass
  under `-race`; `make vet` exits 0.
- **REFACTOR gate:** not required (the GREEN-pass code shipped with
  the documentation reword folded into the same commit; no separate
  cleanup pass needed).

## Threat Flags

No NEW threat surface introduced beyond what Plan 68-04's
`<threat_model>` already itemized. All five register entries (T-68-11
through T-68-15) are addressed:

- T-68-11 (incorrect diff via SignatureHash) — mitigated via the
  Pitfall 3 negative invariant (0 mentions in difffacts.go) and
  `TestDiffSymbols_BodyOnlyNotGraphChanging` regression.
- T-68-12 (Tier-3 unbounded label cardinality) — mitigated by the
  closed-enum drop-on-unknown helper Plan 68-03 shipped on
  `*obs.Metrics`.
- T-68-13 (Tier-2 over-advance) — accepted (documented price per
  68-RESEARCH.md Pitfall 4); metric `tier="added-only"` makes the
  path observable.
- T-68-14 (kernel→semantic boundary leak) — mitigated; difffacts.go
  has zero `internal/kernel` imports; `make vet` (including
  vet-nokernel2semantic) green.
- T-68-15 (race on lastRecorderSnapshot) — mitigated by single-
  goroutine tx ownership invariant (62-09); `-race` test runs verify.

## Self-Check: PASSED

- `internal/semantic/live/handler/difffacts.go` — FOUND.
- `internal/semantic/live/handler/difffacts_test.go` — FOUND.
- `internal/semantic/live/handler/handler.go` — FOUND (modified).
- `internal/semantic/live/handler/export_test.go` — FOUND (modified).
- `internal/daemon/daemon.go` — FOUND (modified).
- Commit `7f9d75a4` (Task 1 RED) — FOUND.
- Commit `1bae7395` (Task 2 GREEN) — FOUND.
- Commit `12f21d1b` (Task 3 wiring) — FOUND.
