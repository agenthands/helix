---
phase: 59
plan: 05
subsystem: daemon
tags: [semantic, daemon, bootstrap, grammar-registry, regression-test, extract-05]
requires: [59-02]
provides:
  - Daemon bootstrap step 6c — extract.NewExtractorRegistry construction with the daemon-singleton GrammarRegistry
  - Daemon.semanticExtractRegistry + Daemon.grammarRegistry struct fields
  - EXTRACT-05 dual regression test — runtime pointer-equality + static source-grep
  - daemon_test_export_test.go test seams (SemanticExtractRegistryForTest, GrammarRegistryForTest, BodyExtractorForTest)
  - resetRepoMapSkillRegistry test helper (reflection-based reset of process-global skill state for test isolation)
affects:
  - 59-03 scheduler (merge-time wiring point — step 6d, SetActivateCallback)
  - 59-04 per-language providers (merge-time wiring point — step 6c variadic providers list)
  - Phase 60 live update pipeline (scheduler.ScheduleIncremental becomes reachable from the daemon callback path post-merge)
tech-stack:
  added: []
  patterns:
    - "Constructor-injected registry — daemon-owned singleton, no init() registration (D-02)"
    - "Reflection-based unsafe.Pointer field access for cross-package pointer-equality assertion in regression tests"
    - "Dual regression net — runtime pointer-equality + static source-grep — for invariants where neither alone suffices"
key-files:
  created:
    - internal/daemon/daemon_grammar_test.go
    - internal/daemon/daemon_extraction_test.go
    - internal/daemon/daemon_test_export_test.go
    - internal/semantic/extract/registry_grep_test.go
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/imports.go
decisions:
  - "Plan-as-written demanded daemon construct goextract.NewProvider, tsextract.NewProvider, pyextract.NewProvider (Phase 59 P04) and scheduler.NewScheduler (Phase 59 P03) inline. Those packages live in PARALLEL worktrees that have NOT yet merged into HEAD; this worktree cannot import them. Wave-2 parallel-execution context (orchestrator briefing) explicitly directed scope-down to HEAD-available types. Per-language providers and scheduler are wired AT MERGE TIME via the merge-point comments in step 6c/6d."
  - "Daemon.grammarRegistry promoted from local variable to struct field. Required for the EXTRACT-05 runtime regression test — the test compares the daemon-owned singleton against every consumer's pointer; without the field, the test has no reference pointer to compare against. Cost: +1 field on Daemon. Benefit: BUG-04 invariant becomes assertable."
  - "Test isolation: repomap skill is a process-global singleton (Caddy-style init() registration). SetRegistry is idempotent — once set, it sticks. When daemon_grammar_test.go runs after another test that constructed a daemon, the repomap skill still holds the FIRST daemon's registry, breaking pointer-equality. Fix: reflection-based resetRepoMapSkillRegistry helper that zeros s.registry/s.elider/s.extractor before each call to New(cfg). Production behavior unchanged (only one daemon per process)."
  - "The grep test in registry_grep_test.go uses a simple line-comment-strip + substring scan. Block comments are imperfectly handled (a block-comment block containing the literal substring would flag conservatively). Acceptable trade-off for a regression net — false positives are easier to fix than missed false negatives."
  - "Step 6d (scheduler construction) and the SetActivateCallback wiring for ScheduleInitialExtraction are DEFERRED to merge time, fully documented in step 6c/6d comments in daemon.go. The TestActivateWorkspace_TriggersScheduler test ships the registry-presence and grammar-pointer assertions today; the scheduler-status assertion (transitions to SemanticIndexing within 200ms) will be added when 59-03 lands."
metrics:
  tasks_completed: 3
  duration_minutes: ~35
  completed_date: 2026-05-04
  red_green_pairs: 1  # only Task 3 hit a true RED→GREEN cycle (deliberate-violation verification); Tasks 1 and 2 are paired feat/test commits
---

# Phase 59 Plan 05: Daemon Wiring + EXTRACT-05 Dual Regression Test Summary

Wire the Phase 59 extract.Registry into daemon bootstrap and ship the EXTRACT-05 / BUG-04 dual regression net (runtime pointer-equality + static source-grep). Last plan in Wave 2 by design; runs in PARALLEL with P03 (scheduler) and P04 (per-language providers) because P05's wiring depends only on the Registry/Provider interfaces from P02 (already in HEAD), not on the per-language implementations.

## What Landed

