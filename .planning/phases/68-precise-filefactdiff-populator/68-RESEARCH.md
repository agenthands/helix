# Phase 68: Precise FileFactDiff Populator — Research

**Researched:** 2026-05-13
**Domain:** Live-update pipeline; semantic store accessors; per-language extractor invocation; graph repair plumbing.
**Confidence:** HIGH (all canonical refs read; pre-edit data shape confirmed against schema; no new external dependencies introduced).

## Summary

Phase 68 fills the Tier-1/Tier-2 scaffolding left by Phase 62-09 in `internal/semantic/live/handler/difffacts.go`. The seam is already wired into `updateChangedFileWithKind`; what is missing is (a) a pre-edit `FileFact` accessor on `*store.Store`, (b) a synchronous per-language `ExtractFile(ctx, repoID, path) ExtractedFile` shim that reads the file from disk and delegates to the existing `provider.Extract`, (c) the actual symbol/edge diff against `priorFact.Symbols`, and (d) bounded-label outcome metrics on `*obs.Metrics`.

The work is plumbing — no new algorithms — but it spans three packages (`store`, `extract/{golang,typescript}`, `live/handler`) so task ordering and the vet-nokernel2semantic invariant matter. The handler today reaches `populateRecorderForFile` (difffacts.go:35) which fans out to `tryFullDiff` / `tryAddedOnlyDiff` (both hard-coded `return false`) before falling through to the Tier-3 `RecordSymbolChanged{KindChanged:true}` marker. Phase 68 turns the two `false`s into real implementations.

**Primary recommendation:** Drive the work in this order: (1) `*Store.GetLatestFileFact` accessor + tests, (2) per-language `ExtractFile` shim (Go + TS only — others remain partial-only), (3) symbol-and-edge diff functions in `difffacts.go`, (4) wire Handler dependency injection for store + extract.Registry, (5) outcome metric registration on `obs.Metrics`, (6) `handler_diff_e2e_test.go` exercising the real path end to end. Steps 1–3 are independent and can land in any order; step 4 is the integration gate.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01: New `*Store.GetLatestFileFact` accessor.** Lives on `internal/semantic/store.Store`. Reads overlay first, snapshot fallback. Returns `(_, false, nil)` on cold-start. Errors only on DB I/O failure.
- **D-02: Vet-boundary invariant.** Accessor in `internal/semantic/store`. Handler call site already in `internal/semantic/live/handler`. No new `internal/kernel/*` imports. `make vet` (which runs `vet-nokernel2semantic`) MUST stay green; plan MUST include a verification step.
- **D-03: Synchronous per-file `ExtractFile(ctx, repoID, path)` shim.** Added on the per-language provider surface (`internal/semantic/extract/{golang,typescript,...}`). Handler invokes it inside `updateChangedFileWithKind` after `UpsertOverlayFile`, before `tx.Commit()`. Result drives Tier-1/2 dispatch.
- **D-04: Same policy for HelixEdit lane and background lanes.** No hybrid sync/async split.
- **D-05: Tier mapping.** Tier-1 = prior fact found AND `ExtractionStatusReady` → full diff. Tier-2 = `ExtractionStatusPartial` → added-only. Tier-3 = prior missing OR `ExtractionStatusFailed` / `Unsupported` → synthetic marker + warn metric.
- **D-06: `graph_version` contract preserved.** Recorder non-empty in all three tiers. `BumpGraphVersion` single call site (`apply_repair.go`) untouched.
- **D-07: Single outcome counter** `helix_live_filefactdiff_total{tier,repo}` with bounded labels (3 tiers × N repos).
- **D-08: Tier-3 warn distinction.** Secondary metric `helix_live_filefactdiff_synthetic_reason_total{reason ∈ {cold_start, extract_failed, extract_unsupported}}`.
- **D-09: E2E test shape.** `internal/semantic/live/handler/handler_diff_e2e_test.go`, Go-only fixture, edit one exported function, assert `RecordSymbolChanged` fires exactly once + `ApplyRepair` called with non-empty `GraphRepair` + outcome metric records `tier="full"`. `-race` clean by construction.
- **D-10: Test seam reuse.** `SetPopulateRecorderForTest` is for unit tests injecting fakes. The E2E test exercises the REAL populator.
- **D-11: DEF-67-F01-FULL-DIFF closure.** Mark resolved in `.planning/deferred-items.md` after Phase 68 lands.

### Claude's Discretion

- Exact column shape returned by `GetLatestFileFact` (whether it returns `extract.FileFact` directly or a store-side `FileFactRow`). Planner/researcher to choose what makes the diff call site cleanest.
- Diff algorithm internals — symbol matching by `stable_key`, edge matching by `(src,dst,kind)`. Researcher to confirm against existing `graphpkg.SymbolDiff` / `GraphEdge` types.
- Exact metric registration site (alongside `apply_repair` metrics vs new `difffacts.go` registration block).
- Per-language extractor shim implementation: whether `ExtractFile` wraps the existing batch extractor or is a parallel code path. Planner to choose smallest patch surface.

### Deferred Ideas (OUT OF SCOPE)

- Handler-local LRU cache in front of `GetLatestFileFact`.
- TypeScript E2E fixture (Go-only this phase).
- Hybrid sync/async lane policy.
- Concurrent same-file edit subtest.
- Multi-projection PageRank metric label on the outcome counter.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **DIFF-01** | Tier-1 full diff populator active in production; recorder receives per-symbol added/removed/changed + per-edge added/removed against pre-edit FileFact. | Symbol diff by `SymbolFact.ID` (= `xxhash64(StableKey)`) which equals `graphpkg.NodeID` per `semantic_wiring.go:1626` ("per-symbol NodeID = SymbolID"). Bit-flags `SignatureChanged` from `SignatureHash` delta, `ExportedChanged` from `Visibility` delta, `KindChanged` from `Kind` delta, `StableKeyChanged` only when `Name`+`Kind` match but key differs (collision/rename heuristic). See § Architecture Patterns Pattern 1. |
| **DIFF-02** | Pre-edit FileFact accessor on `*Store` (or live-handler-local cache) readable from post-commit hook without violating vet-nokernel2semantic. | D-01 chooses `*Store` accessor. Boundary verified: `internal/semantic/store` → `internal/semantic/live/handler` is semantic→semantic, unaffected by `nokernel2semantic.Analyzer` (which only fires when `internal/kernel/*` imports `internal/semantic/*`). See § Vet Boundary. |
| **DIFF-03** | `graph_version` advances on every live edit AND ApplyRepair receives non-empty per-symbol deltas; verified by `handler_diff_e2e_test.go`. | Existing post-commit hook in `handler.go:401-429` already calls `ApplyRepair` when `!recorder.IsEmpty()`. With Tier-1 active the recorder accumulates real `SymbolDiff` entries → `ComputeGraphRepair` produces non-empty `DirtyNodes` → `ApplyRepair` fires → `tx.BumpGraphVersion(ctx)` (apply_repair.go:255). E2E asserts the full chain. |
| **DIFF-04** | Tier-2 (added-only) fallback exercised when extractor returns partial extraction. Behavior is graceful degrade, not error. | `ExtractedFile.File.ExtractionStatus == ExtractionStatusPartial` triggers `tryAddedOnlyDiff`. For every `s` in `newFact.Symbols`, call `RecordSymbolAdded(SymbolDiff{NodeID: s.ID, KindChanged: true})` (KindChanged satisfies the D-06 graph-changing requirement in repair.go:106). |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Pre-edit FileFact retrieval | `internal/semantic/store` (semantic side) | — | Single source of truth (overlay + snapshot tables); survives daemon restart; matches existing `Store.Begin*/Write*/Latest*` API surface. |
| Per-file synchronous extraction | `internal/semantic/extract/{lang}` (provider side) | `internal/semantic/extract` (Provider interface widening) | Existing `Extract(ctx, source, SourceFile)` already exists per language; the shim wraps it with a file-read step. Keeping it on the provider keeps language-specific concerns (path → source bytes, partial classification) inside the language package. |
| Diff computation | `internal/semantic/live/handler` (difffacts.go) | `internal/semantic/graph` (consumes `SymbolDiff`/`GraphEdge`) | The diff is consumer-side (the recorder is owned by the live handler tx). graphpkg owns the *meaning* of `SymbolDiff` bit-flags; the diff computation is a one-pass walk that lives where the recorder lives. |
| Recorder seam | `internal/semantic/live/handler` | — | Already shipped in 62-09 (`FileFactDiffRecorder` + `populateRecorderForFile`). No change. |
| Outcome metric | `internal/obs/metrics.go` (definition) + `internal/semantic/live/handler/difffacts.go` (emission) | — | Mirrors existing `SemanticLiveUpdatesInc` / `SemanticGraphRepairInc` pattern: counter defined and registered in `obs.Metrics`, emission helper with closed-enum guard, drop-on-unknown discipline. |
| E2E test fixture | `internal/semantic/live/handler/handler_diff_e2e_test.go` | — | Production code paths must be the path under test (D-10). |

