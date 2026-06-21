// Package store: snapshot-write API (Phase 63 P63-01).
//
// snapshot.go owns the snapshot-write contract consumed by the Phase 63
// compactor (P63-02):
//
//   - Store.BeginSnapshot(ctx, SnapshotMeta) → *Snapshot
//     Opens a DuckDB *sql.Tx, allocates a fresh `pending` row in
//     semantic_snapshots, and returns a *Snapshot handle bound to the tx.
//
//   - Store.WriteSnapshotFacts(ctx, snap, Facts)
//     Per-table parameterized inserts (`?` placeholders) for files,
//     symbols, references, edges through the snapshot's tx. NEVER uses the
//     duckdb-go Appender (RESEARCH.md Landmine 6: Appender commits are not
//     bound to *sql.Tx).
//
//   - Store.CommitSnapshot(ctx, snap, SnapshotSummary)
//     Flips status='committed', stamps committed_at, commits the tx
//     atomically. Idempotency-guarded against double-commit /
//     commit-after-abort.
//
//   - Store.AbortSnapshot(ctx, snap, reason)
//     Rolls back the tx — DuckDB ACID makes the pending row vanish; no
//     explicit DELETE needed. Idempotency-guarded against double-abort /
//     abort-after-commit.
//
//   - (*Snapshot).DeleteSnapshotsBeyond(ctx, retain)
//     Retention DELETE on the SAME snapshot tx so retention commits
//     atomically with the new snapshot creation (COMPACT-04 invariant).
//     Method on *Snapshot (not on *Store) so retention is bound to the
//     open tx by construction; rollback restores all deleted snapshots.
//
//   - (*Snapshot).ClearOverlayLE(ctx, repoID, capturedEpoch)
//     Issues `DELETE FROM semantic_live_overlay_* WHERE repo_id=? AND
//     write_epoch <= ?` on the snapshot's tx. Closes Phase 60 D-04 CAS:
//     rows committed during compaction (write_epoch > capturedEpoch)
//     survive. Encapsulates the underlying *sql.Tx — there is no exported
//     `Tx()` accessor on *Snapshot; consumers (P63-02) call
//     `snap.ClearOverlayLE` rather than reaching into the tx directly.
//
// T-63-01-03 (DoS via unbounded payload): WriteSnapshotFacts has no size
// cap. The caller (P63-02 compactor) is responsible for the pre-flight
// `OverlayRowCount` guard before invoking BeginSnapshot. P63-01 trusts the
// caller — the boundary is documented here so reviewers can find it.
//
// SECURITY (T-63-01-01 / T-63-01-04):
//   - Every INSERT / UPDATE / DELETE uses `?` parameterized binds; no
//     string concat. The grep gate in 63-01-PLAN.md acceptance criteria
//     enforces this at review time.
//   - Stays in `package store` so the cmd/vet-noduckdb boundary remains
//     intact. duckdb-go is NOT imported here — we route through the
//     existing `database/sql` *Store.db handle.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Phase 63 review IN-03: hoist control-plane string constants out of
// the BeginSnapshot INSERT. Downstream readers (admin/status, future
// cluster-side introspection) match on these values; embedding them as
// SQL string literals meant a v1.10 binary still wrote `'phase63'` for
// compaction-origin snapshots even after a follow-up phase rewrites
// the compactor.
const (
	// snapshotKindCompact is the `kind` column value stamped on every
	// row produced by the compaction pipeline (BeginSnapshot). Other
	// snapshot kinds (e.g., bootstrap, manual) would use distinct
	// literals when those producers land.
	snapshotKindCompact = "compact"

	// snapshotIndexerVersion is the `indexer_version` column value
	// stamped on every row produced by BeginSnapshot. Bumped per phase
	// so downstream readers can correlate row-level data with the
	// schema/extractor combination that wrote it. Replace with a
	// build-time constant (e.g., from internal/version) when the
	// indexer-version registry lands.
	snapshotIndexerVersion = "phase63"
)

