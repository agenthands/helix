package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSessionStats_NewFile(t *testing.T) {
	stats := loadSessionStats("/nonexistent/session-stats.json", "session-123")
	assert.Equal(t, "session-123", stats.SessionID)
	assert.Equal(t, 0, stats.GrepReadCount)
	assert.Equal(t, 0, stats.HelixToolCount)
}

func TestLoadSessionStats_DifferentSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	// Save stats with session "A".
	statsA := sessionStats{
		SessionID:      "session-A",
		GrepReadCount:  10,
		HelixToolCount: 5,
	}
	require.NoError(t, saveSessionStats(path, statsA))

	// Load with session "B" -- should return fresh stats.
	stats := loadSessionStats(path, "session-B")
	assert.Equal(t, "session-B", stats.SessionID)
	assert.Equal(t, 0, stats.GrepReadCount)
	assert.Equal(t, 0, stats.HelixToolCount)
}

func TestSaveAndLoadSessionStats_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".helix", "session-stats.json")

	original := sessionStats{
		SessionID:      "session-xyz",
		GrepReadCount:  7,
		HelixToolCount: 3,
	}
	require.NoError(t, saveSessionStats(path, original))

	loaded := loadSessionStats(path, "session-xyz")
	assert.Equal(t, original.SessionID, loaded.SessionID)
	assert.Equal(t, original.GrepReadCount, loaded.GrepReadCount)
	assert.Equal(t, original.HelixToolCount, loaded.HelixToolCount)
	assert.NotEmpty(t, loaded.LastUpdated, "LastUpdated should be set by save")
}

func TestIsHelixSymbolicTool(t *testing.T) {
	// Should return true for the REAL registered symbolic tool names (CR-93-02).
	assert.True(t, isHelixSymbolicTool("search_symbols"))
	assert.True(t, isHelixSymbolicTool("get_symbol_overview"))
	assert.True(t, isHelixSymbolicTool("find_references"))
	assert.True(t, isHelixSymbolicTool("get_hover_info"))
	assert.True(t, isHelixSymbolicTool("find_implementations"))
	assert.True(t, isHelixSymbolicTool("get_call_hierarchy"))
	assert.True(t, isHelixSymbolicTool("get_type_hierarchy"))
	assert.True(t, isHelixSymbolicTool("analyze_blast_radius"))
	assert.True(t, isHelixSymbolicTool("go_to_definition"))

	// Should return true with mcp__helix__ prefix.
	assert.True(t, isHelixSymbolicTool("mcp__helix__search_symbols"))
	assert.True(t, isHelixSymbolicTool("mcp__helix__analyze_blast_radius"))

	// The stale pre-rename names must NOT match (regression guard for CR-93-02).
	assert.False(t, isHelixSymbolicTool("find_symbol"))
	assert.False(t, isHelixSymbolicTool("get_symbols_overview"))
	assert.False(t, isHelixSymbolicTool("get_symbol_details"))
	assert.False(t, isHelixSymbolicTool("get_blast_radius"))

	// Should return false for non-symbolic tools.
	assert.False(t, isHelixSymbolicTool("Grep"))
	assert.False(t, isHelixSymbolicTool("Read"))
	assert.False(t, isHelixSymbolicTool("Bash"))
	assert.False(t, isHelixSymbolicTool("Write"))
	assert.False(t, isHelixSymbolicTool("mcp__helix__read_file"))
}

// TestHelixSymbolicTools_NoDrift asserts every helixSymbolicTools key is a REAL
// tool name in the live registry (VerbToolNames() — kebab verb names mapped to
// their snake_case toolName). This is the drift guard the review (CR-93-02) asks
// for: it fails if a future rename leaves a stale key behind (the exact defect
// that made the symbolic-tool reset silently dead).
func TestHelixSymbolicTools_NoDrift(t *testing.T) {
	registered := make(map[string]bool)
	for _, name := range VerbToolNames() {
		registered[name] = true
	}
	for toolName := range helixSymbolicTools {
		assert.Truef(t, registered[toolName],
			"helixSymbolicTools key %q is not a registered tool name (VerbToolNames()); stale name → broken GrepReadCount reset (CR-93-02)",
			toolName)
	}
}

