// Phase 64-01 bleve gate benchmark.
//
// Measures (1) bleve scorch indexing throughput and (2) DuckDB FTS5 indexing
// throughput on the deterministic 50k-symbol Go fixture under
// internal/semantic/bench/fixtures/synthetic_50k_go/. The result is fed back into
// internal/semantic/bench/semantic_bench_REPORT.md to PASS or FAIL the bleve dependency under
// CONTEXT.md D-08.
//
// Both benchmarks require the fixture to be present on disk:
//
//	cd internal/semantic/bench/fixtures/synthetic_50k_go && go run gen.go
//
// The DuckDB FTS5 benchmark is gated behind the `benchfts` build tag so
// the noduckdb vet analyzer is satisfied in the default build:
//
//	go test -tags=benchfts -bench='BenchmarkBleveIndexThroughput_50k|BenchmarkDuckDBFTSIndexThroughput_50k' -benchtime=1x ./internal/semantic/bench/...

package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// SymbolDoc is the row shape both benchmarks index. Mirrors the
// internal/semantic/store.BenchSymbolDoc shape so the two backends index
// byte-equivalent content.
type SymbolDoc struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Doc           string `json:"doc"`
	CommentWindow string `json:"comment_window"`
}

// loadCorpus walks the synthetic fixture and produces 50k SymbolDocs by
// reading each `func` and `type` declaration line plus its 3-line godoc
// comment. The shape is best-effort — what matters is the row count and
// the rough text volume per row, both of which are stable given the
// deterministic fixture.
func loadCorpus(tb testing.TB) []SymbolDoc {
	tb.Helper()
	root := fixtureRoot(tb)
	out := make([]SymbolDoc, 0, 50_000)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(raw), "\n")
		// Walk lines: capture each func/type decl + the 3 comment lines
		// directly above it.
		for i, ln := range lines {
			isFunc := strings.HasPrefix(ln, "func ")
			isType := strings.HasPrefix(ln, "type ")
			if !isFunc && !isType {
				continue
			}
			name := extractName(ln, isFunc)
			if name == "" {
				continue
			}
			// godoc window: 3 lines above the decl. Bounds-check.
			start := i - 3
			if start < 0 {
				start = 0
			}
			window := strings.Join(lines[start:i], "\n")
			rel, _ := filepath.Rel(root, path)
			// Include line number in ID to disambiguate the random-name
			// collisions that occur naturally when 100 names per file are
			// drawn from a 200-word list with replacement.
			out = append(out, SymbolDoc{
				ID:            fmt.Sprintf("%s::%d::%s", rel, i, name),
				Name:          name,
				Path:          rel,
				Doc:           window,
				CommentWindow: window,
			})
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("walk fixture: %v", err)
	}
	if len(out) == 0 {
		tb.Skipf("fixture is empty — run `cd %s && go run gen.go` first", root)
	}
	return out
}

func fixtureRoot(tb testing.TB) string {
	tb.Helper()
	// bench_test.go runs from internal/semantic/bench/, fixture lives at
	// internal/semantic/bench/fixtures/synthetic_50k_go/out.
	wd, err := os.Getwd()
	if err != nil {
		tb.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(wd, "fixtures", "synthetic_50k_go", "out")
	if _, err := os.Stat(root); err != nil {
		tb.Skipf("fixture not generated at %s — run gen.go first", root)
	}
	return root
}

// extractName parses `func Name() string { ... }` or `type Name struct{...}`
// returning the bare identifier. Generator emits a fixed shape so the
// extraction is trivial.
func extractName(line string, isFunc bool) string {
	if isFunc {
		// "func Name() string { ... }"
		rest := strings.TrimPrefix(line, "func ")
		i := strings.IndexAny(rest, "(")
		if i < 0 {
			return ""
		}
		return strings.TrimSpace(rest[:i])
	}
	// "type Name struct{ ... }"
	rest := strings.TrimPrefix(line, "type ")
	i := strings.IndexAny(rest, " ")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:i])
}

// TestNothing exists so `go test ./internal/semantic/bench/...` succeeds even when no
// benchmarks are run, satisfying the plan's compile-only sanity check.
func TestNothing(t *testing.T) {}

// BenchmarkBleveIndexThroughput_50k opens a fresh bleve scorch index in a
// temp dir, batch-indexes the 50k SymbolDocs via the canonical bleve.Batch
// API in groups of 1000, then closes the index. Reports b.SetBytes(50000)
// so go test prints throughput in symbols/s as MB/s (treat MB/s value as
// symbols/s — see REPORT.md notes column).
func BenchmarkBleveIndexThroughput_50k(b *testing.B) {
	docs := loadCorpus(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		idxPath := filepath.Join(dir, "bench.bleve")
		idxMapping := mapping.NewIndexMapping()
		idx, err := bleve.New(idxPath, idxMapping)
		if err != nil {
			b.Fatalf("bleve.New: %v", err)
		}
		batch := idx.NewBatch()
		const flushEvery = 1000
		for j, d := range docs {
			if err := batch.Index(d.ID, d); err != nil {
				_ = idx.Close()
				b.Fatalf("batch.Index: %v", err)
			}
			if (j+1)%flushEvery == 0 {
				if err := idx.Batch(batch); err != nil {
					_ = idx.Close()
					b.Fatalf("idx.Batch: %v", err)
				}
				batch = idx.NewBatch()
			}
		}
		if batch.Size() > 0 {
			if err := idx.Batch(batch); err != nil {
				_ = idx.Close()
				b.Fatalf("idx.Batch tail: %v", err)
			}
		}
		if err := idx.Close(); err != nil {
			b.Fatalf("idx.Close: %v", err)
		}
	}
	b.SetBytes(int64(len(docs)))
	_ = context.Background() // silence unused import in benchfts-off builds
}