// SnapshotMeta is the input payload to BeginSnapshot.
type SnapshotMeta struct {
	// RepoID is the workspace identifier the snapshot belongs to. Required.
	RepoID string
	// BaseSnapshotID is the prior committed snapshot the new one is the
	// successor of. Zero on the very first snapshot for the workspace.
	BaseSnapshotID uint64
	// CapturedEpoch is the overlay write_epoch the compactor CAS-read at
	// the start of the compaction cycle. Carried on the in-memory Snapshot
	// handle (not persisted to semantic_snapshots in Schema 3) and consumed
	// by ClearOverlayLE to bound the post-merge DELETE.
	CapturedEpoch uint64
	// BaseOverlayEpoch is the persistent baseline overlay epoch stamped
	// onto semantic_snapshots.base_overlay_epoch at CommitSnapshot time
	// (Phase 70 CONTEXT.md D3 / schema v6). The next incremental build
	// reads this via Store.LatestCommittedSnapshotBaseEpoch and feeds the
	// result to OverlayChangedPathsSince(repoID, baseEpoch) to enumerate
	// paths whose overlay rows arrived after the baseline.
	//
	// Plumbing: callers typically construct SnapshotMeta with this field
	// zero, then call (*Snapshot).SetBaseOverlayEpoch(epoch) after the
	// compactor has computed the post-merge epoch but BEFORE
	// CommitSnapshot. Both paths (Meta at Begin-time, or setter
	// post-Begin) write the same value through to the row.
	BaseOverlayEpoch uint64
}

// Snapshot is the per-tx handle returned by BeginSnapshot. The *sql.Tx is
// intentionally unexported — there is NO public Tx() accessor. Consumers
// who need to mutate state inside the snapshot tx call methods on
// *Snapshot (DeleteSnapshotsBeyond, ClearOverlayLE).
type Snapshot struct {
	// ID is the snapshot_id allocated at BeginSnapshot time.
	ID uint64
	// RepoID is the workspace identifier (mirrors Meta.RepoID).
	RepoID string
	// Meta is the original input passed to BeginSnapshot.
	Meta SnapshotMeta

	tx        *sql.Tx
	store     *Store // back-reference for ClearOverlayLE pending-rows reset
	createdAt time.Time
	committed bool
	aborted   bool
}

// Facts is the bulk insert payload accepted by WriteSnapshotFacts.
type Facts struct {
	Files      []FileFact
	Symbols    []SymbolFact
	References []ReferenceFact
	Edges      []EdgeFact
}

// FileFact is one row inserted into semantic_files per WriteSnapshotFacts.
// The fact-struct field set mirrors the NOT-NULL columns on semantic_files
// (migrations.go schema1Statements §9.4 + schema2Statements partial
// columns); optional partial-extraction columns default to false / "".
type FileFact struct {
	FileID       uint64
	RepoID       string
	Path         string
	Language     string
	ContentHash  string
	SizeBytes    uint64
	LineCount    int
	Generated    bool
	Ignored      bool
	IgnoreReason string
}

// SymbolFact is one row inserted into semantic_symbols.
type SymbolFact struct {
	SymbolID         uint64
	NodeID           uint64
	FileID           uint64
	Language         string
	Kind             string
	Name             string
	QualifiedName    string
	StableKey        string
	OwnerSymbolID    uint64
	ParentScopeID    uint64
	PackagePath      string
	StartByte        int
	EndByte          int
	StartLine        int
	StartCol         int
	EndLine          int
	EndCol           int
	Signature        string
	SignatureHash    string
	Exported         bool
	Visibility       string
	ExtractionSource string
	Confidence       float64
	ContentHash      string
	LSPIdentity      string
}

// ReferenceFact is one row inserted into semantic_references.
type ReferenceFact struct {
	RefID            uint64
	NodeID           uint64
	FileID           uint64
	ScopeSymbolID    uint64
	Name             string
	RefKind          string
	ReceiverText     string
	StartByte        int
	EndByte          int
	StartLine        int
	StartCol         int
	EndLine          int
	EndCol           int
	ResolvedSymbolID uint64
	ResolutionSource string
	ValidationState  string
	Confidence       float64
	Reason           string
}

// EdgeFact is one row inserted into semantic_edges.
type EdgeFact struct {
	EdgeID          uint64
	SrcNodeID       uint64
	DstNodeID       uint64
	SrcKind         string
	DstKind         string
	EdgeKind        string
	Weight          float64
	Confidence      float64
	ValidationState string
	EvidenceRefID   uint64
	EvidenceFileID  uint64
	Source          string
	Reason          string
}

