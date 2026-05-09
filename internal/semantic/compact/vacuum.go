// Package compact: Phase 63 P63-02 Task 2.
//
// vacuum.go is a thin gate-and-call wrapper. The BeginTx →
// ExecContext("VACUUM") → Commit triple lives in
// internal/semantic/store/vacuum.go (warning #5 review fix) so DDL/DML
// SQL stays out of internal/semantic/compact.
//
// VACUUM is currently a no-op-by-DuckDB
// (https://duckdb.org/docs/sql/statements/vacuum — "the VACUUM
// statement reclaims storage by ... currently a no-op"). The
// infrastructure here (config gate, gate.IsReady re-check, separate tx
// encapsulated inside store.Vacuum, dedicated trace span call site,
// dedicated metric outcome label) is fully wired so a future phase can
// swap COPY FROM DATABASE repack into store.Vacuum without
// re-architecting compact.

package compact

import "context"

// maybeVacuum gates the VACUUM call on (a) cfg.VacuumEnabled, (b)
// elapsed VacuumInterval, (c) a fresh gate.IsReady check (window may
// have closed between commit and now). On fire: invokes
// store.Vacuum(ctx) (separate tx — D-05 hard invariant) and persists
// last_vacuum_at via OverlayOps.UpdateLastVacuumAt.
//
// The metric outcome label flows to SemanticVacuumObserve:
//   - "success" — VACUUM ran without error
//   - "skipped" — gates short-circuited
//   - "error"   — store.Vacuum returned a non-nil error
func (c *Compactor) maybeVacuum(ctx context.Context) (outcome string) {
	outcome = "skipped"
	start := c.now()
	defer func() {
		if c.deps.Metrics != nil {
			c.deps.Metrics.SemanticVacuumObserve(outcome, c.now().Sub(start).Seconds())
		}
	}()

	if !c.cfg.VacuumEnabled {
		return
	}
	if !c.lastVacuumAt.IsZero() && c.now().Sub(c.lastVacuumAt) < c.cfg.VacuumInterval {
		return
	}

	// Re-check gate (state may have shifted between CommitSnapshot and
	// here — e.g., a fresh edit landed during checkpoint).
	if c.deps.Gate != nil {
		ready, _ := c.deps.Gate.IsReady(c.workspaceID)
		if !ready {
			return
		}
	}

	if c.deps.Store == nil {
		return
	}

	// Separate tx — encapsulated inside store.Vacuum.
	if err := c.deps.Store.Vacuum(ctx); err != nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.maybeVacuum: store.Vacuum failed",
			"repo_id", c.repoID, "err", err)
		return
	}

	// Persist last_vacuum_at via the migration004 column.
	if c.deps.OverlayOps != nil {
		if err := c.deps.OverlayOps.UpdateLastVacuumAt(ctx, c.repoID, c.now()); err != nil {
			c.deps.Logger.Warn("compact.maybeVacuum: UpdateLastVacuumAt failed",
				"repo_id", c.repoID, "err", err)
		}
	}
	c.lastVacuumAt = c.now()
	outcome = "success"
	return
}
