# Phase 59: Tree-sitter Extraction & Stable Symbol IDs — Research

**Researched:** 2026-05-04
**Domain:** Tree-sitter symbol/reference/import/edge extraction for Go/TS+JS/Python; stable symbol-ID design (xxhash64 over canonicalized SCIP-shape key); constructor-injected provider registry; v1→v2 schema migration via Phase 57 D-02; background extraction scheduler with centralized `RequireReady` gate; closed-enum partial-extraction model.
**Confidence:** HIGH on Phase 57 surface (store schema, migration registry, daemon step 6b, GrammarRegistry singleton, xxhash dependency, repomap pattern); HIGH on SPEC §11.1 / §11.2 / §13.4 / §13.5 / §25 contracts; MEDIUM on the **two cross-document inconsistencies between Phase 59 description and SPEC** flagged below; HIGH on tree-sitter Go API and SCIP prior art.

## Summary

Phase 59 is the most prescriptively-specified phase in v1.10 — CONTEXT.md locks D-01..D-05 with a complete deliverable inventory, fact-struct shapes, registry constructor signature, scheduler interface, `ReadyPolicy` defaults, partial-reason enum, schema-delta column list, hard invariants, and an 11-item acceptance-criteria checklist. The planner's job is **layout into waves**, not redesign. Most planner-relevant uncertainty is mechanical ("which package, which file, what task split") plus two **cross-artifact inconsistencies the planner needs to surface to the user before locking plans**:

