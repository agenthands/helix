# Phase 59: Tree-sitter Extraction & Stable Symbol IDs - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-04
**Phase:** 59-tree-sitter-extraction-stable-symbol-ids
**Areas discussed:** RepoMap reuse strategy, Test matrix layout, First-extraction trigger, partial:true semantics

---

## RepoMap reuse strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Net-new queries, hard split | `internal/semantic/extract/{go,typescript,python}/queries.scm` are net-new and capture SPEC §13.4 generic captures. Repomap untouched, owns its 23-language def/ref world. Two extraction systems coexist. | ✓ |
| Hybrid — borrow queries, own logic | Copy repomap `*_tags.scm` into `extract/<lang>/queries.scm` AND extend with §13.4 captures. Extraction logic net-new. Saves writing queries from scratch but commits to repomap baseline. | |
| Adapter on top of repomap.TagExtractor | `internal/semantic/extract/` wraps `repomap.TagExtractor` and translates `Tag→SymbolFact/ReferenceFact` plus a second pass for §13.4 captures repomap doesn't have. Highest reuse, tightest coupling. | |

**User's choice:** Net-new queries, hard split — explicitly preserve the boundary between navigation-oriented repomap extraction and analysis-oriented semantic extraction. Future consolidation only after semantic proves better coverage and correctness; not in this phase.

**Notes:** User prescribed the directory layout (`provider.go`, `fact.go`, `normalizer.go`, `classifier.go`, `registry.go` + per-language `{queries.scm, provider.go}`), the extended capture taxonomy beyond SPEC §13.4 (decorator.name, receiver.type, receiver.name, definition.parameter, type.return, type.parameter, import.symbol), and the fact struct shapes (`SymbolFact`, `ReferenceFact`, `ImportFact`, `TypeFact`, `HeritageFact`). Hard invariants: no repomap import inside `internal/semantic/extract/`, no copied query files, no `Tag→SymbolFact` adapter.

---

## Provider registration (follow-up to RepoMap strategy)

| Option | Description | Selected |
|--------|-------------|----------|
| Caddy-style `init()` registration | Each per-lang package calls `extract.RegisterProvider(...)` in `init()`. Daemon blank-imports providers. Mirrors `internal/skill/`. | |
| Constructor-injected at bootstrap | `NewExtractorRegistry(grammars, providers ...Provider) *Registry`. Daemon constructs each provider explicitly. Mirrors `langregistry.NewInstaller`. | ✓ |
| Hybrid — `init()` + `Configure()` | Self-register via `init()`, daemon calls `Configure(grammars)` to inject the singleton. | |

**User's choice:** Constructor-injected. Semantic extraction is core infrastructure with real runtime dependencies (`*GrammarRegistry`, query cache, normalizer config) — hiding construction behind init() makes provider availability implicit and tests harder. The two-phase hybrid was rejected for creating a temporal "registered but not configured" invalid state.

**Notes:** User prescribed the API shape, panicking on duplicate `Language()`, and the bootstrap snippet. Acceptance criteria: no init() registration, no blank imports, registry constructible with one provider in tests, GrammarRegistry injected before any provider is usable.

---

## Test matrix layout

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid — golden + table-driven | Golden snapshots for full-extraction correctness; table-driven for stable-ID transitions with explicit `Preserved \| Churned \| NewlyDefined \| Removed` enum. Each scenario picks the mode that fits its assertion. | ✓ |
| All golden snapshots | Every scenario is `extract/<lang>/testdata/<scenario>/{before, after, expected.json}`; JSON includes both extraction and ID transitions. Stable-ID assertions buried in JSON. | |
| All table-driven | 30+ scenarios per language as table entries with `//go:embed` for larger fixtures. Scenarios live next to test code; large fixtures clutter the test file. | |

**User's choice:** Hybrid. "Golden = full extraction correctness; table-driven = stable-ID semantic invariants — those are different kinds of assertions and shouldn't be forced into one format."

**Notes:** User prescribed the directory layout, the golden test contract (`-update` flag, deterministic JSON normalization, `cmp.Diff` on fail), the stable-ID test contract (table-driven with inline source strings, explicit `StableIDExpectation`), the full ~30-scenario taxonomy split (25 shared + 8-9 language-specific) per first-class language, and the JSON normalization rules. Stable IDs may appear in golden once the algorithm is locked, behind a normalize-time skip flag.

---

