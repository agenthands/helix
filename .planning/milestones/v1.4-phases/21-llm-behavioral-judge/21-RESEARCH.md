# Phase 21: LLM Behavioral & Judge - Research

**Researched:** 2026-04-12
**Domain:** LLM behavioral testing, transcript-based judge scoring, Anthropic Go SDK
**Confidence:** HIGH

## Summary

This phase builds LLM behavioral tests proving that Claude can correctly select, disambiguate, and interpret results from every Serena MCP tool, plus an LLM judge that scores transcripts via structured rubrics. The technical challenge is twofold: (1) writing a thin Go wrapper around the Anthropic Messages API that sends single-turn prompts and captures normalized transcripts, and (2) building a judge that replays those transcripts through a scoring rubric producing structured JSON verdicts.

The Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go` v1.35.0) is the only new dependency. It provides `client.Messages.New()` for single-turn completions, which is all this phase needs. No streaming, tool use, or multi-turn conversation is required for either subject or judge calls. The SDK reads `ANTHROPIC_API_KEY` from the environment by default, aligning perfectly with D-18's skip-on-missing-key pattern.

**Primary recommendation:** Build a shared `test/oracle/llm/llmtest` (or internal helpers in `test/oracle/llm/`) package with an Anthropic client wrapper, prompt builders, transcript normalization, and JSON serialization. Keep the judge in `test/oracle/judge/` completely decoupled -- it reads transcript JSON files and produces score JSON files, with no dependency on the harness or daemon.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Single-turn prompts for tool selection. One prompt per tool.
- **D-02:** One test case (subtest) per exposed tool.
- **D-03:** Superset of similar pairs from selectability_test.go plus additional pairs.
- **D-04:** Both structured interpretation and next-action prompts combined for output interpretation.
- **D-05:** 3-level scoring scale (0.0/0.5/1.0), five dimensions: tool_choice, description_use, output_interpretation, uncertainty_handling, polyglot_reasoning.
- **D-06:** Verdict thresholds: pass (no zeroes AND total >= 4.0), soft_fail (one zero OR total 2.5-3.5), fail (two+ zeroes OR total < 2.5).
- **D-07:** Judge output format is structured JSON with specified schema.
- **D-08:** Each rubric dimension requires anchor definitions.
- **D-09:** Per-transcript scoring plus aggregate summary.
- **D-10:** Two env vars: SERENA_TEST_MODEL, SERENA_JUDGE_MODEL. Self-judged fallback.
- **D-11:** Default subject model is Claude Haiku.
- **D-12:** Harness must support separate subject and judge model configuration.
- **D-13:** JSON transcripts in testdata/ directories, normalized, versioned in git.
- **D-14:** File layout: `test/oracle/llm/testdata/transcripts/<scenario-id>.json` for behavioral, `test/oracle/judge/testdata/scores/<scenario-id>.json` for scores.
- **D-15:** Separate input fixtures from captured artifacts.
- **D-16:** Both offline and inline modes. Offline canonical. SERENA_INLINE_JUDGE=1 for inline.
- **D-17:** Build tag separation: `llm` for transcripts, `llmjudge` for judging.
- **D-18:** t.Skip when ANTHROPIC_API_KEY not set.
- **D-19:** Sequential execution with 100-500ms delay. No t.Parallel.

### Claude's Discretion
- Specific prompt templates for tool selection, disambiguation, and interpretation tests
- How to normalize transcripts (which fields to strip/replace)
- Exact delay duration between API calls (within 100-500ms range)
- Whether aggregate summary is written to a file or just stdout
- Internal helper organization (shared LLM client, prompt builders, etc.)
- Which additional disambiguation pairs beyond the selectability_test.go set

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LLM-01 | Tool selection accuracy -- Claude selects correct tool given task description for every exposed tool | SDK Messages API for single-turn prompts; `listToolsWithMeta()` from selectability_test.go for tool list; one subtest per tool |
| LLM-02 | Disambiguation -- Claude distinguishes similar tools based on descriptions alone | Reuse `similarPairs` from selectability_test.go; extend with additional confusable pairs; same SDK single-turn pattern |
| LLM-03 | Output interpretation -- Claude correctly interprets tool results, no hallucination | Golden files from `test/oracle/contract/testdata/golden/` as real tool output; structured + next-action prompts |
| LLM-04 | LLM judge scores transcripts via structured rubrics, manual trigger only | Judge reads transcript JSON, sends to judge model with rubric prompt, parses structured JSON response, writes score files |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/anthropics/anthropic-sdk-go` | v1.35.0 | Anthropic Messages API client | Official SDK, auto-retries, reads ANTHROPIC_API_KEY from env [VERIFIED: `go list -m -versions`] |
| `github.com/stretchr/testify` | (existing) | Test assertions | Already in project [VERIFIED: codebase] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | Transcript serialization, judge output parsing | All transcript I/O and score files |
| `os` | stdlib | Env var reads (API key, model names, inline judge flag) | Test setup/skip logic |
| `time` | stdlib | Inter-call delays | Sequential execution pacing |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| anthropic-sdk-go | Raw HTTP to Messages API | SDK handles retries, auth, error typing; raw HTTP would be fragile |
| System prompt for JSON | Tool use for structured output | Single-turn text prompts are simpler; tool use adds complexity for judge scoring without benefit since we parse JSON from text |