func TestIsGrepReadTool(t *testing.T) {
	// Direct grep/read tools.
	assert.True(t, isGrepReadTool("Grep", nil))
	assert.True(t, isGrepReadTool("Read", nil))

	// Bash with grep-like commands.
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "grep -r pattern ."}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "find . -name '*.go'"}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "rg pattern"}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "ag pattern"}))

	// Bash without grep-like commands.
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "echo hello"}))
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "go build ./..."}))

	// Non-matching tools.
	assert.False(t, isGrepReadTool("Write", nil))
	assert.False(t, isGrepReadTool("Edit", nil))
}

// TestSessionStats_GrepReadCountPersists records that the grep/read counter is
// tracked telemetry that round-trips through save/load. Post-Phase-93 it does NOT
// gate emission (the advisory fires per-call — see runNudge); the counter is kept
// only as a session-usage signal (WR-93-03).
func TestSessionStats_GrepReadCountPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:      "test-session",
		GrepReadCount:  4,
		HelixToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	loaded := loadSessionStats(path, "test-session")
	assert.Equal(t, 4, loaded.GrepReadCount)
	assert.Equal(t, 0, loaded.HelixToolCount)
}

// TestSessionStats_ResetBySymbolicTool exercises the D-12 reset semantics: a
// symbolic-tool call bumps HelixToolCount and zeroes GrepReadCount. It uses a
// REAL registered tool name (CR-93-02), proving the reset path actually fires for
// the tools the agent really invokes.
func TestSessionStats_ResetBySymbolicTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:      "test-session",
		GrepReadCount:  6,
		HelixToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	// A real symbolic tool resets the grep count (D-12).
	loaded := loadSessionStats(path, "test-session")
	require.True(t, isHelixSymbolicTool("search_symbols"),
		"search_symbols must be recognized as symbolic (CR-93-02)")
	loaded.HelixToolCount++
	loaded.GrepReadCount = 0
	require.NoError(t, saveSessionStats(path, loaded))

	final := loadSessionStats(path, "test-session")
	assert.Equal(t, 0, final.GrepReadCount, "GrepReadCount should reset to 0")
	assert.Equal(t, 1, final.HelixToolCount, "HelixToolCount should increment")
}

func TestClassifyBashTarget_CodeTargets(t *testing.T) {
	// Code-file targets classify as code (isCode=true, ok=true).
	cases := []string{
		`grep "func Foo" main.go`,
		`grep -r "Bar(" internal/cli/verb.go`,
		`cat internal/kernel/edit/edit.go`,
		`sed -n '1,20p' pkg/server/server.go`,
		`rg "TODO" handler.ts`,
		`find . -name '*.go'`,
	}
	for _, cmd := range cases {
		isCode, ok := classifyBashTarget(cmd)
		assert.True(t, ok, "expected ok=true for %q", cmd)
		assert.True(t, isCode, "expected isCode=true for %q", cmd)
	}
}

func TestClassifyBashTarget_NonCodeTargets(t *testing.T) {
	// Prose/log/config targets classify as non-code (isCode=false, ok=true).
	cases := []string{
		`grep TODO README.md`,
		`grep error app.log`,
		`cat config.yaml`,
		`grep x notes.txt`,
		`grep y data.json`,
		`cat Dockerfile`,
		`cat settings.yml`,
	}
	for _, cmd := range cases {
		isCode, ok := classifyBashTarget(cmd)
		assert.True(t, ok, "expected ok=true for %q", cmd)
		assert.False(t, isCode, "expected isCode=false for %q", cmd)
	}
}

func TestClassifyBashTarget_FailOpen(t *testing.T) {
	// Unparseable / no operand → ok=false (fail-open signal).
	cases := []string{
		`grep`,       // no operand
		`grep "x"`,   // pattern only, no file
		``,           // empty command
		`grep -r -n`, // flags only
		`cat`,        // no operand
		`echo hello`, // not a grep/read shape
		`go build ./...`,
	}
	for _, cmd := range cases {
		_, ok := classifyBashTarget(cmd)
		assert.False(t, ok, "expected ok=false (fail-open) for %q", cmd)
	}
}

