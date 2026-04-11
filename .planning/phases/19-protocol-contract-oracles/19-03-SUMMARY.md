---
phase: 19-protocol-contract-oracles
plan: 03
subsystem: testing
tags: [golden-files, error-contracts, contract-testing, mcp-tools]

# Dependency graph
requires:
  - phase: 18-harness-extraction-foundation
    provides: test/harness Runner, CallTool, CallToolExpectError, AssertGolden, PrepareFixture
  - phase: 19-protocol-contract-oracles
    plan: 02
    provides: schema_test.go, selectability_test.go, jsonschema/v6
provides:
  - Per-tool golden output tests for 23 MCP tools (CONT-01)
  - Error category contract tests for 3 deterministic error classes (CONT-03 partial)
  - Normalized golden files in testdata/golden/{tool_name}/success.golden
  - Error golden files in testdata/golden/errors/{category}.golden
affects: [19-protocol-contract-oracles, future Phase 20 error categories]

# Tech tracking
tech-stack:
  added: []
  patterns: [normalizeResponse with regex placeholders, filterWorkspaceLines for non-deterministic LS output, workspaceOnly flag for golden cases]

key-files:
  created:
    - test/oracle/contract/golden_test.go
    - test/oracle/contract/errors_test.go
    - test/oracle/contract/testdata/golden/ (23 tool subdirectories + errors/)
  modified: []

key-decisions:
  - "Corrected tool names from plan to match canonical names (onboard->onboard_project, prepare_session_handoff->prepare_for_new_conversation, get_implementations->find_implementations, format_document->format_code, switch_mode uses target_mode param)"
  - "Added date-time normalization for list_directory timestamps and session ID normalization for prepare_for_new_conversation"
  - "Filtered search_symbols output to workspace-local lines only due to non-deterministic stdlib results varying by Go version"
  - "Used correct 0-indexed line/column for symbol tools vs 1-indexed for get_code_actions"
  - "Targeted DemoStruct (line 15, col 5) for get_type_hierarchy and find_implementations since they require type targets"
  - "Removed list_memory_names from plan (tool does not exist in the codebase)"

patterns-established:
  - "normalizeResponse: regex-based placeholder replacement for paths, timestamps, UUIDs, session IDs, date-times"
  - "filterWorkspaceLines: strips non-deterministic stdlib results, keeping only workspace-local output"
  - "goldenCase struct with workspaceOnly flag for tools producing non-deterministic external results"
  - "Error category testing with shared runner pattern (no-workspace runner + workspace runner)"

requirements-completed: [CONT-01, CONT-03]

# Metrics
duration: 9min
completed: 2026-04-11
---

# Phase 19 Plan 03: Golden Output & Error Contract Oracles Summary

**Per-tool golden output files for 23 MCP tools plus error category contract tests for no_workspace, not_found, and invalid_args error classes**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-11T16:28:30Z
- **Completed:** 2026-04-11T16:37:08Z
- **Tasks:** 2
- **Files created:** 27 (2 test files + 23 tool golden files + 3 error golden files - 1 overlap)

## Accomplishments

- Every accessible MCP tool (23 tools) has a golden output file capturing its normalized response shape
- Golden files use stable placeholders: `<WORKSPACE>`, `<FIXTURE_ROOT>`, `<TMPDIR>`, `<TIMESTAMP>`, `<DATETIME>`, `<UUID>`, `session-<TIMESTAMP>`
- LS-dependent tools (11 tools) skip cleanly when gopls is unavailable
- 3 deterministic error categories (no_workspace, not_found, invalid_args) have golden files
- Error responses verified to contain no misleading success phrases
- Error responses verified to be deterministic across repeated calls
- Remaining error categories (timeout, circuit_open, unsupported) deferred to Phase 20 with TODO marker

## Task Commits

Each task was committed atomically:

1. **Task 1: Per-tool golden output tests (CONT-01)** - `2d252654` (feat)
2. **Task 2: Error category contract tests (CONT-03)** - `d761e9fe` (feat)

## Files Created/Modified

- `test/oracle/contract/golden_test.go` - TestGolden_ToolOutputs with 23 tool cases across 3 groups, normalizeResponse helper, filterWorkspaceLines helper
- `test/oracle/contract/errors_test.go` - TestError_CategoryContracts, TestError_NoMisleadingSuccessPayload, TestError_StableErrorStructure
- `test/oracle/contract/testdata/golden/{tool}/success.golden` - 23 golden files (one per tool)
- `test/oracle/contract/testdata/golden/errors/{category}.golden` - 3 error golden files

## Decisions Made

- Corrected 6 tool names from plan to match canonical codebase names (Rule 1 - bug fixes)
- Added short date-time and session ID normalization patterns discovered during golden generation
- Filtered search_symbols to workspace-local lines only (stdlib results non-deterministic across Go versions)
- Used DemoStruct as target for get_type_hierarchy/find_implementations (main function rejected by gopls)
- Removed list_memory_names from test (tool does not exist)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected tool names and argument schemas**
- **Found during:** Task 1
- **Issue:** Plan listed tool names that don't match the canonical MCP tool registry: list_memory_names (nonexistent), onboard (onboard_project), prepare_session_handoff (prepare_for_new_conversation), get_implementations (find_implementations), format_document (format_code), switch_mode mode param (target_mode)
- **Fix:** Discovered actual names via TestSchema_InputSchemaIsObject listing, corrected all tool names and argument keys
- **Files modified:** test/oracle/contract/golden_test.go
- **Commit:** 2d252654

**2. [Rule 1 - Bug] Fixed line/column indexing for LS tools**
- **Found during:** Task 1
- **Issue:** Plan used line 3, col 5 but symbol tools are 0-indexed; line 3 = empty line in fixture. get_code_actions is 1-indexed (different from symbol tools). get_type_hierarchy/find_implementations need type targets, not functions.
- **Fix:** Used line 4, col 5 (0-indexed) for function targets; line 15, col 5 for DemoStruct; line 5, col 5 (1-indexed) for get_code_actions
- **Files modified:** test/oracle/contract/golden_test.go
- **Commit:** 2d252654

**3. [Rule 1 - Bug] Added missing normalization patterns**
- **Found during:** Task 1 verification
- **Issue:** list_directory output contains short date-times (2026-04-11 19:33); prepare_for_new_conversation embeds session IDs with timestamps; search_symbols returns non-deterministic stdlib results
- **Fix:** Added date-time regex, session ID regex, and filterWorkspaceLines helper
- **Files modified:** test/oracle/contract/golden_test.go
- **Commit:** 2d252654

---

**Total deviations:** 3 auto-fixed (3 bugs - all tool name / argument / normalization corrections)
**Impact on plan:** All fixes align with test intent. No scope change. Tool coverage matches what actually exists in the codebase.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all golden files contain actual tool output, no placeholder data.

## Self-Check: PASSED

- All created files exist (2 test files, 23 tool golden files, 3 error golden files)
- Both task commits verified (2d252654, d761e9fe)
- go vet ./... passes
- go test -tags integration ./test/oracle/contract/... passes
