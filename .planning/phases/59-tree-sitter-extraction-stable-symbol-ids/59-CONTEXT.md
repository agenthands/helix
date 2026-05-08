# Phase 59: Tree-sitter Extraction & Stable Symbol IDs - Context

**Gathered:** 2026-05-04
**Updated:** 2026-05-08 — D-06 / D-07 / D-08 / D-11 added to unblock Phase 65
**Status:** Phase 59 base shipped & VERIFIED 2026-05-04 (4/4 must-haves). 2026-05-08 update layers a small interface-promotion + adapter + bookkeeping delta required by Phase 65 Wave 0 (production buildFn). Re-plan only the new tasks; do NOT re-discuss the original D-01..D-05 decisions.

<domain>
## Phase Boundary

Phase 59 produces typed semantic facts (symbols, references, imports, type
annotations, heritage edges, syntax edges) by parsing source files with
tree-sitter, anchored by a **stable symbol ID** that survives whitespace edits,
file renames where content hash is unchanged, and exported-symbol moves where
the qualified name is unchanged. Three languages are first-class:

- Go
- TypeScript / JavaScript (shared provider, two extensions)
- Python

Everything else gets a `semantic_files` row with `extraction_status=unsupported`
— no semantic facts are written for non-first-class languages in this phase.

**Phase 59 ships:**

1. **`internal/semantic/extract/`** — provider interface, fact types, normalizer,
   classifier, stable-ID hash function, query cache, registry constructor.
2. **`internal/semantic/extract/{go,typescript,python}/`** — net-new
   tree-sitter S-expression queries (`queries.scm`) covering the SPEC §13.4
   capture set extended with Phase 59-specific captures
   (`definition.parameter`, `type.return`, `type.parameter`, `import.symbol`,
   `decorator.name`, `receiver.type`, `receiver.name`), plus per-language
   `provider.go` constructors.
3. **`internal/semantic/extract/registry.go`** — constructor-injected provider
   registry; daemon bootstrap wires `goextract.NewProvider`,
   `tsextract.NewProvider`, `pyextract.NewProvider` explicitly. No init()
   registration, no blank imports.
4. **Stable-ID algorithm** — implements SPEC §11.1
   `xxhash64(CanonicalizeStableSymbolKey(key))` with explicit
   canonicalization rules locked in `extract/stable_id.go`.
5. **Initial-walk extraction scheduler** —
   `internal/semantic/scheduler/` (or sibling) ships an `ExtractionScheduler`
   interface, an initial-extraction implementation that walks the workspace
   tree post-activation, and the centralized
   `semantic.RequireReady(ctx, workspaceID, ReadyPolicy)` consumer helper.
6. **State enum** — `SemanticIndexState` with values
   `not_started|indexing|ready|partial|failed|stale`.
7. **Schema migration v1→v2** via Phase 57 D-02 registry — extends
   `semantic_files`, `semantic_symbols`, `semantic_references` with the
   partial-extraction columns prescribed in D-05.
8. **EXTRACT-03 test matrix** — golden snapshots per language (~30 scenarios)
   plus table-driven stable-ID transition tests (`Preserved | Churned |
   NewlyDefined | Removed`).
9. **New config keys** under `semantic_index.extraction.*`:
   `extraction_ready_timeout`, `extraction_file_timeout`,
   `max_parallel_files`, `max_file_size`, `allow_partial_results`,
   `initial_extraction_on_activation`.

**Out of scope (deferred to later v1.10 phases):**

- Live overlay watcher / coalescer / `ScheduleIncremental` — Phase 60.
- LSP enrichment merge into Phase 59 facts (the merge contract from SPEC §11.2
  is encoded in fact types now; the worker that produces LSP facts is
  Phase 61).
- Graph engine, ranking, type resolution — Phase 62.
- `semantic_graph_status` MCP tool wrapper — Phase 64 (Phase 59 ships the
  `Status()`/`Subscribe()` API; Phase 64 wraps it).
- Repomap fallback as a best-effort enrichment provider —
  not in v1.10 scope.

</domain>

<decisions>
## Implementation Decisions

### Extraction strategy vs. internal/repomap

- **D-01: Net-new queries, hard split.** `internal/semantic/extract/` is a
  distinct subsystem from `internal/repomap/`. Per-language queries
  (`extract/go/queries.scm`, `extract/typescript/queries.scm`,
  `extract/python/queries.scm`) are written from scratch against the SPEC §13.4
  capture vocabulary plus the Phase 59 capture additions listed in the Phase
  Boundary above. Repomap stays untouched.

  **Hard invariants** (must be enforced — see "Acceptance criteria" in code):
  - No `import "github.com/agenthands/helix/internal/repomap"` inside
    `internal/semantic/extract/`.
  - No copy of `internal/repomap/queries/*_tags.scm` into
    `internal/semantic/extract/`.
  - Repomap's `TagExtractor` / `Tag` types are not consumed, wrapped, or
    adapted by the semantic extractor in this phase.

  **Rationale:** repomap answers "where are useful navigation tags?";
  semantic extraction answers "what program facts can we trust for analysis,
  graph building, and LLM grounding?" Borrowing repomap query S-expressions
  early would lock the semantic extractor into a navigation-oriented schema.
  Future consolidation (if ever) is its own phase.

### Capture taxonomy (extends SPEC §13.4)

- **D-01a: Full capture set per first-class language.** The provider
  `queries.scm` for Go / TS+JS / Python emit (where syntactically present):

  ```
  definition.function          definition.method
  definition.struct            definition.class
  definition.interface         definition.enum
  definition.type              definition.variable
  definition.parameter         definition.field
  definition.constant
  reference.call               reference.identifier
  reference.field              reference.type
  import.source                import.alias
  import.symbol
  type.annotation              type.return
  type.parameter
  heritage.extends             heritage.implements
  decorator.name
  receiver.type                receiver.name
  ```

  Captures absent from a given language (e.g., `decorator.name` is Python-only)
  are simply unused; the union is the contract.

### Fact type shapes