## Standard Stack

### Core (already in tree — no new deps)

| Package | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/semantic/store` | in-tree | `Store.GetLatestFileFact` accessor; DuckDB-backed overlay+snapshot tables | Existing seam: `Begin*/Write*/Latest*` method family ([VERIFIED: `internal/semantic/store/overlay.go:90`, `:983`, `:1019`; `effective_graph.go:208 LatestCommittedSnapshot`]). |
| `internal/semantic/extract` | in-tree | Provider interface (Language/Extensions/Extract/...) | Already widened to expose `Extract` polymorphically (Phase 65 D-06, `provider.go:41-49` [VERIFIED: read 2026-05-13]). Adding `ExtractFile` is the next contract widening on the same interface. |
| `internal/semantic/graph` | in-tree | `SymbolDiff` + `GraphEdge` types; `ComputeGraphRepair` | `SymbolDiff.NodeID` is keyed on the stable-key-derived hash that equals `extract.SymbolFact.ID` ([VERIFIED: `semantic_wiring.go:1626` "per-symbol NodeID = SymbolID"]). |
| `internal/semantic/live/handler` | in-tree | `FileFactDiffRecorder`, `populateRecorderForFile`, Tier-1/2/3 dispatch | 62-09 closure shipped the seam; Phase 68 fills `tryFullDiff` + `tryAddedOnlyDiff`. |
| `internal/obs` (`*obs.Metrics`) | in-tree | Prometheus CounterVec definitions + bounded-label emission helpers | Existing pattern: `SemanticLiveUpdates`, `SemanticGraphRepairVec`, `EditOutcome` [VERIFIED: `internal/obs/metrics.go:322, 399, 290`]. |

### Supporting

| Package | Purpose | When to Use |
|---------|---------|-------------|
| `xxhash` (via `extract.StableSymbolID`) | Compute `SymbolFact.ID` from `StableKey` | Already used by every provider; nothing new here. |
| `database/sql` + DuckDB driver | `Store.GetLatestFileFact` SQL execution | Mirror `effective_graph.go:208` `LatestCommittedSnapshot` and `overlay.go:983` `CurrentGraphVersion` patterns. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `*Store.GetLatestFileFact` accessor | Handler-local LRU cache | Rejected by D-01 — cache would be cold after every daemon restart, defeating Tier-1 on the first edit per file post-restart. The Store accessor is authoritative; cache can be layered behind the same signature later. |
| `ExtractFile(ctx, repoID, path)` on provider | Scheduler-driven async extract + post-commit recorder population | Rejected by D-03 — async breaks the "graph_version advances per edit" invariant and the recorder is tx-scoped (must be populated before `tx.Commit()`). |
| New `extract.Provider.ExtractFile` interface method | Free function `extract.ExtractFile(provider, ...)` taking the provider as argument | Interface method is cleaner — the per-language file-read + partial classification (e.g., TS `.d.ts` skipping) belongs in the language package. |
| Returning `extract.FileFact` (extraction-status row) | Returning a synthesized `priorFact` struct carrying `Symbols []SymbolFact` + `Edges []GraphEdge` | **Strong recommendation: return a synthesized priorFact carrying symbols (and optionally edges).** The "FileFact" name in the seam contract is misleading — `extract.FileFact` (fact.go:164) is only the per-file *extraction status* row; it does NOT carry symbols. The diff needs `Symbols`. The cleanest signature is `GetLatestFileFact(ctx, repoID, path) (priorFact, found bool, err error)` where `priorFact` is a new store-side struct exposing `Symbols []extract.SymbolFact` + (later) `Edges`. See § Open Questions Q1. |

**Installation:** No new packages; all dependencies in tree.

**Version verification:** N/A (in-tree only).

## Architecture Patterns

### System Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────────┐
│  Live edit event arrives (ChangeHelixEdit | ChangeFileModified | ...)  │
└────────────────────────────┬───────────────────────────────────────────┘
                             │
                             ▼
                   Handler.Dispatch(ev)
                             │
                             ▼
            Handler.updateChangedFileWithKind(...)         [handler.go:348]
                             │
              ┌──────────────┼───────────────┐
              │              │               │
              ▼              ▼               ▼
       hash file        BeginOverlayTx   GetLatestFileFact      ◄── NEW (D-01)
                                          (read overlay
                                           then snapshot)
              │              │               │
              └──────────────┘               │
                             ▼               │
              tx.UpsertOverlayFile           │
                             │               │
                             ▼               │
                    provider.ExtractFile     │  ◄── NEW (D-03)
                    (read source bytes,      │
                     run tree-sitter)        │
                             │               │
                             ▼               ▼
              ┌──────────────────────────────────────────┐
              │   populateRecorderForFile (difffacts.go) │
              │   ┌──────────────────────────────────┐   │
              │   │  Tier select:                    │   │
              │   │   prior FOUND + ready  → Tier 1  │   │
              │   │   status == partial    → Tier 2  │   │
              │   │   else (cold/failed/   → Tier 3  │   │
              │   │       unsupported)               │   │
              │   └──────────────────────────────────┘   │
              │                                          │
              │  Tier 1: diffSymbols(prior, new)         │
              │           → RecordSymbolAdded/           │
              │             Removed/Changed              │
              │          diffEdges(prior, new)           │
              │           → RecordEdgeAdded/Removed      │
              │                                          │
              │  Tier 2: for s in new.Symbols:           │
              │            RecordSymbolAdded(s)          │
              │                                          │
              │  Tier 3: RecordSymbolChanged(            │
              │            KindChanged:true) +           │
              │          synthetic_reason metric         │
              │                                          │
              │  emit helix_live_filefactdiff_total      │
              │    {tier, repo}                          │
              └──────────────────────────────────────────┘
                             │
                             ▼
                       tx.Commit()
                             │
                             ▼
                 recorder.IsEmpty()? ──Yes──► once-INFO log (unreachable
                             │                in production after Phase 68
                             No               since all tiers populate)
                             ▼
              repair = ComputeGraphRepair(recorder.Snapshot())
                             │
                             ▼
                 engine.ApplyRepair(...)  → bumps graph_version
                             │
                             ▼
                  LSPQueue.EnqueueLane(...)
```

