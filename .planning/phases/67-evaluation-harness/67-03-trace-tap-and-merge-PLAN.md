---
phase: 67
plan: 03
type: tdd
wave: 1
depends_on: [67-01]
autonomous: true
requirements: [EVAL-01, EVAL-02]
files_modified:
  - internal/eval/trace/schema.go
  - internal/eval/trace/schema_test.go
  - internal/eval/trace/tap.go
  - internal/eval/trace/tap_test.go
  - internal/eval/trace/merge.go
  - internal/eval/trace/merge_test.go
tags: [eval, trace, telemetry]

must_haves:
  truths:
    - "Daemon JSON-formatted stderr is parseable into typed events (kind=tool_call, outcome, tool, duration_ms, t)"
    - "Claude Code stream-json stdout is parseable into typed events (system_init, assistant, user/tool_result, result)"
    - "Daemon events are pid-stamped; events from foreign pids are rejected (T-67-04 mitigation)"
    - "Merge sorts by wall-clock t ASC; daemon-side wins on tie"
    - "Merged trace.json conforms to RESEARCH §'Merged trace.json Shape' schema_version=1"
    - "Result event from CC populates usage.{input_tokens, output_tokens}"
  artifacts:
    - path: "internal/eval/trace/schema.go"
      provides: "Canonical Event types + MergedTrace struct"
      exports: ["Event", "EventKind", "MergedTrace", "Usage", "ToolCallSummary", "GuardrailCounts"]
    - path: "internal/eval/trace/tap.go"
      provides: "Stream readers for daemon JSONL stderr + CC stream-json stdout"
      exports: ["TapDaemonLog", "TapCCStream"]
    - path: "internal/eval/trace/merge.go"
      provides: "Wall-clock merge + dedupe"
      exports: ["Merge"]
  key_links:
    - from: "internal/eval/trace/tap.go"
      to: "internal/mcp/middleware.go"
      via: "TelemetryMiddleware outcome enum (success/timeout/circuit_open/internal/guardrail_warned/guardrail_blocked/...)"
      pattern: "outcome"
    - from: "internal/eval/trace/merge.go"
      to: "internal/eval/trace/schema.go"
      via: "MergedTrace assembly"
      pattern: "MergedTrace"
---

<objective>
Wave 1 part 2: trace tap + merger. Daemon-side TelemetryMiddleware (D-02 source of truth) emits JSON-formatted log lines we parse from `daemon.log` (stderr); CC emits stream-json events on stdout; merger produces a single canonical `trace.json` per (task, mode) per RESEARCH §"Merged trace.json Shape".

Purpose: EVAL-01 mandates capturing tool calls, outcomes, guardrail compliance, tokens. The merger turns two stream sources into the single artifact every downstream scorer/reporter consumes.

Output: typed schema + two tap readers + merge function with TDD tests including the T-67-04 pid-stamping invariant.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@internal/mcp/middleware.go

<interfaces>
<!-- Outcome enum from internal/mcp/middleware.go (extracted) -->
TelemetryMiddleware emits JSON log lines on `tools/call`. The closed outcome set is:
```
success | invalid_args | not_found | circuit_open | ls_crash |
timeout | internal | guardrail_warned | guardrail_blocked
```
Each line carries: `time` (RFC3339Nano), `level`, `msg=="tool call"`, `tool` (str), `outcome` (str), `duration_ms` (int), `session_id` (str), `pid` (int), `trace_id` (str, optional).

