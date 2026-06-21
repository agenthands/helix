//go:build integration || llm || llmjudge

// Golden output tests for the frozen Phase 92 terse CLI output contract
// (TEST-02, OUT-06, OUT-07).
//
// Re-targeted from the pre-92 MCP `TextContent` goldens to REAL
// `helix <verb> --flags` SUBPROCESS stdout: golden capture now drives a real
// helix binary against a live daemon (brought up via internal/eval/sandbox) and
// freezes the terse `relpath:line:col<TAB>payload` shape, the `--abs` absolute
// form (OUT-07), and the `--color=never` zero-ANSI form (OUT-06). This is the
// authoritative freeze of the renderer's output, asserted against real CLI
// output — not the unit layer and not MCP TextContent.
//
// Gating (mirrors the dial oracle in internal/cli/cli_e2e_test.go): the test
// SKIPs cleanly when HELIX_BIN is unresolvable; the phase verify step builds the
// binary and points HELIX_BIN at it so the test RUNS (not vacuously skips).
//
// First run MUST use GOLDEN_UPDATE=1 to (re)generate baseline golden files:
//   HELIX_BIN=<built> GOLDEN_UPDATE=1 go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...
//
// Subsequent runs compare against the golden files (idempotent, deterministic —
// the 92-02 renderer sorts+dedups loci CLI-side so a second run is byte-identical):
//   HELIX_BIN=<built> go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...

package contract_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/forwarder"
	"github.com/agenthands/helix/test/harness"
)

// resolveHelixBin returns the helix binary that drives the golden subprocess
// capture, or "" if none is available (the caller SKIPs). It prefers the
// HELIX_BIN env override (CI / a freshly-built binary) then falls back to `helix`
// on PATH — mirroring internal/cli/cli_e2e_test.go's resolveHelixBin exactly.
func resolveHelixBin() string {
	if env := os.Getenv("HELIX_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	if p, err := exec.LookPath("helix"); err == nil {
		return p
	}
	return ""
}

// goldenFixtureMain is the seeded Go source the golden subprocess verbs operate
// on. It carries a top-level Helper function (a go_to_definition / find_references
// target whose snippet line is asserted by the behavioral oracle) and a
// DemoStruct type. Kept deterministic and self-contained so the goldens never
// depend on stdlib results that vary by Go version.
const goldenFixtureMain = `package main

import "fmt"

func main() {
	fmt.Println("Hello, Go!")
	Helper()
}

// Helper is a top-level function used for go_to_definition and find_references tests.
func Helper() {
	fmt.Println("Helper function called")
}

// DemoStruct has a known field and method for symbol retrieval tests.
type DemoStruct struct {
	Field int
}

// Value returns the field value. Used for method resolution tests.
func (d *DemoStruct) Value() int {
	return d.Field
}

// UsingHelper calls Helper to create a cross-reference for find_references tests.
func UsingHelper() {
	Helper()
}
`

// goldenEnv stands up a live daemon over a seeded Go fixture and exposes the
// helix binary + socket the subprocess verbs dial. It is the shared bringup the
// goldenCase loop reuses.
type goldenEnv struct {
	helixBin string
	socket   string
	repoDir  string
}

// newGoldenEnv builds a sandbox, seeds a Go workspace, starts a real daemon, and
// activates the workspace over MCP (the path that sets the daemon's
// active-workspace state the file/nav tools read). Reaped via t.Cleanup.
func newGoldenEnv(t *testing.T) *goldenEnv {
	t.Helper()

	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping golden oracle")
	}
	// LS-backed verbs (go-to-definition / find-references / search-symbols /
	// get-symbol-overview / get-hover-info) need gopls; skip cleanly if absent.
	harness.RequireGopls(t)

	const (
		taskID = "contract-golden"
		mode   = "full"
	)

	sb, err := sandbox.NewSandbox("golden", helixBin)
	if err != nil {
		t.Fatalf("sandbox.NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	if err := sb.Prepare(taskID, mode); err != nil {
		t.Fatalf("sandbox.Prepare: %v", err)
	}

	repoDir := sb.RepoFor(taskID, mode)
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(goldenFixtureMain), 0o600); err != nil {
		t.Fatalf("seed main.go: %v", err)
	}
	// A minimal go.mod keeps gopls happy (single-module workspace).
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module goldenfixture\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatalf("seed go.mod: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	h, err := sb.StartDaemon(ctx, taskID, mode, "", "")
	if err != nil {
		t.Fatalf("sandbox.StartDaemon: %v", err)
	}
	t.Cleanup(func() { _ = h.Kill() })

	socket := sb.SocketFor(taskID, mode)

	// Activate over MCP (sets the daemon's active-workspace state; the gRPC
	// activate RPC sets only kernel state).
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	res, err := forwarder.CallTool(ctx, socket, "", logger, "contract-golden",
		"activate_project", map[string]any{"repo_path": repoDir})
	if err != nil {
		t.Fatalf("MCP activate_project: %v", err)
	}
	if res.IsError {
		t.Fatalf("MCP activate_project reported error")
	}

	return &goldenEnv{helixBin: helixBin, socket: socket, repoDir: repoDir}
}

