package tree_sitter_r

// #cgo CFLAGS: -std=c11 -fPIC -Isrc
// #include "src/parser.c"
// #include "src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language for R.
//
// This file is compiled only when CGO is enabled. The non-CGO build picks up
// binding_nocgo.go which returns nil; registry.go skips registering "r" in
// that case. See .planning/phases/51-packaging-goreleaser/deferred-items.md
// DEF-51-01 for the rationale (CGO_ENABLED=0 cross-compile for goreleaser).
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_r())
}
