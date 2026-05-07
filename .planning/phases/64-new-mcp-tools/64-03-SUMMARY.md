---
phase: 64
plan: 03
subsystem: semantic-skill-skeleton
tags: [skill, profile, mode-gating, mcp-tools, accessor-interfaces]
dependency_graph:
  requires:
    - phase: 64
      plan: 01
      provides: "bleve gate cleared (binary-size + indexing throughput thresholds)"
    - phase: 64
      plan: 02
      provides: "effective-graph queries on *Store (5 methods + SymbolRow type)"
  provides:
    - "internal/skill/semantic package skeleton (FINAL — wave-1/wave-2 plans never re-edit skill.go/accessors.go/envelope.go)"
    - "8 narrow accessor interfaces (StoreAccessor, SchedulerAccessor, QueueAccessor, LiveAccessor, RunnerAccessor, RetrievalAccessor, CompactorAccessor, SessionAccessor) — frozen for wave-2 consumers"
    - "Closed-enum response shapes (Freshness, IndexStatus, FreshnessMode) + ClusterStatus struct + CommonEnvelope/IndexResult/RefreshResult/StatusResult/ContextResult/ContextCandidate/ContextEvidence"
    - "modeTier enum + checkMode helper (NEW pattern for Phase 66 GuardrailMiddleware)"
    - "PermissionDenied error Kind + ErrPermissionDenied sentinel (added to internal/errors/kinds.go)"
    - "5 profile YAMLs + 4 mode YAMLs wired with 'semantic' skill identifier (D-14 symmetric matrix)"
    - "tools/list mode-tier exclusion for index_semantic_graph in read.yaml + edit.yaml (review+ tier)"
  affects:
    - phase: 64
      plan: 04
      via: "indexHelp stub var → P64-04 deletes line + adds const indexHelp in tools_index.go; consumes RunnerAccessor.Run + ResolveAuto"
    - phase: 64
      plan: 05
      via: "refreshHelp stub var → P64-05 deletes line + adds const refreshHelp in tools_refresh.go; consumes LiveAccessor.OnWorkspaceChanged + StoreAccessor.CurrentGraphVersion"
    - phase: 64
      plan: 06
      via: "statusHelp stub var → P64-06 deletes line + adds const statusHelp in tools_status.go; consumes SchedulerAccessor.IsQuiescent/ScoreStatus/ClusterStatus + StoreAccessor.LatestCommittedSnapshot + QueueAccessor.DepthAll + LiveAccessor.LastFlushAt"
    - phase: 64
      plan: 07
      via: "contextHelp stub var → P64-07 deletes line + adds const contextHelp in tools_context.go; consumes RetrievalAccessor.QueryBleve/PersonalizedPageRank/RetrievalPending/TopEdgesFor"
    - phase: 64
      plan: 08
      via: "Daemon wiring constructs adapters for all 8 accessors via Set{Store,Scheduler,Queue,Live,Runner,Retrieval,Compactor,SessionAccessor}; SessionAccessor.Workspace(ctx) closure translates *mcp.SessionInfo → workspace.WorkspaceKey"
    - phase: 66
      via: "GuardrailMiddleware will piggy-back on the PermissionDenied error envelope shape introduced by checkMode"
tech_stack:
  added:
    - "PermissionDenied error Kind (internal/errors/kinds.go)"
  patterns:
    - "Caddy-style init() registration (mirrors internal/skill/repomap)"
    - "Post-init setter dependency injection (mirrors RepoMapSkill.SetEnrichFn)"
    - "Narrow accessor interfaces (one per Phase 60-63 subsystem) — daemon owns adapter implementations"
    - "Closed-enum string types for response field values (mirrors internal/kernel/health/tools.go)"
    - "Per-handler mode-tier enforcement via checkMode (NEW pattern; Phase 66 will piggy-back)"
key_files:
  created:
    - internal/skill/semantic/skill.go
    - internal/skill/semantic/accessors.go
    - internal/skill/semantic/envelope.go
    - internal/skill/semantic/envelope_test.go
    - internal/skill/semantic/mode_check.go
    - internal/skill/semantic/mode_check_test.go
  modified:
    - internal/errors/kinds.go
    - internal/profile/profiles/full.yaml
    - internal/profile/profiles/claude-code.yaml
    - internal/profile/profiles/codex.yaml
    - internal/profile/profiles/ide-assistant.yaml
    - internal/profile/profiles/ci-bot.yaml
    - internal/profile/modes/read.yaml
    - internal/profile/modes/edit.yaml
    - internal/profile/modes/review.yaml
    - internal/profile/modes/admin.yaml