- **D-01b: Net-new fact structs in `internal/semantic/extract/fact.go`.**
  These are the in-memory shapes the providers emit; the persistence mapping
  to DuckDB columns lives in `internal/semantic/store/` and is set up by the
  schema-migration plan in this phase.

  ```go
  type SymbolFact struct {
      ID             SymbolID        // xxhash64(CanonicalizeStableSymbolKey)
      StableKey      StableSymbolKey
      Language       string
      Kind           SymbolKind      // function|method|struct|class|...
      Name           string
      QualifiedName  string
      File           string
      Range          Range
      SelectionRange Range
      ContainerID    *SymbolID       // owner symbol (e.g., enclosing class)
      Signature      string
      SignatureHash  string
      Receiver       *ReceiverFact   // Go methods, Python instance methods
      Visibility     string          // exported|private|package|...
      Doc            string          // leading-comment block, if extracted
      Confidence     float32         // 0.70 ts-only baseline; raised on merge
      ExtractionSource string        // "tree_sitter" in this phase
      Partial        bool
      PartialReason  string
  }

  type ReferenceFact struct {
      ID            ReferenceID
      Language      string
      Kind          ReferenceKind   // CALL|REFERENCES|USES_TYPE|READS|WRITES|...
      Name          string
      File          string
      Range         Range
      ContainerID   *SymbolID
      ReceiverText  string          // pre-resolution literal, if any
      ResolvedTarget *SymbolID      // unset in Phase 59; Phase 61 fills via LSP
      ResolutionSource string       // "" until enrichment lands
      ValidationState string        // "syntactic" baseline in Phase 59
      Confidence    float32
      Reason        string
      Partial       bool
      PartialReason string
  }

  type ImportFact struct {
      ID       ImportID  // monotone within file or store
      Language string
      Source   string    // module path / package name
      Alias    string
      Symbols  []string  // for explicit named imports
      File     string
      Range    Range
  }

  type TypeFact struct {
      ID         TypeFactID
      Language   string
      SubjectID  SymbolID  // symbol the annotation is attached to
      Annotation string    // raw annotation text
      Range      Range
  }

  type HeritageFact struct {
      ID        HeritageID
      Language  string
      SubjectID SymbolID
      Relation  string    // "extends"|"implements"|"embeds"
      Target    string    // target qualified name (resolution is later)
      Range     Range
  }
  ```

  `SymbolID`, `ReferenceID`, etc. are already declared in
  `internal/semantic/types.go` (Phase 57). Phase 59 adds `ImportID`,
  `TypeFactID`, `HeritageID` as needed.

### Provider registration

- **D-02: Constructor-injected provider registry — no init() / no blank imports.**

  ```go
  // internal/semantic/extract/registry.go
  func NewExtractorRegistry(
      grammars *treesitter.GrammarRegistry,
      providers ...Provider,
  ) *Registry
  ```

  `Registry` panics on duplicate `Language()`. Tests can construct a registry
  with one provider. Daemon bootstrap (post step 6b — semantic store open):

  ```go
  semanticExtractors := extract.NewExtractorRegistry(
      grammarRegistry,
      goextract.NewProvider(grammarRegistry),
      tsextract.NewProvider(grammarRegistry),
      pyextract.NewProvider(grammarRegistry),
  )
  ```

  **Hard invariants:**
  - No `init()` registration in any `internal/semantic/extract/...` package.
  - No blank-import wiring file.
  - The shared `*treesitter.GrammarRegistry` (Phase 49 BUG-04 singleton) is
    injected before any provider is usable. EXTRACT-05 is satisfied
    structurally — duplicate registries are unconstructible because the
    daemon owns the singleton and passes it in.

### Test matrix layout

- **D-03: Hybrid — golden snapshots + table-driven stable-ID tests.** Each
  language provider owns:

  ```
  internal/semantic/extract/<lang>/
    provider.go
    queries.scm
    provider_test.go            # golden test driver
    stable_id_test.go           # table-driven stable-ID tests
    testdata/<scenario>/
      before.<ext>
      after.<ext>                (only when scenario asserts before/after)
      expected.json
  ```

  **Golden test contract** (`provider_test.go`):
  - Driver reads `before.<ext>`, runs provider extraction, normalizes output
    (deterministic ordering, relative paths only, no timestamps, no raw
    tree-sitter node IDs), compares against `expected.json` with `cmp.Diff`.
  - `go test ./internal/semantic/extract/... -update` rewrites
    `expected.json`. Update flag is gated to dev workflow; CI runs without it.
  - Stable IDs may appear in golden output once the algorithm is stable
    enough; until then the field is omitted via a normalize-time skip flag.

  **Stable-ID test contract** (`stable_id_test.go`):
  - Table-driven with inline before/after source strings (`//go:embed` for
    fixtures > ~50 lines).
  - Expectation enum:
    ```go
    type StableIDExpectation string
    const (
        Preserved    StableIDExpectation = "preserved"
        Churned      StableIDExpectation = "churned"
        NewlyDefined StableIDExpectation = "newly_defined"
        Removed      StableIDExpectation = "removed"
    )
    ```
  - Helper `assertStableIDExpectations(t, beforeFacts, afterFacts, []StableIDCheck)`
    reports symbol name, kind, before ID, after ID, expected vs. actual.

  **Scenario count per first-class language:** ≥30, split across:
  - 25 shared scenarios (function_basic, method_basic, class_or_struct_basic,
    interface_basic, enum_or_literal_type, top_level_variable, local_variable,
    parameters, return_type, field_access, call_reference,
    method_call_reference, import_default_or_basic, import_named, import_alias,
    type_annotation, nested_symbol, container_qualified_name,
    generic_function_or_type, visibility_exported_private,
    comments_and_doc_noise, multifile_package_or_module,
    invalid_partial_syntax, unicode_identifiers, builtin_or_keyword_edge).
  - 8–9 language-specific (Go: receiver_pointer, receiver_value,
    struct_embedded_field, interface_embedding, type_alias, generic_type_param,
    package_import_alias, method_on_generic_type. TS: arrow_function,
    class_extends, interface_extends, implements, decorator,
    type_alias_union, generic_interface, export_default,
    namespace_or_module. Python: decorator, class_inheritance, async_function,
    import_from, type_annotation, dataclass, property, nested_function,
    dunder_method).
  - ≥10 stable-ID scenarios per language: body_edit_preserves_id,
    comment_edit_preserves_id, move_within_file_preserves_id,
    rename_churns_id, signature_change_policy,
    container_change_churns_or_preserves_by_policy, new_symbol_newly_defined,
    deleted_symbol_removed, overload_like_disambiguation,
    same_name_different_container.

  **Golden JSON normalization:**
  - Sort symbols by `(file, range, kind, name)`.
  - Sort references by `(file, range, kind, name)`.
  - Sort imports by `(file, range, source)`.
  - Sort type / heritage facts by `(subject, range)`.
  - Use relative file paths only.
  - Two-space indent.
  - Exclude raw tree-sitter node IDs, absolute paths, timestamps,
    environment-specific fields.