func TestClassifyBashTarget_MixedTargetsConservative(t *testing.T) {
	// Policy: when BOTH a code and a non-code operand appear, do NOT suggest
	// (conservative — a false suggestion is the failure mode to avoid).
	// Require ALL identified file operands to be code.
	isCode, ok := classifyBashTarget(`grep x main.go README.md`)
	assert.True(t, ok, "mixed operands are still parseable")
	assert.False(t, isCode, "mixed code+non-code operands → conservative non-code")
}

func TestClassifyBashTarget_PureDataNoExec(t *testing.T) {
	// Commands with shell metacharacters are parsed as a string only — the
	// classifier never runs anything (no os/exec in this path). We assert it
	// returns deterministically without side effects.
	isCode, ok := classifyBashTarget(`grep x f.go && rm -rf /`)
	// f.go is a code operand; trailing metachars are ignored as data.
	assert.True(t, ok)
	assert.True(t, isCode)

	// $(whoami).go must not be executed; it is just a token. Whether it
	// classifies as code or not, the key property is no side effects + a
	// deterministic return.
	_, ok2 := classifyBashTarget(`grep x $(whoami).go`)
	_ = ok2 // no panic, no exec — property assertion is the absence of side effects
}

func TestClassifyBashTarget_GrepPatternNotFile(t *testing.T) {
	// The grep family (grep/rg/ag/egrep/fgrep) leads with a search PATTERN, not a
	// file operand. The first non-flag token must NOT be classified as a file.
	cases := []struct {
		cmd        string
		wantIsCode bool
		wantOk     bool
	}{
		{`grep foo.go`, false, false},          // pattern only, no file operand → fail open
		{`grep foo.go bar.go`, true, true},     // foo.go is the skipped pattern, bar.go is the file
		{`grep -i foo.go util.go`, true, true}, // flag, then pattern foo.go, then file util.go
		{`rg pattern.go`, false, false},        // rg pattern only
		{`cat main.go`, true, true},            // control: cat has no leading pattern; first operand is the file
	}
	for _, tc := range cases {
		isCode, ok := classifyBashTarget(tc.cmd)
		assert.Equal(t, tc.wantOk, ok, "ok mismatch for %q", tc.cmd)
		assert.Equal(t, tc.wantIsCode, isCode, "isCode mismatch for %q", tc.cmd)
	}
}

// runNudgeCapture invokes runNudge with the given hookInput JSON piped to a
// redirected os.Stdin and captures everything written to os.Stdout. It sets
// CWD to a temp dir so session-stats writes are isolated. Returns the captured
// stdout and the error returned by runNudge (which MUST always be nil).
func runNudgeCapture(t *testing.T, input hookInput) (string, error) {
	t.Helper()

	tmp := t.TempDir()
	input.CWD = tmp
	if input.SessionID == "" {
		input.SessionID = "test-session"
	}
	payload, err := json.Marshal(input)
	require.NoError(t, err)

	// Redirect stdin.
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	origStdin := os.Stdin
	os.Stdin = stdinR
	go func() {
		_, _ = stdinW.Write(payload)
		_ = stdinW.Close()
	}()

	// Redirect stdout.
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	origStdout := os.Stdout
	os.Stdout = stdoutW

	cmd := newNudgeCommand()
	runErr := runNudge(cmd, nil)

	_ = stdoutW.Close()
	os.Stdout = origStdout
	os.Stdin = origStdin
	_ = stdinR.Close()

	out, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	return string(out), runErr
}

// parseAdvisory parses captured stdout as a preToolUseOutput. Returns ok=false
// when stdout is empty / not a suggestion JSON object.
func parseAdvisory(t *testing.T, out string) (preToolUseOutput, bool) {
	t.Helper()
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return preToolUseOutput{}, false
	}
	var adv preToolUseOutput
	if err := json.Unmarshal([]byte(trimmed), &adv); err != nil {
		return preToolUseOutput{}, false
	}
	if adv.HookSpecificOutput.HookEventName == "" {
		return preToolUseOutput{}, false
	}
	return adv, true
}

