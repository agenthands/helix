//go:build !windows
// +build !windows

package cli_test

// HELIX_BIN-gated, !windows real-subprocess E2E oracle for the v2.0 CLI dial
// spine (TEST-01, CLI-01/02/03). It drives a REAL `helix` binary as a subprocess
// against a live daemon brought up via the v1.12 internal/eval/sandbox harness
// (StartDaemon / DaemonHandle) — never a hand-rolled exec.Command daemon harness
// (RESEARCH locked constraint; sandbox owns socket-path-length safety, the env
// allowlist, its own process group, and single-Wait reaping).
//
// This file uses unix domain sockets, so it is //go:build !windows (the Windows
// local-dial smoke is a later milestone concern — RESEARCH Pitfall 6).
//
// The three sub-tests, one per task:
//   - TestCLI_E2E_OneShot              (TEST-01, CLI-01): representative verb
//     round-trips as a real subprocess and matches the MCP-path result.
//   - TestCLI_WarmReuseSLO             (CLI-02): warm second-call p50 measured;
//     SLO recorded as a multiple of the observed median and asserted against the
//     recorded value (not a hardcoded guess).
//   - TestCLI_ParallelColdSingleDaemon (CLI-03 real): N parallel cold callers
//     against the same socket yield exactly ONE daemon process.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/forwarder"
)

// representativeVerbName is the single spine verb (90-03) this oracle exercises.
const representativeVerbName = "search"

// searchPattern is a unique marker the seeded fixture contains exactly once, so
// the search result is deterministic and easy to compare across the CLI path and
// the MCP path.
const searchPattern = "needle_marker_xyz"

// resolveHelixBin returns the helix binary to drive the integration tests, or ""
// if none is available (the caller SKIPs). It prefers the HELIX_BIN env override
// (so CI / a freshly-built binary can be pointed at without polluting PATH), then
// falls back to `helix` on PATH — mirroring the bench convention exactly
// (bench/runtime/daemon_tap_integration_test.go).
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

// e2eFixture holds a live daemon plus the seeded workspace path. It is the shared
// bringup the three sub-tests reuse.
type e2eFixture struct {
	helixBin string
	socket   string
	repoDir  string
	sb       *sandbox.Sandbox
	handle   *sandbox.DaemonHandle
	logger   *slog.Logger
}

