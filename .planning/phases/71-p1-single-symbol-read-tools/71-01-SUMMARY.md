---
phase: 71-p1-single-symbol-read-tools
plan: 01
subsystem: semantic-skill, semantic-store
tags:
  - semantic
  - store-accessor
  - mcp-skill
  - seed-resolution
  - phase-71
  - wave-1
requires:
  - internal/semantic/store.Store (Phase 60+)
  - internal/skill/semantic.SemanticSkill scaffolding (Phase 64)
  - internal/semantic/integ.SymbolID type
provides:
  - "*Store.QuerySymbolByName(ctx,repoID,path,name) []string"
  - "*Store.LatestExtractorRunID(ctx,repoID) string"
  - SymbolByNameAccessor / ExtractorRunAccessor / ClusterMembershipAccessor narrow interfaces
  - "(*SemanticSkill).SetSymbolByName/SetExtractorRun/SetClusterMembership post-init setters"
  - SeedInput / Resolution / ResolvedSeed types + resolveSeed helper (D1 contract)
  - buildPopulatedGraphFixture multi-language fixture builder (Go/TS/Java)
affects:
  - .planning/phases/71-p1-single-symbol-read-tools/71-03-PLAN.md (consumer)
  - .planning/phases/71-p1-single-symbol-read-tools/71-04-PLAN.md (consumer)
  - .planning/phases/71-p1-single-symbol-read-tools/71-05-PLAN.md (consumer)
tech-stack:
  added: []
  patterns:
    - Lock-free *Store read accessor (Phase 69 D1)
    - Narrow accessor interface (Pattern F)
    - Post-init setter with sync.Mutex (Pattern G)
    - Closed-enum const block (Pattern I) for Resolution
    - TDD RED → GREEN cycle (test commit before implementation)
key-files:
  created:
    - internal/skill/semantic/seed_resolve.go
    - internal/skill/semantic/seed_resolve_test.go
    - internal/skill/semantic/populated_graph_fixture_test.go
  modified:
    - internal/semantic/store/effective_graph.go (added 2 accessors)
    - internal/semantic/store/effective_graph_test.go (added 9 tests + fmt import)
    - internal/skill/semantic/accessors.go (added 3 interfaces + integ import)
    - internal/skill/semantic/skill.go (added 3 fields + 3 setters)
decisions:
  - "extractor_run_id derivation: 'snap-<id>' rather than a new column (RESEARCH A4 — no backing column exists; signature stable for future replacement)"
  - "QuerySymbolByName ORDER BY stable_key ASC for deterministic ambiguity ordering"
  - "QuerySymbolByName LIMIT 6 so caller can detect '>5' before D1 cap of 5"
  - "resolveSeed short-circuits on SymbolID (no accessor call) — Pitfall 4 mitigation against lookup amplification"
  - "Fixture uses table-driven in-memory accessors (no *Store dependency) for race-clean skill-package isolation"
metrics:
  duration: "~30 minutes"
  completed: "2026-05-17"
---

# Phase 71 Plan 01: P1 Single-Symbol Read Tools — Seam Foundations Summary

Land the read-only seams the Phase 71 wave-2 handlers depend on: name-keyed symbol lookup on `*Store`, a derived extractor-run identifier, three narrow accessor interfaces, three post-init setters, the shared `resolveSeed` helper, and a multi-language populated-graph fixture reusable by every P1 tool test.

## What Was Built

Four atomic commits land the entire wave-1 surface for Phase 71:

| # | Commit  | Type    | Description                                                         |
| - | ------- | ------- | ------------------------------------------------------------------- |
| 1 | dd6cea19 | feat    | `QuerySymbolByName` + `LatestExtractorRunID` on `*Store` + 9 tests  |
| 2 | 53e5ce0e | feat    | Three narrow accessor interfaces + three `Set*` post-init setters   |
| 3 | ca9677a3 | test    | RED — failing tests for `resolveSeed` + multi-language fixture      |
| 4 | a4da920c | feat    | GREEN — `seed_resolve.go` (Resolution enum + SeedInput + helper)    |

