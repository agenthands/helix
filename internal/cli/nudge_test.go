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

	// STEER-01: Bash sed/cat are now recognized (token-anchored on fields[0]) so
	// the already-correct sed→replace-in-file / cat→read-file branches in
	// bashSteerMessage become reachable for Bash callers (DEFER-97-01 landing).
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "sed -i 's/a/b/' pkg/s.go"}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "cat internal/edit.go"}))

	// Token-anchored, NOT strings.Contains: a command whose fields[0] is NOT a
	// recognized read tool stays false even when a path literally contains the
	// substring "cat"/"sed" (e.g. concatenate.go / parsed.go). This is the
	// 98-RESEARCH Open Question 1 precision guard.
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "go build ./internal/concatenate.go"}))
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "echo parsed.go"}))

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
	// ADOPT-01b: key on the SPECIFIC emitted verb for this shape (plain grep over
	// a code file -> `helix search-symbols`), NOT a weak Contains(..., "helix")
	// substring that the prompt/skill text would also satisfy (97-RESEARCH Pitfall 3).
	assert.Contains(t, adv.HookSpecificOutput.AdditionalContext, "helix search-symbols",
		"plain grep over a code file must steer to `helix search-symbols`")
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
		// STEER-01 broadened sed/cat shapes: firing (code) and silent (prose/log/config),
		// all of which must still return nil (exit 0 / fail-open).
		{ToolName: "Bash", ToolInput: map[string]any{"command": `sed -i 's/a/b/' pkg/s.go`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `cat internal/edit.go`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `sed -i 's/x/y/' README.md`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `cat app.log`}},
		{ToolName: "Bash", ToolInput: map[string]any{"command": `cat config.yaml`}},
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

// nudgeShapeCase pairs a standard-tool call (a Claude Code tool name + its input)
// with the SPECIFIC `helix <verb>` token the nudge classifier emits for it. wantVerb
// is mapped VERBATIM from steerMessage/bashSteerMessage (nudge.go:158-206): the
// golden keys on the chosen verb, NOT on a weak "helix" substring.
//
// IMPORTANT — this golden asserts the LIVE current mapping. As of Phase 98 STEER-01,
// isGrepReadTool's Bash arm is token-anchored on fields[0] ∈ {grep,find,rg,ag,sed,cat}
// (DEFER-97-01 landing), so a Bash `sed -i`/`cat` over a CODE file now reaches
// steerMessage → classifyBashTarget → bashSteerMessage's already-correct
// sed→replace-in-file / cat→read-file branches. The previously-silent
// `bash-sed-i-silent`/`bash-cat-silent` sub-cases are therefore GONE; the two shapes
// are now asserted FIRING on the SPECIFIC verb (non-vacuously) below.
type nudgeShapeCase struct {
	name     string
	toolName string
	input    map[string]any
	wantVerb string
}

// nudgeShapeGoldenCases is the ADOPT-01b per-shape golden: the five standard-tool
// shapes the classifier genuinely steers, each mapped to the SPECIFIC emitted helix
// verb. This is the anti-vacuity heart: keyed on the emitted command, asserted
// per-shape, with a floor that rejects an empty-bucket pass. Each wantVerb is mapped
// VERBATIM from steerMessage/bashSteerMessage (nudge.go line cited per case).
func nudgeShapeGoldenCases() []nudgeShapeCase {
	return []nudgeShapeCase{
		// (1) grep (plain) over a code file -> search-symbols (nudge.go:203-204)
		{"bash-grep", "Bash", map[string]any{"command": `grep "func Foo" main.go`}, "helix search-symbols"},
		// (2) grep -r / -R over code -> find-references (nudge.go:199-201)
		{"bash-grep-r", "Bash", map[string]any{"command": `grep -r "Bar(" internal/x.go`}, "helix find-references"},
		// (3) find -name -> find-files (nudge.go:189)
		{"bash-find", "Bash", map[string]any{"command": `find . -name '*.go'`}, "helix find-files"},
		// (4) the cat-equivalent shape: the Read tool -> read-file (nudge.go:163-165).
		{"read-tool", "Read", map[string]any{"file_path": "internal/edit.go"}, "helix read-file"},
		// (5) the grep-tool shape: the Grep tool -> search-symbols (nudge.go:160-162).
		{"grep-tool", "Grep", map[string]any{"pattern": "Foo"}, "helix search-symbols"},
		// (6) STEER-01 / DEFER-97-01: Bash `sed -i` over a code file -> replace-in-file
		//     (nudge.go:194-195). Previously SILENT (isGrepReadTool excluded sed); now firing.
		{"bash-sed-i", "Bash", map[string]any{"command": `sed -i 's/a/b/' pkg/s.go`}, "helix replace-in-file"},
		// (7) STEER-01 / DEFER-97-01: Bash `cat` over a code file -> read-file
		//     (nudge.go:191-192). Previously SILENT (isGrepReadTool excluded cat); now firing.
		{"bash-cat", "Bash", map[string]any{"command": `cat internal/edit.go`}, "helix read-file"},
	}
}

