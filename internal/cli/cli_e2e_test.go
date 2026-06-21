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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/forwarder"
)

// representativeVerbName is the single spine verb (90-03) this oracle exercises.
// 91-01 flattened the verb surface onto root and removed the `call` parent, so
// the flat verb name is the full generated name (search-in-files -> the
// search_in_files tool), not the short `search` alias the `call` parent carried.
const representativeVerbName = "search-in-files"

// searchPattern is a unique marker the seeded fixture contains exactly once, so
// the search result is deterministic and easy to compare across the CLI path and
// the MCP path.
const searchPattern = "needle_marker_xyz"

// representativeVerbFlag is the required pattern flag of the flat search-in-files
// verb (its generated flag is --pattern -> the pattern tool arg; the removed
// `call search` alias accepted --query, but the root-attached verb uses the
// generated flag name).
const representativeVerbFlag = "--pattern="

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
func newE2EFixture(t *testing.T, runID string, daemonOpts ...sandbox.DaemonOption) *e2eFixture {
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
	// Optional daemonOpts (e.g. WithAdminAddr) let a sub-test enable the admin
	// listener so it can scrape helix_session_lifecycle (WR-06).
	h, err := sb.StartDaemon(ctx, taskID, mode, "", "", daemonOpts...)
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

// runCLIVerb invokes a REAL flat `helix <verb> ...` subprocess pointed at the
// fixture's daemon socket (via HELIX_SOCKET — the same per-uid resolution the
// verb spine honors). Returns combined stdout and the error (nil on exit 0). The
// subprocess inherits a minimal env so it cannot accidentally dial the host's
// default daemon.
//
// VERB-03: 91-01 flattened the generated verbs onto the root command and removed
// the `call` parent, so the oracle drives `helix <verb> --flag=...` directly (no
// `call` prefix) against the root-attached verb surface.
func (f *e2eFixture) runCLIVerb(ctx context.Context, args ...string) (string, error) {
	return f.runCLIVerbInDir(ctx, "", args...)
}

// runCLIVerbInDir is runCLIVerb with an explicit subprocess working directory.
// The behavioral chain (OUT-04) and self-contained-nav (OUT-03) oracles set
// dir=workspace root because the Phase 92 terse renderer derives its
// workspaceRoot from os.Getwd() (92-02 Open Q1 lock) to relativize loci and to
// clamp the CLI-side snippet read. Running the subprocess from the workspace root
// makes the emitted `relpath:line:col` resolve as the next verb's --path input
// verbatim (no manual massaging), which is exactly the copy-paste contract under
// test. dir="" preserves the inherited cwd (the existing dial oracle's behavior).
func (f *e2eFixture) runCLIVerbInDir(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, f.helixBin, args...)
	cmd.Dir = dir
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

	// CLI path: the same tool (search_in_files) via flat `helix search-in-files
	// --pattern=...` against the SAME daemon. The verb renders the tool's text content verbatim,
	// so the CLI stdout must equal the MCP-path reference text.
	out, err := f.runCLIVerb(ctx, representativeVerbName, representativeVerbFlag+searchPattern)
	if err != nil {
		t.Fatalf("helix %s failed: %v\noutput:\n%s", representativeVerbName, err, out)
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
// the WARM second-call round-trip p50 across a sample of real flat `helix <verb>`
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
	if out, err := f.runCLIVerb(ctx, representativeVerbName, representativeVerbFlag+searchPattern); err != nil {
		t.Fatalf("warm-up call failed: %v\n%s", err, out)
	}

	const samples = 9
	timings := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		out, err := f.runCLIVerb(ctx, representativeVerbName, representativeVerbFlag+searchPattern)
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
				representativeVerbName, representativeVerbFlag+searchPattern)
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

// TestCLI_E2E_OneShotCleanShutdown is the WR-06 oracle: it locks in WR-03's
// clean-shutdown contract. A one-shot flat `helix <verb>` must tear its MCP session
// down cleanly (session flush + stream CloseSend) so the daemon records the
// stdio session with outcome="ended", NOT outcome="error". Before WR-03's fix
// the one-shot path never CloseSend'd, so conn.Close() aborted the stream and
// the daemon's stream.Recv() saw a non-EOF RST → SessionLifecycleInc("error").
//
// We assert this directly against the daemon's own metric: scrape
// helix_session_lifecycle_total{transport="stdio"} before and after one CLI
// call and require the "ended" counter to increase while "error" does not.
func TestCLI_E2E_OneShotCleanShutdown(t *testing.T) {
	// Reserve a fixed loopback port for the daemon's admin/metrics listener.
	adminAddr := reserveLoopbackAddr(t)

	f := newE2EFixture(t, "cleanshutdown", sandbox.WithAdminAddr(adminAddr))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metricsURL := "http://" + adminAddr + "/metrics"
	waitForMetrics(t, ctx, metricsURL)

	f.mcpActivate(t, ctx)

	// Baseline AFTER activate (activate itself is a one-shot call on the fixed
	// fix; the WR-03 regression would inflate "error" on every such call, so the
	// delta-based assertion below isolates THIS subprocess call regardless).
	endedBefore := scrapeLifecycle(t, ctx, metricsURL, "ended", "stdio")
	errorBefore := scrapeLifecycle(t, ctx, metricsURL, "error", "stdio")

	// The contract-bearing call: a REAL flat `helix <verb>` subprocess one-shot.
	out, err := f.runCLIVerb(ctx, representativeVerbName, representativeVerbFlag+searchPattern)
	if err != nil {
		t.Fatalf("helix %s failed: %v\noutput:\n%s", representativeVerbName, err, out)
	}
	if !strings.Contains(out, searchPattern) {
		t.Fatalf("CLI result missing the marker; got %q", out)
	}

	// Give the daemon a beat to observe the half-close and record the lifecycle
	// transition before we re-scrape.
	endedAfter, errorAfter := pollLifecycleDelta(t, ctx, metricsURL, endedBefore, errorBefore)

	if endedAfter <= endedBefore {
		t.Fatalf("one-shot call did NOT increment helix_session_lifecycle{outcome=\"ended\",transport=\"stdio\"} "+
			"(before=%v after=%v) — clean shutdown contract violated (WR-03)", endedBefore, endedAfter)
	}
	if errorAfter > errorBefore {
		t.Fatalf("one-shot call incremented helix_session_lifecycle{outcome=\"error\",transport=\"stdio\"} "+
			"(before=%v after=%v) — the daemon saw a non-EOF stream abort instead of clean EOF (WR-03)",
			errorBefore, errorAfter)
	}
}

// reserveLoopbackAddr binds a loopback TCP port, then releases it, returning the
// "127.0.0.1:PORT" string. There is a small TOCTOU window before the daemon
// rebinds it, but for a single isolated test process this is the standard
// free-port idiom and good enough for the oracle.
func reserveLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// waitForMetrics polls the admin /metrics endpoint until it answers 200 or the
// context expires, so the lifecycle scrape does not race the admin listener's
// bind.
func waitForMetrics(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("admin /metrics did not come up: %v", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatalf("admin /metrics at %s did not answer 200 within 10s", url)
}

// lifecycleMetricRe matches a helix_session_lifecycle_total sample line and
// captures phase, transport, and value, tolerant of label ordering.
var lifecycleMetricRe = regexp.MustCompile(
	`^helix_session_lifecycle_total\{([^}]*)\}\s+([0-9.eE+-]+)`)

// scrapeLifecycle fetches /metrics and returns the current value of
// helix_session_lifecycle_total for the given phase+transport (0 if absent).
func scrapeLifecycle(t *testing.T, ctx context.Context, url, phase, transport string) float64 {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("scrape %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		m := lifecycleMetricRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		labels := m[1]
		if !strings.Contains(labels, `phase="`+phase+`"`) ||
			!strings.Contains(labels, `transport="`+transport+`"`) {
			continue
		}
		v, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("parse metric value %q: %v", m[2], err)
		}
		return v
	}
	return 0
}

// pollLifecycleDelta re-scrapes until the "ended" counter advances past its
// baseline (the daemon records the transition asynchronously after the stream
// half-closes) or the context expires, then returns the final ended/error
// values. It returns the latest observed values even on timeout so the caller's
// assertions produce a precise failure message.
func pollLifecycleDelta(t *testing.T, ctx context.Context, url string, endedBefore, errorBefore float64) (ended, errd float64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		ended = scrapeLifecycle(t, ctx, url, "ended", "stdio")
		errd = scrapeLifecycle(t, ctx, url, "error", "stdio")
		if ended > endedBefore || errd > errorBefore || !time.Now().Before(deadline) {
			return ended, errd
		}
		select {
		case <-ctx.Done():
			return ended, errd
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// --- Phase 92-03 behavioral oracle: nav locus feeds a downstream verb verbatim
// (OUT-04) and nav output is self-contained with a snippet (OUT-03/SC#2). ---

// chainSeedMain is a richer fixture than newE2EFixture's default: it carries a
// top-level Helper function (a search-symbols / find-references target) whose
// declaration line content is asserted as the self-contained nav snippet. It is
// written over the fixture's main.go before activation so the chain operates on a
// genuine cross-referenced symbol.
const chainSeedMain = `package main

import "fmt"

func main() {
	fmt.Println("Hello, Go!")
	Helper()
}

// Helper is a top-level function exercised by the behavioral chain oracle.
func Helper() {
	fmt.Println("HELPER_SNIPPET_TOKEN")
}

// UsingHelper creates a second reference so find-references returns > 1 locus.
func UsingHelper() {
	Helper()
}
`

// seedChainFixture overwrites the fixture repo's main.go with chainSeedMain and a
// minimal go.mod (so gopls treats it as a single-module workspace), then
// re-activates the workspace over MCP so the daemon re-reads the seeded source.
func (f *e2eFixture) seedChainFixture(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.repoDir, "main.go"), []byte(chainSeedMain), 0o600); err != nil {
		t.Fatalf("seed chain main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.repoDir, "go.mod"), []byte("module chainfixture\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatalf("seed chain go.mod: %v", err)
	}
	f.mcpActivate(t, ctx)
}

// requireGoplsE2E skips the test when gopls is not on PATH (the chain endpoints
// are LS-backed nav verbs). Mirrors test/harness.RequireGopls without importing
// the harness into this package.
func requireGoplsE2E(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not installed, skipping LS-backed behavioral chain oracle")
	}
}

// locusLineRe matches a terse "relpath:line:col" prefix at the start of a stdout
// line (the frozen Phase 92 shape; the payload, if any, follows after a TAB). It
// captures relpath, line, and column so the chain can feed them VERBATIM into the
// downstream verb's flags.
var locusLineRe = regexp.MustCompile(`(?m)^([^\s:]+):(\d+):(\d+)(?:\t|$)`)

// parseFirstLocus returns the relpath, line, and column of the FIRST
// `relpath:line:col` line in the given terse stdout, or ok=false if none is
// present. No massaging is applied — the values are taken exactly as the upstream
// verb emitted them, which is the copy-paste contract under test.
func parseFirstLocus(stdout string) (relpath string, line, col int, ok bool) {
	return parseFirstLocusMatching(stdout, "")
}

// parseFirstLocusMatching returns the first `relpath:line:col` line whose relpath
// CONTAINS substr (use "" to match any). search-symbols / search-in-files may
// also surface stdlib loci that sort ahead of the workspace symbol (e.g.
// "../../usr/local/go/src/os/env.go" sorts before "main.go"); the chain selects
// the WORKSPACE-LOCAL locus by substr so it feeds a real in-repo symbol to the
// downstream verb. The selected relpath/line/col are still consumed VERBATIM —
// the predicate only picks WHICH emitted locus, it never rewrites the values.
func parseFirstLocusMatching(stdout, substr string) (relpath string, line, col int, ok bool) {
	for _, m := range locusLineRe.FindAllStringSubmatch(stdout, -1) {
		if substr != "" && !strings.Contains(m[1], substr) {
			continue
		}
		l, err1 := strconv.Atoi(m[2])
		c, err2 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil {
			continue
		}
		return m[1], l, c, true
	}
	return "", 0, 0, false
}

// TestCLI_E2E_Chain is the OUT-04 copy-paste chain oracle. It runs a nav/search
// verb (`helix search-symbols --query=Helper`), parses ONE emitted
// `relpath:line:col` locus from the terse stdout, and feeds that exact relpath +
// line + column into a downstream verb (`helix find-references --path --line
// --column`) WITHOUT any manual massaging. The downstream verb MUST exit 0 and
// return a result — proving the emitted locus is directly consumable as the next
// verb's input. Both subprocesses run with CWD=workspace root so the renderer's
// os.Getwd()-derived workspaceRoot relativizes the loci consistently (92-02 Open
// Q1 lock).
func TestCLI_E2E_Chain(t *testing.T) {
	requireGoplsE2E(t)
	f := newE2EFixture(t, "chain")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	f.seedChainFixture(t, ctx)

	// Upstream nav verb: search for the seeded symbol. Run from the workspace root
	// so the emitted locus is a workspace-relative relpath the downstream verb can
	// consume directly.
	upstream, err := f.runCLIVerbInDir(ctx, f.repoDir, "search-symbols", "--query=Helper")
	if err != nil {
		t.Fatalf("upstream search-symbols failed: %v\noutput:\n%s", err, upstream)
	}

	// Select the WORKSPACE-LOCAL locus (search-symbols also surfaces stdlib hits
	// that sort ahead of main.go). The relpath/line/col are still consumed
	// verbatim; the predicate only picks which emitted locus to chain.
	relpath, line, col, ok := parseFirstLocusMatching(upstream, "main.go")
	if !ok {
		t.Fatalf("upstream nav verb did not emit a parseable workspace-local relpath:line:col locus.\noutput:\n%s", upstream)
	}
	t.Logf("OUT-04 parsed upstream locus VERBATIM: relpath=%q line=%d col=%d", relpath, line, col)

	// Downstream verb fed the upstream locus VERBATIM (no massaging). It must exit
	// 0 and produce a result — the proof the emitted locus is copy-paste-able.
	downstream, err := f.runCLIVerbInDir(ctx, f.repoDir,
		"find-references",
		"--path="+relpath,
		"--line="+strconv.Itoa(line),
		"--column="+strconv.Itoa(col),
	)
	if err != nil {
		t.Fatalf("downstream find-references fed the upstream locus VERBATIM exited non-zero: %v\n"+
			"locus: %s:%d:%d\noutput:\n%s", err, relpath, line, col, downstream)
	}
	if strings.TrimSpace(downstream) == "" {
		t.Fatalf("downstream find-references returned no result for the chained locus %s:%d:%d "+
			"(OUT-04 expects the locus to be a usable input)\noutput:\n%s", relpath, line, col, downstream)
	}

	t.Logf("OUT-04 chain proven: search-symbols -> %s:%d:%d -> find-references exit 0 with result:\n%s",
		relpath, line, col, downstream)
}

// TestCLI_E2E_NavSelfContained is the OUT-03/SC#2 self-contained-nav oracle. It
// runs a bare nav verb (`helix go-to-definition`) and asserts its stdout carries
// BOTH the `relpath:line:col` locus AND a snippet (the source line content) in
// the same output block — so an agent does NOT need a follow-up Read to see what
// is at the locus. The seeded Helper declaration line is the snippet target; we
// assert a token from that line ("Helper") appears alongside the locus.
func TestCLI_E2E_NavSelfContained(t *testing.T) {
	requireGoplsE2E(t)
	f := newE2EFixture(t, "navself")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	f.seedChainFixture(t, ctx)

	// go-to-definition on the Helper() call site (line 7, col 2 -> the call in
	// main). 1-indexed flags per verbs_gen.go. Run from the workspace root.
	out, err := f.runCLIVerbInDir(ctx, f.repoDir, "go-to-definition", "--path=main.go", "--line=7", "--column=2")
	if err != nil {
		t.Fatalf("go-to-definition failed: %v\noutput:\n%s", err, out)
	}

	relpath, line, col, ok := parseFirstLocus(out)
	if !ok {
		t.Fatalf("nav verb did not emit a relpath:line:col locus (OUT-03 requires a locus).\noutput:\n%s", out)
	}

	// The snippet must travel with the locus: assert a token from the seeded
	// Helper declaration line appears in the SAME line as the locus (the renderer
	// appends the source line after a TAB for bare nav loci — navToolsWithSnippet).
	var locusLine string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, relpath+":") {
			locusLine = ln
			break
		}
	}
	if locusLine == "" {
		t.Fatalf("could not isolate the locus line for %s in nav output.\noutput:\n%s", relpath, out)
	}
	const snippetToken = "Helper"
	if !strings.Contains(locusLine, snippetToken) {
		t.Fatalf("nav locus line is NOT self-contained: missing snippet token %q on the locus line "+
			"(OUT-03/SC#2 requires the source-line snippet so no follow-up Read is forced).\n"+
			"locus line: %q\nfull output:\n%s", snippetToken, locusLine, out)
	}

	t.Logf("OUT-03 self-contained nav proven: %s:%d:%d carries snippet token %q on the same line: %q",
		relpath, line, col, snippetToken, locusLine)
}
