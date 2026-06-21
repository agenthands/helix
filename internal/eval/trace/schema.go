package trace

import (
	"encoding/json"
	"time"
)

// EventKind identifies the source-event type in a merged trace.
type EventKind string

const (
	// KindSessionInit is emitted by CC when a new Claude session starts.
	KindSessionInit EventKind = "session_init"
	// KindAssistantMsg is emitted by CC for each assistant turn.
	KindAssistantMsg EventKind = "assistant_message"
	// KindToolCall is emitted by the daemon TelemetryMiddleware per tools/call.
	KindToolCall EventKind = "tool_call"
	// KindToolResult is emitted by CC when a tool result is returned.
	KindToolResult EventKind = "tool_result"
	// KindResult is emitted by CC as the final result event with usage data.
	KindResult EventKind = "result"
	// KindAPIRetry is emitted by CC on API retry events.
	KindAPIRetry EventKind = "api_retry"
	// KindReceiptIssued is emitted by the daemon guardrails store when a
	// receipt is issued. Carries ReceiptClass + TraceID + Pid. F-08.
	KindReceiptIssued EventKind = "receipt_issued"
)

// validEventKinds is the closed enum for event kinds.
// KEEP-IN-SYNC with the KindXxx constants above.
var validEventKinds = map[string]struct{}{
	string(KindSessionInit):  {},
	string(KindAssistantMsg): {},
	string(KindToolCall):     {},
	string(KindToolResult):   {},
	string(KindResult):       {},
	string(KindAPIRetry):     {},
	string(KindReceiptIssued): {},
}

// validOutcomes is the closed enum for tool-call outcomes.
// KEEP-IN-SYNC with internal/mcp/middleware.go outcomeEnum.
// Source of truth is middleware.go; update here whenever a new outcome is added.
var validOutcomes = map[string]struct{}{
	"success":          {},
	"invalid_args":     {},
	"not_found":        {},
	"circuit_open":     {},
	"ls_crash":         {},
	"timeout":          {},
	"internal":         {},
	"guardrail_warned":  {},
	"guardrail_blocked": {},
}

// IsValidOutcome returns true if s is a member of the closed outcome enum.
// The enum mirrors internal/mcp/middleware.go outcomeEnum (Phase 66 extended).
func IsValidOutcome(s string) bool {
	_, ok := validOutcomes[s]
	return ok
}

// IsValidEventKind returns true if s is a member of the closed event-kind enum.
func IsValidEventKind(s string) bool {
	_, ok := validEventKinds[s]
	return ok
}

// ToolUse captures a single tool-use block from a CC assistant message.
type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

// GuardrailInfo carries rule + level metadata for a guardrail outcome.
type GuardrailInfo struct {
	Rule  string `json:"rule"`
	Level string `json:"level"`
}

// Usage aggregates token counts from the CC result event.
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
}

// ToolCallSummary tallies tool calls by tool name and outcome.
type ToolCallSummary struct {
	Total     int            `json:"total"`
	ByTool    map[string]int `json:"by_tool"`
	ByOutcome map[string]int `json:"by_outcome"`
}

// GuardrailCounts accumulates guardrail event counts from the trace.
type GuardrailCounts struct {
	Warned          int `json:"warned"`
	Blocked         int `json:"blocked"`
	ReceiptsIssued  int `json:"receipts_issued"`
}

// Event is a single timestamped event in a merged trace. Fields are
// omitempty so daemon-side and CC-side events carry only relevant fields.
type Event struct {
	T      time.Time `json:"t"`
	Source string    `json:"source"` // "daemon" | "cc"
	Kind   EventKind `json:"kind"`

	// tool_call fields (Source == "daemon", Kind == KindToolCall)
	Tool            string         `json:"tool,omitempty"`
	Outcome         string         `json:"outcome,omitempty"`
	DurationMs      int64          `json:"duration_ms,omitempty"`
	TraceID         string         `json:"trace_id,omitempty"`
	ArgsSummary     string         `json:"args_summary,omitempty"`
	ResultSizeBytes int            `json:"result_size_bytes,omitempty"`
	Guardrail       *GuardrailInfo `json:"guardrail,omitempty"`

	// cc fields (Source == "cc")
	SessionID   string    `json:"session_id,omitempty"`
	Model       string    `json:"model,omitempty"`
	MCPServers  []string  `json:"mcp_servers,omitempty"`
	TextSummary string    `json:"text_summary,omitempty"`
	ToolUses    []ToolUse `json:"tool_uses,omitempty"`
	ToolUseID   string    `json:"tool_use_id,omitempty"`
	IsError     bool      `json:"is_error,omitempty"`

	// daemon-side trust attestation (T-67-04)
	Pid int `json:"pid,omitempty"`

	// receipt_issued fields (Source == "daemon", Kind == KindReceiptIssued).
	// F-08: populated by tap when parsing msg=="receipt issued" daemon lines.
	ReceiptClass string `json:"receipt_class,omitempty"`
}

// MergedTrace is the canonical per-(task, mode) artifact produced by Merge.
// SchemaVersion is always "1" in this implementation.
type MergedTrace struct {
	SchemaVersion string    `json:"schema_version"`
	TaskID        string    `json:"task_id"`
	Mode          string    `json:"mode"`
	RunID         string    `json:"run_id"`
	ClaudeVersion string    `json:"claude_version"`
	HelixVersion  string    `json:"helix_version"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	DurationMs    int64     `json:"duration_ms"`
	// Outcome is one of: "success" | "failed" | "failed-with-cause: budget_<axis>"
	Outcome       string          `json:"outcome"`
	Events        []Event         `json:"events"`
	Usage         Usage           `json:"usage"`
	// UsagePresent is the out-of-band presence signal for Usage: true when a CC
	// `result` event actually carried a provider usage block (set by the tap at
	// env.Usage != nil). Because Usage is a plain value struct, a genuine absence
	// and a real all-zero usage block are indistinguishable from the values alone;
	// downstream token grading (token_meter / D-01/D-03) MUST consult this flag
	// rather than infer presence from a value threshold.
	UsagePresent  bool            `json:"usage_present"`
	ToolCallSummary ToolCallSummary `json:"tool_call_summary"`
	Guardrails    GuardrailCounts `json:"guardrails"`
	// FailureReason is set when Outcome == "failed" due to an invariant violation
	// (e.g. "patch_outside_repo" for T-67-02 path-prefix check).
	FailureReason string `json:"failure_reason,omitempty"`
}