<!-- CC stream-json event types (RESEARCH §Trace Merge Schema) -->
With `--output-format=stream-json --verbose --include-partial-messages`:
- `{"type":"system","subtype":"init", "session_id":..., "model":..., "mcp_servers":[...]}`
- `{"type":"system","subtype":"api_retry", ...}`
- `{"type":"assistant","message":{"content":[{"type":"text",...},{"type":"tool_use","id":"toolu_x","name":"mcp__helix__find_references","input":{...}}]}}`
- `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_x","is_error":false}]}}`
- `{"type":"stream_event","event":{"type":"content_block_delta", ...}}` — SKIP these
- `{"type":"result","subtype":"...","usage":{"input_tokens":N,"output_tokens":N,...},"session_id":...,"num_turns":N,"total_cost_usd":...}`
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Schema — typed Event + MergedTrace structs (schema_version=1)</name>
  <files>
    internal/eval/trace/schema.go,
    internal/eval/trace/schema_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestEventJSONRoundTrip — every defined `EventKind` round-trips through JSON marshal/unmarshal byte-identically.
    - Test 2 (RED): TestMergedTraceShapeMatchesResearch — table-driven: encode a MergedTrace and assert the JSON has top-level keys `schema_version, task_id, mode, run_id, claude_version, helix_version, started_at, ended_at, duration_ms, outcome, events, usage, tool_call_summary, guardrails`.
    - Test 3 (RED): TestOutcomeClosedEnum — `IsValidOutcome("success")` true; `IsValidOutcome("guardrail_warned")` true; `IsValidOutcome("badword")` false. Closed enum from middleware.go.
    - Test 4 (RED): TestEventKindClosedEnum — same for event kinds: `session_init`, `assistant_message`, `tool_call`, `tool_result`, `result`, `api_retry`.
  </behavior>
  <action>
    Define the typed schema verbatim per RESEARCH §"Merged trace.json Shape":
    ```go
    type EventKind string
    const (
        KindSessionInit EventKind = "session_init"
        KindAssistantMsg EventKind = "assistant_message"
        KindToolCall EventKind = "tool_call"
        KindToolResult EventKind = "tool_result"
        KindResult EventKind = "result"
        KindAPIRetry EventKind = "api_retry"
    )
    type Event struct {
        T time.Time `json:"t"`
        Source string `json:"source"` // "daemon" | "cc"
        Kind EventKind `json:"kind"`
        // tool_call fields
        Tool string `json:"tool,omitempty"`
        Outcome string `json:"outcome,omitempty"`
        DurationMs int64 `json:"duration_ms,omitempty"`
        TraceID string `json:"trace_id,omitempty"`
        ArgsSummary string `json:"args_summary,omitempty"`
        ResultSizeBytes int `json:"result_size_bytes,omitempty"`
        Guardrail *GuardrailInfo `json:"guardrail,omitempty"`
        // cc fields
        SessionID string `json:"session_id,omitempty"`
        Model string `json:"model,omitempty"`
        MCPServers []string `json:"mcp_servers,omitempty"`
        TextSummary string `json:"text_summary,omitempty"`
        ToolUses []ToolUse `json:"tool_uses,omitempty"`
        ToolUseID string `json:"tool_use_id,omitempty"`
        IsError bool `json:"is_error,omitempty"`
        // daemon-side trust attestation
        Pid int `json:"pid,omitempty"`
    }
    type ToolUse struct{ ID, Name string; Input json.RawMessage }
    type GuardrailInfo struct{ Rule, Level string }
    type Usage struct{ InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens int }
    type ToolCallSummary struct{ Total int; ByTool map[string]int; ByOutcome map[string]int }
    type GuardrailCounts struct{ Warned, Blocked, ReceiptsIssued int }
    type MergedTrace struct {
        SchemaVersion string `json:"schema_version"`
        TaskID, Mode, RunID, ClaudeVersion, HelixVersion string
        StartedAt, EndedAt time.Time
        DurationMs int64
        Outcome string // success | failed | failed-with-cause:budget_<axis>
        Events []Event
        Usage Usage
        ToolCallSummary ToolCallSummary
        Guardrails GuardrailCounts
    }
    ```

    Plus closed-enum validators `IsValidOutcome(s string) bool` and `IsValidEventKind(s string) bool` driven by the same constants used by `internal/mcp/middleware.go`. To avoid a circular import, declare the outcome list as a local `var validOutcomes = map[string]struct{}{...}` literal AND assert (in `internal/mcp/middleware_test.go` if reachable, otherwise in this package's test) that the local list matches `mcp.AllOutcomes` if such a symbol exists. If `internal/mcp` does not export the list, hard-code it from middleware.go and add a // KEEP-IN-SYNC comment.
  </action>
  <verify>
    <automated>go test ./internal/eval/trace -run "TestEvent|TestMergedTrace|TestOutcome|TestEventKindClosed" -count=1 -race</automated>
  </verify>
  <done>
    Schema compiles; four tests green. Schema matches RESEARCH §"Merged trace.json Shape" exactly.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Tap — parse daemon JSON-log stderr and CC stream-json stdout into typed events</name>
  <files>
    internal/eval/trace/tap.go,
    internal/eval/trace/tap_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestTapDaemonLog_ToolCall — feed a JSON-formatted line `{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345,"trace_id":"abc"}` ; assert one Event{Source:"daemon", Kind:KindToolCall, Tool:"find_references", Outcome:"success", DurationMs:23, Pid:12345}.
    - Test 2 (RED): TestTapDaemonLog_PidGate — daemon was spawned with pid=12345; an event with `"pid":99999` is REJECTED (T-67-04 mitigation). The reader returns the rejected count via the result struct; the rejected event does NOT appear in the event slice.
    - Test 3 (RED): TestTapDaemonLog_GuardrailOutcome — outcome=guardrail_warned with `"guardrail":{"rule":"G-001","level":"warn"}` populates Event.Guardrail.
    - Test 4 (RED): TestTapDaemonLog_NoiseLines — non-tool-call log lines (any `msg != "tool call"`) are silently skipped.
    - Test 5 (RED): TestTapDaemonLog_TruncatedLine — a truncated last line returns no error; partial line is dropped with a warning recorded on the Result.
    - Test 6 (RED): TestTapCCStream_SessionInit — emits one Event{Source:"cc", Kind:KindSessionInit, SessionID, Model, MCPServers}.
    - Test 7 (RED): TestTapCCStream_AssistantWithToolUse — emits Event{Source:"cc", Kind:KindAssistantMsg, TextSummary, ToolUses}.
    - Test 8 (RED): TestTapCCStream_ToolResult — emits Event{Source:"cc", Kind:KindToolResult, ToolUseID, IsError}.
    - Test 9 (RED): TestTapCCStream_Result — emits Event{Source:"cc", Kind:KindResult} AND populates a returned Usage struct.
    - Test 10 (RED): TestTapCCStream_SkipsStreamEvent — `"type":"stream_event"` lines are skipped.
    - Test 11 (RED): TestTapCCStream_UnknownTypeWarns — unknown `type` is skipped with a counter increment, not an error.
  </behavior>
  <action>
    `tap.go` exports:
    - `type DaemonTapResult struct { Events []Event; RejectedForeignPid int; SkippedTruncated int }`
    - `func TapDaemonLog(path string, expectedPid int) (DaemonTapResult, error)` — `bufio.Scanner` over the file; per line `json.Unmarshal` into a small struct then map to Event when `msg=="tool call"`. PID gate: skip events whose `pid != expectedPid` and increment `RejectedForeignPid`. Truncated/malformed last lines increment `SkippedTruncated` rather than error.
    - `type CCTapResult struct { Events []Event; Usage Usage; UnknownTypes int; FinalSessionID string }`
    - `func TapCCStream(path string) (CCTapResult, error)` — `bufio.Scanner` with increased buffer (CC events can be large); `json.Decoder.UseNumber()` is fine. Switch on `type` field per the schema in `<interfaces>`. `result` populates Usage.

    Both tappers use `time.Parse(time.RFC3339Nano, ...)`; on bad timestamps, fall back to "now" with a warning counter rather than failing the whole stream (defensive — Pitfall 5 mitigation).
  </action>
  <verify>
    <automated>go test ./internal/eval/trace -run "TestTap" -count=1 -race</automated>
  </verify>
  <done>
    Eleven tap tests pass. PID-gate enforced (T-67-04). Truncated/unknown lines counted, not fatal. Race-clean.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Merge — wall-clock sort, daemon-wins-on-tie, build MergedTrace + Usage + summaries</name>
  <files>
    internal/eval/trace/merge.go,
    internal/eval/trace/merge_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestMergeSortsByWallClock — interleaved daemon and CC events come out sorted by `t` ASC.
    - Test 2 (RED): TestMergeDaemonWinsOnTie — when two events have identical t, daemon-source comes first (wall-clock ambiguity in the < 1ms window between cc tool_use emit and daemon tool_call dispatch).
    - Test 3 (RED): TestMergeBuildsToolCallSummary — given 3 daemon tool_call events (find_references×2 success, rename_symbol×1 guardrail_warned), summary.Total==3, ByTool=={find_references:2, rename_symbol:1}, ByOutcome=={success:2, guardrail_warned:1}.
    - Test 4 (RED): TestMergeBuildsGuardrailCounts — given 1 warned + 1 blocked, GuardrailCounts.Warned=1, Blocked=1.
    - Test 5 (RED): TestMergeUsageFromCC — CC tap result Usage is copied verbatim into MergedTrace.Usage.
    - Test 6 (RED): TestMergeOutcomeFromBudgetBreach — when caller passes a `*budget.BreachReason`, MergedTrace.Outcome equals `failed-with-cause: budget_<axis>` (D-08 wording).
    - Test 7 (RED): TestMergeOutcomeFromVerifyExitCode — when caller passes verifyExitCode==0, Outcome=="success"; non-zero → "failed".
    - Test 8 (RED): TestMergePathPrefixInvariant — given a list of patch paths from the agent run, Merge asserts every path is under `repoRoot`. A path like `../../../etc/passwd` causes Merge to return an error AND set Outcome="failed" with reason "patch_outside_repo" (T-67-02 mitigation).
  </behavior>
  <action>
    `merge.go` exports:
    ```go
    type MergeInput struct {
        TaskID, Mode, RunID, ClaudeVersion, HelixVersion string
        StartedAt, EndedAt time.Time
        Daemon DaemonTapResult
        CC CCTapResult
        VerifyExitCode int
        Budget *budget.BreachReason
        PatchPaths []string
        RepoRoot string
    }
    func Merge(in MergeInput) (MergedTrace, error)
    ```
    Implementation:
    1. Concat daemon and CC events; stable sort by `t` ASC; on tie, daemon source wins (use `sort.SliceStable` with comparator that prefers Source=="daemon").
    2. Build `ToolCallSummary` by iterating over events with Source=="daemon" && Kind==KindToolCall; tally Total, ByTool, ByOutcome.
    3. Build `GuardrailCounts` from outcomes guardrail_warned/_blocked.
    4. Outcome resolution priority: `Budget != nil` → `failed-with-cause: budget_<axis>`; else `VerifyExitCode != 0` → "failed"; else "success".
    5. Path-prefix invariant: for each path in `PatchPaths`, `filepath.Rel(RepoRoot, path)` must NOT begin with ".."; otherwise return error and set Outcome="failed", reason recorded in a top-level `failure_reason` field (add to schema if not already present).
    6. SchemaVersion = "1" hard-coded.
    7. DurationMs = int64(EndedAt.Sub(StartedAt) / time.Millisecond).

    Add `FailureReason string \`json:"failure_reason,omitempty"\`` to `MergedTrace` if not present; update the schema test to keep it optional.
  </action>
  <verify>
    <automated>go test ./internal/eval/trace -run "TestMerge" -count=1 -race && go test ./internal/eval/trace -count=1 -race</automated>
  </verify>
  <done>
    Eight merge tests pass; full trace package green; race-clean. Path-prefix invariant blocks T-67-02. Budget breach surfaces verbatim per D-08.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Daemon stderr → trace tap | Trusted in shape (we control the daemon binary) but pid-stamped to defend against rogue child processes appending to the same log file. |
| CC stdout → trace tap | Semi-trusted: CC version drift may add new event types; unknown types are counted, not fatal. |
| Patch paths → MergedTrace | Untrusted: agent could return paths outside the repo; path-prefix invariant rejects. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-02 | T (Tampering) | claude subprocess writes outside per-task `repo/` | mitigate | TestMergePathPrefixInvariant: any patch path that resolves outside `RepoRoot` causes Merge to error AND mark Outcome="failed" with `failure_reason="patch_outside_repo"`. Combined with `cmd.Dir = sandbox.RepoFor` set in Plan 02, this is double-defense. |
| T-67-04 | S (Spoofing) | daemon JSONL trace injection from foreign pid | mitigate | TestTapDaemonLog_PidGate asserts foreign-pid events are rejected; rejected count is surfaced on the result struct so reporters can highlight anomalies. |
| T-67-Pitfall-3 | (CC version drift) | CC stream-json schema evolution | mitigate | Unknown `type` lines are counted, not fatal (TestTapCCStream_UnknownTypeWarns). Reporter records `claude_version` per D-03 so cross-version diffs filter cleanly. |
| T-67-Pitfall-5 | (Wall-clock ordering) | Same-host clock drift between daemon and CC | accept | Single-host invariant documented in `merge.go` per RESEARCH §"Trace Merge Schema". Future cross-host expansion would require correlation IDs (out of scope this phase). |

Block on: HIGH severity. T-67-02 and T-67-04 are HIGH; both have asserting tests. Pitfall-5 accepted with documented invariant.
</threat_model>

<verification>
- `internal/eval/trace/...` builds clean and tests pass under `-race`.
- Merged JSON shape exactly matches RESEARCH §"Merged trace.json Shape".
- Pid-gate (T-67-04) is asserted by test that would fail if removed.
- Path-prefix (T-67-02) is asserted by test that would fail if removed.
- Outcome wording matches D-08 verbatim: `failed-with-cause: budget_<axis>`.
</verification>

<success_criteria>
- [ ] schema.go + schema_test.go (4 tests green).
- [ ] tap.go + tap_test.go (11 tests green).
- [ ] merge.go + merge_test.go (8 tests green).
- [ ] `go vet ./internal/eval/trace/...` clean.
- [ ] T-67-02 and T-67-04 mitigations have asserting tests.
</success_criteria>

<dependencies>
- Plans: **67-01** for both (a) `internal/eval/trace` package skeleton AND (b) `internal/eval/budget/types.go` BreachReason type declaration (Wave-0 cross-plan type contract — see Plan 01 Task 3 EXCEPTION block). Plan 03 imports `budget.BreachReason` for `MergeInput.Budget *budget.BreachReason`.
- Plans: **NOT** 67-02. Although Plan 02 owns the watchdog *behavior* in the same `internal/eval/budget` package, Plan 03 only needs the *type*, which lands in Wave 0 via Plan 01. This is what makes Plan 02 and Plan 03 parallel-eligible Wave-1 work with disjoint files.
- External: none.
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-03-SUMMARY.md` recording: final schema fields, outcome enum source-of-truth pattern (KEEP-IN-SYNC vs cross-package import), any new `failure_reason` field added.
</output>
