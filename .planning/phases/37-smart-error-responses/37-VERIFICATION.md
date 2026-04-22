---
phase: 37-smart-error-responses
verified: 2026-04-22T18:30:00Z
status: human_needed
score: 3/3
overrides_applied: 0
human_verification:
  - test: "Start Serena daemon, call a tool with a misspelled parameter (e.g., `relativ_path` instead of `relative_path`), and verify the error response includes a 'Did you mean' suggestion"
    expected: "Error response contains original error text plus a newline and 'Did you mean: relative_path (instead of relativ_path)?'"
    why_human: "End-to-end middleware chain behavior through the live daemon cannot be verified without a running server and MCP client"
  - test: "Call a tool with an invalid enum value (e.g., `incming` for a direction field) and verify the error suggests the correct value"
    expected: "Error response includes 'Did you mean: incoming (instead of incming)?'"
    why_human: "Requires live daemon with real tool schemas populated from registered tools"
---

# Phase 37: Smart Error Responses Verification Report

**Phase Goal:** Agents receive actionable parameter corrections when they misuse tools
**Verified:** 2026-04-22T18:30:00Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | When an agent passes a wrong parameter name or value, the error response includes a "did you mean" suggestion with the correct parameter | VERIFIED | `SuggestionMiddleware` in `suggest.go:223-251` intercepts both protocol errors and tool errors, enriches with Levenshtein-matched suggestions. Tests `TestSuggestMiddleware_ParamTypo` and `TestSuggestMiddleware_EnumValue` confirm behavior. |
| 2 | Error suggestions only correct parameters within the same tool -- they never redirect to a different tool | VERIFIED | `enrichProtocolError` and `enrichToolError` both look up params via `sm.tools[toolName]` where toolName comes from `extractToolName(req)`. Test `TestSuggestMiddleware_SameToolOnly` confirms cross-tool suggestion is prevented. |
| 3 | Smart error enrichment is implemented as middleware wrapping the existing typed error taxonomy, not modifying error kinds | VERIFIED | No changes to `internal/errors/` package (git diff confirms). `suggest.go` does not import `internal/errors`. Middleware appends text to error responses only. |