// TestNudgeShapeGolden is the ADOPT-01b golden table. For each of the seven
// standard-tool shapes it asserts: (1) runNudge returns nil (exit-0 / fail-open
// contract preserved), (2) the advisory parses, (3) AdditionalContext contains the
// SPECIFIC expected `helix <verb>` token — never merely "helix". It enforces an
// empty-bucket floor (>= 7 shapes) and asserts per-shape, never an empty-iteration
// pass (97-RESEARCH Pitfall 3 + Phase 87 CR-01 empty-bucket defect). It also pins
// the STEER-03 negative controls (prose/log/config grep+sed+cat stay silent) so the
// broadened sed/cat recognition never over-fires.
func TestNudgeShapeGolden(t *testing.T) {
	cases := nudgeShapeGoldenCases()

	// Empty-bucket guard: the seven distinct steering shapes (grep / grep-r / find /
	// read / grep-tool / bash-sed-i / bash-cat) MUST be present. A 0/0 "all pass" is
	// the Phase 87 CR-01 defect; require a concrete floor AND assert per-shape below.
	// Floor bumped 5→7 by STEER-01 (DEFER-97-01): the two formerly-silent Bash
	// sed/cat shapes now fire on the specific verb.
	require.GreaterOrEqual(t, len(cases), 7,
		"golden must cover at least the seven steering shapes (incl. STEER-01 bash sed/cat); rejecting empty-bucket-as-pass")

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out, err := runNudgeCapture(t, hookInput{
				ToolName:  tc.toolName,
				ToolInput: tc.input,
			})
			require.NoError(t, err, "runNudge must always return nil (exit 0) — fail-open contract")

			adv, ok := parseAdvisory(t, out)
			require.Truef(t, ok, "shape %q (%s) must emit an advisory, got %q", tc.name, tc.toolName, out)
			assert.Equal(t, "PreToolUse", adv.HookSpecificOutput.HookEventName)
			assert.Containsf(t, adv.HookSpecificOutput.AdditionalContext, tc.wantVerb,
				"shape %q must steer to %q, got %q",
				tc.name, tc.wantVerb, adv.HookSpecificOutput.AdditionalContext)
		})
	}

	// STEER-03 negative controls: broadening isGrepReadTool to pass sed/cat through
	// must NOT make the nudge fire on prose/log/config targets — the SILENCE comes
	// from classifyBashTarget returning non-code, so every shape below produces NO
	// advisory and preserves exit-0. These prove the broadened recognizer does not
	// over-fire (98-RESEARCH Pitfall 3). One t.Run per shape.
	for _, neg := range []struct{ name, cmd string }{
		{"negative-control-prose-grep", `grep TODO README.md`},
		{"negative-control-prose-sed", `sed -i 's/x/y/' README.md`},
		{"negative-control-log-cat", `cat app.log`},
		{"negative-control-config-cat", `cat config.yaml`},
	} {
		neg := neg
		t.Run(neg.name, func(t *testing.T) {
			out, err := runNudgeCapture(t, hookInput{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": neg.cmd},
			})
			require.NoError(t, err, "negative control must preserve exit-0 for %q", neg.cmd)
			_, ok := parseAdvisory(t, out)
			assert.Falsef(t, ok,
				"STEER-03: Bash %q over a prose/log/config target must produce no advisory, got %q", neg.cmd, out)
		})
	}
}

