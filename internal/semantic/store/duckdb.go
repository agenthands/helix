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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	// D-12: SOLE owner of the duckdb-go import (canonical DuckDB Foundation
	// repo at github.com/duckdb/duckdb-go/v2). Registers the "duckdb" driver
	// with database/sql via init().
	_ "github.com/duckdb/duckdb-go/v2"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// reopenErrClass classifies an error returned from openExisting on the
// Tier-1 reopen path so Open can distinguish transient filesystem
// contention (worth one bounded retry) from corruption-class signals
// (immediate quarantine). WR-03.
type reopenErrClass int

const (
	reopenUnknown reopenErrClass = iota
	reopenTransient
	reopenCorruption
)

// reopenRetryDecision classifies the action Open should take after an
// initial transient error followed by a retry that ALSO fails. WR-NEW-02:
// the second-attempt classification governs whether we quarantine a
// (likely clean) DB or refuse to quarantine and bubble up a hard fail.
type reopenRetryDecision int

const (
	// reopenRetryHardFail means the second attempt was also transient.
	// The DB is most likely clean — refuse to quarantine and surface
	// the error so the supervisor can restart and try again.
	reopenRetryHardFail reopenRetryDecision = iota
	// reopenRetryQuarantine means the second attempt classified as
	// corruption (or an unknown non-transient error). Quarantine the
	// file and rebuild.
	reopenRetryQuarantine
)

// decideReopenRetry encapsulates the WR-NEW-02 retry decision so it is
// independently unit-testable. The pure-function shape — error → decision
// — sidesteps the need to inject a fake openExisting into Open itself.
func decideReopenRetry(secondErr error) reopenRetryDecision {
	if classifyReopenError(secondErr) == reopenTransient {
		return reopenRetryHardFail
	}
	return reopenRetryQuarantine
}

