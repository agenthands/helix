//go:build !cgo

package edit

import (
	"fmt"

	"github.com/agenthands/helix/internal/treesitter"
	gen "github.com/agenthands/helix/protocol/gen"
)

// BodyExtractor under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type BodyExtractor struct{}

// NewBodyExtractor under !cgo returns an empty stub. Unreachable at runtime.
func NewBodyExtractor(_ *treesitter.GrammarRegistry) *BodyExtractor {
	return &BodyExtractor{}
}

// SupportsLanguage always returns false under !cgo so replace.go's tree-sitter
// path is bypassed even if the stub were ever exercised.
func (*BodyExtractor) SupportsLanguage(_ string) bool { return false }

// ExtractBody is unreachable under !cgo. Returns an error sentinel for safety.
func (*BodyExtractor) ExtractBody(_ []byte, _ string, _ string, _ gen.Range) (uint, uint, error) {
	return 0, 0, fmt.Errorf("edit.BodyExtractor.ExtractBody: tree-sitter unavailable (CGO_ENABLED=0)")
}