// runVerb runs a REAL `helix <verb> --flags...` subprocess against the env's
// daemon socket and returns its stdout. The subprocess CWD is set to the
// workspace root so the renderer's os.Getwd()-derived workspaceRoot relativizes
// loci correctly (92-02 Open Q1 lock) and the CLI-side snippet read resolves.
// stdout and stderr are captured separately so the golden holds stdout only.
func (e *goldenEnv) runVerb(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.helixBin, args...)
	cmd.Dir = e.repoDir
	cmd.Env = []string{
		"HELIX_SOCKET=" + e.socket,
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
		// Force deterministic, TTY-independent color resolution so `auto` golden
		// captures are byte-stable regardless of the harness's stdout being a pipe
		// (belt-and-suspenders; the renderer already leaves auto == never off-TTY).
		"NO_COLOR=1",
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Non-zero exit is acceptable for some verbs (e.g. a nav with no result
		// may still exit 0; we do not assert exit code here — the golden is the
		// stdout SHAPE). Surface stderr only on a hard failure to aid debugging.
		t.Logf("helix %v exited with %v (stderr: %s)", args, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String()
}

// normalizeResponse replaces non-deterministic values with stable placeholders.
// The workspace dir is scrubbed to <WORKSPACE> FIRST (T-92-06: no contributor's
// absolute host path is committed, including in the --abs variant). The default
// terse form is already workspace-relative, so the substitution is a no-op for
// relpath goldens and load-bearing only for --abs goldens.
func normalizeResponse(text string, workspaceDir string) string {
	if workspaceDir != "" {
		text = strings.ReplaceAll(text, workspaceDir, "<WORKSPACE>")
	}
	// Normalize absolute paths containing /testdata/fixtures/ (defensive).
	text = regexp.MustCompile(`/[^\s"]+/testdata/fixtures/`).ReplaceAllString(text, "<FIXTURE_ROOT>/")
	// Normalize /tmp/ and /var/folders/ paths (the sandbox repo lives under one).
	text = regexp.MustCompile(`(?:/tmp|/var/folders)/[^\s"]+`).ReplaceAllString(text, "<TMPDIR>")
	// Normalize ISO timestamps.
	text = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s"]*`).ReplaceAllString(text, "<TIMESTAMP>")
	// Normalize UUIDs.
	text = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).ReplaceAllString(text, "<UUID>")
	return text
}

// filterWorkspaceLines keeps only lines mentioning the fixture file (main.go) so
// search_symbols stdlib results that vary by Go version/platform do not leak into
// the golden. Lines are sorted for stability.
func filterWorkspaceLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "main.go") {
			kept = append(kept, line)
		}
	}
	sort.Strings(kept)
	if len(kept) == 0 {
		return text
	}
	return strings.Join(kept, "\n") + "\n"
}

// goldenDir returns the root directory for golden output files.
func goldenDir() string {
	return filepath.Join(harness.ProjectRoot(), "test", "oracle", "contract", "testdata", "golden")
}

// goldenCase describes one CLI verb invocation captured into a golden. tool is
// the underlying tool name (the golden subdir); verb is the kebab CLI verb; args
// are the verb's CLI flags (1-indexed line/column per verbs_gen.go help text).
type goldenCase struct {
	tool          string   // golden subdir name (underlying tool name)
	verb          string   // kebab CLI verb
	args          []string // CLI flags
	goldenName    string   // golden file basename (default "success.golden")
	workspaceOnly bool     // keep only main.go lines (search_symbols)
	wantRelpath   bool     // assert output carries a relpath:line:col form
	wantNoEsc     bool     // assert output has zero ESC (0x1b) bytes (--color=never)
}

// goldenName returns the golden file basename for a case.
func (c goldenCase) name() string {
	if c.goldenName != "" {
		return c.goldenName
	}
	return "success.golden"
}