**Daemon bootstrap step 6c (`internal/daemon/daemon.go`).** New gated construction site for `*extract.Registry` after the existing step 6 grammar registry creation. When `cfg.SemanticIndex.Enabled = true`, the daemon calls `extract.NewExtractorRegistry(grammarRegistry)` with the daemon-singleton `*treesitter.GrammarRegistry`. The variadic providers list is empty in this worktree; the merge-time insertion shape is documented inline (the `goextract.NewProvider(grammarRegistry), tsextract.NewProvider(grammarRegistry), pyextract.NewProvider(grammarRegistry)` calls go on the line directly after the registry construction). Step 6d (scheduler construction) and the `SetActivateCallback` wiring for `ScheduleInitialExtraction` are similarly deferred to merge — both fully spelled out in the inline comments.

**Daemon struct extension.** Two new fields:
- `semanticExtractRegistry *extract.Registry` — nil when semantic_index is disabled; downstream consumers (future scheduler, P64+ MCP tools) MUST nil-check.
- `grammarRegistry *treesitter.GrammarRegistry` — promoted from local variable to struct field so the EXTRACT-05 regression test can compare every consumer's pointer against the daemon-owned singleton.

**imports.go invariant comment.** The non-blank-import policy for Phase 59 providers is documented permanently in `internal/daemon/imports.go` so future maintainers don't accidentally add a blank import that would trigger `init()`-style registration and risk a second `GrammarRegistry` instance (the BUG-04 violation that EXTRACT-05 is engineered to prevent).

**EXTRACT-05 runtime regression — `internal/daemon/daemon_grammar_test.go`.** `TestBootstrap_GrammarRegistrySingleton` constructs a daemon with `cfg.SemanticIndex.Enabled=true` and asserts pointer-equality of the `*treesitter.GrammarRegistry` across all HEAD-available consumers:

| Consumer | Source | Access |
|----------|--------|--------|
| Daemon-owned singleton | `daemon.go:233` | `d.GrammarRegistryForTest()` (test-only accessor) |
| Extract registry | `extract.Registry.grammars` | `Grammars()` exported method |
| Body extractor | `edit.BodyExtractor.registry` | reflection (`unsafe.Pointer` field read) |
| RepoMap skill | `repomap.RepoMapSkill.registry` | reflection (`unsafe.Pointer` field read) |

Anything but pointer-equality across these four is an EXTRACT-05 / BUG-04 regression. The test is `//go:build cgo` only — semantic extraction is undefined under CGO=0 (step 6a refusal already covers production).

**EXTRACT-05 activation hooks — `internal/daemon/daemon_extraction_test.go`.** Two narrowed activation tests:
- `TestActivateWorkspace_TriggersScheduler` — asserts the extract registry IS constructed when `semantic_index.enabled = true` and holds the daemon-singleton grammar registry. The scheduler-side assertions (status transitions, `ScheduleInitialExtraction` invocation) are deferred to merge.
- `TestActivateWorkspace_SemanticDisabledNoOp` — asserts the no-op path: when disabled, `d.semanticExtractRegistry` is nil, `d.semanticStore` is nil, but `d.grammarRegistry` is still constructed (repomap and the body extractor depend on it regardless of semantic_index).

**EXTRACT-05 source-grep regression — `internal/semantic/extract/registry_grep_test.go`.** `TestNoNewGrammarRegistry` walks `internal/semantic/extract/*` (excluding `_test.go` files and `testutil/`) and fails if any file calls `treesitter.NewGrammarRegistry`. Manually verified the failure path with a deliberate `var _deliberateViolation = treesitter.NewGrammarRegistry` injection in `registry.go` — the test produced the expected offender report; reverting restored GREEN.

**Test seams — `internal/daemon/daemon_test_export_test.go`.** `_test.go` file in package `daemon` exposing three accessors (`SemanticExtractRegistryForTest`, `GrammarRegistryForTest`, `BodyExtractorForTest`) used exclusively by the EXTRACT-05 runtime test. Production callers cannot reach these — the `_test.go` suffix gates them to test compilation.

**Reflection helpers.**
- `extractGrammarRegistryFieldByName` — reads an unexported `*treesitter.GrammarRegistry` field by name on any struct via `unsafe.Pointer` field offset. Acceptable in regression-test paths where exporting a field purely for testing would break encapsulation.
- `resetRepoMapSkillRegistry` — zeros the global repomap skill's `registry`, `elider`, and `extractor` fields between test daemon constructions. Necessary because the skill is a process-global singleton (Caddy-style `init()` registration) and `SetRegistry` is idempotent.

