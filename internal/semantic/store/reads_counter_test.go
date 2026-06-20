// Phase 81 ABLATE-06 (81-01): read-counter RED→GREEN test.
//
// D-05 makes the bench cell assert helix_semantic_store_reads_total == 0 on
// the no_semantic arm. For that assertion to be meaningful the counter must
// (a) increment on a real semantic-store DuckDB read and (b) stay at 0 when
// no read is performed. This test pins both behaviors against the single
// read chokepoint (s.queryContext / s.queryRowContext on *Store).
//
// White-box (package store) so the test can construct a *Store with a
// metrics handle and read the package-private label/metrics fields.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/obs"
)

// openStoreForReadCounterTest opens a fresh DuckDB store and returns it
// alongside the *obs.Metrics handle wired into it, so the test can assert
// exact pre/post counter deltas on helix_semantic_store_reads_total.
func openStoreForReadCounterTest(t *testing.T) (*Store, *obs.Metrics, context.Context) {
	t.Helper()
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(fresh): %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return s, m, ctx
}

// readCounterValue reads the labelless helix_semantic_store_reads_total
// counter total. Returns 0 if the family is absent.
func readCounterValue(t *testing.T, m *obs.Metrics) float64 {
	t.Helper()
	return counterValue(t, m, "helix_semantic_store_reads_total", nil)
}

// TestReadCounter is the Phase 81 read-counter contract:
//
//	subtest "increments on a real read" — a semantic-store read (one that
//	   funnels through the s.db read chokepoint) bumps the counter by ≥ 1.
//	subtest "stays 0 when no read occurs" — opening the store and performing
//	   no read leaves the counter at 0 (the bench-cell assertion's target).
func TestReadCounter(t *testing.T) {
	t.Run("increments on a real read", func(t *testing.T) {
		s, m, ctx := openStoreForReadCounterTest(t)

		before := readCounterValue(t, m)

		// QueryEffectiveAdjacency is a named back-channel read consumer
		// (the rankStoreAdapter / RankScheduler path) and routes through
		// the s.db read chokepoint. An empty store returns empty maps with
		// no error — the read still executes, so the counter must bump.
		if _, _, err := s.QueryEffectiveAdjacency(ctx, "r-it", "call_graph"); err != nil {
			t.Fatalf("QueryEffectiveAdjacency: %v", err)
		}

		after := readCounterValue(t, m)
		if after <= before {
			t.Errorf("helix_semantic_store_reads_total did not increment on a real read: before=%v after=%v", before, after)
		}
	})

	t.Run("stays 0 when no read occurs", func(t *testing.T) {
		_, m, _ := openStoreForReadCounterTest(t)

		// No read performed after Open. Open itself runs schema-version /
		// migration work, but those are NOT routed through the read
		// chokepoint (they are open/maintenance paths), so the reads
		// counter must be exactly 0.
		if v := readCounterValue(t, m); v != 0 {
			t.Errorf("helix_semantic_store_reads_total = %v after a no-read open, want 0", v)
		}
	})
}