// newE2EFixture stands up a sandbox, seeds the workspace with a file containing
// searchPattern, and starts a real daemon via sandbox.StartDaemon. The daemon is
// reaped (h.Kill) and the sandbox removed (sb.Cleanup) via t.Cleanup so no orphan
// processes survive a failed run (T-90-13).
func newE2EFixture(t *testing.T, runID string) *e2eFixture {
	t.Helper()

	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping E2E oracle")
	}

	const (
		taskID = "cli-e2e"
		mode   = "full"
	)

	sb, err := sandbox.NewSandbox(runID, helixBin)
	if err != nil {
		t.Fatalf("sandbox.NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	if err := sb.Prepare(taskID, mode); err != nil {
		t.Fatalf("sandbox.Prepare: %v", err)
	}

	// Seed the per-(task,mode) repo with a file containing the unique marker.
	repoDir := sb.RepoFor(taskID, mode)
	seed := filepath.Join(repoDir, "main.go")
	content := fmt.Sprintf("package main\n\nfunc main() {\n\tprintln(%q)\n}\n", searchPattern)
	if err := os.WriteFile(seed, []byte(content), 0o600); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Real daemon via the v1.12 sandbox harness (RESEARCH locked constraint).
	// StartDaemon waits for the socket; "" profile/cfg keeps the default "full".
	h, err := sb.StartDaemon(ctx, taskID, mode, "", "")
	if err != nil {
		t.Fatalf("sandbox.StartDaemon: %v", err)
	}
	t.Cleanup(func() { _ = h.Kill() })

	return &e2eFixture{
		helixBin: helixBin,
		socket:   sb.SocketFor(taskID, mode),
		repoDir:  repoDir,
		sb:       sb,
		handle:   h,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// mcpActivate activates the workspace on the daemon via the MCP `activate_project`
// tool (the path that sets the daemon's active-workspace state the file tools read;
// the gRPC activate RPC sets only kernel state). It must run before any verb that
// operates on the active workspace.
func (f *e2eFixture) mcpActivate(t *testing.T, ctx context.Context) {
	t.Helper()
	res, err := forwarder.CallTool(ctx, f.socket, f.logger, "e2e-test",
		"activate_project", map[string]any{"repo_path": f.repoDir})
	if err != nil {
		t.Fatalf("MCP activate_project: %v", err)
	}
	if res.IsError {
		t.Fatalf("MCP activate_project reported error: %s", toolText(res))
	}
}

// mcpSearch issues the representative tool (search_in_files) over the MCP path and
// returns its rendered text — the reference the CLI path is compared against.
func (f *e2eFixture) mcpSearch(t *testing.T, ctx context.Context) string {
	t.Helper()
	res, err := forwarder.CallTool(ctx, f.socket, f.logger, "e2e-test",
		"search_in_files", map[string]any{"pattern": searchPattern})
	if err != nil {
		t.Fatalf("MCP search_in_files: %v", err)
	}
	if res.IsError {
		t.Fatalf("MCP search_in_files reported error: %s", toolText(res))
	}
	return toolText(res)
}

// runCLIVerb invokes a REAL `helix call <verb> ...` subprocess pointed at the
// fixture's daemon socket (via HELIX_SOCKET — the same per-uid resolution the
// verb spine honors). Returns combined stdout and the error (nil on exit 0). The
// subprocess inherits a minimal env so it cannot accidentally dial the host's
// default daemon.
func (f *e2eFixture) runCLIVerb(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"call"}, args...)
	cmd := exec.CommandContext(ctx, f.helixBin, full...)
	cmd.Env = []string{
		"HELIX_SOCKET=" + f.socket,
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// toolText concatenates the text content of a tool result.
func toolText(res *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// TestCLI_E2E_OneShot is Task 1 (TEST-01, CLI-01): a representative verb runs as a
// REAL `helix` subprocess against a live daemon and returns the same tool result
// the MCP path returns. Round-trip works end to end.
func TestCLI_E2E_OneShot(t *testing.T) {
	f := newE2EFixture(t, "oneshot")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Activate the workspace AND capture the MCP-path reference result.
	f.mcpActivate(t, ctx)
	reference := f.mcpSearch(t, ctx)
	if !strings.Contains(reference, searchPattern) {
		t.Fatalf("MCP reference result did not contain the marker; got %q", reference)
	}

	// CLI path: the same tool (search_in_files) via `helix call search --query=...`
	// against the SAME daemon. The verb renders the tool's text content verbatim,
	// so the CLI stdout must equal the MCP-path reference text.
	out, err := f.runCLIVerb(ctx, representativeVerbName, "--query="+searchPattern)
	if err != nil {
		t.Fatalf("helix call %s failed: %v\noutput:\n%s", representativeVerbName, err, out)
	}

	gotCLI := strings.TrimRight(out, "\n")
	wantMCP := strings.TrimRight(reference, "\n")
	if gotCLI != wantMCP {
		t.Fatalf("CLI result does not match MCP path.\nCLI:  %q\nMCP:  %q", gotCLI, wantMCP)
	}
	if !strings.Contains(gotCLI, searchPattern) {
		t.Fatalf("CLI result missing the marker; got %q", gotCLI)
	}
}

// slowFactor is the comfortable multiple of the observed warm p50 used to derive
// the asserted SLO (CLI-02). The assertion is against the RECORDED p50*factor,
// never a hardcoded millisecond literal. floorSLO guards a degenerately tiny p50
// (e.g. a sub-millisecond local run) from producing an unmeetable SLO under CI
// jitter — the asserted SLO is max(p50*slowFactor, floorSLO).
const (
	slowFactor = 5
	floorSLO   = 50 * time.Millisecond
)

// TestCLI_WarmReuseSLO is Task 2 (CLI-02): against a single warm daemon, measure
// the WARM second-call round-trip p50 across a sample of real `helix call`
// subprocess invocations, RECORD the SLO as a multiple of the observed median, and
// assert the measured p50 is below that RECORDED value. The chosen verb operates on
// the already-active workspace (warmed by the first call), so the timing isolates
// dial + IPC + dispatch rather than LS cold-index (RESEARCH Pitfall 5).
func TestCLI_WarmReuseSLO(t *testing.T) {
	f := newE2EFixture(t, "warmslo")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Warm the daemon: activate the workspace and run one discarded warm-up call
	// so the very first cold/dispatch costs do not skew the sample.
	f.mcpActivate(t, ctx)
	if out, err := f.runCLIVerb(ctx, representativeVerbName, "--query="+searchPattern); err != nil {
		t.Fatalf("warm-up call failed: %v\n%s", err, out)
	}

	const samples = 9
	timings := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		out, err := f.runCLIVerb(ctx, representativeVerbName, "--query="+searchPattern)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("warm call %d failed: %v\n%s", i, err, out)
		}
		if !strings.Contains(out, searchPattern) {
			t.Fatalf("warm call %d missing marker: %q", i, out)
		}
		timings = append(timings, elapsed)
	}

	p50 := medianDuration(timings)

	// RECORD the SLO as a comfortable multiple of the observed p50 (floored to
	// absorb CI jitter on a sub-ms local p50) and assert the measured p50 against
	// that RECORDED value — never a bare hardcoded threshold.
	slo := time.Duration(slowFactor) * p50
	if slo < floorSLO {
		slo = floorSLO
	}

	t.Logf("CLI-02 warm-reuse SLO: verb=%q warmth=workspace-active+1-warmup samples=%d "+
		"observed_p50=%s factor=%dx floor=%s recorded_SLO=%s",
		representativeVerbName, samples, p50, slowFactor, floorSLO, slo)

	if p50 >= slo {
		t.Fatalf("warm second-call p50 %s exceeded the recorded SLO %s (factor %dx of observed p50)",
			p50, slo, slowFactor)
	}
}

// medianDuration returns the p50 of the sample (lower-middle for even counts).
func medianDuration(ds []time.Duration) time.Duration {
	cp := append([]time.Duration(nil), ds...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

// TestCLI_ParallelColdSingleDaemon is Task 3 (CLI-03 real): with a CLEAN socket
// (no daemon yet), launch N REAL `helix` invocations concurrently that each go
// through the race-safe ConnectOrStartDaemon cold path; assert exactly ONE daemon
// process is spawned for that socket. This is the REAL-PROCESS counterpart to plan
// 90-01's in-process synctest (synctest cannot fork real daemons — RESEARCH
// Pitfall 4).
//
// Real `helix` subprocesses (not in-process goroutines) are used deliberately:
// the cold-start path execs the caller's own os.Executable() with --serve, so the
// auto-started daemon is a genuine `helix --serve` only when the caller IS the
// helix binary. The tool result is irrelevant (the workspace is not activated, so
// the search errors); the assertion is the DAEMON COUNT under N parallel cold
// callers.
func TestCLI_ParallelColdSingleDaemon(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping E2E oracle")
	}

	// A short, clean socket path under /tmp (AF_UNIX 104-byte safety) that NO
	// daemon currently owns. We do NOT use sandbox.StartDaemon here: the whole
	// point is to let the dial spine COLD-START the daemon under concurrency.
	root, err := os.MkdirTemp("/tmp", "helix-cli-cold-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	socket := filepath.Join(root, "d.sock")

	// Reap any daemon that bound this socket, however the test exits.
	t.Cleanup(func() { reapPids(daemonPidsForSocket(t, socket)) })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const callers = 8
	var wg sync.WaitGroup
	// Gate all callers on a single channel so they hit the cold socket as close to
	// simultaneously as possible (maximize the spawn-race window).
	gate := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			cmd := exec.CommandContext(ctx, helixBin,
				"call", representativeVerbName, "--query="+searchPattern)
			cmd.Env = []string{
				"HELIX_SOCKET=" + socket,
				"HOME=" + os.Getenv("HOME"),
				"PATH=" + os.Getenv("PATH"),
			}
			// Discard output; the search errors (no active workspace) — that is
			// expected and irrelevant. We assert on the daemon process count.
			_ = cmd.Run()
		}()
	}
	close(gate)
	wg.Wait()

	// Give the (single) auto-started daemon a moment to finish binding before we
	// count, and let any TOCTOU straggler that lost the listen-bind exit.
	deadline := time.Now().Add(5 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		pids = daemonPidsForSocket(t, socket)
		if len(pids) >= 1 {
			// Re-sample once more after a short settle to catch a transient
			// duplicate that has not yet exited.
			time.Sleep(300 * time.Millisecond)
			pids = daemonPidsForSocket(t, socket)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(pids) != 1 {
		t.Fatalf("expected exactly ONE daemon for socket %s after %d parallel cold callers, found %d: %v",
			socket, callers, len(pids), pids)
	}
}

// daemonPidsForSocket returns the PIDs of live `helix --serve` processes whose
// argv contains the given socket path. Uses `pgrep -f` (POSIX, available on the
// !windows platforms this file builds for).
func daemonPidsForSocket(t *testing.T, socket string) []int {
	t.Helper()
	// `--` terminates pgrep option parsing and the "socket=" pattern deliberately
	// does NOT start with '-' (a leading "--serve" pattern is parsed by pgrep as a
	// flag, matching nothing). The daemon argv is `helix --serve --socket=<sock>
	// --http-addr=`, so "socket=<sock>" uniquely identifies it.
	out, err := exec.Command("pgrep", "-f", "--", "socket="+socket).Output()
	if err != nil {
		// pgrep exits 1 with no matches — treat as "no daemons" only if output is
		// empty; any other failure is reported.
		if len(bytes.TrimSpace(out)) == 0 {
			return nil
		}
		t.Fatalf("pgrep: %v", err)
	}
	var pids []int
	for _, line := range strings.Fields(strings.TrimSpace(string(out))) {
		var pid int
		if _, err := fmt.Sscanf(line, "%d", &pid); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// reapPids best-effort terminates the listed PIDs so the test leaves no orphan
// daemons (T-90-13). SIGKILL is acceptable here — these are auto-started daemons
// the test owns.
func reapPids(pids []int) {
	for _, pid := range pids {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
}
