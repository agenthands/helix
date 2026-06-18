package trace_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/trace"
)

// writeLines writes lines to a temp file and returns its path.
func writeLines(t *testing.T, lines ...string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "trace-*.jsonl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return f.Name()
}

// writeFile writes content to a temp file and returns its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "trace.json")
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return name
}

// TestTapDaemonLog_ToolCall verifies a tool call line is parsed correctly.
func TestTapDaemonLog_ToolCall(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345,"trace_id":"abc"}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(res.Events))
	}
	ev := res.Events[0]
	if ev.Source != "daemon" {
		t.Errorf("Source = %q, want %q", ev.Source, "daemon")
	}
	if ev.Kind != trace.KindToolCall {
		t.Errorf("Kind = %q, want %q", ev.Kind, trace.KindToolCall)
	}
	if ev.Tool != "find_references" {
		t.Errorf("Tool = %q, want %q", ev.Tool, "find_references")
	}
	if ev.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", ev.Outcome, "success")
	}
	if ev.DurationMs != 23 {
		t.Errorf("DurationMs = %d, want 23", ev.DurationMs)
	}
	if ev.Pid != 12345 {
		t.Errorf("Pid = %d, want 12345", ev.Pid)
	}
}

// TestTapDaemonLog_PidGate verifies events with foreign pids are rejected (T-67-04).
func TestTapDaemonLog_PidGate(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345}`,
		`{"time":"2026-05-10T08:30:02.000Z","level":"info","msg":"tool call","tool":"rename_symbol","outcome":"success","duration_ms":10,"pid":99999}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	if len(res.Events) != 1 {
		t.Errorf("want 1 accepted event, got %d", len(res.Events))
	}
	if res.RejectedForeignPid != 1 {
		t.Errorf("RejectedForeignPid = %d, want 1", res.RejectedForeignPid)
	}
	// Make sure it's the right event that was kept
	if len(res.Events) > 0 && res.Events[0].Pid != 12345 {
		t.Errorf("kept event has pid %d, want 12345", res.Events[0].Pid)
	}
}

// TestTapDaemonLog_GuardrailOutcome verifies guardrail outcome populates Event.Guardrail.
func TestTapDaemonLog_GuardrailOutcome(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-10T08:30:01.812Z","level":"warn","msg":"tool call","tool":"delete_file","outcome":"guardrail_warned","duration_ms":5,"pid":12345,"guardrail":{"rule":"G-001","level":"warn"}}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(res.Events))
	}
	ev := res.Events[0]
	if ev.Guardrail == nil {
		t.Fatal("Guardrail is nil, want non-nil")
	}
	if ev.Guardrail.Rule != "G-001" {
		t.Errorf("Guardrail.Rule = %q, want %q", ev.Guardrail.Rule, "G-001")
	}
	if ev.Guardrail.Level != "warn" {
		t.Errorf("Guardrail.Level = %q, want %q", ev.Guardrail.Level, "warn")
	}
}

