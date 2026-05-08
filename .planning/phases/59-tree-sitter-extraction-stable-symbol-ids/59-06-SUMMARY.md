---
phase: 59
plan: "06"
subsystem: semantic-extract
tags: [semantic, extract, interface-widening, scheduler-boundary, bookkeeping, tdd, phase-65-unblock]

dependency_graph:
  requires:
    - "internal/semantic/extract.Provider interface (Phase 59 P02)"
    - "internal/semantic/extract.SourceFile + ExtractedFile (Phase 59 P02)"
    - "concrete Extract methods on golang/typescript/python providers (Phase 59 P04)"
    - "internal/semantic/scheduler.Scheduler.ScheduleInitialExtraction (Phase 59 P03)"
  provides:
    - "extract.Provider interface widened with Extract(ctx, source, file) (*ExtractedFile, error) — polymorphic dispatch"
    - "extract.LanguageMetadata sub-interface (Language / Extensions / SupportsLSPEnrichment)"
    - "extract.ExtractionPipeline sub-interface (Extract)"
    - "scheduler_static_test.go — static gate against scheduler boundary regression"
  affects:
    - "internal/daemon/semantic_wiring.go:687-742 — Phase 65 makeProductionBuildFn (downstream consumer; unblocked by D-06)"
    - "internal/semantic/extract/registry_test.go — fakeProvider gains no-op Extract to satisfy widened interface"

tech_stack:
  added: []
  patterns:
    - "Sub-interface composition (LanguageMetadata + ExtractionPipeline → Provider)"
    - "Source-grep static gate (read .go off disk + brace-walk + forbidden-token refusal)"

key_files:
  created:
    - "internal/semantic/extract/provider_polymorphic_test.go"
    - "internal/semantic/scheduler/scheduler_static_test.go"
    - ".planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-06-SUMMARY.md"
  modified:
    - "internal/semantic/extract/provider.go"
    - "internal/semantic/extract/registry_test.go"

decisions:
  - "D-06 contract widening is purely additive: concrete Extract signatures already match byte-for-byte, so the widened interface needs no concrete-source change. fakeProvider in registry_test.go is the only mock and gains a no-op Extract to satisfy the new method-set."
  - "D-07 enforcement is a static source-grep gate, not a runtime test: the scheduler body is 12 lines with no string literals containing braces, so byte-level brace-walk is sufficient and faster than go/parser. If the body ever grows complex content, the test note documents the upgrade path."
  - "D-11 bookkeeping is grep-only (no source change). REQUIREMENTS.md and ROADMAP.md were already correct as of 2026-05-08; the plan's acceptance greps confirm no drift between authoring and execution."
  - "Sub-interface carving (LanguageMetadata + ExtractionPipeline) follows the CONTEXT.md D-06 mandate verbatim. Deferred helper sub-types (ImportResolver / ScopeBuilder / SymbolNormalizer / ReferenceClassifier / QueryBundle) stay deferred — they have no concrete implementations yet."

metrics:
  duration_minutes: 9
  tasks_completed: 3
  files_created: 3
  files_modified: 2
  commits: 3
  completed: 2026-05-08

requirements_addressed: [EXTRACT-01, EXTRACT-04, EXTRACT-05]
---

# Phase 59 Plan 06: D-06 Provider.Extract interface widening + D-07 scheduler-boundary regression + D-11 bookkeeping confirmation Summary

Polymorphic `extract.Provider.Extract(ctx, source, file)` lands as a contract widening (sub-interfaces `LanguageMetadata` + `ExtractionPipeline`) so Phase 65's locked production buildFn at `internal/daemon/semantic_wiring.go:687-742` can dispatch without per-language switch; scheduler boundary protected by a static source-grep gate; REQUIREMENTS/ROADMAP bookkeeping for EXTRACT-01..05 confirmed clean.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | RED — failing polymorphic Extract interface test | `48927d2c` | `internal/semantic/extract/provider_polymorphic_test.go` |
| 2 | GREEN — promote Extract onto extract.Provider interface (D-06) | `5799abe5` | `internal/semantic/extract/provider.go`, `internal/semantic/extract/registry_test.go` |
| 3 | D-07 scheduler-state-only static gate + D-11 bookkeeping confirmation | `35f3ced0` | `internal/semantic/scheduler/scheduler_static_test.go` |

## Interface widening diff

### Before (pre-D-06)

```go
type Provider interface {
    Language() string
    Extensions() []string
    TreeSitterLanguage() *tree_sitter.Language
    Queries() string
    SupportsLSPEnrichment() bool
}
```

Method-set: `{Language, Extensions, TreeSitterLanguage, Queries, SupportsLSPEnrichment}` — 5 methods.

### After (post-D-06)