**Score:** 3/3 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/mcp/suggest_lev.go` | Levenshtein distance, param extraction, best-match helpers, ForTest exports | VERIFIED | 130 lines, 4 unexported functions + 4 ForTest exports, fully implemented |
| `internal/mcp/suggest.go` | ToolSchemaMap, ToolParamInfo, SuggestionMiddleware, InstallSuggestionMiddleware, BuildToolSchemaMap | VERIFIED | 257 lines, all exports present and functional |
| `internal/mcp/suggest_test.go` | Unit tests for middleware, schema map, fuzzy matching | VERIFIED | 531 lines, 12 test cases covering SERR-01/02/03, all passing |
| `internal/mcp/server.go` | CollectToolSchemas method on SerenaMCPServer | VERIFIED | `toolSchemas` field added (line 30), stored in 5 registration paths, `CollectToolSchemas()` method at line 220 |
| `internal/daemon/daemon.go` | SuggestionMiddleware installation after tool registration | VERIFIED | Lines 325-326: `BuildToolSchemaMap` + `InstallSuggestionMiddleware` called after all tool registrations (lines 228-245) and after `InstallMiddleware` (line 318) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/mcp/suggest.go` | `internal/mcp/suggest_lev.go` | function calls | WIRED | `enrichProtocolError` calls `extractBadParams` and `bestParamSuggestion`; `enrichToolError` calls `bestValueSuggestion` |
| `internal/mcp/suggest.go` | `mcpsdk.Middleware` | implements middleware signature | WIRED | `SuggestionMiddleware` returns `mcpsdk.Middleware`, installed via `server.AddReceivingMiddleware` |
| `internal/daemon/daemon.go` | `internal/mcp/suggest.go` | BuildToolSchemaMap + InstallSuggestionMiddleware calls | WIRED | Line 325: `serenaMCP.BuildToolSchemaMap(mcpServer.CollectToolSchemas())`, Line 326: `serenaMCP.InstallSuggestionMiddleware(...)` |
| `internal/mcp/server.go` | `internal/mcp/suggest.go` | CollectToolSchemas provides tools for BuildToolSchemaMap | WIRED | `toolSchemas` populated in 5 registration paths, `CollectToolSchemas()` returns the slice |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `suggest.go` ToolSchemaMap | `sm.tools` map | `BuildToolSchemaMap` parses `Tool.InputSchema` via JSON roundtrip | Yes -- extracts property names and enum values from real tool schemas | FLOWING |
| `daemon.go` schema map | `suggestionSchemaMap` | `mcpServer.CollectToolSchemas()` returns stored `[]*mcpsdk.Tool` pointers | Yes -- pointers stored at each AddTool/AddSkillTool call, SDK mutates InputSchema in place | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All suggestion tests pass | `go test ./internal/mcp/... -count=1 -run "TestSuggest\|TestLevenshtein\|TestExtractBadParams\|TestBestParam\|TestBestValue\|TestBuildToolSchema"` | 12/12 PASS (0.405s) | PASS |
| Binary compiles with middleware wired | `go build ./cmd/serena` | Exit 0 (warning only: swift scanner.c macro redefined) | PASS |
| No new error kinds introduced | `git diff HEAD~3..HEAD -- internal/errors/` | Empty diff -- no changes | PASS |
| suggest.go has no serr imports | `grep "internal/errors\|serr\." internal/mcp/suggest.go` | No matches | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SERR-01 | 37-01, 37-02 | Error responses include "did you mean" suggestions when agents misuse tool parameters | SATISFIED | `SuggestionMiddleware` enriches both protocol errors (param typos) and tool errors (enum values). Tests: ParamTypo, EnumValue, BuildToolSchemaMap. Middleware wired in daemon. |
| SERR-02 | 37-01 | Error responses suggest parameter corrections (never redirect to different tools) | SATISFIED | `enrichProtocolError` and `enrichToolError` only look up `sm.tools[toolName]` for the requesting tool. Test: SameToolOnly proves cross-tool prevention. |
| SERR-03 | 37-01, 37-02 | Error enrichment middleware wraps existing typed error taxonomy with suggestion payloads | SATISFIED | No changes to `internal/errors/`. No `serr` imports in suggest.go. Middleware appends text only. Tests: AppendOnly confirms original text preserved. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | -- | -- | -- | -- |

No TODOs, FIXMEs, placeholders, or empty implementations detected in phase files.

### Human Verification Required

### 1. End-to-End Parameter Typo Suggestion

**Test:** Start Serena daemon, connect an MCP client, and call a tool (e.g., `find_symbol`) with a misspelled parameter name (e.g., `relativ_path` instead of `relative_path`).
**Expected:** The error response contains the original error text plus `Did you mean: relative_path (instead of relativ_path)?` on a new line.
**Why human:** Requires a running daemon with all tools registered and an MCP client to send the request through the full middleware chain.

### 2. End-to-End Enum Value Suggestion

**Test:** Call a tool with an enum field (e.g., `get_call_hierarchy` with `direction: "incming"`) through a live MCP client.
**Expected:** The error response includes `Did you mean: incoming (instead of incming)?`
**Why human:** Requires live daemon to confirm real tool schemas are populated correctly via `CollectToolSchemas` and that the middleware chain processes the error in the correct order.

### Gaps Summary

No gaps found. All 3 roadmap success criteria are verified with supporting code, tests, and wiring. The implementation follows all 10 CONTEXT.md decisions (D-01 through D-10). All 3 requirements (SERR-01, SERR-02, SERR-03) are satisfied with test coverage.

The only remaining items are end-to-end human verification through a live daemon, which cannot be tested programmatically without starting the server.

---

_Verified: 2026-04-22T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
