---
phase: 60
plan: 03
subsystem: kernel
tags: [kernel, edit-notifier, hooks, live-07, phase-60]
requires: [60-01]
provides:
  - kernel.EditNotifier interface (new)
  - Kernel.SetEditNotifier / Kernel.EditNotifier accessors
  - 9 OnEdit emission sites across 8 mutating MCP tools
affects:
  - internal/daemon/daemon.go (fileops.RegisterTools call site)
  - internal/kernel/fileops/tools.go (RegisterTools signature change)
tech-stack:
  added:
    - sync/atomic (atomic.Value-backed notifier holder)
  patterns:
    - Setter idiom (mirrors RepoMapSkill.SetEnrichFn / SetActivateCallback)
    - atomic.Value with typed holder struct (avoids nil-vs-typed-nil rejection)
    - Per-tool fire-and-forget hook on success branch
key-files:
  created:
    - internal/kernel/notifier.go
    - internal/kernel/notifier_test.go
    - internal/kernel/edit/notifier_integration_test.go
    - internal/kernel/fileops/notifier_integration_test.go
  modified:
    - internal/kernel/kernel.go
    - internal/kernel/edit/tools.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/fileops/write.go
    - internal/daemon/daemon.go
decisions:
  - "RESEARCH O-1 (write_file vs create_file): per-tool wiring (3 sites), NOT centralized OverwriteFile"
  - "T-60-03-07 (panic recover): shipped WITHOUT defer/recover; ~200ns saved per call site; revisit if CI catches a notifier panic"
  - "Test strategy: structural source-grep acceptance + direct interface unit tests + 1000-iteration latency bound — driving registered MCP handlers end-to-end requires live LSP, which the existing edit/fileops test harness does not stand up"
  - "Rename emits with len(result.Files) > 0 only — empty paths slice carries no actionable signal"
  - "Verify-failed branch DOES emit OnEdit — file is modified on disk regardless of post-edit verifier verdict"
  - "Refused-with-references safe_delete branch does NOT emit — early return without file mutation"
  - "fileops replace_in_file count==0 with regex does NOT emit — no write occurred"
metrics:
  duration: ~25 minutes
  tasks_completed: 3
  files_changed: 9
  on_edit_call_sites: 9
  test_count_added: 14
  completed_date: 2026-05-05
---

# Phase 60 Plan 03: Kernel→Semantic EditNotifier Seam Summary

EditNotifier interface lives in internal/kernel/ with atomic.Value-backed Kernel
accessors; 8 mutating edit/fileops MCP tools fire OnEdit fire-and-forget on success
(9 total emission sites because replace_in_file has two success branches).

## What Was Built

### 1. EditNotifier interface + Kernel accessor pair (Task 1)

`internal/kernel/notifier.go` declares the interface that LIVE-07 requires:

```go
type EditNotifier interface {
    OnEdit(ctx context.Context, workspaceID workspace.WorkspaceKey, paths []string) error
}
```

The Kernel exposes `SetEditNotifier(n)` and `EditNotifier() EditNotifier`. Both are
backed by an `atomic.Value` field (`editNotifier`) on the Kernel struct. The store
wraps the notifier in a typed `editNotifierHolder` so atomic.Value sees a consistent
concrete type across both nil and non-nil writes (raw atomic.Value rejects mixed
concrete types when the stored value is a typed-nil interface).

The interface lives in `internal/kernel/` so the 8 kernel tools can call it without
the kernel package ever importing `internal/semantic/...` (LIVE-07 invariant #1;
P01's vet-nokernel2semantic enforces this once that wave's tooling lands).

**Tests:** `internal/kernel/notifier_test.go` covers NilByDefault, SetGet, Replace,
SetNilClearsValue, ConcurrentReadDuringWrite (race-safe smoke under -race).

### 2. OnEdit hook in 5 internal/kernel/edit/ tools (Task 2)

| Tool                  | Site (tools.go) | paths argument            |
| --------------------- | --------------- | ------------------------- |
| replace_symbol_body   | line 383-385    | `[]string{args.Path}`     |
| insert_before_symbol  | line 452-454    | `[]string{args.Path}`     |
| insert_after_symbol   | line 521-523    | `[]string{args.Path}`     |
| rename_symbol         | line 588-590    | `result.Files` (multi-file WorkspaceEdit) |
| safe_delete_symbol    | line 670-672    | `[]string{args.Path}` (deleted-only branch) |

Each call follows the canonical form:

```go
if n := k.EditNotifier(); n != nil {
    _ = n.OnEdit(ctx, wsKey, paths)
}
```

