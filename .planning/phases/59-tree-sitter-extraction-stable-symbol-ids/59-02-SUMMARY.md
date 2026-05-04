---
phase: 59
plan: 02
subsystem: semantic
tags: [semantic, extract, stable-id, xxhash64, confidence-ladder, config, spec-update, tdd]
requires: [57-03, 59-01]
provides:
  - extract.Provider interface (lookup-side surface)
  - extract.Registry constructor with dup/nil panic
  - extract.StableSymbolKey + CanonicalizeStableSymbolKey + StableSymbolID
  - extract.BuildProviderKey (delivers EXTRACT-02 same-content-rename invariant)
  - 5 confidence-ladder constants (float32) per SPEC §11.2
  - SymbolFact / ReferenceFact / ImportFact / TypeFact / HeritageFact / FileFact / SourceFile / ExtractedFile / ReceiverFact
  - Closed enums: SymbolKind, ReferenceKind, ExtractionStatus, PartialReason, FileSemanticAvailability
  - ImportID, TypeFactID, HeritageID identifier types in semantic
  - semantic.ExtractionConfig + 4 koanf defaults under semantic_index.extraction.*
  - obs.SemanticExtractionTotal helper + helix_semantic_extraction_total metric
  - SPEC §25 documents extraction.* sub-tree (4 keys) + indexing.* clarification note
  - ROADMAP Phase 59 SC#3 corrected from 7-rung to 5-rung
affects:
  - 59-03 (consumes extract.Provider, extract.Registry, semantic_index.extraction.*)
  - 59-04 (consumes extract.BuildProviderKey + ConfidenceTSOnly + fact types)
  - 59-05 (consumes extract.NewExtractorRegistry for daemon wiring)
tech-stack:
  added:
    - github.com/cespare/xxhash/v2 (promoted indirect → direct; used by SPEC §11.1)
  patterns:
    - "TDD RED → GREEN per task: test commit then feat commit"
    - "Closed-enum bounded-label allowlist with drop-on-unknown helper (T-59-02-02)"
    - "Constructor-injected GrammarRegistry (no init(), no blank imports — D-02)"
    - "Local Position/Range types in extract/ to keep zero LSP coupling (D-01)"