decisions:
  - "SessionAccessor exposes both Session(ctx) *mcp.SessionInfo AND Workspace(ctx) workspace.WorkspaceKey because *mcp.SessionInfo carries only a hashed WorkspaceKey *string*, not the canonical workspace.WorkspaceKey *struct*. Daemon adapter (P64-08) owns the translation between MCP session state and the workspace registry."
  - "Help-text stubs declared as `var` (not `const`) at the bottom of skill.go so wave-2 plans can land their real `const` declarations in their own tool files via single-line stub deletion + const introduction. Each plan removes a distinct constant — zero conflict surface across parallel wave-2 plans."
  - "PermissionDenied Kind added to internal/errors taxonomy (8th Kind: NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout, PermissionDenied). Phase 66 GuardrailMiddleware will reuse this error envelope shape."
  - "Mode comparisons use strings.ToLower for case-insensitive matching, matching the existing convention (e.g., switch_mode hands snap.Mode through unchanged but RecordModeTransition writes lowercase)."
  - "checkMode suggests target_mode='review' for modeTierReview (lowest-privilege tier that satisfies the requirement) and target_mode='admin' for modeTierAdmin — agents should not need to overshoot."
  - "read.yaml AND edit.yaml exclude_tools both list index_semantic_graph (D-14 + RESEARCH Open-Q #5 belt-and-braces). Handler-side checkMode is the second layer; tools/list filtering is the first."
metrics:
  start_time: "2026-05-08T00:00:00Z"
  end_time: "2026-05-08T00:00:00Z"
  duration: "single-session execution"
  task_count: 3
  files_created: 6
  files_modified: 10
  tests_added: 7
  completed_date: "2026-05-08"
---

# Phase 64 Plan 03: Semantic Skill Skeleton (FINAL W0 file-ownership endpoint)

## One-Liner

Lay the FINAL `internal/skill/semantic/` package skeleton — `skill.go` FINAL with both session helpers, `accessors.go` FINAL with all 8 narrow interfaces, `envelope.go` with closed-enum response shapes — plus the NEW per-handler mode-tier enforcement pattern (`checkMode`) and 9 YAML modifications wiring the new `semantic` skill into all 5 profiles + 4 modes.

## What Shipped

### Task 1: Skill package skeleton (skill.go FINAL + accessors.go FINAL + envelope.go)

**`internal/skill/semantic/skill.go`** — FINAL at end-of-W0. Wave-1 (64-04) and Wave-2 (64-05/06/07) plans NEVER re-edit this file except to delete a single help-text stub line each from the stub-declaration block at the bottom of the file.

Contents:
- `SemanticSkill` struct with 7 narrow accessor fields (`store`, `scheduler`, `queue`, `live`, `runner`, `retrieval`, `compactor`) + `session SessionAccessor` (closes checker W2).
- `init() { skill.Register(&SemanticSkill{}) }` — Caddy-style registration.
- `Name() string` returns `"semantic"`; `Description()` returns the 4-tool summary; `Init()` accepts `skill.SkillDeps`.
- All 8 post-init setters: `SetStore`, `SetScheduler`, `SetQueue`, `SetLive`, `SetRunner`, `SetRetrieval`, `SetCompactor`, `SetSessionAccessor`. Mirrors the `RepoMapSkill.SetEnrichFn` pattern.
- BOTH session-helper methods: `sessionSnapshot(ctx) mcp.SessionSnapshot` (closes checker W1; consumed by `checkMode`) and `workspaceKey(ctx) workspace.WorkspaceKey` (closes revision-W1; consumed by every wave-2 handler). Both return zero-value sentinels when the daemon has not wired SessionAccessor (test path).
- `Tools() []*mcp.ToolDef` returning four `*mcp.ToolDef` entries, each referencing a help-text constant (`indexHelp`, `refreshHelp`, `statusHelp`, `contextHelp`).
- `GetSemanticSkill() *SemanticSkill` — public accessor for daemon post-init wiring.
- Help-text **stubs** declared as `var` (not `const`) at the bottom of the file. Each W1/W2 tool plan deletes its single stub line + adds the real `const` declaration in its tool file. Zero conflict surface across parallel wave-2 plans.

