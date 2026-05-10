---
phase: 67
plan: "03"
subsystem: eval/trace
tags: [eval, trace, telemetry, tdd, security]
dependency_graph:
  requires: [67-01]
  provides: [trace-schema, trace-tap, trace-merge]
  affects: [67-04, 67-05, 67-06, 67-07, 67-08]
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN/REFACTOR per task
    - Closed-enum validators (KEEP-IN-SYNC with mcp/middleware.go)
    - pid-gate security pattern (T-67-04)
    - path-prefix invariant (T-67-02)
    - Defensive timestamp parsing (Pitfall-5)
key_files:
  created:
    - internal/eval/trace/schema.go
    - internal/eval/trace/schema_test.go
    - internal/eval/trace/tap.go
    - internal/eval/trace/tap_test.go
    - internal/eval/trace/merge.go
    - internal/eval/trace/merge_test.go
  modified: []
decisions:
  - "Outcome validator uses local map literal with KEEP-IN-SYNC comment to internal/mcp/middleware.go outcomeEnum; avoids circular import"
  - "CC stream-json events lack per-event timestamps; assigned time.Now() per event during parsing (Pitfall-5 accept disposition, single-host invariant)"
  - "FailureReason field added to MergedTrace for path-prefix violations; schema test updated to treat it as optional"
  - "AssistantMsg TextSummary truncated to 256 chars to prevent bloat in trace artifacts"
  - "TapDaemonLog uses 256KB scanner buffer; TapCCStream uses 4MB scanner buffer (CC events can be large)"
metrics:
  duration: "~18 minutes"
  completed: "2026-05-10"
  tasks_completed: 3
  files_created: 6
  tests_passing: 36
---

# Phase 67 Plan 03: Trace Tap and Merge Summary

Implemented the complete `internal/eval/trace` package: typed event schema, two evidence-stream parsers (daemon JSONL tap + CC stream-json tap), and the wall-clock merge function that produces the canonical `trace.json` per (task, mode) pair.

## One-liner

Typed trace schema (schema_version=1) with daemon pid-gating (T-67-04) and path-prefix invariant (T-67-02) merged from TelemetryMiddleware JSONL + CC stream-json using wall-clock alignment.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Schema - typed Event + MergedTrace structs | 31f37e67 | schema.go, schema_test.go |
| 2 | Tap - daemon JSONL + CC stream-json parsers | b772bd26 | tap.go, tap_test.go |
| 3 | Merge - wall-clock sort + MergedTrace assembly | d48d479d | merge.go, merge_test.go |

## Schema Fields

### Final outcome enum (schema.go `validOutcomes`)

Closed enum sourced from `internal/mcp/middleware.go outcomeEnum` (Phase 66 extended):
- `success`, `invalid_args`, `not_found`, `circuit_open`, `ls_crash`, `timeout`, `internal`
- `guardrail_warned`, `guardrail_blocked` (Phase 66 additions)

Pattern: local `map[string]struct{}` literal with `// KEEP-IN-SYNC with internal/mcp/middleware.go outcomeEnum` comment. Circular import avoided by not importing the `mcp` package from `eval/trace`.

### MergedTrace top-level fields

All required fields per RESEARCH §"Merged trace.json Shape":
`schema_version`, `task_id`, `mode`, `run_id`, `claude_version`, `helix_version`, `started_at`, `ended_at`, `duration_ms`, `outcome`, `events`, `usage`, `tool_call_summary`, `guardrails`

Plus: `failure_reason` (optional, set when `outcome="failed"` due to T-67-02 path-prefix violation).

## Security Mitigations