## Task 1: `*Store` Accessors

**`QuerySymbolByName(ctx, repoID, path, name) ([]string, error)`**

- Reads at the latest committed snapshot via `LatestCommittedSnapshot`.
- JOINs `semantic_symbols` with `semantic_files` on `(snapshot_id, file_id)`; filters on `f.path = ? AND sym.name = ?`.
- `ORDER BY sym.stable_key ASC LIMIT 6` so the caller can detect `>5` matches and truncate to the D1 cap of 5.
- Empty repo / unknown name → `(nil, nil)`. Nil receiver → error. Real SQL errors wrapped with `fmt.Errorf("QuerySymbolByName(%q,%q): %w", path, name, err)`.

**`LatestExtractorRunID(ctx, repoID) (string, error)`**

- Returns `snap-<latest_committed_snapshot_id>` per RESEARCH A4 (no dedicated `extractor_run_id` column exists; this derivation is monotonic and signature-stable for future replacement).
- Empty repo → `("", nil)`. Nil receiver → error.

Both accessors are lock-free (no overlay tx, no Begin/Commit/Abort/Write tokens) and pass 9 new tests under `-race -count=1`:
exact match, ambiguous (3 candidates, deterministic ordering), cap-at-six, not-found, empty-repo, nil-store; plus extractor-run populated, empty-repo, nil-store.

## Task 2: Narrow Accessor Interfaces + Setters

Three new interfaces in `internal/skill/semantic/accessors.go`:

- `SymbolByNameAccessor.QuerySymbolByName(ctx, repoID, path, name string) ([]integ.SymbolID, error)`
- `ExtractorRunAccessor.LatestExtractorRunID(ctx, repoID string) (string, error)`
- `ClusterMembershipAccessor.ClusterIDOf(ctx, repoID, symbolID integ.SymbolID) (uint64, int, error)`

Three setters in `internal/skill/semantic/skill.go` following the existing `SetStore`/`SetScheduler` shape (`s.mu.Lock(); defer s.mu.Unlock(); s.field = a`). Three new private fields on `SemanticSkill`.

**CI grep-gate runner location:** **ABSENT.** No `.github/workflows/`, `scripts/`, or `Makefile` rule today greps for `Begin/Commit/Abort/Write` tokens against the `tools_refresh.go`-style read-only handlers. Plan 71-05 should add a Go-side gate test mirroring `tools_refresh_test.go:116-135` recorder pattern rather than extending an external runner.

## Task 3: `resolveSeed` Helper + Fixture (TDD)

**RED commit (ca9677a3):** Eight resolveSeed cases plus a multi-language fixture test. Build fails because `resolveSeed`, `SeedInput`, and `Resolution` are undefined.

**GREEN commit (a4da920c):** `internal/skill/semantic/seed_resolve.go` declares:

```go
type Resolution string
const (
    ResolutionExact     Resolution = "exact"
    ResolutionAmbiguous Resolution = "ambiguous"
    ResolutionNotFound  Resolution = "not_found"
)
type SeedInput   struct { SymbolID, FilePath, SymbolName string }
type ResolvedSeed struct {
    SymbolID            integ.SymbolID
    Resolution          Resolution
    AmbiguousCandidates []integ.SymbolID
}
func (s *SemanticSkill) resolveSeed(ctx, ws, in) (ResolvedSeed, error)
```

Contract: short-circuits on `SymbolID` (no accessor call — Pitfall 4 / T-71-01-04 mitigation); errors via `serr.InvalidArgs` when the (file_path, symbol_name) pair is incomplete; truncates `AmbiguousCandidates` to 5 (`ambiguousCap` constant).

