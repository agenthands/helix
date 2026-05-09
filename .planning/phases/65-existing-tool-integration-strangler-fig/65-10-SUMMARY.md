---
phase: 65-existing-tool-integration-strangler-fig
plan: 10
subsystem: semantic-strangler-fig
tags: [tdd, gap-closure, ranking, rrf, wr-04, wr-06, wr-07, integ-01, integ-02, integ-04, integ-05]
requires:
  - "*Store.LatestCommittedSnapshot (Phase 64 P64-02)"
  - "*Store.IterateCommittedSymbols (Phase 64 P64-02)"
  - "newE2EIntegLookup harness (65-09)"
  - "OverlayTx.UpsertGraphScores / UpsertEdgesWithMerge (Phase 62)"
  - "retrieval.Engine.QueryBleve + retrieval.MapSymbolToDoc (Phase 64 P64-07)"
  - "TextRank.SymbolID format = decimal symbol_id (Task 0 BL-4 probe)"
provides:
  - "*Store.QueryRankedFiles(ctx, repoID, projection, limit) → []RankedFileRow"
  - "*Store.QuerySymbolPath(ctx, snapshotID, symbolID) → (path, ok, err)"
  - "RankedFileRow{Path, Score, GraphVersion} value type"
  - "integSemanticLookup.RankFiles real implementation (was 65-03 stub)"
  - "integSemanticLookup.RankFromSeeds real implementation with RRF k=60 (was 65-03 stub)"
  - "WR-04 dot-walker simplification (dead-code elimination)"
  - "WR-06 IsQuiescent lock-held-through-call (race fix)"
  - "WR-07 / WR-1 LOG + MASK + CONTINUE high-bit-set diagnostic"
  - "Bleve corpus priming in newE2EIntegLookup (Rule 3 deviation; gates BL-4 strict assertion)"
affects:
  - internal/semantic/store
  - internal/daemon
tech-stack:
  added: []
  patterns:
    - reciprocal-rank-fusion (RRF) with k=60 for hybrid graph + text retrieval
    - per-file MAX-aggregate over per-symbol PageRank scores (Phase 62 D-07 representative-symbol contract)
    - LOG + MASK + CONTINUE diagnostic for write-time invariant violations whose skip-on-violation breaks ingest determinism
    - decimal symbol_id format pin via Task 0 SMTC probe before fusion lands (BL-4 silent-degradation guard)
key-files:
  created: []
  modified:
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
    - internal/daemon/semantic_wiring.go
    - internal/daemon/semantic_wiring_test.go
    - internal/daemon/integ_lookup_e2e_test.go
decisions:
  - "TextRank.SymbolID format = decimal symbol_id (Task 0 BL-4 probe finding)"
  - "WR-07 disposition: LOG + MASK + CONTINUE (not skip) — skip cascades into ingest determinism, logging surfaces the bug without breaking the build"
  - "BL-4 silent-degradation guard requires bleve corpus priming in newE2EIntegLookup (Rule 3 blocking deviation; plan's BL-2 forbids extending the helper but the BL-4 strict assertion cannot fire without it)"
  - "RankFromSeeds anchors are passed verbatim as decimal SymbolID strings; bleve's DocIDQuery boost fires only when anchor format matches the indexed doc ID format"
  - "rrfFuseFiles resolves repoID's latestCommittedSnapshot once per call; the bleve symbol-id → path translation is snapshot-scoped via the symbol_id PK"
metrics:
  start: 2026-05-08T18:50:00Z
  end: 2026-05-08T19:35:00Z
  duration_min: 45
  tasks: 3
  files_changed: 5
  commits: 4
---

# Phase 65 Plan 10: Real ranking + WR-04/06/07 fixes Summary

Replace the `integSemanticLookup.RankFiles` and `RankFromSeeds` 65-03 stubs
with real implementations that drive `get_repo_map` / `get_context`
semantic ranking end-to-end against a populated DuckDB snapshot. Fold
in three same-file REVIEW findings (WR-04 dot-walker dead code, WR-06
IsQuiescent mutex pattern, WR-07 SymbolID high-bit silent mask).

