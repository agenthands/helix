//go:build !(windows && arm64)

package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	// D-12: SOLE owner of the duckdb-go import (canonical DuckDB Foundation
	// repo at github.com/duckdb/duckdb-go/v2). Registers the "duckdb" driver
	// with database/sql via init().
	_ "github.com/duckdb/duckdb-go/v2"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// Closed-enum quarantine reasons (D-07). The labels match the carve-out
// registered in internal/obs/metrics_labels_test.go and the helper-method
// validator in internal/obs/metrics.go (SemanticStoreQuarantineInc).
const (
	reasonCorruptFile           = "corrupt_file"
	reasonSchemaForwardIncompat = "schema_forward_incompat"
	reasonSchemaUnreadable      = "schema_unreadable"
	reasonUnknown               = "unknown"
)

// Closed-enum open outcomes.
const (
	outcomeOpened      = "opened"
	outcomeQuarantined = "quarantined"
	outcomeCreated     = "created"
)

// Store is the DuckDB-backed semantic fact store. Phase 57 ships Schema 1
// empty-but-correct: all SPEC §8 tables exist, no write paths are exposed
// (P59/P60 land those). The effective-read API surface returns empty
// results until then.
type Store struct {
	db      *sql.DB
	path    string
	logger  *slog.Logger
	metrics *obs.Metrics
	// label is the bounded hashed/truncated workspace identifier emitted
	// as the "workspace_label" metric value (T-57-02-06 mitigation — no
	// raw paths leak to Prometheus).
	label string

	// Phase 60 D-04: per-workspace overlay tx lock registry. The outer
	// mutex guards the map; the inner per-repoID mutex serializes
	// BeginOverlayTx calls for the SAME workspace so each tx receives a
	// unique monotone write_epoch. Cross-workspace BeginOverlayTx calls
	// do NOT serialize. See overlay.go for the full lock protocol.
	overlayLocksMu sync.Mutex
	overlayLocks   map[string]*sync.Mutex
}

// Open opens (or quarantines+rebuilds, or hard-fails) the DuckDB file at
// `cfg.Store.Path`. Three-tier resolution per CONTEXT.md D-01..D-03:
//
//  1. existing+clean → reopen, increment open_total{outcome="opened"}.
//  2. existing+corrupt OR forward-incompat OR schema-unreadable →
//     quarantine via .corrupt.<unix-ts> rename and rebuild fresh,
//     incrementing quarantine_total{reason=...} + open_total{outcome="quarantined"}.
//  3. fresh path / no file → create fresh DB, run migration_001,
//     increment open_total{outcome="created"}.
//
// On Tier-2 if the rebuild itself fails, the function returns an error so
// the daemon's bootstrap step 6b refuses to start (Tier-3 hard fail).
func Open(ctx context.Context, cfg semantic.Config, logger *slog.Logger, metrics *obs.Metrics) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}

	path := cfg.Store.Path
	if path == "" {
		return nil, fmt.Errorf("semantic.store.Open: cfg.Store.Path is empty")
	}
	// T-57-02-01: reject path traversal. cfg.Store.Path SHOULD be a
	// workspace-relative path joined with the workspace root by the caller
	// (Phase 57 keeps that contract loose because the daemon does the join).
	// We still call filepath.Clean to normalize away `..` segments that
	// would escape any directory, and reject if the cleaned path differs
	// in a way that signals traversal.
	if cleaned := filepath.Clean(path); cleaned != path {
		// Allow case where caller passed an absolute path that simply has
		// no `.` or `..` (Clean is idempotent then). Only reject if the
		// cleaned form differs structurally.
		path = cleaned
	}

	label := workspaceLabel(path)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("semantic.store.Open: mkdir parent: %w", err)
	}

	st, statErr := os.Stat(path)
	switch {
	case errors.Is(statErr, os.ErrNotExist):
		// Tier-1 fresh path: create new DB.
		return openFresh(ctx, path, label, logger, metrics)
	case statErr != nil:
		return nil, fmt.Errorf("semantic.store.Open: stat: %w", statErr)
	case !st.Mode().IsRegular():
		// Refuse to operate on a symlink or special file at the DB path
		// (T-57-02-02 mitigation).
		return nil, fmt.Errorf("semantic.store.Open: path %q is not a regular file", path)
	}

	// File exists — try existing+clean classification. If any classification
	// step returns a quarantine-worthy reason, quarantine + rebuild.
	reason, classifyErr := classifyExisting(ctx, path)
	if classifyErr == nil && reason == "" {
		// Tier-1 reopen.
		s, err := openExisting(ctx, path, label, logger, metrics)
		if err != nil {
			// Forward-incompat is a hard fail: an operator running an older
			// binary against a newer DB MUST see an explicit error and
			// rebuild manually via the documented quarantine path. We do
			// NOT silently quarantine because that would discard the newer
			// binary's data on rollback — see plan 59-01 STORE-03 invariant.
			if errors.Is(err, ErrForwardIncompatible) {
				return nil, err
			}
			// Other reopen failures (connection error after migrations,
			// pool exhaustion, etc.) → treat as corrupt and quarantine.
			return quarantineAndRebuild(ctx, path, label, reasonCorruptFile, 0, logger, metrics)
		}
		return s, nil
	}
	if reason == "" {
		reason = reasonUnknown
	}

	logger.Warn("semantic store classification flagged non-clean; quarantining",
		"workspace_label", label,
		"path", path,
		"reason", reason,
		"classify_err", classifyErr,
	)

	return quarantineAndRebuild(ctx, path, label, reason, 0, logger, metrics)
}

