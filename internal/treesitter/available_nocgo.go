//go:build !cgo

package treesitter

// Available reports whether tree-sitter parsing is compiled into this binary.
// Resolved at compile time via build tags.
const Available = false