**Installation:**
```bash
go get github.com/anthropics/anthropic-sdk-go@v1.35.0
```

**Version verification:** v1.35.0 published 2026-04-10 [VERIFIED: `go list -m -versions` and pkg.go.dev]

## Architecture Patterns

### Recommended Project Structure
```
test/oracle/llm/
├── doc.go                          # Build tag: //go:build llm || llmjudge (EXISTS)
├── client.go                       # Shared Anthropic client wrapper + config
├── prompt.go                       # Prompt templates for selection/disambiguation/interpretation
├── transcript.go                   # Transcript types, normalization, JSON I/O
├── selection_test.go               # LLM-01: one subtest per tool
├── disambiguation_test.go          # LLM-02: similar pairs
├── interpretation_test.go          # LLM-03: golden file output interpretation
└── testdata/
    └── transcripts/                # Normalized transcript JSON files
        ├── select-activate_project.json
        ├── disambig-search_symbols-vs-find_references.json
        └── interpret-search_symbols-success.json

test/oracle/judge/
├── doc.go                          # Build tag: //go:build llmjudge (EXISTS)
├── rubric.go                       # Rubric definitions, anchor text, scoring types
├── scorer.go                       # Judge scoring logic: read transcript, call judge model, parse score
├── aggregate.go                    # Aggregate report: averages, pass/fail counts, worst performers
├── judge_test.go                   # Reads transcripts from llm/testdata, scores them
└── testdata/
    └── scores/                     # Per-scenario score JSON files
        ├── select-activate_project.json
        └── disambig-search_symbols-vs-find_references.json
```

### Pattern 1: Shared Anthropic Client with Model Resolution
**What:** A thin wrapper that resolves model from env vars per D-10/D-11/D-12.
**When to use:** Every test that calls the Anthropic API.
**Example:**
```go
// Source: Derived from SDK docs + CONTEXT.md D-10/D-11/D-12
package llm

import (
    "os"
    "github.com/anthropics/anthropic-sdk-go"
)

const (
    EnvAPIKey     = "ANTHROPIC_API_KEY"
    EnvTestModel  = "SERENA_TEST_MODEL"
    EnvJudgeModel = "SERENA_JUDGE_MODEL"
    EnvInlineJudge = "SERENA_INLINE_JUDGE"

    DefaultSubjectModel = "claude-haiku-4-20250414" // D-11: cheapest
)

// SubjectModel returns the model ID for behavioral tests.
func SubjectModel() string {
    if m := os.Getenv(EnvTestModel); m != "" {
        return m
    }
    return DefaultSubjectModel
}

// JudgeModel returns the model ID for judge scoring.
// Returns (model, selfJudged).
func JudgeModel() (string, bool) {
    if m := os.Getenv(EnvJudgeModel); m != "" {
        return m, false
    }
    return SubjectModel(), true // self-judged fallback
}

// NewClient creates an Anthropic client. Reads ANTHROPIC_API_KEY from env.
func NewClient() *anthropic.Client {
    return anthropic.NewClient() // auto-reads ANTHROPIC_API_KEY
}
```