### Recommended Project Structure

```
internal/
├── semantic/
│   ├── store/
│   │   ├── overlay.go              # existing; no change
│   │   ├── snapshot.go             # existing; no change
│   │   ├── filefact_accessor.go    # NEW: GetLatestFileFact + tests
│   │   └── filefact_accessor_test.go
│   ├── extract/
│   │   ├── provider.go             # MODIFIED: widen Provider with ExtractFile
│   │   ├── golang/
│   │   │   ├── provider.go         # MODIFIED: add (p *Provider) ExtractFile(...)
│   │   │   └── provider_extract_file_test.go  # NEW
│   │   └── typescript/
│   │       ├── provider.go         # MODIFIED: same shape
│   │       └── provider_extract_file_test.go
│   └── live/
│       └── handler/
│           ├── handler.go          # MODIFIED: inject store + extract.Registry; wire ExtractFile
│           ├── difffacts.go        # MODIFIED: fill tryFullDiff + tryAddedOnlyDiff; add diffSymbols/diffEdges
│           ├── difffacts_test.go   # NEW: unit tests for diff helpers
│           └── handler_diff_e2e_test.go  # NEW (D-09 spec)
└── obs/
    └── metrics.go                  # MODIFIED: add LiveFileFactDiffVec + helper
```

### Pattern 1: Symbol diff by stable-key-derived NodeID

**What:** Match prior↔new symbols by `SymbolFact.ID` (= `xxhash64(StableKey)`). The same value is `graphpkg.NodeID` for live overlay rows (per `semantic_wiring.go:1626`).

**When to use:** Tier-1 full diff.

**Algorithm:**

```go
// Source: derived from internal/semantic/graph/repair.go SymbolDiff fields
// + internal/semantic/extract/fact.go SymbolFact fields.
func diffSymbols(prior, new []extract.SymbolFact, rec *FileFactDiffRecorder) {
    priorByID := make(map[semantic.SymbolID]extract.SymbolFact, len(prior))
    for _, s := range prior {
        priorByID[s.ID] = s
    }
    newByID := make(map[semantic.SymbolID]extract.SymbolFact, len(new))
    for _, s := range new {
        newByID[s.ID] = s
    }

    // Removed: in prior, not in new.
    for id, p := range priorByID {
        if _, stillThere := newByID[id]; !stillThere {
            rec.RecordSymbolRemoved(graphpkg.SymbolDiff{
                NodeID: graphpkg.NodeID(id),
                // RemovedSymbols always invalidate per ComputeGraphRepair
                // (repair.go:100) — no flag set needed.
            })
        }
    }

    // Added: in new, not in prior.
    for id, n := range newByID {
        if _, wasThere := priorByID[id]; !wasThere {
            rec.RecordSymbolAdded(graphpkg.SymbolDiff{
                NodeID:      graphpkg.NodeID(id),
                KindChanged: true, // any flag is sufficient (repair.go:114 ignores flags for adds)
            })
        }
    }

    // Changed: in both — compute bit-flags.
    for id, n := range newByID {
        p, ok := priorByID[id]
        if !ok {
            continue
        }
        d := graphpkg.SymbolDiff{NodeID: graphpkg.NodeID(id)}
        if p.SignatureHash != n.SignatureHash {
            // SignatureHash mixes body content for Go (signatureHash() in
            // golang/provider.go:379). Use a separate signal if you need
            // to distinguish body-only changes.
            d.SignatureChanged = true
        }
        if p.Visibility != n.Visibility {
            d.ExportedChanged = true
        }
        if p.Kind != n.Kind {
            d.KindChanged = true
        }
        if p.StableKey != n.StableKey {
            // Defensive: should be unreachable when ID matches (ID is
            // derived from StableKey). Surfaces hash collisions in tests.
            d.StableKeyChanged = true
        }
        if d.SignatureChanged || d.ExportedChanged || d.KindChanged || d.StableKeyChanged {
            rec.RecordSymbolChanged(d)
        } else {
            // Body-only — D-06 explicitly excludes these from graph-
            // changing. Don't record.
        }
    }
}
```

### Pattern 2: Edge diff by (src, dst, kind) tuple

**What:** Edges identified by the tuple `(SrcNodeID, DstNodeID, EdgeKind)`. In Phase 68 the canonical edge source is the live overlay's `semantic_live_overlay_edges` table; the *new* edges come from `ExtractedFile.References` (CALL / USES_TYPE / ...). The mapping from `ReferenceFact` to `GraphEdge` is not yet wired in the live path — Phase 62 P05's type resolver is deferred, so Phase 68's edge diff is conservatively scoped to **edges whose endpoints exist in the file under edit**.

**When to use:** Tier-1 full diff. For Phase 68 the recommendation is to start with **symbols only** and emit an empty edge diff (no `RecordEdgeAdded`/`RecordEdgeRemoved`), so DIFF-01's "per-edge add/remove against the pre-edit FileFact" is read as "the contract is hooked up; production edge diff lands when type-resolver retrofit fires." The plan-checker should flag if the PLAN claims live edge diffing without an extracted-edges pipeline.

**Alternative if edges in scope:** Match by `(SrcNodeID, DstNodeID, EdgeKind)`. Iterate prior edges, drop any (src,dst,kind) tuple absent in new — that's `RecordEdgeRemoved`. Iterate new edges, add tuples absent in prior — that's `RecordEdgeAdded`. The store has `IterateCommittedSymbols` and similar; an edge-iterator on overlay tables would need to be added (out of Phase 68 scope per D-11 deferred-items hygiene).

### Pattern 3: Pre-edit FileFact accessor (D-01 shape recommendation)

**What:** `*Store.GetLatestFileFact` returns a synthesized struct (not `extract.FileFact`) carrying the symbols needed for the diff.

**Why a synthesized struct (not `extract.FileFact`):**

- `extract.FileFact` (`fact.go:164`) only carries `ExtractionStatus` + extractor metadata — no symbols.
- `store.FileFact` (`snapshot.go:131`) carries persistent identity (FileID, RepoID, ...) but not symbols either.
- The diff *requires* the symbol list. The accessor must surface it.

**Proposed shape:**

```go
// internal/semantic/store/filefact_accessor.go
package store

import (
    "context"

    "github.com/agenthands/helix/internal/semantic/extract"
)

// PriorFileFact is the pre-edit fact snapshot the live handler diffs against.
// Returned by GetLatestFileFact. Carries the symbols and the extraction
// status of the source row so the handler can decide Tier-1 vs Tier-2
// without a second query.
type PriorFileFact struct {
    Path             string
    Language         string
    ExtractionStatus extract.ExtractionStatus
    Symbols          []extract.SymbolFact
    // Edges []graphpkg.GraphEdge // deferred — see Pattern 2 note.
}

// GetLatestFileFact returns the latest pre-edit fact for (repoID, path).
// Reads semantic_live_overlay_symbols first; on miss, falls back to the
// latest committed snapshot via semantic_files + semantic_symbols.
//
// Returns (zero, false, nil) on cold-start (no prior fact). Errors surface
// only for DB I/O failures.
func (s *Store) GetLatestFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
    // 1. Check semantic_live_overlay_files for (repoID, path); if status='deleted'
    //    OR row absent, fall through to snapshot.
    // 2. If live row exists: SELECT * FROM semantic_live_overlay_symbols
    //    WHERE repo_id=? AND file_id=? AND status='live'
    //    → hydrate SymbolFact via fact_json JSON column.
    // 3. Snapshot fallback: latest snapshot_id from LatestCommittedSnapshot;
    //    JOIN semantic_files (path → file_id) + semantic_symbols.
    //    Note: snapshot SymbolFact has different shape (store.SymbolFact);
    //    a conversion is needed to extract.SymbolFact (see § Open Questions Q2).
    //
    // Implementation lives in internal/semantic/store and is the only
    // mutation to that package required by Phase 68.
}
```