// TestTapDaemonLog_NoiseLines verifies non-tool-call log lines are silently skipped.
func TestTapDaemonLog_NoiseLines(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-10T08:30:00.000Z","level":"info","msg":"daemon started"}`,
		`{"time":"2026-05-10T08:30:01.000Z","level":"info","msg":"workspace activated","workspace":"/tmp/repo"}`,
		`{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345}`,
		`{"time":"2026-05-10T08:30:02.000Z","level":"info","msg":"session closed"}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	if len(res.Events) != 1 {
		t.Errorf("want 1 event (tool call only), got %d", len(res.Events))
	}
}

// TestTapDaemonLog_TruncatedLine verifies a truncated last line doesn't error,
// is dropped, and is counted in SkippedTruncated.
func TestTapDaemonLog_TruncatedLine(t *testing.T) {
	t.Parallel()
	// Write a valid line then an incomplete line without a trailing newline
	f, err := os.CreateTemp(t.TempDir(), "trace-*.jsonl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	// valid tool call
	_, _ = f.WriteString(`{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345}` + "\n")
	// truncated (no newline, incomplete JSON)
	_, _ = f.WriteString(`{"time":"2026-05-10T08:30:02.000Z","level":"info","msg":"tool call","tool":`)
	f.Close()

	res, err := trace.TapDaemonLog(f.Name(), 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog should not error on truncated last line, got: %v", err)
	}
	// The valid line should be kept
	if len(res.Events) != 1 {
		t.Errorf("want 1 event, got %d", len(res.Events))
	}
	// The truncated line should be counted
	if res.SkippedTruncated != 1 {
		t.Errorf("SkippedTruncated = %d, want 1", res.SkippedTruncated)
	}
}

// TestTapCCStream_SessionInit verifies session_init event parsing.
func TestTapCCStream_SessionInit(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"system","subtype":"init","session_id":"sess-abc","model":"claude-3-7-sonnet","mcp_servers":[{"name":"helix"}]}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	// Find session_init event
	var found *trace.Event
	for i := range res.Events {
		if res.Events[i].Kind == trace.KindSessionInit {
			found = &res.Events[i]
			break
		}
	}
	if found == nil {
		t.Fatal("no session_init event found")
	}
	if found.Source != "cc" {
		t.Errorf("Source = %q, want %q", found.Source, "cc")
	}
	if found.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", found.SessionID, "sess-abc")
	}
	if found.Model != "claude-3-7-sonnet" {
		t.Errorf("Model = %q, want %q", found.Model, "claude-3-7-sonnet")
	}
	if len(found.MCPServers) == 0 {
		t.Error("MCPServers is empty, want at least one entry")
	}
}

// TestTapCCStream_AssistantWithToolUse verifies assistant message with tool use.
func TestTapCCStream_AssistantWithToolUse(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I'll find references for you."},{"type":"tool_use","id":"toolu_x","name":"mcp__helix__find_references","input":{"symbol":"Foo"}}]}}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	var found *trace.Event
	for i := range res.Events {
		if res.Events[i].Kind == trace.KindAssistantMsg {
			found = &res.Events[i]
			break
		}
	}
	if found == nil {
		t.Fatal("no assistant_message event found")
	}
	if found.Source != "cc" {
		t.Errorf("Source = %q, want %q", found.Source, "cc")
	}
	if found.TextSummary == "" {
		t.Error("TextSummary is empty, want non-empty")
	}
	if len(found.ToolUses) == 0 {
		t.Error("ToolUses is empty, want at least one entry")
	}
	if found.ToolUses[0].Name != "mcp__helix__find_references" {
		t.Errorf("ToolUses[0].Name = %q, want %q", found.ToolUses[0].Name, "mcp__helix__find_references")
	}
}

// TestTapCCStream_ToolResult verifies tool result event parsing.
func TestTapCCStream_ToolResult(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_x","is_error":false,"content":[{"type":"text","text":"Found 3 references"}]}]}}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	var found *trace.Event
	for i := range res.Events {
		if res.Events[i].Kind == trace.KindToolResult {
			found = &res.Events[i]
			break
		}
	}
	if found == nil {
		t.Fatal("no tool_result event found")
	}
	if found.Source != "cc" {
		t.Errorf("Source = %q, want %q", found.Source, "cc")
	}
	if found.ToolUseID != "toolu_x" {
		t.Errorf("ToolUseID = %q, want %q", found.ToolUseID, "toolu_x")
	}
	if found.IsError {
		t.Error("IsError = true, want false")
	}
}

// TestTapCCStream_Result verifies the result event populates Usage.
func TestTapCCStream_Result(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"result","subtype":"success","usage":{"input_tokens":1234,"output_tokens":567,"cache_read_input_tokens":100},"session_id":"sess-abc","num_turns":3,"total_cost_usd":0.01}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	var found *trace.Event
	for i := range res.Events {
		if res.Events[i].Kind == trace.KindResult {
			found = &res.Events[i]
			break
		}
	}
	if found == nil {
		t.Fatal("no result event found")
	}
	if res.Usage.InputTokens != 1234 {
		t.Errorf("Usage.InputTokens = %d, want 1234", res.Usage.InputTokens)
	}
	if res.Usage.OutputTokens != 567 {
		t.Errorf("Usage.OutputTokens = %d, want 567", res.Usage.OutputTokens)
	}
	if !res.UsagePresent {
		t.Error("UsagePresent = false, want true (result event carried a usage block)")
	}
}

// TestTapCCStream_AllZeroUsageStillPresent verifies that a result event whose
// usage block is genuinely all-zero is still classified usage-present (MD-01 /
// D-01/D-03): UsagePresent must be a presence signal, not a value threshold, so
// a real claude run reporting input_tokens:0/output_tokens:0 is not mistaken for
// "no usage block".
func TestTapCCStream_AllZeroUsageStillPresent(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"result","subtype":"success","usage":{"input_tokens":0,"output_tokens":0},"session_id":"sess-zero"}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	if !res.UsagePresent {
		t.Error("UsagePresent = false for an all-zero usage block, want true (presence != value threshold)")
	}
}

// TestTapCCStream_NoUsageBlockAbsent verifies that a result event with NO usage
// block leaves UsagePresent false (the scripted/absent contract).
func TestTapCCStream_NoUsageBlockAbsent(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"result","subtype":"success","session_id":"sess-none"}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	if res.UsagePresent {
		t.Error("UsagePresent = true with no usage block, want false")
	}
}

// TestTapCCStream_SkipsStreamEvent verifies stream_event lines are skipped.
func TestTapCCStream_SkipsStreamEvent(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}}`+"\n"+
			`{"type":"result","subtype":"success","usage":{"input_tokens":10,"output_tokens":5}}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream error: %v", err)
	}
	// Only result event should appear, not stream_event
	for _, ev := range res.Events {
		if string(ev.Kind) == "stream_event" {
			t.Error("stream_event should have been skipped")
		}
	}
	// Should have exactly one event (result)
	resultCount := 0
	for _, ev := range res.Events {
		if ev.Kind == trace.KindResult {
			resultCount++
		}
	}
	if resultCount != 1 {
		t.Errorf("want 1 result event, got %d", resultCount)
	}
}

// TestTapCCStream_UnknownTypeWarns verifies unknown type lines are skipped
// with a counter increment and no error.
func TestTapCCStream_UnknownTypeWarns(t *testing.T) {
	t.Parallel()
	path := writeFile(t,
		`{"type":"unknown_future_type","data":"something"}`+"\n"+
			`{"type":"another_unknown","data":"something_else"}`+"\n"+
			`{"type":"result","subtype":"success","usage":{"input_tokens":10,"output_tokens":5}}`+"\n",
	)
	res, err := trace.TapCCStream(path)
	if err != nil {
		t.Fatalf("TapCCStream should not error on unknown types, got: %v", err)
	}
	if res.UnknownTypes != 2 {
		t.Errorf("UnknownTypes = %d, want 2", res.UnknownTypes)
	}
}

// TestTapDaemonLog_TimestampParsing verifies that time parsing works for RFC3339Nano.
func TestTapDaemonLog_TimestampParsing(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-10T08:30:01.812Z","level":"info","msg":"tool call","tool":"find_references","outcome":"success","duration_ms":23,"pid":12345}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(res.Events))
	}
	expected := time.Date(2026, 5, 10, 8, 30, 1, 812000000, time.UTC)
	if !res.Events[0].T.Equal(expected) {
		t.Errorf("T = %v, want %v", res.Events[0].T, expected)
	}
}

// TestTapDaemonLog_ReceiptIssued verifies F-08 receipt-issued lines parse into
// KindReceiptIssued events carrying ReceiptClass + TraceID + Pid.
func TestTapDaemonLog_ReceiptIssued(t *testing.T) {
	t.Parallel()
	path := writeLines(t,
		`{"time":"2026-05-12T10:00:00.000Z","level":"INFO","msg":"receipt issued","receipt_class":"definition","workspace":"/repo","snapshot_id":42,"graph_version":7,"trace_id":"trace-abc","pid":12345}`,
		`{"time":"2026-05-12T10:00:01.000Z","level":"INFO","msg":"receipt issued","receipt_class":"references","workspace":"/repo","snapshot_id":42,"graph_version":7,"trace_id":"trace-def","pid":99999}`,
		`{"time":"2026-05-12T10:00:02.000Z","level":"INFO","msg":"tool call","tool":"x","outcome":"success","duration_ms":1,"pid":12345}`,
	)
	res, err := trace.TapDaemonLog(path, 12345)
	if err != nil {
		t.Fatalf("TapDaemonLog error: %v", err)
	}
	// Expect: 1 receipt + 1 tool_call (foreign-pid receipt rejected).
	if len(res.Events) != 2 {
		t.Fatalf("want 2 events, got %d (events=%+v)", len(res.Events), res.Events)
	}
	if res.RejectedForeignPid != 1 {
		t.Errorf("RejectedForeignPid = %d, want 1", res.RejectedForeignPid)
	}
	var receipt *trace.Event
	for i := range res.Events {
		if res.Events[i].Kind == trace.KindReceiptIssued {
			receipt = &res.Events[i]
			break
		}
	}
	if receipt == nil {
		t.Fatalf("no KindReceiptIssued event in result")
	}
	if receipt.ReceiptClass != "definition" {
		t.Errorf("ReceiptClass = %q, want %q", receipt.ReceiptClass, "definition")
	}
	if receipt.TraceID != "trace-abc" {
		t.Errorf("TraceID = %q, want %q", receipt.TraceID, "trace-abc")
	}
	if receipt.Pid != 12345 {
		t.Errorf("Pid = %d, want 12345", receipt.Pid)
	}
}
