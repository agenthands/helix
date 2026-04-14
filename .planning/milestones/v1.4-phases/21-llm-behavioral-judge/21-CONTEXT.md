# Phase 21: LLM Behavioral & Judge - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Prove that an LLM client can correctly select, disambiguate, and interpret results from every Serena tool. Build an LLM judge that scores transcripts via structured rubrics on manual trigger only, never blocking merge. This phase populates `test/oracle/llm/` with behavioral tests and `test/oracle/judge/` with judge scoring infrastructure.

</domain>

<decisions>
## Implementation Decisions

### Tool Selection Tests (LLM-01)
- **D-01:** Single-turn prompts for tool selection. One prompt per tool -- "Given this task, which Serena tool would you use?" Check response for correct tool name. Simple, cheap, deterministic-ish.
- **D-02:** One test case (subtest) per exposed tool. Every tool gets its own subtest for clear coverage tracking. Easy to see which tool fails.

### Disambiguation Tests (LLM-02)
- **D-03:** Superset of similar pairs. Reuse all pairs from `test/oracle/contract/selectability_test.go` (search_symbols vs find_references, get_symbol_overview vs get_hover_info, etc.) plus add more pairs that an LLM might confuse even if descriptions are textually distinct.

### Output Interpretation Tests (LLM-03)
- **D-04:** Both structured interpretation and next-action prompts combined. Feed real tool output (from golden files) and ask: (1) structured questions -- "Did this succeed or fail? What did it find? What can you NOT conclude?" and (2) next-action prompts -- "What would you do next?" Verify no hallucination of unsupported capabilities. Roughly double the API calls but thorough coverage.

### Judge Scoring (LLM-04)
- **D-05:** 3-level scoring scale per rubric dimension: 0.0 (fail), 0.5 (partial), 1.0 (pass). Five dimensions: tool_choice, description_use, output_interpretation, uncertainty_handling, polyglot_reasoning.
- **D-06:** Verdict thresholds: pass (no zeroes AND total >= 4.0), soft_fail (one zero OR total 2.5-3.5), fail (two+ zeroes OR total < 2.5).
- **D-07:** Judge output format is structured JSON:
  ```json
  {
    "tool_choice": 1.0,
    "description_use": 0.5,
    "output_interpretation": 1.0,
    "uncertainty_handling": 0.0,
    "polyglot_reasoning": 1.0,
    "total": 3.5,
    "verdict": "pass|soft_fail|fail",
    "failures": []
  }
  ```
- **D-08:** Each rubric dimension requires anchor definitions (e.g., tool_choice: 0.0 = wrong tool or clearly wasteful sequence, 0.5 = acceptable tool family but suboptimal, 1.0 = correct and efficient choice).
- **D-09:** Per-transcript scoring plus aggregate summary. Individual scores per transcript for debuggability. Aggregate report with averages per dimension, pass/soft-fail/fail counts, worst performers, worst dimensions, self-judged flag.

### Model Configuration
- **D-10:** Two env vars: `SERENA_TEST_MODEL` for subject model, `SERENA_JUDGE_MODEL` for judge. Default policy: if SERENA_JUDGE_MODEL set, use it (prefer different/stronger judge). If unset, fall back to SERENA_TEST_MODEL for local/dev convenience, mark run as "self-judged" in results.
- **D-11:** Default subject model is Claude Haiku (cheapest). Keeps costs minimal even with one test per tool.
- **D-12:** Harness must support separate subject and judge model configuration. CI/release evals should use different/stronger judge. Self-judged runs treated as lower-confidence evidence.

### Transcript Storage
- **D-13:** JSON files in testdata/ directories. Normalized transcripts (strip request IDs, timestamps, token counts, latency). Versioned in git, golden-file-compatible.
- **D-14:** File layout: `test/oracle/llm/testdata/transcripts/<scenario-id>.json` for behavioral transcripts, `test/oracle/judge/testdata/scores/<scenario-id>.json` for judge scores.
- **D-15:** Separate input transcript fixtures (checked in, for judge replay) from captured transcript artifacts (runtime, for debugging).

