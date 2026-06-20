package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoSemanticZeroReads is the independent runtime verification (D-05,
// criterion #2): the bench cell asserts helix_semantic_store_reads_total == 0
// after a no_semantic run and FAILS the cell hard (not a warn) on any non-zero
// read. The counter value is driven deterministically through the same seam the
// production cell uses — scrapeSemanticReadsTotal parses the daemon-log line the
// daemon emits at shutdown (Task 0 path A: HTTP is disabled over the bench Unix
// socket, so the Prometheus /metrics scrape is unreachable; the count is read
// from <modeDir>/daemon.log instead). assertNoSemanticReads is the pure
// fail-closed predicate RunCell calls.
func TestNoSemanticZeroReads(t *testing.T) {
	// Test 1 (pass case): a no_semantic cell whose counter reads 0 AND whose proof
	// line is PRESENT passes (the clean proof).
	t.Run("no_semantic_zero_reads_passes", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_no_semantic", 0, true)
		assert.NoError(t, err,
			"a no_semantic cell with 0 semantic-store reads (proof line present) must pass (no violation)")
	})

	// Test 2 (fail case): a no_semantic cell whose counter reads N>0 FAILS (proof
	// line present, but a read survived the gate).
	t.Run("no_semantic_nonzero_reads_fails_hard", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_no_semantic", 3, true)
		require.Error(t, err,
			"a no_semantic cell with N>0 semantic-store reads must FAIL the cell (hard fail, not warn)")
		assert.Contains(t, err.Error(), "helix_semantic_store_reads_total",
			"the violation diagnostic must name the counter so the failure is greppable")
		assert.Contains(t, err.Error(), "3",
			"the violation diagnostic must report the observed non-zero count")
	})

	// Test 3 (scope guard): the assertion fires ONLY on the no_semantic arm — a
	// non-zero read count off-arm is benign regardless of line presence.
	t.Run("other_modes_unaffected_by_nonzero_reads", func(t *testing.T) {
		for _, mode := range []string{
			"your_agent_full",
			"no_lsp",
			"no_structured_edit",
			"baseline_plain",
		} {
			err := assertNoSemanticReads(mode, 42, true)
			assert.NoErrorf(t, err,
				"mode %q is NOT the no_semantic arm; a non-zero read count must not fail it", mode)
		}
	})

	// Test 7 (fail-CLOSED, WR-02): a no_semantic cell whose proof line is ABSENT
	// HARD-FAILS — a missing line means the daemon never reached graceful shutdown
	// so the zero-reads guarantee was never proven. This is the inversion of the
	// old fail-open silent count=0.
	t.Run("no_semantic_absent_line_fails_hard", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_no_semantic", 0, false)
		require.Error(t, err,
			"a no_semantic cell whose reads-total proof line is ABSENT must FAIL hard (fail-closed, WR-02)")
		assert.Contains(t, err.Error(), "helix_semantic_store_reads_total",
			"the absent-line diagnostic must name the counter so the failure is greppable")
	})

	// Test 8 (scope guard for absent line): an ABSENT line off the no_semantic arm
	// is benign — the fail-closed behavior must NOT leak to other modes.
	t.Run("other_modes_absent_line_benign", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_full", 0, false)
		assert.NoError(t, err,
			"an absent reads-total line off the no_semantic arm must NOT fail (scope guard)")
	})

	// Test 4: scrapeSemanticReadsTotal parses the daemon-log line the daemon
	// emits at shutdown (Task 0 path A). The line carries msg="semantic store
	// reads total" and a count field; the scraper returns that count.
	t.Run("scrape_reads_total_from_daemon_log", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "daemon.log")
		// A representative daemon JSONL log: unrelated lines plus the one the
		// scraper targets (count=0 — the clean no_semantic shutdown).
		logBody := `{"time":"2026-06-20T11:00:00Z","level":"INFO","msg":"daemon started"}
{"time":"2026-06-20T11:00:01Z","level":"INFO","msg":"tool call","tool":"read_file","pid":1234}
{"time":"2026-06-20T11:00:02Z","level":"INFO","msg":"semantic store reads total","count":0}
{"time":"2026-06-20T11:00:03Z","level":"INFO","msg":"graceful shutdown complete"}
`
		require.NoError(t, os.WriteFile(logPath, []byte(logBody), 0600))

		got, present, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
		assert.True(t, present, "the reads-total line is present in the log")
		assert.Equal(t, 0, got, "scraper must read the count field from the shutdown line")
	})

	// Test 5: scrapeSemanticReadsTotal reads a non-zero count (the violation
	// signal that drives a hard cell failure end-to-end).
	t.Run("scrape_reads_total_nonzero", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "daemon.log")
		logBody := `{"time":"2026-06-20T11:00:02Z","level":"INFO","msg":"semantic store reads total","count":7}
`
		require.NoError(t, os.WriteFile(logPath, []byte(logBody), 0600))

		got, present, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
		assert.True(t, present, "the non-zero reads-total line is present")
		assert.Equal(t, 7, got, "scraper must read a non-zero count")
	})

	// Test 6 (WR-02 fail-CLOSED): a daemon.log with NO reads-total line reports
	// present=false (NOT a silent count=0). The absent-line case is no longer
	// indistinguishable from a real count=0; the no_semantic arm's
	// assertNoSemanticReads (Tests 7/8 above) turns present=false into a HARD
	// failure on-arm and a benign no-op off-arm. This replaces the old
	// "scrape_missing_line_yields_zero" fail-open assertion.
	t.Run("scrape_missing_line_reports_absent", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "daemon.log")
		logBody := `{"time":"2026-06-20T11:00:00Z","level":"INFO","msg":"daemon started"}
`
		require.NoError(t, os.WriteFile(logPath, []byte(logBody), 0600))

		got, present, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
		assert.False(t, present,
			"a log with no reads-total line must report present=false (fail-closed signal, not a silent 0)")
		assert.Equal(t, 0, got, "count is the zero value when no line was seen")
	})
}
