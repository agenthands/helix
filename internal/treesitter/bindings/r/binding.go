package tree_sitter_r

// #cgo CFLAGS: -std=c11 -fPIC -Isrc
// #include "src/parser.c"
// #include "src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language for R.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_r())
}
