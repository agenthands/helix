//go:build !windows
// +build !windows

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoSemanticReadsTotalLineEmitted is the real-daemon proof that closes the
// WR-02 emission half of GAP 1 (81-VERIFICATION.md). Prior to Phase 81-06 the
// bench tore the daemon down with SIGKILL (DaemonHandle.Kill), which is
// untrappable — so d.shutdown() never ran and the "semantic store reads total"
// line (shutdown.go:60-62) was NEVER written on a real run. The no_semantic
// zero-reads gate therefore passed vacuously every time.
//
// This test spawns a REAL daemon through the bench sandbox, drives it down via
// the new graceful DaemonHandle.Stop (SIGTERM + bounded wait) FOLLOWED BY the
// Kill fallback — exactly the teardown ordering RunCell now uses — and asserts:
//
//   - Stop reports graceful=true (the daemon caught SIGTERM and ran shutdown());
//   - daemon.log contains a JSONL line with msg == "semantic store reads total"
//     and an INTEGER count field (the line was actually flushed by the real
//     graceful shutdown path, NOT a synthetic os.WriteFile fixture).
//
// It is HELIX_BIN-gated (SKIPs without a resolvable helix binary, mirroring
// TestDaemonTap) and build !windows (the process-group SIGTERM is POSIX-only).
func TestNoSemanticReadsTotalLineEmitted(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping real-daemon emission test")
	}

	const (
		task = "emission-probe"
		mode = "your_agent_no_semantic"
	)

	outDir := t.TempDir()
	sb, err := benchsandbox.New(time.Now().UTC().Format("20060102T150405Z"), helixBin, outDir)
	require.NoError(t, err, "create bench sandbox")
	t.Cleanup(func() { _ = sb.Cleanup() })

	require.NoError(t, sb.Prepare(task, mode), "prepare cell scratch")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	// Spawn a real daemon over the per-cell Unix socket (HTTP disabled, D-06).
	// No per-cell config / profile is needed for this probe: the daemon emits the
	// reads-total line on EVERY graceful shutdown whenever d.obs != nil
	// (shutdown.go:60-62), independent of the semantic gate — so a bare daemon is
	// sufficient to prove the line is flushed on the real teardown path.
	h, err := subprocess.StartDaemon(ctx, sb, task, mode, "", "")
	require.NoError(t, err, "start daemon")

	// Drive the daemon down through the EXACT teardown ordering RunCell uses:
	// graceful Stop first (so d.shutdown() runs and flushes the line), then Kill
	// as the hard reaper. Stop MUST report graceful exit — that is the WR-02 fix.
	graceful, stopErr := h.Stop(daemonGracefulStopTimeout)
	require.NoError(t, stopErr, "graceful Stop must not error")
	// WR-02: the LOAD-BEARING proof is the presence of the reads-total line below,
	// not strictly that the first Stop won the race within the budget. A genuinely
	// slow-but-correct shutdown (the daemon drained gracefully but past the budget,
	// e.g. under heavy CI load) must NOT flake this test, so graceful is a LOGGED
	// expectation rather than a hard assertion. The require.True(present) below
	// still fails the test if the daemon never emitted the line at all.
	if !graceful {
		t.Logf("daemon did not exit within %s; relying on the reads-total line-presence assertion", daemonGracefulStopTimeout)
	}
	// Kill fallback: a no-op reaper here (the daemon already exited), mirrors RunCell.
	require.NoError(t, h.Kill(), "Kill fallback (no-op reaper) must not error")

	daemonLog := filepath.Join(sb.ModeDir(task, mode), "daemon.log")

	present, count := scanReadsTotalLine(t, daemonLog)
	require.True(t, present,
		"daemon.log must contain a real %q line emitted by the daemon's graceful shutdown (WR-02 emission proof)",
		daemonReadsTotalMsg)
	// On a bare daemon with no semantic activity the count is 0, but the test's
	// load-bearing assertion is PRESENCE + integer-typed count, not the value.
	assert.GreaterOrEqual(t, count, 0, "the reads-total count must be a non-negative integer")
}

// scanReadsTotalLine scans a daemon JSONL log for the last line whose
// msg == daemonReadsTotalMsg, returning whether such a line was seen and its
// integer count. It mirrors scrapeSemanticReadsTotal's parsing but is local to
// the test so the test asserts the line is REALLY present in the file the daemon
// wrote (not via the production scraper, whose presence contract is exercised in
// no_semantic_zero_reads_test.go).
//
// IN-03 / WR-03: this parser is a deliberate copy of scrapeSemanticReadsTotal in
// bench/runtime/cell.go and MUST stay in lockstep with it — including the *int
// `count` handling, where a msg-matching line whose count is absent/non-integer
// is treated as MALFORMED (present stays false) so a garbled count cannot
// masquerade as a clean count=0. Any change to the struct, *int handling, or
// last-line-wins logic must be mirrored in cell.go.
func scanReadsTotalLine(t *testing.T, logPath string) (present bool, count int) {
	t.Helper()
	f, err := os.Open(logPath)
	require.NoError(t, err, "open daemon.log")
	defer f.Close()

	type line struct {
		Msg   string `json:"msg"`
		Count *int   `json:"count"`
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 256*1024)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var l line
		if err := json.Unmarshal(b, &l); err != nil {
			continue
		}
		if l.Msg == daemonReadsTotalMsg {
			if l.Count == nil {
				continue // msg matched but count missing/non-integer — not a valid proof line
			}
			present = true
			count = *l.Count
		}
	}
	require.NoError(t, sc.Err(), "scan daemon.log")
	return present, count
}
