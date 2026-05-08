# Phase 65: Existing-Tool Integration (Strangler Fig) — Pattern Map

**Mapped:** 2026-05-08
**Files analyzed:** 14 (6 NEW, 8 MODIFIED)
**Analogs found:** 14 / 14 (every target has an in-tree precedent)

## File Classification

| New / Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------------|------|-----------|----------------|---------------|
| `internal/semantic/integ/lookup.go` (NEW) | interface + value types (types-only package) | request-response (read+) | `internal/skill/semantic/accessors.go:18-143` | exact (narrow-accessor seam) |
| `internal/semantic/integ/source.go` (NEW) | closed-enum constants + classifier | request-response | `internal/kernel/health/tools.go:50-81` (`SemanticReason*` consts + `classifySemanticProbeError`) | exact (WR-NEW-01 closed-enum precedent) |
| `internal/kernel/symbols/skill_adapter.go` (NEW) | skill (kernel-resident tool wrapped as ToolProvider) | request-response | `internal/kernel/health/skill_adapter.go` (whole file, 34 LOC) | exact |
| `internal/kernel/symbols/blast_radius_strangler.go` (NEW) | service / orchestrator (two-pass) | request-response (graph-first → LSP-validates) | `internal/kernel/symbols/tools.go:560-585` (`registerAnalyzeBlastRadius`) wrapping pure-LSP `AnalyzeBlastRadius` | role-match (Pass 2 primitive already shipped; new file ADDS Pass 1) |
| `internal/skill/repomap/strangler.go` (NEW, optional — may extend `skill.go` in place) | skill setter + lookup-consult branch | request-response | `internal/skill/repomap/skill.go:113-144` (`SetEnrichFn` / `SetFallbackDeps` / `SetMetricsSink` setter family) | exact |
| `internal/lint/nokernel2semantic/analyzer_test.go` (MODIFIED — fixture add) | test (analysistest) | test | existing `analyzer_test.go` (24 LOC, two `analysistest.Run` cases) | exact |
| `internal/daemon/semantic_wiring.go` (MODIFIED — buildFn + WS adapter + integSemanticLookup) | service (daemon-side adapter family) | CRUD (snapshot lifecycle) + request-response (lookup) | (a) buildFn: `semantic_wiring.go:687-742` (existing empty-Facts skeleton) + inline contract at lines 621-686; (b) WS adapter: `semSessionAdapter` lines 498-525; (c) `integSemanticLookup`: `semStoreAdapter` lines 321-357 + `semRetrievalAdapter` lines 530-619 | exact (joins existing adapter family) |
| `internal/skill/repomap/skill.go` (MODIFIED — setter + envelope wrap) | skill (CRUD on graph + ranking) | request-response | `internal/skill/repomap/skill.go:115-144` (own file, four setter precedents) | exact (literal next entry in the setter family) |
| `internal/skill/repomap/skill_integration_test.go` (MODIFIED — golden mechanical update) | test (golden fixture) | test | own file lines 141-170 (`TestGetRepoMap_WithWorkspace`, `TestGetContext_WithWorkspace`) | exact |
| `internal/kernel/health/tools.go` (MODIFIED — additive `semantic_index` block) | controller (MCP tool registration) | request-response | own file lines 139-181 (existing `RegisterTools` with `semProbe SemanticStoreProbe` seam, envelope-additive pattern) | exact (extension of the file's own pattern) |
| `internal/kernel/symbols/tools.go` (MODIFIED — registerAnalyzeBlastRadius accepts lookup) | controller (MCP tool registration) | request-response | own file lines 560-585 (existing `registerAnalyzeBlastRadius`) | exact (extension in place) |
| `internal/lint/nokernel2semantic/analyzer.go` (MODIFIED — allowlist) | utility (go/analysis Analyzer) | static-analysis (AST walk) | own file lines 33-57 (existing forbidden-prefix check) | exact (one-line allowlist amendment) |
| `internal/daemon/daemon.go` (MODIFIED — post-init wiring) | config / wiring | startup | existing daemon post-init wiring (e.g., `repomapSkill.SetEnrichFn` block; kernel `RegisterTools` calls; `wsKeyFn` closure) | exact |
| `internal/skill/semantic/integration_test.go` (MODIFIED — extend 15-symbol fixture for source × fallback_reason matrix) | test (E2E) | test | own file `TestE2E_IndexThenContext_SymbolCount` at line 438 | exact |

---

## Pattern Assignments

### `internal/semantic/integ/lookup.go` (NEW — interface + value types)

**Analog:** `internal/skill/semantic/accessors.go:18-143` — Phase 64 narrow-accessor pattern (StoreAccessor, RetrievalAccessor, etc.).

**Why this analog:** Phase 65's `SemanticLookup` is the same shape — narrow read+ surface, types-only package, zero concretions, designed so kernel and skill can both import without dragging store/retrieval. Phase 64 froze this discipline; Phase 65 is the next entry in the family.

**Interface declaration shape** (lines 16-33):
```go
// StoreAccessor is the narrow seam between SemanticSkill and *internal/semantic/store.Store.
// Daemon wires a concrete adapter in internal/daemon/semantic_wiring.go (P64-08).
type StoreAccessor interface {
    LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error)
    CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
    OverlayHasPendingRows(repoID string) bool
    QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
        out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
    )
}
```

**Apply directly:** Mirror this exact shape — every method is a one-line method-set declaration, no implementation, no internal helpers in this file. Mirror the doc-comment style ("`X` is the narrow seam between..."). The `SemanticLookup` interface from CONTEXT.md D-03 (`Available`, `RankFiles`, `RankFromSeeds`, `ExpandFrom`, `ValidateCriticalEdges`, `Status`) goes here verbatim.

**Constraints to honor in the package:**
- `vet-noduckdb`: package MUST NOT import `duckdb-go`
- `vet-nokernel2semantic`: package is allowlisted (see analyzer amendment below)
- No imports of `internal/semantic/store/`, `internal/semantic/retrieval/`, `internal/kernel/*`, `internal/skill/*`

---

### `internal/semantic/integ/source.go` (NEW — closed-enum constants + classifier)

**Analog:** `internal/kernel/health/tools.go:50-81` — `SemanticReason*` constants + `classifySemanticProbeError` mapper.

**Closed-enum constants pattern** (lines 50-55):
```go
// Closed-enum reasons for SemanticStoreStatus.Reason. The set is locked
// to keep the get_health JSON envelope predictable for MCP clients and
// to eliminate any risk of leaking raw database/sql or DuckDB error text
// (WR-NEW-01).
const (
    SemanticReasonProbeTimeout = "probe_timeout"
    SemanticReasonNilHandle    = "nil_handle"
    SemanticReasonDBError      = "db_error"
    SemanticReasonUnknown      = "unknown"
)
```

**Classifier pattern** (lines 66-81):
```go
// classifySemanticProbeError maps a probe error onto the closed Reason
// enum (WR-NEW-01).
func classifySemanticProbeError(err error) string {
    if err == nil {
        return ""
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return SemanticReasonProbeTimeout
    }
    msg := err.Error()
    if strings.Contains(msg, "DB handle nil") || strings.Contains(msg, "store unavailable") {
        return SemanticReasonNilHandle
    }
    return SemanticReasonDBError
}
```

**Apply directly:** Define the `Source` and `FallbackReason` types as named-string types with closed-enum constants:
```go
type Source string
const (
    SourceSemantic   Source = "semantic"
    SourceTreeSitter Source = "tree_sitter"
    SourceFallback   Source = "fallback"
)

type FallbackReason string
const (
    FallbackReasonIndexDisabled    FallbackReason = "index_disabled"
    FallbackReasonNoSnapshotYet    FallbackReason = "no_snapshot_yet"
    FallbackReasonIndexBuilding    FallbackReason = "index_building"
    FallbackReasonIndexError       FallbackReason = "index_error"
    FallbackReasonBleveRebuilding  FallbackReason = "bleve_rebuilding"
)
```

**Mini-classifier** mirrors the `errors.Is` ladder verbatim against `ErrNoSnapshot` / `ErrIndexBuilding` / `ErrIndexErrored` / `ErrBleveRebuilding` sentinels; raw text NEVER leaks (RESEARCH Pitfall §1 + WR-NEW-01).

---

### `internal/kernel/symbols/skill_adapter.go` (NEW)

**Analog:** `internal/kernel/health/skill_adapter.go` (whole file, 34 LOC). Identical-shape sibling.

**Full file copy template:**
```go
package symbols

import (
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/skill"
)

// SymbolsSkill is a ToolProvider adapter that exposes the analyze_blast_radius
// tool through the skill interface for daemon discovery.
type SymbolsSkill struct{}

func init() {
    skill.Register(&SymbolsSkill{})
}

func (s *SymbolsSkill) Name() string { return "symbols" }
func (s *SymbolsSkill) Description() string {
    return "Symbol analysis tools (analyze_blast_radius)"
}
func (s *SymbolsSkill) Init(deps skill.SkillDeps) error { return nil }
func (s *SymbolsSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        {Name: "analyze_blast_radius",
         Description: "Analyze the blast radius (impact) of changing a symbol",
         BriefDescription: "Analyze the impact of changing a symbol"},
    }
}
```

**Critical caveat:** kernel/symbols already registers ~9 other tools (definition, references, hover, hierarchy, etc.). Decide during planning whether to add a `SetSemanticLookup(...)` method on `SymbolsSkill` for the kernel-resident `analyze_blast_radius`, or inject the lookup via a function closure passed through `RegisterTools` (see `tools.go:560` extension below). RESEARCH Code Example 2 favors the latter (`lookupFn func() integ.SemanticLookup`) because the kernel tool is registered via `mcpsdk.AddTool` direct, not the skill `ExecuteTool` path.

**vet-nokernel2semantic note:** This file imports `internal/semantic/integ` — the analyzer must be allowlisted first (see `analyzer.go` task below) or this package will not compile under `make vet`.

---

### `internal/kernel/symbols/blast_radius_strangler.go` (NEW — two-pass orchestrator)

**Analog (Pass 2 primitive, untouched):** `internal/kernel/symbols/tools.go:560-585` — existing `registerAnalyzeBlastRadius` calling `AnalyzeBlastRadius` (the LSP primitive at `internal/kernel/symbols/blast.go:21`).

**Existing pure-LSP path being wrapped** (lines 560-583):
```go
func registerAnalyzeBlastRadius(server *mcp.SerenaMCPServer, k *kernel.Kernel,
    wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer,
) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "analyze_blast_radius",
        Description: "Analyze the blast radius (impact) of changing a symbol",
    }, kernel.WrapToolSpan(tracer, "analyze_blast_radius",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args BlastRadiusArgs) (*mcpsdk.CallToolResult, any, error) {
            // ... arg validation + lease acquisition ...
            br, err := AnalyzeBlastRadius(ctx, lease,
                pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol)
            if err != nil {
                return errorResult(err.Error()), nil, nil
            }
            return textResult(formatBlastRadius(br)), nil, nil
        }))
}
```

**Apply pattern:** New file declares the two-pass orchestrator (`AnalyzeBlastRadiusStrangler` or `analyzeWithLookup`). Pseudocode shape per RESEARCH Code Example 2:
1. Check `lookup.Available()` — if false, fall back to the existing `AnalyzeBlastRadius` LSP primitive with `confidence ≤ 0.6` cap (D-08, ROADMAP SC #2).
2. Pass 1: `impacts, err := lookup.ExpandFrom(ctx, ws, sym, depth=2)`. On error → fallback path.
3. Pass 2: filter `crossesPublicAPI || confidence < 0.80` → `lookup.ValidateCriticalEdges`. Apply verdicts (1.00 validated / 0.20 refuted; LSP error → leave Pass 1 untouched, log debug).
4. Format envelope with `source=semantic` + per-node confidence/evidence.

**Pitfall guard (RESEARCH §6):** copy the Pass 1 result before mutating with Pass 2 verdicts — lookup may reuse internal buffers across calls.

---

### `internal/skill/repomap/strangler.go` OR extending `internal/skill/repomap/skill.go` in place (NEW / MODIFIED)

**Analog:** `internal/skill/repomap/skill.go:115-144` — four existing setter precedents (`SetEnrichFn`, `SetFallbackDeps`, `SetMetricsSink`, `SetRegistry`). Phase 65's `SetSemanticLookup` is literally the next entry in this family.

**Setter pattern** (lines 137-144):
```go
// SetMetricsSink wires the repomap.MetricsSink for per-extractor latency
// observation (Q-2 Option 2). Called from internal/daemon/daemon.go post-init
// (block 12d). Phase 53 D-15. A nil argument is normalized to NoopSink{} so
// the dispatcher's emission sites never need nil-checks.
func (s *RepoMapSkill) SetMetricsSink(sink repomap.MetricsSink) {
    if sink == nil {
        sink = repomap.NoopSink{}
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    s.metrics = sink
}
```

**Lookup-consult branch in execGetRepoMap** — extend lines 269-287:
```go
// Existing minimal v1.9 path (lines 269-287):
func (s *RepoMapSkill) execGetRepoMap(args map[string]interface{}) (string, error) {
    budget := extractTokenBudget(args, defaultRepoMapBudget)
    if err := s.ensureGraph(); err != nil {
        return "", serr.Wrap(serr.Internal, "building reference graph", err).WithTool("get_repo_map")
    }
    ranked := s.graph.RankFiles(0.85, nil)
    if len(ranked) == 0 {
        return "No files found in repository.", nil
    }
    output := s.renderer.RenderBudgeted(ranked, budget)
    return output, nil
}
```

**Apply pattern:** Add a `s.semanticLookup` struct field (defaulted to nil, normalized via a `s.lookup() integ.SemanticLookup` helper that returns a no-op stub when nil — mirrors the `metricsSink()` normalization at lines 149-157). Add the consult-on-execute branch BEFORE the existing v1.9 path: if `lookup.Available()`, try `lookup.RankFiles(...)`; on success use semantic ranks, on error fall through to v1.9 with `source=fallback + fallback_reason`. JSON-wrap the existing tree string under `.tree` (RESEARCH Pitfall §2).

**Critical: RepoMapSkill.ExecuteTool signature stays `(string, error)`** — wrap output as JSON-string (do NOT migrate to typed-args). Existing goldens preserve the tree text verbatim under `.tree` after the wrap (CONTEXT.md "index-disabled goldens preserved verbatim" applies to TREE TEXT, not wire format).

---

### `internal/daemon/semantic_wiring.go` (MODIFIED — three changes)

This file gets three concurrent edits; each has its own analog.

#### (a) Replace empty-Facts buildFn at lines 687-742

**Analog:** Inline contract at lines 621-686 + RESEARCH §"Pattern 4" production buildFn (RESEARCH lines 396-490).

**Existing skeleton being replaced** (lines 687-742):
```go
func (b *semanticBundle) makeProductionBuildFn() semantic.RunnerBuildFn {
    return func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st semantic.BuildState) (semantic.IndexResult, error) {
        repoID := ws.Hash()
        startedAt := time.Now()
        var baseSnapshotID uint64
        if mode == "incremental" {
            latest, err := b.store.LatestCommittedSnapshot(ctx, repoID)
            if err != nil { return semantic.IndexResult{}, fmt.Errorf("buildFn: LatestCommittedSnapshot: %w", err) }
            baseSnapshotID = latest
        }
        snap, err := b.store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{RepoID: repoID, BaseSnapshotID: baseSnapshotID})
        // ... defer Abort, SetSnapshotID ...
        // PLACEHOLDER:
        if err := b.store.WriteSnapshotFacts(ctx, snap, semanticstore.Facts{}); err != nil { /*...*/ }
        // ... CommitSnapshot, return IndexResult{FilesIndexed: 0} ...
    }
}
```

**Apply pattern:** Insert per-language extraction + classifier walk + overlay drain BETWEEN `BeginSnapshot` and `WriteSnapshotFacts`. RESEARCH §"Pattern 4" gives the verified concrete pipeline:
1. Walk (full mode) or drain overlay (incremental mode).
2. For each path: `kind, ok, err := live.ClassifyPathChange(ctx, semantic.RepoID(repoID), path, b.store, nil, live.ChangeSourceWatcher)`.
3. Resolve language → `b.extractRegistry.Provider(lang)` → `provider.Extract(ctx, source, extract.SourceFile{...})`.
4. `facts := extract.ToStoreFacts(extracted)` — locked Phase 65 unblock adapter at `internal/semantic/extract/to_store.go:58`.
5. Replace the placeholder `WriteSnapshotFacts(ctx, snap, semanticstore.Facts{})` with `WriteSnapshotFacts(ctx, snap, facts)`.

**ToStoreFacts adapter** (analog reference, `internal/semantic/extract/to_store.go:58-88`):
```go
func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts {
    if len(files) == 0 { return semanticstore.Facts{} }
    out := semanticstore.Facts{
        Files: make([]semanticstore.FileFact, 0, len(files)),
        Symbols: make([]semanticstore.SymbolFact, 0, 4*len(files)),
        References: make([]semanticstore.ReferenceFact, 0, 4*len(files)),
    }
    for _, ef := range files {
        if ef == nil { continue } // defensive nil-skip
        out.Files = append(out.Files, fileFactToStore(ef.File))
        for _, s := range ef.Symbols { out.Symbols = append(out.Symbols, symbolFactToStore(s)) }
        for _, r := range ef.References { out.References = append(out.References, referenceFactToStore(r)) }
    }
    return out
}
```

#### (b) Replace zero-value WorkspaceKey return at lines 509-525

**Analog (existing zero-value return):**
```go
func (a *semSessionAdapter) Workspace(ctx context.Context) workspace.WorkspaceKey {
    if a == nil || a.getSession == nil {
        return workspace.WorkspaceKey{}
    }
    sess := a.getSession(ctx)
    if sess == nil {
        return workspace.WorkspaceKey{}
    }
    // SessionInfo.WorkspaceKey is the hashed repo identifier (a string),
    // not the canonical workspace.WorkspaceKey struct. ... Phase 65 wires
    // a real registry lookup.
    return workspace.WorkspaceKey{}
}
```

**Apply pattern (RESEARCH §"Pattern 3", option (a)):** Add a `wsKeyFn func() workspace.WorkspaceKey` field to `semSessionAdapter`, populated from the daemon's existing `activeWSKey` closure at `internal/daemon/daemon.go:521-523`. Replace the zero return with `return a.wsKeyFn()`. The closure already exists and is passed to `symbols.RegisterTools` at `daemon.go:526` — Phase 65 just threads it through `newSemanticBundle`.

#### (c) Add `integSemanticLookup` adapter alongside `semStoreAdapter` family

**Analog:** `semStoreAdapter` lines 321-357 (verbatim-delegating adapter shape) + `semRetrievalAdapter` lines 530-619 (multi-engine wrapping).

**Verbatim-delegating adapter pattern** (lines 321-357):
```go
// semStoreAdapter wraps *semanticstore.Store with the narrow StoreAccessor
// surface the SemanticSkill consumes. Delegates verbatim to *Store; the
// surface mirrors accessors.go's StoreAccessor interface.
type semStoreAdapter struct {
    store *semanticstore.Store
}

func (a *semStoreAdapter) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
    if a == nil || a.store == nil { return 0, nil }
    return a.store.LatestCommittedSnapshot(ctx, repoID)
}

func (a *semStoreAdapter) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
    if a == nil || a.store == nil { return 0, nil }
    return a.store.CurrentGraphVersion(ctx, repoID)
}
```

**Compile-time guard pattern** (lines 745-755):
```go
var (
    _ semantic.StoreAccessor     = (*semStoreAdapter)(nil)
    _ semantic.SchedulerAccessor = (*semSchedulerAdapter)(nil)
    // ...
)
```

**Apply pattern:** Define `integSemanticLookup struct { bundle *semanticBundle; store *semanticstore.Store; retrieval *semRetrievalAdapter; rank *rankBundle; enabledFn func() bool; wsKeyFn func() workspace.WorkspaceKey }` next to `semSessionAdapter`. Each `SemanticLookup` method nil-guards the receiver and the underlying handle, then delegates. Add a compile-time guard `_ integ.SemanticLookup = (*integSemanticLookup)(nil)`. Status() composes existing accessors:
```go
func (l *integSemanticLookup) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
    if !l.Available() { return integ.SemanticStatus{State: integ.StatusDisabled}, nil }
    repoID := ws.Hash()
    snap, _ := l.store.LatestCommittedSnapshot(ctx, repoID)
    gv, _ := l.store.CurrentGraphVersion(ctx, repoID)
    overlay := l.store.OverlayHasPendingRows(repoID)
    return integ.SemanticStatus{
        State: integ.StatusReady,
        LatestSnapshotID: snap,
        GraphVersion: gv,
        OverlayActive: overlay,
        PendingLSP: l.bundle.queue.DepthAll(ws),
        LastLiveUpdateMs: l.bundle.live.LastFlushAt(ws),
    }, nil
}
```

---

### `internal/kernel/health/tools.go` (MODIFIED — additive `semantic_index` block)

**Analog:** Own file lines 139-181 — existing `RegisterTools(server, k, semProbe SemanticStoreProbe)` with the additive envelope pattern.

**Existing additive envelope pattern** (lines 158-172):
```go
// SC-1: surface semantic store readiness alongside LS health. Use
// an envelope so the existing report shape is preserved verbatim
// and `semantic_store` is additive — downstream consumers parsing
// only `workspaces` are unaffected; new consumers see the field.
semStatus := ComputeSemanticStoreStatus(ctx, semProbe)
envelope := struct {
    *lspool.HealthReport
    SemanticStore SemanticStoreStatus `json:"semantic_store"`
}{
    HealthReport:  report,
    SemanticStore: semStatus,
}

jsonBytes, err := json.MarshalIndent(envelope, "", "  ")
if err != nil {
    return errorResult(fmt.Sprintf("marshaling health report: %v", err)), nil, nil
}
return textResult(string(jsonBytes)), nil, nil
```

**Apply pattern (RESEARCH Code Example 3):** Widen `RegisterTools(server, k, semProbe SemanticStoreProbe, semIndex SemanticIndexAccessor)` (or add it as the next parameter). Embed the new block additively — DO NOT rename `semantic_store` (Pitfall §4: existing test `tools_semantic_test.go:120-128` locks the JSON shape):
```go
envelope := struct {
    *lspool.HealthReport
    SemanticStore SemanticStoreStatus `json:"semantic_store"`            // unchanged (SC-1)
    SemanticIndex SemanticIndexBlock  `json:"semantic_index,omitempty"`  // NEW (INTEG-04)
}{...}
```

`SemanticIndexAccessor` interface mirrors `SemanticStoreProbe` shape (lines 28-31) — kernel-side seam, daemon implements against `integSemanticLookup.Status`. Closed-enum `LastError string` field uses the same WR-NEW-01 discipline (no raw text).

---

### `internal/kernel/symbols/tools.go` (MODIFIED — `registerAnalyzeBlastRadius` accepts lookup)

**Analog:** Own file lines 560-585 — existing function being widened.

**Apply pattern:** Add a `lookupFn func() integ.SemanticLookup` parameter (same closure shape as the existing `wsKeyFn func() workspace.WorkspaceKey`). Inside the handler, branch on `lookupFn() == nil || !lookup.Available()` → existing pure-LSP path; else delegate to the new `blast_radius_strangler.go` orchestrator. The `mcpsdk.AddTool` shell, `kernel.WrapToolSpan`, `errorResult` / `textResult` helpers stay verbatim — only the body branches.

**Same-file precedent for closure-injected dependency:** `wsKeyFn func() workspace.WorkspaceKey` is already threaded through every `register*` function in this file. `lookupFn` is the next sibling.

---

### `internal/lint/nokernel2semantic/analyzer.go` (MODIFIED — allowlist amendment)

**Analog:** Own file lines 33-57 — existing two-const + Run pattern.

**Existing forbidden-prefix check** (lines 33-57):
```go
const checkedPkgPrefix = "github.com/agenthands/helix/internal/kernel"
const forbiddenImportPrefix = "github.com/agenthands/helix/internal/semantic"

var Analyzer = &analysis.Analyzer{
    Name: "nokernel2semantic",
    Doc:  "fails if internal/kernel/* imports internal/semantic/*",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
            return nil, nil
        }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if strings.HasPrefix(path, forbiddenImportPrefix) {
                    pass.Reportf(imp.Pos(),
                        "internal/kernel/* must not import internal/semantic/* (got import %q in %s)",
                        path, pass.Pkg.Path())
                }
            }
        }
        return nil, nil
    },
}
```

**Apply pattern (RESEARCH Pitfall §1, option (a) — recommended):** Add an allowlist constant and amend the predicate:
```go
const allowedSemanticIntegPath = "github.com/agenthands/helix/internal/semantic/integ"

// In Run:
if strings.HasPrefix(path, forbiddenImportPrefix) &&
    !strings.HasPrefix(path, allowedSemanticIntegPath) {
    pass.Reportf(imp.Pos(), "...", path, pass.Pkg.Path())
}
```

~10-line diff total. Update the doc comment on `Analyzer` to mention the allowlisted seam.

---

### `internal/lint/nokernel2semantic/analyzer_test.go` (MODIFIED — analysistest fixture)

**Analog:** Own file (24 LOC, two `analysistest.Run` calls).

**Existing test pattern**:
```go
func TestAnalyzer_RejectsKernelImportingSemantic(t *testing.T) {
    analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
        "github.com/agenthands/helix/internal/kernel/badpkg")
}

func TestAnalyzer_AllowsKernelWithoutSemanticImport(t *testing.T) {
    analysistest.Run(t, analysistest.TestData(), nokernel2semantic.Analyzer,
        "github.com/agenthands/helix/internal/kernel/goodpkg")
}
```

**Apply pattern:** Add `TestAnalyzer_AllowsKernelImportingSemanticInteg` with a fresh fixture under `testdata/src/github.com/agenthands/helix/internal/kernel/integimport/` that imports the allowlisted package. Mirror the existing call shape exactly — same one-line `analysistest.Run` call with the new package path.

---

### `internal/skill/repomap/skill_integration_test.go` (MODIFIED — golden mechanical update)

**Analog:** Own file lines 141-170.

**Existing index-disabled goldens** (lines 141-150):
```go
func TestGetRepoMap_WithWorkspace(t *testing.T) {
    s, _ := newIntegrationSkill(t)
    result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
        "token_budget": float64(4096),
    })
    require.NoError(t, err)
    assert.NotContains(t, result, "No files found", "get_repo_map should return data, not empty message")
    assert.Contains(t, result, ".go", "output should contain Go files")
}
```

**Apply pattern:** RESEARCH Pitfall §2 — the result string is now JSON. Two updates per test:
1. Parse the JSON envelope: `var env struct { Source string `json:"source"`; Tree string `json:"tree"`; FallbackReason string `json:"fallback_reason"` }; require.NoError(t, json.Unmarshal([]byte(result), &env))`.
2. Assert against `env.Tree` (preserves verbatim tree-text contract) and `env.Source == "tree_sitter"` (D-04 — index disabled by config).

Add NEW counterparts: `TestGetRepoMap_WithSemanticEnabled`, `TestGetContext_WithSemanticEnabled` for the semantic-on path.

---

### `internal/daemon/daemon.go` (MODIFIED — post-init wiring)

**Analog:** Existing post-init wiring blocks — `repomapSkill.SetEnrichFn(...)`, `repomapSkill.SetFallbackDeps(...)`, `repomapSkill.SetMetricsSink(...)`, `kernel.RegisterTools(...)`, `health.RegisterTools(server, k, semProbe)`.

**Apply pattern:** Three additive insertions in post-init order:
1. Construct `lookup := &integSemanticLookup{bundle: semBundle, store: semBundle.store, retrieval: ..., rank: rankBundle, enabledFn: func() bool { return cfg.SemanticIndex.Enabled }, wsKeyFn: wsKeyFn}`.
2. Call `repomapSkill.SetSemanticLookup(lookup)`.
3. Pass `lookup` into the kernel-symbols `registerAnalyzeBlastRadius` parameter list (via the existing `symbols.RegisterTools` extension), and into the widened `health.RegisterTools(server, k, semProbe, semIndexAccessor)` signature.

The `wsKeyFn` closure already exists at `daemon.go:521-523` — Phase 65 just consumes it via the new `semSessionAdapter.wsKeyFn` field.

---

### `internal/skill/semantic/integration_test.go` (MODIFIED — extend 15-symbol fixture)

**Analog:** Own file lines 438-463 — `TestE2E_IndexThenContext_SymbolCount` 15-symbol fixture.

**Existing pattern** (lines 438-463):
```go
func TestE2E_IndexThenContext_SymbolCount(t *testing.T) {
    const expectedSymbolCount = 15
    h := newE2EHarness(t, "review", makeFixtureFacts(expectedSymbolCount))
    res := h.skill.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
    if res.IsError { t.Fatalf("index returned error: %s", extractText(t, res)) }
    ir := decodeIndexResult(t, res)
    if ir.SnapshotID == 0 { t.Fatalf("SnapshotID: got 0, want non-zero (build did not commit)") }
    count := 0
    err := h.store.IterateCommittedSymbols(context.Background(), ir.SnapshotID, func(_ semanticstore.SymbolRow) bool {
        count++
        return true
    })
    if err != nil { t.Fatalf("IterateCommittedSymbols: %v", err) }
    if count != expectedSymbolCount { t.Fatalf("committed symbol count: got %d, want %d", count, expectedSymbolCount) }
}
```

**Apply pattern:** Add a sibling `TestE2E_StranglerFig_SourceMatrix` table-driven test exercising every `{source × fallback_reason}` pair: `(semantic, "")`, `(tree_sitter, "")`, `(fallback, no_snapshot_yet)`, `(fallback, index_building)`, `(fallback, index_error)`, `(fallback, bleve_rebuilding)`. Each row drives the same `handleGetSemanticContext` (or the strangler-fig tool) under a different harness state and asserts envelope shape.

---

## Shared Patterns

### Pattern S1: Closed-enum reasons (WR-NEW-01)

**Source:** `internal/kernel/health/tools.go:50-81`
**Apply to:** `internal/semantic/integ/source.go`, `integSemanticLookup` Status() method, every `fallback_reason` emission in `repomap/skill.go` and `kernel/symbols/blast_radius_strangler.go`, `health/tools.go` `LastError` field.

**Excerpt** (already shown above under `internal/semantic/integ/source.go`):
```go
const (
    SemanticReasonProbeTimeout = "probe_timeout"
    SemanticReasonNilHandle    = "nil_handle"
    SemanticReasonDBError      = "db_error"
    SemanticReasonUnknown      = "unknown"
)
```

Discipline: every reason value is a closed-enum constant; `fmt.Sprintf("...%v", err)` NEVER appears in an MCP envelope. Raw error text goes to `slog.Warn` for operators only.

### Pattern S2: Setter post-init wiring with nil-safe normalization

**Source:** `internal/skill/repomap/skill.go:137-157`
**Apply to:** `RepoMapSkill.SetSemanticLookup`, the kernel-symbols-side equivalent (if added on the SymbolsSkill type).

**Excerpt:**
```go
func (s *RepoMapSkill) SetMetricsSink(sink repomap.MetricsSink) {
    if sink == nil {
        sink = repomap.NoopSink{}
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    s.metrics = sink
}

func (s *RepoMapSkill) metricsSink() repomap.MetricsSink {
    s.mu.Lock()
    sink := s.metrics
    s.mu.Unlock()
    if sink == nil {
        return repomap.NoopSink{}
    }
    return sink
}
```

Discipline: setter normalizes nil to a no-op stub at the boundary; emission sites NEVER need nil-checks.

### Pattern S3: Daemon-side adapter with verbatim delegation + compile-time guard

**Source:** `internal/daemon/semantic_wiring.go:321-357` (`semStoreAdapter`) + lines 745-755 (compile-time guards block).
**Apply to:** `integSemanticLookup`.

**Excerpt:**
```go
type semStoreAdapter struct {
    store *semanticstore.Store
}

func (a *semStoreAdapter) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
    if a == nil || a.store == nil { return 0, nil }
    return a.store.LatestCommittedSnapshot(ctx, repoID)
}

// At end of file:
var (
    _ semantic.StoreAccessor = (*semStoreAdapter)(nil)
    // ...
)
```

Discipline: each method nil-guards both the receiver and the wrapped handle. Compile-time interface assertions catch breaking-change drift at build time.

### Pattern S4: Stable-key tiebreak (Phase 62 CR-03)

**Source:** sort-before-iterate doctrine (CONTEXT.md "Locked from prior phases").
**Apply to:** `integSemanticLookup.RankFiles` and `integSemanticLookup.RankFromSeeds` deterministic iteration. Tiebreak ladder: score → graph_version → symbol_id / file_path. Implementation lives in the Phase 62 graph engine; Phase 65 only READS already-sorted rank rows — no new sort code if reading via `semantic_graph_scores` rows. If post-processing combines multiple rank streams, copy the Phase 62 sort.Slice tiebreak block verbatim.

### Pattern S5: Result-shape helpers (`textResult` / `errorResult` / `mcpsdk.AddTool` / `kernel.WrapToolSpan`)

**Source:** `internal/kernel/symbols/tools.go` and `internal/kernel/health/tools.go` (used throughout).
**Apply to:** `internal/kernel/symbols/tools.go` (extension of `registerAnalyzeBlastRadius`), `internal/kernel/health/tools.go` (extension of `RegisterTools`). Reuse unchanged — Phase 65 adds no new helpers.

---

## No Analog Found

None. Every Phase 65 target file has a direct in-tree precedent.

The strangler-fig is composition: every concrete capability the four tools need is already shipped by Phase 57–64 (RESEARCH §"Don't Hand-Roll" enumerates 9 such building blocks). Phase 65's value is the wiring, the closed-enum envelope, and the two-pass orchestrator — every one of which has a directly applicable analog.

---

## Metadata

**Analog search scope:** `internal/skill/repomap/`, `internal/skill/semantic/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/kernel/symbols/`, `internal/daemon/`, `internal/lint/nokernel2semantic/`, `internal/semantic/integ/` (does not exist yet — confirmed), `internal/semantic/extract/`, `internal/semantic/store/`, `internal/semantic/retrieval/`.

**Files scanned (Read calls):**
- `internal/skill/repomap/skill.go` (lines 1-200, 200-360)
- `internal/kernel/health/skill_adapter.go` (whole file)
- `internal/kernel/help/skill_adapter.go` (whole file)
- `internal/kernel/health/tools.go` (lines 1-200)
- `internal/kernel/symbols/tools.go` (lines 540-614)
- `internal/lint/nokernel2semantic/analyzer.go` (whole file)
- `internal/lint/nokernel2semantic/analyzer_test.go` (whole file)
- `internal/daemon/semantic_wiring.go` (lines 1-80, 300-380, 490-755)
- `internal/skill/repomap/skill_integration_test.go` (lines 130-180)
- `internal/skill/semantic/integration_test.go` (lines 420-480)
- `internal/skill/semantic/accessors.go` (whole file)
- `internal/semantic/extract/to_store.go` (lines 1-90)
- `.planning/phases/65-existing-tool-integration-strangler-fig/65-CONTEXT.md` (whole file)
- `.planning/phases/65-existing-tool-integration-strangler-fig/65-RESEARCH.md` (lines 1-400, 400-450)

**Pattern extraction date:** 2026-05-08