## Why the Dual Regression Net

EXTRACT-05 is "every consumer of `*treesitter.GrammarRegistry` holds the same pointer the daemon constructed." Either test alone is insufficient:

- **Runtime pointer-equality only:** Catches what the executed bootstrap actually wires up. Misses dead-code paths and reachable-but-untested constructors that a future PR could add. A future provider that accidentally constructs its own `GrammarRegistry` in an early-init code path that happens to never run in the daemon tests would slip through.
- **Static source-grep only:** Catches any `treesitter.NewGrammarRegistry` call site at the source-text level. Misses behavioural regressions where the call site is technically allowed (e.g., in `_test.go`) but ends up in a production code path through a misconfigured build tag or import.

Both together: any future regression trips at least one gate.

## Scheduler Wiring Deferred to Merge

Phase 59 P03 ships the `internal/semantic/scheduler/` package (`ExtractionScheduler` interface, initial-walk implementation, `RequireReady` gate, `SemanticIndexState` enum) in a parallel worktree. P04 ships per-language providers in another parallel worktree. P05 was authored expecting both Wave-2 siblings to land before its merge, but Wave 2 is parallel-by-design — P03/P04/P05 all run simultaneously.

At merge time, the following wiring lands in step 6c/6d:

```go
// Step 6c — variadic providers list filled in
semanticExtractRegistry = extract.NewExtractorRegistry(
    grammarRegistry,
    goextract.NewProvider(grammarRegistry),
    tsextract.NewProvider(grammarRegistry),
    pyextract.NewProvider(grammarRegistry),
)

// Step 6d — scheduler construction
semanticScheduler := scheduler.NewScheduler(semanticExtractRegistry, semanticStore, cfg.SemanticIndex)

// SetActivateCallback hook
mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
    rt, err := k.ActivateWorkspace(ctx, repoPath)
    if err != nil { return err }
    if semanticScheduler != nil {
        semanticScheduler.ScheduleInitialExtraction(
            semantic.WorkspaceID(rt.ID),
            scheduler.InitialExtraction{Reason: "workspace_activation", Mode: scheduler.ModeIncrementalIfPossible},
        )
    }
    // ... existing activation logic ...
})
```

Both inline comments in `daemon.go` step 6c and step 6d carry the merge-time insertion shape verbatim. The merge agent applies it deterministically without re-deriving from CONTEXT.md.

## Test Surface

```
internal/daemon/
  daemon_grammar_test.go
    TestBootstrap_GrammarRegistrySingleton  (cgo) — pointer-equality across 4 consumers
  daemon_extraction_test.go
    TestActivateWorkspace_TriggersScheduler   (cgo) — registry-construction assertion (pre-merge form)
    TestActivateWorkspace_SemanticDisabledNoOp (cgo) — disabled no-op assertion

internal/semantic/extract/
  registry_grep_test.go
    TestNoNewGrammarRegistry — walks the package, fails on offender call sites
```

`go test ./internal/daemon/ -count=1` and `go test ./internal/semantic/extract/ -count=1` are both green.

## Threat-Model Coverage

| Threat ID | Mitigation Status | Evidence |
|-----------|-------------------|----------|
| T-59-05-01 (duplicate GrammarRegistry construction) | mitigated | `TestBootstrap_GrammarRegistrySingleton` (runtime, 4 consumers) + `TestNoNewGrammarRegistry` (static, package-walk) — dual gate per CONTEXT.md D-01b acceptance #11 |
| T-59-05-02 (DoS via blocked activation) | deferred to merge | scheduler construction not in this worktree; merge-time SetActivateCallback wiring documented inline |
| T-59-05-03 (infinite extraction job per workspace) | deferred to merge | scheduler invariants land with P03 |
| T-59-05-04 (scheduler error surface) | accepted | non-PII operator telemetry surface; out of scope |

## Deviations from Plan

**Rule 3 — auto-fix blocking issue. Scope reduction: per-language provider construction and scheduler construction in daemon.go DEFERRED to merge time.** The plan as written demanded the daemon construct `goextract.NewProvider`, `tsextract.NewProvider`, `pyextract.NewProvider` (from Phase 59 P04) and `scheduler.NewScheduler` (from Phase 59 P03) inline at step 6c/6d. Those packages do not exist in HEAD or in this worktree — they are being built in parallel worktrees per Wave 2's design. The orchestrator's parallel-execution briefing was explicit: "Your wiring depends only on the Registry/Provider interfaces from 59-02 (already in HEAD)." Two viable interpretations of the conflict:

