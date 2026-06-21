package trace

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// DaemonTapResult holds the parsed output from TapDaemonLog.
type DaemonTapResult struct {
	// Events contains all accepted tool-call events (pid-gated, msg-filtered).
	Events []Event
	// RejectedForeignPid is the count of events dropped because their pid did
	// not match the expected daemon pid (T-67-04 mitigation).
	RejectedForeignPid int
	// SkippedTruncated is the count of lines that could not be JSON-parsed
	// (e.g. truncated last line) and were dropped defensively.
	SkippedTruncated int
}

// daemonLogLine is the minimal struct for parsing a daemon JSONL log line.
// Covers both msg=="tool call" (F-07) and msg=="receipt issued" (F-08).
type daemonLogLine struct {
	Time       string         `json:"time"`
	Level      string         `json:"level"`
	Msg        string         `json:"msg"`
	Tool       string         `json:"tool"`
	Outcome    string         `json:"outcome"`
	DurationMs int64          `json:"duration_ms"`
	Pid        int            `json:"pid"`
	TraceID    string         `json:"trace_id"`
	SessionID  string         `json:"session_id"`
	Guardrail  *GuardrailInfo `json:"guardrail"`

	// F-08 receipt-issued fields.
	ReceiptClass string `json:"receipt_class"`
}

// TapDaemonLog reads a daemon JSONL log file and extracts tool-call events.
// Only lines with msg=="tool call" AND pid==expectedPid are accepted.
// Malformed or truncated lines are counted in SkippedTruncated, not errored.
// The expectedPid parameter gates against T-67-04 (foreign-pid injection).
func TapDaemonLog(path string, expectedPid int) (DaemonTapResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return DaemonTapResult{}, err
	}
	defer f.Close()

	var res DaemonTapResult
	scanner := bufio.NewScanner(f)
	// Allow larger tokens for daemon log lines (default 64KB may be too small).
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw daemonLogLine
		if err := json.Unmarshal(line, &raw); err != nil {
			res.SkippedTruncated++
			continue
		}

		// Dispatch on msg: "tool call" (F-07) or "receipt issued" (F-08).
		switch raw.Msg {
		case "tool call":
			// PID gate (T-67-04): reject events from foreign processes.
			if raw.Pid != expectedPid {
				res.RejectedForeignPid++
				continue
			}
			t := parseTimeDefensively(raw.Time)
			ev := Event{
				T:          t,
				Source:     "daemon",
				Kind:       KindToolCall,
				Tool:       raw.Tool,
				Outcome:    raw.Outcome,
				DurationMs: raw.DurationMs,
				TraceID:    raw.TraceID,
				Pid:        raw.Pid,
				Guardrail:  raw.Guardrail,
			}
			res.Events = append(res.Events, ev)
		case "receipt issued":
			// F-08: PID gate also applies — only receipts from the expected
			// daemon are counted.
			if raw.Pid != expectedPid {
				res.RejectedForeignPid++
				continue
			}
			t := parseTimeDefensively(raw.Time)
			ev := Event{
				T:            t,
				Source:       "daemon",
				Kind:         KindReceiptIssued,
				ReceiptClass: raw.ReceiptClass,
				TraceID:      raw.TraceID,
				Pid:          raw.Pid,
			}
			res.Events = append(res.Events, ev)
		default:
			continue
		}
	}

	// Check scanner error (real I/O errors) but NOT bufio.ErrFinalToken
	// which is used for truncated last lines — those are already counted.
	if err := scanner.Err(); err != nil {
		// If the scanner hit a buffer overflow or read error, return the error.
		return res, err
	}

	return res, nil
}

// parseTimeDefensively tries RFC3339Nano then RFC3339; returns time.Now() on failure.
// This defends against Pitfall 5 (wall-clock timestamp format drift).
func parseTimeDefensively(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	formats := []string{time.RFC3339Nano, time.RFC3339}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

// CCTapResult holds the parsed output from TapCCStream.
type CCTapResult struct {
	// Events contains typed events extracted from the CC stream-json output.
	Events []Event
	// Usage is populated from the "result" event's usage block.
	Usage Usage
	// UsagePresent is true when a "result" event actually carried a usage block
	// (env.Usage != nil). It is the out-of-band presence signal that lets
	// downstream token grading distinguish a genuine all-zero provider usage
	// from "no usage block at all" — Usage alone cannot (D-01/D-03).
	UsagePresent bool
	// UnknownTypes counts lines with an unrecognized "type" field (CC version drift).
	UnknownTypes int
	// FinalSessionID is the session_id from the "result" event.
	FinalSessionID string
}

// ccEnvelope is the outer envelope for all CC stream-json lines.
type ccEnvelope struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message *ccMessage      `json:"message"`
	Event   json.RawMessage `json:"event"`

	// system/init fields
	SessionID  string     `json:"session_id"`
	Model      string     `json:"model"`
	MCPServers []ccServer `json:"mcp_servers"`

	// result fields
	Usage     *ccUsage `json:"usage"`
	NumTurns  int      `json:"num_turns"`
}

