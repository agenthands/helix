package ragindex

import (
	"context"
	"testing"
)

func TestEmbedderIDsDistinct(t *testing.T) {
	// With OPENAI_API_KEY set, selectEmbedder picks the OpenAI primary.
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	_, openaiID, err := selectEmbedder()
	if err != nil {
		t.Fatalf("selectEmbedder (openai): %v", err)
	}
	if openaiID != embedderIDOpenAI {
		t.Fatalf("with OPENAI_API_KEY set, embedder_id = %q, want %q", openaiID, embedderIDOpenAI)
	}

	// With no key (and forcing offline), selectEmbedder falls back to the
	// deterministic stub.
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv(forceStubEnv, "1")
	_, stubID, err := selectEmbedder()
	if err != nil {
		t.Fatalf("selectEmbedder (stub): %v", err)
	}
	if stubID != embedderIDStub {
		t.Fatalf("offline embedder_id = %q, want %q", stubID, embedderIDStub)
	}

	// All three documented backend IDs must be distinct strings.
	ids := map[string]bool{embedderIDOpenAI: true, embedderIDOllama: true, embedderIDStub: true}
	if len(ids) != 3 {
		t.Fatalf("embedder IDs are not distinct: %v", ids)
	}
}

func TestPinnedEmbedderHonoredOrFailClosed(t *testing.T) {
	// A pinned stub is always usable and selected deterministically, ignoring
	// OPENAI_API_KEY (which would otherwise win the probe order).
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	t.Setenv(pinnedEmbedderEnv, embedderIDStub)
	_, id, err := selectEmbedder()
	if err != nil {
		t.Fatalf("pinned stub: unexpected error: %v", err)
	}
	if id != embedderIDStub {
		t.Fatalf("pinned stub: embedder_id = %q, want %q", id, embedderIDStub)
	}

	// A pinned OpenAI embedder FAILS CLOSED when OPENAI_API_KEY is unset rather
	// than silently falling back to a different embedder (WR-01).
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv(pinnedEmbedderEnv, embedderIDOpenAI)
	if _, _, err := selectEmbedder(); err == nil {
		t.Fatal("pinned openai without key: expected fail-closed error, got nil")
	}

	// An unknown pinned id is rejected.
	t.Setenv(pinnedEmbedderEnv, "not-a-real-embedder")
	if _, _, err := selectEmbedder(); err == nil {
		t.Fatal("unknown pinned embedder: expected error, got nil")
	}
}

func TestStubEmbedderDeterministic(t *testing.T) {
	ef := stubEmbedder()
	ctx := context.Background()

	v1, err := ef(ctx, "hello world")
	if err != nil {
		t.Fatalf("stub embed: %v", err)
	}
	v2, err := ef(ctx, "hello world")
	if err != nil {
		t.Fatalf("stub embed (repeat): %v", err)
	}
	if len(v1) == 0 {
		t.Fatal("stub embedding is empty")
	}
	if len(v1) != len(v2) {
		t.Fatalf("stub embedding length not stable: %d != %d", len(v1), len(v2))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("stub embedding not deterministic at dim %d", i)
		}
	}

	// Different input => (almost certainly) different vector.
	other, err := ef(ctx, "a completely different string")
	if err != nil {
		t.Fatalf("stub embed (other): %v", err)
	}
	same := true
	for i := range v1 {
		if v1[i] != other[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("stub embedding did not vary with input")
	}
}
