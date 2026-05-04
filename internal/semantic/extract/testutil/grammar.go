//go:build cgo

// Package testutil exposes test-only helpers for the per-language extract
// providers (internal/semantic/extract/golang, .../typescript, .../python).
//
// It deliberately lives one directory level deeper than
// internal/semantic/extract/* so the EXTRACT-05 regression-grep test (which
// scans non-test source files inside internal/semantic/extract/* for
// `NewGrammarRegistry` calls) does not flag this helper. Tests that need a
// real GrammarRegistry construct one through this entry point.
//
// The helper is named NewTestRegistry (NOT TestGrammarRegistry) — the Test*
// prefix is reserved for go-test functions and would otherwise be invoked
// by the test runner.
package testutil

import (
	"testing"

	"github.com/agenthands/helix/internal/treesitter"
)

// NewTestRegistry returns a freshly-constructed *treesitter.GrammarRegistry
// suitable for tests. Production code MUST NOT use this — the daemon owns
// the singleton (BUG-04 invariant) and injects it into providers via
// Phase 59 P05 wiring.
func NewTestRegistry(t *testing.T) *treesitter.GrammarRegistry {
	t.Helper()
	return treesitter.NewGrammarRegistry()
}
