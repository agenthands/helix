package retrieval

import (
	"context"
	"path/filepath"
	"testing"
)

// bleve_test.go — RED-gate failing tests for retrieval.Engine.
// Stub Engine methods panic; these tests compile (exercising the typed
// signature) and fail at runtime until Task 1 GREEN supplies the real
// implementation.

// TestBleve_NewOpenClose exercises the create + close cycle in a tempdir,
// then re-Open and Close again. Asserts no error along the path.
func TestBleve_NewOpenClose(t *testing.T) {
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "test.bleve")

	eng, err := New(idxPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := eng.Close(); err != nil {
		t.Fatalf("Close after New: %v", err)
	}

	eng2, err := Open(idxPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := eng2.Close(); err != nil {
		t.Fatalf("Close after Open: %v", err)
	}
}

// TestBleve_UpsertAndQuery indexes 5 SymbolDocs with overlapping terms in
// name + doc, then queries "foo bar". Asserts at least one hit with score > 0.
func TestBleve_UpsertAndQuery(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(filepath.Join(dir, "qtest.bleve"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer eng.Close()

	docs := []SymbolDoc{
		{ID: "1", Name: "foo handler", Path: "src auth", Doc: "handles foo bar requests"},
		{ID: "2", Name: "bar utility", Path: "src bar", Doc: "utility for bar things"},
		{ID: "3", Name: "baz computer", Path: "src compute", Doc: "computes baz"},
		{ID: "4", Name: "foobar combined", Path: "src api", Doc: "combined foo and bar logic"},
		{ID: "5", Name: "qux entry", Path: "src qux", Doc: "qux entry point"},
	}
	if err := eng.UpsertBatch(context.Background(), docs); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	hits, err := eng.QueryBleve("foo bar", nil)
	if err != nil {
		t.Fatalf("QueryBleve: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected ≥1 hit for query 'foo bar', got 0")
	}
	if hits[0].Score <= 0 {
		t.Fatalf("top hit score: got %v, want > 0", hits[0].Score)
	}
}

// TestBleve_MetaRoundTrip exercises SetMeta + GetMeta on the bleve internal
// store. The recovery procedure uses these to persist
// `last_indexed_snapshot_id` across daemon restarts.
func TestBleve_MetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(filepath.Join(dir, "meta.bleve"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer eng.Close()

	if err := eng.SetMeta("last_snap", []byte("42")); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	got, err := eng.GetMeta("last_snap")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if string(got) != "42" {
		t.Fatalf("GetMeta value: got %q, want %q", string(got), "42")
	}
}