1. **Confidence-ladder rung-count mismatch.** Phase 59 ROADMAP success criterion #3 says "the documented 7-rung confidence ladder (1.00 LSP-confirmed → 0.20 unknown)". SPEC §11.2 (the symbol-merge ladder Phase 59 actually emits into) is **5 rungs**: `1.00 / 0.95 / 0.80 / 0.70 / 0.45`. The 7-rung ladder (`1.00 / 0.90 / 0.80 / 0.70 / 0.60 / 0.45 / 0.20`) is SPEC §38.2 — **type resolution**, owned by Phase 62. EXTRACT-04 in REQUIREMENTS.md correctly references the 5-rung §11.2 ladder. **Recommendation: plan to §11.2 (5 rungs) as the contract, treat ROADMAP's "7-rung" wording as a transcription error, and note the discrepancy in 59-VERIFICATION.md so it's resolved at milestone close.** [VERIFIED: SPEC-DRAFT.md:998-1004 vs SPEC-DRAFT.md:3921-3928 vs .planning/REQUIREMENTS.md:35]
2. **Config sub-tree placement.** CONTEXT.md introduces `semantic_index.extraction.*` (six new keys: `extraction_ready_timeout`, `extraction_file_timeout`, `max_parallel_files`, `max_file_size`, `allow_partial_results`, `initial_extraction_on_activation`). SPEC §25 has **no** `extraction.*` block — it has `indexing.*` with overlapping keys (`max_file_size`, `auto_index_on_activate`). The Phase 57 typed mirror (`internal/semantic/config.go`) already has `IndexingConfig` with `MaxFileSize` and `AutoIndexOnActivate`. **Two viable shapes:** (a) add a new `Extraction` sub-struct under `Config` for the four genuinely-new keys (`extraction_ready_timeout`, `extraction_file_timeout`, `max_parallel_files`, `allow_partial_results`) and **reuse** the existing `Indexing.MaxFileSize` and `Indexing.AutoIndexOnActivate` (CONTEXT.md's `initial_extraction_on_activation` is the same idea); (b) add a new `Extraction` sub-struct mirroring CONTEXT.md verbatim and accept some duplication. **Recommendation: shape (a) — extend the existing `IndexingConfig` defaults model, add a new `Extraction` sub-struct only for the four genuinely-new keys, and update SPEC §25 in the same plan.** [VERIFIED: SPEC-DRAFT.md:2518-2526 vs internal/semantic/config.go:86-95 vs CONTEXT.md "New config keys"]

Beyond those two flags, the research surfaces three planner-relevant facts:

3. **The v1→v2 schema migration is non-trivial.** Phase 57's `semantic_files` has `path, language, content_hash, size_bytes, line_count, generated, ignored, ignore_reason, indexed_at` — but **no** `extraction_status, extraction_partial, partial_reason, extractor_name, extractor_version, error_message`. `semantic_symbols` has `extraction_source, confidence` but no `partial, partial_reason`. `semantic_references` has `validation_state, confidence, reason` but no `partial, partial_reason`. CONTEXT.md D-05's "Note for the planner" calls this out — **per-column ALTER-add vs. repurpose-existing requires a migration-design subtask**. The Phase 57 D-02 `Migration{From, To, Kind}` registry exists but currently has only `applyMigration001` wired; the `Apply` field is stubbed with a TODO (`migrations_types.go:37`). Phase 59 lights up the registry for the first time. [VERIFIED: internal/semantic/store/migrations.go, internal/semantic/store/migrations_types.go]
4. **GrammarRegistry singleton is real and structurally enforceable.** Daemon step 6 creates `grammarRegistry := treesitter.NewGrammarRegistry()` exactly once (daemon.go:233). Repomap (12a, line 315), edit body extractor (line 234), and Phase 59 extractor registry (new wiring) all receive the same pointer. EXTRACT-05 acceptance criterion #11 "regression test attempts to construct a second `GrammarRegistry` inside the semantic extractor path and fails" can be implemented as a `go vet`-style import-graph check OR as a runtime test that grep-scans `internal/semantic/extract/` source for `treesitter.NewGrammarRegistry`. **Recommendation: combine both — runtime test for the executed code path, source-grep test for the discoverable surface.** [VERIFIED: internal/daemon/daemon.go:233, internal/treesitter/registry_cgo.go:53]
5. **Stable-ID design has one significant precedent (Sourcegraph SCIP) and it validates the Phase 59 approach.** SCIP encodes symbols as `<scheme> <package> <descriptor>+` where descriptors are `namespace/`, `type#`, `method(disambiguator).`, `parameter()`, etc. Phase 59's `StableSymbolKey{RepoID, Language, PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash, LSPIdentity, FilePathFallback}` is structurally a SCIP scheme with `RepoID+Language` as the scheme/manager prefix, `PackagePath` as the package, `OwnerPath+QualifiedName+Kind` as the descriptor chain, and `SignatureHash` playing the role of SCIP's `method-disambiguator` for overloads. `xxhash64` is the canonical hash, not part of SCIP — SCIP keeps the symbol as a literal string for cross-tool readability. **Helix's choice of a hashed ID instead of a textual symbol is a deliberate divergence: it gains fixed-size storage, fast equality, and easy SQL indexing at the cost of human-readable IDs in DuckDB rows.** SCIP also explicitly does NOT address rename/move identity survival ("These concerns fall outside the specified grammar's scope") — Phase 59's five canonicalization rules in SPEC §11.1 are **net-new contract surface for Helix**, not borrowed from SCIP. [CITED: github.com/sourcegraph/scip/blob/main/docs/scip.md, VERIFIED: SPEC-DRAFT.md:944-981]

**Primary recommendation:** Plan Phase 59 as **5 plans, executed in 3 waves**:

- **Wave 0 (independent prep):** P01 — schema migration v1→v2 (modifies Phase 57 store; reviewable in isolation; lights up the migration registry).
- **Wave 1 (parallel):** P02 — `internal/semantic/extract/` package skeleton (`provider.go`, `fact.go`, `normalizer.go`, `classifier.go`, `registry.go`, `stable_id.go`) with no per-language providers yet, plus the bounded-label metric and the new `extraction.*` config keys. P03 — `internal/semantic/scheduler/` package (`ExtractionScheduler` interface, initial-extraction implementation, `RequireReady` helper, `SemanticIndexState` enum). P02 and P03 can run in parallel because P02 owns the `extract.Registry` ctor signature and P03 consumes the registry interface.
- **Wave 2 (parallel after Wave 1):** P04 — three per-language providers (`internal/semantic/extract/{go,typescript,python}/`) with golden + table-driven tests; ≥30 scenarios per language. P05 — daemon bootstrap wiring + `kernel.ActivateWorkspace` callback + EXTRACT-05 regression test + per-feature defaults test. P05 depends on P02 (registry shape) and P03 (scheduler interface) but not on P04 (providers compile independently).

This split keeps the **180+ test scenarios** (≥30 each × 3 languages × ~2 modes) inside one focused plan (P04) so reviewer attention concentrates on the contract that all of v1.10 depends on, while the wiring/migration/scheduler work proceeds independently.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01: Net-new queries, hard split.** `internal/semantic/extract/` is a distinct subsystem from `internal/repomap/`. Per-language queries (`extract/go/queries.scm`, `extract/typescript/queries.scm`, `extract/python/queries.scm`) are written from scratch against the SPEC §13.4 capture vocabulary plus the Phase 59 capture additions. **Hard invariants:**
  - No `import "github.com/agenthands/helix/internal/repomap"` inside `internal/semantic/extract/`.
  - No copy of `internal/repomap/queries/*_tags.scm` into `internal/semantic/extract/`.
  - Repomap's `TagExtractor` / `Tag` types are not consumed, wrapped, or adapted by the semantic extractor.
- **D-01a: Full capture set per first-class language.** Provider `queries.scm` for Go / TS+JS / Python emit (where syntactically present): `definition.{function,method,struct,class,interface,enum,type,variable,parameter,field,constant}`, `reference.{call,identifier,field,type}`, `import.{source,alias,symbol}`, `type.{annotation,return,parameter}`, `heritage.{extends,implements}`, `decorator.name`, `receiver.{type,name}`. Captures absent from a language are simply unused.
- **D-01b: Net-new fact structs in `internal/semantic/extract/fact.go`** — `SymbolFact, ReferenceFact, ImportFact, TypeFact, HeritageFact` per the verbatim CONTEXT.md schemas. `SymbolID, ReferenceID` already exist in `internal/semantic/types.go`; Phase 59 adds `ImportID, TypeFactID, HeritageID`.
- **D-02: Constructor-injected provider registry — no init() / no blank imports.** `extract.NewExtractorRegistry(grammars *treesitter.GrammarRegistry, providers ...Provider) *Registry`. Registry panics on duplicate `Language()`. Daemon bootstrap (post step 6b) constructs each provider explicitly. Hard invariants: no `init()` registration in any `internal/semantic/extract/...` package; no blank-import wiring file; the shared `*treesitter.GrammarRegistry` is injected before any provider is usable.
- **D-03: Hybrid — golden snapshots + table-driven stable-ID tests.** Each language provider owns `provider.go, queries.scm, provider_test.go (golden), stable_id_test.go (table-driven), testdata/<scenario>/{before,after,expected.json}`. Golden test contract: `-update` flag for regeneration, `cmp.Diff` on fail, deterministic JSON normalization (sort by `(file, range, kind, name)`, relative paths only, two-space indent, no tree-sitter node IDs). Stable-ID test contract: table-driven, inline `before/after` source strings (`//go:embed` for >50 lines), `StableIDExpectation` enum (`Preserved | Churned | NewlyDefined | Removed`), helper `assertStableIDExpectations(t, beforeFacts, afterFacts, []StableIDCheck)`. **Scenario count per first-class language: ≥30 split as 25 shared + 8-9 language-specific + ≥10 stable-ID transitions.**
- **D-04: Background extraction on workspace activation, with centralized ready-gate.** `kernel.ActivateWorkspace` does NOT block on full extraction. `SemanticIndexState` enum: `not_started|indexing|ready|partial|failed|stale`. `ExtractionScheduler` interface ships `ScheduleInitialExtraction (working), ScheduleIncremental (Phase 60 fills), Status, Subscribe`. Scheduler invariants: one active job per workspace, `ScheduleInitialExtraction` idempotent, file changes during in-flight initial queued for incremental, jobs cancellable on deactivation. **Centralized `semantic.RequireReady(ctx, workspaceID, ReadyPolicy) (ReadyResult, error)` is the only readiness API; no per-tool `time.Sleep` polling.** `ReadyPolicy` defaults: `Timeout = extraction_ready_timeout (30s)`, `AllowPartial = true`, `MinState = SemanticPartial`, `TriggerIfCold = true`. **Initial-walk priority order:** in-flight tool referenced files → repomap-ranked important files → remaining first-class source → non-first-class. **Non-semantic tools** (read_file, find_symbol-via-LSP, search, repomap tools) DO NOT call `RequireReady`.
- **D-05: Two-tier partial.** Non-first-class languages: `semantic_files` row only with `extraction_status="unsupported", extraction_partial=true, partial_reason="unsupported_language"`, no symbol/ref/import/edge rows. First-class with errors: row + all safely-extracted facts + per-file (and where applicable per-symbol/per-ref) `partial=true` + reason. **Closed enum:** `unsupported_language | parse_error | query_error | timeout | file_too_large | binary_or_generated | permission_denied | extractor_bug`. **Schema delta:** v1→v2 migration via Phase 57 D-02 registry adds `extraction_status, extraction_partial, partial_reason, extractor_name, extractor_version, error_message` columns to `semantic_files / semantic_symbols / semantic_references`. **Consumer-side enum:** `FileSemanticAvailability ∈ {ready, partial, unsupported, failed, missing}`. **Hard invariant: NO repomap fallback inside `internal/semantic/extract/`.**

### Claude's Discretion (per CONTEXT.md)

- **Stable-ID hash function:** `xxhash64` per SPEC §11.1 verbatim. Canonicalization joins `RepoID, Language, PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash, LSPIdentity, FilePathFallback` with `\x00`. Tie-break: "preserve first-seen, log warning".
- **Per-language `provider.go` shape:** `goextract.Provider, tsextract.Provider, pyextract.Provider` are concrete types implementing `extract.Provider`. Constructor takes `*treesitter.GrammarRegistry`, returns `extract.Provider`. Per-package state limited to precompiled query handles and an optional `querycache.Cache`.
- **TS+JS share a provider** with one `Language() = "typescript"` and `Extensions() = [.ts, .tsx, .js, .jsx, .mjs, .cjs]`. Splittable later if overlap proves smaller than expected.
- **Scheduler home:** `internal/semantic/scheduler/` suggested; planner's call as long as outside `internal/semantic/extract/`.
- **Initial-walk concurrency:** `max_parallel_files=4` default (matches existing repomap walker).
- **Per-file extraction timeout:** `extraction_file_timeout=3s` default.
- **Max file size:** `max_file_size=2 MiB` default; larger files get `partial_reason="file_too_large"` and no extraction attempted.
- **Bounded-label metric:** `helix_semantic_extraction_total{language, outcome}` where `language ∈ {go, typescript, python, other}` and `outcome ∈ {ready, partial, unsupported, failed}`.
- **No `helix index` CLI subcommand** in Phase 59.

### Deferred Ideas (OUT OF SCOPE)

- `helix index` CLI subcommand (`helix index`, `helix index --rebuild`, `helix index --status`) — post-Phase-64.
- Repomap fallback as best-effort enrichment provider (`internal/semantic/enrich/repomapfallback/`) — post-v1.10.
- Generic semantic-schema key-coverage matrix — inherited deferral from Phase 57.
- Multi-provider per language (TS / JS split) — future phase if overlap proves smaller than expected.
- Cross-file LSP-resolved references in fact rows (`ReferenceFact.ResolvedTarget`, `ResolutionSource`) — Phase 61.
- Stable-ID generic-instantiation handling (one ID per declaration, not per instantiation) — Phase 62 if ever.
- `semantic_graph_status` MCP tool wrapper — Phase 64 (Phase 59 ships `Status()` / `Subscribe()` API).
- Live overlay `overlay_epoch` interaction with extraction state — Phase 60.
- Hard total-extraction budget — future, only if monorepo runs need it.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EXTRACT-01 | Tree-sitter extraction produces typed symbols, references, imports, syntax edges for Go/TS+JS/Python; non-supported languages emit `partial:true` marker. | D-01a capture taxonomy, D-01b fact shapes, D-05 two-tier partial model + closed reason enum, schema delta in §"Schema Migration" below. |
| EXTRACT-02 | Stable symbol IDs use SPEC §11.1 contract; survive whitespace, file rename (same content hash), exported-symbol move (same qualified name). | §"Stable-ID Design" below — canonicalization rules, xxhash64 over `\x00`-joined fields, comparison to SCIP prior art, deliberate divergence from SCIP textual scheme. |
| EXTRACT-03 | 30+ before/after test matrix per first-class language across overload signatures, generics, anonymous closures, decorators (Python), method-on-receiver renames; identity transitions explicit. | D-03 hybrid test layout, scenario taxonomy in §"Test Matrix Design" below, `StableIDExpectation` enum, golden + table-driven split. |
| EXTRACT-04 | Tree-sitter and LSP facts merge per SPEC §11.2 confidence ladder (1.00 LSP-confirmed, 0.95 merged, 0.80 ts+local, 0.70 ts-only, 0.45 heuristic). | SPEC §11.2 verbatim — **5 rungs**, NOT 7. ROADMAP wording is wrong; see Summary finding #1. Phase 59 emits Confidence=0.70 baseline; Phase 61 worker raises during merge. |
| EXTRACT-05 | Extraction reuses single canonical `GrammarRegistry` injected from daemon bootstrap; no duplicate registries; BUG-04 invariant preserved with regression test. | D-02 constructor-injected registry, structural enforcement via daemon ownership of singleton, dual regression test (runtime + source-grep) recommended in Summary finding #4. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Per-language tree-sitter parsing + capture extraction | Layer 1 (Code Intelligence Kernel) — `internal/semantic/extract/` | `internal/treesitter/` (grammar provider) | Pure semantic-fact production; no MCP surface; consumes injected `GrammarRegistry`. |
| Stable symbol-ID canonicalization + xxhash | Layer 1 — `internal/semantic/extract/stable_id.go` | — | Algorithm-only; no I/O; reused by every fact emitter and every consumer (graph, overlay, MCP tools). |
| Provider registry + dispatch by language | Layer 1 — `internal/semantic/extract/registry.go` | Layer 0 daemon bootstrap (constructor-injection site) | Same constructor-injection pattern as `langregistry.NewInstaller`; daemon owns the singleton. |
| Initial-walk extraction scheduler + ready-gate | Layer 1 — `internal/semantic/scheduler/` (or chosen path under `internal/semantic/`) | Layer 0 daemon bootstrap + `kernel.ActivateWorkspace` callback | Scheduler reads repomap PageRank for priority ordering — allowed to import repomap (D-01 forbids only `internal/semantic/extract/` from doing so). |
| Schema migration v1→v2 | Layer 1 — `internal/semantic/store/` (Phase 57 ownership) | — | Plugs into existing `Migration{From, To, Kind}` registry; light-up of the registry that Phase 57 D-02 created but never exercised. |
| New config keys (`extraction.*` sub-tree) | Layer 3 — `internal/config/defaults.go` + `internal/semantic/config.go` | SPEC §25 update in same plan | koanf 4-layer precedence; per-feature defaults test mirrors Phase 57 D-09. |
| Bounded-label metric `helix_semantic_extraction_total` | Layer 1 — `internal/obs/` registration | — | Closed-enum labels per Phase 57 D-07. |
| EXTRACT-05 BUG-04 regression test | Layer 1 — `internal/semantic/extract/` test + `internal/daemon/` test | — | Dual: runtime test asserts injected pointer equality; source-grep test asserts no `treesitter.NewGrammarRegistry` call inside `internal/semantic/extract/`. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/tree-sitter/go-tree-sitter` | v0.25.0 | Parser, query compilation, query cursor, capture iteration | Already in `go.mod` (line 27); used by `internal/repomap/` and `internal/kernel/edit/`. **Phase 59 reuses, does NOT bump.** [VERIFIED: go.mod] |
| `github.com/cespare/xxhash/v2` | v2.3.0 | `xxhash.Sum64(b []byte) uint64` for stable symbol IDs | Already an indirect dependency (go.mod:65); Phase 59 promotes to direct. SPEC §11.1 prescribes `xxhash64` verbatim. [VERIFIED: go.mod, SPEC-DRAFT.md:977-979] |
| `*treesitter.GrammarRegistry` | internal | Sole source of compiled grammars; injected from daemon | BUG-04 singleton, EXTRACT-05 invariant. 23 grammars wired in `registry_cgo.go`. Phase 59 needs only `go, typescript, tsx, python` from the existing set. [VERIFIED: internal/treesitter/registry_cgo.go:53-99] |
| `internal/semantic/store` | Phase 57 | Persistence target for facts; owns DuckDB driver | STORE-06 invariant: only `internal/semantic/store/` may import `duckdb-go`. Phase 59 lands writes via the existing `Migration` registry + new fact-write APIs (which the planner names; not in this scope). [VERIFIED: internal/semantic/store/migrations.go, internal/semantic/store/migrations_types.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/treesitter` | internal | `GrammarRegistry.GetLanguage(lang) (*tree_sitter.Language, bool)` | Every `provider.go` constructor calls this once per language and caches the `*Language` result. |
| `internal/obs` | internal | Bounded-label metric registration | New `helix_semantic_extraction_total{language, outcome}` counter; closed-enum allowlist per Phase 57 D-07. |
| `internal/config` koanf 4-layer | internal | New `semantic_index.extraction.*` keys | `defaults.go` registration + `SerenaConfig.SemanticIndex.Extraction` field + `TestLoad_SemanticExtractionDefaults` test. |
| `internal/repomap` PageRank | internal | Read-only consumer for initial-walk priority order | **Allowed import** for `internal/semantic/scheduler/`; **forbidden import** for `internal/semantic/extract/` per D-01 hard invariant. |
| `internal/errors` `serr` | internal | `serr.Unsupported` for non-first-class languages on consumer side | Used when a tool explicitly errors out for unsupported languages (consumer-side, not extractor-side). |
| `github.com/google/go-cmp/cmp` | already present | Golden test diff output | `cmp.Diff` on `expected.json` mismatch; used elsewhere in repo for test snapshots. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `xxhash64` | SHA-1 truncated to 64 bits | SHA-1 collision-resistant but 6× slower per byte; SPEC §11.1 already locked `xxhash64`. No reason to deviate. |
| Constructor-injected registry | Caddy-style `init()` registration (matches `internal/skill/`) | **Rejected by D-02.** Init() registration creates "registered but not configured" temporal invalid state for `*GrammarRegistry`-dependent providers. |
| Net-new queries per language | Borrow `internal/repomap/queries/<lang>_tags.scm` | **Rejected by D-01.** Repomap queries are navigation-oriented (function/method/type names + call/type refs only — see go_tags.scm:1-19); semantic extraction needs SPEC §13.4 captures (parameters, returns, decorators, receivers, heritage, imports). Borrowing locks semantic into a navigation schema. |
| TS+JS one provider | Two providers (`tsextract` and `jsextract`) | Discretion — query overlap is large enough to justify one. Splittable later per CONTEXT.md if overlap proves smaller than expected. |
| New `Extraction` config sub-struct | Extend existing `IndexingConfig` with new fields | **Recommended hybrid:** keep `Indexing.MaxFileSize` and `Indexing.AutoIndexOnActivate` (already present), add new `Extraction` sub-struct only for the four genuinely-new keys. See Summary finding #2. |

**Installation / wire-up:**

```bash
# No new external dependency installs.
# Promote xxhash/v2 from indirect to direct via:
go mod tidy   # after first xxhash import in internal/semantic/extract/stable_id.go
```

**Version verification:**

```bash
# go-tree-sitter — already at v0.25.0; do not bump.
go list -m -json github.com/tree-sitter/go-tree-sitter
# xxhash — already at v2.3.0; latest as of training data.
go list -m -json github.com/cespare/xxhash/v2
```

## Architecture Patterns

### System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Daemon Bootstrap (internal/daemon/daemon.go) — step 6/6b ordering            │
│                                                                              │
│   step 6a: CGO=0 refusal                                                     │
│   step 6b: semanticstore.Open(cfg.SemanticIndex)              [Phase 57]     │
│   step 6 : grammarRegistry := treesitter.NewGrammarRegistry() [SINGLETON]    │
│   step 6c: extract.NewExtractorRegistry(grammarRegistry,      [PHASE 59]     │
│              goextract.NewProvider(grammarRegistry),                         │
│              tsextract.NewProvider(grammarRegistry),                         │
│              pyextract.NewProvider(grammarRegistry),                         │
│            )                                                                 │
│   step 6d: scheduler.New(extractRegistry, semanticStore, cfg) [PHASE 59]     │
│   step 12a: repomap.SetRegistry(grammarRegistry)             [existing]      │
│                                                                              │
└────────┬────────────────────────────────────────────────────────────┬────────┘
         │                                                            │
         │ kernel.ActivateWorkspace(ctx, req)                         │
         ▼                                                            │
┌────────────────────────────────────────────┐                        │
│ Workspace Activation [LazyInitMiddleware]  │                        │
│                                            │                        │
│   workspaceManager.Open()                  │                        │
│   semanticStoreManager.Open(ws.ID)         │                        │
│   watcherManager.Start(ws)      [Phase 60] │                        │
│   semanticScheduler.ScheduleInitialExtraction(ws.ID, {...}) ───────►│
│   return ws  (DOES NOT block on extract)   │                        │
└────────────────────────────────────────────┘                        │
                                                                      ▼
              ┌──────────────────────────────────────────────────────────────┐
              │ ExtractionScheduler (internal/semantic/scheduler/)           │
              │                                                              │
              │   one job per workspace; idempotent ScheduleInitialExtraction│
              │   priority order:                                            │
              │     1. files referenced by in-flight tool call               │
              │     2. repomap-ranked important files (PageRank)             │
              │     3. remaining first-class source                          │
              │     4. non-first-class (file-row only, D-05)                 │
              │   max_parallel_files=4, extraction_file_timeout=3s/file      │
              │                                                              │
              └────┬─────────────────────────────────────────────────────────┘
                   │  for each file (worker pool):
                   ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │ ExtractFile(ctx, provider, file) [SPEC §13.5]                         │
   │                                                                       │
   │   tree   := parser.Parse(file.Content)         ── parse_error → partial│
   │   caps   := RunQueries(tree, provider.Queries())── query_error → partial│
   │   scopes := provider.ScopeBuilder.Build(...)                          │
   │   syms   := ExtractSymbols(file, caps, scopes)                        │
   │   refs   := ExtractReferences(file, caps, scopes)                     │
   │   imps   := ExtractImports(file, caps)                                │
   │   syms = provider.SymbolNormalizer.Normalize(syms)                    │
   │   refs = provider.ReferenceClassifier.Classify(refs, scopes)          │
   │   for each sym: sym.ID = StableSymbolID(BuildKey(sym))                │
   │   syntaxEdges := BuildSyntaxEdges(syms, refs, imps)                   │
   │   return ExtractedFile{File, Symbols, Refs, Imports, SyntaxEdges}     │
   │                                                                       │
   └────┬──────────────────────────────────────────────────────────────────┘
        │
        ▼
   ┌────────────────────────────────────────────────────────────────────┐
   │ semanticstore.Write*(snapshotID, ExtractedFile) [Phase 59 surface] │
   │   semantic_files (extraction_status, extraction_partial, ...)      │
   │   semantic_symbols (stable_key, signature_hash, confidence=0.70)   │
   │   semantic_references (validation_state="syntactic", confidence)   │
   │   semantic_edges (syntax edges only; resolved edges are Phase 62)  │
   └────┬───────────────────────────────────────────────────────────────┘
        │
        │  scheduler.Status(workspaceID) updated → SemanticIndexState
        ▼
   ┌────────────────────────────────────────────────────────────────────┐
   │ semantic.RequireReady(ctx, ws, ReadyPolicy) — sole readiness API   │
   │   ready    → return immediately                                    │
   │   partial  → return (AllowPartial=true)                            │
   │   indexing → wait until ready|partial|failed or Timeout            │
   │   not_started → if TriggerIfCold, kick scheduler then wait         │
   │   timeout  → return best-partial or not_ready error                │
   └────────────────────────────────────────────────────────────────────┘
        ▲
        │ called by Phase 64 MCP tool wrappers (NOT by Phase 59 itself)
        │ NOT called by read_file / find_symbol / search / repomap tools
        │
   ┌────┴───────────────────────────────────────────────────────────────┐
   │ Phase 61: LSP Enrichment Worker (downstream — out of Phase 59)     │
   │   reads ts-only facts, runs LSP, raises confidence per §11.2 ladder│
   │   updates ResolvedTarget, ResolutionSource on ReferenceFact        │
   └────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/semantic/
├── config.go                                        # existing — add Extraction sub-struct
├── types.go                                         # existing — add ImportID, TypeFactID, HeritageID
├── store/                                           # existing — v1→v2 migration extends here
│   ├── migrations.go                                # add applyMigration002
│   └── migrations_types.go                          # set Apply field, append Migration{From:1,To:2,Kind:InPlace}
├── extract/                                         # NEW — D-01 hard split from internal/repomap
│   ├── doc.go
│   ├── provider.go                                  # Provider interface (mirrors SPEC §13.3 LanguageProvider)
│   ├── fact.go                                      # SymbolFact, ReferenceFact, ImportFact, TypeFact, HeritageFact
│   ├── normalizer.go                                # shared normalizer helpers
│   ├── classifier.go                                # shared reference-classifier helpers
│   ├── registry.go                                  # NewExtractorRegistry(grammars, providers...) *Registry
│   ├── stable_id.go                                 # StableSymbolKey, CanonicalizeStableSymbolKey, StableSymbolID
│   ├── stable_id_test.go                            # canonicalization unit tests
│   ├── querycache.go                                # optional precompiled-query cache
│   ├── extract_cgo.go / extract_nocgo.go            # CGO=0 stub mirror per Phase 51.1 / Phase 57 D-04
│   ├── go/
│   │   ├── provider.go                              # goextract.NewProvider(grammars) extract.Provider
│   │   ├── queries.scm                              # NET-NEW; covers SPEC §13.4 + receivers/generics
│   │   ├── provider_test.go                         # golden test driver, -update flag
│   │   ├── stable_id_test.go                        # table-driven, ≥10 stable-ID transitions
│   │   └── testdata/<scenario>/{before,after,expected.json}
│   ├── typescript/
│   │   ├── provider.go                              # tsextract.NewProvider; Language()="typescript"; Extensions={.ts,.tsx,.js,.jsx,.mjs,.cjs}
│   │   ├── queries.scm                              # NET-NEW; query alternation handles TS-specific (interface, decorators, generics)
│   │   ├── provider_test.go
│   │   ├── stable_id_test.go
│   │   └── testdata/<scenario>/...
│   └── python/
│       ├── provider.go                              # pyextract.NewProvider(grammars) extract.Provider
│       ├── queries.scm                              # NET-NEW; decorators, dataclasses, async, type hints
│       ├── provider_test.go
│       ├── stable_id_test.go
│       └── testdata/<scenario>/...
└── scheduler/                                       # NEW — extraction scheduler + RequireReady gate
    ├── doc.go
    ├── scheduler.go                                 # ExtractionScheduler interface + initial-walk impl
    ├── state.go                                     # SemanticIndexState enum, SemanticStatus struct
    ├── ready.go                                     # RequireReady, ReadyPolicy, ReadyResult
    ├── priority.go                                  # initial-walk priority (consumes repomap PageRank)
    └── scheduler_test.go
```

### Pattern 1: Provider Constructor Pattern (mirrors `langregistry.NewInstaller`, NOT skill init())

**What:** Each per-language provider package exports a single constructor that takes `*treesitter.GrammarRegistry` and returns the `extract.Provider` interface. No `init()`, no blank imports.

**When to use:** Always. This is D-02 hard invariant.

**Example:**

```go
// internal/semantic/extract/go/provider.go
package goextract

import (
    _ "embed"

    tree_sitter "github.com/tree-sitter/go-tree-sitter"

    "github.com/agenthands/helix/internal/semantic/extract"
    "github.com/agenthands/helix/internal/treesitter"
)

//go:embed queries.scm
var goQueries string

type Provider struct {
    grammar *tree_sitter.Language
    query   *tree_sitter.Query
}

// NewProvider compiles the embedded query against the injected grammar.
// Caller MUST pass the daemon-singleton GrammarRegistry (BUG-04 / EXTRACT-05).
func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
    g, ok := grammars.GetLanguage("go")
    if !ok {
        // panic on construction — daemon owns the registry; missing grammar is a build bug.
        panic("goextract: 'go' grammar missing from injected GrammarRegistry")
    }
    q, err := tree_sitter.NewQuery(g, goQueries)
    if err != nil {
        panic("goextract: query compile failed: " + err.Message)
    }
    return &Provider{grammar: g, query: q}
}