// SnapshotSummary is the aggregate row metadata captured at CommitSnapshot.
// Schema 3 has no separate `semantic_snapshot_summaries` table; the counts
// are accepted by CommitSnapshot for diagnostic logging today and reserved
// for a future column-on-snapshots write once the schema lands the columns.
type SnapshotSummary struct {
	FileCount      int
	SymbolCount    int
	ReferenceCount int
	EdgeCount      int
	DurationMs     int64
}

// BeginSnapshot opens a snapshot-write transaction for meta.RepoID. It
// allocates a fresh snapshot_id from the dedicated DuckDB SEQUENCE
// `semantic_snapshot_id_seq` (Phase 63 review CR-03 — replaces the racy
// MAX+1 SELECT that produced PRIMARY KEY collisions under concurrent
// BeginSnapshot calls), inserts a pending row in semantic_snapshots, and
// returns a *Snapshot bound to the tx. The caller MUST terminate via
// CommitSnapshot or AbortSnapshot — leaking the tx leaks a DuckDB
// transaction slot.
//
// The id allocation runs OUTSIDE the snapshot tx (mirrors the
// current_epoch bump in overlay.go:116-136) so concurrent allocators
// observe a globally-monotone sequence rather than the per-tx MVCC
// snapshot of the table. The SEQUENCE is created by applyMigration005
// (schema v4→v5).
//
// CapturedEpoch is stamped on the in-memory Snapshot (not persisted in
// Schema 3) so ClearOverlayLE can later bound its DELETE without a
// separate column-add migration.
func (s *Store) BeginSnapshot(ctx context.Context, meta SnapshotMeta) (*Snapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("BeginSnapshot: nil store")
	}
	if meta.RepoID == "" {
		return nil, fmt.Errorf("BeginSnapshot: empty repoID")
	}

	// Allocate snapshot_id from the dedicated SEQUENCE on s.db (NOT inside
	// the snapshot tx) so concurrent BeginSnapshot calls observe a
	// globally-monotone allocation rather than colliding on a shared
	// per-tx MVCC snapshot of MAX(snapshot_id). Mirrors the current_epoch
	// bump pattern in overlay.go:116-136.
	var id uint64
	if err := s.db.QueryRowContext(ctx, `SELECT nextval('semantic_snapshot_id_seq')`).Scan(&id); err != nil {
		return nil, fmt.Errorf("BeginSnapshot: allocate snapshot_id: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("BeginSnapshot: open tx: %w", err)
	}

	// INSERT the pending row using the pre-allocated id. semantic_snapshots
	// requires several NOT NULL columns (repo_id, repo_root, kind,
	// worktree_hash, schema_version, indexer_version, status, created_at) —
	// provide minimal placeholder values for the columns the compactor does
	// not currently set; P63-02 (compactor) supplies real values via meta
	// extension if needed.
	//
	// Phase 63 review WR-04: created_at uses DuckDB-side `now()` rather
	// than a Go-side time.Now() bind so all timestamps in the
	// semantic_snapshots table are written by the same clock (the DB
	// engine's). Test seed paths and CommitSnapshot follow the same
	// convention, eliminating Go-vs-DB clock-skew incoherence under
	// container time-namespaces and monotonic-vs-wall clock swaps.
	// Phase 63 review IN-03: bind kind / indexer_version through `?`
	// placeholders backed by package constants instead of inline SQL
	// literals so downstream readers (admin/status, cluster
	// introspection) reason about them as configuration rather than
	// magic strings. status='pending' stays inline because it is bound
	// to the BeginSnapshot vs CommitSnapshot lifecycle, not to a
	// caller-controllable value.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO semantic_snapshots (
			snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
			worktree_hash, schema_version, indexer_version, status,
			partial, created_at
		) VALUES (
			?, ?, '', ?, ?, '', ?, ?, 'pending',
			false, now()
		)
	`, id, meta.RepoID, meta.BaseSnapshotID,
		snapshotKindCompact, CurrentSchemaVersion, snapshotIndexerVersion); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("BeginSnapshot: insert pending row: %w", err)
	}

	return &Snapshot{
		ID:        id,
		RepoID:    meta.RepoID,
		Meta:      meta,
		tx:        tx,
		store:     s,
		createdAt: time.Now(),
	}, nil
}

// WriteSnapshotFacts inserts every Fact in `facts` into the corresponding
// snapshot fact table on snap's tx. Empty Facts is a no-op (returns nil).
//
// Per-row error returns `fmt.Errorf("WriteSnapshotFacts(%s): %w", table,
// err)` WITHOUT rolling back the tx — the caller decides Abort vs continue.
func (s *Store) WriteSnapshotFacts(ctx context.Context, snap *Snapshot, facts Facts) error {
	if s == nil {
		return fmt.Errorf("WriteSnapshotFacts: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("WriteSnapshotFacts: nil snapshot")
	}
	if len(facts.Files) == 0 && len(facts.Symbols) == 0 && len(facts.References) == 0 && len(facts.Edges) == 0 {
		return nil
	}

	for _, f := range facts.Files {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_files (
				snapshot_id, file_id, repo_id, path, language, content_hash,
				size_bytes, line_count, generated, ignored, ignore_reason,
				indexed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, f.FileID, f.RepoID, f.Path, f.Language, f.ContentHash,
			f.SizeBytes, f.LineCount, f.Generated, f.Ignored,
			nullIfEmpty(f.IgnoreReason), time.Now()); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_files): %w", err)
		}
	}

	for _, s2 := range facts.Symbols {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_symbols (
				snapshot_id, symbol_id, node_id, file_id, language, kind,
				name, qualified_name, stable_key, owner_symbol_id,
				parent_scope_id, package_path, start_byte, end_byte,
				start_line, start_col, end_line, end_col, signature,
				signature_hash, exported, visibility, extraction_source,
				confidence, content_hash, lsp_identity
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, s2.SymbolID, s2.NodeID, s2.FileID, s2.Language, s2.Kind,
			s2.Name, s2.QualifiedName, s2.StableKey,
			nullIfZero(s2.OwnerSymbolID), nullIfZero(s2.ParentScopeID),
			nullIfEmpty(s2.PackagePath), s2.StartByte, s2.EndByte,
			s2.StartLine, s2.StartCol, s2.EndLine, s2.EndCol,
			nullIfEmpty(s2.Signature), nullIfEmpty(s2.SignatureHash),
			s2.Exported, nullIfEmpty(s2.Visibility),
			s2.ExtractionSource, s2.Confidence,
			nullIfEmpty(s2.ContentHash), nullIfEmpty(s2.LSPIdentity)); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_symbols): %w", err)
		}
	}

	for _, r := range facts.References {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_references (
				snapshot_id, ref_id, node_id, file_id, scope_symbol_id,
				name, ref_kind, receiver_text, start_byte, end_byte,
				start_line, start_col, end_line, end_col,
				resolved_symbol_id, resolution_source, validation_state,
				confidence, reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, r.RefID, r.NodeID, r.FileID,
			nullIfZero(r.ScopeSymbolID), r.Name, r.RefKind,
			nullIfEmpty(r.ReceiverText), r.StartByte, r.EndByte,
			r.StartLine, r.StartCol, r.EndLine, r.EndCol,
			nullIfZero(r.ResolvedSymbolID), nullIfEmpty(r.ResolutionSource),
			r.ValidationState, r.Confidence,
			nullIfEmpty(r.Reason)); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_references): %w", err)
		}
	}

	for _, e := range facts.Edges {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_edges (
				snapshot_id, edge_id, src_node_id, dst_node_id, src_kind,
				dst_kind, edge_kind, weight, confidence, validation_state,
				evidence_ref_id, evidence_file_id, source, reason,
				created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, e.EdgeID, e.SrcNodeID, e.DstNodeID, e.SrcKind,
			e.DstKind, e.EdgeKind, e.Weight, e.Confidence,
			e.ValidationState, nullIfZero(e.EvidenceRefID),
			nullIfZero(e.EvidenceFileID), e.Source,
			nullIfEmpty(e.Reason), time.Now()); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_edges): %w", err)
		}
	}

	return nil
}

