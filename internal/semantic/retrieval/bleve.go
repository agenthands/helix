package retrieval

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	bleve "github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// Exported bleve-meta keys (Phase 69-02). The retrieval package owns the
// canonical names so cross-package readers — internal/semantic/compact (Plan
// 69-03) and internal/daemon (Plan 69-05) — share one set of constants with
// no string-literal drift. The pre-existing unexported metaKeyLastIndexed
// (recovery.go) is intentionally not exported: only the retrieval package
// ever writes it.
const (
	MetaKeyCorpusVersion = "corpus_version"
	MetaKeyIndexedFiles  = "indexed_files"
	MetaKeyLastCompactAt = "last_compact_at"
)

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
//
// Concurrency: bleve's batch operations are internally goroutine-safe (per
// bleve docs); QueryBleve is read-only and may run concurrently with
// UpsertBatch. Close MUST NOT race with in-flight queries — daemon shutdown
// drains both before invoking Close.
type Engine struct {
	idx    bleve.Index
	path   string
	logger *slog.Logger
}

// New creates a fresh bleve scorch index at path. Errors if the path already
// exists. The index uses the default scorch backend (recommended in v2+).
func New(path string) (*Engine, error) {
	idx, err := bleve.New(path, buildMapping())
	if err != nil {
		return nil, fmt.Errorf("retrieval.New(%q): %w", path, err)
	}
	return &Engine{idx: idx, path: path, logger: slog.Default()}, nil
}

// Open opens an existing bleve index at path.
func Open(path string) (*Engine, error) {
	idx, err := bleve.Open(path)
	if err != nil {
		return nil, fmt.Errorf("retrieval.Open(%q): %w", path, err)
	}
	return &Engine{idx: idx, path: path, logger: slog.Default()}, nil
}

// Close releases the bleve index. Safe to call once; subsequent calls return
// nil.
func (e *Engine) Close() error {
	if e == nil || e.idx == nil {
		return nil
	}
	idx := e.idx
	e.idx = nil
	return idx.Close()
}

// UpsertBatch indexes a batch of SymbolDocs in a single bleve.Batch. Empty
// or nil docs is a no-op.
func (e *Engine) UpsertBatch(ctx context.Context, docs []SymbolDoc) error {
	if e == nil || e.idx == nil {
		return fmt.Errorf("retrieval.UpsertBatch: engine closed")
	}
	if len(docs) == 0 {
		return nil
	}
	batch := e.idx.NewBatch()
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		// Honor ctx cancellation between batched docs (cheap; bleve's own
		// batch.Index call is non-blocking — the work happens at idx.Batch).
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		// bleve indexes the SymbolDoc struct via reflection against the
		// IndexMapping (default mapping treats every field).
		if err := batch.Index(d.ID, d); err != nil {
			return fmt.Errorf("batch.Index(%q): %w", d.ID, err)
		}
	}
	if err := e.idx.Batch(batch); err != nil {
		return fmt.Errorf("idx.Batch: %w", err)
	}
	return nil
}

