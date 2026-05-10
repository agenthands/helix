package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/eval/sandbox"
)

// TestAgentBuildsArgv verifies that buildArgv produces the correct ordered argv.
func TestAgentBuildsArgv(t *testing.T) {
	sb := newTestSandbox(t)
	a := NewAgent(sb, "/usr/local/bin/helix")

	task := Task{
		ID:     "t1",
		Prompt: "Rename X to Y",
		Budget: BudgetParams{MaxToolCalls: 42},
	}
	argv := a.buildArgv(task, "baseline")

	// Expected order per plan spec:
	expected := []string{
		"--print",
		"--bare",
		"--strict-mcp-config",
		"--mcp-config", sb.McpConfigPath("t1", "baseline"),
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--max-turns", "42",
		"Rename X to Y",
	}
	assert.Equal(t, expected, argv)
}

// TestAgentCleanEnv verifies that cleanEnv returns only the allowlisted keys and
// that unknown host env keys are stripped (T-67-Pitfall-1 mitigation).
func TestAgentCleanEnv(t *testing.T) {
	sb := newTestSandbox(t)
	a := NewAgent(sb, "/usr/local/bin/helix")

	// Inject a foreign key into the environment for the test.
	t.Setenv("OPENAI_API_KEY", "sk-should-not-appear")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-anthropic")
	t.Setenv("HELIX_LOG_LEVEL", "debug")

	env := a.cleanEnv("t1", "baseline")

	// Convert to a map for assertion.
	envMap := make(map[string]string)
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Allowlisted keys that are set.
	assert.Contains(t, envMap, "PATH")
	assert.Contains(t, envMap, "HOME")
	assert.Equal(t, sb.HomeFor("t1", "baseline"), envMap["HOME"])
	assert.Equal(t, "sk-test-anthropic", envMap["ANTHROPIC_API_KEY"])
	assert.Equal(t, "debug", envMap["HELIX_LOG_LEVEL"])

	// Forbidden keys must NOT appear.
	assert.NotContains(t, envMap, "OPENAI_API_KEY")

	// Only 4 keys maximum (PATH, HOME, ANTHROPIC_API_KEY, HELIX_LOG_LEVEL) — no extras.
	assert.LessOrEqual(t, len(envMap), 4)
}

// TestAgentSkipsIfClaudeMissing verifies that Run returns ErrClaudeNotFound
// when the claude binary is not on PATH.
func TestAgentSkipsIfClaudeMissing(t *testing.T) {
	sb := newTestSandbox(t)
	a := NewAgent(sb, "/usr/local/bin/helix")

	// Point PATH at an empty directory so claude is not found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	task := Task{ID: "t1", Prompt: "test", Budget: BudgetParams{MaxToolCalls: 5}}
	require.NoError(t, sb.Prepare("t1", "baseline"))

	_, err := a.Run(context.Background(), task, "baseline")
	assert.ErrorIs(t, err, ErrClaudeNotFound)
}

// TestAgentCapturesStdoutStderr verifies that Run captures stdout and stderr of
// the claude subprocess to mode 0600 files and reports their paths.
func TestAgentCapturesStdoutStderr(t *testing.T) {
	sb := newTestSandbox(t)
	fakeClaude := buildFakeClaudeBin(t, fakeClaude{stdout: "hello", stderr: "warn", exitCode: 0})

	// Point PATH at the directory containing the fake claude binary.
	t.Setenv("PATH", filepath.Dir(fakeClaude))

	a := NewAgent(sb, "/usr/local/bin/helix")
	task := Task{ID: "t1", Prompt: "do thing", Budget: BudgetParams{MaxToolCalls: 5}}
	require.NoError(t, sb.Prepare("t1", "baseline"))

	result, err := a.Run(context.Background(), task, "baseline")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check captured stdout.
	stdoutData, err := os.ReadFile(result.StdoutPath)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(stdoutData))

	// Check captured stderr.
	stderrData, err := os.ReadFile(result.StderrPath)
	require.NoError(t, err)
	assert.Equal(t, "warn", string(stderrData))

	// Files must be mode 0600.
	for _, p := range []string{result.StdoutPath, result.StderrPath} {
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file %s should be 0600", p)
	}
}

// TestAgentRespectsContextDeadline verifies that Run kills the subprocess and
// returns the context error when the context is cancelled.
func TestAgentRespectsContextDeadline(t *testing.T) {
	sb := newTestSandbox(t)
	// Build a fake claude that sleeps for 10 seconds.
	fakeClaude := buildFakeClaudeBin(t, fakeClaude{sleepSeconds: 10})

	t.Setenv("PATH", filepath.Dir(fakeClaude))

	a := NewAgent(sb, "/usr/local/bin/helix")
	task := Task{ID: "t1", Prompt: "hang", Budget: BudgetParams{MaxToolCalls: 5}}
	require.NoError(t, sb.Prepare("t1", "baseline"))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := a.Run(ctx, task, "baseline")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ---- helpers ----

func newTestSandbox(t *testing.T) *sandbox.Sandbox {
	t.Helper()
	sb, err := sandbox.NewSandbox("agenttest", "/usr/bin/true")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Cleanup() })
	return sb
}

type fakeClaude struct {
	stdout       string
	stderr       string
	exitCode     int
	sleepSeconds int
}

// buildFakeClaudeBin builds a small Go binary named "claude" that writes
// fixed stdout/stderr, optionally sleeps, then exits with a given code.
func buildFakeClaudeBin(t *testing.T, fc fakeClaude) string {
	t.Helper()

	src := fmt.Sprintf(`package main
import (
	"fmt"
	"os"
	"time"
)
func main() {
	fmt.Fprint(os.Stdout, %q)
	fmt.Fprint(os.Stderr, %q)
	if %d > 0 {
		time.Sleep(time.Duration(%d) * time.Second)
	}
	os.Exit(%d)
}
`, fc.stdout, fc.stderr, fc.sleepSeconds, fc.sleepSeconds, fc.exitCode)

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	// The binary must be named "claude" so exec.LookPath finds it.
	binFile := filepath.Join(dir, "claude")

	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0644))

	var buf bytes.Buffer
	cmd := exec.Command("go", "build", "-o", binFile, srcFile)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	require.NoError(t, cmd.Run(), "build fake claude: %s", buf.String())

	return binFile
}
