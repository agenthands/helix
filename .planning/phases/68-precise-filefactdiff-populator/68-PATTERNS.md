# Phase 68: Precise FileFactDiff Populator - Pattern Map

**Mapped:** 2026-05-13
**Files analyzed:** 11 new/modified
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/store/filefact_accessor.go` (NEW) | accessor (read-only Store method) | request-response (SQL read) | `internal/semantic/store/effective_graph.go` (`LatestCommittedSnapshot`) + `internal/semantic/store/overlay.go` (`CurrentGraphVersion`) | exact |
| `internal/semantic/store/filefact_accessor_test.go` (NEW) | test (unit) | request-response | `internal/semantic/store/effective_graph_test.go` (assumed; standard `*Store` table-driven test pattern) | role-match |
| `internal/semantic/extract/provider.go` (MOD) | interface widening | contract | already-extant `ExtractionPipeline.Extract` widening (Phase 65 D-06, lines 41-49) | exact |
| `internal/semantic/extract/golang/provider.go` (MOD) | shim (per-language `ExtractFile`) | file-I/O + transform | sibling `Extract` (lines 72-300) + `partialFile` helper (lines 302-322); also `buildFn` path-read pattern at `internal/daemon/semantic_wiring.go:1587-1605` | exact |
| `internal/semantic/extract/golang/provider_extract_file_test.go` (NEW) | test (unit) | file-I/O | existing `internal/semantic/extract/golang/*_test.go` (provider-level extract tests) | role-match |
| `internal/semantic/extract/typescript/provider.go` (MOD) | shim | file-I/O + transform | same as Go provider | exact |
| `internal/semantic/extract/typescript/provider_extract_file_test.go` (NEW) | test | file-I/O | same as Go test | role-match |
| `internal/semantic/live/handler/difffacts.go` (MOD) | dispatch + diff compute | transform (in-tx) | itself (Tier-3 already implemented at lines 35-53); diff algorithm has no exact analog — derived from `internal/semantic/graph/repair.go` `SymbolDiff` shape | role-match |
| `internal/semantic/live/handler/difffacts_test.go` (NEW) | test (unit, diff helpers + tier dispatch) | transform | `internal/semantic/live/handler/recorder_test.go` (fake store + recording applier scaffolding) | exact |
| `internal/semantic/live/handler/handler_diff_e2e_test.go` (NEW) | test (E2E) | full pipeline | `internal/semantic/live/handler/recorder_test.go` (`newRecorderTestHandler` shape, `recordingRankApplier`, `slogLoggerAdapter`) | exact |
| `internal/semantic/live/handler/handler.go` (MOD) | wiring (DI for store + registry) | n/a | `SetRankApplier` nil-safe setter at lines 256-265 | exact |
| `internal/semantic/live/handler/export_test.go` (MOD) | test seam extension | n/a | itself (existing `SetPopulateRecorderForTest`) | exact |
| `internal/obs/metrics.go` (MOD) | metric (CounterVec + helper) | metric emission | `SemanticLiveUpdates` field (line 103) + `SemanticLiveUpdatesInc` helper (lines 704-717) + Register block (lines 322-331) | exact |

## Pattern Assignments

### `internal/semantic/store/filefact_accessor.go` (NEW — accessor, request-response)

**Analog:** `internal/semantic/store/overlay.go` (`CurrentGraphVersion`, lines 978-1002) + `internal/semantic/store/effective_graph.go` (`LatestCommittedSnapshot`, lines 202-226).

**Package + imports pattern** (mirror `overlay.go` header):
```go
package store

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"

    "github.com/agenthands/helix/internal/semantic/extract"
)
```

**Single-row read pattern** (`overlay.go:983-1002`):
```go
func (s *Store) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
    if s == nil || s.db == nil {
        return 0, fmt.Errorf("CurrentGraphVersion: nil store")
    }
    if repoID == "" {
        return 0, fmt.Errorf("CurrentGraphVersion: empty repoID")
    }
    var gv uint64
    err := s.db.QueryRowContext(ctx, `
        SELECT graph_version FROM semantic_live_overlay_meta
         WHERE repo_id = ?
    `, repoID).Scan(&gv)
    if err == sql.ErrNoRows {
        return 0, nil
    }
    if err != nil {
        return 0, fmt.Errorf("CurrentGraphVersion(%q): %w", repoID, err)
    }
    return gv, nil
}
```

**Apply for Phase 68:**
- Copy the nil-receiver guard + empty-repoID guard verbatim.
- Use `sql.ErrNoRows` → `(zero, false, nil)` cold-start pattern (DIFF-02 ColdStart contract).
- Wrap I/O errors with `fmt.Errorf("GetLatestFileFact(%q,%q): %w", ...)` per file convention.

**Latest-snapshot fallback pattern** (`effective_graph.go:208-226`):
```go
const q = `
    SELECT COALESCE(MAX(snapshot_id), 0)
      FROM semantic_snapshots
     WHERE repo_id = ?
       AND status  = 'committed'
`
var id uint64
if err := s.db.QueryRowContext(ctx, q, repoID).Scan(&id); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return 0, nil
    }
    return 0, fmt.Errorf("LatestCommittedSnapshot(%q): %w", repoID, err)
}
return id, nil
```
Reuse `s.LatestCommittedSnapshot(ctx, repoID)` directly — do NOT inline a duplicate query.

---

### `internal/semantic/extract/golang/provider.go` (MOD — shim, file-I/O)

**Analog:** sibling `Extract` (lines 72-300) + `partialFile` helper (lines 302-322) + `buildFn` read-pattern at `internal/daemon/semantic_wiring.go:1587-1605`.

**File-read + delegate pattern** (compose `os.ReadFile` from `semantic_wiring.go:1587-1605` with the existing `Extract` call):
```go
// semantic_wiring.go:1587-1605 (reference pattern)
source, err := os.ReadFile(path)
if err != nil {
    if b.logger != nil {
        b.logger.Debug("buildFn: read source failed; skipping path", ...)
    }
    continue
}
ef, err := provider.Extract(ctx, source, extract.SourceFile{
    Path:     path,
    Language: lang,
})
```

**Partial-on-failure pattern** (`golang/provider.go:302-322`):
```go
func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
    msg := ""
    if err != nil {
        msg = err.Error()
    }
    status := extract.ExtractionStatusPartial
    return &extract.ExtractedFile{
        File: extract.FileFact{
            Path:              file.Path,
            Language:          "go",
            ExtractionStatus:  status,
            ExtractionPartial: true,
            PartialReason:     reason,
            ExtractorName:     "goextract",
            ExtractorVersion:  "1",
            ErrorMessage:      msg,
        },
        Partial:       true,
        PartialReason: string(reason),
    }
}
```

**New `ExtractFile` shim** (combines the two — single function, in same file, after `Extract`):
```go
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
    source, err := os.ReadFile(path)
    if err != nil {
        return partialFile(
            extract.SourceFile{Path: path, Language: "go"},
            extract.PartialReasonPermissionDenied, err), nil
    }
    return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "go"})
}
```

TypeScript provider follows identical shape with `Language: "typescript"` and language-appropriate `partialFile`.

---

### `internal/semantic/extract/provider.go` (MOD — interface widening)

**Analog:** existing `ExtractionPipeline` interface widening from Phase 65 D-06 (same file, lines 41-49).

**Existing precedent — copy comment style:**
```go
type ExtractionPipeline interface {
    Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)
}
```

**Add a second method on the same interface:**
```go
type ExtractionPipeline interface {
    Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)

    // ExtractFile reads `path` from disk and returns the per-file
    // ExtractedFile. Phase 68 D-03 contract: synchronous in-tx
    // invocation from the live handler. Per-file partials surface via
    // ExtractedFile.File.ExtractionStatus rather than returned errors.
    ExtractFile(ctx context.Context, repoID, path string) (*ExtractedFile, error)
}
```

---

### `internal/semantic/live/handler/difffacts.go` (MOD — dispatch + transform)

**Analog:** itself — Tier-3 implementation at lines 35-53 is the template. Diff algorithm derived from `SymbolDiff` bit-flags in `internal/semantic/graph/repair.go:25-30`.

**Tier-3 production stub** (today's lines 50-52):
```go
recorder.RecordSymbolChanged(graphpkg.SymbolDiff{
    KindChanged: true,
})
```

**SymbolDiff field set to map against** (`repair.go:25-30`):
```go
type SymbolDiff struct {
    NodeID            NodeID
    SignatureChanged  bool
    ExportedChanged   bool
    KindChanged       bool
    StableKeyChanged  bool
    BodyOnlyChanged   bool  // (D-06 NOT-graph-changing per repair.go:106-108)
}
```

**Tier-1 fill-in** — replace lines 64-70 with:
```go
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
        return false  // cold start → Tier-3 handles
    }
    lang := langFromExt(path)
    provider, ok := h.extractRegistry.Provider(lang)
    if !ok {
        return false
    }
    ef, err := provider.ExtractFile(ctx, string(repoID), path)
    if err != nil || ef == nil {
        return false
    }
    if ef.File.ExtractionStatus != extract.ExtractionStatusReady {
        return false  // Tier-2 handles partial
    }
    diffSymbols(prior.Symbols, ef.Symbols, recorder)
    h.emitFileFactDiffOutcome(repoID, "full")
    return true
}
```

**diffSymbols helper** — match by `SymbolFact.ID` (= `graphpkg.NodeID` per `semantic_wiring.go:1626`):
```go
func diffSymbols(prior, new []extract.SymbolFact, rec *FileFactDiffRecorder) {
    priorByID := make(map[semantic.SymbolID]extract.SymbolFact, len(prior))
    for _, s := range prior {
        priorByID[s.ID] = s
    }
    newByID := make(map[semantic.SymbolID]extract.SymbolFact, len(new))
    for _, s := range new {
        newByID[s.ID] = s
    }
    for id := range priorByID {
        if _, still := newByID[id]; !still {
            rec.RecordSymbolRemoved(graphpkg.SymbolDiff{NodeID: graphpkg.NodeID(id)})
        }
    }
    for id := range newByID {
        if _, was := priorByID[id]; !was {
            rec.RecordSymbolAdded(graphpkg.SymbolDiff{
                NodeID: graphpkg.NodeID(id), KindChanged: true,
            })
        }
    }
    for id, n := range newByID {
        p, ok := priorByID[id]
        if !ok { continue }
        d := graphpkg.SymbolDiff{NodeID: graphpkg.NodeID(id)}
        if p.Signature != n.Signature { d.SignatureChanged = true }  // see Pitfall 3 — compare Signature, NOT SignatureHash
        if p.Visibility != n.Visibility { d.ExportedChanged = true }
        if p.Kind != n.Kind { d.KindChanged = true }
        if p.StableKey != n.StableKey { d.StableKeyChanged = true }
        if d.SignatureChanged || d.ExportedChanged || d.KindChanged || d.StableKeyChanged {
            rec.RecordSymbolChanged(d)
        }
    }
}
```

**Tier-2 fill-in** (replace lines 81-87):
```go
func (h *Handler) tryAddedOnlyDiff(ctx context.Context, repoID semantic.RepoID, path string, recorder *FileFactDiffRecorder) bool {
    if h.extractRegistry == nil {
        return false
    }
    lang := langFromExt(path)
    provider, ok := h.extractRegistry.Provider(lang)
    if !ok {
        return false
    }
    ef, err := provider.ExtractFile(ctx, string(repoID), path)
    if err != nil || ef == nil {
        return false
    }
    if ef.File.ExtractionStatus != extract.ExtractionStatusPartial {
        return false
    }
    for _, s := range ef.Symbols {
        recorder.RecordSymbolAdded(graphpkg.SymbolDiff{
            NodeID: graphpkg.NodeID(s.ID), KindChanged: true,
        })
    }
    h.emitFileFactDiffOutcome(repoID, "added-only")
    return true
}
```

**Tier-3 + reason metric** — augment the existing recorder call (lines 50-52) with:
```go
// Determine reason before the fall-through.
reason := "cold_start"  // default; refine if ExtractFile produced failed/unsupported
h.Metrics.LiveFileFactDiffSyntheticReasonInc(reason)
h.emitFileFactDiffOutcome(repoID, "synthetic")
recorder.RecordSymbolChanged(graphpkg.SymbolDiff{KindChanged: true})
```

---

### `internal/semantic/live/handler/handler.go` (MOD — wiring)

**Analog:** `SetRankApplier` nil-safe setter at lines 256-265.

**Existing pattern** (verbatim copy structure for two new setters):
```go
func (h *Handler) SetRankApplier(r RankApplier) {
    if h == nil {
        return
    }
    h.rankApplier = r
}
```

**Add two new fields + setters:**
```go
// On Handler struct (around line 230, alongside rankApplier):
factStore       FileFactStore        // nil-safe; daemon wires post-init
extractRegistry ExtractRegistry      // nil-safe; daemon wires post-init

// Interface (minimal — only the methods difffacts.go calls):
type FileFactStore interface {
    GetLatestFileFact(ctx context.Context, repoID, path string) (store.PriorFileFact, bool, error)
}
type ExtractRegistry interface {
    Provider(lang string) (extract.Provider, bool)
}

func (h *Handler) SetFileFactStore(s FileFactStore)        { if h != nil { h.factStore = s } }
func (h *Handler) SetExtractRegistry(r ExtractRegistry)    { if h != nil { h.extractRegistry = r } }
```

**Why interfaces (not concrete types):**
- Mirrors `OverlayWriter` (handler.go:6-7) and `IncrementalScheduler` (handler.go:11-12) — handler is interface-driven on every dependency boundary to keep tests Drive-able without DuckDB.
- Avoids importing `*store.Store` directly (cleaner test bootstrap; tests already use `fakeStore` from `noop_test.go`).

---

### `internal/semantic/live/handler/handler_diff_e2e_test.go` (NEW — E2E test)

**Analog:** `internal/semantic/live/handler/recorder_test.go` (`newRecorderTestHandler`, `recordingRankApplier`, `recordingSlogHandler`, `slogLoggerAdapter` — all at lines 20-87).

**Core scaffolding** (`recorder_test.go:23-35`):
```go
type recordingRankApplier struct {
    calls []recordedApplyRepairCall
}

type recordedApplyRepairCall struct {
    repoID string
    repair graphpkg.GraphRepair
}

func (r *recordingRankApplier) ApplyRepair(_ context.Context, repoID string, repair graphpkg.GraphRepair) (uint64, bool, error) {
    r.calls = append(r.calls, recordedApplyRepairCall{repoID: repoID, repair: repair})
    return 1, true, nil
}
```

**Apply for Phase 68 E2E:** REUSE `recordingRankApplier` from `recorder_test.go` (same `package handler_test`). Add a real `*storepkg.Store` (DuckDB tmpfile, mirror `internal/semantic/store/snapshot_test.go` setup) and a real `extract.Registry` with `goextract.NewProvider(grammars)`. Do NOT mock `populateRecorderForTest` (D-10 — exercise REAL populator).

**Test assertion shape** (mirror recorder_test assertions):
```go
if len(applier.calls) != 1 {
    t.Fatalf("ApplyRepair: want 1 call, got %d", len(applier.calls))
}
if len(applier.calls[0].repair.DirtyNodes) == 0 {
    t.Fatalf("ApplyRepair: want non-empty DirtyNodes")
}
```

**Anti-pattern to avoid** (per Pitfall 5): do NOT import `lspclient` / `fsnotify`. Drive the test through `h.Dispatch(ctx, live.SourceChangeEvent{Kind: live.ChangeHelixEdit, ...})` directly.

---

### `internal/obs/metrics.go` (MOD — metric definition + helper)

**Analog:** `SemanticLiveUpdates` field (line 103) + `SemanticLiveUpdates: prometheus.NewCounterVec(...)` registration (lines 322-331) + `SemanticLiveUpdatesInc` helper (lines 704-717).

**Field declaration pattern** (lines 95-103):
```go
// Phase 60 D-07: live-update pipeline outcome counter (60-05B).
// Closed-enum "kind" ∈ {"file_created","file_modified",...}
// Closed-enum "outcome" ∈ {"applied","no_op","error","dropped"}.
// Cardinality bound: 6 × 4 = 24 combos per workspace.
SemanticLiveUpdates *prometheus.CounterVec
```

**Apply for Phase 68:**
```go
// Phase 68: precise FileFactDiff populator outcome counter.
// Closed-enum "tier" ∈ {"full","added-only","synthetic"}; "repo" bounded
// per workspace. Cardinality bound: 3 × N repos.
LiveFileFactDiff *prometheus.CounterVec

// Phase 68: Tier-3 fall-through reason metric.
// Closed-enum "reason" ∈ {"cold_start","extract_failed","extract_unsupported"}.
LiveFileFactDiffSynRsn *prometheus.CounterVec
```

**Registration block pattern** (lines 322-331):
```go
SemanticLiveUpdates: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_semantic_live_updates_total",
        Help: "Live-update pipeline outcomes by SourceChangeKind and result. Phase 60 D-07.",
    },
    []string{"kind", "outcome"},
),
```

**Apply for Phase 68:**
```go
LiveFileFactDiff: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_live_filefactdiff_total",
        Help: "Live FileFactDiff populator outcomes by tier (full/added-only/synthetic) and repo. Phase 68.",
    },
    []string{"tier", "repo"},
),
LiveFileFactDiffSynRsn: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_live_filefactdiff_synthetic_reason_total",
        Help: "Tier-3 synthetic-marker reason breakdown. Phase 68 D-08.",
    },
    []string{"reason"},
),
```

**Helper pattern with drop-on-unknown** (`SemanticLiveUpdatesInc`, lines 704-717):
```go
func (m *Metrics) SemanticLiveUpdatesInc(kind, outcome string) {
    switch kind {
    case "file_created", "file_modified", "file_deleted", "file_renamed",
        "helix_edit", "bulk_update":
    default:
        return
    }
    switch outcome {
    case "applied", "no_op", "error", "dropped":
    default:
        return
    }
    m.SemanticLiveUpdates.WithLabelValues(kind, outcome).Inc()
}
```

**Apply for Phase 68:**
```go
func (m *Metrics) LiveFileFactDiffInc(tier, repo string) {
    if m == nil || m.LiveFileFactDiff == nil {
        return
    }
    switch tier {
    case "full", "added-only", "synthetic":
    default:
        return  // drop-on-unknown
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

---

### `internal/semantic/live/handler/export_test.go` (MOD — extend test seam)

**Analog:** itself — `SetPopulateRecorderForTest` at lines 13-18.

**Existing pattern:**
```go
func SetPopulateRecorderForTest(h *Handler, fn func(*FileFactDiffRecorder)) {
    if h == nil {
        return
    }
    h.populateRecorderForTest = fn
}
```

**Add (per Open Question Q4) — expose last recorder snapshot for E2E inspection:**
```go
func LastRecorderSnapshotForTest(h *Handler) graphpkg.FileFactDiff {
    if h == nil {
        return graphpkg.FileFactDiff{}
    }
    return h.lastRecorderSnapshot  // requires storing snapshot pre-ComputeGraphRepair
}
```

---

### `internal/semantic/live/handler/difffacts_test.go` (NEW — unit tests)

**Analog:** `internal/semantic/live/handler/recorder_test.go` lines 64-87 (`newRecorderTestHandler`, `fakeStore` from `noop_test.go`).

**Test handler construction pattern:**
```go
func newRecorderTestHandler(t *testing.T, applier *recordingRankApplier, populator func(*handler.FileFactDiffRecorder), logHandler slog.Handler) *handler.Handler {
    t.Helper()
    store := &fakeStore{tx: &fakeTx{epoch: 1}}
    var logger handler.Logger
    if logHandler != nil {
        logger = &slogLoggerAdapter{l: slog.New(logHandler)}
    }
    h := handler.New(store, goodHasher, nil, logger)
    h.SetRankApplier(applier)
    return h
}
```

**Apply for Phase 68 unit tests:** REUSE same scaffolding. Inject a `fakeFileFactStore` and `fakeExtractRegistry` via the new `SetFileFactStore` / `SetExtractRegistry` setters. Drive `tryFullDiff` / `tryAddedOnlyDiff` directly or through `Dispatch`. Assertions read the recorder snapshot via the new `LastRecorderSnapshotForTest` seam.

---

## Shared Patterns

### Nil-safe receiver guard
**Source:** `internal/semantic/store/overlay.go:984-988`, `internal/obs/metrics.go:704-715` (helper-side), `internal/semantic/live/handler/handler.go:260-264` (setter-side).
**Apply to:** All new Store methods, all new metric helpers, all new Handler setters.
```go
if s == nil || s.db == nil {
    return ..., fmt.Errorf("FuncName: nil store")
}
```

### Drop-on-unknown closed-enum
**Source:** `internal/obs/metrics.go:704-717` (`SemanticLiveUpdatesInc`).
**Apply to:** Both new metric helpers (`LiveFileFactDiffInc`, `LiveFileFactDiffSyntheticReasonInc`).
Switch the enum label, fall through to `return` (silent drop) on unknown — keeps cardinality bounded.

### SQL error → cold-start mapping
**Source:** `internal/semantic/store/overlay.go:995-997`, `internal/semantic/store/effective_graph.go:220-223`.
**Apply to:** `GetLatestFileFact` overlay branch and snapshot branch.
```go
err := s.db.QueryRowContext(ctx, ...).Scan(...)
if err == sql.ErrNoRows {  // OR errors.Is(err, sql.ErrNoRows) — file convention varies
    return zero, false, nil
}
if err != nil {
    return zero, false, fmt.Errorf("FnName(%q): %w", arg, err)
}
```

### Nil-safe optional setter (post-construction DI)
**Source:** `internal/semantic/live/handler/handler.go:256-265` (`SetRankApplier`).
**Apply to:** new `SetFileFactStore` and `SetExtractRegistry` on Handler.
- Mirrors how the daemon wires `rankApplier` post-init (`SetRankApplier` is called after the rank engine is constructed).
- Field stays nil through `handler.New(...)`; nil-receiver branches in `tryFullDiff` / `tryAddedOnlyDiff` short-circuit to `return false` so existing handler tests stay green without wiring.

### Recording-applier + slog-capture test scaffolding
**Source:** `internal/semantic/live/handler/recorder_test.go:20-87`.
**Apply to:** Both `difffacts_test.go` and `handler_diff_e2e_test.go`.
- `recordingRankApplier` captures every `ApplyRepair` call.
- `recordingSlogHandler` + `slogLoggerAdapter` lets tests assert on once-INFO log emission.
- `newRecorderTestHandler` is the canonical test-handler builder; both new test files should extend it (not reinvent) with `SetFileFactStore` / `SetExtractRegistry` injection.

### Partial-on-failure return (per-language provider)
**Source:** `internal/semantic/extract/golang/provider.go:302-322` (`partialFile` helper).
**Apply to:** `ExtractFile` in Go provider and TS provider — on `os.ReadFile` failure, return `partialFile(..., PartialReasonPermissionDenied, err), nil` (error in the return slot is reserved for unrecoverable extraction-engine failures per existing `Extract` contract at provider.go:43-47).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | | | Every Phase 68 file has a strong analog within the same package or sibling packages. The phase is plumbing — recombining existing patterns, not inventing new ones. |

## Metadata

**Analog search scope:**
- `internal/semantic/store/` (overlay.go, effective_graph.go, snapshot.go)
- `internal/semantic/extract/` (provider.go, golang/provider.go, fact.go)
- `internal/semantic/live/handler/` (handler.go, difffacts.go, recorder_test.go, export_test.go, noop_test.go)
- `internal/semantic/graph/` (repair.go — `SymbolDiff` shape only)
- `internal/obs/metrics.go` (CounterVec + helper patterns)
- `internal/daemon/semantic_wiring.go` (file-read + provider.Extract precedent)

**Files scanned:** 11 core analogs, ~12 supporting files referenced from research.

**Pattern extraction date:** 2026-05-13

**Vet-boundary note:** All recommended analogs live under `internal/semantic/*` or `internal/obs/*`. None import `internal/kernel/*`. The vet-nokernel2semantic invariant is preserved by construction.