1. Apply Rule 4 (architectural decision) and STOP — return a checkpoint asking the user to either reorder waves or supply stub packages.
2. Apply Rule 3 (auto-fix blocking issue) and ship the maximum compilable subset, with merge-time integration fully spelled out in inline comments.

Path 2 was taken: the orchestrator's wave layout intentionally runs P05 in parallel with P03/P04 to maximize wave throughput; stopping for clarification would have stalled all three siblings. The merge-time wiring is documented exhaustively in the daemon.go step 6c/6d comments and in this Summary, so the merge agent does not need to re-derive from CONTEXT.md. The runtime test (Task 2) was correspondingly narrowed: provider-pointer-equality assertions and scheduler-trigger assertions land at merge; the test today asserts what IS in HEAD.

**Rule 1 — auto-fixed test-isolation issue. Repomap skill SetRegistry idempotency.** While running Task 2 RED, the runtime regression test failed with a real pointer-equality mismatch on the repomap skill's registry. Investigation: the repomap skill is a process-global singleton (Caddy-style `init()` registration in `internal/skill/repomap/skill.go:62-63`) and `SetRegistry` is idempotent — once set, subsequent calls are silent no-ops. When `TestActivateWorkspace_TriggersScheduler` runs first (alphabetical ordering), it constructs a daemon and `SetRegistry`-injects grammarRegistry-A. When `TestBootstrap_GrammarRegistrySingleton` runs second, it constructs ANOTHER daemon with grammarRegistry-B, but the global skill is stuck on grammarRegistry-A — the test sees pointer-mismatch and fails. Fix: a `resetRepoMapSkillRegistry` helper in `daemon_grammar_test.go` that uses reflection + `unsafe.Pointer` to zero `s.registry`, `s.elider`, `s.extractor` before the daemon construction. Production behavior is unchanged (only one daemon per process). Files modified: `internal/daemon/daemon_grammar_test.go` (added the helper). Tracked here as a deviation rather than a separate commit because the helper is part of the same test file the helper exists to support.

**TDD form note.** Task 2 is `tdd="true"` in the plan, but its implementation dependencies (Daemon struct fields `semanticExtractRegistry` and `grammarRegistry`) belong to Task 1 — without Task 1's wiring, Task 2's tests have nothing to assert against. The committed shape: Task 1 ships the implementation, Task 2 ships the tests. Strict RED→GREEN was only achievable on Task 3 (introduced a deliberate violation to verify the grep gate fires; reverted to confirm GREEN on clean tree).

## Commits

| # | Hash      | Message                                                                |
|---|-----------|------------------------------------------------------------------------|
| 1 | `56193a46` | feat(59-05): wire semantic extract registry into daemon bootstrap      |
| 2 | `ad394edd` | test(59-05): EXTRACT-05 runtime regression — GrammarRegistry singleton |
| 3 | `bbda9f53` | test(59-05): EXTRACT-05 source-grep regression — TestNoNewGrammarRegistry |

## Notes for Phase 60 (Live Update Pipeline)

Phase 60 fills the body of `scheduler.ScheduleIncremental` and adds the file-watcher invocation. The daemon callback path that Phase 60 needs (workspace activation → scheduler kickoff) is structurally complete in P05's design — only the actual scheduler construction call is deferred. Once P03/P04 merge, Phase 60's watcher hooks into `daemon.go` immediately after step 6d and shares the same `semanticScheduler` pointer used by `SetActivateCallback`.

## Self-Check: PASSED

**Files exist:**
- internal/daemon/daemon.go (modified) ✓
- internal/daemon/imports.go (modified) ✓
- internal/daemon/daemon_grammar_test.go (created) ✓
- internal/daemon/daemon_extraction_test.go (created) ✓
- internal/daemon/daemon_test_export_test.go (created) ✓
- internal/semantic/extract/registry_grep_test.go (created) ✓

**Commits exist on branch:**
- 56193a46, ad394edd, bbda9f53 — all reachable from HEAD ✓

**Tests pass:**
- `go test ./internal/daemon/ -run "TestBootstrap_GrammarRegistrySingleton|TestActivateWorkspace_" -count=1 -timeout 60s` → ok ✓
- `go test ./internal/semantic/extract/ -run TestNoNewGrammarRegistry -count=1` → ok ✓
- `go vet ./internal/daemon/... ./internal/semantic/extract/...` → clean ✓
- `go build ./cmd/helix` → clean ✓
