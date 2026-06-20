// Package store: filefact_accessor.go implements the Phase 68 DIFF-02
// seam — *Store.GetLatestFileFact — the pre-edit FileFact accessor that
// Phase 68's Tier-1 populator (Plan 68-04) diffs against.
//
// Deviation from 68-01-PLAN.md (Pitfall 1 in 68-RESEARCH.md): the plan
// proposes PriorFileFact.Symbols be []extract.SymbolFact, but
// internal/semantic/extract already imports internal/semantic/store
// (extract/to_store.go:46) for the ToStoreFacts adapter. Mirroring
// extract.SymbolFact in store would create an import cycle. The plan
// flags this exact case under D-01 "Claude's Discretion" — Pitfall 1
// states "diverge from the CONTEXT.md sketch and return a new
// store.PriorFileFact (or equivalent) carrying Symbols []extract.SymbolFact"
// but []extract.SymbolFact directly is unworkable, so we go one step
// further and carry a store-local PriorSymbol shape with the fields the
// Tier-1 diff actually consumes. The handler (which CAN import both
// packages) can convert in either direction when 68-04 lands.
//
// Reads `semantic_live_overlay_*` first; falls back to the latest
// committed snapshot. Returns `(PriorFileFact{}, false, nil)` on
// cold-start. Lives entirely within internal/semantic/store so the
// kernel↔semantic vet boundary stays intact (Phase 68 D-02).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agenthands/helix/internal/semantic"
)

// PriorSymbol is the store-local symbol shape returned by
// GetLatestFileFact. Mirrors the subset of extract.SymbolFact fields
// the Tier-1 diff consumes. Field names match extract.SymbolFact so
// JSON unmarshalling from semantic_live_overlay_symbols.fact_json (the
// JSON form of extract.SymbolFact written by Phase 60 P04 — when it
// lands) Just Works.
type PriorSymbol struct {
	ID            semantic.SymbolID
	StableKey     string
	Name          string
	Kind          string
	Signature     string
	SignatureHash string
	Visibility    string
}

// PriorFileFact is the pre-edit FileFact view returned by
// GetLatestFileFact. ExtractionStatus is a plain string (carries
// extract.ExtractionStatus values like "ready" / "partial" — kept as
// string here to avoid the extract → store cycle).
type PriorFileFact struct {
	Path             string
	Language         string
	ExtractionStatus string
	Symbols          []PriorSymbol
}

// GetLatestFileFact returns the most-recent FileFact visible for
// (repoID, path). Resolution order:
//
//  1. semantic_live_overlay_files (status='live', file_id != 0) +
//     semantic_live_overlay_symbols (status='live', fact_json present).
//     Phase 60 P02 inserts placeholder rows with file_id=0; those are
//     skipped (treated as if absent) so the snapshot fallback fires.
//  2. semantic_files / semantic_symbols at LatestCommittedSnapshot.
//
// Returns (zero, false, nil) on cold-start (no overlay row, no
// committed snapshot). Returns a wrapped error only on infrastructure
// failures (DuckDB I/O). Per-symbol JSON unmarshal failures on the
// overlay branch are non-fatal: the bad row is skipped and the rest
// surface (Pitfall in 68-RESEARCH.md Example 1 line 571).
func (s *Store) GetLatestFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
	if s == nil || s.db == nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: nil store")
	}
	if repoID == "" {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: empty repoID")
	}
	if path == "" {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact: empty path")
	}

	fact, ok, err := s.readOverlayFileFact(ctx, repoID, path)
	if err != nil {
		return PriorFileFact{}, false, err
	}
	if ok {
		return fact, true, nil
	}
	return s.readSnapshotFileFact(ctx, repoID, path)
}