### Pattern 2: Single-Turn Prompt with Transcript Capture
**What:** Send one message, capture normalized transcript, write to JSON file.
**When to use:** Every LLM behavioral test (selection, disambiguation, interpretation).
**Example:**
```go
// Source: SDK docs + CONTEXT.md D-01/D-13
func askSingleTurn(ctx context.Context, client *anthropic.Client, model string,
    system string, userPrompt string) (*anthropic.Message, error) {

    params := anthropic.MessageNewParams{
        Model:     model,
        MaxTokens: 1024,
        Messages: []anthropic.MessageParam{
            anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
        },
    }
    if system != "" {
        params.System = []anthropic.TextBlockParam{{Text: system}}
    }
    return client.Messages.New(ctx, params)
}
```

### Pattern 3: Transcript Normalization
**What:** Strip non-deterministic fields before writing transcript JSON.
**When to use:** After every API call, before writing to testdata.
**Example:**
```go
// Source: CONTEXT.md D-13 -- normalize transcripts
type Transcript struct {
    ScenarioID string          `json:"scenario_id"`
    Model      string          `json:"model"`
    System     string          `json:"system,omitempty"`
    UserPrompt string          `json:"user_prompt"`
    Response   string          `json:"response"`
    StopReason string          `json:"stop_reason"`
    SelfJudged bool            `json:"self_judged,omitempty"`
    // Stripped: request ID, timestamps, token counts, latency, provider-specific fields
}
```

### Pattern 4: Judge Rubric with Anchors
**What:** Structured rubric with per-dimension anchor definitions (D-08).
**When to use:** Judge scoring prompt construction.
**Example:**
```go
// Source: CONTEXT.md D-05/D-06/D-07/D-08
type Score struct {
    ToolChoice            float64  `json:"tool_choice"`
    DescriptionUse        float64  `json:"description_use"`
    OutputInterpretation  float64  `json:"output_interpretation"`
    UncertaintyHandling   float64  `json:"uncertainty_handling"`
    PolyglotReasoning     float64  `json:"polyglot_reasoning"`
    Total                 float64  `json:"total"`
    Verdict               string   `json:"verdict"` // "pass", "soft_fail", "fail"
    Failures              []string `json:"failures"`
}

// Verdict computes the verdict from dimension scores per D-06.
func (s *Score) ComputeVerdict() {
    zeros := 0
    for _, v := range []float64{s.ToolChoice, s.DescriptionUse,
        s.OutputInterpretation, s.UncertaintyHandling, s.PolyglotReasoning} {
        if v == 0.0 {
            zeros++
        }
    }
    s.Total = s.ToolChoice + s.DescriptionUse + s.OutputInterpretation +
        s.UncertaintyHandling + s.PolyglotReasoning

    switch {
    case zeros >= 2 || s.Total < 2.5:
        s.Verdict = "fail"
    case zeros == 1 || (s.Total >= 2.5 && s.Total < 4.0):
        s.Verdict = "soft_fail"
    default: // no zeroes AND total >= 4.0
        s.Verdict = "pass"
    }
}
```