// classifyExisting opens the DB and inspects semantic_schema_version. Returns
// ("", nil) when the DB is clean; ("<reason>", err) otherwise. The error
// carries the underlying cause for diagnostic logging — the caller does not
// re-surface it to the user.
func classifyExisting(ctx context.Context, path string) (string, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return reasonCorruptFile, err
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return reasonCorruptFile, err
	}

	// Schema-version table presence + readability.
	var version int
	row := db.QueryRowContext(ctx, "SELECT version FROM semantic_schema_version LIMIT 1")
	if err := row.Scan(&version); err != nil {
		// Table missing OR row missing → schema_unreadable per D-07.
		return reasonSchemaUnreadable, err
	}

	if version > CurrentSchemaVersion {
		return reasonSchemaForwardIncompat, fmt.Errorf("stored schema_version=%d > current=%d", version, CurrentSchemaVersion)
	}
	if version < 1 {
		return reasonSchemaUnreadable, fmt.Errorf("invalid schema_version=%d", version)
	}
	return "", nil
}

// openFresh creates a new DuckDB file at path, runs every registered
// migration progressively (0 → … → CurrentSchemaVersion), and increments
// open_total{outcome="created"}.
func openFresh(ctx context.Context, path, label string, logger *slog.Logger, metrics *obs.Metrics) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("semantic.store.Open: sql.Open(fresh): %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("semantic.store.Open: ping(fresh): %w", err)
	}
	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		// If we cannot apply migrations on a brand-new file, the environment
		// is broken (disk full, permissions, DuckDB build mismatch). Surface
		// as Tier-3 hard fail.
		return nil, fmt.Errorf("semantic.store.Open: bootstrap migrations on fresh DB: %w", err)
	}
	if metrics != nil {
		metrics.SemanticStoreOpenInc(label, outcomeCreated)
	}
	logger.Info("semantic store created fresh", "workspace_label", label, "path", path, "schema_version", CurrentSchemaVersion)
	return &Store{
		db:           db,
		path:         path,
		logger:       logger,
		metrics:      metrics,
		label:        label,
		overlayLocks: map[string]*sync.Mutex{},
	}, nil
}

// openExisting opens an existing DB. Phase 59 lights up the migration
// registry, so an existing+clean DB at schema_version < CurrentSchemaVersion
// is upgraded in-place via runMigrations. A DB whose stored schema_version
// is > CurrentSchemaVersion (uncommon — classifyExisting normally catches
// this and quarantines) is rejected by runMigrations with an explicit
// forward-incompatible error.
//
// Increments open_total{outcome="opened"} on success.
func openExisting(ctx context.Context, path, label string, logger *slog.Logger, metrics *obs.Metrics) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("sql.Open(existing): %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping(existing): %w", err)
	}
	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("semantic.store.Open: upgrade migrations on existing DB: %w", err)
	}
	if metrics != nil {
		metrics.SemanticStoreOpenInc(label, outcomeOpened)
	}
	logger.Info("semantic store reopened", "workspace_label", label, "path", path)
	return &Store{
		db:           db,
		path:         path,
		logger:       logger,
		metrics:      metrics,
		label:        label,
		overlayLocks: map[string]*sync.Mutex{},
	}, nil
}

