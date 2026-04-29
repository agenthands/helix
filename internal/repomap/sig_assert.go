package repomap

// Compile-time signature assertions: these interface satisfaction checks
// run in BOTH cgo and !cgo builds and catch silent drift between
// extractor.go / elide.go and their _nocgo.go stubs. If a cgo signature
// changes, update the assertion AND the stub together — `go build` will
// fail loudly in either build mode rather than only in CI's nocgo job.
//
// Methods listed here must exist with identical signatures in both build
// variants. Methods that intentionally exist only in cgo (e.g. those whose
// types reference *tree_sitter.Language) are deliberately omitted.

var _ interface {
	Extract([]byte, string, string) ([]Tag, error)
	Close()
} = (*TagExtractor)(nil)

var _ interface {
	RenderFile([]byte, string, []Tag) string
} = (*ElisionRenderer)(nil)
