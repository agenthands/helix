---
phase: 65-existing-tool-integration-strangler-fig
plan: 09
subsystem: semantic-strangler-fig
tags: [tdd, gap-closure, e2e-harness, cr-02, integ-04, integ-05]
requires:
  - integSemanticLookup adapter shape (65-03)
  - SemanticLookup interface + ConfigGate / Source / FallbackReason (65-04)
  - integ.SymbolID + RankedFile + Edge / ValidatedEdge / SemanticStatus (65-04)
  - serr.ErrUnsupported sentinel + daemon.semanticStoreProbe.Probe wrapping
  - semanticstore.Store: BeginSnapshot / WriteSnapshotFacts / CommitSnapshot
  - overlay.OverlayTx: BeginOverlayTx / UpsertGraphScores / UpsertEdgesWithMerge
provides:
  - newE2EIntegLookup test harness (Facts + ScoreRows + EdgeRows + canonical syms map)
  - NewIntegSemanticLookupForTest (cross-package bare adapter constructor)
  - NewE2EIntegLookupForTest (cross-package populated harness builder, BL-A)
  - FixtureSymbolMeta exported type + canonical key set
  - Skipf placeholder tests gating 65-10 (RankFiles, RankFromSeeds), 65-11 (SymbolID, ExpandFrom), 65-12 (ValidateCriticalEdges)
  - Active TestIntegSemanticLookup_E2E_Status_RealStore canary
  - errors.Is(err, serr.ErrUnsupported)-based classifySemanticProbeError (CR-02)
  - TestSemanticStoreStatus_Unhealthy_DBErrorTextLooksLikeNilHandle regression guard
affects:
  - internal/daemon (test-only surface)
  - internal/skill/semantic (test-only placeholders)
  - internal/kernel/health (production classifier swap, no public-API change)
tech-stack:
  added: []
  patterns:
    - test-only export via _test.go suffix in production package (cross-package access seam without build tags)
    - canonical-key fixture map for cross-plan asssertion stability
    - sentinel-based error classification via errors.Is (closed-enum doctrine)
key-files:
  created:
    - internal/daemon/integ_lookup_e2e_test.go
    - internal/daemon/integ_lookup_export_for_test.go
  modified:
    - internal/skill/semantic/integration_test.go
    - internal/kernel/health/tools.go
    - internal/kernel/health/tools_semantic_test.go
