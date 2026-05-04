//go:build !cgo

// Package goextract is the Go extraction provider. Under !cgo tree-sitter
// is unavailable; the daemon refuses to start at step 6a so this stub is
// never exercised at runtime. Provided for build-tag completeness.
package goextract

import (
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

// NewProvider under !cgo returns nil — the daemon refuses to enable
// semantic_index without CGO so providers never see this path.
func NewProvider(_ *treesitter.GrammarRegistry) extract.Provider {
	return nil
}