| Threat | Mitigation | Asserting Test |
|--------|-----------|----------------|
| T-67-02: patch paths outside repo | `Merge()` checks `filepath.Rel(RepoRoot, path)` starts with `..`; returns error + Outcome="failed" + FailureReason="patch_outside_repo" | `TestMergePathPrefixInvariant/path_outside_repo_errors` |
| T-67-04: foreign-pid JSONL injection | `TapDaemonLog` gates on `pid==expectedPid`; increments `RejectedForeignPid` counter | `TestTapDaemonLog_PidGate` |
| T-67-Pitfall-3: CC version drift | Unknown `type` lines counted in `CCTapResult.UnknownTypes`, not fatal | `TestTapCCStream_UnknownTypeWarns` |
| T-67-Pitfall-5: wall-clock drift | Single-host invariant documented in merge.go; timestamp parse falls back to `time.Now()` | `TestTapDaemonLog_TimestampParsing` |

## Decisions Made

1. **Outcome enum pattern — KEEP-IN-SYNC comment**: chose local map literal over cross-package import to avoid circular dependency (`eval/trace` importing `mcp`). Comment at `schema.go:validOutcomes` instructs future developers to sync when `mcp/middleware.go outcomeEnum` changes.

2. **CC stream-json lacks per-event timestamps**: CC stream-json envelope does not carry a `time` field per line (unlike daemon JSONL which has `"time"`). Events parsed from CC streams receive `time.Now().UTC()` as their timestamp. This is documented as the Pitfall-5 accept disposition. The merge sort by wall-clock still works because daemon events carry real timestamps and dominate the ordering.

3. **`FailureReason` field added to `MergedTrace`**: The plan action specified "add `FailureReason` if not present"; it was not in the initial schema skeleton, so it was added as `json:"failure_reason,omitempty"`. The schema test `TestMergedTraceShapeMatchesResearch` does NOT require this key (it's optional), matching the plan's instruction to "update the schema test to keep it optional."

4. **TextSummary truncation at 256 chars**: assistant message text content is truncated to avoid bloat in trace artifacts. A `...` suffix marks truncated summaries.

5. **Scanner buffer sizes**: `TapDaemonLog` uses 256KB (daemon lines are compact JSONL); `TapCCStream` uses 4MB (CC assistant messages with large tool inputs can exceed 64KB default).

## Test Coverage

- `schema_test.go`: 4 tests (JSON round-trip × 6 kinds, shape keys, outcome enum, event-kind enum)
- `tap_test.go`: 12 tests (5 daemon tap + 6 CC tap + 1 timestamp parsing)
- `merge_test.go`: 10 tests (wall-clock sort, tie-break, tool-call summary, guardrail counts, usage copy, budget breach, exit code, path-prefix × 2, schema-version, duration-ms)

Total: 36 test functions/subtests, all passing, race-clean.

## Deviations from Plan

### Auto-added functionality

**1. [Rule 2 - Missing] TestTapDaemonLog_TimestampParsing**
- Added beyond the 11 spec tests to explicitly assert RFC3339Nano timestamp parsing
- Ensures the defensive fallback path doesn't silently corrupt timestamps
- Files modified: tap_test.go

**2. [Rule 2 - Missing] TestMergeSchemaVersion and TestMergeDurationMs**
- Added beyond the 8 spec merge tests to assert invariants explicitly
- Plan specified 8 tests; these 2 additional tests strengthen correctness
- Files modified: merge_test.go

None — plan executed as written with minor test additions for completeness.

## Known Stubs

None. All exported functions are fully implemented.

## Threat Flags

None. The files created stay within `internal/eval/trace` and introduce no new network endpoints or trust boundaries beyond those documented in the plan's threat model.

## Self-Check: PASSED

Files verified:
- `internal/eval/trace/schema.go` — EXISTS
- `internal/eval/trace/schema_test.go` — EXISTS
- `internal/eval/trace/tap.go` — EXISTS
- `internal/eval/trace/tap_test.go` — EXISTS
- `internal/eval/trace/merge.go` — EXISTS
- `internal/eval/trace/merge_test.go` — EXISTS

Commits verified:
- 31f37e67 (schema)
- b772bd26 (tap)
- d48d479d (merge)

`go test ./internal/eval/trace/... -count=1 -race` — PASS
`go vet ./internal/eval/trace/...` — PASS