decisions:
  - FixtureSymbolMeta.EdgeID declared as uint64 (integ.EdgeID type does not exist; uint64 matches overlay edgeIDForTriple's 63-bit hash output)
  - edgeIDForTripleLocal mirrors overlay.edgeIDForTriple (package-private) to derive canonical EdgeIDs without dragging in a public accessor
  - Status canary uses bundle=nil to exercise the nil-guarded PendingLSP / LastLiveUpdateMs paths
metrics:
  start: 2026-05-08T16:08:00Z
  end: 2026-05-08T16:20:46Z
  duration_min: 13
  tasks: 3
  files_changed: 5
  commits: 3
---

# Phase 65 Plan 09: Gap-closure Wave 4 (E2E harness + CR-02 fix) Summary

Stand up the production-adapter end-to-end test harness that 65-10 / 65-11 /
65-12 will turn from skipped → GREEN, and replace the substring-matching
`classifySemanticProbeError` with an `errors.Is(err, serr.ErrUnsupported)`
sentinel match (REVIEW.md CR-02).

## What Shipped

### Task 1 — `internal/daemon/integ_lookup_e2e_test.go` + `internal/daemon/integ_lookup_export_for_test.go` (commit `1ecf50ef`)

- `newE2EIntegLookup(t)` opens a real `*semanticstore.Store` + `*retrieval.Engine`
  inside a `t.TempDir()`, commits a fixture snapshot of 3 files × 5 symbols =
  15 symbols (every symbol carries explicit `start_line`, `start_col`,
  `end_line`, `end_col`, `stable_key`), then opens an `OverlayTx` and writes
  15 ScoreRows (`status="exact"`, `projection="call_graph"`, varying scores)
  plus ≥ 6 EdgeRows including the canonical confirmable + refutable edge
  pair (`UpsertEdgesWithMerge`).
- The helper publishes a `symbolsByStableKey` map of type
  `map[string]FixtureSymbolMeta`. Canonical keys (BL-A — fixed contract for
  cross-package callers): `confirmed_edge_from`, `confirmed_edge_to`,
  `refuted_edge_from`, `refuted_edge_to`, `confirmed_edge`, `refuted_edge`.
  The remaining 15 symbol-shaped entries are keyed by their stable_key
  string.
- 5 Skipf placeholder tests + 1 active canary:
  - `TestIntegSemanticLookup_E2E_RankFiles_RealStore` — Skipf "PENDING 65-10"
  - `TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore` — Skipf "PENDING 65-10"
  - `TestIntegSemanticLookup_E2E_SymbolID_RealStore` — Skipf "PENDING 65-11"
  - `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore` — Skipf "PENDING 65-11"
  - `TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore` — Skipf
    "PENDING 65-12 — see 65-12 Task 2 passthrough rewrite"
  - `TestIntegSemanticLookup_E2E_Status_RealStore` — ACTIVE: PASSes today,
    proves harness wiring (asserts `State == StatusReady`, `Store == "duckdb"`,
    `LatestSnapshotID > 0`).
- `internal/daemon/integ_lookup_export_for_test.go` (`_test.go` suffix → excluded
  from production builds by the standard Go convention; no build tag) exposes:
  - `type FixtureSymbolMeta struct { Path; Line; Col; SymbolID; EdgeID; StableKey }`
  - `NewIntegSemanticLookupForTest(s, ws) integ.SemanticLookup` — bare adapter
    against a caller-built store.
  - `NewE2EIntegLookupForTest(t)` — wraps `newE2EIntegLookup` so cross-package
    callers in 65-12 (`internal/kernel/symbols/blast_radius_strangler_test.go`)
    consume the populated harness verbatim (BL-A).

### Task 2 — `internal/skill/semantic/integration_test.go` (commit `8cb3c9b7`)

Two Skipf'd skill-level placeholders that 65-12 Task 4 flips to GREEN:

- `TestE2E_StranglerFig_ProductionAdapter_SourceSemantic` — post-flip asserts
  `envelope.Source == integ.SourceSemantic` and `envelope.GraphVersion != 0`.
- `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence` — thin
  envelope-shape smoke check; the strict 1.00/0.20 confidence-ladder
  assertion lives in `internal/kernel/symbols/blast_radius_strangler_test.go`
  (BL-1) and consumes `daemon.NewE2EIntegLookupForTest`.

### Task 3 — CR-02 fix (commit `437841b7`)

- `internal/kernel/health/tools.go`: `classifySemanticProbeError` now matches
  the daemon-emitted sentinel via `errors.Is(err, serr.ErrUnsupported)`.
  `strings.Contains(msg, "DB handle nil")` and
  `strings.Contains(msg, "store unavailable")` are gone; the `strings`
  import is removed; a new `serr "github.com/agenthands/helix/internal/errors"`
  import is added.
- `internal/kernel/health/tools_semantic_test.go`:
  `TestSemanticStoreStatus_Unhealthy_NilHandle` rewritten with three
  sub-cases (`sentinel_direct`, `sentinel_wrapped_fmt_errorf`,
  `sentinel_double_wrapped`) — each wraps `serr.ErrUnsupported` and asserts
  `Reason == SemanticReasonNilHandle`.
  New `TestSemanticStoreStatus_Unhealthy_DBErrorTextLooksLikeNilHandle`
  regression guard proves an `errors.New("semantic store unavailable: connection reset")`
  routes to `SemanticReasonDBError`, not `SemanticReasonNilHandle`.

## Skipf Gates 65-10 / 65-11 / 65-12 Must Flip

| Test | File | Owner |
|---|---|---|
| `TestIntegSemanticLookup_E2E_RankFiles_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-10 |
| `TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-10 |
| `TestIntegSemanticLookup_E2E_SymbolID_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-11 |
| `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-11 |
| `TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-12 (renamed to `*_PassthroughContract`) |
| `TestE2E_StranglerFig_ProductionAdapter_SourceSemantic` | `internal/skill/semantic/integration_test.go` | 65-12 Task 4 |
| `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence` | `internal/skill/semantic/integration_test.go` | 65-12 Task 4 (smoke) + 65-12 Task 2 (strict, kernel-side) |

## BL-2 Fixture Continuity Contract

Downstream waves 65-10 (Wave 5) and 65-11 (Wave 6) consume `newE2EIntegLookup`
**unchanged**. They only flip Skipf gates and add real assertions. No plan's
E2E `done` depends on a fixture extension that lives only in pseudocode —
the helper writes Facts + ScoreRows + EdgeRows + builds `symbolsByStableKey`
in this plan.

## BL-A Canonical-Key Contract

The fixed key set the cross-package consumer in 65-12 Task 2
(`TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder` and siblings) reads:

| Key | Shape | Fields populated |
|---|---|---|
| `confirmed_edge_from` | symbol | `Path`, `Line`, `Col`, `SymbolID`, `StableKey` |
| `confirmed_edge_to`   | symbol | `Path`, `Line`, `Col`, `SymbolID`, `StableKey` |
| `refuted_edge_from`   | symbol | `Path`, `Line`, `Col`, `SymbolID`, `StableKey` |
| `refuted_edge_to`     | symbol | `Path`, `Line`, `Col`, `SymbolID`, `StableKey` |
| `confirmed_edge`      | edge   | `EdgeID`, `StableKey` |
| `refuted_edge`        | edge   | `EdgeID`, `StableKey` |

Plus all 15 underlying symbol-shaped entries keyed by `stable_key`
("file0-sym0-stable" .. "file2-sym4-stable").

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] FixtureSymbolMeta.EdgeID type substitution**
- **Found during:** Task 1 implementation
- **Issue:** Plan specified `EdgeID integ.EdgeID`, but no `EdgeID` type
  exists in `internal/semantic/integ/`. Importing the type would fail to
  compile.
- **Fix:** Declared `EdgeID` as `uint64`. The value-shape is identical:
  overlay edge IDs are `uint64` hashes from `overlay.edgeIDForTriple`
  (FNV-1a → 63-bit mask). Downstream consumers in 65-12 read it as a
  numeric edge identifier; no type-introduction wave is needed.
- **Files modified:** `internal/daemon/integ_lookup_export_for_test.go`
- **Commit:** `1ecf50ef`

**2. [Rule 3 — Blocking] edgeIDForTriple is package-private**
- **Found during:** Task 1 implementation
- **Issue:** The deterministic edge-id hash `overlay.edgeIDForTriple`
  needed to populate `syms["confirmed_edge"]` / `syms["refuted_edge"]`
  is package-private to `internal/semantic/store/`.
- **Fix:** Re-implemented the same FNV-1a-with-63-bit-mask hash inline
  as `edgeIDForTripleLocal` in `integ_lookup_e2e_test.go`. Mirrors the
  upstream signature exactly (same constants, same byte-walk order).
  The alternative — exporting `EdgeIDForTriple` from the store package —
  would have widened the public surface for a test-only consumer.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `1ecf50ef`

**3. [Rule 3 — Blocking] WorkspaceKey has Hash() not RepoHash field**
- **Found during:** Task 1 implementation
- **Issue:** Plan pseudocode used `ws.RepoHash` to derive the snapshot
  `RepoID`, but `WorkspaceKey` has only `RepoRoot`, `Language`,
  `Toolchain` fields plus a `Hash()` method.
- **Fix:** Used `ws.Hash()` per the existing `internal/skill/semantic/integration_test.go`
  pattern. Functionally equivalent — `Hash()` returns the deterministic
  per-workspace identifier the snapshot writer expects.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `1ecf50ef`

**4. [Rule 3 — Blocking] UpsertEdgesWithMerge signature does not take a projection arg**
- **Found during:** Task 1 implementation
- **Issue:** Plan pseudocode showed `tx.UpsertEdgesWithMerge(ctx, "call_graph", edgeRows)`,
  but the real signature is `(ctx, edgeRows []EdgeRow)` — no projection.
- **Fix:** Called the function with the actual signature `(ctx, edgeRows)`.
  Edge rows are not projection-keyed in the schema; the projection is
  carried on `ScoreRow`s only.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `1ecf50ef`

**5. [Rule 3 — Blocking] semanticstore.Open signature requires logger + metrics**
- **Found during:** Task 1 implementation
- **Issue:** Plan pseudocode showed `semanticstore.Open(ctx, cfg)`, but the
  real signature is `Open(ctx, cfg, logger, metrics)`.
- **Fix:** Wired a discard-handler `slog.Logger` plus `obs.Noop` provider's
  `Metrics()`, mirroring the existing `internal/skill/semantic/integration_test.go:newE2EHarness`
  pattern.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `1ecf50ef`

No architectural changes were required (no Rule 4 escalations).

## Verification

```
$ go test ./internal/daemon/... ./internal/skill/semantic/... ./internal/kernel/health/... -count=1
ok  	github.com/agenthands/helix/internal/daemon	3.630s
ok  	github.com/agenthands/helix/internal/skill/semantic	4.249s
ok  	github.com/agenthands/helix/internal/kernel/health	1.366s

$ go vet ./internal/daemon/... ./internal/skill/semantic/... ./internal/kernel/health/...
# clean

$ go build ./...
# clean
```

E2E targeted run:

```
$ go test ./internal/daemon/... -count=1 -run "TestIntegSemanticLookup_E2E" -v
--- SKIP: TestIntegSemanticLookup_E2E_RankFiles_RealStore (PENDING 65-10)
--- SKIP: TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore (PENDING 65-10)
--- SKIP: TestIntegSemanticLookup_E2E_SymbolID_RealStore (PENDING 65-11)
--- SKIP: TestIntegSemanticLookup_E2E_ExpandFrom_RealStore (PENDING 65-11)
--- SKIP: TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore (PENDING 65-12)
--- PASS: TestIntegSemanticLookup_E2E_Status_RealStore (0.10s)
PASS

$ go test ./internal/skill/semantic/... -count=1 -run "TestE2E_StranglerFig_ProductionAdapter" -v
--- SKIP: TestE2E_StranglerFig_ProductionAdapter_SourceSemantic (PENDING 65-12 Task 4)
--- SKIP: TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence (PENDING 65-12)
PASS

$ go test ./internal/kernel/health/... -count=1 -run "TestSemanticStoreStatus_Unhealthy" -v
--- PASS: TestSemanticStoreStatus_Unhealthy
--- PASS: TestSemanticStoreStatus_Unhealthy_NilHandle/sentinel_direct
--- PASS: TestSemanticStoreStatus_Unhealthy_NilHandle/sentinel_wrapped_fmt_errorf
--- PASS: TestSemanticStoreStatus_Unhealthy_NilHandle/sentinel_double_wrapped
--- PASS: TestSemanticStoreStatus_Unhealthy_DBErrorTextLooksLikeNilHandle  ← CR-02 regression guard
--- PASS: TestSemanticStoreStatus_Unhealthy_ProbeTimeout
PASS
```

Read-tier canary still passes (the new `BeginSnapshot` / `WriteSnapshotFacts`
/ `CommitSnapshot` calls live in package-level helper functions, NOT inside
any method on `*integSemanticLookup`):

```
$ go test ./internal/daemon/... -count=1 -run "TestIntegSemanticLookup_ReadTierCanary"
--- PASS: TestIntegSemanticLookup_ReadTierCanary
PASS
```

## Self-Check: PASSED

- `internal/daemon/integ_lookup_e2e_test.go` — FOUND
- `internal/daemon/integ_lookup_export_for_test.go` — FOUND
- `internal/skill/semantic/integration_test.go` — modified (FOUND)
- `internal/kernel/health/tools.go` — modified (FOUND)
- `internal/kernel/health/tools_semantic_test.go` — modified (FOUND)
- Commit `1ecf50ef` — FOUND
- Commit `8cb3c9b7` — FOUND
- Commit `437841b7` — FOUND