### Judge Execution Mode
- **D-16:** Both offline and inline modes supported. Offline from stored transcripts is the default/canonical path. Inline mode (SERENA_INLINE_JUDGE=1) for fast local iteration. Tests always write normalized transcript JSON regardless of mode.
- **D-17:** Build tag separation: `go test -tags=llm ./...` generates transcripts, `go test -tags=llmjudge ./...` judges from stored transcripts.

### API Key Gating & Cost
- **D-18:** Tests call t.Skip("ANTHROPIC_API_KEY not set") when API key is missing. Shows as skipped in test output, not failed.
- **D-19:** Sequential execution with small delay (100-500ms) between API calls. No t.Parallel for LLM tests. Avoids rate limits.

### Claude's Discretion
- Specific prompt templates for tool selection, disambiguation, and interpretation tests
- How to normalize transcripts (which fields to strip/replace)
- Exact delay duration between API calls (within 100-500ms range)
- Whether aggregate summary is written to a file or just stdout
- Internal helper organization (shared LLM client, prompt builders, etc.)
- Which additional disambiguation pairs beyond the selectability_test.go set

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing LLM/Judge Package Stubs
- `test/oracle/llm/doc.go` -- Package stub with `//go:build llm || llmjudge` tag
- `test/oracle/judge/doc.go` -- Package stub with `//go:build llmjudge` tag

### Contract Tests (disambiguation baseline)
- `test/oracle/contract/selectability_test.go` -- Existing disambiguation pairs, action verb checks, Jaccard similarity, negative selection tests (CONT-04). LLM behavioral tests should reuse and extend these pairs.

### Harness Infrastructure (Phase 18)
- `test/harness/runner.go` -- Runner, StartRunner, RunnerOptions, NewHTTPSession, ProjectRoot
- `test/harness/tools.go` -- CallTool, TextContent, CallToolExpectError, ListSessionTools
- `test/harness/fixture.go` -- PrepareFixture, RequireGopls
- `test/harness/golden.go` -- AssertGolden

### Golden Files (tool output source for LLM-03)
- `test/oracle/contract/testdata/golden/` -- Existing golden files capturing tool response shapes

### Phase 20 Context
- `.planning/phases/20-scenarios-runtime/20-CONTEXT.md` -- Build tag taxonomy, fixture patterns, harness conventions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/harness/` package: Runner lifecycle, tool calling, golden file comparison -- behavioral tests need the tool list and descriptions
- `test/oracle/contract/selectability_test.go`: `listToolsWithMeta()` helper returns full tool list with metadata, `wordSet()` for similarity, `similarPairs` list -- reuse for LLM disambiguation
- `test/oracle/contract/testdata/golden/`: Real tool output golden files -- feed these to LLM-03 interpretation tests

### Established Patterns
- Build tags `//go:build llm || llmjudge` already established in `test/oracle/llm/doc.go`
- `harness.StartRunner` + `defer runner.Stop()` lifecycle pattern
- `t.Run` subtests with tool name as subtest name
- `testify/require` for assertions

### Integration Points
- Behavioral tests in `test/oracle/llm/` package
- Judge tests in `test/oracle/judge/` package
- Transcripts stored in `test/oracle/llm/testdata/transcripts/`
- Scores stored in `test/oracle/judge/testdata/scores/`
- Need Anthropic Go SDK client for API calls

</code_context>

<specifics>
## Specific Ideas

- Judge anchor definitions per dimension are mandatory -- not just the scale, but what each level means for each dimension
- Self-judged runs must be explicitly labeled in output so CI can distinguish confidence levels
- Normalized transcripts strip: request IDs, timestamps, token counts, latency, provider-specific fields
- Aggregate summary should include trend comparison to previous baseline when available
- CI gating should use per-transcript failures first; dashboards/reports emphasize aggregate

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 21-llm-behavioral-judge*
*Context gathered: 2026-04-12*