func (p *Provider) Language() string      { return "go" }
func (p *Provider) Extensions() []string  { return []string{".go"} }
// ... TreeSitterLanguage(), Queries(), ImportResolver(), ScopeBuilder(),
//     SymbolNormalizer(), ReferenceClassifier(), SupportsLSPEnrichment()
```

### Pattern 2: Constructor-Injected Registry (mirrors langregistry, NOT skill registry)

```go
// internal/semantic/extract/registry.go
package extract

import (
    "fmt"

    "github.com/agenthands/helix/internal/treesitter"
)

type Registry struct {
    grammars  *treesitter.GrammarRegistry
    providers map[string]Provider           // keyed by Provider.Language()
}

// NewExtractorRegistry constructs a registry from explicitly-passed providers.
// Panics on duplicate Provider.Language() — the daemon owns the singleton and
// passing duplicates is a wiring bug, not a runtime condition.
func NewExtractorRegistry(grammars *treesitter.GrammarRegistry, providers ...Provider) *Registry {
    if grammars == nil {
        panic("extract.NewExtractorRegistry: nil GrammarRegistry")
    }
    r := &Registry{
        grammars:  grammars,
        providers: make(map[string]Provider, len(providers)),
    }
    for _, p := range providers {
        if _, dup := r.providers[p.Language()]; dup {
            panic(fmt.Sprintf("extract.NewExtractorRegistry: duplicate provider for language %q", p.Language()))
        }
        r.providers[p.Language()] = p
    }
    return r
}

