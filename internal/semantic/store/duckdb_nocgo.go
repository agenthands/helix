//go:build !cgo

package store

import (
	"context"
	"log/slog"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// Store under !cgo is a stub; the daemon refuses to start at step 6a before
// any instance is exercised at runtime (see internal/daemon/daemon.go and
// internal/repomap/extractor_nocgo.go for the parallel pattern from Phase
// 51.1).
type Store struct{}

// Open under !cgo always returns serr.ErrUnsupported (defense-in-depth —
// daemon already hard-fails at step 6a).
func Open(_ context.Context, _ semantic.Config, _ *slog.Logger, _ *obs.Metrics) (*Store, error) {
	return nil, serr.ErrUnsupported
}

// Close on the !cgo stub returns serr.ErrUnsupported.
func (*Store) Close() error { return serr.ErrUnsupported }

// Available on the !cgo stub returns false. Mirrors the
// repomap.TagExtractor / treesitter.Available pattern.
func (*Store) Available() bool { return false }

// QueryEffectiveFiles on the !cgo stub returns serr.ErrUnsupported.
func (*Store) QueryEffectiveFiles(_ context.Context, _, _ any) (any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveSymbols on the !cgo stub returns serr.ErrUnsupported.
func (*Store) QueryEffectiveSymbols(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveReferences on the !cgo stub returns serr.ErrUnsupported.
func (*Store) QueryEffectiveReferences(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}

// QueryEffectiveEdges on the !cgo stub returns serr.ErrUnsupported.
func (*Store) QueryEffectiveEdges(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}