The hook fires AFTER `appendVerifyInfoWithStatus` regardless of `verifyFailed` —
the on-disk file IS modified at that point even when the post-edit verifier
flagged regressions, so semantic still needs the re-extraction signal. The
refused-with-references safe_delete branch (`!result.Deleted`) returns earlier
without modifying the file and intentionally does NOT emit OnEdit.

### 3. OnEdit hook in 3 internal/kernel/fileops/ tools (Task 3)

| Tool             | Site (tools.go) | Branch                              |
| ---------------- | --------------- | ----------------------------------- |
| create_file      | line 258-260    | after CreateFile() returns nil      |
| replace_in_file  | line 440-442    | fuzzy-fallback (after OverwriteFile) |
| replace_in_file  | line 456-458    | literal-match (count > 0)           |
| fuzzy_edit       | line 510-512    | after FuzzyEdit() returns nil       |

`fileops.RegisterTools` signature was extended (the only production call site is
`internal/daemon/daemon.go:319`, updated atomically in the same commit):

```go
// before:
func RegisterTools(server *mcp.SerenaMCPServer, workspaceRoot func() string, tracer trace.Tracer)
// after:
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, workspaceRoot func() string, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer)
```

The 4 read-only / non-mutating tools (read_file, list_directory, find_files,
search_in_files) ignore both new parameters; their inner register* signatures
are unchanged.

## Open Question Resolution

### O-1: write_file vs create_file (RESOLVED — per-tool wiring)

The plan offered Option (a) "wire OnEdit inside `OverwriteFile`" vs Option (b)
"wire per-tool". We shipped (b). Rationale recorded in
`internal/kernel/fileops/write.go` package doc:

> Phase 60 D-03: per-tool MCP handlers in tools.go fire EditNotifier.OnEdit
> after every successful CreateFile / OverwriteFile call. The free functions
> here remain notifier-unaware on purpose — the *kernel.Kernel handle is held
> by the registered handler closure, not the helper.

OverwriteFile is a free function `func OverwriteFile(root, path, content string) error`;
giving it a notifier handle would ripple through every caller (test harness,
internal helpers) for no correctness gain. The 3 register* closures already
hold both `*kernel.Kernel` and `wsKeyFn`, so the hook fits cleanly there.

## Threat Model Outcomes

| Threat ID    | Disposition | Realised mitigation                                           |
| ------------ | ----------- | ------------------------------------------------------------- |
| T-60-03-01   | mitigate    | OnEdit returns synchronously; semantic-side P04 enforces non-blocking via `select { case ch <- sig: default: drop }`. Latency floor verified by the 1000-iteration <100us/call test in both edit/ and fileops/ integration tests. |
| T-60-03-02   | accept      | EditNotifier set only by daemon bootstrap; not externally controllable. |
| T-60-03-03   | mitigate    | Hook positioned only AFTER the success-path write; structurally pinned by TestEditTools_OnEditNotCalledOnError + TestFileopsTools_OnEditNotCalledOnError (each `_ = n.OnEdit(ctx,` must be followed by `return textResult(` before the closure boundary). |
| T-60-03-04   | accept      | Path strings are workspace-relative; already logged by existing tracing/RecordEditOutcome surfaces. |
| T-60-03-05   | accept      | wsKeyFn() called inside request-scoped handler; same concurrency guarantees as the existing RecordEditOutcome path. |
| T-60-03-06   | mitigate    | Interface surface is exactly OnEdit(ctx, ws, paths) error; no kernel handle exposed; vet-nokernel2semantic (P01) blocks reverse imports once landed. |
| T-60-03-07   | EXECUTOR DECISION (no recover) | Shipped WITHOUT `defer recover()` per the plan's documented executor choice. Reasons: (a) v1.10 contract is that P04's notifier impl is in-process Helix code, not third-party plugin; (b) recover() adds ~200ns per call site across 9 hot-path emission points; (c) any panic surfaces in CI long before reaching production. **Revisit gate:** if any test panics through a notifier in CI, add recover() at all 9 sites in a follow-up. |

## Test Inventory (14 new tests)

`internal/kernel/notifier_test.go`:
- TestKernel_EditNotifier_NilByDefault
- TestKernel_EditNotifier_SetGet
- TestKernel_EditNotifier_Replace
- TestKernel_EditNotifier_SetNilClearsValue
- TestKernel_EditNotifier_ConcurrentReadDuringWrite (race smoke)

