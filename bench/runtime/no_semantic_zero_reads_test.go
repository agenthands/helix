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
	// Test 1 (pass case): a no_semantic cell whose counter reads 0 passes.
	t.Run("no_semantic_zero_reads_passes", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_no_semantic", 0)
		assert.NoError(t, err,
			"a no_semantic cell with 0 semantic-store reads must pass (no violation)")
	})

	// Test 2 (fail case): a no_semantic cell whose counter reads N>0 FAILS.
	t.Run("no_semantic_nonzero_reads_fails_hard", func(t *testing.T) {
		err := assertNoSemanticReads("your_agent_no_semantic", 3)
		require.Error(t, err,
			"a no_semantic cell with N>0 semantic-store reads must FAIL the cell (hard fail, not warn)")
		assert.Contains(t, err.Error(), "helix_semantic_store_reads_total",
			"the violation diagnostic must name the counter so the failure is greppable")
		assert.Contains(t, err.Error(), "3",
			"the violation diagnostic must report the observed non-zero count")
	})

	// Test 3 (scope guard): the assertion fires ONLY on the no_semantic arm.
	t.Run("other_modes_unaffected_by_nonzero_reads", func(t *testing.T) {
		for _, mode := range []string{
			"your_agent_full",
			"no_lsp",
			"no_structured_edit",
			"baseline_plain",
		} {
			err := assertNoSemanticReads(mode, 42)
			assert.NoErrorf(t, err,
				"mode %q is NOT the no_semantic arm; a non-zero read count must not fail it", mode)
		}
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

		got, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
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

		got, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
		assert.Equal(t, 7, got, "scraper must read a non-zero count")
	})

	// Test 6: a daemon.log with NO reads-total line yields 0 (a daemon that
	// shut down without emitting the line is treated as zero reads, not an
	// error — the line only appears when the semantic subsystem ran).
	t.Run("scrape_missing_line_yields_zero", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "daemon.log")
		logBody := `{"time":"2026-06-20T11:00:00Z","level":"INFO","msg":"daemon started"}
`
		require.NoError(t, os.WriteFile(logPath, []byte(logBody), 0600))

		got, err := scrapeSemanticReadsTotal(logPath)
		require.NoError(t, err)
		assert.Equal(t, 0, got, "a log with no reads-total line means zero reads")
	})
}