## First-extraction trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Background on activation, ready-gate | Activation returns immediately; goroutine kicks off extraction; consumers wait via shared helper. Non-semantic tools never wait. | ✓ |
| Eager during activation | Activation blocks until extraction (or budget hit). Simple consumer contract; first-tool latency includes extraction. | |
| Lazy on first semantic tool call | Extraction triggered by first `find_semantic_*` call. Two sync.Once layers; variable semantic-tool latency. | |
| Explicit CLI-only | User runs `helix index`. No automatic extraction. Bad onboarding. | |

**User's choice:** Background on activation, with an adjustment: **centralize the ready-gate** via `semantic.RequireReady(ctx, workspaceID, ReadyPolicy)` to avoid per-tool boilerplate.

**Notes:** User prescribed the full lifecycle: `SemanticIndexState` enum (`not_started|indexing|ready|partial|failed|stale`), `ExtractionScheduler` interface, `ScheduleInitialExtraction` idempotency, scheduler invariants (one job per workspace, queued incremental during initial, cancellable on deactivation), `ReadyPolicy` shape and defaults (`Timeout = extraction_ready_timeout=30s`, `AllowPartial=true`, `MinState=SemanticPartial`, `TriggerIfCold=true`), default config keys, initial-walk priority order (open files → repomap-ranked → remaining first-class → unsupported), and the consumer contract ("semantic tools call `RequireReady`; never `time.Sleep` polling").

---

## partial:true semantics

| Option | Description | Selected |
|--------|-------------|----------|
| File row only, no extraction | Walk the tree, write `semantic_files` row with `extraction_partial=true, partial_reason='unsupported_language'`. No facts. | partially ✓ (non-first-class langs) |
| Best-effort via repomap, low confidence | Run `repomap.TagExtractor` on non-first-class langs, translate Tag→SymbolFact at confidence 0.45. Soft repomap dependency. | |
| File row + parse-error per-symbol | File row with `extraction_partial=true`; for first-class langs with parse errors, write what extracted before error + per-symbol partial markers. | partially ✓ (schema model) |
| Skip entirely | Files in non-supported langs not walked or recorded. Violates EXTRACT-01 wording. | |

**User's choice:** Two-tier hybrid — option 3's **schema and parse-error handling** for first-class languages with errors, **option 1's behavior** for non-first-class languages. **No repomap fallback** in this phase (would violate D-01 hard split).

**Notes:** User prescribed the closed enum of `partial_reason` values (`unsupported_language|parse_error|query_error|timeout|file_too_large|binary_or_generated|permission_denied|extractor_bug`), the schema delta against Phase 57 (`extraction_status`, `extraction_partial`, `partial_reason`, `extractor_name`, `extractor_version`, `error_message` columns added via v1→v2 migration through Phase 57 D-02 registry), the consumer-side `FileSemanticAvailability` enum (`ready|partial|unsupported|failed|missing`), and the firm rule against "polluting" the semantic graph with low-confidence heuristic facts. Future repomap fallback (if ever) lands in a separate `internal/semantic/enrich/repomapfallback/` package, post-v1.10.

---

## Claude's Discretion

- **Stable-ID hash function**: `xxhash64` per SPEC §11.1 verbatim; canonicalization joins fields with `\x00`; tie-break is "first-seen wins, log warning" since 64-bit collisions are statistically a canonicalization bug, not a hash bug.
- **Per-language `provider.go` shape**: concrete `Provider` type with `extract.Provider` interface return; per-package state limited to compiled queries + optional `querycache.Cache`.
- **TS+JS share one provider** with two language IDs (`Language() = "typescript"`, `Extensions = .ts/.tsx/.js/.jsx/.mjs/.cjs`); query alternation handles TS-specific syntax (interface, decorators, generics). Planner may split if overlap turns out smaller than expected.
- **Scheduler package home**: `internal/semantic/scheduler/` suggested; planner's call as long as it stays under `internal/semantic/` and out of `internal/semantic/extract/`.
- **Initial-walk concurrency**: `max_parallel_files=4` default (matches existing repomap walker); tunable.
- **Per-file extraction timeout**: `extraction_file_timeout=3s` default.
- **Max file size**: `max_file_size=2 MiB` default; larger files get `partial_reason="file_too_large"` and no extraction attempted.
- **Bounded-label metric**: `helix_semantic_extraction_total{language, outcome}` where `language ∈ {go, typescript, python, other}` and `outcome ∈ {ready, partial, unsupported, failed}` — closed enums via `internal/obs/`.
- **No `helix index` CLI subcommand in Phase 59**; deferred.

