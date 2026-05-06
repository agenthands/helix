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

// TestOpen_RejectsAbsolutePath proves BL-01 (follow-up to CR-01): the
// StoreConfig.Path doc promises absolute paths are rejected at Open time;
// this test asserts that contract. An absolute path — even one that lives
// inside the workspace — must be refused with serr.ErrInvalidArgs so the
// caller is forced to keep the path workspace-relative.
func TestOpen_RejectsAbsolutePath(t *testing.T) {
	cases := []string{
		"/etc/passwd",
		"/tmp/escape.duckdb",
	}
	if filepath.Separator == '\\' {
		cases = append(cases, `C:\\Windows\\Temp\\escape.duckdb`)
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
// workspace-relative path with no `..` segments and no leading `/` is still
// accepted on the happy path. The test chdirs into a tempdir-rooted
// workspace so the relative path resolves to a writable location.
func TestOpen_AcceptsCleanRelativePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rel := filepath.Join("subdir", "test.duckdb") // relative, no ..
	if err := openWithPath(t, rel); err != nil {
		t.Fatalf("Open(%q): want nil, got %v", rel, err)
	}
	// The file must land at the chdir-rooted location.
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Fatalf("Stat(%s): %v", filepath.Join(dir, rel), err)
	}
}