### First-extraction lifecycle

- **D-04: Background extraction on workspace activation, with centralized
  ready-gate.** `kernel.ActivateWorkspace` does NOT block on full extraction.
  The activation flow is:

  ```go
  func ActivateWorkspace(ctx, req) (*Workspace, error) {
      ws := workspaceManager.Open(...)
      semanticStore := semanticStoreManager.Open(ctx, ws.ID)  // Phase 57 path
      watcherManager.Start(ws)                                 // Phase 60 fills body
      semanticScheduler.ScheduleInitialExtraction(ws.ID, InitialExtraction{
          Reason: "workspace_activation",
          Mode:   IncrementalIfPossible,
      })
      return ws, nil
  }
  ```

  **State enum:**

  ```go
  type SemanticIndexState string
  const (
      SemanticNotStarted SemanticIndexState = "not_started"
      SemanticIndexing   SemanticIndexState = "indexing"
      SemanticReady      SemanticIndexState = "ready"
      SemanticPartial    SemanticIndexState = "partial"
      SemanticFailed     SemanticIndexState = "failed"
      SemanticStale      SemanticIndexState = "stale"
  )
  ```

  **Scheduler interface:**

  ```go
  type ExtractionScheduler interface {
      ScheduleInitialExtraction(workspaceID WorkspaceID, req InitialExtraction) JobID
      ScheduleIncremental(workspaceID WorkspaceID, changes []FileChange) JobID  // body landed in Phase 60
      Status(workspaceID WorkspaceID) SemanticStatus
      Subscribe(workspaceID WorkspaceID) <-chan SemanticStatus
  }
  ```

  Phase 59 ships the interface and a working `ScheduleInitialExtraction`
  implementation. Phase 60 fills `ScheduleIncremental`. Phase 64 wraps
  `Status()` as the `semantic_graph_status` MCP tool.

  **Scheduler invariants:**
  - One active extraction job per workspace at a time.
  - `ScheduleInitialExtraction` is idempotent — concurrent calls return the
    same `JobID`.
  - File changes during an in-flight initial extraction are queued for
    incremental application after initial completes (Phase 60 finalizes
    queue semantics).
  - Jobs are cancellable on workspace deactivation.

  **Centralized readiness helper** — semantic consumers MUST use this; no
  per-tool readiness loops:

  ```go
  type ReadyPolicy struct {
      Timeout       time.Duration       // default extraction_ready_timeout
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

  func RequireReady(
      ctx context.Context,
      workspaceID WorkspaceID,
      policy ReadyPolicy,
  ) (ReadyResult, error)
  ```

  Behavior:
  - `state == ready` → return immediately.
  - `state == partial` and `AllowPartial` → return partial.
  - `state == indexing` → wait until `ready|partial|failed` or `Timeout`.
  - `state == not_started` and `TriggerIfCold` → kick scheduler, then wait.
  - Timeout → return best partial if any facts exist; otherwise
    `not_ready` error / empty partial result.

  **Default `ReadyPolicy`:** `Timeout = cfg.SemanticIndex.ExtractionReadyTimeout`
  (default 30s), `AllowPartial = true`, `MinState = SemanticPartial`,
  `TriggerIfCold = true`.

  **Initial-walk priority order** (so partial results become useful fast):
  1. Files referenced by the in-flight tool call.
  2. Repomap-ranked "important" files (Phase 25/30 PageRank already exists).
  3. Remaining first-class-language source files.
  4. Non-first-class-language files (file-row-only, see D-05).

  **Non-semantic tools** (read_file, find_symbol-via-LSP, search,
  repomap tools) DO NOT call `RequireReady`. They run with no semantic
  dependency.

### Partial extraction model