key-files:
  created:
    - internal/semantic/extract/doc.go
    - internal/semantic/extract/stable_id.go
    - internal/semantic/extract/stable_id_test.go
    - internal/semantic/extract/confidence.go
    - internal/semantic/extract/confidence_test.go
    - internal/semantic/extract/provider.go
    - internal/semantic/extract/fact.go
    - internal/semantic/extract/registry.go
    - internal/semantic/extract/registry_test.go
    - internal/semantic/extract/extract_cgo.go
    - internal/semantic/extract/extract_nocgo.go
    - SPEC-DRAFT.md (first commit — was untracked before Phase 59)
  modified:
    - internal/semantic/types.go (added ImportID, TypeFactID, HeritageID)
    - internal/semantic/config.go (added ExtractionConfig sub-struct + Config.Extraction)
    - internal/config/defaults.go (added 4 semantic_index.extraction.* defaults)
    - internal/config/loader_test.go (added 2 ExtractionConfig tests)
    - internal/obs/metrics.go (added SemanticExtraction vector + SemanticExtractionTotal helper)
    - internal/obs/metrics_labels_test.go (added allowlist entry, priming, dedicated test)
    - go.mod (cespare/xxhash/v2 promoted indirect → direct)
    - .planning/milestones/v1.10-ROADMAP.md (Phase 59 SC#3 fixed)
    - .planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-RESEARCH.md (Open Questions resolved)
decisions:
  - "Phase 59 emits 5-rung confidence ladder per SPEC §11.2, NOT the 7-rung §38.2 ladder (that's Phase 62 type-resolution territory). ROADMAP corrected."
  - "max_file_size and auto_index_on_activate STAY under semantic_index.indexing.*; only the 4 genuinely-new keys live under semantic_index.extraction.* (per user CONTEXT.md decision)."
  - "BuildProviderKey, not StableSymbolID, delivers EXTRACT-02's same-content-rename invariant. Exported symbols get FilePathFallback=''; unexported get RelPath."
  - "LSPIdentity stays in canonicalization input but is set to '' in Phase 59. Phase 61 may populate; the ID changes if and only if Phase 61 chooses to include it."
  - "Local Position/Range structs in extract/ instead of importing protocol/gen LSP types — keeps the package's zero-LSP-coupling contract (D-01)."
  - "Frozen TestCanonicalize_KnownVector pins the SPEC §11.1 hash for vector ('helix','go','github.com/x/pkg','*R','M','method','(int)','','') to xxhash64 = 0x032208087c467dcd."
metrics:
  tasks_completed: 3
  duration_minutes: ~50
  completed_date: 2026-05-04
  red_green_pairs: 3
---

# Phase 59 Plan 02: extract/ Package Skeleton + Stable-ID + Confidence Ladder + ExtractionConfig Summary

Lock the contract surface for Phase 59's tree-sitter extraction layer: deterministic stable-ID emission, named confidence-ladder constants, Provider/Registry shapes, fact-type structs, ExtractionConfig with 4 koanf-bound defaults, plus two doc amendments (ROADMAP SC#3 corrected from 7-rung to 5-rung; SPEC §25 documents the new extraction.* sub-tree).

## What Landed

**Stable-ID canonicalization (`internal/semantic/extract/stable_id.go`).** `StableSymbolKey` is the 9-field shape per SPEC §11.1 (RepoID, Language, PackagePath, OwnerPath, QualifiedName, Kind, SignatureHash, LSPIdentity, FilePathFallback). `CanonicalizeStableSymbolKey` joins with `\x00` bytes in normative field order. `StableSymbolID` returns `xxhash.Sum64String` of the canonicalized key as `semantic.SymbolID`. `BuildProviderKey` is the provider-side helper that delivers EXTRACT-02's same-content-rename invariant: for `Visibility=="exported"` it sets `FilePathFallback=""` so the ID survives renames within the package; for unexported symbols it stamps `RelPath` for disambiguation.

**Confidence ladder (`internal/semantic/extract/confidence.go`).** 5 named `float32` constants per SPEC §11.2: `ConfidenceLSPOnly=1.00`, `ConfidenceLSPMerged=0.95`, `ConfidenceTSPlusLocal=0.80`, `ConfidenceTSOnly=0.70` (Phase 59 emit value), `ConfidenceHeuristic=0.45`. NOT the §38.2 7-rung type-resolution ladder.

**Frozen vector (`TestCanonicalize_KnownVector`).** Pins the canonicalized string and xxhash64 of one full key to `0x032208087c467dcd`. Computed once and pinned; future hash drift fails CI loudly. Cross-phase reference: `helix\x00go\x00github.com/x/pkg\x00*R\x00M\x00method\x00(int)\x00\x00` → `0x032208087c467dcd`.

**Provider + Registry (`provider.go`, `registry.go`).** `Provider` is the lookup-side interface (Language, Extensions, TreeSitterLanguage, Queries, SupportsLSPEnrichment). `NewExtractorRegistry(grammars *treesitter.GrammarRegistry, providers ...Provider) *Registry` panics on nil grammar registry and on duplicate `Language()`. The daemon owns the singleton GrammarRegistry (BUG-04 invariant) and injects it; no init() registration anywhere in the package (D-02).

**Fact types (`fact.go`).** SymbolFact, ReferenceFact, ImportFact, TypeFact, HeritageFact, FileFact, SourceFile, ExtractedFile, ReceiverFact per CONTEXT.md D-01b. Closed enums: SymbolKind (11), ReferenceKind, ExtractionStatus (4), PartialReason (8), FileSemanticAvailability (5). Local Position/Range types — package has zero LSP-protocol coupling (D-01 invariant).

**Identifier types (`internal/semantic/types.go`).** Added `ImportID`, `TypeFactID`, `HeritageID` as `uint64` aliases alongside the Phase 57 family.

**ExtractionConfig (`internal/semantic/config.go` + `defaults.go`).** Four genuinely-new keys under `semantic_index.extraction.*`: `extraction_ready_timeout=30s`, `extraction_file_timeout=3s`, `max_parallel_files=4`, `allow_partial_results=true`. Per user decision, `max_file_size` and `auto_index_on_activate` STAY under `indexing.*` (already present from Phase 57). Two new tests in `loader_test.go` cover defaults and 4-layer precedence (CLI > project > user > profile).

**Extraction-outcome metric (`internal/obs/metrics.go`).** New `helix_semantic_extraction_total{language, outcome}` counter. Helper `SemanticExtractionTotal` enforces bounded-label allowlist: `language ∈ {go, typescript, python, other}` (unknown coerced to `other`), `outcome ∈ {ready, partial, unsupported, failed}` (unknown drops). Closed-cardinality bound: 4 × 4 = 16 combos. T-59-02-02 mitigation.

**ROADMAP fix.** Phase 59 SC#3 changed from "7-rung confidence ladder (1.00 LSP-confirmed → 0.20 unknown)" to "5-rung confidence ladder per SPEC §11.2 (1.00 → 0.95 → 0.80 → 0.70 → 0.45)" with explicit callout that the §38.2 7-rung ladder is Phase 62 territory.

**SPEC-DRAFT.md §25.** Added the `extraction:` sub-tree (4 keys) sibling to `indexing:`. Added an explanatory note that `max_file_size` and `auto_index_on_activate` remain under `indexing.*` per Phase 59 P02 user decision, NOT duplicated under `extraction.*`. SPEC-DRAFT.md was untracked before Phase 59; this is its first git-tracked commit. (P59 P03/04/05 will continue editing it.)

**59-RESEARCH.md Open Questions resolved.** All five questions (Q1 7-vs-5 rung, Q2 config sub-tree placement, Q3 LSPIdentity behavior, Q4 per-symbol vs. per-file partial, Q5 EXTRACT-05 source-grep scope) now carry `**RESOLVED:**` lines documenting the chosen disposition; section retitled `## Open Questions (RESOLVED)`.

## Test Surface

```
internal/semantic/extract/stable_id_test.go
  TestStableSymbolID_Deterministic            (1000-iteration determinism)
  TestCanonicalizeStableSymbolKey_FieldOrder  (9 distinct strings, 8 NULs each)
  TestCanonicalize_WhitespaceEditPreserves    (SignatureHash diff → ID diff)
  TestCanonicalize_SameContentRenamePreserves (raw hash sees FilePathFallback)
  TestCanonicalize_KnownVector                (frozen 0x032208087c467dcd)
  TestCanonicalize_NoEmptyFieldCollision      (NULs preserve field positions)
  TestBuildProviderKey_ExportedRenameStable   (EXTRACT-02 invariant)
  TestBuildProviderKey_UnexportedDisambiguates (file-path disambiguation)

internal/semantic/extract/confidence_test.go
  TestConfidenceLadder            (5 constants vs. SPEC §11.2)
  TestConfidenceLadder_NoFloatDrift (== 0.70 with no epsilon)

internal/semantic/extract/registry_test.go
  TestRegistry_NilGrammarPanics         (panic msg contains 'nil GrammarRegistry')
  TestRegistry_DuplicateLanguagePanics  (panic msg contains 'duplicate provider for language')
  TestRegistry_ProviderLookup           ((provider, true) / (_, false))

internal/obs/metrics_labels_test.go
  TestSemanticExtractionTotal_BoundedLabels  (drop-on-unknown outcome,
                                              coerce-on-unknown language)

internal/config/loader_test.go
  TestLoad_SemanticExtractionDefaults    (4 new keys + reassert indexing.* keys)
  TestLoad_SemanticExtractionPrecedence  (CLI > project > user > profile)
```

`go test ./internal/semantic/extract/ ./internal/config/ ./internal/obs/ -count=1` is green. `go vet` clean across the new surface. `go build ./...` clean.

## Confidence Ladder: 5-Rung vs. 7-Rung Reasoning

SPEC carries TWO confidence ladders, and they apply to TWO different concerns:

- **§11.2 — "Symbol/Reference Confidence" — 5 rungs (this is Phase 59's target).** 1.00 LSP-confirmed, 0.95 merged (LSP+ts agree), 0.80 ts+local-resolver, 0.70 ts-only, 0.45 heuristic. Phase 59 emits exactly `0.70` for every fact (every fact is ts-only at extraction time); Phase 61's enrichment merge raises the rungs.
- **§38.2 — "Type Resolution Confidence" — 7 rungs (Phase 62's target).** 1.00 / 0.90 / 0.80 / 0.70 / 0.60 / 0.45 / 0.20. Different concern, different lifecycle, different consumer.

ROADMAP Phase 59 success criterion #3 originally referenced the 7-rung ladder, which was a wording mistake — it conflated the two ladders. P02 corrected it to point at §11.2 with the 5 explicit rung values, and added a clarifying sentence noting §38.2 is Phase 62 territory. REQUIREMENTS EXTRACT-04 was always 5 rungs.

## ExtractionConfig: Why Some Keys Stayed Under Indexing.*

CONTEXT.md introduced 6 candidate keys for the new `extraction.*` sub-tree. Two of them (`max_file_size`, `initial_extraction_on_activation`) overlapped semantically with existing Phase 57 keys (`indexing.max_file_size`, `indexing.auto_index_on_activate`). The user's CONTEXT.md decision was:

- Keep the overlapping pair under `indexing.*` (no duplication, no migration churn).
- Put only the 4 genuinely-new keys (`extraction_ready_timeout`, `extraction_file_timeout`, `max_parallel_files`, `allow_partial_results`) under `extraction.*`.

This minimizes config-file churn for users upgrading from Phase 57 (existing keys keep working) and keeps the Phase 59 surface narrowly scoped to the four knobs that are actually new.

The `TestLoad_SemanticExtractionDefaults` test asserts BOTH the new defaults AND that the relocated keys still surface from `indexing.*` — so a future plan that "tidies" the keys into `extraction.*` would be caught.

## Threat-Model Coverage

| Threat ID | Mitigation Status | Evidence |
|-----------|-------------------|----------|
| T-59-02-01 | mitigated | `TestRegistry_NilGrammarPanics` + `TestRegistry_DuplicateLanguagePanics` exercise both panic paths |
| T-59-02-02 | mitigated | `TestSemanticExtractionTotal_BoundedLabels` exercises drop-on-unknown-outcome and coerce-on-unknown-language; `TestMetricsLabelsAllowlist` lints labels at scrape-time |
| T-59-02-03 | mitigated | `TestCanonicalizeStableSymbolKey_FieldOrder` + `TestCanonicalize_KnownVector` pin the canonicalization order and known-vector hash |
| T-59-02-04 | accepted | Confidence values are non-secret; no mitigation needed |

## Deviations from Plan

None of substance. Two minor adaptations worth noting:

1. **`go mod tidy` blocked by unrelated transitive dep issue** (a `github.com/go-openapi/testify/v2/assert/yaml` resolution failure surfacing through `sigstore-go`). Worked around by manually editing `go.mod` to promote `github.com/cespare/xxhash/v2` from indirect to direct — same end state. Not introduced by this plan; pre-existing transitive issue. Not a Rule 4 item; the build and tests are clean.
2. **SPEC-DRAFT.md was untracked in git before Phase 59.** The plan listed it under `files_modified`, but in fact it's its first commit by Phase 59 P02 (the file existed in the working tree only). Recorded under `key-files.created` rather than `key-files.modified` for correctness.

Plan executed exactly as written otherwise: TDD RED→GREEN per task, three test commits + three feat commits.

## Commits

| # | Hash | Message |
|---|------|---------|
| 1 | `ca3ccc9d` | test(59-02): add failing tests for stable-ID + confidence ladder |
| 2 | `b7f98085` | feat(59-02): implement stable-ID + confidence ladder + new identifier types |
| 3 | `22868b25` | test(59-02): add failing tests for Provider registry + extraction-outcome metric |
| 4 | `cfb73cef` | feat(59-02): Provider interface, fact types, Registry, extraction metric |
| 5 | `dab93924` | test(59-02): add failing tests for ExtractionConfig defaults + precedence |
| 6 | `db3dd4f8` | feat(59-02): ExtractionConfig + defaults + ROADMAP fix + SPEC §25 update |

## TDD Gate Compliance

Plan-level type=tdd. Per-task gate sequence verified:

- Task 1: `test(59-02)` (ca3ccc9d) RED → `feat(59-02)` (b7f98085) GREEN ✓
- Task 2: `test(59-02)` (22868b25) RED → `feat(59-02)` (cfb73cef) GREEN ✓
- Task 3: `test(59-02)` (dab93924) RED → `feat(59-02)` (db3dd4f8) GREEN ✓

No REFACTOR commits — implementations were correct on first GREEN pass; no cleanup needed.

## Self-Check: PASSED

**Files exist:**
- internal/semantic/extract/{doc,stable_id,confidence,provider,fact,registry,extract_cgo,extract_nocgo}.go ✓
- internal/semantic/extract/{stable_id,confidence,registry}_test.go ✓
- SPEC-DRAFT.md (committed) ✓

**Commits exist on branch:**
- ca3ccc9d, b7f98085, 22868b25, cfb73cef, dab93924, db3dd4f8 — all reachable from HEAD ✓

**Tests pass:**
- `go test ./internal/semantic/extract/ ./internal/config/ ./internal/obs/ -count=1` → all 3 packages OK ✓
- `go vet ./internal/semantic/extract/ ./internal/config/` → clean ✓
- `go build ./...` → clean ✓
