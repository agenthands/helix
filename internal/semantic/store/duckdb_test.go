//go:build !(windows && arm64)

package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/semantic"
)

// openWithPath is a thin Open() wrapper that lets the path-traversal cases
// share scaffolding without each repeating the cfg/logger/metrics dance.
func openWithPath(t *testing.T, path string) error {
	t.Helper()
	cfg := semantic.Config{Enabled: true}
	cfg.Store = semantic.StoreConfig{
		Kind: "duckdb",
		Path: path,
	}
	_, err := Open(context.Background(), cfg, silentLogger(), newTestObsMetrics(t))
	return err
}

// TestOpen_RejectsParentTraversal proves CR-01: the documented
// T-57-02-01 mitigation now matches code — `..` segments are refused
// with serr.ErrInvalidArgs instead of being silently rewritten by
// filepath.Clean.
func TestOpen_RejectsParentTraversal(t *testing.T) {
	cases := []string{
		"../../etc/passwd",
		"subdir/../../escape.duckdb",
		"..",
		"a/../b/../c/../d.duckdb",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			err := openWithPath(t, p)
			if err == nil {
				t.Fatalf("Open(%q): want error, got nil", p)
			}
			if !errors.Is(err, serr.ErrInvalidArgs) {
				t.Fatalf("Open(%q): want errors.Is(err, serr.ErrInvalidArgs), got %v", p, err)
			}
		})
	}
}

// TestOpen_AcceptsCleanRelativePath proves the rejection is precise — a
// path with no `..` segments is still accepted on the happy path.
func TestOpen_AcceptsCleanRelativePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "subdir", "test.duckdb") // absolute, no ..
	if err := openWithPath(t, p); err != nil {
		t.Fatalf("Open(%q): want nil, got %v", p, err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("Stat(%s): %v", p, err)
	}
}
