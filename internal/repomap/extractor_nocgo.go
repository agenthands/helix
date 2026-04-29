//go:build !cgo

package repomap

import (
	"fmt"

	"github.com/postfix/serena/internal/treesitter"
)

// TagExtractor under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type TagExtractor struct{}

// NewTagExtractor under !cgo returns an error sentinel. Loud failure per D-01.
func NewTagExtractor(_ *treesitter.GrammarRegistry) (*TagExtractor, error) {
	return nil, fmt.Errorf("repomap.TagExtractor is unavailable in this build (CGO_ENABLED=0)")
}

// Extract is unreachable under !cgo (constructor errored). Returns nil to keep
// the public method set compilable for any reflective references.
func (*TagExtractor) Extract(_ []byte, _ string, _ string) ([]Tag, error) {
	return nil, fmt.Errorf("repomap.TagExtractor.Extract: tree-sitter unavailable (CGO_ENABLED=0)")
}

// Close is a no-op under !cgo.
func (*TagExtractor) Close() {}
