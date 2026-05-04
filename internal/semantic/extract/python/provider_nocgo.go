//go:build !cgo

package pyextract

import (
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

func NewProvider(_ *treesitter.GrammarRegistry) extract.Provider { return nil }
