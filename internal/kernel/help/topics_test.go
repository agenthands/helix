package help

import (
	"testing"
)

func TestTopicRegistry_Register_GetExactMatch(t *testing.T) {
	r := NewTopicRegistry()
	r.Register("my-topic", "# My Topic\n\nSome content here.")

	topic, ok := r.Get("my-topic")
	if !ok {
		t.Fatal("expected to find topic 'my-topic', got not found")
	}
	if topic == nil {
		t.Fatal("expected non-nil topic")
	}
	if topic.Name != "my-topic" {
		t.Errorf("expected Name 'my-topic', got %q", topic.Name)
	}
	if topic.Content != "# My Topic\n\nSome content here." {
		t.Errorf("unexpected content: %q", topic.Content)
	}
}

func TestTopicRegistry_Get_UnknownReturnsFalse(t *testing.T) {
	r := NewTopicRegistry()

	_, ok := r.Get("does-not-exist")
	if ok {
		t.Error("expected Get to return false for unknown topic, got true")
	}
}

func TestTopicRegistry_LoadDefaults_All6Present(t *testing.T) {
	r := NewTopicRegistry()
	if err := r.LoadDefaults(EmbeddedTopicDocs); err != nil {
		t.Fatalf("LoadDefaults failed: %v", err)
	}

	expected := []string{
		"guardrails",
		"dod",
		"workflow:rename",
		"workflow:delete",
		"workflow:large-edit",
		"workflow:security-sensitive-edit",
	}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			topic, ok := r.Get(name)
			if !ok {
				t.Fatalf("expected topic %q to be registered, not found", name)
			}
			if topic == nil {
				t.Fatal("expected non-nil topic")
			}
			if len(topic.Content) < 100 {
				t.Errorf("topic %q content too short (%d bytes), expected >= 100", name, len(topic.Content))
			}
		})
	}
}

func TestTopicRegistry_Names_Sorted(t *testing.T) {
	r := NewTopicRegistry()
	r.Register("zebra", "content")
	r.Register("apple", "content")
	r.Register("mango", "content")

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "apple" || names[1] != "mango" || names[2] != "zebra" {
		t.Errorf("expected sorted [apple, mango, zebra], got %v", names)
	}
}

func TestTopicRegistry_DefaultSingleton_All6Present(t *testing.T) {
	// defaultTopics is initialized in init(); verify it has all 6 topics.
	expected := []string{
		"guardrails",
		"dod",
		"workflow:rename",
		"workflow:delete",
		"workflow:large-edit",
		"workflow:security-sensitive-edit",
	}

	for _, name := range expected {
		topic, ok := defaultTopics.Get(name)
		if !ok {
			t.Errorf("defaultTopics missing topic %q", name)
			continue
		}
		if len(topic.Content) < 100 {
			t.Errorf("defaultTopics topic %q content too short (%d bytes)", name, len(topic.Content))
		}
	}
}
