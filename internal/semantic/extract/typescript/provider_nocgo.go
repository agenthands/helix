//go:build !cgo

package tsextract

import (
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

// NewProvider under !cgo returns nil — daemon refuses to enable
// semantic_index without CGO so this stub never runs at runtime.
func NewProvider(_ *treesitter.GrammarRegistry) extract.Provider {
	return nil
}
