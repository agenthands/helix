// Package extract emits typed semantic facts (symbols, references, imports,
// types, heritage, syntax edges) by parsing source files with tree-sitter.
//
// D-01 hard invariants (DO NOT VIOLATE):
//   - This package MUST NOT import "github.com/agenthands/helix/internal/repomap".
//   - This package MUST NOT contain init() registration of providers.
//   - The shared *treesitter.GrammarRegistry is INJECTED by the daemon
//     (Phase 49 BUG-04 / EXTRACT-05 invariant).
package extract