**[ASSUMED]:** `semantic_live_overlay_symbols.fact_json` carries the full `extract.SymbolFact` JSON. Schema column exists (migrations.go:323 `fact_json JSON`) but the *writer* path (live overlay's symbol-write surface) is not yet implemented — Phase 60 P04 was scaffolded then partially deferred. The plan MUST verify the writer side or treat the overlay branch as "future capability" and ship Phase 68 reading snapshot-only first. See § Open Questions Q3.

### Pattern 4: Per-language ExtractFile shim

**What:** Widen `extract.Provider` (specifically `ExtractionPipeline` sub-interface) with a new method that takes a path and reads the file internally.

**Why over wrapper-in-handler:**

- Path → source bytes → `provider.Extract` is repeated 3× already in the codebase (`semantic_wiring.go:1587`, scheduler batch flow, and Phase 68 needs a 4th caller).
- Per-language *partial-skip* logic (e.g., TypeScript skipping `.d.ts` ambient declarations) lives in the language package; the shim keeps it there.
- The `Extract` signature is already widened to interface-level (Phase 65 D-06); adding `ExtractFile` is a parallel widening.

**Proposed shape:**

```go
// internal/semantic/extract/provider.go (modification)
type ExtractionPipeline interface {
    Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)

    // ExtractFile reads `path` from disk and returns the per-file
    // ExtractedFile. Best-effort: I/O failures and per-file partials are
    // surfaced via ExtractedFile.File.ExtractionStatus rather than as
    // returned errors. Error is reserved for unrecoverable extraction-
    // engine failures (matching Extract's contract).
    //
    // Phase 68 D-03: synchronous in-tx invocation from the live handler.
    ExtractFile(ctx context.Context, repoID, path string) (*ExtractedFile, error)
}
```

**Per-provider implementation** (the minimum patch):

```go
// internal/semantic/extract/golang/provider.go (addition)
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
    source, err := os.ReadFile(path)
    if err != nil {
        // Per-file I/O failure → partial with reason=permission_denied or
        // file_too_large depending on errno. Mirrors partialFile() helper
        // in this file at line 302.
        return partialFile(extract.SourceFile{Path: path, Language: "go"},
            extract.PartialReasonPermissionDenied, err), nil
    }
    return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "go"})
}
```

`repoID` is plumbed through the signature even though the current Go/TS providers don't need it, because:

1. D-01's `*Store.GetLatestFileFact(ctx, repoID, path)` uses it on the other side of the seam, so the handler call site is symmetric.
2. Future providers (Python module-resolution, Java classpath) may need workspace context.
3. Cost is zero — the parameter is ignored when unused.

### Anti-Patterns to Avoid

- **Reading source from the handler then calling `provider.Extract` directly.** Duplicates the file-read pattern across 4 sites and ties handler code to language-specific partial-skip logic.
- **Returning `extract.FileFact` from `GetLatestFileFact`.** That struct carries no symbols; the diff can't use it. Either return a synthesized struct (recommendation) or pair it with a second symbols accessor — but two-call-site is a worse seam.
- **Synthesizing `RecordSymbolAdded` for unchanged-but-present symbols in Tier-1.** A no-op Tier-1 (no symbols actually changed) MUST leave the recorder empty so the once-INFO path is preserved AND `ApplyRepair` doesn't fire spuriously. Verified by reading `ComputeGraphRepair` (repair.go:114) — added symbols always go to `DirtyNodes`, so adding a still-present symbol would falsely advance graph_version.
- **Calling `BumpGraphVersion` from the new code.** It is the D-06 single call site in `apply_repair.go:255` and Phase 68 MUST NOT add a second.
- **Importing anything under `internal/kernel/*` from the new code.** Phase 68 touches semantic-side only; `vet-nokernel2semantic` enforces this.
- **Using `provider.Extract` from inside `store` package.** The accessor reads pre-edit state only; it never re-extracts. Calling Extract from store would invert the dependency arrow.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Symbol stable identity | Custom string match on name+kind+line | `SymbolFact.ID` (= `extract.StableSymbolID(StableKey)`) | Already xxhash64 of canonicalized key per SPEC §11.1; handles overloads, generics, receivers correctly. Matching by name is wrong (overloads collide). |
| Diff dispatch | `if/else` ladder against `ExtractionStatus` strings | Closed enum constants (`ExtractionStatusReady/Partial/Failed/Unsupported`) | Already in `fact.go:34-38`; switch on enum, default branch fires Tier-3. |
| Edge identity | `fmt.Sprintf("%d-%d-%s")` lookup keys | Struct-keyed map `map[edgeKey]struct{}` with `edgeKey = struct{Src,Dst NodeID; Kind string}` | Same as the graph layer's existing edge handling; zero allocations on the hot path. |
| Tier-3 once-warn gating | Per-call `sync.Once` allocation | Existing `Handler.emptyDiffOnce(repoID, fn)` pattern OR a new `synthRsnOnces sync.Map` mirroring it | Phase 62-09 already established the pattern in `handler.go:251`; reuse the shape. (Tier-3 metric is bounded by `reason` enum so per-repo once-gating is optional; recommend metric-only, no log spam.) |
| Outcome metric registration | New top-level metric registry | Add `LiveFileFactDiffVec *prometheus.CounterVec` to `obs.Metrics` struct | Mirrors `SemanticLiveUpdates` field exactly (metrics.go:103). One field add + one helper method + one register call. |
| Bounded-label enforcement | Free-form label strings | Closed-enum switch in helper (drop-on-unknown) | The existing `SemanticLiveUpdatesInc` (metrics.go:704) is the load-bearing pattern; copy the shape. |
| Reading source bytes for extract | `os.ReadFile` inside the handler | New `Provider.ExtractFile` method | Per-language packages own partial-skip / size-limit / encoding policy. |

**Key insight:** Every piece of Phase 68 plumbing already has a precedent within 2 files of the new code. The work is *recombining* existing pieces, not inventing new patterns. If the plan introduces a novel structure (e.g., a per-handler symbol cache, a parallel metric system, a new diff algorithm), that's a smell — push back.

## Runtime State Inventory

> N/A for this phase. Phase 68 ships new code paths and a metric; it does not rename a string, refactor a stored identifier, or migrate stored data.
>
> - Stored data: None — `semantic_live_overlay_*` and `semantic_*` snapshot tables are READ from; no schema change required.
> - Live service config: None — no n8n / Datadog / Tailscale / Cloudflare integrations involved.
> - OS-registered state: None — no scheduled tasks / pm2 / systemd registrations affected.
> - Secrets/env vars: None — no new credentials introduced.
> - Build artifacts: Standard `go build` rebuild on source change suffices; no installed package caches with old names.

## Common Pitfalls

### Pitfall 1: Misusing `extract.FileFact` as a symbol carrier

**What goes wrong:** Plan attempts to make `GetLatestFileFact` return `extract.FileFact` (because the name matches CONTEXT.md's signature sketch). The struct carries no symbols, so the diff has nothing to diff against. Tier-1 silently degrades to Tier-2-via-fallthrough.

**Why it happens:** Naming collision — CONTEXT.md says "`GetLatestFileFact(...) (extract.FileFact, ...)`" verbatim. The name "FileFact" is overloaded: `extract.FileFact` is the per-file *status* row; `store.FileFact` is the snapshot *identity* row; neither carries symbols.

**How to avoid:** D-01 has "Claude's Discretion" specifically on the return shape. The planner SHOULD diverge from the CONTEXT.md sketch and return a new `store.PriorFileFact` (or equivalent) carrying `Symbols []extract.SymbolFact`. Document the divergence in the PLAN and the SUMMARY.

**Warning signs:** `tryFullDiff` body references `priorFact.Symbols` but the returned type doesn't have that field → compile error before tests run.

### Pitfall 2: Overlay-symbol-write path not populating `fact_json`

**What goes wrong:** `GetLatestFileFact` reads `semantic_live_overlay_symbols.fact_json` and finds it empty / `NULL` because the live overlay writer (Phase 60 P02 only ships file-row upserts; Phase 60 P04 — full symbol upsert — is partially deferred). Result: overlay branch returns zero symbols even when the file has been edited live, falsely triggering snapshot fallback or cold-start.

**Why it happens:** The overlay symbol schema exists (`migrations.go:315 CREATE TABLE semantic_live_overlay_symbols`), but the writer side may not yet stamp `fact_json`. Verify before implementing.

**How to avoid:**
1. Wave 0 verification: query `SELECT COUNT(*) FROM semantic_live_overlay_symbols WHERE fact_json IS NOT NULL` against a dev DB after a live edit. If zero, the overlay branch is stub-only.
2. If overlay branch is unreachable in practice, the Phase 68 implementation MAY ship snapshot-only and document the overlay branch as "future capability" — Tier-1 still works (snapshot is authoritative for committed state), and the only loss is Tier-1 on rapid-fire edits to the same file within one snapshot window.

**Warning signs:** E2E test passes with snapshot fixture but fails on a second-edit-of-same-file scenario.

### Pitfall 3: SignatureHash semantics for Go include body bytes

**What goes wrong:** `golang/provider.go:379 signatureHash` returns a hash of `body.Utf8Text(source)` where `body` is the symbol body node — NOT just the signature. Body-only edits will flip `SignatureHash` and the diff will (incorrectly) emit `SignatureChanged: true`, advancing `graph_version` on body-only edits and breaking the D-06 invariant ("BodyOnlyChanged WITHOUT any other flag → not graph-changing").

**Why it happens:** Provider field is named `SignatureHash` but is actually a *content hash* covering the body. The graph layer expects `SignatureChanged` to mean "the externally-visible signature changed" which would NOT advance for body-only edits.

**How to avoid:** Compare `Signature` (the condensed text) directly, not `SignatureHash`. Or have the Go provider expose a separate `SignatureBytesHash` (signature line only) and a `BodyHash` so the diff can distinguish. Phase 68 takes the simpler path: **compare `p.Signature != n.Signature` for `SignatureChanged`, and additionally compute a body-content hash (or just diff `p.SignatureHash != n.SignatureHash`) for "body changed but signature same" → no flag set, no graph_version advance.**

**Warning signs:** Add a test where a function body changes but the signature line is byte-identical; assert `ApplyRepair` is NOT called. If this test fails, the diff is over-eager.

**Reference:** `internal/semantic/extract/golang/provider.go:257-258`:
```go
sf.SignatureHash = signatureHash(*body, source)
sf.Signature = condenseWhitespace(body.Utf8Text(source))
```
Both fields mix body content; `SignatureHash` is a strict subset (hash of what `Signature` truncates).

### Pitfall 4: Forgetting to invalidate `graph_version` advance contract

**What goes wrong:** Tier-2 (added-only) runs against a non-empty prior fact and synthesizes `RecordSymbolAdded` for every symbol — including symbols that are unchanged from the prior state. `ComputeGraphRepair` (repair.go:114) adds every "added" symbol to `DirtyNodes`. Result: every edit on a `partial`-extracted file invalidates the entire file's rank rows, even for body-only changes.

**Why it happens:** Tier-2 is the "no prior to compare against" fallback by D-05's letter; in practice, if prior IS present but extractor is partial, the implementation might still treat it as "no prior" for simplicity.

**How to avoid:** D-05 is explicit: Tier-2 fires on `ExtractionStatusPartial` *regardless of prior*. If a prior fact exists AND extractor returned partial, the safe behavior is still "added-only" (recording every new symbol as added) because partial extraction means we can't trust the new fact's completeness — we have to assume what's there is new. The over-advance of `graph_version` is the documented price of partial extraction; the alternative (silently dropping deletions) is worse.

**Warning signs:** Operators report excessive ApplyRepair calls correlated with `helix_live_filefactdiff_total{tier="added-only"}` rate.

### Pitfall 5: Test fixture relies on real LSP / file watcher

**What goes wrong:** `handler_diff_e2e_test.go` tries to bootstrap a workspace with a real LSP server, real fsnotify watcher, and real scheduler. Test becomes flaky on CI (LSP startup latency, file-system event ordering).

**Why it happens:** D-09 calls for "real populator path" — easy to misread as "real everything."

**How to avoid:** The "real populator path" means the production `populateRecorderForFile` → `tryFullDiff` chain executes (not `SetPopulateRecorderForTest`). It does NOT mean real LSP / real watcher. Use the existing `noop_test.go` `fakeStore`/`fakeTx` pattern (handler_test.go-style) but provide a real `Store` + real `extract.Registry` so the new accessor and the new shim run for real. RankApplier stays a `recordingRankApplier` (recorder_test.go:23). No LSP, no fsnotify.

**Warning signs:** Test imports `lspclient` or `fsnotify` → red flag.

## Code Examples

### Example 1: GetLatestFileFact accessor skeleton

```go
// Source: derived from internal/semantic/store/effective_graph.go:208
// (LatestCommittedSnapshot) and internal/semantic/store/overlay.go:983
// (CurrentGraphVersion) — same Store.db.QueryRowContext pattern.
package store

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    "github.com/agenthands/helix/internal/semantic/extract"
)

type PriorFileFact struct {
    Path             string
    Language         string
    ExtractionStatus extract.ExtractionStatus
    Symbols          []extract.SymbolFact
}

func (s *Store) GetLatestFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
    if s == nil || s.db == nil {
        return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: nil store")
    }

    // Overlay branch.
    if fact, ok, err := s.readOverlayFileFact(ctx, repoID, path); err != nil {
        return PriorFileFact{}, false, err
    } else if ok {
        return fact, true, nil
    }

    // Snapshot fallback.
    return s.readSnapshotFileFact(ctx, repoID, path)
}

func (s *Store) readOverlayFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
    // 1. Resolve file_id from semantic_live_overlay_files (status='live').
    var fileID uint64
    var lang string
    err := s.db.QueryRowContext(ctx, `
        SELECT file_id, language FROM semantic_live_overlay_files
        WHERE repo_id = ? AND path = ? AND status = 'live'
    `, repoID, path).Scan(&fileID, &lang)
    if err == sql.ErrNoRows {
        return PriorFileFact{}, false, nil
    }
    if err != nil {
        return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: overlay file lookup: %w", err)
    }
    if fileID == 0 {
        // 60 P02 stub case — overlay row exists but is the placeholder
        // (file_id=0). Fall through to snapshot.
        return PriorFileFact{}, false, nil
    }

    // 2. Load symbols from semantic_live_overlay_symbols via fact_json.
    rows, err := s.db.QueryContext(ctx, `
        SELECT fact_json FROM semantic_live_overlay_symbols
        WHERE repo_id = ? AND file_id = ? AND status = 'live'
    `, repoID, fileID)
    if err != nil {
        return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: overlay symbols: %w", err)
    }
    defer rows.Close()

    var symbols []extract.SymbolFact
    for rows.Next() {
        var raw sql.NullString
        if err := rows.Scan(&raw); err != nil {
            return PriorFileFact{}, false, err
        }
        if !raw.Valid || raw.String == "" {
            continue
        }
        var sf extract.SymbolFact
        if err := json.Unmarshal([]byte(raw.String), &sf); err != nil {
            // Per-symbol unmarshal failure is non-fatal; skip and continue.
            continue
        }
        symbols = append(symbols, sf)
    }

    return PriorFileFact{
        Path:             path,
        Language:         lang,
        ExtractionStatus: extract.ExtractionStatusReady,
        Symbols:          symbols,
    }, true, nil
}

func (s *Store) readSnapshotFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
    // Resolve latest snapshot id for repo.
    snapID, err := s.LatestCommittedSnapshot(ctx, repoID)
    if err != nil {
        return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: latest snapshot: %w", err)
    }
    if snapID == 0 {
        return PriorFileFact{}, false, nil
    }
    // JOIN semantic_files (path → file_id) + semantic_symbols + convert
    // store.SymbolFact rows to extract.SymbolFact (see Open Questions Q2).
    // Implementation elided.
    // ...
}
```

### Example 2: difffacts.go Tier-1 implementation

```go
// Source: replaces internal/semantic/live/handler/difffacts.go tryFullDiff stub.
func (h *Handler) tryFullDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) bool {
    if h.factStore == nil || h.extractRegistry == nil {
        return false
    }
    prior, ok, err := h.factStore.GetLatestFileFact(ctx, string(repoID), path)
    if err != nil {
        h.Logger.Warn("populator: GetLatestFileFact failed; falling through",
            "repo", repoID, "path", path, "err", err)
        return false
    }
    if !ok {
        // Cold start — caller (Tier-3) handles via synthetic-marker.
        return false
    }
    lang := langFromExt(path) // share with semantic_wiring helper
    provider, ok := h.extractRegistry.Provider(lang)
    if !ok {
        return false
    }
    ef, err := provider.ExtractFile(ctx, string(repoID), path)
    if err != nil || ef == nil {
        return false
    }
    if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
        return false // Tier-2 will handle ExtractionStatusPartial.
    }
    diffSymbols(prior.Symbols, ef.Symbols, recorder)
    // diffEdges deferred — see Pattern 2.
    h.emitFileFactDiffOutcome(repoID, "full")
    return true
}
```

### Example 3: Metric helper on obs.Metrics

```go
// Source: mirrors internal/obs/metrics.go:704 SemanticLiveUpdatesInc verbatim shape.
// Add to obs.Metrics struct:
//   LiveFileFactDiff       *prometheus.CounterVec  // {tier, repo}
//   LiveFileFactDiffSynRsn *prometheus.CounterVec  // {reason}

func (m *Metrics) LiveFileFactDiffInc(tier, repo string) {
    if m == nil || m.LiveFileFactDiff == nil {
        return
    }
    switch tier {
    case "full", "added-only", "synthetic":
    default:
        return // drop-on-unknown
    }
    m.LiveFileFactDiff.WithLabelValues(tier, repo).Inc()
}

func (m *Metrics) LiveFileFactDiffSyntheticReasonInc(reason string) {
    if m == nil || m.LiveFileFactDiffSynRsn == nil {
        return
    }
    switch reason {
    case "cold_start", "extract_failed", "extract_unsupported":
    default:
        return
    }
    m.LiveFileFactDiffSynRsn.WithLabelValues(reason).Inc()
}
```

### Example 4: E2E test skeleton (D-09)

```go
// Source: skeleton derived from internal/semantic/live/handler/recorder_test.go
// (handler_test.newRecorderTestHandler) + internal/semantic/store/snapshot_test.go
// (real Store bootstrap).
package handler_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    graphpkg "github.com/agenthands/helix/internal/semantic/graph"
    "github.com/agenthands/helix/internal/semantic/live"
    "github.com/agenthands/helix/internal/semantic/live/handler"
    storepkg "github.com/agenthands/helix/internal/semantic/store"
    // ... extract registry, grammars, etc.
)

func TestE2E_LiveEditFiresPreciseDiff(t *testing.T) {
    if testing.Short() {
        t.Skip("e2e")
    }
    // 1. Bootstrap real *storepkg.Store on tmpdir DuckDB.
    // 2. Build extract.Registry with golang provider.
    // 3. Seed a snapshot with one Go file containing `func Hello() string { return "v1" }`.
    // 4. Construct Handler with real store + real registry + recording applier.
    // 5. Rewrite the file on disk to `func Hello() int { return 42 }` (signature change).
    // 6. Dispatch ChangeHelixEdit.
    // 7. Assertions:
    //    - applier.calls has exactly 1 entry.
    //    - applier.calls[0].repair.DirtyNodes is non-empty.
    //    - the SymbolDiff in the recorder snapshot carries SignatureChanged=true.
    //      (assert via a test seam that captures the recorder snapshot OR via
    //       a side-channel on the handler — see § Open Questions Q4).
    //    - obs.Metrics LiveFileFactDiff{tier="full"} incremented once.
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `var diff graphpkg.FileFactDiff` (unconditionally empty) → empty `GraphRepair` → `ApplyRepair` short-circuits → `graph_version` never advances on live edits. | `FileFactDiffRecorder` seam populated by `populateRecorderForFile`; recorder non-empty on every live edit; `ApplyRepair` fires; `graph_version` advances. | Phase 62-09 closure (2026-05-07) shipped the seam + Tier-3 synthetic marker. Tier-1/2 are scaffolding stubs (`difffacts.go:64-87` return false). | Phase 68 is the second half of the same story — turning the scaffolding into real diff plumbing. |
| `extract.Provider` had concrete-only `Extract` methods (no interface method); buildFn used per-language switch. | `Extract` widened to `ExtractionPipeline.Extract` interface method (Phase 65 D-06, `provider.go:41-49`). | Phase 65 (2026-05-08). | Phase 68 widens `ExtractionPipeline` again with `ExtractFile` — second contract widening, same precedent. |
| Phase 60 P04 was to ship full FileFact upsert into the overlay tables (writing symbols into `semantic_live_overlay_symbols`). | Schema exists; writer is partially deferred. | Schema landed Phase 60 P02; writer in 60-04 SUMMARY noted as scoped down. | Phase 68's overlay read branch may be unreachable in practice — see Pitfall 2 and Open Questions Q3. |

**Deprecated/outdated:**

- `RecordSymbolChanged{KindChanged:true}` synthetic marker as the production fallback for live edits — replaced by Tier-1/2 in this phase. Marker becomes Tier-3 cold-start/failure-only.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `semantic_live_overlay_symbols.fact_json` will contain a JSON-serialized `extract.SymbolFact` (or close-enough shape to deserialize). | Pattern 3, Example 1, Pitfall 2 | HIGH — if writer doesn't populate, overlay branch never fires; Tier-1 effectively snapshot-only. Mitigation: ship snapshot-only first, document overlay branch as future capability. |
| A2 | `extract.SymbolFact.ID` (uint64) is the canonical `graphpkg.NodeID` for *live* overlay symbols. | Pattern 1 | MEDIUM — `semantic_wiring.go:1626` confirms this for *snapshot* facts; the live overlay table has a separate `node_id` column (`migrations.go:317`). They might diverge for symbols that don't yet have a stable_key (e.g., partial extractions). Mitigation: verify `semantic_live_overlay_symbols.node_id == symbol_id` invariant or use `node_id` from the overlay row directly. |
| A3 | `Provider.Extract` is safe to call from within the live tx (no blocking I/O beyond `os.ReadFile`, no LSP roundtrip). | Pattern 4 | LOW — tree-sitter parses are CPU-bound and fast (sub-ms for typical Go/TS files). Verified: golang/provider.go does NOT touch LSP; it's pure tree-sitter + xxhash. |
| A4 | `Handler` can accept new dependencies (`factStore`, `extractRegistry`) without breaking existing callers. | Pattern 4, Example 2 | LOW — `Handler` is constructed via `handler.New(...)` in `daemon/live_wiring.go` only; new fields can be added as nil-safe optional setters (mirrors `SetRankApplier`). |
| A5 | `langFromExt` helper (or equivalent) can be moved/shared from `daemon/semantic_wiring.go` into a package both `daemon` and `live/handler` can import (or duplicated cheaply). | Example 2 | LOW — three-line function; duplication is acceptable until a shared `internal/semantic/langext` lands. |
| A6 | The vet-nokernel2semantic invariant remains green because all new code lives under `internal/semantic/*` and `internal/obs/*`; neither path is `internal/kernel/*`. | Vet Boundary | HIGH if violated, but easy to verify — `make vet` in the phase verification gate. |

**Mitigation strategy:** A1 and A2 are testable in Wave 0 by querying a dev DB. A3, A4, A5 are mechanical. A6 is enforced by the existing test (`internal/lint/nokernel2semantic/realtree_integration_test.go`).

## Open Questions

1. **Should `GetLatestFileFact` return `extract.FileFact`, `store.FileFact`, or a new `store.PriorFileFact`?**
   - What we know: Both existing `*FileFact` types are insufficient (one has status only, one has identity only — neither carries `Symbols`).
   - What's unclear: Whether the planner prefers a new type vs. extending an existing one.
   - Recommendation: New `store.PriorFileFact` (Pattern 3 shape). The CONTEXT.md signature `(extract.FileFact, ...)` SHOULD be revised in the plan because it doesn't satisfy DIFF-01's data requirements.

2. **How does `store.SymbolFact` (snapshot row) convert to `extract.SymbolFact` (extraction in-memory)?**
   - What we know: Field sets are similar but not identical (snapshot has `FileID`, `NodeID`, `Exported bool`; extract has `Visibility string`, `Confidence float32`, `Partial bool`). No conversion helper exists.
   - What's unclear: Whether a one-way `storeSymbolToExtractSymbol(store.SymbolFact) extract.SymbolFact` helper belongs in `store` or `extract`.
   - Recommendation: Helper in `store` (snapshot-side) since the snapshot row is what's being adapted to the diff's expected shape. Map `Exported bool` → `Visibility` (`"exported"` if true else `"private"`). Set `Confidence: 0` / `Partial: false` since they're not used by the diff.

3. **Is the overlay symbol-write path actually populated in production?**
   - What we know: Schema column exists; writer not confirmed (Pitfall 2).
   - What's unclear: Whether Phase 60 P04 (full FileFact upsert) lands before, with, or after Phase 68.
   - Recommendation: Wave 0 verification step — grep for INSERT statements into `semantic_live_overlay_symbols` in production paths. If absent, Phase 68 ships **snapshot-fallback-only** for Tier-1 (the overlay branch becomes dead code until Phase 60 P04 lands), and the SUMMARY documents this. The phase still closes DEF-67-F01-FULL-DIFF because Tier-1 against snapshot is a real (non-synthetic) diff.

4. **How does the E2E test inspect the recorder snapshot to assert `SignatureChanged: true`?**
   - What we know: `FileFactDiffRecorder.Snapshot()` is exported (handler.go:112) and returns `graphpkg.FileFactDiff`. The recording RankApplier (recorder_test.go:23) receives `GraphRepair` (post-`ComputeGraphRepair`), which drops flag detail (only NodeIDs survive).
   - What's unclear: Whether to add a new test seam exposing the pre-Compute snapshot, OR verify the SymbolDiff payload indirectly via `DirtyNodes` containing the expected NodeID.
   - Recommendation: Indirect verification — assert that `recorder.Snapshot().ChangedSymbols` contains exactly one entry with the expected `NodeID` and `SignatureChanged: true`. Reaching this requires hoisting the snapshot capture before `ComputeGraphRepair` — add a test-only export `LastRecorderSnapshotForTest(*Handler) graphpkg.FileFactDiff` mirroring the existing `SetPopulateRecorderForTest` pattern.

5. **Should `ExtractFile` take `repoID` or omit it?**
   - What we know: D-03 signature is `ExtractFile(ctx, repoID, path)`. Current Go/TS providers don't need it.
   - What's unclear: Whether plumbing an unused argument is worse than a future API break.
   - Recommendation: Keep `repoID` (per D-03) — costs nothing, future-proofs the interface, and matches the symmetric `GetLatestFileFact(ctx, repoID, path)` call site.

## Environment Availability

> N/A — phase is code-and-test only; no new external tools, services, or runtimes.

The existing Go toolchain (1.22+, per repo) and DuckDB-go driver (already imported via `internal/semantic/store`) cover everything. `make vet`, `make test`, `go vet ./...`, `go test ./... -race` are the only build/test commands needed.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib + `go test -race` |
| Config file | none — standard Go convention |
| Quick run command | `go test ./internal/semantic/live/handler/... ./internal/semantic/store/... ./internal/semantic/extract/... -count=1` |
| Full suite command | `go test ./... -race -count=1` (matches CLAUDE.md mandate) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| DIFF-01 | Tier-1 full diff populates recorder with per-symbol added/removed/changed; ApplyRepair receives non-empty repair. | e2e | `go test -race -run TestE2E_LiveEditFiresPreciseDiff ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| DIFF-01 | `diffSymbols` correctly computes added/removed/changed bit-flags from prior+new symbol slices. | unit | `go test -race -run TestDiffSymbols ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| DIFF-01 | Body-only change does NOT advance graph_version (no flag set). | unit | `go test -race -run TestDiffSymbols_BodyOnlyNotGraphChanging ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| DIFF-02 | `*Store.GetLatestFileFact` returns overlay row when overlay file is live + has symbols. | unit | `go test -race -run TestGetLatestFileFact_OverlayHit ./internal/semantic/store/...` | ❌ Wave 0 |
| DIFF-02 | `*Store.GetLatestFileFact` falls back to snapshot when overlay missing. | unit | `go test -race -run TestGetLatestFileFact_SnapshotFallback ./internal/semantic/store/...` | ❌ Wave 0 |
| DIFF-02 | `*Store.GetLatestFileFact` returns `(_, false, nil)` on cold-start. | unit | `go test -race -run TestGetLatestFileFact_ColdStart ./internal/semantic/store/...` | ❌ Wave 0 |
| DIFF-02 | `make vet` passes (vet-nokernel2semantic invariant). | smoke | `make vet` | ✅ existing |
| DIFF-03 | `graph_version` advances on live edit (end-to-end). | e2e | covered by `TestE2E_LiveEditFiresPreciseDiff` assertion on `applier.calls[0].repair.DirtyNodes` | ❌ Wave 0 |
| DIFF-03 | E2E race-clean. | e2e (race) | `go test -race -run TestE2E_LiveEditFiresPreciseDiff ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| DIFF-04 | Tier-2 fires on `ExtractionStatusPartial`; recorder receives N `RecordSymbolAdded` entries. | unit | `go test -race -run TestTryAddedOnlyDiff_Partial ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| DIFF-04 | Tier-2 outcome metric records `tier="added-only"`. | unit | `go test -race -run TestFileFactDiffOutcomeMetric ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| (success crit 5) | Tier-3 fires on cold-start AND on extract-failed AND on extract-unsupported; synthetic_reason metric records each distinct reason. | unit | `go test -race -run TestTier3_BoundedReasonMetric ./internal/semantic/live/handler/...` | ❌ Wave 0 |
| (regression) | Existing handler tests (lane selection, bulk-update, rename) still green. | unit | `go test -race ./internal/semantic/live/handler/... -count=1` | ✅ existing |
| (regression) | Existing graph tests (ComputeGraphRepair, ApplyRepair) still green. | unit | `go test -race ./internal/semantic/graph/... -count=1` | ✅ existing |

### Sampling Rate

- **Per task commit:** `go test -race ./internal/semantic/live/handler/... ./internal/semantic/store/... ./internal/semantic/extract/...`
- **Per wave merge:** `go test -race ./internal/semantic/... ./internal/obs/... -count=1 && make vet`
- **Phase gate:** `go test ./... -race -count=1` (full repo) green before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/semantic/store/filefact_accessor_test.go` — covers DIFF-02 (overlay hit, snapshot fallback, cold-start, error propagation).
- [ ] `internal/semantic/extract/golang/provider_extract_file_test.go` — covers Go `ExtractFile` (happy path, file-not-found → partial, large-file → partial).
- [ ] `internal/semantic/extract/typescript/provider_extract_file_test.go` — covers TS `ExtractFile` (happy path).
- [ ] `internal/semantic/live/handler/difffacts_test.go` — covers `diffSymbols`, `tryFullDiff` (mocked store + registry), `tryAddedOnlyDiff` (mocked registry returning partial), Tier-3 dispatch.
- [ ] `internal/semantic/live/handler/handler_diff_e2e_test.go` — covers DIFF-03 end-to-end against real `*Store` + real `extract.Registry`.
- [ ] Test seam: `internal/semantic/live/handler/export_test.go` extension — add `LastRecorderSnapshotForTest` accessor (see Open Questions Q4).
- [ ] No framework install needed — Go stdlib + already-installed providers cover everything.

## Project Constraints (from CLAUDE.md)

| Constraint | How Phase 68 Honors It |
|------------|------------------------|
| "Always run `go vet` and `go test` before completing any Go task." | Both included in Validation Architecture sampling rates. `make vet` includes `vet-nokernel2semantic` enforcement. |
| GSD Workflow Enforcement: no direct edits outside a GSD workflow. | All work proceeds through `/gsd-plan-phase` → `/gsd-execute-phase`. |
| SMTC-first tool routing for code-aware ops. | Research used SMTC-equivalent inspection (Read/Grep on a Go-native codebase where SMTC's Go support applies); planner and executor are expected to use `mcp__smtc__*` tools for code navigation (e.g., `goto_definition` on `populateRecorderForFile`, `find_references` on `FileFactDiffRecorder`). |
| No CI benchmarks. | Phase 68 introduces no benchmarks. |
| Helix ships binary archives only. | N/A — code-only change. |
| Legacy reference: `legacy/` is read-only. | Phase 68 touches `internal/semantic/...` only; no legacy/ involvement. |

## Sources

### Primary (HIGH confidence)

- `internal/semantic/live/handler/handler.go` (lines 1–556) — read 2026-05-13.
- `internal/semantic/live/handler/difffacts.go` (lines 1–87) — read 2026-05-13.
- `internal/semantic/live/handler/export_test.go` — read 2026-05-13.
- `internal/semantic/live/handler/recorder_test.go` (relevant sections) — read 2026-05-13.
- `internal/semantic/live/handler/noop_test.go` — read 2026-05-13.
- `internal/semantic/graph/repair.go` (full file) — read 2026-05-13.
- `internal/semantic/graph/apply_repair.go` (relevant grep) — read 2026-05-13.
- `internal/semantic/extract/fact.go` (full file) — read 2026-05-13.
- `internal/semantic/extract/provider.go` (full file) — read 2026-05-13.
- `internal/semantic/extract/golang/provider.go` (lines 1–300) — read 2026-05-13.
- `internal/semantic/store/snapshot.go` (lines 1–365) — read 2026-05-13.
- `internal/semantic/store/overlay.go` (relevant sections) — read 2026-05-13.
- `internal/semantic/store/migrations.go` (lines 292–457) — read 2026-05-13.
- `internal/daemon/semantic_wiring.go` (lines 1540–1612) — read 2026-05-13.
- `internal/obs/metrics.go` (lines 689–720, 322–399, 856–870) — read 2026-05-13.
- `internal/lint/nokernel2semantic/analyzer.go` (lines 1–90) — read 2026-05-13.
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-09-SUMMARY.md` (lines 1–100) — read 2026-05-13.
- `.planning/phases/68-precise-filefactdiff-populator/68-CONTEXT.md` (full) — read 2026-05-13.
- `.planning/REQUIREMENTS.md` (DIFF-01..04 entries) — read 2026-05-13.
- `.planning/ROADMAP.md` (Phase 68 entry) — read 2026-05-13.

### Secondary (MEDIUM confidence)

- Field semantics for `golang/provider.go signatureHash()` — inferred from the function body (lines 379–404) showing it hashes `body.Utf8Text(source)`. Documented as Pitfall 3.

### Tertiary (LOW confidence)

- None. All claims in this research are anchored to read source.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all packages in-tree, all signatures verified by reading source.
- Architecture: HIGH — the seam already exists (62-09) and Phase 68 is filling stubs; no novel architecture.
- Pitfalls: MEDIUM — Pitfall 2 and 3 depend on runtime behavior (overlay writer state, SignatureHash semantics) that should be confirmed in Wave 0 before Tier-1 lands.
- Assumptions A1/A2: MEDIUM — testable in Wave 0, but if they fail the implementation pivots from "overlay+snapshot" to "snapshot-only" for Tier-1.

**Research date:** 2026-05-13

**Valid until:** 2026-06-12 (30 days; stable subsystem, no fast-moving external deps).

---

*Phase: 68-precise-filefactdiff-populator*
*Research generated: 2026-05-13 via /gsd-research-phase*
