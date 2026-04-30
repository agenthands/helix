//go:build !cgo

package repomap

import (
	"fmt"

	"github.com/agenthands/helix/internal/treesitter"
)

// TagExtractor under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type TagExtractor struct{}

// NewTagExtractor under !cgo returns a non-nil empty stub to mirror sibling
// stubs (NewBodyExtractor, NewElisionRenderer). Loud failure is delivered by
// the daemon refusal hook before any caller is exercised; methods on this
// stub still return errors as defense-in-depth if the refusal is bypassed.
func NewTagExtractor(_ *treesitter.GrammarRegistry) (*TagExtractor, error) {
	return &TagExtractor{}, nil
}

// Extract is unreachable under !cgo at runtime (daemon refuses). Returns an
// error sentinel for safety if the stub is ever exercised directly.
func (*TagExtractor) Extract(_ []byte, _ string, _ string) ([]Tag, error) {
	return nil, fmt.Errorf("repomap.TagExtractor.Extract: tree-sitter unavailable (CGO_ENABLED=0)")
}

// Close is a no-op under !cgo.
func (*TagExtractor) Close() {}