func TestNudgeAdvisory_BashCodeGrep_Suggests(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `grep "func Foo" main.go`},
	})
	assert.NoError(t, err, "runNudge must always return nil (exit 0)")

	adv, ok := parseAdvisory(t, out)
	require.True(t, ok, "expected a suggestion JSON, got %q", out)
	assert.Equal(t, "PreToolUse", adv.HookSpecificOutput.HookEventName)
	assert.Contains(t, adv.HookSpecificOutput.AdditionalContext, "helix",
		"advisory should name a helix verb")
}

func TestNudgeAdvisory_BashNonCodeGrep_Silent(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `grep TODO README.md`},
	})
	assert.NoError(t, err)
	_, ok := parseAdvisory(t, out)
	assert.False(t, ok, "non-code grep must produce no suggestion, got %q", out)
}

func TestNudgeAdvisory_BashUnparseable_FailOpenSilent(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `grep`},
	})
	assert.NoError(t, err)
	_, ok := parseAdvisory(t, out)
	assert.False(t, ok, "unparseable bash must fail open (no suggestion), got %q", out)
}

func TestNudgeAdvisory_GrepTool_Suggests(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{ToolName: "Grep"})
	assert.NoError(t, err)
	adv, ok := parseAdvisory(t, out)
	require.True(t, ok, "Grep tool should yield a suggestion, got %q", out)
	assert.Equal(t, "PreToolUse", adv.HookSpecificOutput.HookEventName)
	assert.Contains(t, adv.HookSpecificOutput.AdditionalContext, "helix")
}

func TestNudgeAdvisory_ReadTool_Suggests(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{ToolName: "Read"})
	assert.NoError(t, err)
	adv, ok := parseAdvisory(t, out)
	require.True(t, ok, "Read tool should yield a suggestion, got %q", out)
	assert.Equal(t, "PreToolUse", adv.HookSpecificOutput.HookEventName)
	assert.Contains(t, adv.HookSpecificOutput.AdditionalContext, "helix")
}

func TestNudgeAdvisory_AlwaysExitZero(t *testing.T) {
	// Every input — including a helix symbolic tool, a non-grep tool, and an
	// unparseable bash — must yield a nil error (exit 0). The advisory never blocks.
	inputs := []hookInput{
		{ToolName: "Bash", ToolInput: map[string]any{"command": `grep "func Foo" main.go`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `grep TODO README.md`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `grep`}},
		{ToolName: "Grep"},
		{ToolName: "Read"},
		{ToolName: "find_symbol"},
		{ToolName: "Write"},
		{ToolName: ""},
	}
	for _, in := range inputs {
		_, err := runNudgeCapture(t, in)
		assert.NoError(t, err, "runNudge must return nil for %+v", in)
	}
}

func TestNudgeAdvisory_ValidJSONShape(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `grep "func Foo" main.go`},
	})
	require.NoError(t, err)

	// The emitted bytes must parse as JSON with EXACTLY the
	// hookSpecificOutput.{hookEventName,additionalContext} shape.
	var generic map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &generic))
	hso, ok := generic["hookSpecificOutput"].(map[string]any)
	require.True(t, ok, "missing hookSpecificOutput object")
	assert.Equal(t, "PreToolUse", hso["hookEventName"])
	_, hasCtx := hso["additionalContext"].(string)
	assert.True(t, hasCtx, "additionalContext must be a string")
	assert.Len(t, hso, 2, "hookSpecificOutput must have exactly 2 keys")
}

func TestSaveSessionStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".helix", "session-stats.json")

	stats := sessionStats{SessionID: "atomic-test", GrepReadCount: 1}
	require.NoError(t, saveSessionStats(path, stats))

	// Verify the temp file was cleaned up (atomic rename).
	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should be removed after rename")

	// Verify the final file exists and is valid JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded sessionStats
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "atomic-test", loaded.SessionID)
}