func (r *Registry) Provider(lang string) (Provider, bool) {
    p, ok := r.providers[lang]
    return p, ok
}
```

### Pattern 3: CGO=0 Stub Mirror (Phase 51.1 / Phase 57 D-04)

```go
// internal/semantic/extract/go/provider_nocgo.go
//go:build !cgo

package goextract

import (
    "github.com/agenthands/helix/internal/semantic/extract"
    "github.com/agenthands/helix/internal/treesitter"
)

func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
    return nil // daemon refuses to start on CGO=0 at step 6a; this never runs.
}
```

### Pattern 4: Stable-ID Canonicalization

```go
// internal/semantic/extract/stable_id.go
package extract

import (
    "strings"

    "github.com/cespare/xxhash/v2"

    "github.com/agenthands/helix/internal/semantic"
)

type StableSymbolKey struct {
    RepoID           string
    Language         string
    PackagePath      string
    OwnerPath        string
    QualifiedName    string
    Kind             string
    SignatureHash    string
    LSPIdentity      string
    FilePathFallback string
}

// CanonicalizeStableSymbolKey joins fields with NUL bytes in the SPEC §11.1
// order. Field order is normative — never reorder; new fields append only.
func CanonicalizeStableSymbolKey(k StableSymbolKey) string {
    var b strings.Builder
    b.Grow(256)
    fields := [...]string{
        k.RepoID, k.Language, k.PackagePath, k.OwnerPath,
        k.QualifiedName, k.Kind, k.SignatureHash,
        k.LSPIdentity, k.FilePathFallback,
    }
    for i, f := range fields {
        if i > 0 {
            b.WriteByte(0x00)
        }
        b.WriteString(f)
    }
    return b.String()
}