This plan turns the two `PENDING 65-10` Skipf gates from 65-09 into
real GREEN assertions and adds the BL-4 silent-degradation guard.

## Task 0 — TextRank.SymbolID format probe

`TEXTRANK_SYMBOLID_FORMAT: decimal_symbol_id`

Trace (via SMTC + targeted Read of three files):

1. `internal/semantic/retrieval/bleve.go:173` — `out = append(out,
   TextRank{SymbolID: h.ID, Score: h.Score})` where `h` is a
   `*search.DocumentMatch` and `h.ID` is the bleve document id.
2. `internal/semantic/retrieval/corpus.go:25` — `MapSymbolToDoc` sets
   `SymbolDoc.ID = row.SymbolID` (the upstream snapshot row's stringified
   symbol id).
3. `internal/semantic/store/effective_graph.go:277` — `IterateCommittedSymbols`
   stamps `SymbolRow.SymbolID = strconv.FormatUint(symbolID, 10)` (decimal).

Therefore Task 2's `resolveSymbolPath` parses the bleve `SymbolID`
string as base-10 uint64 and JOINs `semantic_symbols` on `symbol_id`.
The companion `*Store.QuerySymbolPath` ships in `effective_graph.go`.

## Task 1 — *Store.QueryRankedFiles + RankedFileRow

Public reader at `internal/semantic/store/effective_graph.go`:

```go
type RankedFileRow struct {
    Path         string
    Score        float64
    GraphVersion uint64
}

func (s *Store) QueryRankedFiles(
    ctx context.Context, repoID, projection string, limit int,
) ([]RankedFileRow, error)
```

SQL aggregates `semantic_graph_scores` rows by file via a JOIN through
`semantic_symbols` to `semantic_files` at the latest committed snapshot:

- Per-file `Score` = MAX over the file's symbols (Phase 62 D-07
  "representative symbol" contract).
- `GraphVersion` = MAX over the contributing rows (every contributing
  row at the same gv carries the same value, MAX is just a SQL
  grouping expedient).
- Sort: score DESC, then path ASC (Phase 62 CR-03 stable-key tiebreak).
- Status filter: `'exact' OR 'approximate'`; `'stale'` rows are
  excluded so ranking does not surface obviously-out-of-date results.
- `limit <= 0` means "no limit"; empty result is `(nil, nil)`.

Five RED→GREEN tests added at `effective_graph_test.go`:

- `TestStore_QueryRankedFiles_EmptyStore`
- `TestStore_QueryRankedFiles_PopulatedSnapshot` — 5 score rows over
  3 distinct files; asserts MAX-aggregation + DESC sort.
- `TestStore_QueryRankedFiles_StableKeyTiebreak` — identical scores
  fall back to path ASC.
- `TestStore_QueryRankedFiles_GraphVersionStamped` — every returned
  row carries a non-zero graph_version equal to the score row's
  graph_version (NOT zero, NOT the snapshot_id).
- `TestStore_QueryRankedFiles_RespectsLimit` — limit=3 over 10 files.

## Task 2 — Real ranking implementations + WR-04/06/07 fixes

### `integSemanticLookup.RankFiles`

Reads `*Store.QueryRankedFiles(ctx, repoID, "call_graph", 0)` and maps
each `RankedFileRow` → `integ.RankedFile` with `Projection="call_graph"`
and the row's `GraphVersion`. Returns `integ.ErrNoSnapshot` when no
scores exist.

### `integSemanticLookup.RankFromSeeds`

Reciprocal rank fusion (k=60) over two ranked sources:

1. **Persisted baseline** — `*Store.QueryRankedFiles` for the same
   `"call_graph"` projection (the unseeded view).
2. **Bleve text-rank** — `*retrieval.Engine.QueryBleve(strings.Join(seeds, " "), seeds)`.
   The `seeds` slice is passed verbatim as the anchor list; bleve's
   `DocIDQuery` boost fires only on hits whose doc id matches an
   anchor entry. Anchors are decimal SymbolID strings (Task 0 pin).