// classifyReopenError categorises an error returned from openExisting so
// Open's Tier-1 reopen path can distinguish transient filesystem
// contention (worth one bounded retry) from corruption (immediate
// quarantine). WR-03.
func classifyReopenError(err error) reopenErrClass {
	if err == nil {
		return reopenUnknown
	}
	// Transient: file-locking / interrupted-syscall / busy-text-segment.
	// ETXTBSY is POSIX-only but syscall.ETXTBSY is defined on linux and
	// darwin (the only platforms where CGO=1 ships in v1.10; the file's
	// build tag already excludes windows/arm64).
	if errors.Is(err, syscall.EBUSY) ||
		errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.EAGAIN) ||
		errors.Is(err, syscall.ETXTBSY) {
		return reopenTransient
	}
	// Corruption-class: substring match against a closed list of sentinel
	// signatures DuckDB surfaces in Open / Ping error messages.
	msg := err.Error()
	for _, sig := range []string{"checksum", "corrupt", "header", "malformed"} {
		if strings.Contains(msg, sig) {
			return reopenCorruption
		}
	}
	return reopenUnknown
}

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

	// Phase 63 P63-02 Task 1: per-workspace counters consumed by the
	// compaction gate. Both maps are lazily installed under
	// overlayCountsMu; the per-workspace atomic.Int32 / atomic.Int64
	// values are then mutated lock-free.
	//
	//   - overlayTxCounts: number of currently-open OverlayTx handles per
	//     repoID. Incremented in BeginOverlayTx, decremented in
	//     OverlayTx.releaseLock. Drives BlockedOverlayTxActive.
	//   - overlayPendingRows: cumulative count of overlay row writes since
	//     the last ClearOverlayLE drained the table. Bumped on every
	//     successful overlay-row write path; reset to 0 when
	//     ClearOverlayLE removes ≥ 1 row. Drives BlockedOverlayEmpty
	//     in O(1) without a SELECT (CONTEXT.md D-04 hard invariant).
	overlayCountsMu    sync.Mutex
	overlayTxCounts    map[string]*atomic.Int32
	overlayPendingRows map[string]*atomic.Int64
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
	// T-57-02-01: reject absolute paths AND path traversal explicitly. The
	// StoreConfig.Path doc promises both halves; this guard implements them
	// in one place. The absolute-path check runs first so an absolute path
	// containing `..` surfaces the more specific "must be workspace-relative"
	// error rather than the parent-reference error. Splitting on the
	// forward-slash form (filepath.ToSlash converts Windows backslashes)
	// catches every "/../" anywhere in the path, including the bare ".."
	// case. We refuse rather than silently rewrite via filepath.Clean — the
	// caller is responsible for handing us a path that already lives inside
	// the workspace.
	if filepath.IsAbs(path) {
		return nil, fmt.Errorf("semantic.store.Open: path %q must be workspace-relative (T-57-02-01): %w", path, serr.ErrInvalidArgs)
	}
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		if seg == ".." {
			return nil, fmt.Errorf("semantic.store.Open: path %q contains parent-reference segment (T-57-02-01): %w", path, serr.ErrInvalidArgs)
		}
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
		if err == nil {
			return s, nil
		}
		// Forward-incompat is a hard fail: an operator running an older
		// binary against a newer DB MUST see an explicit error and
		// rebuild manually via the documented quarantine path. We do
		// NOT silently quarantine because that would discard the newer
		// binary's data on rollback — see plan 59-01 STORE-03 invariant.
		if errors.Is(err, ErrForwardIncompatible) {
			return nil, err
		}
		// WR-03: classify and (for transient errors only) retry once with
		// a small backoff before quarantining. A clean DB hit by EBUSY /
		// EINTR / EAGAIN / ETXTBSY does not deserve to be quarantined.
		switch classifyReopenError(err) {
		case reopenTransient:
			logger.Warn("semantic store reopen hit transient error; retrying once",
				"workspace_label", label, "path", path, "err", err)
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			s, err2 := openExisting(ctx, path, label, logger, metrics)
			if err2 == nil {
				return s, nil
			}
			// WR-NEW-02: if the SECOND attempt is ALSO transient (e.g.,
			// persistent EBUSY because a sibling daemon or backup process
			// is holding the file) the original code labelled the workspace
			// reasonCorruptFile and renamed the file — destroying a
			// perfectly healthy DB. Refuse to quarantine on retry-still-
			// transient and surface as a hard fail; the supervisor can
			// restart and try again. Only ACTUAL corruption-class errors
			// on retry quarantine.
			switch decideReopenRetry(err2) {
			case reopenRetryHardFail:
				return nil, fmt.Errorf("semantic.store.Open: transient errors persist across retry; refusing to quarantine clean DB: %w", err2)
			case reopenRetryQuarantine:
				return quarantineAndRebuild(ctx, path, label, reasonCorruptFile, 0, logger, metrics)
			}
			return quarantineAndRebuild(ctx, path, label, reasonCorruptFile, 0, logger, metrics)
		case reopenCorruption, reopenUnknown:
			return quarantineAndRebuild(ctx, path, label, reasonCorruptFile, 0, logger, metrics)
		}
		// Unreachable — every reopenErrClass value is handled above.
		return quarantineAndRebuild(ctx, path, label, reasonCorruptFile, 0, logger, metrics)
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

	// Schema-version table presence + readability. CR-02: the parent ctx
	// is typically context.Background() at daemon bootstrap (daemon.go
	// step 6b), so a wedged DuckDB read here would hang the daemon
	// indefinitely. Match the 5s budget already used for PingContext above.
	queryCtx, qcancel := context.WithTimeout(ctx, 5*time.Second)
	defer qcancel()

	var version int
	row := db.QueryRowContext(queryCtx, "SELECT version FROM semantic_schema_version LIMIT 1")
	if err := row.Scan(&version); err != nil {
		// Table missing OR row missing OR query timed out → schema_unreadable
		// per D-07. The closed enum already absorbs context.DeadlineExceeded
		// under schema_unreadable; no new reason value is required.
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

// DB returns the underlying *sql.DB handle. Returns nil iff the store is
// not open. On windows/arm64 (the duckdb_winarm64.go stub) it always
// returns nil because this platform has no DuckDB build (DEF-51-04);
// Available() returns false long before a caller ever reaches this
// method, but the nil return keeps the contract uniform across both
// build paths. Intended for read-only admin/status queries (e.g.,
// get_health surfacing overlay epoch) and integration tests that need
// to inspect internal state without going through BeginOverlayTx (which
// would bump the epoch as a side effect). Callers MUST NOT issue
// schema-altering statements through this handle — migrations.go is the
// single owner of schema evolution.
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

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
