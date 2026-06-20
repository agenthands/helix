package ragindex

import (
	"context"
	"sync/atomic"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

// countingStub wraps the deterministic stub embedder with an atomic call
// counter so tests can assert zero re-embeds on the warm cache path.
func countingStub(counter *int64) chromem.EmbeddingFunc {
	base := stubEmbedder()
	return func(ctx context.Context, text string) ([]float32, error) {
		atomic.AddInt64(counter, 1)
		return base(ctx, text)
	}
}

func TestCachePathAndReuse(t *testing.T) {
	t.Setenv("HELIX_CACHE_DIR", t.TempDir())

	root := writeCorpus(t, map[string]string{
		"a.go":     "package a\nfunc Alpha() string { return \"alpha\" }\n",
		"b.go":     "package b\nfunc Beta() string { return \"beta\" }\n",
		"sub/c.go": "package sub\nfunc Gamma() string { return \"gamma\" }\n",
	})
	ctx := context.Background()

	// Cold build: the stub embedder MUST be called at least once per chunk.
	var coldCalls int64
	idx, err := openWith(ctx, root, countingStub(&coldCalls), embedderIDStub)
	if err != nil {
		t.Fatalf("cold Open: %v", err)
	}
	if coldCalls == 0 {
		t.Fatal("cold build did not embed any chunk")
	}
	if got := idx.EmbedderID(); got != embedderIDStub {
		t.Fatalf("EmbedderID = %q, want %q", got, embedderIDStub)
	}
	if idx.Count() == 0 {
		t.Fatal("cold index has zero documents")
	}

	// Warm reopen of the SAME path: chromem auto-loads gob docs+embeddings;
	// the embedder MUST NOT be invoked again (zero re-embeds).
	var warmCalls int64
	idx2, err := openWith(ctx, root, countingStub(&warmCalls), embedderIDStub)
	if err != nil {
		t.Fatalf("warm Open: %v", err)
	}
	if warmCalls != 0 {
		t.Fatalf("warm path re-embedded: %d calls, want 0", warmCalls)
	}
	if idx2.Count() != idx.Count() {
		t.Fatalf("warm index doc count %d != cold %d", idx2.Count(), idx.Count())
	}
}

func TestQueryReturnsRelevantChunk(t *testing.T) {
	t.Setenv("HELIX_CACHE_DIR", t.TempDir())

	root := writeCorpus(t, map[string]string{
		"auth.go":   "package auth\nfunc ValidatePassword(p string) bool { return len(p) > 8 }\n",
		"render.go": "package render\nfunc DrawTriangle() {}\n",
		"net.go":    "package net\nfunc OpenSocket() {}\n",
	})
	ctx := context.Background()

	var calls int64
	idx, err := openWith(ctx, root, countingStub(&calls), embedderIDStub)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	results, err := idx.Query(ctx, "ValidatePassword", 2)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Query returned no results")
	}
	// The chunk cut from auth.go must appear among the top-k for a query that is
	// literally a substring of auth.go (stub embedder is a bag-of-bytes hash, so
	// shared bytes raise similarity).
	found := false
	for _, r := range results {
		if r.RelPath == "auth.go" {
			found = true
		}
		if r.ChunkID == "" {
			t.Fatal("result has empty ChunkID")
		}
	}
	if !found {
		t.Fatalf("auth.go chunk not in top-%d results: %+v", len(results), results)
	}
}
