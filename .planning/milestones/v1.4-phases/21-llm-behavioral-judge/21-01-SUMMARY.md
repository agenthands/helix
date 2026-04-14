---
phase: 21-llm-behavioral-judge
plan: 01
subsystem: test/oracle/llm
tags: [llm, behavioral-testing, anthropic, tool-selection, disambiguation]
dependency_graph:
  requires: [test/harness, test/oracle/contract/selectability_test.go]
  provides: [test/oracle/llm/client.go, test/oracle/llm/prompt.go, test/oracle/llm/transcript.go, test/oracle/llm/selection_test.go, test/oracle/llm/disambiguation_test.go]
  affects: [go.mod, go.sum]
tech_stack:
  added: [anthropic-sdk-go v1.35.0]
  patterns: [build-tag-gated-tests, single-turn-prompts, transcript-normalization]
key_files:
  created:
    - test/oracle/llm/client.go
    - test/oracle/llm/prompt.go
    - test/oracle/llm/transcript.go
    - test/oracle/llm/selection_test.go
    - test/oracle/llm/disambiguation_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - "Used anthropic.Client as value type (not pointer) matching SDK NewClient return"
  - "FormatToolList accepts []*mcp.Tool to match ListTools return type"
  - "250ms inter-call delay chosen (middle of D-19 100-500ms range)"
  - "30 tool task descriptions cover all known tools with generic fallback"
  - "22 curated disambiguation tasks for 11 pairs (bidirectional)"
metrics:
  duration: ~5 min
  completed: 2026-04-12
  tasks: 2/2
  files_created: 5
  files_modified: 2
---

# Phase 21 Plan 01: LLM Test Infrastructure & Behavioral Tests Summary

Anthropic SDK integration with shared client/prompt/transcript helpers, plus tool selection and disambiguation behavioral tests covering all 30+ exposed MCP tools and 11 confusable pairs.

## What Was Built

### Shared LLM Test Helpers (client.go, prompt.go, transcript.go)

**client.go** provides the Anthropic client wrapper with model resolution per D-10/D-11/D-12:
- `SkipWithoutAPIKey` skips tests when ANTHROPIC_API_KEY is not set (D-18, T-21-01)
- `NewClient` returns an Anthropic client that auto-reads env
- `SubjectModel` / `JudgeModel` resolve model IDs from env vars with defaults
- `AskSingleTurn` sends a single-turn prompt with MaxTokens: 1024 and extracts text response
- `InterCallDelay` sleeps 250ms between API calls (D-19)

**prompt.go** provides prompt templates for selection and disambiguation:
- `SelectionSystemPrompt` / `SelectionUserPrompt` for tool selection tests
- `DisambiguationSystemPrompt` / `DisambiguationUserPrompt` for pair tests
- `FormatToolList` formats tool list from MCP session
- `ToolTaskDescription` maps 30 tools to natural-language task descriptions
- `DisambiguationTask` provides 22 curated task descriptions for 11 pairs

**transcript.go** provides transcript normalization and JSON I/O:
- `Transcript` struct per D-13 schema (scenario_id, category, model, system, user_prompt, response, stop_reason, self_judged)
- `WriteTranscript` writes indented JSON to testdata/transcripts/
- `ReadTranscript` reads transcript from file path
- `TranscriptDir` resolves absolute path via harness.ProjectRoot()

### Behavioral Tests (selection_test.go, disambiguation_test.go)

**TestSelection** (LLM-01): One subtest per exposed tool. Starts a runner with SkipLS, gets the full tool list from session, asks Claude Haiku to select the correct tool for each task description, asserts response contains the tool name. Writes transcript JSON for each test.

**TestDisambiguation** (LLM-02): 11 similar pairs tested bidirectionally (22 subtests). Includes all 7 pairs from selectability_test.go plus 4 additional LLM-confusable pairs (edit_symbol_body/replace_symbol, get_call_hierarchy/get_type_hierarchy, search_symbols/search_in_files, onboard_project/list_directory). Asserts correct target tool in response.

Both tests skip gracefully without API key and execute sequentially with inter-call delays.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | c4a7271f | Add Anthropic SDK and shared LLM test helpers |
| 2 | 956a3811 | Implement tool selection and disambiguation LLM tests |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] FormatToolList signature mismatch**
- **Found during:** Task 2 verification
- **Issue:** `FormatToolList` accepted `[]mcp.Tool` but `ListTools` returns `[]*mcp.Tool`
- **Fix:** Changed parameter type to `[]*mcp.Tool`
- **Files modified:** test/oracle/llm/prompt.go
- **Commit:** 956a3811

## Verification Results

1. `go vet -tags="llm" ./test/oracle/llm/...` -- PASSED
2. `go test -tags="llm" ./test/oracle/llm/ -count=1 -short` -- PASSED (tests skip without API key)
3. Tests show "SKIP: ANTHROPIC_API_KEY not set" -- not FAIL

## Self-Check: PASSED

All 5 created files exist. Both commits (c4a7271f, 956a3811) verified in git log.