// QueryBleve runs a full-text query over the indexed corpus, biasing scores
// for documents whose ID is in `anchors` (CONTEXT.md D-05/D-06).
//
// Query shape:
//   - Must:   match-query on the task string across name/path/doc/comment_window.
//   - Should: a doc-id query for each anchor — anchored docs that ALSO match
//     the must clause score higher.
//
// Returns a deterministically-ordered slice (by descending score, then
// ascending document id for ties — see returned slice ordering note).
func (e *Engine) QueryBleve(task string, anchors []string) ([]TextRank, error) {
	if e == nil || e.idx == nil {
		return nil, fmt.Errorf("retrieval.QueryBleve: engine closed")
	}
	task = strings.TrimSpace(task)
	if task == "" {
		// Empty task -> no hits; avoid passing an empty MatchQuery which
		// matches everything in some bleve versions.
		return nil, nil
	}

	// Match query across the four text-analyzed fields. We rely on the
	// default mapping treating SymbolDoc fields as text-indexed; the
	// MatchQuery without SetField fans out across all text fields.
	mq := bleve.NewMatchQuery(task)

	// Boolean root: must = match-query; should = anchor doc-id boost.
	bq := bleve.NewBooleanQuery()
	bq.AddMust(mq)
	if len(anchors) > 0 {
		// Filter to non-empty anchors; bleve's DocIDQuery rejects empty IDs.
		nonEmpty := make([]string, 0, len(anchors))
		for _, a := range anchors {
			if a != "" {
				nonEmpty = append(nonEmpty, a)
			}
		}
		if len(nonEmpty) > 0 {
			docIDQ := bleve.NewDocIDQuery(nonEmpty)
			bq.AddShould(docIDQ)
		}
	}

	req := bleve.NewSearchRequest(bq)
	// Cap result size — agents consume the top-N anyway, and we don't want
	// QueryBleve to return tens of thousands of low-confidence hits.
	req.Size = 256
	res, err := e.idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("idx.Search: %w", err)
	}
	if res == nil || res.Hits == nil {
		return nil, nil
	}
	out := make([]TextRank, 0, len(res.Hits))
	for _, h := range res.Hits {
		out = append(out, TextRank{SymbolID: h.ID, Score: h.Score})
	}
	return out, nil
}

// DocCount returns the number of documents currently indexed in the bleve
// segment. Wraps bleve.Index.DocCount; used by Plan 69-05's
// semRetrievalAdapter.RetrievalStatus to populate indexed_symbols.
func (e *Engine) DocCount() (uint64, error) {
	if e == nil || e.idx == nil {
		return 0, fmt.Errorf("retrieval.DocCount: engine closed")
	}
	n, err := e.idx.DocCount()
	if err != nil {
		return 0, fmt.Errorf("idx.DocCount: %w", err)
	}
	return n, nil
}

// GetMeta reads an internal metadata key from the bleve index. Used by the
// recovery procedure to read `last_indexed_snapshot_id`. A missing key
// returns (nil, nil) — bleve's GetInternal is not error-typed for absence.
func (e *Engine) GetMeta(key string) ([]byte, error) {
	if e == nil || e.idx == nil {
		return nil, fmt.Errorf("retrieval.GetMeta: engine closed")
	}
	v, err := e.idx.GetInternal([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("idx.GetInternal(%q): %w", key, err)
	}
	return v, nil
}

// SetMeta writes an internal metadata key. Used by the recovery procedure to
// persist `last_indexed_snapshot_id` after a rebuild commits.
func (e *Engine) SetMeta(key string, val []byte) error {
	if e == nil || e.idx == nil {
		return fmt.Errorf("retrieval.SetMeta: engine closed")
	}
	if err := e.idx.SetInternal([]byte(key), val); err != nil {
		return fmt.Errorf("idx.SetInternal(%q): %w", key, err)
	}
	return nil
}

// buildMapping constructs the bleve IndexMapping used by all SymbolDoc
// indices. The mapping marks ID as a keyword (exact match) field and the
// other four fields as text-analyzed.
func buildMapping() *mapping.IndexMappingImpl {
	im := bleve.NewIndexMapping()

	docMapping := bleve.NewDocumentMapping()

	keywordField := bleve.NewKeywordFieldMapping()
	keywordField.Store = false
	docMapping.AddFieldMappingsAt("ID", keywordField)

	textField := bleve.NewTextFieldMapping()
	textField.Store = false
	docMapping.AddFieldMappingsAt("Name", textField)
	docMapping.AddFieldMappingsAt("Path", textField)
	docMapping.AddFieldMappingsAt("Doc", textField)
	docMapping.AddFieldMappingsAt("CommentWindow", textField)

	im.DefaultMapping = docMapping
	return im
}
