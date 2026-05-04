//go:build windows && arm64

// Package store stub for windows/arm64. Upstream duckdb-go-bindings does not
// ship lib/windows-arm64 (verified 2026-05-04 against
// github.com/duckdb/duckdb-go-bindings); this file keeps the
// `internal/semantic/store` package buildable on that target so the
// 6-archive goreleaser matrix is preserved. Restoration tracked in
// .planning/milestones/v1.9-phases/51-packaging-goreleaser/deferred-items.md
// (DEF-51-04, contingent on upstream lib/windows-arm64 ship).
//
// This stub is PLATFORM-conditional, NOT CGO-conditional — it does not
// reintroduce the //go:build cgo pattern that Phase 59.1 retires.
package store

import (
	"context"
	"log/slog"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// Store on windows/arm64 is a platform-tagged stub. semantic_index is
// unavailable on this target until duckdb-go-bindings ships
// lib/windows-arm64; the daemon detects this via (*Store).Available() ==
// false and downgrades gracefully (no hard refusal).
type Store struct{}

func Open(_ context.Context, _ semantic.Config, _ *slog.Logger, _ *obs.Metrics) (*Store, error) {
	return nil, serr.ErrUnsupported
}
func (*Store) Close() error    { return serr.ErrUnsupported }
func (*Store) Available() bool { return false }
func (*Store) QueryEffectiveFiles(_ context.Context, _, _ any) (any, error) {
	return nil, serr.ErrUnsupported
}
func (*Store) QueryEffectiveSymbols(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}
func (*Store) QueryEffectiveReferences(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}
func (*Store) QueryEffectiveEdges(_ context.Context, _ any) ([]any, error) {
	return nil, serr.ErrUnsupported
}
