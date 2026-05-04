//go:build !cgo

// Package testutil exposes test-only helpers. Under CGO=0 tree-sitter is
// unavailable, so NewTestRegistry skips the calling test rather than
// returning a non-functional registry. Mirrors
// internal/repomap/extractor_nocgo.go's stub posture.
package testutil

import "testing"

// NewTestRegistry under !cgo skips the calling test — tree-sitter requires
// CGO and the daemon refuses to start with semantic_index.enabled=true on
// CGO=0 builds (Phase 51.1 / Phase 57 D-04 invariant).
//
// Returns `any` (not the real registry type) so callers under !cgo do not
// type-check against a CGO-only struct.
func NewTestRegistry(t *testing.T) any {
	t.Helper()
	t.Skip("testutil.NewTestRegistry: CGO required for tree-sitter")
	return nil
}