**`internal/skill/semantic/accessors.go`** — FINAL at end-of-W0. Every interface every wave-2 plan needs is declared here and frozen.

8 narrow accessor interfaces:
- `StoreAccessor` — `LatestCommittedSnapshot` / `CurrentGraphVersion` / `OverlayHasPendingRows` / `QueryEffectiveAdjacency`. Daemon wires the adapter against `*internal/semantic/store.Store` (Plan 64-02 effective-graph methods).
- `SchedulerAccessor` — `IsQuiescent` / `ScoreStatus` / `ClusterStatus`. ClusterStatus closes checker W1: production adapter returns `{State: "unknown", Reason: "phase-62-clustering-no-status-accessor"}` until Phase 65/67 wires a live source.
- `QueueAccessor` — `DepthAll(ws)` / `LastEnqueueAt(ws)`. Bridges to Phase 61 LSPQueue.
- `LiveAccessor` — `OnWorkspaceChanged(ws, paths)` / `LastFlushAt(ws)`. Bridges to Phase 60 live update service.
- `RunnerAccessor` — `Run(ctx, ws, mode, maxMs)` / `ResolveAuto(ctx, ws)`. Closes checker B5: handlers MUST call `ResolveAuto` BEFORE `Run` so the singleflight key is keyed on the RESOLVED mode.
- `RetrievalAccessor` — `QueryBleve` / `PersonalizedPageRank` / `RetrievalPending(ws)` / `TopEdgesFor(ctx, repoID, symbolID)`. TopEdgesFor closes checker B4 (capped at 5 edges per CONTEXT.md "evidence shape" decision).
- `CompactorAccessor` — `OnFlush(ws)`. Closes checker B3: refresh handler MUST NOT invoke OnFlush; production adapter delegates to `internal/semantic/compact/compactor.go`.
- `SessionAccessor` — `Session(ctx) *mcp.SessionInfo` AND `Workspace(ctx) workspace.WorkspaceKey`. Two-method shape because `*mcp.SessionInfo` only carries a hashed WorkspaceKey *string*, not the canonical `workspace.WorkspaceKey` *struct*; daemon adapter owns the translation (closes checker W2).

**`internal/skill/semantic/envelope.go`** — closed-enum response shapes shared by all four tools:

- `Freshness` (4 closed values: `fresh` / `stale` / `structurally_fresh_semantically_pending` / `overlay_active`) per SPEC §26.2.
- `IndexStatus` (3 closed values: `committed` / `building` / `failed`) per SPEC §23.1.
- `FreshnessMode` (3 closed values: `allow_stale` / `require_current` / `validate_live`) per SPEC §23.4.
- `ClusterStatus` struct with `State` (closed-enum at value level) + `Reason` (free-form, omitempty).
- `CommonEnvelope` (Freshness, GraphVersion, OverlayActive) embedded by every tool's response struct.
- `IndexResult` (SPEC §23.1), `RefreshResult` (SPEC §23.2), `StatusResult` (SPEC §23.3), `ContextResult` (SPEC §23.4).
- `ContextCandidate` + `ContextEvidence` (TextRank, GraphRank, MatchedTerms, TopEdges capped at 5).
- `TextRank` / `GraphRank` — internal ranking primitives.

**`internal/skill/semantic/envelope_test.go`** — 3 tests, all passing:
- `TestFreshnessEnum_ClosedSet` — string equality of all 4 constants.
- `TestIndexStatusEnum_ClosedSet` — string equality of all 3 constants.
- `TestClusterStatus_DefaultUnknownShape` — JSON marshal asserts the documented W1 shape `{"state":"unknown","reason":"phase-62-clustering-no-status-accessor"}` AND the omitempty Reason regression guard for `{State:"current"}` → `{"state":"current"}`.

**Commit:** `39faebf1` — `feat(64-03): create semantic skill skeleton (skill.go FINAL + accessors.go FINAL + envelope.go)`

### Task 2: NEW mode-tier enforcement pattern (mode_check.go + 4 tests)

**`internal/skill/semantic/mode_check.go`** — establishes per-tool handler-side mode-tier enforcement. Phase 66 GuardrailMiddleware will piggy-back on the error envelope shape introduced here.

