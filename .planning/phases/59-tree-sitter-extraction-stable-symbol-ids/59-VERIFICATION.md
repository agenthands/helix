---
phase: 59-tree-sitter-extraction-stable-symbol-ids
verified: 2026-05-04T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
---

# Phase 59: Tree-sitter Extraction & Stable Symbol IDs — Verification Report

**Phase Goal:** First-class symbol/reference/import/edge extraction for Go, TypeScript+JavaScript, and Python, with a stable symbol-ID contract that survives whitespace, file rename, and exported-symbol-move — locked before any consumer depends on it.

**Verified:** 2026-05-04
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Tree-sitter extraction produces typed symbols, references, imports, and syntax edges for Go, TS+JS, and Python files; non-supported languages emit a `partial:true` extraction marker. | ✓ VERIFIED | `internal/semantic/extract/{golang,typescript,python}/provider.go` each implement `extract.Provider`; `fact.go` defines SymbolFact / ReferenceFact / ImportFact / TypeFact / HeritageFact / EdgeFact / FileFact. `partial.go:20` `PartialExtract` builds `*ExtractedFile{Partial: true, PartialReason: …}`. 135 golden testdata scenarios pass (`go test ./internal/semantic/extract/{golang,typescript,python}/... -count=1` ok). |
| 2 | The 30+ before/after test matrix per first-class language passes — overload signatures, generics, anonymous closures, decorators (Python), and method-on-receiver renames produce correct identity transitions. | ✓ VERIFIED | `find internal/semantic/extract/{golang,typescript,python}/testdata -type d -mindepth 1 -maxdepth 1 \| wc -l` = 135 (44 Go + 46 TS+JS + 45 Python — each language exceeds the 30+ floor). 135 expected.json fixtures. Per-language stable_id_test.go covers ≥10 transitions including method-on-receiver renames (Go), arrow-vs-function-decl normalization (TS), decorator add/remove (Python). All pass. |
| 3 | Tree-sitter and LSP facts merge under the documented 5-rung confidence ladder per SPEC §11.2 (1.00 → 0.95 → 0.80 → 0.70 → 0.45). | ✓ VERIFIED | `internal/semantic/extract/confidence.go:11-15` defines exactly 5 named float32 constants with the SPEC §11.2 values (`ConfidenceLSPOnly=1.00`, `ConfidenceLSPMerged=0.95`, `ConfidenceTSPlusLocal=0.80`, `ConfidenceTSOnly=0.70`, `ConfidenceHeuristic=0.45`). `TestConfidenceLadder` and `TestConfidenceLadder_NoFloatDrift` pass. ROADMAP line 65 carries the corrected "5-rung" wording with §11.2 citation. The §38.2 type-resolution ladder is correctly deferred to Phase 62. |
| 4 | Extraction reuses the single canonical `GrammarRegistry` injected from daemon bootstrap (BUG-04 invariant, regression-asserted). | ✓ VERIFIED | Dual gate: (1) **runtime** — `internal/daemon/daemon_grammar_test.go:TestBootstrap_GrammarRegistrySingleton` asserts pointer-equality across 3 consumers (extract.Registry, edit.BodyExtractor, repomap.RepoMapSkill) all hold the daemon-singleton; (2) **static** — `internal/semantic/extract/registry_grep_test.go:TestNoNewGrammarRegistry` walks `internal/semantic/extract/*` (excluding `_test.go` and `testutil/`) and fails on any `treesitter.NewGrammarRegistry` call site. Both tests pass. Per-language providers do not store the registry pointer (only language pointers), so the static gate alone covers them; the static gate would catch any future provider that constructed its own registry. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/semantic/extract/provider.go` | Provider interface | ✓ VERIFIED | `type Provider interface { Language(); Extensions(); TreeSitterLanguage(); Queries(); SupportsLSPEnrichment() }` |
| `internal/semantic/extract/fact.go` | SymbolFact, ReferenceFact, ImportFact, TypeFact, HeritageFact, EdgeFact, FileFact, ExtractedFile + closed enums | ✓ VERIFIED | All fact structs present; 8-value `PartialReason` closed enum; 4-value `ExtractionStatus` closed enum |
| `internal/semantic/extract/registry.go` | NewExtractorRegistry with dup/nil panic | ✓ VERIFIED | Used by daemon at step 6c |
| `internal/semantic/extract/stable_id.go` | StableSymbolKey + CanonicalizeStableSymbolKey + StableSymbolID + BuildProviderKey + SymbolMeta | ✓ VERIFIED | xxhash64 over NUL-joined canonicalized key; `BuildProviderKey` delivers EXTRACT-02 same-content-rename invariant |
| `internal/semantic/extract/confidence.go` | 5-rung confidence constants (float32) | ✓ VERIFIED | 5 named constants, frozen values, no drift |
| `internal/semantic/extract/partial.go` | PartialExtract helper | ✓ VERIFIED | Sets `Partial: true`, `PartialReason: …`, `ExtractionStatus` per closed-enum classification |
| `internal/semantic/extract/golang/provider.go` + queries.scm | Go provider | ✓ VERIFIED | `Language()=="go"`, embedded queries.scm; 44 testdata scenarios pass |
| `internal/semantic/extract/typescript/provider.go` + queries.scm | Shared TS+JS provider | ✓ VERIFIED | `Language()=="typescript"`, Extensions={.ts,.tsx,.js,.jsx,.mjs,.cjs}; 46 testdata scenarios pass |
| `internal/semantic/extract/python/provider.go` + queries.scm | Python provider | ✓ VERIFIED | `Language()=="python"`; 45 testdata scenarios pass |
| `internal/semantic/scheduler/scheduler.go` + state.go + ready.go + priority.go | ExtractionScheduler interface, idempotent Schedule, RequireReady gate, 4-tier priority queue | ✓ VERIFIED | All 4 files present; 18 scheduler tests pass; no `time.Sleep` in non-test code |
| `internal/semantic/store/migrations.go` (v1→v2) | applyMigration002 with 10 ALTER TABLE columns + version stamp | ✓ VERIFIED | `CurrentSchemaVersion=2`; 6 cols on semantic_files, 2 on semantic_symbols, 2 on semantic_references; migration tests pass |
| `internal/daemon/daemon.go` step 6c/6d + activate callback | Daemon constructs extract.Registry + scheduler; activate callback fires ScheduleInitialExtraction | ✓ VERIFIED | daemon.go:269-286 constructs both behind `cfg.SemanticIndex.Enabled`; daemon.go:481-489 fires `ScheduleInitialExtraction(workspace_activation, ModeAuto)` non-blockingly |
| `internal/daemon/daemon_grammar_test.go` | EXTRACT-05 runtime singleton test | ✓ VERIFIED | Test passes; 3 consumers compared (extract.Registry, BodyExtractor, RepoMapSkill) |
| `internal/daemon/daemon_extraction_test.go` | Activation tests | ✓ VERIFIED (with caveat — see Warnings) | Both tests pass; pre-merge form asserts registry construction. The post-merge wiring (commit 9f1a623f) added the SetActivateCallback line but did not extend the test to assert ScheduleInitialExtraction is actually invoked at activation. |
| `internal/semantic/extract/registry_grep_test.go` | EXTRACT-05 static source-grep test | ✓ VERIFIED | Test passes; the testutil/ subdirectory is correctly excluded |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| internal/daemon/daemon.go | extract.NewExtractorRegistry | step 6c — passes grammarRegistry singleton | ✓ WIRED | daemon.go:270-275 — `extract.NewExtractorRegistry(grammarRegistry, goextract.NewProvider(grammarRegistry), tsextract.NewProvider(grammarRegistry), pyextract.NewProvider(grammarRegistry))` |
| internal/daemon/daemon.go | scheduler.NewScheduler | step 6d — passes extractRegistry | ✓ WIRED | daemon.go:284 — `scheduler.NewScheduler(semanticExtractRegistry)` |
| internal/daemon/daemon.go | kernel.ActivateWorkspace callback | SetActivateCallback fires ScheduleInitialExtraction | ✓ WIRED | daemon.go:481-489 inside the `mcpServer.SetActivateCallback` callback closure |
| internal/semantic/scheduler/scheduler.go | internal/semantic/extract.Registry | constructor takes *extract.Registry | ✓ WIRED | scheduler.go:NewScheduler(registry *extract.Registry) |
| internal/semantic/extract/{golang,typescript,python}/provider.go | internal/treesitter (GrammarRegistry) | constructor injection — `grammars.GetLanguage(...)` | ✓ WIRED | All three providers accept `*treesitter.GrammarRegistry` and call `GetLanguage(...)`; none store it |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| extract.Registry | providers map | passed by daemon at step 6c | Yes — three live `extract.Provider` instances | ✓ FLOWING |
| scheduler.Scheduler | jobs/states/subscribers maps | populated by ScheduleInitialExtraction | Yes — but ScheduleInitialExtraction is currently a fast in-memory state transition (the actual file walk is deferred to a later phase per scheduler.go:76-77 inline comment) | ⚠️ STATIC (stub body — but no roadmap criterion in this phase requires actual file-walk execution; SC-2 is satisfied by per-provider golden tests) |
| activate callback → ScheduleInitialExtraction | semanticScheduler != nil branch | nil-check threads through cfg.SemanticIndex.Enabled gate | Yes — called with valid args (`semantic.WorkspaceID(repoPath)`, `Reason: "workspace_activation"`, `Mode: scheduler.ModeAuto`) | ✓ FLOWING |

The "STATIC" status on the scheduler body is intentional and documented — the scheduler ships the orchestration surface; the file-walk body lands when Phase 60 wires the live-update pipeline. No Phase 59 success criterion requires ScheduleInitialExtraction to actually walk files.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build helix binary with all wiring | `go build ./cmd/helix` | exit 0 | ✓ PASS |
| Vet semantic + daemon packages | `go vet ./internal/semantic/... ./internal/daemon/...` | exit 0 | ✓ PASS |
| Phase 59 schema migration tests | `go test ./internal/semantic/store/ -run TestMigration -count=1` | ok 0.890s | ✓ PASS |
| Per-language provider golden tests | `go test ./internal/semantic/extract/{golang,typescript,python}/ -count=1 -timeout 120s` | all ok | ✓ PASS |
| Scheduler tests (incl. NoTimeSleepPolling static check) | `go test ./internal/semantic/scheduler/ -count=1` | ok 3.067s (18 tests) | ✓ PASS |
| EXTRACT-05 runtime + activation tests | `go test ./internal/daemon/ -run "TestBootstrap_GrammarRegistrySingleton\|TestActivateWorkspace_" -count=1 -timeout 60s` | ok | ✓ PASS |
| EXTRACT-05 static source-grep test | `go test ./internal/semantic/extract/ -run TestNoNewGrammarRegistry -count=1` | ok | ✓ PASS |
| Determinism check (per-language goldens) | `go test ./internal/semantic/extract/golang/... -count=2` | byte-identical across runs | ✓ PASS |
| Full Phase 59 test gate | `go test ./internal/daemon/ ./internal/semantic/... ./internal/config/ ./internal/obs/ -count=1` | all ok | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| EXTRACT-01 | 59-01,02,03,04 | Tree-sitter extraction produces typed facts; non-supported emit `partial:true` | ✓ SATISFIED | 3 providers + PartialExtract helper + 135 passing testdata scenarios |
| EXTRACT-02 | 59-02,03,04 | Stable symbol IDs survive whitespace / same-content rename / exported-symbol move | ✓ SATISFIED | `BuildProviderKey` omits FilePathFallback for exported symbols; `TestBuildProviderKey_ExportedRenameStable` passes in all three providers |
| EXTRACT-03 | 59-04 | 30+ before/after matrix per first-class language | ✓ SATISFIED | 44/46/45 testdata scenarios — all exceed 30+ floor; ≥10 stable-ID transitions per language |
| EXTRACT-04 | 59-02,05 | TS+LSP facts merge under documented 5-rung ladder | ✓ SATISFIED | 5 named confidence constants with frozen values per SPEC §11.2 |
| EXTRACT-05 | 59-05 | Single canonical GrammarRegistry from daemon bootstrap | ✓ SATISFIED | Dual regression: runtime pointer-equality + static source-grep, both passing |

No orphaned requirement IDs — REQUIREMENTS.md only maps EXTRACT-01..05 to Phase 59, all five are claimed by at least one plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/daemon/daemon_grammar_test.go` | 35-44 | Stale comments ("NOT yet in HEAD" — but providers ARE in HEAD post commit 9f1a623f) | ℹ️ Info | Documentation drift, no behavioral impact. The comments correctly anticipated extending the test with provider/scheduler pointer-equality checks "at merge time"; the merge happened (9f1a623f) but the test was not extended. As noted in Truth #4 evidence, providers do not actually store the registry pointer (only language pointers), so the static source-grep gate covers the structural invariant — extending the runtime test would not catch any new failure mode. Recommend updating the comments in a future cleanup. |
| `internal/daemon/daemon_extraction_test.go` | 18-32 | Test name `TestActivateWorkspace_TriggersScheduler` promises scheduler-trigger verification but body asserts only registry construction | ⚠️ Warning | Misleading test name. The test header explicitly defers the scheduler-trigger assertion ("Post-merge ... this test gains: kernel.ActivateWorkspace fires scheduler.ScheduleInitialExtraction"). Post-merge wiring landed (daemon.go:481-489) but the assertion was never added. The wiring IS verified statically by `grep -nE "ScheduleInitialExtraction" internal/daemon/daemon.go` returning the activate-callback call site, and the in-process activate path calls `semanticScheduler.ScheduleInitialExtraction(...)` directly — there is no behavioral gap, only a missing runtime assertion. |
| `internal/semantic/extract/{golang,typescript,python}/provider.go` | doc comment | mentions `internal/repomap/queries/go_tags.scm` | ℹ️ Info | Comment-only reference, not an import. Documents the lineage / D-01 rule. `grep -nE "^import\|^\\s+\"github.*repomap" internal/semantic/extract/{golang,typescript,python}/provider.go` returns 0 actual imports. |

