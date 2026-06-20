// Package kernelextra is the lookalike sibling used by the slash-boundary
// regression guard: its path shares the `internal/kernel` bare-prefix but not
// the slash boundary, so the analyzer must NOT flag an import of it.
package kernelextra
