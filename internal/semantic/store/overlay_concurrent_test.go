package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestOverlayEpochConcurrent is the load-bearing concurrency proof for the
// Phase 60 D-04 epoch contract. N=64 goroutines race against a SINGLE
// workspace; each calls BeginOverlayTx, writes one overlay-file row stamped
// with its tx.epoch, and commits. Post-join we assert:
//
//   - max(write_epoch) == 64
//   - every integer in [1..64] appears in semantic_live_overlay_files
//   - no duplicate epochs (no two tx received the same value)
//
// Run under `-race` to surface any unguarded shared-state access in
// BeginOverlayTx / overlayLockFor / OverlayTx.unlock. The race detector is
// the primary tool for catching the failure modes this test addresses
// (T-60-02-01 Tampering: non-monotone epochs).
//
// CONTRACT-CRITICAL: this test is referenced by 60-02-PLAN.md verification
// gate `go test ./internal/semantic/store/... -run TestOverlayEpochConcurrent
// -race -count=1`. If you rename it, update the plan's verification block.
func TestOverlayEpochConcurrent(t *testing.T) {
	const N = 64

	s, _, _ := openStoreForOverlayTest(t)

	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			ctx := context.Background()
			tx, err := s.BeginOverlayTx(ctx, "ws-shared")
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: BeginOverlayTx: %w", i, err)
				return
			}
			// Stamp the tx's epoch onto a unique-per-goroutine path so we
			// can read back N distinct rows. Use the goroutine index in the
			// path; the epoch (1..N in some assignment) lands in the
			// write_epoch column independently.
			path := fmt.Sprintf("src/g_%03d.go", i)
			if err := tx.UpsertOverlayFile(ctx, path, fmt.Sprintf("hash-%d", i)); err != nil {
				_ = tx.Rollback()
				errs <- fmt.Errorf("goroutine %d: UpsertOverlayFile: %w", i, err)
				return
			}
			if err := tx.Commit(); err != nil {
				errs <- fmt.Errorf("goroutine %d: Commit: %w", i, err)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("worker error: %v", err)
	}
	if t.Failed() {
		return
	}

	// All N goroutines committed — assert epoch invariants.
	rows, err := s.db.Query(`SELECT write_epoch
		FROM semantic_live_overlay_files
		WHERE repo_id = 'ws-shared'
		ORDER BY write_epoch`)
	if err != nil {
		t.Fatalf("read write_epochs: %v", err)
	}
	defer rows.Close()

	seen := map[uint64]bool{}
	var max uint64
	for rows.Next() {
		var e uint64
		if err := rows.Scan(&e); err != nil {
			t.Fatalf("Scan write_epoch: %v", err)
		}
		if seen[e] {
			t.Errorf("duplicate write_epoch=%d (T-60-02-01: non-monotone epoch allocation)", e)
		}
		seen[e] = true
		if e > max {
			max = e
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if len(seen) != N {
		t.Errorf("unique epoch count: got %d, want %d (each tx must receive a distinct epoch)", len(seen), N)
	}
	if max != N {
		t.Errorf("max write_epoch: got %d, want %d (epoch counter must be monotone with no gaps under serialized BeginOverlayTx)", max, N)
	}
	for e := uint64(1); e <= N; e++ {
		if !seen[e] {
			t.Errorf("missing epoch %d in [1..%d] (epoch allocation has a gap)", e, N)
		}
	}

	// Meta row's current_epoch must equal N.
	var ce uint64
	if err := s.db.QueryRow(`SELECT current_epoch FROM semantic_live_overlay_meta
		WHERE repo_id = 'ws-shared'`).Scan(&ce); err != nil {
		t.Fatalf("read current_epoch: %v", err)
	}
	if ce != N {
		t.Errorf("after %d serialized tx: current_epoch=%d, want %d", N, ce, N)
	}
}
