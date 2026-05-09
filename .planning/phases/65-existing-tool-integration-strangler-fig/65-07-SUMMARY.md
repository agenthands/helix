---
phase: 65-existing-tool-integration-strangler-fig
plan: 07
subsystem: health
tags: [get_health, semantic_index, integ, mcp, envelope, strangler-fig]

# Dependency graph
requires:
  - phase: 65
    plan: 03
    provides: integ.SemanticLookup interface + integSemanticLookup.Status accessor in semantic_wiring.go
  - phase: 65
    plan: 04
    provides: integ.ChooseSource priority-ladder + closed-enum FallbackReason classifier
  - phase: 57
    plan: SC-1
    provides: SemanticStoreProbe + SemanticStoreStatus envelope block (preserved verbatim)
provides:
  - get_health envelope semantic_index block (8 SPEC §24.5 fields, additive)
  - get_health envelope top-level source field (closed-enum: semantic | tree_sitter | fallback)
  - SemanticIndexAccessor interface (kernel-side seam, daemon implements)
  - daemonSemIndexAccessor adapter wrapping integ.SemanticLookup.Status
  - BuildEnvelopeJSON helper exported for cross-package envelope tests
affects:
  - 65-08 (cross-tool envelope integration tests)
  - operator dashboards consuming get_health JSON
  - INTEG-04 closed
  - INTEG-05 (get_health portion) closed

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Strangler-fig kernel-side accessor: daemon implements an interface defined kernel-side; kernel imports only integ value-types"
    - "INTEG-05 uniform envelope: every semantic-aware tool stamps top-level source + fallback_reason via integ.ChooseSource(cfgGate, lookup, nil) BEFORE any semantic call"
    - "Additive envelope evolution (Pitfall §4): new blocks added alongside existing ones; old blocks NEVER renamed"
    - "Closed-enum LastError pattern (WR-NEW-01): raw error text never reaches MCP envelope; only FallbackReason enum values"

key-files:
  created: []
  modified:
    - internal/kernel/health/tools.go
    - internal/kernel/health/tools_semantic_test.go
    - internal/daemon/daemon.go

key-decisions:
  - "Exported BuildEnvelopeJSON helper rather than testing through full MCP server: keeps tests fast + pinned to the JSON contract directly, mirrors the unit-level pattern of ComputeSemanticStoreStatus."
  - "daemonSemIndexAccessor wraps integ.SemanticLookup (not integSemanticLookup directly) so the adapter is decoupled from the bundle internals; nil-safe through normalization to NoopLookup{} at call site."
  - "mapSemanticState maps StatusDisabled → 'missing' (per SPEC §24.5 wire enum). Defensive default also 'missing' for unknown values — never panics, never leaks."
  - "Reused symbolsLookupFn() return value in daemon for both source-field stamping and the SemanticIndexAccessor — single source of truth, no double-construction of integSemanticLookup adapters."

patterns-established:
  - "Kernel package accepts interface seams ONLY; daemon owns concrete implementations. The kernel never imports internal/semantic concretions — only the integ value-types boundary (SemanticStatus, ConfigGate, FallbackReason)."
  - "Compile-time guard pattern: `var _ Interface = (*Type)(nil)` placed alongside the adapter to catch contract drift at build time (mirrors daemonCfgGate)."

requirements-completed: [INTEG-04, INTEG-05]

# Metrics
duration: ~50min
completed: 2026-05-08
---

# Phase 65 Plan 07: get_health semantic_index block + INTEG-05 source field

**Additive evolution of get_health JSON envelope: adds the eight-field SPEC §24.5 semantic_index block plus the INTEG-05 top-level closed-enum source field, while preserving the Phase 57 SC-1 semantic_store block verbatim.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-05-08T13:14:00Z (approx)
- **Completed:** 2026-05-08T14:03:40Z
- **Tasks:** RED + GREEN gates (tdd plan)
- **Files modified:** 3 (tools.go, tools_semantic_test.go, daemon.go)

## Accomplishments

- New SemanticIndexAccessor kernel-side interface; daemon implements via daemonSemIndexAccessor adapter that delegates to integ.SemanticLookup.Status (M-readtier: read-only by contract).
- New SemanticIndexBlock struct carrying all eight SPEC §24.5 fields (enabled, store, latest_snapshot_status, graph_version, overlay_active, pending_lsp_revalidations, last_live_update_ms, last_error). LastError is closed-enum FallbackReason — WR-NEW-01 holds.
- ComputeSemanticIndexBlock helper handles nil accessor (Enabled=false, omitted), accessor error (LastError=index_error closed enum, raw text logged via slog only), and success (full mapping) paths.
- mapSemanticState translates integ.Status* enum to SPEC §24.5 wire strings (ready / building / error / missing).
- BuildEnvelopeJSON exported helper renders the envelope deterministically; production handler in RegisterTools uses the same path.
- RegisterTools widened to accept (semProbe, semIndex, cfgGate, semLookup, wsKeyFn). The handler stamps top-level source + fallback_reason via integ.ChooseSource(cfgGate, lookup, nil) — the cfg-disabled → tree_sitter contract (D-04 + Pitfall §3) is honoured BEFORE any semantic call.
- semantic_store block preserved verbatim (Pitfall §4 enforced); existing TestSemanticStoreStatus_JSONShape continues to pass.
- Daemon wiring re-uses symbolsLookupFn() so health and analyze_blast_radius share a single SemanticLookup instance.