- `modeTier` int enum + 3 constants (`modeTierRead`, `modeTierReview`, `modeTierAdmin`) matching SPEC §30.2 mode-gating language.
- `checkMode(snap mcp.SessionSnapshot, required modeTier) error`:
  - Returns nil when the session's current mode satisfies the required tier.
  - Returns a structured `serr.PermissionDenied` error with `Message` = `tool requires mode <label>; current mode is <mode>` and `Detail` = `call switch_mode(target_mode=<suggested>) to elevate`.
  - Suggested elevation target is the lowest-privilege tier that satisfies the requirement: `review` for modeTierReview, `admin` for modeTierAdmin. Agents should not need to overshoot.
  - Mode comparisons use `strings.ToLower` for case-insensitive matching.

**`internal/skill/semantic/mode_check_test.go`** — 4 tests (29 sub-tests), all passing:
- `TestCheckMode_TierReadAcceptsAny` — every mode (8 mode values incl. case variations and empty string) passes modeTierRead.
- `TestCheckMode_TierReviewAcceptsReviewAndAdmin` — review/admin (case-insensitive) pass; read/edit/empty denied with PermissionDenied.
- `TestCheckMode_TierAdminOnlyAdmin` — only admin (case-insensitive) passes; read/edit/review/empty denied with PermissionDenied.
- `TestCheckMode_ErrorEnvelopeShape` — Kind=PermissionDenied (matched via `errors.Is(err, serr.ErrPermissionDenied)`), Message contains `current mode is`, Detail hints at `switch_mode`, modeTierReview suggests `target_mode="review"`, modeTierAdmin suggests `target_mode="admin"`.

**Side change:** `internal/errors/kinds.go` — added `PermissionDenied` Kind + `ErrPermissionDenied` sentinel (8th Kind in the taxonomy).

**Commit:** `06fd22a2` — `feat(64-03): add mode-tier enforcement helper (NEW pattern for Phase 66)`

### Task 3: Wire `semantic` skill into all 5 profiles + 4 modes (D-14)

D-14 symmetric matrix: every profile sees every Phase 64 tool at `tools/list`, subject to mode-tier exclusions.

**5/5 profile YAMLs** add `semantic` to skills block: `full`, `claude-code`, `codex`, `ide-assistant`, `ci-bot`.

**4/4 mode YAMLs** add `semantic` to skills block: `read`, `edit`, `review`, `admin`.