- **D-05: Two-tier partial.**

  **Non-first-class languages** (Rust, Java, C, C++, C#, Ruby, PHP, Kotlin,
  Swift, Scala, ...): write `semantic_files` row only.

  ```
  extraction_status   = "unsupported"
  extraction_partial  = true
  partial_reason      = "unsupported_language"
  extractor_name      = NULL
  symbols/refs/imports/edges = NOT WRITTEN
  ```

  Distinguishes "file seen but unsupported" from "file missing from index".

  **First-class languages with errors:** write `semantic_files` row, write all
  facts the parser/queries safely produced, mark file (and boundary
  symbols/refs where applicable) as partial.

  ```
  extraction_status   = "partial"
  extraction_partial  = true
  partial_reason      ∈ { parse_error | query_error | timeout
                        | file_too_large | binary_or_generated
                        | permission_denied | extractor_bug }
  ```

  **Reason enum (closed set):**
  ```
  unsupported_language
  parse_error
  query_error
  timeout
  file_too_large
  binary_or_generated
  permission_denied
  extractor_bug
  ```

  **Schema delta vs. Phase 57:** the existing
  `semantic_files`/`semantic_symbols`/`semantic_references` tables don't yet
  carry `extraction_status` / `extraction_partial` / `partial_reason` /
  `extractor_name` / `extractor_version` / `error_message` columns. Phase 59
  ships a `Migration{From: 1, To: 2, Kind: InPlace}` via Phase 57's D-02
  registry adding these columns.

  **Note for the planner:** existing columns may already cover some of these
  semantics (`semantic_symbols.extraction_source`,
  `semantic_references.validation_state`, `confidence`, `reason`).
  The planner must compare the prescribed columns against the live Phase 57
  schema and choose, per column, whether to ALTER-add or repurpose-existing.
  The intent contract is: every consumer can answer
  "fully extracted? unsupported language? partial-with-which-reason?" from
  table columns alone, without joining or guessing.

  **Hard invariant:** Phase 59 does NOT use `internal/repomap` as a fallback
  inside `internal/semantic/extract/`. If best-effort fallback ever ships, it
  lands as `internal/semantic/enrich/repomapfallback/` with explicit
  `ExtractionSource = "repomap_fallback"` provenance and is excluded from
  default-confidence semantic queries. That work is post-v1.10.

  **Consumer-side availability classification** (used by the readiness helper
  and by Phase 64 tool wrappers):

  ```go
  type FileSemanticAvailability string
  const (
      AvailabilityReady       FileSemanticAvailability = "ready"
      AvailabilityPartial     FileSemanticAvailability = "partial"
      AvailabilityUnsupported FileSemanticAvailability = "unsupported"
      AvailabilityFailed      FileSemanticAvailability = "failed"
      AvailabilityMissing     FileSemanticAvailability = "missing"
  )
  ```

### 2026-05-08 update — Phase 65 unblock delta

The base Phase 59 (D-01..D-05) verified PASSED 2026-05-04. Phase 65 Wave 0
(production buildFn at `internal/daemon/semantic_wiring.go:687-742`) cannot
consume the shipped extractors polymorphically because (a) `Extract` was not
promoted to the `extract.Provider` interface and (b) no `*ExtractedFile →
semanticstore.Facts` adapter exists. The four decisions below close that gap.
They are additive — base D-01..D-05 still hold verbatim.

- **D-06: Promote `Extract` onto the `extract.Provider` interface.** Each
  per-language provider already implements
  `Extract(ctx context.Context, source []byte, file extract.SourceFile)
  (*extract.ExtractedFile, error)` as a concrete method
  (`internal/semantic/extract/golang/provider.go:72`,
  `typescript/provider.go`, `python/provider.go`). The 2026-05-04
  comment at `extract/provider.go:11-15` ("Per-language helper signatures
  ... are intentionally NOT here yet — they are finalized in Phase 59 P04
  when the per-language providers land") was the right call at design
  time and the wrong shape now that P04 has shipped. Promote it.

  ```go
  // internal/semantic/extract/provider.go
  type LanguageMetadata interface {
      Language() string
      Extensions() []string
      SupportsLSPEnrichment() bool
  }

  type ExtractionPipeline interface {
      Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)
  }

  type Provider interface {
      LanguageMetadata
      ExtractionPipeline
      TreeSitterLanguage() *tree_sitter.Language
      Queries() string
  }
  ```

  **Hard invariants:**
  - The signature MUST match the existing concrete `Extract` method shape
    byte-for-byte — `(ctx context.Context, source []byte, file SourceFile)
    (*ExtractedFile, error)` — so promotion is a 1-line interface delta
    plus mock retrofits, not a per-provider rewrite.
  - The named sub-interfaces (`LanguageMetadata`, `ExtractionPipeline`)
    are non-load-bearing today — they exist so future helper sub-types
    can attach without churning every consumer. Phase 65 callers should
    type against `Provider` not the partial sub-interfaces unless they
    have a concrete reason to narrow.
  - The deferred helper sub-types from the original P02 design
    (`ImportResolver`, `ScopeBuilder`, `SymbolNormalizer`,
    `ReferenceClassifier`, `QueryBundle`) stay deferred — they do not
    exist as concrete types yet and adding them speculatively would lock
    in shapes ahead of need. `Queries() string` stays as-is for now.

  **Why not a registry-level helper (`Registry.Extract(lang, ...)`):**
  pretends `Provider` is lookup-only while implementing provider
  behavior — splits the responsibility. Extraction is provider-owned.

  **Why not concrete-package import from Phase 65:** every future
  language addition would force a Phase 65 edit. Forfeits the registry
  abstraction at exactly the place the abstraction matters.

  **Phase 65 call site after promotion:**

  ```go
  provider, ok := registry.Provider(lang)
  if !ok { /* unsupported_language path */ }
  extracted, err := provider.Extract(ctx, source, file)
  ```

- **D-07: Phase 65 owns the workspace walk; `ScheduleInitialExtraction`
  stays state-only.** The 2026-05-04 verification flagged the scheduler
  body as `STATIC` (state transition only, no file walk) and treated that
  as intentional. Phase 65 confirms the same boundary going forward —
  Phase 65's production buildFn drives its own walk and dispatches to
  providers via D-06's interface. The scheduler's value to Phase 65 is
  the orchestration surface (`Status`/`Subscribe`/job lifecycle/priority
  queue helpers from `internal/semantic/scheduler/priority.go`), not the
  walk itself.

  **Hard invariants:**
  - Phase 59 ships NO new file-walk code in this update.
  - The 4-tier `PriorityQueue` from `internal/semantic/scheduler/priority.go`
    is exported / consumable by Phase 65's buildFn — if it currently is
    package-private, expose what's needed (planner's call). No new logic
    inside the queue.
  - Phase 65's buildFn is responsible for reporting progress back into
    the scheduler's `Status()` so external observers see consistent
    state. The scheduler does not introspect the buildFn.