```go
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

Method-set: `{Language, Extensions, SupportsLSPEnrichment, Extract, TreeSitterLanguage, Queries}` — 6 methods. The pre-D-06 method-set is a strict subset of the post-D-06 method-set, so widening is purely additive — no consumer that compiled against the old interface can break.

### Concrete-provider invariance

No concrete provider was modified. Verification:

```
$ git diff a7c53ed5..HEAD -- internal/semantic/extract/golang/ \
                              internal/semantic/extract/typescript/ \
                              internal/semantic/extract/python/
(empty diff)
```

The three concrete `*Provider` types (goextract, tsextract, pyextract) already declared `Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error)` byte-for-byte; the widened interface absorbs the existing methods without source change.

## Test counts

### internal/semantic/extract/provider_polymorphic_test.go (NEW, package `extract_test`)

4 tests:

- `TestProvider_PolymorphicExtract_Go` — registry-lookup → polymorphic Extract → assert "Hello" function symbol present.
- `TestProvider_PolymorphicExtract_NoTypeAssertion` — `var p extract.Provider = ...` interface-typed variable proves dispatch through interface method-set.
- `TestProvider_PolymorphicExtract_AllFirstClass` — table test over `{go, typescript, python}`; each Provider responds to `.Extract` through the interface.
- `TestProvider_InterfaceShape` — runtime mirror of compile-time `var _ extract.Provider = (*goextract.Provider)(nil)` etc. anchors at the top of the file.

### internal/semantic/scheduler/scheduler_static_test.go (NEW, package `scheduler_test`)

2 tests:

- `TestScheduleInitialExtraction_BodyRemainsStateOnly` — D-07 invariant gate; reads `scheduler.go` off disk, brace-walks `ScheduleInitialExtraction`'s body, refuses forbidden tokens (`os.Open`, `os.ReadFile`, `os.ReadDir`, `os.Stat`, `filepath.Walk`, `filepath.WalkDir`, `filepath.Glob`, `ioutil.ReadFile`, `ioutil.ReadDir`, `provider.Extract`, `.Provider(`).
- `TestD07_IdempotencyAnchor` — discoverability anchor; the actual idempotency invariant is covered by the existing `TestScheduler_Idempotent` in `scheduler_test.go`.

## D-11 grep-confirmation snapshot (execution time: 2026-05-08)

```
$ grep -cE '^- \[x\] \*\*EXTRACT-0[1-5]\*\*' .planning/REQUIREMENTS.md
5
$ grep -cE '^\| EXTRACT-0[1-5] \| Phase 59 \| Complete \|' .planning/REQUIREMENTS.md
5
$ grep -F "Phase 59: Tree-sitter Extraction & Stable Symbol IDs (5/5 plans)" .planning/ROADMAP.md
- [x] Phase 59: Tree-sitter Extraction & Stable Symbol IDs (5/5 plans) (completed 2026-05-04; 2026-05-08 CONTEXT update adds D-06/D-07/D-08/D-11 — Phase 65 unblock delta, re-plan only the new tasks)
```

All three checks PASS. No drift between 2026-05-08 CONTEXT authoring and execution.

## Verification

`go vet ./...` exits 0 across the repo.

`go test ./internal/daemon/ ./internal/semantic/... ./internal/config/ ./internal/obs/ -count=1` runs clean (all packages `ok`):

```
ok      github.com/agenthands/helix/internal/daemon
ok      github.com/agenthands/helix/internal/semantic/cluster
ok      github.com/agenthands/helix/internal/semantic/compact
ok      github.com/agenthands/helix/internal/semantic/extract
ok      github.com/agenthands/helix/internal/semantic/extract/golang
ok      github.com/agenthands/helix/internal/semantic/extract/python
ok      github.com/agenthands/helix/internal/semantic/extract/typescript
ok      github.com/agenthands/helix/internal/semantic/graph
ok      github.com/agenthands/helix/internal/semantic/live
ok      github.com/agenthands/helix/internal/semantic/live/coalescer
ok      github.com/agenthands/helix/internal/semantic/live/handler
ok      github.com/agenthands/helix/internal/semantic/live/lspqueue
ok      github.com/agenthands/helix/internal/semantic/live/scanner
ok      github.com/agenthands/helix/internal/semantic/live/service
ok      github.com/agenthands/helix/internal/semantic/live/watcher
ok      github.com/agenthands/helix/internal/semantic/lspenrich
ok      github.com/agenthands/helix/internal/semantic/retrieval
ok      github.com/agenthands/helix/internal/semantic/scheduler
ok      github.com/agenthands/helix/internal/semantic/store
ok      github.com/agenthands/helix/internal/semantic/types
ok      github.com/agenthands/helix/internal/semantic/types/{golang,java,php,python,ruby,typescript}
ok      github.com/agenthands/helix/internal/config
ok      github.com/agenthands/helix/internal/obs
```

`go test ./internal/semantic/extract/{golang,typescript,python}/... -count=2` PASSES — Phase 59 P04 golden snapshots remain byte-identical across two consecutive runs.

## Acceptance criteria

- [x] **#12** — `extract.Provider` interface declares `Extract(...)` (verified by grep: comment-stripped grep for the exact method signature returns 1).
- [x] **#13** — Polymorphic call site test passes (Task 1 GREEN after Task 2; 4 tests pass under `go test ./internal/semantic/extract/...`).
- [x] **#14** — `ScheduleInitialExtraction` body remains state-only (Task 3 static gate `TestScheduleInitialExtraction_BodyRemainsStateOnly` PASSES).
- [x] **#16** — `go vet ./...` exits 0; goldens byte-identical across `-count=2` (verified inline).
- [x] **#17** — REQUIREMENTS.md `[x]` and ROADMAP.md `(5/5 plans)` confirmed (D-11 grep PASSES).

Acceptance criterion #15 (`ToStoreFacts` pure-determinism test) is owned by plan 59-07 (D-08 adapter).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Add no-op Extract to fakeProvider in registry_test.go**
- **Found during:** Task 2 GREEN (build failure).
- **Issue:** After widening `extract.Provider` with `Extract(...)`, the `fakeProvider` mock in `internal/semantic/extract/registry_test.go` no longer satisfied the interface, breaking the registry-construction tests with `*fakeProvider does not implement Provider (missing method Extract)`.
- **Fix:** Added a no-op `Extract` method that returns `(nil, nil)` — registry-construction tests never invoke it; the method exists solely to satisfy the widened interface. Added `"context"` import.
- **Files modified:** `internal/semantic/extract/registry_test.go`.
- **Commit:** `5799abe5` (folded into Task 2 GREEN, single atomic interface-widening commit).
- **Why folded:** the interface widening + the mock fix together form a single coherent atomic change; splitting them would have left an intermediate broken-test state in the history.

No other deviations. Phase 65's downstream consumer at `internal/daemon/semantic_wiring.go:687-742` is unblocked: the locked call site

```go
provider, ok := b.extractRegistry.Provider(lang)
if !ok { /* unsupported_language path */ }
extracted, err := provider.Extract(ctx, source, extract.SourceFile{Path: path, Language: lang})
```

now compiles polymorphically (verified statically by `TestProvider_PolymorphicExtract_Go` + `TestProvider_PolymorphicExtract_NoTypeAssertion`, which exercise the same shape).

## Cross-reference: plan 59-07

This plan supplies step 4 of Phase 65's six-step buildFn pipeline (polymorphic `provider.Extract(...)` dispatch). Plan **59-07** ships **D-08 `extract.ToStoreFacts(...)` adapter** — step 5 of the pipeline (accumulate `*ExtractedFile` slice → `semanticstore.Facts`). Phase 65 supplies steps 1, 2, 3, 6 plus orchestration glue.

After 59-06 + 59-07 land, the Phase 65 unblock delta is complete.

## Threat Flags

None — the contract widening introduces no new external surface, no new network endpoints, no new auth paths, no new file-access patterns. The threat register from the plan (T-59-06-01..07) was reviewed and the mitigations confirmed:

- T-59-06-01 (Tampering / interface drift) — mitigated by goldens byte-identical across `-count=2` (verified).
- T-59-06-02 (DoS / new failure modes) — mitigated by signature byte-for-byte mirror (no new error paths).
- T-59-06-03 (Tampering / scheduler boundary) — mitigated by `TestScheduleInitialExtraction_BodyRemainsStateOnly` (verified PASSES).
- T-59-06-04..07 — `accept` per plan (no new privilege/PII/audit/identity surface).

## Self-Check: PASSED

- File `internal/semantic/extract/provider_polymorphic_test.go` exists: FOUND.
- File `internal/semantic/scheduler/scheduler_static_test.go` exists: FOUND.
- File `internal/semantic/extract/provider.go` exists: FOUND.
- File `internal/semantic/extract/registry_test.go` exists: FOUND.
- Commit `48927d2c` (Task 1 RED): FOUND in `git log --all`.
- Commit `5799abe5` (Task 2 GREEN): FOUND in `git log --all`.
- Commit `35f3ced0` (Task 3 D-07/D-11): FOUND in `git log --all`.

## TDD Gate Compliance

Plan declared `type: tdd`. Gate sequence verified in `git log`:

1. RED: `48927d2c test(59-06): add failing polymorphic Extract interface test (RED)` — failing test landed first.
2. GREEN: `5799abe5 feat(59-06): promote Extract onto extract.Provider interface (D-06 GREEN)` — implementation that turns the test green.
3. (No REFACTOR commit — none needed; the GREEN diff is a minimal contract widening with no follow-up cleanup.)

Both required gates present, in correct order. Compliance: PASS.