**Mode-tier exclusions for `index_semantic_graph` (review+ tier per SPEC §30.2):**
- `read.yaml`: `exclude_tools += index_semantic_graph` (D-14 + RESEARCH Open-Q #5 belt-and-braces; handler-side `checkMode` is the second layer).
- `edit.yaml`: `exclude_tools += index_semantic_graph` (edit is NOT review+; the handler also enforces, but `tools/list` filters it out for visibility).
- `review.yaml` + `admin.yaml`: NO exclusion (review+ privileged tier).

The other three Phase 64 tools (`refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`) are read+ tier and visible in every mode.

**Commit:** `add03eba` — `feat(64-03): wire semantic skill into all 5 profiles + 4 modes (D-14)`

**Cosmetic follow-up commit:** `810f4d32` — `style(64-03): gofmt mode_check.go const block alignment` (gofmt aligned trailing line comments uniformly).

## Verification

| Check | Result |
|-------|--------|
| `go vet ./internal/skill/semantic/... ./internal/profile/...` | PASS |
| `gofmt -l internal/skill/semantic/` | empty (after gofmt fix in 810f4d32) |
| `go test ./internal/skill/semantic/ -count=1` | PASS — 7 tests (3 envelope + 4 mode_check, 32 sub-tests) |
| `go test ./internal/profile/... -count=1` | PASS — existing tests unchanged |
| `go test ./internal/errors/... -count=1` | PASS — existing tests unchanged after PermissionDenied addition |
| `go build ./internal/... ./cmd/...` | PASS (only pre-existing C-warning in tree-sitter Swift bindings) |
| All 5 profile YAMLs contain `^  - semantic$` | PASS |
| All 4 mode YAMLs contain `^  - semantic$` | PASS |
| `read.yaml` + `edit.yaml` contain `^  - index_semantic_graph$` in exclude_tools | PASS |
| `review.yaml` + `admin.yaml` do NOT exclude `index_semantic_graph` | PASS |

## Acceptance Criteria (must_haves)

- [x] `internal/skill/semantic/skill.go` declares `func init() { skill.Register(&SemanticSkill{})`.
- [x] All 8 post-init setters declared (`SetStore`, `SetScheduler`, `SetQueue`, `SetLive`, `SetRunner`, `SetRetrieval`, `SetCompactor`, `SetSessionAccessor`).
- [x] BOTH session-helper methods declared (`sessionSnapshot` AND `workspaceKey`) — closes W1.
- [x] `accessors.go` declares all 8 interfaces (StoreAccessor, SchedulerAccessor with ScoreStatus + ClusterStatus, QueueAccessor, LiveAccessor, RunnerAccessor with ResolveAuto, RetrievalAccessor with TopEdgesFor, CompactorAccessor with OnFlush, SessionAccessor with Workspace) — closes B1, B3, B4, W1, W2.
- [x] `envelope.go` declares Freshness (4 values), IndexStatus (3 values), FreshnessMode (3 values), ClusterStatus struct, CommonEnvelope, IndexResult, RefreshResult, StatusResult, ContextResult.
- [x] `mode_check.go` declares modeTier enum + checkMode helper.
- [x] All 5 profile YAMLs list `semantic` in their skills block.
- [x] All 4 mode YAMLs list `semantic` in their skills block.
- [x] `read.yaml` and `edit.yaml` add `index_semantic_graph` to exclude_tools.
- [x] `review.yaml` and `admin.yaml` do NOT exclude any of the four tools.
- [x] All tests pass (7 tests / 32 sub-tests).
- [x] `go vet` passes; `gofmt -l` empty.

## Threat Model Coverage

From the plan's `<threat_model>`:

| Threat | Status | Notes |
|--------|--------|-------|
| T-64-03-01 (EoP: Mode-tier bypass via spoofed session.Mode) | **mitigate (closed)** | `checkMode` reads exclusively from `mcp.SessionSnapshot` (RLock-protected per session.go:50-71); Mode never sourced from request payload. The session.Mode field is only writable via `SessionInfo.RecordModeTransition`. |
| T-64-03-02 (Information Disclosure: Error message leaks current mode) | accept | `current mode is %q` intentionally surfaces the mode for actionable UX; no secrets implicated. |
| T-64-03-03 (DoS: tools/list filter loop unbounded) | accept | Profile YAMLs are static config; ProfileFilterMiddleware iterates a fixed-size list. |
| T-64-03-04 (Tampering: Profile YAML tampering at runtime) | accept | YAMLs are bundled at build time (embedded); existing infrastructure. |
| T-64-03-05 (EoP: SessionAccessor returns wrong session) | **mitigate (closed at P64-08)** | Daemon-injected closure pulls from the same getSession used by InstallMiddleware; no surface for caller to swap. P64-03 ships the interface; P64-08 wires the adapter. |

## Self-Check: PASSED

- File `internal/skill/semantic/skill.go` exists.
- File `internal/skill/semantic/accessors.go` exists.
- File `internal/skill/semantic/envelope.go` exists.
- File `internal/skill/semantic/envelope_test.go` exists.
- File `internal/skill/semantic/mode_check.go` exists.
- File `internal/skill/semantic/mode_check_test.go` exists.
- File `internal/errors/kinds.go` contains `PermissionDenied`.
- All 9 YAMLs (5 profiles + 4 modes) modified as specified.
- Commit `39faebf1` (Task 1) found in git log.
- Commit `06fd22a2` (Task 2) found in git log.
- Commit `add03eba` (Task 3) found in git log.
- Commit `810f4d32` (style follow-up) found in git log.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] PermissionDenied error Kind not present in taxonomy**

- **Found during:** Task 2 implementation (mode_check.go creation)
- **Issue:** The plan's mode_check.go pattern uses `serr.PermissionDenied` but the `internal/errors` package only declared 7 Kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout). Without adding PermissionDenied, mode_check.go could not compile.
- **Fix:** Added `PermissionDenied Kind = "permission_denied"` constant + `ErrPermissionDenied = &Error{Kind: PermissionDenied}` sentinel to `internal/errors/kinds.go`. Existing 7-Kind taxonomy is preserved; PermissionDenied is the 8th Kind. Phase 66 GuardrailMiddleware will reuse this Kind.
- **Files modified:** `internal/errors/kinds.go`
- **Commit:** `39faebf1` (folded into Task 1 commit because the addition is consumed by Task 2)

**2. [Plan-design refinement — additive] SessionAccessor extended with Workspace(ctx)**