No blockers. The two warnings are documentation/test-completeness issues, not behavioral failures.

### Human Verification Required

(none — phase 59 ships internal infrastructure with comprehensive automated coverage)

### Gaps Summary

None. All four ROADMAP success criteria are verified with concrete evidence in the codebase. Build/vet pass; the full Phase 59 test gate (daemon + semantic/* + config + obs) is green. The EXTRACT-05 dual regression net (runtime + static) is wired and passing. 135 testdata scenarios across the three first-class languages pass deterministically across two consecutive runs. All five EXTRACT-* requirement IDs have at least one PLAN that owns them and at least one passing test that validates them.

Two minor warnings recorded under Anti-Patterns concern test-comment drift after the post-merge wiring commit (9f1a623f). Neither blocks the phase goal: the source-level wiring is correct, the static source-grep gate enforces the structural invariant, and the per-provider runtime registry lookup is verified indirectly through the dependency chain (daemon → registry → provider construction). A small follow-up could (a) update the stale "NOT yet in HEAD" comments in daemon_grammar_test.go and (b) add a runtime assertion that `scheduler.Status(ws)` transitions to `SemanticIndexing` after activation. Neither is required for goal achievement and both could ride along with Phase 60's scheduler-body work where the same wiring is exercised end-to-end.

---

_Verified: 2026-05-04_
_Verifier: Claude (gsd-verifier)_