// goldenCases is the re-targeted CLI verb capture set. Line/column are 1-indexed
// (the verb flags are 1-based per verbs_gen.go). Line 11 col 6 targets "Helper"
// in "func Helper() {" of the seeded fixture.
//
// Coverage spans the three render classes:
//   - locus-list (terse relpath:line:col<TAB>payload): go-to-definition,
//     find-references, search-symbols, search-in-files
//   - tree (shape-only passthrough): get-symbol-overview
//   - opaque (markdown passthrough): get-hover-info
//
// plus the two flag-variant goldens that freeze OUT-06/OUT-07 against real CLI
// output: a `--abs` go-to-definition golden (absolute <WORKSPACE>/...:L:C form)
// and a `--color=never` find-references golden (zero ESC bytes).
func goldenCases() []goldenCase {
	return []goldenCase{
		// --- locus-list (terse) ---
		{
			tool: "go_to_definition", verb: "go-to-definition",
			args:        []string{"--path=main.go", "--line=7", "--column=2"},
			wantRelpath: true,
		},
		{
			tool: "find_references", verb: "find-references",
			args:        []string{"--path=main.go", "--line=11", "--column=6"},
			wantRelpath: true,
		},
		{
			tool: "search_symbols", verb: "search-symbols",
			args:          []string{"--query=Helper"},
			workspaceOnly: true,
		},
		{
			tool: "search_in_files", verb: "search-in-files",
			args:        []string{"--pattern=func"},
			wantRelpath: true,
		},
		// --- tree (shape-only passthrough) ---
		{
			tool: "get_symbol_overview", verb: "get-symbol-overview",
			args: []string{"--path=main.go"},
		},
		// --- opaque (markdown passthrough) ---
		{
			tool: "get_hover_info", verb: "get-hover-info",
			args: []string{"--path=main.go", "--line=11", "--column=6"},
		},
		// --- OUT-07: --abs variant freezes the absolute <WORKSPACE>/...:L:C form ---
		{
			tool: "go_to_definition", verb: "go-to-definition",
			args:        []string{"--abs", "--path=main.go", "--line=7", "--column=2"},
			goldenName:  "abs.golden",
			wantRelpath: true,
		},
		// --- OUT-06: --color=never variant freezes the zero-ANSI form ---
		{
			tool: "find_references", verb: "find-references",
			args:       []string{"--color=never", "--path=main.go", "--line=11", "--column=6"},
			goldenName: "color_never.golden",
			wantNoEsc:  true,
		},
	}
}

// TestGolden_CLIStdout captures golden output for each CLI verb as REAL
// `helix <verb> --flags` subprocess stdout in the frozen terse shape (TEST-02),
// plus the --abs (OUT-07) and --color=never (OUT-06) flag variants. It SKIPs
// cleanly when HELIX_BIN (or gopls) is unavailable.
func TestGolden_CLIStdout(t *testing.T) {
	env := newGoldenEnv(t)
	gDir := goldenDir()

	for _, tc := range goldenCases() {
		tc := tc
		runName := tc.verb
		if tc.goldenName != "" {
			runName = tc.verb + "/" + strings.TrimSuffix(tc.goldenName, ".golden")
		}
		t.Run(runName, func(t *testing.T) {
			out := env.runVerb(t, append([]string{tc.verb}, tc.args...)...)

			normalized := normalizeResponse(out, env.repoDir)
			if tc.workspaceOnly {
				normalized = filterWorkspaceLines(normalized)
			}

			// Behavioral assertions on the captured shape (run pre-golden so a
			// regenerate with GOLDEN_UPDATE=1 still exercises them).
			if tc.wantRelpath {
				assertRelpathForm(t, normalized)
			}
			if tc.wantNoEsc {
				if bytes.IndexByte([]byte(out), 0x1b) >= 0 {
					t.Errorf("--color=never output contains an ESC (0x1b) byte; OUT-06 requires zero ANSI.\noutput: %q", out)
				}
			}

			goldenPath := filepath.Join(gDir, tc.tool, tc.name())
			harness.AssertGolden(t, goldenPath, []byte(normalized))
		})
	}
}

// relpathFormRe matches a terse "relpath:line:col" (or absolute "<WORKSPACE>/...:L:C")
// locus prefix anywhere in a line. It deliberately does NOT match a "file://"
// URI-scheme prefix — the re-targeted shape is scheme-less (the unit renderer
// strips the formatLocations "file://<abs>" prefix to a relpath).
var relpathFormRe = regexp.MustCompile(`(?m)^(?:<WORKSPACE>/)?[^\s:]+:\d+:\d+`)

// assertRelpathForm asserts the normalized output carries at least one
// relpath:line:col locus and no absolute "file://" URI-scheme prefix (the freeze
// is the scheme-less terse form). It tolerates a "(no results)" passthrough body
// (some LS setups return no definition for a stdlib-resolved target) — in that
// case the freeze is the passthrough text, captured by the golden directly.
func assertRelpathForm(t *testing.T, normalized string) {
	t.Helper()
	if strings.Contains(normalized, "file://") { //nolint - the contract forbids this scheme prefix
		t.Errorf("output carries an absolute file:// URI-scheme prefix; the frozen shape is scheme-less relpath:line:col.\noutput:\n%s", normalized)
	}
	if strings.TrimSpace(normalized) == "" {
		return
	}
	if strings.Contains(normalized, "(no results)") {
		return
	}
	if !relpathFormRe.MatchString(normalized) {
		t.Errorf("output does not carry a relpath:line:col locus form.\noutput:\n%s", normalized)
	}
}

// _ keeps fmt imported for ad-hoc debugging during golden bring-up without a
// churny import toggle; it is a no-op at runtime.
var _ = fmt.Sprintf
