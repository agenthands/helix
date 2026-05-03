//go:build cgo

package store

import (
	"context"
	"database/sql"
	"log/slog"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// Store is the DuckDB-backed semantic fact store. Phase 57 ships Schema 1
// empty-but-correct; the real Open implementation is wired in Task 2b
// (GREEN). This RED-step stub returns serr.ErrUnsupported on every method
// so the integration tests in store_test.go fail with assertion errors —
// the failure is the RED gate.
type Store struct {
	db      *sql.DB
	path    string
	logger  *slog.Logger
	metrics *obs.Metrics
}

// Open opens (or quarantines + rebuilds, or hard-fails) the DuckDB file at
// `<workspaceRoot>/<cfg.Store.Path>`. RED stub — Task 2b implements the
// three-tier open contract documented in doc.go.
func Open(_ context.Context, _ semantic.Config, _ *slog.Logger, _ *obs.Metrics) (*Store, error) {
	return nil, serr.ErrUnsupported
}

// Close releases the underlying *sql.DB handle. RED stub.
func (*Store) Close() error { return serr.ErrUnsupported }

// Available reports whether the store is functional in the current build.
// CGO=1 RED stub returns false so TestStub_Available_ReturnsFalse fails;
// Task 2b returns true under CGO=1.
func (*Store) Available() bool { return false }

// QueryEffectiveFiles returns the effective file fact (snapshot ⊕ overlay −
// tombstones) for a single repo+path key. Schema 1 returns empty.
func (*Store) QueryEffectiveFiles(_ context.Context, _, _ any) (any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveSymbols returns the effective symbol facts matching a query.
// Schema 1 returns an empty slice.
func (*Store) QueryEffectiveSymbols(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveReferences returns the effective reference facts matching a
// query. Schema 1 returns an empty slice.
func (*Store) QueryEffectiveReferences(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveEdges returns the effective edge facts matching a query.
// Schema 1 returns an empty slice.
func (*Store) QueryEffectiveEdges(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}
