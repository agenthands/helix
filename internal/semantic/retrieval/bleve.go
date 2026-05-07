package retrieval

import (
	"context"
)

// bleve.go — STUB (RED gate). Real implementation lands in Task 1 GREEN.

// SymbolDoc is the per-symbol document indexed by bleve. Field shape mirrors
// CONTEXT.md D-06: name + path + docstring + a 5-line comment window are the
// indexed text fields; ID is a keyword field for exact lookup.
type SymbolDoc struct {
	ID            string
	Name          string
	Path          string
	Doc           string
	CommentWindow string
}

// TextRank is one (symbol, score) entry returned by Engine.QueryBleve.
// Lives in this package (not internal/skill/semantic) so the retrieval
// engine has no skill-package dependency; the daemon adapter (P64-08)
// translates retrieval.TextRank → semantic.TextRank.
type TextRank struct {
	SymbolID string
	Score    float64
}

// GraphRank mirrors TextRank for Phase 62 personalized PageRank scores.
type GraphRank struct {
	SymbolID string
	Score    float64
}

// Engine wraps a single bleve scorch index keyed at <workspaceDir>/.helix/semantic.bleve/.
// One Engine per workspace; the daemon constructs/owns the lifecycle.
type Engine struct {
	// Real fields (idx, path, logger) land in Task 1 GREEN.
}

// New creates a fresh bleve scorch index at path. Errors if the path already
// exists.
func New(path string) (*Engine, error) {
	panic("retrieval.New: not implemented (RED gate — Task 1 GREEN fills this in)")
}

// Open opens an existing bleve index at path.
func Open(path string) (*Engine, error) {
	panic("retrieval.Open: not implemented (RED gate — Task 1 GREEN fills this in)")
}

// Close releases the bleve index.
func (e *Engine) Close() error {
	panic("retrieval.Engine.Close: not implemented (RED gate)")
}

// UpsertBatch indexes a batch of SymbolDocs in a single bleve.Batch.
func (e *Engine) UpsertBatch(ctx context.Context, docs []SymbolDoc) error {
	panic("retrieval.Engine.UpsertBatch: not implemented (RED gate)")
}

// QueryBleve runs a full-text query over the indexed corpus, biasing scores
// for documents whose ID is in `anchors` (CONTEXT.md D-05/D-06).
func (e *Engine) QueryBleve(task string, anchors []string) ([]TextRank, error) {
	panic("retrieval.Engine.QueryBleve: not implemented (RED gate)")
}

// GetMeta reads an internal metadata key from the bleve index. Used by the
// recovery procedure to read `last_indexed_snapshot_id`.
func (e *Engine) GetMeta(key string) ([]byte, error) {
	panic("retrieval.Engine.GetMeta: not implemented (RED gate)")
}

// SetMeta writes an internal metadata key. Used by the recovery procedure to
// persist `last_indexed_snapshot_id` after a rebuild commits.
func (e *Engine) SetMeta(key string, val []byte) error {
	panic("retrieval.Engine.SetMeta: not implemented (RED gate)")
}
