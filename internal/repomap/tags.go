// Package repomap provides tree-sitter-based tag extraction for building
// repository maps with ranked symbol importance.
package repomap

// TagKind distinguishes definition tags from reference tags.
type TagKind string

const (
	// TagDef marks a symbol definition (function, type, class, etc.).
	TagDef TagKind = "def"
	// TagRef marks a symbol reference (call site, type usage, etc.).
	TagRef TagKind = "ref"
)

// Tag represents a single definition or reference extracted from a source file.
// Per D-01: two kinds only (def/ref). Per D-02: flat, no scope nesting.
// Per D-04: byte offsets only, no raw text stored.
type Tag struct {
	Name      string
	Kind      TagKind
	File      string
	Line      int
	Column    int
	StartByte uint
	EndByte   uint
}