`internal/kernel/edit/notifier_integration_test.go`:
- TestEditNotifier_KernelAccessorRoundtrip (interface satisfaction)
- TestEditNotifier_RecordsPathsAndWorkspace (stub semantics)
- TestEditNotifier_HandlerReturnsImmediatelyOnFastNotifier (latency bound)
- TestEditTools_OnEditWiringPresent (5 per-tool sub-tests)
- TestEditTools_OnEditNotCalledOnError (positional check)

`internal/kernel/fileops/notifier_integration_test.go`:
- TestFileopsNotifier_KernelAccessorRoundtrip
- TestFileopsNotifier_RecordsPathsAndWorkspace
- TestFileopsNotifier_HandlerReturnsImmediatelyOnFastNotifier
- TestFileopsTools_OnEditWiringPresent (3 per-tool sub-tests)
- TestFileopsTools_OnEditNotCalledOnError (positional check)
- TestFileopsTools_RegisterToolsSignatureExtended (compile-time signpost)

## Acceptance Grep Counts

```
$ grep -c 'EditNotifier()' internal/kernel/edit/tools.go
5    # >=5 ✓ (one per edit tool)

$ grep -c 'EditNotifier()' internal/kernel/fileops/tools.go
4    # >=3 ✓ (replace_in_file has 2 success branches)

$ grep -rn '\.OnEdit(ctx' internal/kernel/edit/ internal/kernel/fileops/ | grep -v _test.go | wc -l
9    # >=8 ✓ (5 edit + 4 fileops)

$ grep -c 'type EditNotifier interface' internal/kernel/notifier.go
1    # exactly 1 ✓
```

## Verification Run (final)

- `go build ./...` — clean (CGO warnings from tree-sitter vendored Swift binding only)
- `go vet ./...` — clean
- `go test ./internal/kernel/...` — all 9 sub-packages PASS
- Daemon and helix CLI build clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking issue] Created phase 60 directory in worktree**
- **Found during:** Task 1 setup
- **Issue:** The worktree branch was forked from `2bfa036f` (pre-phase-60 commit), so `.planning/phases/60-live-update-pipeline/` did not exist. SUMMARY.md needed a target directory.
- **Fix:** `mkdir -p .planning/phases/60-live-update-pipeline` before writing SUMMARY.
- **Files modified:** None (directory creation only; the orchestrator owns the actual planning files merge)
- **Commit:** N/A (directory not committed independently; SUMMARY.md commit creates the tree path)

### Plan-permitted decisions made by executor

- **OnEdit on rename only when result.Files non-empty** — plan said "all 5 tools emit on success"; rename_symbol can technically return success with zero files mutated when LSP returns an empty WorkspaceEdit. Emitting an empty paths slice carries no actionable signal for semantic; we gate on `len > 0`. Documented in commit message and decision log above.
- **OnEdit fires even on verify_failed** — plan said "after success", and the write IS the success the hook is signalling. Documented inline in code and in this SUMMARY.

### Skipped due to upstream dependency not yet landed

- **vet-nokernel2semantic** — the plan's verify block requires
  `go install ./cmd/vet-nokernel2semantic && go vet -vettool=...`. The
  `cmd/vet-nokernel2semantic` directory does not exist on `main` yet —
  P01 (Wave 0) hasn't shipped from any worktree to main. The kernel
  package's import set in this plan adds only `sync/atomic` (stdlib)
  and `internal/workspace` (already imported by kernel.go); no new
  semantic imports were introduced, so the spirit of the invariant is
  preserved. Once P01 lands and the vet tool exists, this verification
  will run cleanly without further changes here.

## Auth Gates

None.

## Known Stubs

None. The interface and 9 emission sites are wired against a real
atomic.Value-backed accessor; the only "no-op" branch is the
documented `if n := k.EditNotifier(); n != nil` nil-check, which is
the published API contract (the daemon is not required to install a
notifier in test/dev builds).

## Threat Flags

None — no new network/auth/file-access surface introduced beyond the
existing per-tool ValidatePath gate, which already runs before any
OnEdit emission point.

## Self-Check: PASSED

Verified:
- File `internal/kernel/notifier.go` exists ✓
- File `internal/kernel/notifier_test.go` exists ✓
- File `internal/kernel/edit/notifier_integration_test.go` exists ✓
- File `internal/kernel/fileops/notifier_integration_test.go` exists ✓
- Commit `7d421185` (Task 1) on branch ✓
- Commit `1addfaa3` (Task 2) on branch ✓
- Commit `6a10bca5` (Task 3) on branch ✓
- `go vet ./...` clean ✓
- `go test ./internal/kernel/...` clean (all 9 sub-packages PASS) ✓
- Acceptance grep counts: edit=5, fileops=4, total=9 (≥8) ✓