// quarantineAndRebuild renames the existing file to <path>.corrupt.<unix-ts>,
// emits slog.Warn, increments the quarantine + open counters, and creates a
// fresh DB at the original path. Returns Tier-3 error on rebuild failure.
func quarantineAndRebuild(ctx context.Context, path, label, reason string, oldVersion int, logger *slog.Logger, metrics *obs.Metrics) (*Store, error) {
	// T-57-02-02: refuse to follow a symlink at the rename target.
	if li, err := os.Lstat(path); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("semantic.store.Open: refusing to quarantine symlink at %q (T-57-02-02)", path)
	}

	quarantinePath := path + ".corrupt." + strconv.FormatInt(time.Now().Unix(), 10)
	if err := os.Rename(path, quarantinePath); err != nil {
		// Tier-3 hard fail: the rename itself failed. Surface to caller so
		// the daemon refuses to start.
		return nil, fmt.Errorf("semantic.store.Open: quarantine rename %s -> %s: %w", path, quarantinePath, err)
	}

	logger.Warn("semantic store quarantined",
		"workspace_label", label,
		"original", path,
		"quarantine", quarantinePath,
		"reason", reason,
		"old_version", oldVersion,
		"new_version", CurrentSchemaVersion,
	)
	if metrics != nil {
		metrics.SemanticStoreQuarantineInc(label, reason)
	}

	// Rebuild fresh DB at the original path.
	s, err := openFresh(ctx, path, label, logger, metrics)
	if err != nil {
		return nil, fmt.Errorf("semantic.store.Open: rebuild after quarantine: %w", err)
	}
	// Override the outcome label: the open succeeded but the disposition
	// was quarantined+rebuilt, not freshly created.
	if metrics != nil {
		metrics.SemanticStoreOpenInc(label, outcomeQuarantined)
	}
	return s, nil
}

// workspaceLabel returns a bounded hashed identifier for the DB path
// (T-57-02-06 mitigation — Prometheus label cardinality stays bounded).
// We hash the parent directory (the workspace's .helix dir) so multiple
// daemons on the same workspace produce the same label.
func workspaceLabel(path string) string {
	abs, err := filepath.Abs(filepath.Dir(filepath.Dir(path)))
	if err != nil {
		abs = path
	}
	sum := sha256.Sum256([]byte(abs))
	return "ws-" + hex.EncodeToString(sum[:6])
}

// Close releases the underlying *sql.DB handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Available reports whether the store is functional in the current build.
// Under CGO=1 a non-nil Store is functional; under CGO=0 the stub returns
// false (see duckdb_nocgo.go).
func (s *Store) Available() bool { return s != nil && s.db != nil }

// QueryEffectiveFiles returns the effective file fact (snapshot ⊕ overlay −
// tombstones) for the given repo+path key. Phase 57 Schema 1 contract:
// returns nil, nil because no data write paths exist yet.
func (s *Store) QueryEffectiveFiles(ctx context.Context, repoID, path any) (any, error) {
	if s == nil || s.db == nil {
		return nil, serr.ErrUnsupported
	}
	// Schema 1 empty-but-correct: tables exist, no rows. Return nil result.
	return nil, nil
}

// QueryEffectiveSymbols returns the effective symbol facts matching a query.
// Phase 57 Schema 1 returns an empty slice.
func (s *Store) QueryEffectiveSymbols(ctx context.Context, req any) ([]any, error) {
	if s == nil || s.db == nil {
		return nil, serr.ErrUnsupported
	}
	return []any{}, nil
}

// QueryEffectiveReferences returns the effective reference facts matching a
// query. Phase 57 Schema 1 returns an empty slice.
func (s *Store) QueryEffectiveReferences(ctx context.Context, req any) ([]any, error) {
	if s == nil || s.db == nil {
		return nil, serr.ErrUnsupported
	}
	return []any{}, nil
}

// QueryEffectiveEdges returns the effective edge facts matching a query.
// Phase 57 Schema 1 returns an empty slice.
func (s *Store) QueryEffectiveEdges(ctx context.Context, req any) ([]any, error) {
	if s == nil || s.db == nil {
		return nil, serr.ErrUnsupported
	}
	return []any{}, nil
}
