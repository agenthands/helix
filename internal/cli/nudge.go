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
	HelixToolCount int    `json:"helix_tool_count"`
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

	statsPath := filepath.Join(wsDir, ".helix", "session-stats.json")

	// Load stats for the current session.
	stats := loadSessionStats(statsPath, input.SessionID)

	// Check if this is a Helix symbolic tool call.
	if isHelixSymbolicTool(toolName) {
		stats.HelixToolCount++
		stats.GrepReadCount = 0 // Reset -- agent is using symbolic tools (D-12).
		_ = saveSessionStats(statsPath, stats)
		return nil
	}

	// Check if this is a grep/read tool call.
	if isGrepReadTool(toolName, input.ToolInput) {
		stats.GrepReadCount++
		_ = saveSessionStats(statsPath, stats)

		// Per-call advisory steer toward the frozen helix verbs. Fail-open:
		// only emit when we can positively justify the suggestion. Always exit 0
		// regardless (advisory only, never blocks — T-93-04 / D-11).
		if advisory, emit := steerMessage(toolName, input.ToolInput); emit {
			emitAdvisory(advisory)
		}
		return nil
	}

	// Other tool -- just save and return.
	_ = saveSessionStats(statsPath, stats)
	return nil
}

// preToolUseOutput is the Claude Code PreToolUse hook output envelope. Emitting
// it on stdout with exit 0 surfaces AdditionalContext into the agent's context
// as an ADVISORY (exit 2 would instead make Claude Code treat it as a blocking
// error and ignore the JSON — so exit 0 is mandatory). See:
// https://code.claude.com/docs/en/hooks
type preToolUseOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// emitAdvisory marshals an advisory message into the PreToolUse output envelope
// and prints it to stdout. It NEVER string-concatenates JSON (T-34-01 spirit)
// and never returns an error — a marshal failure simply emits nothing (the
// nudge stays silent rather than blocking).
func emitAdvisory(text string) {
	var out preToolUseOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.AdditionalContext = text

	data, err := json.Marshal(out)
	if err != nil {
		return // stay silent; never block
	}
	fmt.Println(string(data))
}

// steerMessage maps a grep/read tool call to an advisory `helix <verb>`
// substitution. Returns emit=false when no suggestion is warranted (fail-open).
//
// For the Grep/Read TOOLS it always suggests the symbol-aware equivalents. For
// Bash it classifies the command's file operand via classifyBashTarget and only
// suggests on a positively-identified CODE target; prose/log/config targets,
// no-operand commands, and unparseable shapes stay silent.
func steerMessage(toolName string, input map[string]any) (msg string, emit bool) {
	switch toolName {
	case "Grep":
		return "Tip: for code, `helix search-symbols --query=<name>` finds symbol declarations and " +
			"`helix search-in-files --pattern=<pat>` does content search — both terser and symbol-aware than grep.", true
	case "Read":
		return "Tip: `helix read-file --path=<file>` reads a file and `helix get-symbol-overview` shows a " +
			"file's symbol outline — symbol-aware alternatives to a raw Read.", true
	case "Bash":
		cmd, _ := input["command"].(string)
		isCode, ok := classifyBashTarget(cmd)
		if !ok || !isCode {
			return "", false // fail-open: non-code, no-operand, or unparseable → silent
		}
		return bashSteerMessage(cmd), true
	}
	return "", false
}