### Anti-Patterns to Avoid
- **Multi-turn conversations for testing:** Single-turn is cheaper, more deterministic, and sufficient per D-01. Do not build conversation loops.
- **Streaming for short responses:** Selection/disambiguation responses are short (tool name + brief reasoning). Non-streaming is simpler and cheaper.
- **Parsing LLM output with regex:** Use `strings.Contains` or `strings.EqualFold` for tool name matching. Do not over-engineer parsers for non-deterministic text.
- **Hard-coding tool lists:** Always get tools dynamically from `listToolsWithMeta()` or the harness. Tools may change between runs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP client for Anthropic | Custom REST client | `anthropic-sdk-go` | Handles auth, retries, rate limits, error typing |
| JSON schema validation for scores | Custom validator | Go struct tags + `encoding/json` | Score schema is simple enough for typed structs |
| Tool list enumeration | Hard-coded tool arrays | `listToolsWithMeta()` from harness via selectability_test.go pattern | Always current with actual daemon tool set |
| Golden file reading | Custom file loader | `os.ReadFile` with paths from `harness.ProjectRoot()` | Simple, proven pattern already in codebase |

**Key insight:** This phase is primarily prompt engineering + file I/O + JSON serialization. The only external call is `client.Messages.New()`. Keep the infrastructure minimal.

## Common Pitfalls

### Pitfall 1: Non-Deterministic LLM Responses Breaking Tests
**What goes wrong:** Same prompt returns different tool names or phrasing across runs, causing test flakes.
**Why it happens:** LLMs are inherently non-deterministic even with temperature=0.
**How to avoid:** Match on tool name substring presence rather than exact string equality. For selection tests, check that the correct tool name appears in the response text. For interpretation tests, check for required concepts (e.g., "success", "found N results") rather than exact wording. Use `temperature: 0` (or omit -- SDK default is fine) for maximum consistency.
**Warning signs:** Tests pass locally but fail in CI, or pass 9/10 times.

### Pitfall 2: Rate Limiting with Sequential Calls
**What goes wrong:** 38+ tools * (selection + disambiguation + interpretation) = 60-100+ API calls. Rate limits hit mid-run.
**Why it happens:** Default Anthropic rate limits, especially on Haiku tier.
**How to avoid:** D-19 specifies 100-500ms delays between calls. Use 250ms as default. The SDK has built-in retries (2 retries with backoff) which helps with transient 429s. Consider grouping tests so the test runner's sequential execution provides natural pacing.
**Warning signs:** `429 Too Many Requests` errors in test output.

### Pitfall 3: Judge Self-Reference Bias
**What goes wrong:** When SERENA_JUDGE_MODEL is unset and falls back to SERENA_TEST_MODEL, the same model judges its own output, inflating scores.
**Why it happens:** A model tends to rate its own reasoning style favorably.
**How to avoid:** D-10/D-12 address this -- mark runs as "self_judged: true" in score output. CI/release should always set a separate (stronger) judge model. Self-judged runs are lower-confidence evidence, not invalid.
**Warning signs:** Perfect scores on self-judged runs that degrade with external judge.

### Pitfall 4: Golden File Path Brittleness
**What goes wrong:** Golden file paths contain `<WORKSPACE>` placeholders that differ between machines.
**Why it happens:** Golden files are normalized but contain workspace-relative paths.
**How to avoid:** When feeding golden file content to LLM-03 interpretation tests, read the raw golden file content as-is. The LLM should interpret the output format, not depend on resolved paths. The `<WORKSPACE>` placeholder is already normalized in existing golden files.
**Warning signs:** Interpretation tests fail because LLM sees unfamiliar path patterns.

### Pitfall 5: Judge JSON Parsing Failures
**What goes wrong:** LLM judge returns free-form text instead of valid JSON for the score.
**Why it happens:** Without explicit instruction, models may add commentary around JSON.
**How to avoid:** Use a system prompt that says "Respond with ONLY a JSON object, no other text." Parse response by finding the first `{` and last `}` in the response text. Retry once on parse failure with a more explicit prompt. Validate all score values are in {0.0, 0.5, 1.0}.
**Warning signs:** `json.Unmarshal` errors in judge tests.

## Code Examples

Verified patterns from official sources and codebase:

### Anthropic SDK Basic Call
```go
// Source: https://platform.claude.com/docs/en/api/sdks/go [VERIFIED]
client := anthropic.NewClient() // reads ANTHROPIC_API_KEY from env

message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     "claude-haiku-4-20250414",
    MaxTokens: 1024,
    System: []anthropic.TextBlockParam{
        {Text: "You are a tool selection expert."},
    },
    Messages: []anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Which tool would you use?")),
    },
})
// Response text:
for _, block := range message.Content {
    switch v := block.AsAny().(type) {
    case anthropic.TextBlock:
        fmt.Println(v.Text)
    }
}
```

### API Key Skip Pattern
```go
// Source: CONTEXT.md D-18 + existing test patterns [VERIFIED: codebase]
func skipWithoutAPIKey(t *testing.T) {
    t.Helper()
    if os.Getenv("ANTHROPIC_API_KEY") == "" {
        t.Skip("ANTHROPIC_API_KEY not set")
    }
}
```

### Sequential Execution with Delay
```go
// Source: CONTEXT.md D-19
func runSequential(t *testing.T, tools []*mcp.Tool, fn func(t *testing.T, tool *mcp.Tool)) {
    for i, tool := range tools {
        if i > 0 {
            time.Sleep(250 * time.Millisecond) // D-19: 100-500ms range
        }
        t.Run(tool.Name, func(t *testing.T) {
            fn(t, tool)
        })
    }
}
```

### Reading Golden Files for Interpretation Tests
```go
// Source: Codebase pattern [VERIFIED: test/harness/golden.go + test/oracle/contract/testdata/golden/]
func loadGoldenOutput(toolName, scenario string) (string, error) {
    path := filepath.Join(harness.ProjectRoot(),
        "test", "oracle", "contract", "testdata", "golden", toolName, scenario+".golden")
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| anthropic-sdk-go alpha | v1.35.0 stable (v1.x GA) | 2025-02 (v1.0.0) | Stable API, `omitzero` semantics, proper union types |
| `anthropic.F()` field wrapper | Direct values + `omitzero` | v1.0.0+ | Params use plain values for required fields, `param.Opt[T]` for optional |
| Model string literals | `anthropic.ModelClaudeHaiku4_20250414` constants | Ongoing | SDK provides typed model constants; but env-var override needs string |

**Note on model constants:** The SDK provides typed model constants (e.g., `anthropic.ModelClaudeHaiku4_20250414`), but since D-10 allows env-var override with arbitrary model strings, the code should accept string model IDs and pass them directly. The SDK's `Model` field in `MessageNewParams` accepts `string` type. [VERIFIED: SDK docs show `Model: anthropic.ModelClaudeOpus4_6` which is a string constant]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Claude Haiku model ID is `claude-haiku-4-20250414` for cheapest option | Standard Stack | Wrong model ID causes API errors; easily fixable by checking Anthropic docs |
| A2 | 250ms delay between calls is sufficient to avoid rate limits for Haiku tier | Common Pitfalls | May need to increase to 500ms; SDK retries provide safety net |
| A3 | System prompt "Respond with ONLY a JSON object" is sufficient for judge JSON output | Code Examples | May need to extract JSON from mixed text response; fallback parser handles this |

## Open Questions

1. **Haiku Model ID String**
   - What we know: D-11 says "Claude Haiku (cheapest)". The SDK has model constants.
   - What's unclear: Exact model ID string for the current cheapest Haiku variant.
   - Recommendation: Use env var override as primary mechanism; hard-code a reasonable default that can be updated. Check `anthropic.ModelClaudeHaiku4_20250414` or similar constant at implementation time.

2. **Additional Disambiguation Pairs**
   - What we know: D-03 says "superset of similar pairs from selectability_test.go plus additional pairs."
   - What's unclear: Which additional pairs beyond the 7 in selectability_test.go.
   - Recommendation: At implementation time, review the full tool list and identify any additional pairs that an LLM might confuse (e.g., `edit_symbol_body` vs `replace_symbol`, `get_call_hierarchy` vs `get_type_hierarchy`, `search_symbols` vs `search_in_files`). This is explicitly in Claude's discretion.

3. **Aggregate Summary Output Format**
   - What we know: D-09 says per-transcript + aggregate. Claude's discretion includes "whether aggregate summary is written to a file or just stdout."
   - Recommendation: Write aggregate to both `t.Log()` for test output and a summary JSON file in `test/oracle/judge/testdata/` for persistence.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (existing) |
| Config file | Build tags `llm`, `llmjudge` in existing doc.go stubs |
| Quick run command | `go test -tags=llm -run TestSelection ./test/oracle/llm/ -count=1` |
| Full suite command | `go test -tags=llm ./test/oracle/llm/ -count=1 && go test -tags=llmjudge ./test/oracle/judge/ -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LLM-01 | Tool selection accuracy per tool | LLM behavioral | `go test -tags=llm -run TestSelection ./test/oracle/llm/ -count=1` | Wave 0 |
| LLM-02 | Disambiguation of similar pairs | LLM behavioral | `go test -tags=llm -run TestDisambiguation ./test/oracle/llm/ -count=1` | Wave 0 |
| LLM-03 | Output interpretation correctness | LLM behavioral | `go test -tags=llm -run TestInterpretation ./test/oracle/llm/ -count=1` | Wave 0 |
| LLM-04 | Judge scoring via rubrics | LLM judge | `go test -tags=llmjudge -run TestJudge ./test/oracle/judge/ -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go vet ./test/oracle/llm/... ./test/oracle/judge/...` (compile check only -- actual tests need API key)
- **Per wave merge:** Full suite if ANTHROPIC_API_KEY available
- **Phase gate:** Compilation passes; if API key available, all tests pass or skip gracefully

