# Phase 21: LLM Behavioral & Judge - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 21-llm-behavioral-judge
**Areas discussed:** Tool selection test design, Judge rubrics & scoring, Transcript capture & storage, API key gating & cost

---

## Tool Selection Test Design

| Option | Description | Selected |
|--------|-------------|----------|
| Single-turn prompts | One prompt per tool, check response for correct tool name. Simple, cheap. | ✓ |
| Multi-turn with tool_use API | Use Claude's tool_use to present all tools, give task, check which tool called. More realistic but costlier. | |
| You decide | Claude picks the approach. | |

**User's choice:** Single-turn prompts
**Notes:** Recommended approach. Covers LLM-01 and LLM-02.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Feed real tool output, ask structured questions | "Did this succeed or fail? What did it find? What can you NOT conclude?" | |
| Feed tool output, ask for next action | Verify no hallucination of unsupported capabilities. | |
| Both approaches combined | Structured + next-action. More thorough, roughly double API calls. | ✓ |

**User's choice:** Both approaches combined
**Notes:** Thorough coverage of LLM-03 with both structured interpretation and hallucination detection.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse pairs from selectability_test.go | Same pairs tested via LLM prompt instead of Jaccard heuristic. | |
| Superset -- reuse + add more | Reuse existing pairs plus additional LLM-confusable pairs. | ✓ |
| You decide | Claude picks based on actual tool list analysis. | |

**User's choice:** Superset -- reuse + add more
**Notes:** More coverage by extending beyond heuristic-tested pairs.

---

| Option | Description | Selected |
|--------|-------------|----------|
| One test case per tool | Every tool gets its own subtest. Clear coverage tracking. | ✓ |
| Grouped by category | One prompt per category, multiple tasks. Fewer API calls. | |
| You decide | Claude optimizes for cost vs coverage. | |

**User's choice:** One test case per tool
**Notes:** Clear coverage tracking, easy to pinpoint failures.

---

## Judge Rubrics & Scoring

| Option | Description | Selected |
|--------|-------------|----------|
| Binary pass/fail per dimension | 0 or 1 per dimension. Simple. | |
| 1-5 Likert scale per dimension | Allows nuance. Needs anchor descriptions. | |
| 3-level (fail/partial/pass) | 0.0, 0.5, or 1.0 per dimension. Middle ground. | ✓ |
| You decide | Claude picks what works best. | |

**User's choice:** 3-level (fail/partial/pass)
**Notes:** User provided detailed specification: encoding (0.0/0.5/1.0), JSON output format, verdict thresholds (pass: no zeroes + total >= 4.0, soft_fail: one zero or total 2.5-3.5, fail: two+ zeroes or total < 2.5), and anchor definition requirements per dimension.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Same model judges itself | Simpler setup, one API key. Risk: self-evaluation bias. | |
| Different model as judge | Avoids self-evaluation bias. E.g., Sonnet judges Haiku. | |
| Configurable via env var | SERENA_TEST_MODEL + SERENA_JUDGE_MODEL. Defaults configurable. | ✓ |

**User's choice:** Configurable via env var, with default of different model
**Notes:** Two env vars. If SERENA_JUDGE_MODEL unset, falls back to SERENA_TEST_MODEL but marks run as "self-judged". CI/release should use different judge. Referenced OpenAI eval best practices on self-evaluation bias.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Per-transcript scoring | Individual rubric scores per transcript. Fine-grained. | |
| Per-transcript + aggregate summary | Individual scores plus summary report with averages, worst performers. | ✓ |
| You decide | Claude picks the approach. | |

**User's choice:** Per-transcript + aggregate summary
**Notes:** Per-transcript for debuggability, aggregate for CI signal. CI gating uses per-transcript failures first, dashboards emphasize aggregate. Referenced OpenAI and LangSmith eval guidance.

---

## Transcript Capture & Storage

| Option | Description | Selected |
|--------|-------------|----------|
| JSON files in testdata/ | Structured JSON files versioned in git. Golden-file-compatible. | ✓ |
| Inline in test code | Go string literals or test table entries. | |
| Generated at runtime, not stored | No golden files for transcripts. | |

**User's choice:** JSON files in testdata/
**Notes:** Normalized transcripts (strip request IDs, timestamps, token counts). Separate input fixtures from captured artifacts. Layout: test/oracle/llm/testdata/transcripts/ and test/oracle/judge/testdata/scores/.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Offline from stored transcripts | Judge reads JSON transcripts, scores independently. Decoupled. | |
| Inline during behavioral test | Immediate judging after each behavioral test. | |
| Both modes supported | Offline default + inline mode for local iteration. | ✓ |

**User's choice:** Both modes supported, offline as default
**Notes:** Build tag separation: -tags=llm generates transcripts, -tags=llmjudge judges from stored. SERENA_INLINE_JUDGE=1 for inline mode. Tests always persist transcripts regardless of mode.

---

## API Key Gating & Cost

| Option | Description | Selected |
|--------|-------------|----------|
| t.Skip with clear message | t.Skip("ANTHROPIC_API_KEY not set"). Standard Go pattern. | ✓ |
| Build tag gating only | Already behind //go:build llm tags. | |
| Both -- tag + runtime skip | Belt and suspenders. | |

**User's choice:** t.Skip with clear message
**Notes:** Shows as skipped in test output, not failed.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Claude Haiku (cheapest) | Fast, cheap, good enough for tool selection. | ✓ |
| Claude Sonnet (balanced) | Better accuracy, moderate cost. | |
| Configurable, no default | Require explicit SERENA_TEST_MODEL. | |

**User's choice:** Claude Haiku (cheapest)
**Notes:** Default for CI. Keeps costs minimal.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Simple sequential with small delay | No t.Parallel, 100-500ms delay. Avoids rate limits. | ✓ |
| No rate limiting, rely on API backoff | Let SDK handle retries. | |
| Configurable concurrency | SERENA_LLM_CONCURRENCY env var. | |

**User's choice:** Simple sequential with small delay
**Notes:** Simple approach, avoids rate limit issues.

---

## Claude's Discretion

- Specific prompt templates for tool selection, disambiguation, and interpretation tests
- How to normalize transcripts (which fields to strip/replace)
- Exact delay duration between API calls (within 100-500ms range)
- Internal helper organization
- Additional disambiguation pairs beyond selectability_test.go set

## Deferred Ideas

None -- discussion stayed within phase scope