// bashSteerMessage builds the advisory text for a Bash command already known to
// target a CODE file. It maps the command shape to the closest frozen helix
// verb (93-PATTERNS.md steer table). The command string is read as DATA only.
func bashSteerMessage(cmd string) string {
	fields := strings.Fields(cmd)
	tool := ""
	if len(fields) > 0 {
		tool = fields[0]
	}

	switch tool {
	case "find":
		return "Tip: `helix find-files --pattern='**/*.ext'` lists files structurally — an alternative to `find -name`."
	case "cat":
		return "Tip: `helix read-file --path=<file>` reads a file and `helix get-symbol-overview` shows its symbol " +
			"outline — symbol-aware alternatives to `cat`."
	case "sed":
		if strings.Contains(cmd, "-i") {
			return "Tip: `helix replace-in-file` / `helix replace-symbol-body` edit code structurally — safer than `sed -i`."
		}
		return "Tip: `helix read-file --path=<file>` reads a file (or a range) — an alternative to `sed -n`."
	default: // grep / rg / ag / egrep / fgrep
		if strings.Contains(cmd, "-r") || strings.Contains(cmd, "-R") {
			return "Tip: `helix find-references` / `helix get-call-hierarchy` find callers semantically, and " +
				"`helix search-in-files --pattern=<pat>` does code-aware content search — alternatives to recursive grep."
		}
		return "Tip: `helix search-symbols --query=<name>` finds symbol declarations and " +
			"`helix search-in-files --pattern=<pat>` does content search — symbol-aware alternatives to grep."
	}
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

// helixSymbolicTools lists Helix MCP tool names that indicate symbolic tool usage.
var helixSymbolicTools = map[string]bool{
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

// isHelixSymbolicTool returns true if the tool name matches a Helix symbolic tool,
// either with or without the "mcp__helix__" prefix.
func isHelixSymbolicTool(name string) bool {
	if helixSymbolicTools[name] {
		return true
	}
	// Strip "mcp__helix__" prefix for flexibility.
	const prefix = "mcp__helix__"
	if strings.HasPrefix(name, prefix) {
		return helixSymbolicTools[strings.TrimPrefix(name, prefix)]
	}
	return false
}

// codeExtensions is a small static allowlist of source-code file extensions.
// Used by classifyBashTarget to positively identify a CODE target without
// constructing a langregistry.Registry on every hook call (hot path, T-93-05).
// It mirrors the spirit of langregistry.ByExtension but stays allocation-free.
var codeExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rb": true, ".rs": true, ".java": true, ".kt": true,
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true,
	".cs": true, ".php": true, ".swift": true, ".scala": true, ".m": true,
	".mm": true, ".lua": true, ".dart": true, ".ex": true, ".exs": true,
	".clj": true, ".hs": true, ".ml": true, ".r": true, ".sh": true,
	".bash": true, ".zig": true, ".vue": true, ".svelte": true,
}

// nonCodeExtensions classifies prose/log/config targets as explicitly NON-code.
// A target with one of these extensions (or a recognized no-extension config
// filename like Dockerfile) yields isCode=false, ok=true so the caller can stay
// silent (fail-open by intent) per RESEARCH Pitfall 2.
var nonCodeExtensions = map[string]bool{
	".md": true, ".markdown": true, ".log": true, ".txt": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".ini": true, ".cfg": true, ".conf": true, ".csv": true,
	".html": true, ".htm": true, ".css": true, ".xml": true,
	".lock": true, ".env": true, ".rst": true,
}

// nonCodeBasenames classifies extensionless prose/config files as NON-code.
var nonCodeBasenames = map[string]bool{
	"Dockerfile": true, "Makefile": true, "LICENSE": true,
	"README": true, "CHANGELOG": true, ".gitignore": true,
}

// classifyBashTarget tokenizes a Bash command string as DATA (never executing
// it, T-36-01) and classifies its file/path operand(s) as code vs non-code.
//
// Returns:
//   - (true, true)   when at least one file operand was found and ALL recognized
//     file operands are code files (conservative: a single non-code operand
//     demotes the whole command to non-code, since a false suggestion is the
//     failure mode to avoid).
//   - (false, true)  when file operand(s) were found but at least one is
//     positively non-code (prose/log/config) — caller stays silent.
//   - (_, false)     when no file operand can be identified or the command
//     cannot be tokenized into a recognizable grep/sed/cat/find shape
//     (fail-open: caller emits nothing).
//
// It NEVER imports os/exec and never runs the command string. Tokenization is a
// bounded whitespace split (no regex backtracking on attacker input, T-93-05).
func classifyBashTarget(cmd string) (isCode bool, ok bool) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false, false
	}

	// Recognize only grep/sed/cat/find-shaped read tools; anything else fails open.
	switch fields[0] {
	case "grep", "rg", "ag", "sed", "cat", "find", "egrep", "fgrep":
		// recognized read/search tool
	default:
		return false, false
	}

	sawCode := false
	sawNonCode := false
	sawAnyOperand := false

	for _, tok := range fields[1:] {
		// Skip option flags (leading '-'). This also skips grep patterns that
		// happen to start with '-' via -e, which is acceptable (we only need
		// to find file operands, not patterns).
		if strings.HasPrefix(tok, "-") {
			continue
		}

		// For `find . -name '*.go'` the operand carrying the extension is the
		// glob value, e.g. '*.go' (quotes stripped by the JSON/shell layer, but
		// strip residual quotes defensively as DATA).
		tok = strings.Trim(tok, `'"`)
		if tok == "" {
			continue
		}

		// A file/path operand is a token containing a path separator OR a
		// recognizable extension. Pure patterns (no '/', no '.ext') are skipped.
		base := filepath.Base(tok)
		ext := filepath.Ext(tok)

		switch {
		case ext != "" && codeExtensions[ext]:
			sawCode = true
			sawAnyOperand = true
		case ext != "" && nonCodeExtensions[ext]:
			sawNonCode = true
			sawAnyOperand = true
		case ext == "" && nonCodeBasenames[base]:
			sawNonCode = true
			sawAnyOperand = true
		case strings.ContainsRune(tok, filepath.Separator) && ext == "":
			// A bare directory/path operand with no extension (e.g. `internal/cli`)
			// is not a positively-identified code FILE; treat as no signal.
			// (Do not count it as an operand so a pure-dir grep fails open.)
		default:
			// Token with an unknown extension or no extension and no separator:
			// not a recognizable code or non-code file operand. Ignore it.
		}
	}

	if !sawAnyOperand {
		return false, false // no file operand → fail open
	}
	if sawNonCode {
		return false, true // any non-code operand → conservative non-code
	}
	if sawCode {
		return true, true
	}
	return false, false
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
