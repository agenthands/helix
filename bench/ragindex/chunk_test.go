package ragindex

import (
	"testing"
	"time"
)

// mustParseTime returns a fixed, deterministic past timestamp used by cache
// tests to prove mtime is excluded from CorpusSHA.
func mustParseTime(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2001-02-03T04:05:06Z")
	if err != nil {
		t.Fatalf("parse fixed time: %v", err)
	}
	return ts
}

func TestChunkDeterministic(t *testing.T) {
	content := ""
	for i := 0; i < 200; i++ {
		content += "line of source code number that is reasonably long\n"
	}
	rel := "pkg/foo.go"

	got1 := Chunk(content, rel)
	got2 := Chunk(content, rel)

	if len(got1) == 0 {
		t.Fatal("Chunk returned zero chunks for a non-empty file")
	}
	if len(got1) != len(got2) {
		t.Fatalf("Chunk non-deterministic count: %d != %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i].ID != got2[i].ID {
			t.Fatalf("chunk %d ID not deterministic: %q != %q", i, got1[i].ID, got2[i].ID)
		}
		if got1[i].Content != got2[i].Content {
			t.Fatalf("chunk %d content not deterministic", i)
		}
	}

	// Chunk IDs must be of the form "<rel>#<ordinal>" and start at 0.
	if got1[0].ID != rel+"#0" {
		t.Fatalf("first chunk ID = %q, want %q", got1[0].ID, rel+"#0")
	}
	for i, c := range got1 {
		if c.ID == "" {
			t.Fatalf("chunk %d has empty ID", i)
		}
	}
}

func TestChunkEmptyFile(t *testing.T) {
	// Decision: an empty file yields ZERO chunks (nothing to embed).
	got := Chunk("", "empty.go")
	if len(got) != 0 {
		t.Fatalf("empty file should yield 0 chunks, got %d", len(got))
	}

	// A whitespace-only file is also effectively empty => 0 chunks.
	got = Chunk("\n\n   \n", "ws.go")
	if len(got) != 0 {
		t.Fatalf("whitespace-only file should yield 0 chunks, got %d", len(got))
	}
}

func TestChunkSmallFileSingleChunk(t *testing.T) {
	got := Chunk("package a\nfunc A() {}\n", "a.go")
	if len(got) != 1 {
		t.Fatalf("small file should yield exactly 1 chunk, got %d", len(got))
	}
	if got[0].ID != "a.go#0" {
		t.Fatalf("single-chunk ID = %q, want a.go#0", got[0].ID)
	}
	if got[0].RelPath != "a.go" {
		t.Fatalf("chunk RelPath = %q, want a.go", got[0].RelPath)
	}
}
