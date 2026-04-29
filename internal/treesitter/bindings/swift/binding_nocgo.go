//go:build !cgo

package tree_sitter_swift

import "unsafe"

// Language returns nil when the binary is built with CGO_ENABLED=0.
//
// The CGO version of this binding (binding.go) wraps tree-sitter-swift's C parser,
// which cannot cross-compile under CGO_ENABLED=0. The goreleaser pipeline
// (.goreleaser.yaml, Phase 51) builds with CGO_ENABLED=0 to keep the project
// invariant of a single static Go binary. Returning nil here is paired with a
// nil-check in internal/treesitter/registry.go that skips registering "swift" when
// the no-CGO build is in use.
//
// Net effect: CGO_ENABLED=0 binaries support 21 tree-sitter languages (R and
// Swift omitted); CGO_ENABLED=1 binaries support all 23. See
// .planning/phases/51-packaging-goreleaser/deferred-items.md DEF-51-01 for the
// full rationale and resolution history.
func Language() unsafe.Pointer {
	return nil
}