// ccMessage is the message object within assistant / user lines.
type ccMessage struct {
	Content []ccContent `json:"content"`
}

// ccContent is a polymorphic content block (text | tool_use | tool_result).
type ccContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
}

// ccServer holds minimal MCP server info from the system/init event.
type ccServer struct {
	Name string `json:"name"`
}

// ccUsage holds token counts from the CC result event.
type ccUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// TapCCStream reads a Claude Code --output-format=stream-json output file and
// extracts typed events. Unknown types are counted in UnknownTypes, not errored.
// "stream_event" lines are always skipped. The "result" event populates Usage.
func TapCCStream(path string) (CCTapResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return CCTapResult{}, err
	}
	defer f.Close()

	var res CCTapResult
	// CC events can be large (full assistant message content); use a generous buffer.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	// Use time.Now as the base for events that lack a timestamp in CC stream-json.
	// CC stream-json doesn't include per-event timestamps in the outer envelope;
	// we assign time.Now() per-event as a best-effort wall-clock approximation.
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var env ccEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			// Malformed line: count and skip.
			res.UnknownTypes++
			continue
		}

		switch env.Type {
		case "stream_event":
			// Always skip per plan spec.
			continue

		case "system":
			if env.Subtype == "init" {
				servers := make([]string, 0, len(env.MCPServers))
				for _, s := range env.MCPServers {
					servers = append(servers, s.Name)
				}
				ev := Event{
					T:          time.Now().UTC(),
					Source:     "cc",
					Kind:       KindSessionInit,
					SessionID:  env.SessionID,
					Model:      env.Model,
					MCPServers: servers,
				}
				res.Events = append(res.Events, ev)
			} else if env.Subtype == "api_retry" {
				ev := Event{
					T:      time.Now().UTC(),
					Source: "cc",
					Kind:   KindAPIRetry,
				}
				res.Events = append(res.Events, ev)
			}
			// Other system subtypes: skip silently.

		case "assistant":
			ev := buildAssistantEvent(env.Message)
			res.Events = append(res.Events, ev)

		case "user":
			// User messages may contain tool results.
			if env.Message == nil {
				continue
			}
			for _, c := range env.Message.Content {
				if c.Type == "tool_result" {
					ev := Event{
						T:         time.Now().UTC(),
						Source:    "cc",
						Kind:      KindToolResult,
						ToolUseID: c.ToolUseID,
						IsError:   c.IsError,
					}
					res.Events = append(res.Events, ev)
				}
			}

		case "result":
			ev := Event{
				T:         time.Now().UTC(),
				Source:    "cc",
				Kind:      KindResult,
				SessionID: env.SessionID,
			}
			res.Events = append(res.Events, ev)
			if env.Usage != nil {
				res.Usage = Usage{
					InputTokens:         env.Usage.InputTokens,
					OutputTokens:        env.Usage.OutputTokens,
					CacheReadTokens:     env.Usage.CacheReadInputTokens,
					CacheCreationTokens: env.Usage.CacheCreationInputTokens,
				}
				res.UsagePresent = true // a provider usage block was parsed
			}
			res.FinalSessionID = env.SessionID

		default:
			res.UnknownTypes++
		}
	}

	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
}

// buildAssistantEvent constructs a KindAssistantMsg Event from a CC message.
func buildAssistantEvent(msg *ccMessage) Event {
	ev := Event{
		T:      time.Now().UTC(),
		Source: "cc",
		Kind:   KindAssistantMsg,
	}
	if msg == nil {
		return ev
	}
	for _, c := range msg.Content {
		switch c.Type {
		case "text":
			// Summarize: take first 256 chars of text as TextSummary.
			text := c.Text
			if len(text) > 256 {
				text = text[:256] + "..."
			}
			if ev.TextSummary == "" {
				ev.TextSummary = text
			}
		case "tool_use":
			ev.ToolUses = append(ev.ToolUses, ToolUse{
				ID:    c.ID,
				Name:  c.Name,
				Input: c.Input,
			})
		}
	}
	return ev
}
