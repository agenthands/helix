package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// hookInput represents the JSON piped to hook commands on stdin by Claude Code.
// See: https://code.claude.com/docs/en/hooks
type hookInput struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path"`
	CWD            string         `json:"cwd"`
	HookEventName  string         `json:"hook_event_name"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
}

// sessionStats tracks tool call counts per session for nudge threshold logic.
type sessionStats struct {
	SessionID       string `json:"session_id"`
	GrepReadCount   int    `json:"grep_read_count"`
	SerenaToolCount int    `json:"serena_tool_count"`
	LastUpdated     string `json:"last_updated"`
}

// newNudgeCommand creates the nudge subcommand invoked by Claude Code's PreToolUse hook.
func newNudgeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "nudge",
		Short:         "Nudge agent toward symbolic tools",
		Long:          "Reads hook input from stdin (Claude Code PreToolUse), tracks tool call counters, and outputs a nudge message when agents overuse grep/read without trying symbolic tools.",
		RunE:          runNudge,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().String("tool", "", "Tool name (fallback if stdin not available)")

	return cmd
}

// runNudge reads stdin JSON from Claude Code, tracks grep/read counts,
// and outputs a nudge message after 5+ calls without symbolic tool usage.
// Always exits 0 (advisory only, per D-11).
func runNudge(cmd *cobra.Command, _ []string) error {
	// Read stdin JSON into hookInput via json.Decoder (safe parser per T-36-01).
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		// If stdin fails (manual invocation), return nil silently.
		return nil
	}

	// Get toolName from input; fall back to --tool flag.
	toolName := input.ToolName
	if toolName == "" {
		toolName, _ = cmd.Flags().GetString("tool")
	}

	// Get workspace directory from input; fall back to cwd.
	wsDir := input.CWD
	if wsDir == "" {
		var err error
		wsDir, err = os.Getwd()
		if err != nil {
			return nil // Non-fatal: can't determine workspace.
		}
	}

	// Resolve to absolute path per T-36-03.
	wsDir, _ = filepath.Abs(wsDir)

	statsPath := filepath.Join(wsDir, ".serena", "session-stats.json")

	// Load stats for the current session.
	stats := loadSessionStats(statsPath, input.SessionID)

	// Check if this is a Serena symbolic tool call.
	if isSerenaSymbolicTool(toolName) {
		stats.SerenaToolCount++
		stats.GrepReadCount = 0 // Reset -- agent is using symbolic tools (D-12).
		_ = saveSessionStats(statsPath, stats)
		return nil
	}

	// Check if this is a grep/read tool call.
	if isGrepReadTool(toolName, input.ToolInput) {
		stats.GrepReadCount++
		_ = saveSessionStats(statsPath, stats)

		// Check threshold (D-10).
		if stats.GrepReadCount >= 5 && stats.SerenaToolCount == 0 {
			fmt.Println("Tip: Serena provides find_symbol and get_symbols_overview for code navigation. These give you precise symbol locations, references, and type hierarchies instead of text pattern matching with grep.")
		}
		return nil
	}

	// Other tool -- just save and return.
	_ = saveSessionStats(statsPath, stats)
	return nil
}

// loadSessionStats reads session stats from the given path.
// If the file is missing, unreadable, or for a different session, returns fresh stats.
func loadSessionStats(path string, sessionID string) sessionStats {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionStats{SessionID: sessionID}
	}

	var stats sessionStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return sessionStats{SessionID: sessionID}
	}

	// Different session = fresh stats.
	if stats.SessionID != sessionID {
		return sessionStats{SessionID: sessionID}
	}

	return stats
}

// saveSessionStats writes session stats atomically using a temp file + rename
// pattern per RESEARCH Pitfall 3 to avoid corruption from concurrent hook invocations.
func saveSessionStats(path string, stats sessionStats) error {
	stats.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session stats: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating stats directory: %w", err)
	}

	// Atomic write: write to temp file, then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing temp stats file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming stats file: %w", err)
	}

	return nil
}

// serenaSymbolicTools lists Serena MCP tool names that indicate symbolic tool usage.
var serenaSymbolicTools = map[string]bool{
	"find_symbol":          true,
	"get_symbol_details":   true,
	"get_symbols_overview": true,
	"find_references":      true,
	"get_hover_info":       true,
	"find_implementations": true,
	"get_call_hierarchy":   true,
	"get_type_hierarchy":   true,
	"get_blast_radius":     true,
}

// isSerenaSymbolicTool returns true if the tool name matches a Serena symbolic tool,
// either with or without the "mcp__serena__" prefix.
func isSerenaSymbolicTool(name string) bool {
	if serenaSymbolicTools[name] {
		return true
	}
	// Strip "mcp__serena__" prefix for flexibility.
	const prefix = "mcp__serena__"
	if strings.HasPrefix(name, prefix) {
		return serenaSymbolicTools[strings.TrimPrefix(name, prefix)]
	}
	return false
}

// isGrepReadTool returns true if the tool is Grep, Read, or Bash running a grep-like command.
// Never executes values from stdin -- only reads ToolName/ToolInput as data (per T-36-01).
func isGrepReadTool(name string, input map[string]any) bool {
	switch name {
	case "Grep", "Read":
		return true
	case "Bash":
		cmd, _ := input["command"].(string)
		return strings.Contains(cmd, "grep") ||
			strings.Contains(cmd, "find") ||
			strings.Contains(cmd, "rg") ||
			strings.Contains(cmd, "ag")
	}
	return false
}