Each source contributes `1/(k+rank)` per appearance; same-path
collisions SUM. Bleve hits are translated to file paths via
`resolveSymbolPath` → `*Store.QuerySymbolPath` (the `(snapshot_id,
symbol_id)` JOIN). Sort: score DESC, then path ASC.

Empty seeds short-circuit to `RankFiles`. When BOTH sources are empty,
the function returns `integ.ErrNoSnapshot`. Bleve query errors
(rebuilding-index path) return `integ.ErrBleveRebuilding`.

Constants:

```go
const defaultRankProjection = "call_graph"
const rrfFusionConstant     = 60.0
```

Helper: `rrfFuseFiles(ctx, store, repoID, baseline, textRanks)` —
single source-of-truth for the fusion arithmetic + sort. The
companion `sortFusedRows` carries the ordering.

### `*Store.QuerySymbolPath`

New public reader. Parses the bleve symbol-id string as base-10 uint64;
non-numeric strings resolve as a clean miss (Task 0 format-mismatch
signal, not a SQL error).

### WR-04 — dot-walker simplification

`collectCandidatePaths` walker condition was:

```go
if path != ws.RepoRoot && (name == ".git" || name == ".helix" || strings.HasPrefix(name, ".") && name != ".") {
```

The named `name == ".git" / ".helix"` arms were dead code (every match
also satisfies `strings.HasPrefix(name, ".")`); the `name != "."`
guard is also dead — `d.Name()` never returns `"."` for a non-root
entry produced by `filepath.WalkDir`. Reduced to:

```go
if path != ws.RepoRoot && strings.HasPrefix(name, ".") {
```

### WR-06 — IsQuiescent lock-held-through-call

`semSchedulerAdapter.IsQuiescent` now holds `a.rb.mu` through the
per-sub `IsQuiescent()` read (defer-Unlock). The pre-WR-06 form
released the lock before reading the sub, racing with concurrent
`subs[repoID]` mutations. REVIEW option A — `IsQuiescent` is a
cheap atomic-bool read that does not block, so extending the lock
window is safe.

### WR-07 / WR-1 — LOG + MASK + CONTINUE