// readOverlayFileFact: SELECT live overlay file row, then its live
// symbol rows. Cold-start / placeholder maps to (zero, false, nil) so
// the caller falls through to the snapshot branch.
func (s *Store) readOverlayFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
	var fileID uint64
	var lang sql.NullString
	err := s.queryRowContext(ctx, `
		SELECT file_id, language FROM semantic_live_overlay_files
		 WHERE repo_id = ? AND path = ? AND status = 'live'
	`, repoID, path).Scan(&fileID, &lang)
	if errors.Is(err, sql.ErrNoRows) {
		return PriorFileFact{}, false, nil
	}
	if err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay file lookup: %w", repoID, path, err)
	}
	if fileID == 0 {
		// Phase 60 P02 placeholder row; let snapshot fallback handle it.
		return PriorFileFact{}, false, nil
	}

	rows, err := s.queryContext(ctx, `
		SELECT fact_json::VARCHAR
		  FROM semantic_live_overlay_symbols
		 WHERE repo_id = ? AND file_id = ? AND status = 'live'
		   AND fact_json IS NOT NULL
	`, repoID, fileID)
	if err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol query: %w", repoID, path, err)
	}
	defer rows.Close()

	var symbols []PriorSymbol
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol scan: %w", repoID, path, err)
		}
		var sym PriorSymbol
		if err := json.Unmarshal([]byte(raw), &sym); err != nil {
			// Non-fatal: skip malformed rows; matches 68-RESEARCH.md
			// Example 1 disposition (corrupt JSON → drop row, continue).
			continue
		}
		symbols = append(symbols, sym)
	}
	if err := rows.Err(); err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol rows: %w", repoID, path, err)
	}

	return PriorFileFact{
		Path:             path,
		Language:         lang.String,
		ExtractionStatus: "ready",
		Symbols:          symbols,
	}, true, nil
}

// readSnapshotFileFact: SELECT against the latest committed snapshot.
// (zero, false, nil) on cold-start (no committed snapshot OR snapshot
// doesn't contain the path).
func (s *Store) readSnapshotFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error) {
	snapID, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): %w", repoID, path, err)
	}
	if snapID == 0 {
		return PriorFileFact{}, false, nil
	}

	// Resolve file_id + language for the path within this snapshot.
	var fileID uint64
	var lang string
	err = s.queryRowContext(ctx, `
		SELECT file_id, language FROM semantic_files
		 WHERE snapshot_id = ? AND repo_id = ? AND path = ?
	`, snapID, repoID, path).Scan(&fileID, &lang)
	if errors.Is(err, sql.ErrNoRows) {
		return PriorFileFact{}, false, nil
	}
	if err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): snapshot file lookup: %w", repoID, path, err)
	}

	rows, err := s.queryContext(ctx, `
		SELECT symbol_id, stable_key, name, kind,
		       COALESCE(signature, ''), COALESCE(signature_hash, ''),
		       COALESCE(visibility, ''), COALESCE(exported, false)
		  FROM semantic_symbols
		 WHERE snapshot_id = ? AND file_id = ?
	`, snapID, fileID)
	if err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): snapshot symbol query: %w", repoID, path, err)
	}
	defer rows.Close()

	var symbols []PriorSymbol
	for rows.Next() {
		var (
			symID         uint64
			stableKey     string
			name          string
			kind          string
			signature     string
			signatureHash string
			visibility    string
			exported      bool
		)
		if err := rows.Scan(&symID, &stableKey, &name, &kind, &signature, &signatureHash, &visibility, &exported); err != nil {
			return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): snapshot symbol scan: %w", repoID, path, err)
		}
		if visibility == "" {
			// Open Question Q2: Exported bool → Visibility string.
			if exported {
				visibility = "exported"
			} else {
				visibility = "private"
			}
		}
		symbols = append(symbols, PriorSymbol{
			ID:            semantic.SymbolID(symID),
			StableKey:     stableKey,
			Name:          name,
			Kind:          kind,
			Signature:     signature,
			SignatureHash: signatureHash,
			Visibility:    visibility,
		})
	}
	if err := rows.Err(); err != nil {
		return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): snapshot symbol rows: %w", repoID, path, err)
	}

	return PriorFileFact{
		Path:             path,
		Language:         lang,
		ExtractionStatus: "ready",
		Symbols:          symbols,
	}, true, nil
}