- **D-08: `*ExtractedFile → semanticstore.Facts` adapter lives in
  `internal/semantic/extract/`.** Ship `extract.ToStoreFacts` (or
  similar — planner picks final name) as a pure conversion function:

  ```go
  // internal/semantic/extract/to_store.go (NEW)
  func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts
  ```

  **Rationale:** the fact-shape conversion is intimate with the
  `*ExtractedFile` shape; placing the adapter next to the fact
  definitions keeps the conversion under the same review lens as the
  shapes it converts. Phase 65's buildFn becomes a thin orchestrator
  (walk → Extract → ToStoreFacts → WriteSnapshotFacts) instead of
  carrying conversion logic.

  **Hard invariants:**
  - Pure function — no I/O, no scheduler/store dependencies. Determinism
    is the contract: same `[]*ExtractedFile` → byte-identical
    `semanticstore.Facts`.
  - The function lives under `internal/semantic/extract/` but the
    package MUST NOT introduce a cyclic import:
    `internal/semantic/extract/` already does not import
    `internal/semantic/store/`; this conversion needs the store's
    `Facts` type, so the adapter file imports
    `internal/semantic/store/` one-way (extract → store). That edge is
    new — verify it does not close a cycle (store does not import
    extract today; the cycle check is the planner's gate).
  - If the cycle check fails, fall back to placing the adapter in
    `internal/semantic/store/` as `Facts.FromExtracted([]*ExtractedFile)`
    — preserves the shape with the inverse import direction. The user's
    locked decision is "live in extract/" but cycle-safety is a hard
    constraint that overrides location preference.

- **D-11: Bookkeeping — tick EXTRACT-01..05 and correct ROADMAP plan
  count in this update.** REQUIREMENTS.md still shows the EXTRACT
  requirements as `[ ]` Pending and ROADMAP.md line 135 says "Phase 59
  (0/0 plans)" though five plans (59-01..59-05) shipped. This is
  documentation drift that already caused Phase 65's CONTEXT.md to
  declare the phase BLOCKED on Phase 59 when in fact the base phase had
  shipped. Fix in this same commit:
  - REQUIREMENTS.md `[ ]` → `[x]` for EXTRACT-01..05 in both the
    requirement-detail block and the requirement-table.
  - ROADMAP.md line 135 `(0/0 plans)` → `(5/5 plans)`.
  - Cross-reference: `.planning/phases/59-.../59-VERIFICATION.md`
    (already PASSED 2026-05-04) is the evidence that the requirements
    are satisfied.

### Acceptance criteria (must hold at end of phase)

1. `internal/semantic/extract/` does not import `internal/repomap`.
2. No `init()` registration in any `internal/semantic/extract/...` package; no
   blank-imports file for providers.
3. `NewExtractorRegistry` panics on duplicate `Provider.Language()`.
4. Each first-class provider has both golden tests (with `-update` support)
   and table-driven stable-ID tests; ≥30 scenarios per language.
5. Stable IDs are deterministic across runs and OSes; whitespace-only edits
   preserve IDs; same-content rename preserves IDs; exported-symbol move
   within package preserves IDs.
6. Failed golden tests print `cmp.Diff`; failed stable-ID tests report
   symbol, kind, before ID, after ID, expected transition.
7. Schema version stamps move from `1` (Phase 57) to `2` (Phase 59); the
   migration succeeds on a clean store and on a Phase-57-populated store.
8. `kernel.ActivateWorkspace` returns within the existing activation budget
   (no regression vs. pre-Phase-59 baseline) regardless of repo size.
9. `semantic.RequireReady` is the only readiness API used by semantic
   consumers; no `time.Sleep`-based polling appears in `internal/semantic/`
   or its callers.
10. For a workspace containing Rust files (or any non-first-class language),
    `semantic_files` rows exist with `extraction_status="unsupported"` and no
    `semantic_symbols` rows.
11. EXTRACT-05 invariant: every provider receives the daemon-injected
    `*treesitter.GrammarRegistry`. A regression test attempts to construct a
    second `GrammarRegistry` inside the semantic extractor path and fails.

### Acceptance criteria — 2026-05-08 update (D-06 / D-07 / D-08 / D-11)

12. The `extract.Provider` interface in
    `internal/semantic/extract/provider.go` declares `Extract(ctx
    context.Context, source []byte, file SourceFile) (*ExtractedFile,
    error)`. The three concrete providers (`golang`, `typescript`,
    `python`) compile against the new interface without source change to
    their `Extract` method bodies.
13. A polymorphic call site exercises the promoted method:
    `registry.Provider("go").Extract(...)` (or equivalent) succeeds in a
    test under `internal/semantic/extract/`. No type assertion on
    concrete `*Provider` types is needed in the call site.
14. `internal/semantic/scheduler/`'s `ScheduleInitialExtraction` body
    remains state-only (Phase 65 owns the walk per D-07). The 2026-05-04
    "STATIC" data-flow note in 59-VERIFICATION.md remains accurate after
    this update.
15. `extract.ToStoreFacts([]*ExtractedFile) semanticstore.Facts` (or
    fallback location per D-08 cycle-safety clause) is a pure
    deterministic function with a unit test asserting same-input →
    byte-identical output across repeated calls.
16. `go vet ./...` and `go test ./internal/semantic/...` pass after the
    interface promotion. The Phase 59 P04 golden snapshots remain
    byte-identical (the promotion is a contract widening, not a
    behavioral change).
17. REQUIREMENTS.md EXTRACT-01..05 read `[x]` (both the detail block and
    the requirement table). ROADMAP.md line 135 reads `(5/5 plans)`. No
    other phases or requirements are touched in this bookkeeping pass.

### Claude's Discretion (no user input needed)

- **Stable-ID hash function:** `xxhash64` per SPEC §11.1 verbatim. The
  canonicalization function uses the field order `RepoID, Language,
  PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash, LSPIdentity,
  FilePathFallback` joined by `\x00`, matching the `StableSymbolKey` struct
  order in SPEC §11.1. Tie-breaking in case of hash collision (extremely
  unlikely with 64 bits) is "preserve first-seen, log warning" — collisions
  beyond statistical noise indicate a canonicalization bug, not a hash bug.
- **Per-language `provider.go` shape:** `goextract.Provider` /
  `tsextract.Provider` / `pyextract.Provider` are concrete types implementing
  the `extract.Provider` interface. Constructor takes `*treesitter.GrammarRegistry`,
  returns `extract.Provider` (interface). Per-package state limited to
  precompiled query handles and a `querycache.Cache` if needed.
- **TypeScript and JavaScript share a provider** with two language IDs (and
  two extension lists) — the syntax/query overlap is large enough that the
  divergence (TS-only `interface`, decorators, generics) is captured by
  query alternation rather than two providers. Provider name:
  `tsextract.NewProvider`. The provider's `Language()` returns
  `"typescript"` for canonical purposes; the `Extensions()` list covers
  `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`. (If the planner finds the
  query overlap is smaller than expected, splitting into two providers is
  acceptable; document the change in PATTERNS.md and SUMMARY.md.)
- **`internal/semantic/scheduler/`** is the suggested home for the scheduler
  + readiness helper. Final package name is the planner's call as long as it
  stays under `internal/semantic/` and out of `internal/semantic/extract/`.
- **Initial-walk concurrency:** `max_parallel_files` defaults to 4 — matches
  the existing repomap walker default in `internal/repomap/extractor.go`.
  Tunable via config.
- **Per-file extraction timeout:** `extraction_file_timeout` defaults to 3s
  per the user prescription in the discussion. Files that exceed this get
  `partial_reason="timeout"` and whatever facts were extracted before the
  timer fired.
- **`max_file_size`:** default 2 MiB; files larger get
  `extraction_status="unsupported"`, `partial_reason="file_too_large"`.
  No partial extraction attempted (we don't know what state the file is in).
- **Bounded-label metric for extraction outcomes:**
  `helix_semantic_extraction_total{language, outcome}` where `outcome ∈
  {ready, partial, unsupported, failed}` and `language` is restricted to the
  closed set `go|typescript|python|other`. Goes through the existing
  `internal/obs/` bounded-label registration (Phase 57 D-06/D-07 path).
  This is one metric — additional metrics are the planner's call but must
  use the same bounded-label allowlist.
- **No `helix index` CLI subcommand in Phase 59.** The user's discussion
  flagged it as a "should still exist" CLI but explicitly outside the
  current decision. Capture in Deferred Ideas.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §EXTRACT — EXTRACT-01 through EXTRACT-05
  phrasing is the contract.
- `.planning/milestones/v1.10-ROADMAP.md` Phase 59 block (lines 58–68) —
  Goal, Depends on, Requirements, 4 Success Criteria.

### Specification (source of truth for shapes and key names)

- `SPEC-DRAFT.md` §6 — `internal/semantic/` directory layout (the layer-1.5
  envelope `internal/semantic/extract/` plugs into).
- `SPEC-DRAFT.md` §7 — `SnapshotID`, `FileID`, `SymbolID`, `Freshness` types
  (already declared in `internal/semantic/types.go` by Phase 57).
- `SPEC-DRAFT.md` §11.1 — `StableSymbolKey` shape, canonicalization rules,
  `xxhash64` ID. The five canonicalization rules are normative.
- `SPEC-DRAFT.md` §11.2 — merge priority + 7-rung confidence ladder. Phase
  59 emits `Confidence = 0.70` for tree-sitter facts; Phase 61's enrichment
  worker raises it to 0.80/0.95/1.00 per the merge rule.
- `SPEC-DRAFT.md` §12.1 — Edge vocabulary. `ReferenceKind` and edge kinds
  emitted by the semantic classifier draw from this enum (`CALLS`,
  `REFERENCES`, `USES_TYPE`, `READS`, `WRITES`, `IMPORTS`, `CONTAINS`,
  `IMPLEMENTS`, `EXTENDS`, `OVERRIDES`, ...).
- `SPEC-DRAFT.md` §13.2 — File discovery exclusions (`.git/`, `node_modules/`,
  `vendor/` unless configured, `dist/`, `build/`, `target/`, `coverage/`,
  generated files above configured size, binary files,
  files above `max_file_size`).
- `SPEC-DRAFT.md` §13.3 — `LanguageProvider` interface. Phase 59's
  `extract.Provider` mirrors the spec interface; `extract.Registry` adds the
  constructor-injection layer.
- `SPEC-DRAFT.md` §13.4 — Generic capture names (extended in D-01a above).
- `SPEC-DRAFT.md` §13.5 — Extraction pseudocode. The Phase 59 pipeline
  (parse → run queries → build scopes → ExtractSymbols/Refs/Imports →
  Normalize → Classify → BuildSyntaxEdges) follows this verbatim.
- `SPEC-DRAFT.md` §25 — `semantic_index.*` config keys. Phase 59 adds the
  `semantic_index.extraction.*` subtree (`extraction_ready_timeout`,
  `extraction_file_timeout`, `max_parallel_files`, `max_file_size`,
  `allow_partial_results`, `initial_extraction_on_activation`).
- `SPEC-DRAFT.md` §32 Phase 1 — "Tree-sitter Extraction" deliverables
  and acceptance criteria.

### Phase 57 lock-down (must not regress)

- `.planning/phases/57-semantic-store-foundation-pipeline-dag-library/57-CONTEXT.md`
  — D-01 (schema_version=1 stamped on snapshots), D-02 (explicit migration
  registry — Phase 59's v1→v2 migration MUST flow through this), D-04 (plan
  structure precedent), D-12 (DuckDB module path lock).
- `internal/semantic/types.go` — typed identifiers; Phase 59 adds new IDs
  (`ImportID`, `TypeFactID`, `HeritageID`) here.
- `internal/semantic/store/migrations.go` — current schema (Phase 57). The
  v1→v2 migration extends `semantic_files`/`semantic_symbols`/
  `semantic_references` per D-05.

### Architectural invariants

- `.planning/milestones/v1.9-MILESTONE-AUDIT.md` — Phase 49 (single canonical
  `GrammarRegistry`) is the BUG-04 invariant referenced by EXTRACT-05.
- `internal/treesitter/registry_cgo.go` / `registry_nocgo.go` — the
  singleton `*treesitter.GrammarRegistry` source. Daemon bootstrap injects
  this into the extractor registry.
- `CLAUDE.md` "Middleware Execution Order (LIFO)" section — Phase 59 does
  not modify middleware. `LazyInitMiddleware` triggering activation is the
  entry-point edge for D-04's background-extraction kickoff.
- `internal/daemon/daemon.go` — bootstrap step ordering; new wiring lands
  after step 6b (Phase 57 semantic store open).

### Pattern templates (must mirror)

- `internal/repomap/extractor.go` — example of a tree-sitter extraction
  pipeline with query loading, walker, ignore rules. Phase 59 does NOT
  import or borrow this code, but the planner reads it as a structural
  reference for the equivalent `internal/semantic/extract/` walker.
- `internal/repomap/queries/*.scm` — example tree-sitter query format. Phase
  59's `queries.scm` files are net-new but must lex/compile under the same
  go-tree-sitter query API.
- `internal/skill/` Caddy-style registry — explicitly NOT mirrored in
  Phase 59 (D-02 rejects init-style registration for semantic extraction).
  Listed here so the planner doesn't accidentally copy that pattern.
- `internal/langregistry/installer.go` — the constructor-injected pattern
  the new `extract.Registry` mirrors.
- `internal/config/loader_test.go` `TestLoad_*Defaults` family — the
  per-feature defaults test template. Phase 59 adds
  `TestLoad_SemanticExtractionDefaults` (or similar) for the new keys.

### Validation tooling

- `cmd/vet-noduckdb/` — Phase 57 vet-tool. Phase 59's `internal/semantic/extract/`
  packages MUST NOT import `duckdb-go`; the existing analyzer enforces this.

### 2026-05-08 update — additional refs for D-06 / D-07 / D-08 / D-11

- `internal/semantic/extract/provider.go` — current `Provider` interface
  (5 methods, lookup-side). D-06 promotes `Extract` to this interface.
- `internal/semantic/extract/golang/provider.go:72` — concrete `Extract`
  signature; the promoted interface method MUST match this byte-for-byte.
- `internal/semantic/extract/typescript/provider.go` — concrete `Extract`.
- `internal/semantic/extract/python/provider.go` — concrete `Extract`.
- `internal/semantic/extract/registry.go` — Registry; D-06 unchanged at
  the registry level (`Provider(lang)` already returns the interface).
- `internal/semantic/store/snapshot.go` — `WriteSnapshotFacts` and the
  `Facts` type that D-08's `ToStoreFacts` returns.
- `internal/daemon/semantic_wiring.go:687-742` — Phase 65 Wave 0
  consumer; the production buildFn placeholder this update unblocks.
- `internal/semantic/scheduler/priority.go` — 4-tier `PriorityQueue`
  Phase 65's buildFn may consume per D-07; check current export
  surface.
- `.planning/phases/59-.../59-VERIFICATION.md` — 2026-05-04 PASSED
  evidence; D-11 cites this when ticking REQUIREMENTS.md.
- `.planning/REQUIREMENTS.md` — EXTRACT-01..05 (`[ ]` today; `[x]` post
  D-11). Both the detail block and the requirement table.
- `.planning/ROADMAP.md:135` — `(0/0 plans)` today; `(5/5 plans)` post
  D-11.
- `.planning/phases/65-existing-tool-integration-strangler-fig/65-CONTEXT.md`
  D-09/D-10 — Phase 65's "BLOCKED on Phase 59" claim that this update
  retires.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`*treesitter.GrammarRegistry`** (`internal/treesitter/registry_cgo.go`) —
  the BUG-04 singleton holding all 23 compiled grammars. The daemon
  bootstrap creates it once and passes it to repomap, the kernel edit
  pipeline, and (Phase 59) the semantic extractor registry. Phase 59 does
  NOT create a second registry; it accepts one.
- **`internal/repomap/pagerank.go` / `internal/repomap/graph.go`** — the
  ranking signal used by the initial-walk priority order in D-04. The
  scheduler reads ranked file order from repomap, NOT by importing repomap
  into `internal/semantic/extract/` (D-01 invariant) but via a thin
  scheduler-level call (`internal/semantic/scheduler/` may import repomap;
  `internal/semantic/extract/` may not).
- **`internal/semantic/store/`** — Phase 57 store with three-tier open,
  schema_version stamping, migration registry. Phase 59's v1→v2 migration
  plugs into the existing `Migration{From, To, Kind}` registry.
- **`internal/obs/`** — bounded-label metric registration. The new
  `helix_semantic_extraction_total` counter uses the existing path.
- **`internal/errors/kinds.go` `serr.Unsupported`** — used by
  `extraction_status="unsupported"` consumer paths if anything explicitly
  errors out for non-first-class langs (consumer-side, not extractor-side).
- **`internal/config/`** koanf 4-layer precedence — extraction config keys
  added to `defaults.go` and the `SerenaConfig.SemanticIndex` struct
  field that Phase 57 P02/P03 already set up.
- **xxhash64** — `github.com/cespare/xxhash/v2` is already in the dependency
  graph (used by other Helix subsystems); reuse the existing import.

### Established Patterns

- **Constructor-injected registry** (`internal/langregistry/installer.go`,
  `internal/repomap/extractor.go`'s `NewTagExtractor(registry)`). Phase 59
  follows this pattern, not the Caddy-style skill init() pattern.
- **CGO=0 stub mirror** (Phase 51.1 / Phase 57 D-04). Tree-sitter requires
  CGO; the daemon refuses CGO=0 at step 6a so semantic extraction never
  runs in CGO=0. Provider packages may need a `_nocgo.go` stub returning
  `serr.ErrUnsupported` for build-tag completeness, mirroring
  `internal/repomap/extractor_nocgo.go`.
- **Per-feature defaults test** (Phase 57 D-09). The new
  `extraction_*` config keys get their own `TestLoad_SemanticExtractionDefaults`.
- **Bounded-label metrics with closed enum** (Phase 57 D-07). The
  `language` and `outcome` labels on the new extraction counter both come
  from closed enums; neither accepts arbitrary strings.

### Integration Points

- **Daemon bootstrap (`internal/daemon/daemon.go`)** — after Phase 57's
  step 6b (semantic store open), Phase 59 adds:
  - Construct `extract.Registry` with the three providers.
  - Construct `scheduler.Scheduler` (or chosen package name) with the
    extractor registry, semantic store, and config.
  - Pass scheduler into `kernel.ActivateWorkspace` callback path so
    activation triggers `ScheduleInitialExtraction`.
  - Register the readiness API at `daemon.Semantic` (or equivalent
    surface) so `internal/semantic/.../require_ready.go` and future
    Phase 64 tool wrappers can resolve it.
- **`SerenaConfig.SemanticIndex`** field (Phase 57 P02/P03) gains an
  `Extraction` sub-struct carrying the new keys. koanf binding for
  `semantic_index.extraction.*` added to `defaults.go`.
- **`Makefile` `vet` target** (Phase 57 P04) already enforces "no
  duckdb-go outside store/". Phase 59 adds no new vet tool; just make sure
  the new packages pass the existing analyzer.

### Constraints

- Phase 59 must NOT touch middleware order (CLAUDE.md "Middleware Execution
  Order (LIFO)") — `LazyInitMiddleware` keeps its current install position.
- Phase 59 must NOT introduce a second `GrammarRegistry` (BUG-04 / EXTRACT-05).
  The regression test in D-01b acceptance criteria #11 enforces this.
- Phase 59 must NOT import `internal/repomap` from inside
  `internal/semantic/extract/` (D-01).

</code_context>

<specifics>
## Specific Ideas

- **Provider-test golden output excludes stable IDs initially**, then opts in
  via a normalizer flag once the algorithm is locked. This avoids golden
  churn while the canonicalization rules are tuned during EXTRACT-03 review.
- **TS+JS provider** uses query alternation rather than two providers
  (Claude's discretion above); the planner may split if the overlap turns
  out smaller than expected, but the default is one provider.
- **TypeScript decorator support** is the gating syntax for the Python
  decorator scenario reuse — both languages emit `decorator.name` captures
  but their decorator semantics differ; the classifier handles this per
  language, not in shared code.
- **Stable-ID test fixtures live inline** for ≤50-line cases, in `testdata/`
  for larger ones. The line-cutoff threshold is the planner's call but the
  user's prescription suggests "small before/after identity fixtures may be
  inline".
- **The `unsupported_language` extraction outcome** is registered as a metric
  label so we can dashboard "how many files in the workspace are non-first-class
  today?" without joining tables. This data informs which language gets
  promoted to first-class in v1.11/v1.12.
- **Initial-walk priority** is an opportunity, not a hard contract. Phase 59
  ships the priority logic; the planner can choose to implement priority via
  a heap of `(priority, file)` entries or via a multi-pass walker. Same
  externally observable behavior either way.
- **The schema-migration plan is its own task within Phase 59**, not folded
  into the provider-implementation plan, because it touches Phase 57 code
  and benefits from a focused review. The planner decides the wave layout.

</specifics>

<deferred>
## Deferred Ideas

- **`helix index` CLI subcommand** — the user mentioned it as a "should still
  exist" knob (`helix index`, `helix index --rebuild`, `helix index --status`).
  Phase 59 does NOT ship it; daemon scheduler is the entry point. Future
  small-phase candidate, post-Phase-64.
- **Repomap fallback as best-effort enrichment provider** —
  `internal/semantic/enrich/repomapfallback/` translating repomap tags to
  low-confidence semantic facts under explicit `ExtractionSource`
  provenance. Excluded from default semantic queries; opt-in via
  `semantic_index.fallback.repomap.enabled=true`. Post-v1.10 backlog.
- **Semantic schema generic key-coverage matrix** — Phase 57 deferred this;
  Phase 59 inherits the deferral. Per-feature defaults test only.
- **Multi-provider per language** — TS+JS as one provider in Phase 59; if the
  query overlap is smaller than expected, future phase may split. Not now.
- **Cross-file LSP-resolved references in fact rows** — Phase 59 leaves
  `ReferenceFact.ResolvedTarget = nil` and `ResolutionSource = ""`. Phase 61
  populates these via the LSP enrichment worker.
- **Stable-ID generic-instantiation handling** — generic functions/types
  produce one stable ID per declaration, not per instantiation. Per-instantiation
  IDs are an analysis-grade question for Phase 62 (graph engine) if it ever
  needs them. Captured in shared scenario `generic_function_or_type`.
- **`semantic_graph_status` MCP tool wrapper** — Phase 64. Phase 59 ships
  `Status()`/`Subscribe()` API only.
- **Live overlay `overlay_epoch` interaction with extraction state** — Phase
  60 owns this; Phase 59 emits to snapshot tables only, not overlay tables.
- **Hard total-extraction budget** — initial-extraction has no hard total
  timeout by default (per user prescription). Future phase may add one if
  large-monorepo runs need it.

### Deferred — 2026-05-08 update

- **Helper sub-types on `Provider`** (`ImportResolver`, `ScopeBuilder`,
  `SymbolNormalizer`, `ReferenceClassifier`, `QueryBundle`) — the
  original P02 design referenced these as future extension points.
  D-06 splits the interface into `LanguageMetadata` +
  `ExtractionPipeline` so they can attach later without churning every
  consumer, but the helper types themselves stay deferred until a
  consumer actually needs them. Do not add speculatively.
- **Phase 59-side workspace walker** — D-07 locks Phase 65 as the owner
  of the file walk. If a future phase needs a generic walker, ship it
  as `extract.WalkAndExtract(ctx, registry, root, opts)` per the option
  rejected in 2026-05-08 discussion. Not in this update.
- **`Facts.FromExtracted` on the store side** — only ships if D-08's
  cycle-safety check fails. If extract → store imports cleanly, this
  variant stays deferred indefinitely.

</deferred>

---

*Phase: 59-tree-sitter-extraction-stable-symbol-ids*
*Context gathered: 2026-05-04*
*Updated: 2026-05-08 — D-06 / D-07 / D-08 / D-11 (Phase 65 unblock delta)*