## Task Commits

This is a `type: tdd` plan; gates were committed atomically:

1. **RED: TDD tests for semantic_index block + source field** — `371c34c8` (test)
2. **GREEN: implementation + daemon wiring** — `1c7d3fa9` (feat)

REFACTOR gate skipped (no doc-comment polish needed beyond what was written in GREEN).

**Plan metadata commit:** included with this SUMMARY (final commit).

## Files Created/Modified

- `internal/kernel/health/tools.go` — Added SemanticIndexAccessor interface, SemanticIndexBlock struct, mapSemanticState helper, ComputeSemanticIndexBlock, healthEnvelope inner struct, nilIfDisabled helper, BuildEnvelopeJSON exported helper. Widened RegisterTools to accept (semProbe, semIndex, cfgGate, semLookup, wsKeyFn).
- `internal/kernel/health/tools_semantic_test.go` — Added six new tests pinning the SPEC §24.5 + INTEG-05 contracts: TestGetHealth_SC1Preserved, TestGetHealth_SemanticIndexBlock (full population), TestGetHealth_SemanticIndexBlock_Building, TestGetHealth_SemanticIndexBlock_LastErrorClosedEnum, TestComputeSemanticIndexBlock_NilAccessor, TestComputeSemanticIndexBlock_AccessorError, TestGetHealth_TopLevelSourceField (3-row table over the priority ladder).
- `internal/daemon/daemon.go` — New daemonSemIndexAccessor adapter type with compile-time guard; updated step-10 wiring to capture symbolsLookupFn() into healthLookup, build the adapter, and pass through to health.RegisterTools.

## Decisions Made

- **BuildEnvelopeJSON exported as a helper** rather than testing through a full MCP server. Keeps tests fast and unit-level, and pins the JSON contract directly. Production handler invokes the same helper.
- **Adapter delegates to a parameter, not a bundle** (`daemonSemIndexAccessor.lookup integ.SemanticLookup`). Decouples the adapter from semanticBundle internals; nil-lookup is normalized to NoopLookup{} at the daemon wiring site so the kernel always sees a non-nil delegate.
- **mapSemanticState handles Disabled and unknown identically** ("missing"). Defensive default never panics, never leaks store internals.
- **Reuse symbolsLookupFn()** so source-field stamping and Status accessor share a single SemanticLookup instance per request — no duplicated integSemanticLookup construction. Source of truth: daemon.go step 10.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Worktree base was older than expected**
- **Found during:** Task 1 RED build (initial test run failed with "no required module provides package github.com/agenthands/helix/internal/semantic/integ")
- **Issue:** The worktree HEAD was at d60e40d3 (Phase 62 work), missing Phase 65 plans 03–06. The prompt's `<worktree_branch_check>` reset script handled this — it asserted the merge-base mismatch and reset to 1f9d8ec5 (the expected base).
- **Fix:** Stashed the WIP RED test edit, ran `git reset --hard 1f9d8ec55dfe9a89dd53e24b43ef694f4f6ebfe0`, popped the stash. The file restored cleanly with only my new tests applied.
- **Files modified:** none (worktree-state-only)
- **Verification:** Re-ran RED tests; build now reports the expected "undefined: ComputeSemanticIndexBlock / BuildEnvelopeJSON" instead of "package not found".
- **Committed in:** N/A (pre-RED reset; not a commit-able change)

**2. [Rule 1 - Bug] Edit/Write tool initially wrote to main repo path, not worktree**
- **Found during:** Task 1 RED tests staging
- **Issue:** First Edit applied to `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/kernel/health/tools_semantic_test.go` (main repo) instead of the worktree path under `.claude/worktrees/agent-a32bfb77c7f28ad19/...`. The cwd-drift warning in the prompt called this out — #3099 absolute-path safety. A subsequent `git -C <main-repo>` add accidentally staged the change in the main repo.
- **Fix:** Reset the staged file in the main repo via `git reset HEAD` + `git checkout --`, then re-applied the Edit using the explicit worktree absolute path. From that point onward all Edit/Write calls used the worktree path explicitly. No commits leaked into the main repo.
- **Files modified:** none in main repo (reverted before commit)
- **Verification:** `git -C <main-repo> status --short` shows no Phase 65-07 modifications; all commits land in the worktree branch only.
- **Committed in:** N/A (pre-RED; reverted before any commit)