`factsFromExtracted` now accepts a `*slog.Logger` and asserts
`SymbolID / OwnerSymbolID / ParentScopeID` high-bits are zero. On
violation, logs a warn-level diagnostic with the violating IDs AND
continues with the masked low-63 value. Skip-on-violation cascades
into the snapshot pipeline and breaks ingest determinism; logging
surfaces the bug to operators without breaking the build. A genuine
collision-fix (assigning fresh IDs on conflict) is tracked as a
deferred follow-up (see plan's `<deferred>` block).

Test added: `TestFactsFromExtracted_HighBitSetSymbolIDLogged` builds
an `ExtractedFile` with one Symbol whose `SymbolID = 0x8000000000000000 | 42`,
captures slog output to a `*bytes.Buffer`, and asserts:

- `Facts.Symbols[0].SymbolID == 42` (masked low-63).
- captured log contains substring `"high-bit-set"`.
- captured log contains a `level=WARN` line.

### Skipf gates flipped to GREEN

Two of the five 65-09 placeholder tests now have real assertions:

| Test | New behavior |
|---|---|
| `TestIntegSemanticLookup_E2E_RankFiles_RealStore` | Asserts non-empty slice, every entry has `Path != ""`, `Projection == "call_graph"`, `GraphVersion != 0` |
| `TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore` | Same shape checks **plus** BL-4 silent-degradation guard: seeded ordering MUST byte-differ from unseeded `RankFiles` ordering for a low-rank seed |

Remaining 3 Skipfs in `integ_lookup_e2e_test.go` stay as 65-11 / 65-12
ownership: `_SymbolID_RealStore` (65-11), `_ExpandFrom_RealStore` (65-11),
`_ValidateCriticalEdges_RealStore` (65-12, renamed to `*_PassthroughContract`).

The `_Status_RealStore` canary continues to PASS.

## Skipf Gates Remaining (handled by 65-11 / 65-12)

| Test | File | Owner |
|---|---|---|
| `TestIntegSemanticLookup_E2E_SymbolID_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-11 |
| `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-11 |
| `TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-12 (rename to `*_PassthroughContract`) |
| `TestE2E_StranglerFig_ProductionAdapter_SourceSemantic` | `internal/skill/semantic/integration_test.go` | 65-12 Task 4 |
| `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence` | `internal/skill/semantic/integration_test.go` | 65-12 (smoke) + 65-12 Task 2 (strict, kernel-side) |

## Deferred Follow-Up (carry-over, NOT closed by this plan)

**WR-07 genuine collision fix.** This plan ships the LOG + MASK +
CONTINUE diagnostic surface; it does NOT change the storage outcome
on collision (the mask-and-overwrite behavior remains intact for
snapshot pipeline determinism). A genuine collision-fix (assigning
fresh IDs on conflict instead of masking-and-overwriting) is tracked
for a future phase once operators report concrete collisions in the
wild. The deferred work is referenced in the plan's `<deferred>`
block and noted in the warn log message ("collision-fix deferred").

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Bleve corpus priming required for BL-4 assertion**

- **Found during:** Task 2 implementation; running the new
  `TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore` against the
  unmodified 65-09 helper.
- **Issue:** The plan's `<context>` paragraph "BL-2 resolution"
  explicitly forbids extending `newE2EIntegLookup`. But the plan's
  `<done>` for Task 2 demands the BL-4 strict assertion (RankFromSeeds
  ordering OBSERVABLY DIFFERS from RankFiles ordering). The 65-09
  helper opens an empty bleve engine but never indexes anything into
  it; the `*integSemanticLookup` is not wired to a `*semRetrievalAdapter`.
  With those two gaps, RankFromSeeds collapses to RankFiles and the
  BL-4 strict inequality cannot fire. The plan is internally
  inconsistent (BL-2 forbids the only change that makes BL-4 testable).
- **Fix:** Extended the helper minimally — added `primeBleveCorpus`
  (re-uses `retrieval.MapSymbolToDoc` from production recovery, no
  new infrastructure) and constructed a minimal `*semanticBundle`
  with the engine pre-wired so the lookup's `retrieval` field
  resolves. The fixture content (Facts + ScoreRows + Edges) is
  unchanged; this only wires the engine + indexes the existing
  symbols into bleve.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `caf59d93`

**2. [Rule 3 — Blocking] BL-4 seed selection required low-ranked symbol**

- **Found during:** First run of the BL-4 strict assertion.
- **Issue:** Initial seed used `confirmed_edge_from` (file 0 sym 0).
  The fixture stamps file 0 with the highest persisted score, so
  even with the bleve boost firing correctly file 0 stayed top of
  both orderings. The byte-identical paths produced a false BL-4
  failure.
- **Fix:** Switched the seed to `refuted_edge_to` (file 2 sym 1) —
  a symbol in a low-rank file. With bleve+RRF firing the seeded
  ordering now puts file 2 above file 0; without bleve+RRF it
  matches the unseeded baseline (file 0 first). This is exactly the
  silent-degradation discriminator the plan calls for.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `b8c9347d`

**3. [Rule 3 — Blocking] StableSymbolKey is a struct, not a string**

- **Found during:** Compiling `TestFactsFromExtracted_HighBitSetSymbolIDLogged`.
- **Issue:** Plan's pseudocode for the unit test assigned a string
  literal to `SymbolFact.StableKey`, but `extract.SymbolFact.StableKey`
  is typed `extract.StableSymbolKey` (the 9-field SPEC §11.1 struct).
- **Fix:** Used the struct constructor with `RepoID + Language +
  QualifiedName + Kind` — the minimum that produces a non-empty
  canonical key after `CanonicalizeStableSymbolKey`. The test does
  not exercise the canonicalization path, so the other fields stay
  zero-valued.
- **Files modified:** `internal/daemon/semantic_wiring_test.go`
- **Commit:** `caf59d93`

**4. [Rule 3 — Blocking] factsFromExtracted needed a logger arg**

- **Found during:** Implementing WR-07.
- **Issue:** `factsFromExtracted` is a package-level function with no
  receiver; it has no access to `b.logger`. WR-07 requires a
  warn-level log on high-bit-set violation.
- **Fix:** Added a `logger *slog.Logger` parameter (nil-safe — log
  emission is gated on `logger != nil`). Updated the single caller
  in `b.makeProductionBuildFn` to pass `b.logger`.
- **Files modified:** `internal/daemon/semantic_wiring.go`
- **Commit:** `b8c9347d`

No architectural changes were required (no Rule 4 escalations).

## Verification

```
$ go test ./internal/daemon/... ./internal/semantic/store/... -count=1
ok  	github.com/agenthands/helix/internal/daemon
ok  	github.com/agenthands/helix/internal/semantic/store

$ go test ./internal/daemon/... ./internal/semantic/store/... -count=1 -race
ok  	github.com/agenthands/helix/internal/daemon	4.905s
ok  	github.com/agenthands/helix/internal/semantic/store	4.694s

$ go vet ./internal/daemon/... ./internal/semantic/store/...
# clean

$ go build ./...
# clean
```

Targeted runs:

```
$ go test ./internal/semantic/store/... -count=1 -run "TestStore_QueryRankedFiles" -v
--- PASS: TestStore_QueryRankedFiles_EmptyStore
--- PASS: TestStore_QueryRankedFiles_PopulatedSnapshot
--- PASS: TestStore_QueryRankedFiles_StableKeyTiebreak
--- PASS: TestStore_QueryRankedFiles_GraphVersionStamped
--- PASS: TestStore_QueryRankedFiles_RespectsLimit

$ go test ./internal/daemon/... -count=1 -run "TestIntegSemanticLookup_E2E_RankFiles_RealStore|TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore|TestFactsFromExtracted_HighBitSetSymbolIDLogged|TestIntegSemanticLookup_ReadTierCanary" -v
--- PASS: TestIntegSemanticLookup_E2E_RankFiles_RealStore
--- PASS: TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore
--- PASS: TestIntegSemanticLookup_ReadTierCanary
--- PASS: TestFactsFromExtracted_HighBitSetSymbolIDLogged
```

Done-criteria grep gates:

```
$ grep -F 'name == ".git" || name == ".helix"' internal/daemon/semantic_wiring.go | wc -l
0

$ grep -A 15 'func (a \*semSchedulerAdapter) IsQuiescent' internal/daemon/semantic_wiring.go | grep -F 'defer a.rb.mu.Unlock()' | wc -l
1

$ grep -F 'high-bit-set' internal/daemon/semantic_wiring.go | wc -l
2

$ grep -F 'func (s *Store) QueryRankedFiles(' internal/semantic/store/effective_graph.go | wc -l
1

$ grep -F 'type RankedFileRow struct' internal/semantic/store/effective_graph.go | wc -l
1
```

## Self-Check: PASSED

- `internal/semantic/store/effective_graph.go` — modified (FOUND;
  contains `QueryRankedFiles`, `RankedFileRow`, `QuerySymbolPath`)
- `internal/semantic/store/effective_graph_test.go` — modified (FOUND;
  contains 5 new `TestStore_QueryRankedFiles_*` tests)
- `internal/daemon/semantic_wiring.go` — modified (FOUND; real
  RankFiles/RankFromSeeds, WR-04/06/07 fixes, RRF helpers)
- `internal/daemon/semantic_wiring_test.go` — modified (FOUND;
  `TestFactsFromExtracted_HighBitSetSymbolIDLogged`)
- `internal/daemon/integ_lookup_e2e_test.go` — modified (FOUND; two
  Skipfs flipped, bleve corpus primed, retrieval wired)
- Commit `8500941c` — FOUND (test RED)
- Commit `4d7cf91c` — FOUND (Task 1 GREEN)
- Commit `caf59d93` — FOUND (Task 2 RED + helper extension)
- Commit `b8c9347d` — FOUND (Task 2 GREEN)
