// Phase 64-01 DuckDB FTS5 baseline benchmark. Build-gated to keep the
// noduckdb vet analyzer happy in default builds — the helper it calls,
// store.BenchDuckDBFTSIngest, lives inside internal/semantic/store/ under
// the same `benchfts` tag.

//go:build benchfts

package bench

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic/store"
)

// BenchmarkDuckDBFTSIndexThroughput_50k opens a fresh DuckDB DB under a
// temp dir, ingests the same 50k SymbolDocs, and builds the FTS5 index.
// Reports b.SetBytes(50000) so go test prints symbols/s as MB/s.
func BenchmarkDuckDBFTSIndexThroughput_50k(b *testing.B) {
	docs := loadCorpus(b)
	storeDocs := make([]store.BenchSymbolDoc, len(docs))
	for i, d := range docs {
		storeDocs[i] = store.BenchSymbolDoc{
			ID:            d.ID,
			Name:          d.Name,
			Path:          d.Path,
			Doc:           d.Doc,
			CommentWindow: d.CommentWindow,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		closer, err := store.BenchDuckDBFTSIngest(context.Background(), filepath.Clean(dir), storeDocs)
		if err != nil {
			if closer != nil {
				_ = closer()
			}
			b.Fatalf("BenchDuckDBFTSIngest: %v", err)
		}
		if err := closer(); err != nil {
			b.Fatalf("close: %v", err)
		}
	}
	b.SetBytes(int64(len(docs)))
}