// CommitSnapshot flips snap to status='committed' and commits the tx
// atomically. Double-commit and commit-after-abort are rejected with
// sentinel errors. The summary is logged but not yet persisted to a
// dedicated table (Schema 3 has no semantic_snapshot_summaries column);
// reserved for a future schema bump if/when downstream readers need it.
func (s *Store) CommitSnapshot(ctx context.Context, snap *Snapshot, summary SnapshotSummary) error {
	if s == nil {
		return fmt.Errorf("CommitSnapshot: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("CommitSnapshot: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("CommitSnapshot: already committed")
	}
	if snap.aborted {
		return fmt.Errorf("CommitSnapshot: already aborted")
	}

	// Phase 63 review WR-04: committed_at uses DuckDB-side `now()` to
	// keep timestamp clocks consistent with BeginSnapshot's `now()`
	// stamp (and the seed-helper's monotone fixture clocks). See
	// BeginSnapshot for the rationale.
	//
	// Phase 70 P70-02 (CONTEXT.md D3): also stamp base_overlay_epoch
	// atomically with the status flip — the column is the persistent
	// baseline the next incremental build queries via
	// OverlayChangedPathsSince. Callers seed snap.Meta.BaseOverlayEpoch
	// either at BeginSnapshot time (Meta field) or post-Begin via
	// (*Snapshot).SetBaseOverlayEpoch. Unset (zero) is the cold-start
	// signal — the next build with epoch=0 enumerates everything since
	// the beginning, matching pre-Phase-70 full-rebuild behavior.
	if _, err := snap.tx.ExecContext(ctx, `
		UPDATE semantic_snapshots
		   SET status='committed', committed_at=now(), base_overlay_epoch=?
		 WHERE snapshot_id=?
	`, snap.Meta.BaseOverlayEpoch, snap.ID); err != nil {
		return fmt.Errorf("CommitSnapshot: flip status: %w", err)
	}

	if err := snap.tx.Commit(); err != nil {
		return fmt.Errorf("CommitSnapshot: %w", err)
	}
	snap.committed = true

	slog.Default().Debug("snapshot committed",
		"snapshot_id", snap.ID,
		"repo_id", snap.RepoID,
		"file_count", summary.FileCount,
		"symbol_count", summary.SymbolCount,
		"reference_count", summary.ReferenceCount,
		"edge_count", summary.EdgeCount,
		"duration_ms", summary.DurationMs,
	)
	return nil
}

// AbortSnapshot rolls back the snapshot tx — DuckDB ACID makes the pending
// row disappear without an explicit DELETE. Double-abort and
// abort-after-commit are rejected.
//
// Phase 63 review WR-02: ctx is now actually consumed. *sql.Tx.Rollback()
// has no ctx-aware overload, but we (a) pipe ctx through to the slog
// emission so trace propagation reaches the abort log line, and (b)
// surface ctx.Err() in the returned error when the parent ctx is
// already cancelled at entry — the rollback is still attempted (the tx
// must not leak) but the cause is reported back to the caller.
func (s *Store) AbortSnapshot(ctx context.Context, snap *Snapshot, reason string) error {
	if s == nil {
		return fmt.Errorf("AbortSnapshot: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("AbortSnapshot: nil snapshot")
	}
	if snap.aborted {
		return fmt.Errorf("AbortSnapshot: already aborted")
	}
	if snap.committed {
		return fmt.Errorf("AbortSnapshot: already committed")
	}

	slog.Default().InfoContext(ctx, "snapshot aborted",
		"snapshot_id", snap.ID,
		"repo_id", snap.RepoID,
		"reason", reason,
	)

	rbErr := snap.tx.Rollback()
	snap.aborted = rbErr == nil

	// If the parent ctx was already cancelled at entry, surface the
	// cancellation as the primary error (the rollback is best-effort:
	// callers still expect the tx not to leak even when cancelled).
	if ctxErr := ctx.Err(); ctxErr != nil {
		if rbErr != nil {
			return fmt.Errorf("AbortSnapshot: ctx cancelled: %w (rollback also failed: %v)", ctxErr, rbErr)
		}
		return fmt.Errorf("AbortSnapshot: ctx cancelled: %w", ctxErr)
	}

	if rbErr != nil {
		return fmt.Errorf("AbortSnapshot: %w", rbErr)
	}
	return nil
}

// DeleteSnapshotsBeyond deletes every committed snapshot for snap.RepoID
// EXCEPT (a) snap itself (which is still pending in the open tx) and
// (b) the `retain-1` most-recent committed prior snapshots. Cascades to
// per-snapshot fact-table rows via explicit per-table DELETEs (the schema
// has no FK ON DELETE CASCADE today).
//
// The DELETE runs on the SAME tx as snap's pending row so retention
// commits atomically with the new snapshot creation (COMPACT-04). If the
// caller later AbortSnapshot's, the DELETE rolls back and all prior
// snapshots are restored — verified by TestSnapshot_DeleteSnapshotsBeyondAtomic.
//
// retain < 1 is rejected; retain ≥ priors+1 is a no-op.
func (snap *Snapshot) DeleteSnapshotsBeyond(ctx context.Context, retain int) error {
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("DeleteSnapshotsBeyond: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("DeleteSnapshotsBeyond: snapshot already committed")
	}
	if snap.aborted {
		return fmt.Errorf("DeleteSnapshotsBeyond: snapshot already aborted")
	}
	if retain < 1 {
		return fmt.Errorf("DeleteSnapshotsBeyond: retain must be >= 1, got %d", retain)
	}

	// Materialize the set of doomed snapshot_ids first (everything outside
	// the keep set) so we can cascade per-table deletes.
	//
	// Phase 63 review CR-04: tie-break ORDER BY on snapshot_id DESC so
	// the survivor set is deterministic when two snapshots share an
	// indistinguishable created_at (sub-microsecond writes; container
	// time-namespace skew). Without the tie-break, DuckDB's
	// implementation-defined tie resolution makes retention behavior
	// flaky under retain<priors and same-clock-bucket creation.
	rows, err := snap.tx.QueryContext(ctx, `
		SELECT snapshot_id FROM semantic_snapshots
		 WHERE repo_id = ? AND snapshot_id != ?
		   AND snapshot_id NOT IN (
		       SELECT snapshot_id FROM semantic_snapshots
		        WHERE repo_id = ? AND snapshot_id != ?
		          AND status = 'committed'
		        ORDER BY created_at DESC, snapshot_id DESC
		        LIMIT ?
		   )
	`, snap.RepoID, snap.ID, snap.RepoID, snap.ID, retain-1)
	if err != nil {
		return fmt.Errorf("DeleteSnapshotsBeyond: select doomed: %w", err)
	}
	doomed := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("DeleteSnapshotsBeyond: scan doomed: %w", err)
		}
		doomed = append(doomed, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("DeleteSnapshotsBeyond: iter doomed: %w", err)
	}
	rows.Close()
	if len(doomed) == 0 {
		return nil
	}

	// Per-table cascade. The schema doesn't enforce FK cascade, so we issue
	// explicit deletes for every fact table keyed on snapshot_id. Order is
	// not load-bearing — all run in the same tx.
	//
	// Phase 63 review WR-05: collapse the prior len(doomed)×7 ExecContext
	// loop into one DELETE per table by binding the doomed ids as a
	// parameter-expansion list (`IN (?, ?, ...)`). Placeholder-string
	// generation derives ONLY from len(doomed) — the SQL itself remains
	// hardcoded except for the per-statement table name and the
	// expansion of `?`s; no caller-derived data flows into the SQL
	// string. The doomed ids continue to flow through positional `?`
	// binds, preserving the parameterization invariant.
	tables := []string{
		"semantic_files",
		"semantic_symbols",
		"semantic_references",
		"semantic_edges",
		"semantic_nodes",
		"semantic_diagnostics",
		"semantic_snapshots",
	}
	placeholders := strings.Repeat("?,", len(doomed))
	placeholders = placeholders[:len(placeholders)-1] // drop trailing ","
	args := make([]any, len(doomed))
	for i, id := range doomed {
		args[i] = id
	}
	for _, table := range tables {
		// Concatenation is bounded to two trusted constants (table name
		// from the hardcoded slice above + placeholder string built from
		// len(doomed)). No caller-supplied data flows into the SQL.
		stmt := "DELETE FROM " + table + " WHERE snapshot_id IN (" + placeholders + ")"
		if _, err := snap.tx.ExecContext(ctx, stmt, args...); err != nil {
			return fmt.Errorf("DeleteSnapshotsBeyond(%s): %w", table, err)
		}
	}
	return nil
}

// ClearOverlayLE issues `DELETE FROM semantic_live_overlay_* WHERE
// repo_id = ? AND write_epoch <= ?` on snap's underlying tx, for each of
// the four overlay fact tables. The deletes commit atomically with snap's
// snapshot creation (rollback restores all overlay rows); rows committed
// during compaction (write_epoch > capturedEpoch) survive — closing the
// Phase 60 D-04 CAS contract.
//
// Encapsulates the underlying *sql.Tx. P63-02's compactor calls
// snap.ClearOverlayLE rather than reaching into snap.tx directly — there
// is no public Tx() accessor on *Snapshot.
//
// Pending-rows counter accounting (Phase 63 review CR-02): the in-memory
// pending-rows counter (overlayPendingRowsFor / OverlayHasPendingRows) is
// the proxy for the gate's BlockedOverlayEmpty check. Pre-fix, this
// method unconditionally Store(0)'d the counter — but the CAS contract
// preserves rows with write_epoch > capturedEpoch, and forcing the
// counter to zero in their presence would make OverlayHasPendingRows
// return false while pending rows still exist on disk, deadlocking the
// gate at BlockedOverlayEmpty until the next concurrent overlay write
// re-stamped the counter. Post-fix, we sum the per-table RowsAffected
// from the four DELETEs and atomically subtract that count from the
// in-memory counter (clamped at zero so a counter that under-counted
// pre-existing rows can never go negative). The subtract must happen
// AFTER all four DELETEs succeed.
func (snap *Snapshot) ClearOverlayLE(ctx context.Context, repoID string, capturedEpoch uint64) error {
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("ClearOverlayLE: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("ClearOverlayLE: snapshot already committed")
	}
	if snap.aborted {
		return fmt.Errorf("ClearOverlayLE: snapshot already aborted")
	}
	if repoID == "" {
		return fmt.Errorf("ClearOverlayLE: empty repoID")
	}

	// Per-table DELETE on the snapshot's own tx. Each statement is a
	// hardcoded constant (no string interpolation, no caller-derived data
	// in the SQL) so the parameterization invariant is preserved: only
	// repo_id and write_epoch flow through `?` binds.
	overlayDeletes := []struct {
		name string
		sql  string
	}{
		{"semantic_live_overlay_files", `DELETE FROM semantic_live_overlay_files WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_symbols", `DELETE FROM semantic_live_overlay_symbols WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_references", `DELETE FROM semantic_live_overlay_references WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_edges", `DELETE FROM semantic_live_overlay_edges WHERE repo_id = ? AND write_epoch <= ?`},
	}
	var totalDeleted int64
	for _, d := range overlayDeletes {
		res, err := snap.tx.ExecContext(ctx, d.sql, repoID, capturedEpoch)
		if err != nil {
			return fmt.Errorf("ClearOverlayLE(%s): %w", d.name, err)
		}
		// RowsAffected may return an error on drivers that do not support
		// it; we tolerate that by treating it as zero (the counter then
		// drifts conservatively high and the gate stays unblocked at
		// worst — preferable to the pre-fix blanket zero-out).
		if n, raerr := res.RowsAffected(); raerr == nil && n > 0 {
			totalDeleted += n
		}
	}

	// Phase 63 review CR-02: AFTER all four DELETEs succeed, decrement the
	// in-memory pending-rows counter by the number of rows deleted (NOT
	// unconditionally zero it). Rows committed during compaction with
	// write_epoch > capturedEpoch are NOT deleted and MUST keep the
	// counter positive so the next gate evaluation correctly observes
	// OverlayHasPendingRows == true.
	subtractOverlayPendingRowsIfPresent(snap.store, repoID, totalDeleted)

	return nil
}

// subtractOverlayPendingRowsIfPresent atomically decrements the *Store's
// per-workspace pending-rows counter by n, clamped at zero. Phase 63
// review CR-02: replaces the unconditional Store(0) reset that
// destroyed the OverlayHasPendingRows signal in the presence of rows
// committed during compaction (write_epoch > capturedEpoch — which
// SURVIVE ClearOverlayLE per the Phase 60 D-04 CAS contract).
//
// Called from Snapshot.ClearOverlayLE AFTER the four overlay-table
// DELETEs succeed — the AFTER ordering is load-bearing: if the
// decrement ran before the DELETEs the gate could observe
// OverlayHasPendingRows == false transiently while overlay rows still
// existed on disk. nil-safe.
//
// Clamp-at-zero is defensive: the counter is bumped by overlay-write
// helpers that may under-count (e.g., if a row is overwritten by an
// ON CONFLICT DO UPDATE the counter is bumped twice but only one row
// exists); a strict subtract could otherwise underflow. The clamp
// preserves the gate's monotone-towards-zero invariant.
func subtractOverlayPendingRowsIfPresent(s *Store, repoID string, n int64) {
	if s == nil || repoID == "" || n <= 0 {
		return
	}
	s.overlayCountsMu.Lock()
	c, ok := s.overlayPendingRows[repoID]
	s.overlayCountsMu.Unlock()
	if !ok || c == nil {
		return
	}
	for {
		cur := c.Load()
		next := cur - n
		if next < 0 {
			next = 0
		}
		if c.CompareAndSwap(cur, next) {
			return
		}
	}
}

// SetBaseOverlayEpoch records the baseline overlay epoch the compactor
// has just computed (post-merge `current_epoch`) on the in-memory
// *Snapshot handle. CommitSnapshot consumes this value and writes it to
// `semantic_snapshots.base_overlay_epoch` in the same tx as the status
// flip (Phase 70 CONTEXT.md D3). Idempotent — repeated calls with the
// same or different values are allowed up until CommitSnapshot reads
// the field.
//
// Plumbing intent (per 70-02-PLAN.md action step 2a): callers
// (Plan 04's buildFn) BeginSnapshot → run the merge → SetBaseOverlayEpoch
// with the post-merge epoch → CommitSnapshot. Setting after a Commit /
// Abort is a no-op on the persisted row but mutates the in-memory
// field; callers should not rely on this and the order is enforced by
// the existing committed/aborted guards in CommitSnapshot.
func (snap *Snapshot) SetBaseOverlayEpoch(epoch uint64) {
	if snap == nil {
		return
	}
	snap.Meta.BaseOverlayEpoch = epoch
}

// LatestCommittedSnapshotBaseEpoch returns the `base_overlay_epoch`
// value of the most-recent committed snapshot for repoID — the
// persistent baseline Phase 70 Plan 04's incremental buildFn feeds to
// OverlayChangedPathsSince(repoID, baseEpoch) to enumerate paths whose
// overlay rows arrived after the baseline.
//
// Returns:
//   - (epoch, true, nil)  on hit (at least one committed snapshot exists)
//   - (0, false, nil)     on cold-start (no committed snapshot for repoID)
//   - (0, false, err)     on any other DB failure (wrapped)
//
// The query orders by snapshot_id DESC (the SEQUENCE-allocated id is
// monotone across BeginSnapshot calls per Phase 63 CR-03, so the
// highest id is the most recently created committed snapshot) and
// filters on status='committed' so pending or aborted snapshots never
// shadow a committed reading.
//
// Receiver-on-*Store (not on *Snapshot) so the daemon's StoreAccessor
// wrapper can route to this accessor without holding a snapshot handle.
// Nil-store and empty-repoID are rejected to mirror the
// OverlayChangedPathsSince discipline (Plan 01).
func (s *Store) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (epoch uint64, ok bool, err error) {
	if s == nil || s.db == nil {
		return 0, false, fmt.Errorf("LatestCommittedSnapshotBaseEpoch: nil store")
	}
	if repoID == "" {
		return 0, false, fmt.Errorf("LatestCommittedSnapshotBaseEpoch: empty repoID")
	}
	row := s.queryRowContext(ctx, `
		SELECT base_overlay_epoch
		  FROM semantic_snapshots
		 WHERE repo_id = ? AND status = 'committed'
		 ORDER BY snapshot_id DESC
		 LIMIT 1
	`, repoID)
	var got uint64
	if scanErr := row.Scan(&got); scanErr != nil {
		// sql.ErrNoRows → cold-start; any other error is wrapped.
		if errors.Is(scanErr, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("LatestCommittedSnapshotBaseEpoch(%q): %w", repoID, scanErr)
	}
	return got, true, nil
}

// nullIfEmpty maps "" → nil (so the database/sql driver writes SQL NULL)
// and any non-empty string passes through unchanged. Used for nullable
// TEXT columns where the runtime "no value" must round-trip as NULL rather
// than an empty string (the SPEC distinguishes the two for several
// columns: e.g., signature, partial_reason, ignore_reason).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullIfZero maps 0 → nil for nullable UBIGINT columns (e.g.,
// owner_symbol_id, parent_scope_id, scope_symbol_id, evidence_ref_id) so
// the absence of a referenced row is represented as SQL NULL rather than
// a literal 0 that would later collide with a real id=0 row.
func nullIfZero(v uint64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
