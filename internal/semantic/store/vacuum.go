// Package store: Phase 63 P63-02 Task 1 — VACUUM encapsulation.
//
// vacuum.go owns the BeginTx → ExecContext("VACUUM") → Commit triple so
// the DDL/DML SQL stays inside internal/semantic/store. The caller in
// internal/semantic/compact/vacuum.go is a thin gate-and-call wrapper
// that owns ONLY the VacuumEnabled gate, the interval gate, the
// gate.IsReady re-check, the trace span, and the metric — never SQL.
// This is the warning-#5 review fix codified at the package boundary.
//
// VACUUM is currently a no-op-by-DuckDB (https://duckdb.org/docs/sql/statements/vacuum
// — "the VACUUM statement reclaims storage by ... currently a no-op").
// The infrastructure (separate tx, idempotency on rollback, error wrap)
// is in place so a future phase can swap in COPY FROM DATABASE repack
// without touching the caller surface.

package store

import (
	"context"
	"fmt"
)

// Vacuum runs `VACUUM` inside its own transaction. Phase 63 P63-02 Task
// 1 / D-05: the VACUUM statement MUST run in a separate tx — never
// piggybacked on the snapshot tx — so the lock window stays bounded if
// a future phase swaps in real repack semantics.
//
// nil-safe.
func (s *Store) Vacuum(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("Vacuum: nil store")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Vacuum: open tx: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "VACUUM"); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("Vacuum: exec: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Vacuum: commit: %w", err)
	}
	return nil
}

// UpdateLastVacuumAt stamps the per-workspace `last_vacuum_at` column on
// `semantic_live_overlay_meta` (added by migration004). Phase 63 P63-02
// Task 1 / D-05 storage. The compactor invokes this after a successful
// Vacuum() so the next idle-interval check can read the previous run.
//
// No-op when the meta row does not exist (UPDATE affects zero rows).
// nil-safe.
func (s *Store) UpdateLastVacuumAt(ctx context.Context, repoID string, t interface{}) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("UpdateLastVacuumAt: nil store")
	}
	if repoID == "" {
		return fmt.Errorf("UpdateLastVacuumAt: empty repoID")
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE semantic_live_overlay_meta
		   SET last_vacuum_at = ?
		 WHERE repo_id = ?
	`, t, repoID); err != nil {
		return fmt.Errorf("UpdateLastVacuumAt(%q): %w", repoID, err)
	}
	return nil
}