func StableSymbolID(k StableSymbolKey) semantic.SymbolID {
    return semantic.SymbolID(xxhash.Sum64String(CanonicalizeStableSymbolKey(k)))
}
```

### Anti-Patterns to Avoid

- **Calling `treesitter.NewGrammarRegistry()` inside `internal/semantic/extract/*`.** Violates EXTRACT-05 / BUG-04. Construction must happen exactly once in `internal/daemon/daemon.go` step 6.
- **Importing `internal/repomap` from `internal/semantic/extract/*`.** Violates D-01 hard invariant. Repomap may be imported only from `internal/semantic/scheduler/` (for PageRank-priority order) and from `internal/skill/repomap/`.
- **Copying `internal/repomap/queries/*_tags.scm` into `internal/semantic/extract/<lang>/queries.scm`.** Violates D-01. Repomap captures are a strict subset of SPEC §13.4 — copying locks semantic into a navigation-oriented schema (see go_tags.scm: 5 captures vs. SPEC §13.4: 18+ captures).
- **`init()` registration in any per-language provider package.** Violates D-02. `Caddy-style registry` pattern is correct for `internal/skill/`, wrong for semantic extraction (real runtime dependencies on `*GrammarRegistry`).
- **Per-tool readiness-polling loops with `time.Sleep`.** Violates D-04 acceptance criterion #9. All semantic consumers MUST go through `semantic.RequireReady`.
- **Returning a `partial:true` row from a non-first-class language handler when no facts were extracted.** Use `extraction_status="unsupported"` with `partial_reason="unsupported_language"` per D-05; this distinguishes "file seen but unsupported" from "first-class language with parse error" — different consumer-side handling.
- **Treating `confidence` as a free float.** SPEC §11.2 enumerates 5 valid values: `1.00 / 0.95 / 0.80 / 0.70 / 0.45`. Phase 59 emits exactly `0.70` for tree-sitter-only facts; do not invent intermediate values. The 7-rung type-resolution ladder (§38.2) is Phase 62 territory and does NOT apply here.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 64-bit symbol-ID hash | Custom hash function (FNV-1a, sip-hash-truncated, MD5-truncated) | `xxhash.Sum64String` from `github.com/cespare/xxhash/v2` | SPEC §11.1 prescribes `xxhash64`. Custom hashes invalidate the contract and require their own collision study. |
| Tree-sitter query parsing | Hand-written S-expression parser | `tree_sitter.NewQuery(lang, src)` | Built into go-tree-sitter; handles predicates, captures, alternation, quantifiers per upstream tree-sitter grammar. |
| Tree-sitter `Query` lifetime | Manual `runtime.SetFinalizer` cleanup | Constructor-acquired `*Query` cached on Provider, `Close()` on shutdown (rarely; provider lives for daemon lifetime) | go-tree-sitter README: "Objects that allocate memory from C, including Query and QueryCursor, must always have Close called." Daemon-lived providers don't churn; per-extraction `QueryCursor` is `defer cursor.Close()` per repomap pattern. [CITED: github.com/tree-sitter/go-tree-sitter README] |
| File-walk concurrency | Hand-rolled goroutine pool with semaphore | Match `internal/repomap/extractor.go`'s walker pattern (worker count via config; default 4) | Existing pattern in repo; concurrency knob already exposed; no need to invent a second model. |
| Schema migration sequencing | Ad-hoc CREATE TABLE in daemon bootstrap | Phase 57 `Migration{From, To, Kind}` registry in `internal/semantic/store/migrations_types.go` | Registry already exists; Phase 59 lights up the second entry. The TODO at `migrations_types.go:37` is the wiring point. |
| SCIP-style symbol scheme | Helix-native textual symbol grammar | xxhash64 over `\x00`-joined fields per SPEC §11.1 | Helix opted for fixed-size hashed IDs over SCIP's textual scheme — fixed cost in DuckDB, faster equality. SCIP's grammar is a useful **mental model** (descriptors = our OwnerPath+QualifiedName+Kind chain; method-disambiguator = our SignatureHash) but Helix does NOT emit SCIP-format symbols. [CITED: github.com/sourcegraph/scip] |
| Stable-ID survival rules | Inferred from "feels right" | Five rules in SPEC §11.1 verbatim: (1) prefer LSP identity; (2) else package + owner + qname + kind + sighash; (3) file path only as fallback; (4) same-content rename preserves lineage + updates FilePathFallback; (5) moved exported symbols preserve identity if package + qname unchanged | The 5 rules are the contract that EXTRACT-02 asserts and EXTRACT-03's 30+ test matrix exercises. Per-language nuance (Go method-on-receiver, Python class methods, TS class instances) is implementation choice INSIDE these 5 rules, not deviation from them. |
| Confidence ladder | Float arithmetic on confidence values | Closed-enum bucket assignment per SPEC §11.2: `0.70` for ts-only, `0.80` if local-resolver-validated, `0.95` if merged with LSP, `1.00` if LSP-only, `0.45` if heuristic | Confidence is a 5-bucket enum with float labels, not a continuous score. Phase 59 emits only `0.70` (every fact is ts-only at extraction time). Other rungs land in Phase 61/62. |

**Key insight:** Three things look like "stuff Helix could write itself" but absolutely should not be hand-rolled: the hash function (SPEC-locked), the tree-sitter query API (CGO bindings exist), and the schema-migration registry (Phase 57 already shipped it). The interesting design work in Phase 59 lives in two places: (a) the per-language `queries.scm` files — net-new and the load-bearing artifact for EXTRACT-01, and (b) the canonicalization function input bytes per language — what `OwnerPath` looks like for Go method-on-receiver vs. Python instance method vs. TS class field. Those two artifacts are where the planner's review attention should concentrate.

## Stable-ID Design (deep-dive for EXTRACT-02 / EXTRACT-03)

### Per-language `OwnerPath` and `QualifiedName` recipes

The 5 canonicalization rules in SPEC §11.1 are language-agnostic. The **per-language interpretation** of those rules — specifically what goes into `OwnerPath` and `QualifiedName` — determines whether method-on-receiver renames produce the right transitions.

| Language | Symbol | PackagePath | OwnerPath | QualifiedName | Kind | SignatureHash input |
|----------|--------|-------------|-----------|---------------|------|---------------------|
| Go | top-level `func F(int) string` | `github.com/x/pkg` | `""` | `F` | `function` | `(int)string` (params + return; whitespace-stripped) |
| Go | method `func (r *R) M(int)` | `github.com/x/pkg` | `*R` | `M` | `method` | `(int)` |
| Go | method `func (r R) M(int)` | `github.com/x/pkg` | `R` | `M` | `method` | `(int)` |
| Go | generic `func F[T any](T) T` | `github.com/x/pkg` | `""` | `F` | `function` | `[any](T)T` |
| TS | top-level `function f(x: number)` | `src/foo.ts` (relative) | `""` | `f` | `function` | `(number)` |
| TS | method `class C { m() {} }` | `src/foo.ts` | `C` | `m` | `method` | `()` |
| TS | namespace `namespace N { function f() {} }` | `src/foo.ts` | `N` | `f` | `function` | `()` |
| TS | arrow assigned to const `const f = () => {}` | `src/foo.ts` | `""` | `f` | `function` (per normalizer) | `()` |
| TS | default export `export default function() {}` | `src/foo.ts` | `""` | `default` | `function` | signature |
| Python | top-level `def f(x):` | `pkg.mod` | `""` | `f` | `function` | `(x)` |
| Python | method `class C: def m(self):` | `pkg.mod` | `C` | `m` | `method` | `(self)` |
| Python | decorated `@dec\ndef f():` | `pkg.mod` | `""` | `f` | `function` | `(...)`; decorators are NOT in sighash |

**Why the recipes matter for EXTRACT-03:**

- **Go method-on-receiver rename** (`(r *R) Old()` → `(r *R) New()`): rule 5 says `QualifiedName` changed → ID **churns**. This is the canonical EXTRACT-03 scenario.
- **Go pointer-vs-value receiver change** (`(r R) M()` → `(r *R) M()`): `OwnerPath` changed → ID **churns**. (This is intentional — semantically different in Go.)
- **Python decorator add/remove** (`def f():` → `@dec\ndef f():`): `OwnerPath`, `QualifiedName`, `Kind`, `SignatureHash` all unchanged → ID **preserved**. (Decorator presence is a `decorator.name` capture stored separately, not part of identity.)
- **TS arrow vs. function declaration** (`function f() {}` → `const f = () => {}`): normalizer collapses both to `Kind=function` so ID **preserved**, even though tree-sitter sees different node kinds. This is the planner's biggest test-design concern.
- **TS export default rename** (`export default function() {}` → `export default function newName() {}`): `QualifiedName` is `"default"` either way → ID **preserved**. Adding a name to a default export does not break identity.
- **Same-content file rename** (move `pkg/foo.go` → `pkg/bar.go` with no edits): `PackagePath` for Go is the import path, not the file path → unchanged → ID **preserved**. `FilePathFallback` is updated. Rule 4.
- **Exported symbol move within package** (Go: move `func F` from `foo.go` to `bar.go`, same package): `PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash` unchanged → ID **preserved**. Rule 5.

### `LSPIdentity` field — when populated

`LSPIdentity` is empty in Phase 59 emit (LSP enrichment is Phase 61). Rule 1 ("prefer LSP identity if stable and available") applies during the merge in Phase 61 — Phase 59's IDs are pure rule-2 IDs. The field is in the canonicalization input nonetheless, so when Phase 61 raises a fact's confidence to 0.95 and writes back the LSP identity, the canonicalized key changes — meaning **Phase 61 will produce a different `xxhash64` than Phase 59 did for the same symbol**.

This is a **planner concern that CONTEXT.md does not address**. Two valid resolutions:
- **(a)** Phase 59's stable-ID lookup is by `(RepoID, Language, PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash)` — i.e., stable across enrichment because we don't include `LSPIdentity` until enrichment writes it back.
- **(b)** Phase 59 emits with `LSPIdentity=""`, Phase 61 emits with `LSPIdentity=<lsp-uri>`, the merge step matches via the §11.2 rules and **updates** the row to the LSP-derived ID, deleting the old.

**Recommendation: (a).** The Phase 61 merge step rewrites the row; emitting two different IDs for the same logical symbol creates row churn during enrichment. The cleanest interpretation of SPEC §11.1 is "LSPIdentity is canonicalized as empty string until LSP confirms; when LSP confirms, the same row's confidence is bumped, the row is NOT renamed by ID." **Surface this to the user before locking the plan.**

## Test Matrix Design (EXTRACT-03)

### Scenario count budget per first-class language

| Bucket | Count | Examples |
|--------|-------|----------|
| Shared (D-03 25-list) | 25 | function_basic, method_basic, class_or_struct_basic, interface_basic, enum_or_literal_type, top_level_variable, local_variable, parameters, return_type, field_access, call_reference, method_call_reference, import_default_or_basic, import_named, import_alias, type_annotation, nested_symbol, container_qualified_name, generic_function_or_type, visibility_exported_private, comments_and_doc_noise, multifile_package_or_module, invalid_partial_syntax, unicode_identifiers, builtin_or_keyword_edge |
| Go-specific | 8-9 | receiver_pointer, receiver_value, struct_embedded_field, interface_embedding, type_alias, generic_type_param, package_import_alias, method_on_generic_type |
| TS-specific | 8-9 | arrow_function, class_extends, interface_extends, implements, decorator, type_alias_union, generic_interface, export_default, namespace_or_module |
| Python-specific | 8-9 | decorator, class_inheritance, async_function, import_from, type_annotation, dataclass, property, nested_function, dunder_method |
| Stable-ID transitions | ≥10 | body_edit_preserves_id, comment_edit_preserves_id, move_within_file_preserves_id, rename_churns_id, signature_change_policy, container_change_churns_or_preserves_by_policy, new_symbol_newly_defined, deleted_symbol_removed, overload_like_disambiguation, same_name_different_container |

**Per-language total:** 25 + 8 + ≥10 = **≥43 scenarios per language**, of which 30 must hit the EXTRACT-03 success-criterion bar. **Test-file budget guidance:** assume 1 testdata directory per shared scenario + 1 per language-specific + 1 per stable-ID transition = ~43 dirs per language × 3 languages = **~130 testdata directories**, each with `before.<ext>` (always), `after.<ext>` (when scenario asserts before/after, ~half the cases), `expected.json` (always). That's ~520 testdata files at 50-200 LOC apiece. Treat this as one focused plan (P04 in the recommended split).

### Stable-ID transition coverage map

| Transition | Sample scenario | Expected | Why |
|------------|-----------------|----------|-----|
| Whitespace edit | `func F(){}` → `func F() {}` | Preserved | Rule 2: nothing in canonicalization input changed. |
| Comment edit | `func F() {}` → `// new\nfunc F() {}` | Preserved | Comments not part of canonicalization. |
| Move within file | swap order of two `func F` and `func G` | Preserved (both) | Range changed, identity-input unchanged. |
| Body-only edit | change `return 1` to `return 2` inside `func F() int` | Preserved | Body not part of `SignatureHash` (which is param + return only). |
| Rename | `func F()` → `func G()` | Churned (F removed, G newly_defined) | `QualifiedName` changed. |
| Signature change | `func F(int)` → `func F(int, string)` | Churned | `SignatureHash` changed. |
| Receiver type change | `(r R) M()` → `(r *R) M()` | Churned | `OwnerPath` changed. |
| Same-content file rename | `mv pkg/foo.go pkg/bar.go` no edits | Preserved | Rule 4 — `PackagePath` is import path, not file path. |
| Move exported across files in same package | move `func F` from `pkg/foo.go` to `pkg/bar.go` | Preserved | Rule 5. |
| Add overload (TS, Python) | add second `def f(x, y)` next to `def f(x)` | First preserved, second newly_defined | `SignatureHash` differs between the two. |
| Decorator add (Python) | `def f():` → `@dec\ndef f():` | Preserved | Decorators not in canonicalization input. |
| Decorator remove (Python) | inverse | Preserved | Same. |
| Method-on-receiver rename (Go) | `(r R) Old()` → `(r R) New()` | Churned | `QualifiedName` changed. |
| Generic type-param rename | `func F[T any]()` → `func F[U any]()` | Preserved | Type-param NAME not part of `SignatureHash`; only constraint matters. |
| Anonymous closure | `f := func() {}` body change | Preserved if same `f`; `OwnerPath` is enclosing function | Local-scope identity recipe. |

### Property-based test (recommended; not required by CONTEXT.md)

A property test asserts: **for every Phase 59 emit, re-extracting the same source file produces byte-identical IDs.** This is a determinism test; it's cheap (no source mutation) and catches accidental nondeterminism in canonicalization (e.g., map iteration leaking into field order). Recommend as a one-liner per language test file.

### Differential test against LSP (deferred to Phase 61)

CONTEXT.md correctly defers LSP merge to Phase 61. A differential test ("xxhash64 of Phase 59 fact equals xxhash64 of corresponding gopls/pyright/tsserver SymbolInformation key") is NOT a Phase 59 test — Phase 59's IDs intentionally do NOT match LSP IDs (they're rule-2 IDs; LSP IDs land in `LSPIdentity` only post-merge).

## Schema Migration v1→v2

### Current Phase 57 schema (verified against `internal/semantic/store/migrations.go`)

| Table | Existing partial-related columns | Missing columns per CONTEXT.md D-05 |
|-------|----------------------------------|-------------------------------------|
| `semantic_files` | `generated, ignored, ignore_reason, indexed_at` | `extraction_status, extraction_partial, partial_reason, extractor_name, extractor_version, error_message` — **all 6 missing** |
| `semantic_symbols` | `extraction_source, confidence` | `partial, partial_reason` — **both missing** |
| `semantic_references` | `validation_state, confidence, reason` | `partial, partial_reason` — **both missing** |
| `semantic_snapshots` | `partial, partial_reason` | none — already has these (file-level rollup) |

### Recommended migration shape

```go
// internal/semantic/store/migrations.go
//go:build cgo

func applyMigration002(ctx context.Context, db *sql.DB) error {
    stmts := []string{
        `ALTER TABLE semantic_files ADD COLUMN extraction_status   TEXT NOT NULL DEFAULT ''`,
        `ALTER TABLE semantic_files ADD COLUMN extraction_partial  BOOLEAN NOT NULL DEFAULT false`,
        `ALTER TABLE semantic_files ADD COLUMN partial_reason      TEXT`,
        `ALTER TABLE semantic_files ADD COLUMN extractor_name      TEXT`,
        `ALTER TABLE semantic_files ADD COLUMN extractor_version   TEXT`,
        `ALTER TABLE semantic_files ADD COLUMN error_message       TEXT`,
        `ALTER TABLE semantic_symbols ADD COLUMN partial           BOOLEAN NOT NULL DEFAULT false`,
        `ALTER TABLE semantic_symbols ADD COLUMN partial_reason    TEXT`,
        `ALTER TABLE semantic_references ADD COLUMN partial        BOOLEAN NOT NULL DEFAULT false`,
        `ALTER TABLE semantic_references ADD COLUMN partial_reason TEXT`,
        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (2, now())`,
    }
    for i, stmt := range stmts {
        if _, err := db.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("applyMigration002: stmt %d: %w", i+1, err)
        }
    }
    return nil
}
```

### DuckDB ALTER TABLE compatibility

DuckDB supports `ALTER TABLE ... ADD COLUMN ... DEFAULT <value>` — verified by the table layout in `internal/semantic/store/migrations.go` already using `DEFAULT false`. **No data backfill needed:** existing Phase 57 rows have all the legacy columns; new columns default to empty/false on existing rows. Per Phase 57 D-02 spec, this is `MigrationKind=InPlace`, runs at Open time, no reindex. [VERIFIED: internal/semantic/store/migrations_types.go:17-25]

### Test surface for migration

- Open a fresh DB → schema_version = 2, all columns present.
- Open a Phase-57-populated DB (schema_version = 1, real rows) → migration runs, schema_version = 2, existing rows present + new columns default-valued, no row count change.
- Open a future-version DB (schema_version > 2) → daemon refuses (forward-incompatible per STORE-03).
- Per-table column-presence query (probing `information_schema.columns` is cleanest in DuckDB).

## Common Pitfalls

### Pitfall 1: GrammarRegistry leak through tests

**What goes wrong:** A provider unit test convenience-helper does `treesitter.NewGrammarRegistry()` to avoid plumbing the singleton. The test passes; the EXTRACT-05 regression test (D-01b acceptance #11) catches it because the source-grep finds the call inside `internal/semantic/extract/`.

**Why it happens:** Tests "feel different" — the daemon is not running, why not just construct one? Because the regression test scans **all source files**, not just non-test files.

**How to avoid:** Tests construct providers via `goextract.NewProvider(grammars)` where `grammars` is a `treesitter.NewGrammarRegistry()` constructed in a **test helper outside `internal/semantic/extract/`** — e.g., `internal/semantic/extract/testutil/grammar.go`. The source-grep regex in the regression test should be tightly scoped to non-test files (`*.go` minus `*_test.go`) AND non-helper files (`internal/semantic/extract/testutil/` excluded).

**Warning signs:** Test file references `treesitter.NewGrammarRegistry`. New per-language test introduces its own helper.

### Pitfall 2: Tree-sitter query compilation cost on every extraction

**What goes wrong:** `tree_sitter.NewQuery(lang, src)` is called inside `ExtractFile` instead of once at provider construction. Per-file extraction time goes from ~10ms to ~50ms because query compilation is the dominant cost.

**Why it happens:** The natural shape is "give the function everything it needs each time." But tree-sitter `Query` objects are immutable and reusable; only `QueryCursor` is per-execution.

**How to avoid:** Compile queries in the provider constructor (cache as `*tree_sitter.Query` on the Provider struct); per-extraction allocate only `QueryCursor` and `defer cursor.Close()`. Repomap pattern at `internal/repomap/extractor.go:88-141` is the reference.

**Warning signs:** Per-file extraction budget exceeds 30ms for typical Go files; benchmark shows query-compile in profile.

### Pitfall 3: Non-deterministic golden output

**What goes wrong:** Golden test snapshot includes `tree-sitter` node IDs, absolute paths, or map iteration order. Test passes locally; fails on CI; fails after `-update` regenerates with different IDs.

**Why it happens:** Default Go map iteration is randomized. tree-sitter assigns node IDs from C memory addresses. `os.Getwd()` produces absolute paths.

**How to avoid:** Per CONTEXT.md D-03 — sort symbols by `(file, range, kind, name)`, sort references by `(file, range, kind, name)`, sort imports by `(file, range, source)`, sort type/heritage by `(subject, range)`. Use **relative paths only**. Exclude raw tree-sitter node IDs. Two-space JSON indent. No timestamps, no environment-specific fields. The normalizer is a single function in `internal/semantic/extract/testutil/golden.go`.

**Warning signs:** `cmp.Diff` shows reordering only. Test passes after re-running. CI fails with `-update`-derived golden.

### Pitfall 4: `RequireReady` deadlock from semantic-tool calls during extraction

**What goes wrong:** Phase 64 MCP tool wrapper calls `RequireReady` from within an extraction-completion callback. `RequireReady` waits for `ready|partial|failed`; the callback holds a goroutine the scheduler needs to mark `ready`. Deadlock.

**Why it happens:** The "centralized readiness API" temptation is to use it from everywhere. But it's specifically designed for **consumer-side** wait, not producer-side polling.

**How to avoid:** `RequireReady` is called from MCP tool handlers only, never from inside scheduler / extractor / provider code. Document in `RequireReady` doc-comment: "Callers MUST be on a goroutine that does not hold any scheduler-internal lock." `Subscribe()` is the producer-side primitive; `RequireReady` is the consumer-side primitive.

**Warning signs:** Stress test hangs. Extraction never reaches `ready` state. `goroutine` dump shows scheduler goroutine waiting on `RequireReady`'s channel.

### Pitfall 5: Confidence-ladder "round to nearest rung"

**What goes wrong:** Phase 59 emits `Confidence = 0.7` (float literal). Some downstream consumer compares `if c >= 0.75` and rejects all Phase 59 facts. Or worse: emits `Confidence = 0.7000001` from a calculation that should have produced exactly `0.7`.

**Why it happens:** Float arithmetic. The 5-rung ladder is a **closed enum with float labels**, not a continuous score.

**How to avoid:** Define `package extract` constants:

```go
const (
    ConfidenceLSPOnly       = 1.00
    ConfidenceLSPMerged     = 0.95
    ConfidenceTSPlusLocal   = 0.80
    ConfidenceTSOnly        = 0.70
    ConfidenceHeuristic     = 0.45
)
```

Phase 59 emits **exactly** `ConfidenceTSOnly`. Phase 61 merge raises to `ConfidenceLSPMerged` or `ConfidenceLSPOnly`. Float comparison uses `==` against named constants.

**Warning signs:** Confidence values like `0.7000000001`. Code does `c > 0.7 - epsilon`. Code computes confidence from anything other than constant assignment.

### Pitfall 6: Per-symbol partial when whole-file partial would do

**What goes wrong:** Parser fails halfway through a file. Some symbols parsed, some didn't. The implementation marks individual symbols partial but the file-level `extraction_status` stays "ready". A consumer query `WHERE extraction_status = 'ready'` returns the file but is missing half its symbols silently.

**Why it happens:** Granularity confusion: "the file extracted, just not all of it" reads like file=ready, symbol=partial. But the contract is opposite — file-level status reflects "did anything go wrong?"

**How to avoid:** D-05 says "first-class languages with errors: write `semantic_files` row, write all facts the parser/queries safely produced, **mark file (and boundary symbols/refs where applicable) as partial**." File-level `extraction_status="partial"` is the source-of-truth for consumer queries. Per-symbol `partial=true` is for fine-grained UI display only ("this symbol's signature was guessed because the parser recovered from an error here").

**Warning signs:** Consumer query against `extraction_status='ready'` returns rows that show a `partial=true` symbol. Test for "file-level vs. symbol-level partial classification" missing.

## Code Examples

### Provider's `ExtractFile` skeleton (mirrors SPEC §13.5)

```go
// internal/semantic/extract/extract_cgo.go
//go:build cgo

package extract

import (
    "context"
    "errors"

    tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type ExtractedFile struct {
    File        FileFact
    Symbols     []SymbolFact
    References  []ReferenceFact
    Imports     []ImportFact
    Types       []TypeFact
    Heritage    []HeritageFact
    SyntaxEdges []EdgeFact
    Partial     bool
    PartialReason string
}

func ExtractFile(ctx context.Context, p Provider, source []byte, file SourceFile) (*ExtractedFile, error) {
    parser := tree_sitter.NewParser()
    defer parser.Close()
    if err := parser.SetLanguage(p.TreeSitterLanguage()); err != nil {
        return nil, err
    }

    tree := parser.Parse(source, nil)
    if tree == nil {
        return PartialExtract(file, "parse_error", errors.New("tree-sitter parse returned nil")), nil
    }
    defer tree.Close()

    captures, qErr := RunQueries(tree, p.Queries(), source)
    if qErr != nil {
        return PartialExtract(file, "query_error", qErr), nil
    }

    scopes := p.ScopeBuilder().Build(tree, captures)
    syms   := ExtractSymbols(file, captures, scopes, source)
    refs   := ExtractReferences(file, captures, scopes, source)
    imps   := ExtractImports(file, captures, source)

    syms = p.SymbolNormalizer().Normalize(syms)
    refs = p.ReferenceClassifier().Classify(refs, scopes)

    // Stamp stable IDs.
    for i := range syms {
        syms[i].ID = StableSymbolID(BuildKeyForSymbol(p.Language(), file, syms[i]))
        syms[i].Confidence = ConfidenceTSOnly
        syms[i].ExtractionSource = "tree_sitter"
    }
    for i := range refs {
        refs[i].Confidence = ConfidenceTSOnly
        refs[i].ValidationState = "syntactic"
    }

    syntaxEdges := BuildSyntaxEdges(syms, refs, imps)

    return &ExtractedFile{
        File:        BuildFileFact(file, ExtractionStatusReady, false, ""),
        Symbols:     syms,
        References:  refs,
        Imports:     imps,
        SyntaxEdges: syntaxEdges,
    }, nil
}
```

### `RequireReady` consumer pattern

```go
// internal/semantic/scheduler/ready.go (or chosen path)

type ReadyPolicy struct {
    Timeout       time.Duration       // default cfg.SemanticIndex.Extraction.ReadyTimeout (30s)
    AllowPartial  bool                // default true
    MinState      SemanticIndexState  // default SemanticPartial
    TriggerIfCold bool                // default true
}

type ReadyResult struct {
    State      SemanticIndexState
    Partial    bool
    Ready      bool
    IndexedAt  time.Time
    FilesTotal int
    FilesDone  int
    Errors     []IndexError
}

func (s *Scheduler) RequireReady(ctx context.Context, ws WorkspaceID, policy ReadyPolicy) (ReadyResult, error) {
    st := s.Status(ws)

    switch st.State {
    case SemanticReady:
        return st.toResult(true), nil
    case SemanticPartial:
        if policy.AllowPartial {
            return st.toResult(true), nil
        }
        // fall through to wait
    case SemanticFailed:
        return st.toResult(false), errors.New("semantic index failed")
    case SemanticNotStarted:
        if policy.TriggerIfCold {
            s.ScheduleInitialExtraction(ws, InitialExtraction{Reason: "require_ready_cold", Mode: IncrementalIfPossible})
        }
        // fall through to wait
    case SemanticIndexing, SemanticStale:
        // fall through to wait
    }

    sub := s.Subscribe(ws)
    defer s.Unsubscribe(ws, sub)

    timer := time.NewTimer(policy.Timeout)
    defer timer.Stop()

    for {
        select {
        case <-ctx.Done():
            return s.Status(ws).toResult(false), ctx.Err()
        case <-timer.C:
            st := s.Status(ws)
            // return best partial if anything was indexed
            return st.toResult(st.FilesDone > 0), nil
        case st := <-sub:
            if st.State >= policy.MinState {
                return st.toResult(true), nil
            }
            if st.State == SemanticFailed {
                return st.toResult(false), errors.New("semantic index failed")
            }
        }
    }
}
```

(Iterator semantics, `Subscribe` channel buffering, fan-out to multiple subscribers — left to the implementation; CONTEXT.md does not constrain.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| LSIF (Language Server Index Format) | SCIP (Sourcegraph Code Intelligence Protocol) — proto3 with formal symbol grammar | 2022 (Sourcegraph) | Confirms symbol-as-string-grammar is the modern shape; Helix's hashed-ID approach is a deliberate divergence for storage-efficiency reasons. [CITED: sourcegraph.com/blog/announcing-scip] |
| Tree-sitter S-expressions only | Tree-sitter S-expressions + per-tool capture-name conventions (each tool defines its own `@reference.call` etc.) | ongoing | SPEC §13.4 picks generic capture names; Phase 59 extends with `decorator.name`, `receiver.type`, etc. — net-new but lexically compatible with all tree-sitter consumers. [VERIFIED: SPEC-DRAFT.md:1190-1211] |
| Per-language symbol scheme (TypeScript Compiler API ID, gopls package paths, jedi for Python) | Hybrid: tool-specific symbol IDs at the tool tier + cross-tool stable contract at the index tier | ongoing | Helix's stable-ID is the cross-tool contract; LSP-confirmed identity (when available, Phase 61) takes precedence per SPEC §11.1 rule 1. |

**Deprecated/outdated:**
- **smacker/go-tree-sitter** (older Go bindings): replaced by `tree-sitter/go-tree-sitter` (the official upstream). Helix is on the official one (`v0.25.0`). Don't accidentally pull in smacker's via `pkg.go.dev` examples. [VERIFIED: go.mod:27]
- **`tree_sitter.Query.PredicateUsage` C-side variants** (pre-0.22 API): the `MatchPredicate` plumbing was reshaped; if any 0.21-era examples are referenced, ignore them. v0.25 API is what repomap uses today.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | All build/test | ✓ | 1.22+ assumed | — |
| CGO | tree-sitter, DuckDB; semantic extraction is CGO=1 only | ✓ (dev), ✓ (CI) | clang on darwin, gcc on linux | CGO=0 stubs already exist (Phase 51.1, Phase 57 D-04); semantic features unavailable at runtime, daemon refuses to start with `Enabled=true` |
| `github.com/tree-sitter/go-tree-sitter` | All extraction | ✓ | v0.25.0 | — |
| `github.com/cespare/xxhash/v2` | Stable-ID hash | ✓ | v2.3.0 (indirect → direct) | — |
| `github.com/marcboeker/go-duckdb` (or `duckdb-go-bindings`) | semantic store v1→v2 migration | ✓ | (whatever Phase 57 selected) | — |
| `tree-sitter-go`, `tree-sitter-typescript`, `tree-sitter-python` grammars | per-language extraction | ✓ | already wired in `internal/treesitter/registry_cgo.go` | — |
| `go test` race detector | concurrency test for scheduler | ✓ | built-in | — |

**Missing dependencies with no fallback:** none — every dependency is already in tree.

**Missing dependencies with fallback:** none — CGO=0 fallback exists upstream and is already gated.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/google/go-cmp/cmp` for diffs |
| Config file | none — Go conventions; per-package `*_test.go` |
| Quick run command | `go test ./internal/semantic/...` |
| Full suite command | `go test -race ./...` |
| Per-package fast | `go test ./internal/semantic/extract/go/...` (single language at a time) |
| Goldens regenerate | `go test ./internal/semantic/extract/... -update` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| EXTRACT-01 | Go provider emits typed symbols/refs/imports/edges from a Go file | golden | `go test ./internal/semantic/extract/go/... -run TestProvider_Golden` | ❌ Wave 0 |
| EXTRACT-01 | TypeScript provider emits typed symbols/refs/imports/edges from a TS file | golden | `go test ./internal/semantic/extract/typescript/... -run TestProvider_Golden` | ❌ Wave 0 |
| EXTRACT-01 | Python provider emits typed symbols/refs/imports/edges from a Py file | golden | `go test ./internal/semantic/extract/python/... -run TestProvider_Golden` | ❌ Wave 0 |
| EXTRACT-01 | Non-first-class language file produces `semantic_files` row only with `extraction_status="unsupported"` | integration | `go test ./internal/semantic/scheduler/... -run TestScheduler_UnsupportedLanguage` | ❌ Wave 0 |
| EXTRACT-02 | `xxhash64(canonicalize(key))` is deterministic across runs / OSes | unit | `go test ./internal/semantic/extract/ -run TestStableSymbolID_Deterministic` | ❌ Wave 0 |
| EXTRACT-02 | Whitespace-only edit preserves ID | table-driven | `go test ./internal/semantic/extract/go/... -run TestStableID/whitespace_edit` | ❌ Wave 0 |
| EXTRACT-02 | Same-content file rename preserves ID | table-driven | `go test ./internal/semantic/extract/go/... -run TestStableID/same_content_rename` | ❌ Wave 0 |
| EXTRACT-02 | Exported-symbol move within package preserves ID | table-driven | `go test ./internal/semantic/extract/go/... -run TestStableID/move_exported_within_package` | ❌ Wave 0 |
| EXTRACT-03 | ≥30 before/after scenarios per language with explicit transitions | table-driven matrix | `go test ./internal/semantic/extract/{go,typescript,python}/... -run TestStableID` | ❌ Wave 0 |
| EXTRACT-03 | Overload disambiguation via SignatureHash (TS, Python) | table-driven | `go test ./internal/semantic/extract/typescript/... -run TestStableID/overload_disambiguation` | ❌ Wave 0 |
| EXTRACT-03 | Method-on-receiver rename churns ID (Go) | table-driven | `go test ./internal/semantic/extract/go/... -run TestStableID/receiver_method_rename` | ❌ Wave 0 |
| EXTRACT-03 | Decorator add/remove preserves ID (Python) | table-driven | `go test ./internal/semantic/extract/python/... -run TestStableID/decorator_add_remove` | ❌ Wave 0 |
| EXTRACT-04 | Phase 59 emits exactly `Confidence = 0.70` for all facts | unit | `go test ./internal/semantic/extract/ -run TestExtract_ConfidenceTSOnly` | ❌ Wave 0 |
| EXTRACT-04 | `MergeSymbols(ts, lsp)` per SPEC §11.2 picks 1.00/0.95/0.80/0.70/0.45 buckets | unit (Phase 61 owns merge; Phase 59 ships the input) | `go test ./internal/semantic/extract/ -run TestConfidence_LadderConstants` | ❌ Wave 0 |
| EXTRACT-05 | Daemon-singleton `*GrammarRegistry` flows into every extractor | integration | `go test ./internal/daemon/ -run TestBootstrap_GrammarRegistrySingleton` | ❌ Wave 0 |
| EXTRACT-05 | Source-grep finds zero `treesitter.NewGrammarRegistry` calls in `internal/semantic/extract/` non-test, non-testutil files | static | `go test ./internal/semantic/extract/ -run TestNoSecondaryGrammarRegistry` | ❌ Wave 0 |
| D-04 invariant | `RequireReady` is the ONLY semantic-readiness API; no `time.Sleep` polling in semantic consumers | static | grep-based test in `internal/semantic/...` | ❌ Wave 0 |
| D-02 invariant | Registry panics on duplicate `Language()` | unit | `go test ./internal/semantic/extract/ -run TestRegistry_DuplicatePanic` | ❌ Wave 0 |
| D-04 invariant | Scheduler is idempotent (concurrent `ScheduleInitialExtraction` returns same JobID) | unit | `go test ./internal/semantic/scheduler/ -run TestScheduler_Idempotent` | ❌ Wave 0 |
| Schema migration | v1→v2 on fresh DB → schema_version=2, all columns present | integration | `go test ./internal/semantic/store/ -run TestMigration_Fresh_v2` | ❌ Wave 0 |
| Schema migration | v1→v2 on Phase-57-populated DB → existing rows preserved | integration | `go test ./internal/semantic/store/ -run TestMigration_Existing_v2` | ❌ Wave 0 |
| Per-feature defaults | `semantic_index.extraction.*` keys flow through all 4 layers | integration | `go test ./internal/config/ -run TestLoad_SemanticExtractionDefaults` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go vet ./internal/semantic/... && go test ./internal/semantic/...` (~5-10s on M-class hardware)
- **Per wave merge:** `go test -race ./...` plus `make build` to confirm both CGO=1 and (via build tags) CGO=0 stub paths compile
- **Phase gate (before `/gsd-verify-work`):** Full suite green + golden-test snapshots stable across two consecutive runs (catches non-determinism)

### Wave 0 Gaps

- [ ] `internal/semantic/extract/testutil/grammar.go` — test helper for `*treesitter.GrammarRegistry` construction outside the EXTRACT-05 grep scope
- [ ] `internal/semantic/extract/testutil/golden.go` — JSON normalization + `cmp.Diff` driver shared across the three per-language `provider_test.go` files
- [ ] `internal/semantic/extract/testutil/stable_id_helper.go` — `assertStableIDExpectations(t, beforeFacts, afterFacts, []StableIDCheck)` driver per D-03
- [ ] EXTRACT-05 regression test scaffolding in `internal/semantic/extract/grammar_singleton_test.go`
- [ ] No framework install needed (Go stdlib + already-present `cmp`)

## Project Constraints (from CLAUDE.md)

- **`go vet ./...` and `go test ./...` MUST run before completing any Go task.** Phase 59's per-task command list MUST include both.
- **Single binary, native concurrency.** Scheduler goroutines respect existing patterns (errgroup orchestration in `internal/daemon/`); no shelling out, no sidecars.
- **MCP is the primary interface.** Phase 59 ships ZERO new MCP tools (all consumer-facing surface deferred to Phase 64). The `Status()`/`Subscribe()` API is internal Go-package API.
- **LSP only for code intelligence.** Phase 59 ships ZERO LSP integration (deferred to Phase 61). Phase 59 tests do NOT spin up `gopls`/`pyright`/`tsserver`.
- **GSD workflow enforcement.** All file edits flow through GSD plans (P01..P05 in the recommended split).
- **SMTC-first tool routing.** Where a Phase 59 task requires reading existing Helix code structure, prefer SMTC tools (`mcp__smtc__list_file_outline`, `mcp__smtc__find_references`, etc.) over `Grep`. Phase 59's own *output* is what tools like SMTC will eventually consume; it does not consume them at runtime.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DuckDB supports `ALTER TABLE ... ADD COLUMN` with `DEFAULT` for `BOOLEAN NOT NULL`. | Schema Migration v1→v2 | If DuckDB requires a different syntax (e.g., separate ADD COLUMN + UPDATE), migration plan splits one statement into two. Low risk — DuckDB documentation supports `ADD COLUMN ... DEFAULT`. [ASSUMED based on standard SQL behavior; verify with DuckDB docs in P01 task.] |
| A2 | The `Apply` field on `Migration` (currently TODO at `migrations_types.go:37`) is a `func(ctx, *sql.DB) error`. | Schema Migration v1→v2 | If Phase 57 P57-02 Task 2b's wiring chose a different signature (e.g., taking a `*Store` receiver, or returning sentinel for reindex), Phase 59's migration entry plumbing changes. Medium risk — the TODO is explicit and the signature is conventional, but Phase 57's actual landed shape should be re-verified at P01 start. [ASSUMED] |
| A3 | The four genuinely-new config keys (`extraction_ready_timeout`, `extraction_file_timeout`, `max_parallel_files`, `allow_partial_results`) belong under a new `Extraction` sub-struct rather than extending `IndexingConfig`. | Summary finding #2; Recommended Project Structure | If the user/planner picks the alternative (extend `IndexingConfig`), the per-feature defaults test name and koanf binding paths change. Surface to user before locking. [ASSUMED — Claude's recommendation, not a CONTEXT.md decision] |
| A4 | `LSPIdentity` is canonicalized as empty string by Phase 59 and remains empty in the canonicalization input even after Phase 61 enrichment (i.e., enrichment updates the row in place, does not produce a new ID). | Stable-ID Design — `LSPIdentity` field — when populated | If the user picks alternative (b) (Phase 61 produces new ID, deletes old), Phase 59's `BuildKeyForSymbol` recipe simplifies but Phase 61's merge gets harder. Surface to user before locking. [ASSUMED — Claude's recommendation, not a CONTEXT.md decision] |
| A5 | The 7-rung ladder mention in v1.10-ROADMAP.md Phase 59 success criterion #3 is a transcription error from SPEC §38.2 (type resolution). EXTRACT-04 in REQUIREMENTS.md correctly references the 5-rung §11.2 ladder. | Summary finding #1; Phase Requirements EXTRACT-04 row | If the user actually intended a 7-rung ladder for symbol confidence, Phase 59 needs to redefine §11.2 (which would touch Phases 61, 62, 64, 65 too). Very high impact — surface to user FIRST. [ASSUMED based on cross-document evidence] |
| A6 | `extraction.max_file_size=2 MiB` (CONTEXT.md Claude's discretion) supersedes `indexing.max_file_size="2MiB"` (SPEC §25 / `IndexingConfig`); they are the same knob with different config-key names. | Summary finding #2 | If they're meant to be independent (e.g., `indexing` controls index-time max, `extraction` controls extraction-time max), the planner needs both. Low risk — the values are identical and the discussion log treats them as one knob. [ASSUMED] |
| A7 | `xxhash.Sum64String` is stable across Go versions and architectures. | Pattern 4 (Stable-ID Canonicalization) | If `xxhash.Sum64String` differs by platform, IDs are non-portable across machines and `EXTRACT-02` determinism fails on cross-OS test runs. Very low risk — xxhash is endianness-explicit by spec; the upstream Go package has shipped stable since v2.0. [ASSUMED, but standard library practice] |
| A8 | Repomap PageRank exposes a per-file rank value through a public method on `internal/repomap/`. | Architecture Diagram (initial-walk priority order); Architectural Responsibility Map | If PageRank scores are not externally accessible from `internal/semantic/scheduler/`, the priority-order implementation needs a wrapper or a new accessor on repomap. Low-medium risk — repomap is fairly enclosed; verify in P03. [ASSUMED] |

## Open Questions

1. **7-rung vs. 5-rung confidence ladder (CRITICAL).**
   - What we know: SPEC §11.2 is 5 rungs; SPEC §38.2 is 7 rungs; ROADMAP Phase 59 SC#3 says "7-rung"; REQUIREMENTS EXTRACT-04 says 5 rungs.
   - What's unclear: whether ROADMAP's "7-rung" is a typo (referencing the type-resolution ladder by mistake) or whether the user intends to extend §11.2.
   - Recommendation: surface to user at plan-discuss-phase or as the very first item in 59-VERIFICATION.md. Do not silently pick one.

2. **`semantic_index.extraction.*` config sub-tree placement.**
   - What we know: SPEC §25 has no such sub-tree; CONTEXT.md introduces 6 keys; 2 of those keys (`max_file_size`, `initial_extraction_on_activation`) overlap with existing `semantic_index.indexing.*` keys.
   - What's unclear: whether to extend `IndexingConfig` or add a new `ExtractionConfig` sub-struct.
   - Recommendation: extend `IndexingConfig` for the overlapping pair; add `ExtractionConfig` for the four genuinely-new keys; update SPEC §25 in the same plan.

3. **`LSPIdentity` field behavior across Phase 59 → Phase 61.**
   - What we know: Phase 59 emits `LSPIdentity=""`; Phase 61 produces LSP identity.
   - What's unclear: whether LSP identity becomes part of canonicalization (changing the ID) or only a side-channel field on the row (preserving the ID).
   - Recommendation: side-channel; surface to user; document in PATTERNS.md when written.

4. **Per-symbol vs. per-file `partial=true` distinction.**
   - What we know: D-05 says "mark file (and boundary symbols/refs where applicable) as partial."
   - What's unclear: which symbols/refs are "boundary" — the ones tree-sitter recovered from? The ones inside the parser's error region? All symbols in a file with any error?
   - Recommendation: implementation choice; document in PATTERNS.md the chosen heuristic. Default suggestion: symbols that lie within or adjacent to a tree-sitter ERROR node get `partial=true`; everything else does not.

5. **EXTRACT-05 source-grep test scope.**
   - What we know: "regression test attempts to construct a second `GrammarRegistry`."
   - What's unclear: whether "attempts to construct" means (a) runtime check (try to call NewGrammarRegistry from extractor code, expect refusal — but there's no refusal mechanism, the constructor just works) or (b) static check (grep source).
   - Recommendation: static source-grep test scoped to `internal/semantic/extract/` non-test files + non-`testutil/`; runtime test asserting `provider.grammar == daemon.grammarRegistry.GetLanguage("go").value` for at least one provider after bootstrap.

## Sources

### Primary (HIGH confidence)

- **SPEC-DRAFT.md** — sections §6 (package layout), §7 (typed identifiers), §11.1 (StableSymbolKey + canonicalization rules + xxhash), §11.2 (5-rung confidence ladder + merge pseudocode), §12.1 (edge vocabulary), §12.3 (edge confidence — 5-rung ladder), §13.1-§13.5 (extraction layer: scope, file discovery, LanguageProvider interface, generic capture names, extraction pseudocode), §25 (`semantic_index.*` config keys), §38.2 (type-resolution 7-rung ladder — NOT applicable to Phase 59).
- **CONTEXT.md** (`.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-CONTEXT.md`) — D-01 through D-05, capture taxonomy, fact shapes, registry shape, scheduler shape, partial enum, schema delta, acceptance criteria, Claude's discretion.
- **DISCUSSION-LOG.md** (sibling) — alternatives considered for each decision; helpful for understanding the "why not?" of rejected options.
- **REQUIREMENTS.md** §EXTRACT (lines 31-36) — EXTRACT-01..EXTRACT-05 verbatim, including correct 5-rung ladder reference.
- **`internal/semantic/store/migrations.go`** + **`migrations_types.go`** — Phase 57's exact column layout for v1; the `Migration{From, To, Kind}` registry shape; the `applyMigration001` pattern Phase 59 mirrors as `applyMigration002`.
- **`internal/semantic/types.go`** + **`config.go`** — typed identifiers (Phase 57 already declares `SymbolID, ReferenceID, EdgeID`); typed config mirror (existing `IndexingConfig` overlap).
- **`internal/treesitter/registry_cgo.go`** — confirms 23 grammars wired; confirms the singleton constructor pattern Phase 59's bootstrap consumes.
- **`internal/daemon/daemon.go`** lines 213-235 (step 6/6b) and line 315 (12a) — exact bootstrap insertion site.
- **`internal/repomap/extractor.go`** lines 84-180 — reference pattern for tree-sitter pipeline (parser/tree/cursor lifecycle, `defer Close`, query compilation).
- **`internal/repomap/queries/go_tags.scm`** — confirms repomap queries are 5-capture navigation queries, validating D-01's "would lock semantic into a navigation schema" rationale.
- **`go.mod`** lines 27, 65 — confirms `tree-sitter/go-tree-sitter v0.25.0` and `cespare/xxhash/v2 v2.3.0` available.

### Secondary (MEDIUM confidence)

- **Phase 57 RESEARCH.md** (`.planning/phases/57-semantic-store-foundation-pipeline-dag-library/57-RESEARCH.md`) — Phase 57 conventions Phase 59 inherits (CGO=0 stub gate, per-feature defaults test pattern, vet-noduckdb scope).
- **Phase 57 PATTERNS.md** — analog mappings for `internal/semantic/store/` files; useful when planning P01 to mirror migration test shape.

### Tertiary (LOW confidence — verified against primary sources before stating)

- **Sourcegraph SCIP spec** (`github.com/sourcegraph/scip/blob/main/docs/scip.md`) [WebFetch, MEDIUM after cross-check] — confirms the SCIP descriptor model + method-disambiguator pattern; used here as design context, not as a binding contract. Helix deliberately diverges (hashed IDs vs. textual scheme).
- **go-tree-sitter README** (`github.com/tree-sitter/go-tree-sitter`) [WebSearch, MEDIUM] — confirms `Query`/`QueryCursor` lifetime requirements (`Close()` mandatory), confirms `Query` has no internal cache (planner concern: precompile queries in provider constructor).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dependency is already in `go.mod` and either used by repomap (tree-sitter, xxhash) or already wired into the daemon (treesitter.GrammarRegistry, semantic.store).
- Architecture: HIGH for the parts CONTEXT.md locks (registry shape, scheduler shape, partial model, fact structs); MEDIUM for the two planner-decision items flagged in Open Questions (config sub-tree placement, LSPIdentity behavior).
- Pitfalls: HIGH — six pitfalls drawn from concrete Phase 57 + repomap patterns + tree-sitter upstream guidance, not generic.
- Stable-ID design: HIGH on canonicalization rules (SPEC-locked); MEDIUM on per-language `OwnerPath` recipes (these are implementation choices the planner should review).
- Test matrix: HIGH on scenario taxonomy (CONTEXT.md locks the 25 + 8-9 + ≥10 split); MEDIUM on per-scenario file structure (judgement call on testdata layout).
- Schema migration: HIGH — column layout verified directly against `migrations.go`; DuckDB ALTER TABLE syntax is standard.

**Research date:** 2026-05-04
**Valid until:** 2026-06-04 (30 days; the SPEC and Phase 57 surface are stable; revisit if any of EXTRACT-01..EXTRACT-05 phrasing changes or if Phase 57 P57-02 Task 2b lands a different `Migration.Apply` shape than assumed).

## RESEARCH COMPLETE

**Phase:** 59 - Tree-sitter Extraction & Stable Symbol IDs
**Confidence:** HIGH (with two flagged cross-document inconsistencies the planner must surface to the user before locking plans)

### Key Findings

1. **Two cross-artifact inconsistencies must be surfaced before planning locks.** (a) ROADMAP says "7-rung confidence ladder" but EXTRACT-04 in REQUIREMENTS and SPEC §11.2 are both 5-rung — the 7-rung ladder belongs to SPEC §38.2 (type resolution, Phase 62). Recommend planning to 5-rung. (b) CONTEXT.md introduces `semantic_index.extraction.*` sub-tree, but two of those six keys overlap with existing `semantic_index.indexing.*` keys in SPEC §25. Recommend extending `IndexingConfig` for the overlapping pair, adding new `ExtractionConfig` for the four genuinely-new keys, updating SPEC §25 in the same plan.
2. **Phase 59 is the first phase to light up the Phase 57 D-02 migration registry.** The `Apply` field on `Migration` is currently a TODO at `migrations_types.go:37`. P01 wires it for the first time AND adds 10 ALTER TABLE statements (6 on `semantic_files`, 2 each on `semantic_symbols` and `semantic_references`). DuckDB supports `ALTER TABLE ... ADD COLUMN ... DEFAULT` natively.
3. **GrammarRegistry singleton enforcement (EXTRACT-05) needs both static and runtime tests.** Static: source-grep `internal/semantic/extract/` non-test, non-testutil files for `treesitter.NewGrammarRegistry` calls (zero allowed). Runtime: assert injected pointer equality during daemon bootstrap test.
4. **Recommended 5-plan / 3-wave split.** Wave 0: P01 schema migration (independent prep, modifies Phase 57 store, reviewable in isolation). Wave 1 (parallel): P02 `extract/` package skeleton + stable-ID + config keys + metric; P03 `scheduler/` + RequireReady + state enum. Wave 2 (parallel): P04 three per-language providers + ≥30-scenario tests per language; P05 daemon bootstrap wiring + EXTRACT-05 regression test + per-feature defaults test. P04 carries the bulk of the test surface (~130 testdata directories, ~520 test files); concentrating it in one plan focuses reviewer attention on the contract that all of v1.10 depends on.
5. **SCIP prior art validates the design without changing it.** Phase 59's `StableSymbolKey` is structurally a SCIP scheme (descriptor chain + method-disambiguator) but emits 64-bit hashed IDs instead of textual symbols — deliberate divergence for fixed-size DuckDB storage. SCIP also explicitly does not address rename/move identity survival; SPEC §11.1's five canonicalization rules are net-new Helix contract surface.

### File Created

`/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | All dependencies verified in `go.mod`; no version bumps required. |
| Architecture | HIGH (locked parts) / MEDIUM (config sub-tree placement, `LSPIdentity` behavior) | CONTEXT.md prescribes most architecture; two judgement calls flagged in Open Questions. |
| Pitfalls | HIGH | Six concrete pitfalls drawn from existing Phase 57 + repomap patterns + tree-sitter upstream guidance. |
| Stable-ID design | HIGH (canonicalization) / MEDIUM (per-language recipes) | SPEC §11.1 locks canonicalization; per-language `OwnerPath` choices are implementation latitude inside the rules. |
| Schema migration | HIGH | Existing schema verified directly against `migrations.go`; DuckDB syntax standard. |
| Test matrix | HIGH (taxonomy) / MEDIUM (file layout) | CONTEXT.md locks the 25+8-9+≥10 split; testdata directory layout is judgement. |

### Open Questions Surfacing to Planner / User

1. **7-rung vs. 5-rung ladder** (CRITICAL — surface first; affects EXTRACT-04 SC#3 wording).
2. **`semantic_index.extraction.*` placement** (extend `IndexingConfig` vs. new sub-struct).
3. **`LSPIdentity` field behavior across Phase 59 → Phase 61** (in canonicalization input or side-channel).
4. **Per-symbol vs. per-file `partial=true` boundary heuristic** (implementation choice; document in PATTERNS.md).
5. **EXTRACT-05 regression test scope** (static-grep only vs. static + runtime).

### Ready for Planning

Research complete. Five open questions identified — recommend the planner surface them in the planning conversation (or 59-DISCUSSION-LOG.md addendum) before locking PLAN.md files. Five-plan / three-wave split recommended; the bulk of test surface concentrates in P04 (per-language providers + ≥30 scenarios each).
