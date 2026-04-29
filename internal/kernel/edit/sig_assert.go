package edit

import (
	gen "github.com/postfix/serena/protocol/gen"
)

// Compile-time signature assertions: these interface satisfaction checks
// run in BOTH cgo and !cgo builds and catch silent drift between
// treesitter.go and treesitter_nocgo.go. If the cgo signature changes,
// update the assertion AND the !cgo stub together — `go build` will fail
// loudly in either build mode rather than only in CI's nocgo job.
//
// Methods listed here must exist with identical signatures in both build
// variants of *BodyExtractor.
var _ interface {
	SupportsLanguage(string) bool
	ExtractBody([]byte, string, string, gen.Range) (uint, uint, error)
} = (*BodyExtractor)(nil)
