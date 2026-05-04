//go:build !cgo

package extract

// CGO=0 stub: tree-sitter bindings are unavailable. The daemon's
// step 6a (Phase 57) refuses to start with semantic_index.enabled=true
// and CGO=0, so this file's existence is purely to keep
// `go build ./...` green on CGO=0 builds (Phase 51.1 / Phase 57 D-04
// invariant).