## Deferred Ideas

- `helix index` CLI subcommand (user-mentioned but explicitly out-of-scope here)
- Repomap fallback as a best-effort enrichment provider (`internal/semantic/enrich/repomapfallback/`)
- Cross-file LSP-resolved references in fact rows (Phase 61)
- Generic-instantiation stable IDs (Phase 62 if ever)
- `semantic_graph_status` MCP tool wrapper (Phase 64)
- Live overlay interaction (Phase 60)
- Hard total-extraction budget (future, only if monorepo runs need it)
- Multi-provider per language for TS+JS split (future phase if query overlap is smaller than expected)
- Generic config-key-coverage matrix (inherited deferral from Phase 57)

---

## 2026-05-08 update — Phase 65 unblock delta

**Trigger:** Phase 65 (existing-tool integration / strangler fig) declared
itself BLOCKED on Phase 59 in `65-CONTEXT.md` D-09/D-10. Reconciliation
showed Phase 59 base verified PASSED on 2026-05-04 (4/4 must-haves) but
Phase 65 Wave 0's production buildFn (`internal/daemon/semantic_wiring.go:687-742`)
could not consume the shipped extractors polymorphically. Documentation
drift (REQUIREMENTS `[ ]`, ROADMAP `0/0 plans`) compounded the confusion.

### Areas discussed

#### Extract API shape

**Options presented:**
1. Promote `Extract` to the `Provider` interface
2. Add a `Registry.Extract(lang, ...)` helper, keep interface lookup-only
3. Phase 65 imports the concrete language packages directly

**User selection:** **Option 1 — promote `Extract` to `Provider`.**

Rationale (paraphrased from the user's reply): extraction is provider-owned,
not registry-owned; option 2 splits the responsibility (registry pretending
to be lookup-only while implementing provider behavior); option 3 forfeits
the registry abstraction at exactly the place the abstraction matters and
forces every future language addition to edit Phase 65. User further
proposed splitting the interface into named sub-interfaces
(`LanguageMetadata`, `ExtractionPipeline`) for future helper attachment
without consumer churn — captured as the recommended shape in D-06. The
helper sub-types (`ImportResolver`, `ScopeBuilder`, `SymbolNormalizer`,
`ReferenceClassifier`, `QueryBundle`) stay deferred since they don't exist
as concrete types today.

Decision recorded as **D-06**.

#### Walk body ownership

**Options presented:**
1. Phase 65 owns the walk; scheduler stays state-only
2. Phase 59 ships the walk inside `ScheduleInitialExtraction`
3. Split — Phase 59 ships a `WalkAndExtract` library helper

**User selection:** **Option 1 — Phase 65 owns the walk.**

Confirms the 2026-05-04 verification's intentional `STATIC` data-flow
note. Phase 59 does not ship file-walk code in this update.

Decision recorded as **D-07**.

#### Facts adapter location

**Options presented:**
1. `internal/semantic/extract/` — extends Phase 59
2. `internal/semantic/store/` — Phase 57 territory
3. `internal/daemon/semantic_wiring.go` — Phase 65 inline

**User selection:** **Option 1 — `internal/semantic/extract/`.**

D-08 captures the decision plus a hard cycle-safety fallback to option 2
if the new extract → store import edge would close a cycle (planner
gate; today store does not import extract, so the edge is expected to be
clean).

Decision recorded as **D-08**.

#### Bookkeeping

**Options presented:**
1. Tick + correct in this update
2. Defer until interface-promotion plan executes & verifies

**User selection:** **Option 1 — tick + correct now.**

REQUIREMENTS.md EXTRACT-01..05 → `[x]`; ROADMAP.md line 135 → `(5/5 plans)`
with a 2026-05-08 update note. Cross-references the existing PASSED
2026-05-04 verification.

Decision recorded as **D-11**.

### Deferred (this update)

- Helper sub-types on `Provider` (`ImportResolver`, `ScopeBuilder`,
  `SymbolNormalizer`, `ReferenceClassifier`, `QueryBundle`) — wait until
  a consumer actually needs them.
- Phase 59-side workspace walker — locked out by D-07.
- `Facts.FromExtracted` on the store side — only if D-08 cycle check
  fails; otherwise stays deferred indefinitely.

### Scope guardrail

No scope creep this round. The four delta decisions all clarify HOW to
integrate the already-shipped extractors with the Phase 65 buildFn —
none add new capability beyond Phase 59's existing requirement set.