- **Found during:** Task 1 implementation (workspaceKey helper authoring)
- **Issue:** The plan's accessor sketch declared `SessionAccessor` as a single-method interface (`Session(ctx) *mcp.SessionInfo`) and the `workspaceKey` helper was specified as `return sess.Workspace()`. However, `*mcp.SessionInfo` does NOT expose a `Workspace()` method — `SessionInfo.WorkspaceKey` is a hashed *string*, not the canonical `workspace.WorkspaceKey` *struct*. The plan permitted adaptation: "If the field is private, expose it via a small accessor on SessionInfo. Adapt to the actual surface found in internal/mcp/session.go; do not invent an API."
- **Fix:** Extended `SessionAccessor` to two methods: `Session(ctx) *mcp.SessionInfo` AND `Workspace(ctx) workspace.WorkspaceKey`. Daemon adapter (P64-08) will own both translations: from MCP session lookup AND from MCP session → canonical workspace.WorkspaceKey. This is the cleanest design — it does not require modifying `*mcp.SessionInfo`, keeps the accessor narrow, and lets the daemon use whatever lookup mechanism is most natural (probably consulting the workspace registry by RepoRoot).
- **Tradeoff considered:** Could have added `WorkspaceKey() workspace.WorkspaceKey` to `*mcp.SessionInfo`, but that would require also adding a structured WorkspaceKey field (currently only the hash is stored). Wider blast radius for what's a localized concern.
- **Files modified:** `internal/skill/semantic/accessors.go` + `internal/skill/semantic/skill.go` (workspaceKey helper)
- **Commit:** `39faebf1` (folded into Task 1)

**3. [Cosmetic] gofmt-driven trailing-comment alignment**

- **Found during:** Task 3 final-verification gofmt run
- **Issue:** `mode_check.go` const block had varying iota declaration widths; gofmt reflows trailing line comments to a uniform column.
- **Fix:** Ran `gofmt -w internal/skill/semantic/mode_check.go`. Cosmetic only.
- **Commit:** `810f4d32`

### Architectural Changes

None.

### Authentication Gates

None.

## Threat Flags

None — this plan ships only an interface skeleton, response-shape types, a per-handler mode-tier helper, and YAML config additions. No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries. The `PermissionDenied` Kind addition is a pure data-type addition consumed only by `checkMode` in the same plan.

## Forward Wiring Notes (for downstream plans)

**For P64-04 (W1 — index_semantic_graph handler):**
- Delete `indexHelp` line from skill.go's stub block.
- Add `const indexHelp = "..."` (real help text) at the top of the new `tools_index.go` file.
- Use `s.sessionSnapshot(ctx)` + `checkMode(snap, modeTierReview)` for mode-tier enforcement.
- Use `s.workspaceKey(ctx)` to get the canonical `workspace.WorkspaceKey`.
- Call `s.runner.ResolveAuto(ctx, ws)` BEFORE `s.runner.Run(ctx, ws, resolved, maxMs)` (closes B5).
- Return `IndexResult{...}` from `envelope.go`.

**For P64-05/06/07 (W2 — refresh/status/context handlers):**
- Same stub-deletion + const-introduction pattern (one stub line per plan, zero conflict surface).
- Use `s.sessionSnapshot(ctx)` + `checkMode(snap, modeTierRead)` for read+ tier.
- Use `s.workspaceKey(ctx)` for canonical workspace key.
- Refresh (P64-05) MUST NOT call `s.compactor.OnFlush()` (closes B3).
- Status (P64-06) builds `StatusResult` with `s.scheduler.ClusterStatus(repoID)` returning the structured ClusterStatus shape.
- Context (P64-07) builds `ContextResult` with TopEdges capped at 5 via `s.retrieval.TopEdgesFor(...)`.

**For P64-08 (daemon wiring):**
- Add `_ "github.com/agenthands/helix/internal/skill/semantic"` to `internal/daemon/imports.go`.
- Construct adapters in `internal/daemon/semantic_wiring.go` for all 8 accessors.
- Wire SessionAccessor: `s.SetSessionAccessor(adapter{getSession, workspaceLookup})` where:
  - `getSession(ctx) *mcp.SessionInfo` is the same closure passed to InstallMiddleware (currently in daemon.go).
  - `workspaceLookup(ctx) workspace.WorkspaceKey` reads the active workspace from the workspace registry (likely via repo root scoped to the session).
- Wire FOUR `register*Tools(server, semanticSkill, tracer)` calls — to be authored in P64-04/05/06/07 tool files.

---

*Plan 64-03 — Generated 2026-05-08 — Sequential executor on main working tree*
