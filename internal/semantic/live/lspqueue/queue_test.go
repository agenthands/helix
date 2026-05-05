package lspqueue_test

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
)

func TestQueue_EnqueueDequeue(t *testing.T) {
	q := lspqueue.New(1)
	if !q.Enqueue(lspqueue.RevalidateFileJob{Path: "a"}) {
		t.Fatal("first enqueue should succeed")
	}
	if q.Enqueue(lspqueue.RevalidateFileJob{Path: "b"}) {
		t.Fatal("second enqueue on cap-1 buffer should drop")
	}
	got := <-q.Channel()
	if got.Path != "a" {
		t.Fatalf("dequeued %v, want Path=a", got)
	}
}

func TestQueue_DefaultBuffer(t *testing.T) {
	q := lspqueue.New(0) // → default 1024
	for i := 0; i < 1024; i++ {
		if !q.Enqueue(lspqueue.RevalidateFileJob{Path: "x"}) {
			t.Fatalf("enqueue %d should succeed on default-buffer queue", i)
		}
	}
	if q.Enqueue(lspqueue.RevalidateFileJob{Path: "x"}) {
		t.Fatal("enqueue 1025 should drop")
	}
	if got := q.Len(); got != 1024 {
		t.Fatalf("Len() = %d, want 1024", got)
	}
}