### Wave 0 Gaps
- [ ] `test/oracle/llm/client.go` -- shared Anthropic client wrapper, model resolution
- [ ] `test/oracle/llm/transcript.go` -- transcript types, normalization, file I/O
- [ ] `test/oracle/llm/prompt.go` -- prompt templates for all three test types
- [ ] `test/oracle/judge/rubric.go` -- rubric definitions, anchor text, score types
- [ ] `test/oracle/judge/scorer.go` -- judge scoring logic
- [ ] `go get github.com/anthropics/anthropic-sdk-go@v1.35.0` -- add dependency

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A -- tests only, no user-facing auth |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | Validate score values are in {0.0, 0.5, 1.0}; validate JSON parse of judge output |
| V6 Cryptography | no | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| API key in test output | Information Disclosure | Never log ANTHROPIC_API_KEY value; t.Skip message says "not set", not the value |
| Prompt injection via tool descriptions | Tampering | Tool descriptions come from trusted daemon, not external input |

## Sources

### Primary (HIGH confidence)
- Anthropic Go SDK v1.35.0 -- verified via `go list -m -versions`, docs at platform.claude.com/docs/en/api/sdks/go [VERIFIED]
- Codebase: `test/oracle/contract/selectability_test.go` -- disambiguation pairs, listToolsWithMeta() [VERIFIED: read file]
- Codebase: `test/oracle/llm/doc.go`, `test/oracle/judge/doc.go` -- build tag stubs [VERIFIED: read files]
- Codebase: `test/harness/runner.go`, `test/harness/tools.go` -- harness infrastructure [VERIFIED: read files]
- Codebase: `test/oracle/contract/testdata/golden/` -- 24 golden file directories [VERIFIED: ls]

### Secondary (MEDIUM confidence)
- Anthropic Go SDK README and pkg.go.dev for API patterns [CITED: github.com/anthropics/anthropic-sdk-go, pkg.go.dev]

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - single dependency (anthropic-sdk-go), verified version and API
- Architecture: HIGH - follows established codebase patterns (build tags, harness, golden files)
- Pitfalls: MEDIUM - rate limiting thresholds and LLM non-determinism are experience-based
- Judge scoring: HIGH - verdict logic is pure Go math, fully deterministic

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (SDK stable, patterns established)