**3. [Rule 1 - Bug] Reword doc comment to avoid M-readtier consumer-side canary tripping**
- **Found during:** Plan verification grep checks (post-GREEN)
- **Issue:** A doc comment on the SemanticIndexAccessor interface enumerated the forbidden snapshot-write methods literally (BeginSnapshot / CommitSnapshot / AbortSnapshot / WriteSnapshotFacts / OnFlush / BumpGraphVersion). The plan's grep canary `grep -nE "(BeginSnapshot|...)" tools.go | grep -v "^[[:space:]]*//"` was supposed to skip comment lines but used a regex that did not account for grep's `<line>:<content>` output format prefix.
- **Fix:** Reworded the comment to refer to the snapshot-write surface indirectly ("the begin/commit/abort/write-facts/on-flush/version-bump methods enumerated in the daemon grep canary"). Kernel code itself never references the write methods; only the doc comment touched. M-readtier intent preserved.
- **Files modified:** internal/kernel/health/tools.go
- **Verification:** Plan canary regex (with the line-prefix-aware variant) returns zero hits.
- **Committed in:** 1c7d3fa9 (GREEN commit)

---

**Total deviations:** 3 auto-fixed (1 blocking worktree state, 1 bug-class path-resolution recovery, 1 bug-class doc-comment polish)
**Impact on plan:** All three were process / wording issues caught and corrected before any unsafe commit landed. The substantive plan (RED → GREEN gates, INTEG-04 + INTEG-05 closure, Pitfall §4 preservation) executed exactly as written.

## Issues Encountered

- **Pre-existing race in `internal/kernel/jsonrpc/codec_test.go::TestConn_Call`**: the wider `go test ./internal/kernel/... -race` run reports a race detector hit in a Phase 02-01 test. Untouched by 65-07 (not in `files_modified`). Plan's `<verification>` scope is `./internal/kernel/health/... ./internal/daemon/...` (both pass clean with -race). Logged here for visibility — out-of-scope for this plan.
- The `grep -c "\"source\":" internal/kernel/health/tools.go returns at least 1` verification step in the plan looks for a literal `"source":` substring in the source file. Go struct tags use the form `\`json:"source"\`` (no trailing colon), so the grep never matches even though the contract is satisfied. The TestGetHealth_TopLevelSourceField test pins the actual envelope JSON via a `"source": "..."` substring assertion on marshalled output (which DOES include the colon), which is the meaningful check. Surfaced as a noted artifact, not a blocker.

## TDD Gate Compliance

- **RED gate:** commit `371c34c8` (`test(65-07): add failing tests for semantic_index block (INTEG-04)`). Build failed on undefined ComputeSemanticIndexBlock + BuildEnvelopeJSON, exactly as required.
- **GREEN gate:** commit `1c7d3fa9` (`feat(65-07): add semantic_index block to get_health envelope (INTEG-04)`). All six new tests + the Phase 57 SC-1 TestSemanticStoreStatus_JSONShape pass under `go test ./internal/kernel/health/... ./internal/daemon/... -count=1 -race`.
- **REFACTOR gate:** none required; doc comments written cleanly in GREEN.

## Self-Check

- `internal/kernel/health/tools.go` — FOUND
- `internal/kernel/health/tools_semantic_test.go` — FOUND
- `internal/daemon/daemon.go` — FOUND
- Commit `371c34c8` (RED) — FOUND in branch worktree-agent-a32bfb77c7f28ad19
- Commit `1c7d3fa9` (GREEN) — FOUND in branch worktree-agent-a32bfb77c7f28ad19
- `go vet ./...` — PASSES (clean; only pre-existing CGO macro warning unrelated to this plan)
- `go test ./internal/kernel/health/... ./internal/daemon/... -count=1 -race` — PASSES
- `grep -c "SemanticIndexBlock" tools.go` — 14 (≥ 2 required)
- `grep -c 'json:"semantic_index' tools.go` — 2 (≥ 1 required)
- `grep -c 'json:"semantic_store"' tools.go` — 1 (≥ 1 required, preserved)
- M-readtier consumer canary (snapshot-write tokens in non-comment, non-test code) — clean
- TestSemanticStoreStatus_JSONShape (Phase 57 SC-1 lock) — PASSES (envelope shape preserved verbatim)

## Self-Check: PASSED

## User Setup Required

None — no external service configuration required. Operators consuming get_health JSON will see the new top-level `source` field and the new optional `semantic_index` block automatically; existing parsers that read only `workspaces` / `semantic_store` are unaffected (additive evolution; Pitfall §4 mitigation).

## Next Phase Readiness

- INTEG-04 (semantic_index block) closed.
- INTEG-05 (uniform envelope source field for all four semantic-aware tools) — closed for get_health. The remaining surface (get_repo_map / get_context / analyze_blast_radius) was already shipped in 65-05 / 65-06; with this plan, all four MCP tools now stamp source uniformly.
- 65-08 (cross-tool envelope integration tests) can build on this plan's BuildEnvelopeJSON + daemonSemIndexAccessor patterns.

---
*Phase: 65-existing-tool-integration-strangler-fig*
*Completed: 2026-05-08*