**Fixture (`populated_graph_fixture_test.go`):** `buildPopulatedGraphFixture(t)` returns a `*PopulatedGraphFixture` with one Go symbol (`ServeHTTP`), one Go support symbol (`handle`), one Go type (`Request`), one TypeScript symbol (`fetchUser`) + type (`User`), and one Java method (`Foo.bar`) + class (`Bar`). Three accessor implementations satisfy the Phase 71 seam interfaces over a table-driven in-memory store (no `*Store` dependency — race-clean and platform-agnostic). Exported fields `GoSeedSymbolID`, `TSSeedSymbolID`, `JavaSeedSymbolID` let 71-03/04/05 tests reuse seed IDs without redeclaring them.

## Verification

- `go vet ./internal/semantic/store/ ./internal/skill/semantic/` — clean (no diagnostics).
- `go test ./internal/semantic/store/ ./internal/skill/semantic/ -race -count=1` — all green.
- Total new test cases: 9 (`*Store` accessors) + 8 (`resolveSeed`) + 1 (fixture) = **18**.
- No `Begin/Commit/Abort/Write` tokens introduced; all new code uses the read-only accessor path.

## Threat Model Realization

| Threat ID    | Mitigation in code                                                                                  |
| ------------ | --------------------------------------------------------------------------------------------------- |
| T-71-01-01   | `QuerySymbolByName` uses `?` placeholders only — no string concatenation of `path` or `name`        |
| T-71-01-02   | `LatestExtractorRunID` returns opaque `snap-<id>` — no PII; `read+` tier is the only consumer       |
| T-71-01-03   | `LIMIT 6` SQL cap + `ambiguousCap = 5` in `resolveSeed`                                             |
| T-71-01-04   | `resolveSeed` short-circuits on `SymbolID` and does NOT invoke `s.symbolByName` — lookup amplification impossible |

## Deviations from Plan

None. Plan executed exactly as written. The optional REFACTOR step in Task 3 was skipped because the GREEN implementation had no duplicated SQL-insert plumbing to extract — the fixture is intentionally table-driven and the helper is already minimal. Plan 71-04's fusion wiring can extend the fixture without re-touching this file.

## Open Questions Resolved by Execution

- **Open Question 1 (CI grep-gate runner location):** No external runner exists today. Plan 71-05 must add a Go-side gate test (recorder pattern). Documented in Task 2 commit message.
- **Open Question 2 (`ClusterMembershipAccessor` interface declaration vs implementation):** Interface lands here; 71-04 owns the production binding and decides on the cluster-boost-enabled vs cluster-boost-unavailable disposition. Fixture defaults to `(0, 0, nil)` for unknown symbols, modeling the disabled-boost path.

## Consumer Hooks

Wave-2 plans consume the seams declared here as follows:

- `71-03 explain_symbol_deep` → uses `resolveSeed` for the (file, name) input variant; uses `ExtractorRunAccessor` for the FreshnessV2 `extractor_run_id` field.
- `71-04 find_related_symbols` → uses `resolveSeed`; uses `ClusterMembershipAccessor` for the optional cluster co-membership boost (or emits `fallback_reason: "cluster_boost_unavailable"`).
- `71-05 validate_graph_edge` → uses `resolveSeed` for both `from` and `to` seeds; uses `ExtractorRunAccessor` for the envelope.

All three consumer plans can import `buildPopulatedGraphFixture(t)` and the shared accessor types without redeclaring or copying fixture logic.

## Self-Check: PASSED

- `internal/semantic/store/effective_graph.go` — FOUND (contains `QuerySymbolByName` + `LatestExtractorRunID`)
- `internal/semantic/store/effective_graph_test.go` — FOUND (9 new tests pass)
- `internal/skill/semantic/accessors.go` — FOUND (3 new interfaces)
- `internal/skill/semantic/skill.go` — FOUND (3 new fields + 3 new setters)
- `internal/skill/semantic/seed_resolve.go` — FOUND
- `internal/skill/semantic/seed_resolve_test.go` — FOUND (8 tests pass)
- `internal/skill/semantic/populated_graph_fixture_test.go` — FOUND (1 test passes, exports reused by future plans)
- Commits dd6cea19, 53e5ce0e, ca9677a3, a4da920c — all present in git log.