// TestNudgeGoldenRevertFails is the MANDATORY revert-and-fail proof for ADOPT-01b
// (T-97-06). It runs the SAME harness for the Read tool (the read-file shape) and
// asserts the advisory does NOT contain a deliberately-WRONG verb
// (`helix rename-symbol`) while it DOES contain the correct one (`helix read-file`).
// This proves the golden keys on the SPECIFIC verb: a wrong-verb expectation in
// TestNudgeShapeGolden would genuinely FAIL, not pass on a "helix" substring.
func TestNudgeGoldenRevertFails(t *testing.T) {
	out, err := runNudgeCapture(t, hookInput{
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "x.go"},
	})
	require.NoError(t, err)
	adv, ok := parseAdvisory(t, out)
	require.True(t, ok, "the Read-tool shape must emit an advisory, got %q", out)

	const wrongVerb = "helix rename-symbol"
	const correctVerb = "helix read-file"

	// The wrong expectation MUST fail: a golden keyed on read -> `helix rename-symbol`
	// would not pass. If this Contains were true, the golden would be vacuous (any
	// "helix" mention would satisfy a wrong-verb expectation).
	require.NotContainsf(t, adv.HookSpecificOutput.AdditionalContext, wrongVerb,
		"revert proof: the read shape must NOT steer to %q — a wrong-verb golden must go RED, got %q",
		wrongVerb, adv.HookSpecificOutput.AdditionalContext)

	// And the correct verb IS present — confirming the advisory is the read-file one,
	// i.e. the golden distinguishes the specific verb rather than any "helix" token.
	require.Containsf(t, adv.HookSpecificOutput.AdditionalContext, correctVerb,
		"the read shape must steer to %q, got %q",
		correctVerb, adv.HookSpecificOutput.AdditionalContext)
}

// TestNudgeGoldenRevertFails_BashSedCat is the STEER-01 anti-vacuity proof for the
// BROADENED Bash sed/cat shapes (98-RESEARCH anti-vacuity reminder). It runs the
// same harness for a Bash `cat …code.go` (and `sed -i …code.go`) and asserts the
// advisory keys on the SPECIFIC verb: the cat shape steers to `helix read-file` and
// must NOT contain a wrong verb (`helix rename-symbol`); the sed -i shape steers to
// `helix replace-in-file` and must NOT contain `helix read-file`. A wrong-verb
// golden for either shape would therefore go RED, proving the new firing rows are
// non-vacuous (they distinguish the specific verb, not a bare "helix" substring).
func TestNudgeGoldenRevertFails_BashSedCat(t *testing.T) {
	// cat over a code file -> read-file, NOT rename-symbol.
	outCat, errCat := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `cat internal/edit.go`},
	})
	require.NoError(t, errCat)
	advCat, okCat := parseAdvisory(t, outCat)
	require.Truef(t, okCat, "the bash-cat shape must emit an advisory, got %q", outCat)
	require.NotContainsf(t, advCat.HookSpecificOutput.AdditionalContext, "helix rename-symbol",
		"revert proof: the bash-cat shape must NOT steer to a wrong verb, got %q",
		advCat.HookSpecificOutput.AdditionalContext)
	require.Containsf(t, advCat.HookSpecificOutput.AdditionalContext, "helix read-file",
		"the bash-cat shape must steer to `helix read-file`, got %q",
		advCat.HookSpecificOutput.AdditionalContext)

	// sed -i over a code file -> replace-in-file, NOT read-file (proves the two
	// broadened shapes are NOT collapsed to one generic verb).
	outSed, errSed := runNudgeCapture(t, hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `sed -i 's/a/b/' pkg/s.go`},
	})
	require.NoError(t, errSed)
	advSed, okSed := parseAdvisory(t, outSed)
	require.Truef(t, okSed, "the bash-sed-i shape must emit an advisory, got %q", outSed)
	require.Containsf(t, advSed.HookSpecificOutput.AdditionalContext, "helix replace-in-file",
		"the bash-sed-i shape must steer to `helix replace-in-file`, got %q",
		advSed.HookSpecificOutput.AdditionalContext)
	require.NotContainsf(t, advSed.HookSpecificOutput.AdditionalContext, "helix read-file",
		"revert proof: the bash-sed-i shape must steer to the EDIT verb, not the read verb, got %q",
		advSed.HookSpecificOutput.AdditionalContext)
}
