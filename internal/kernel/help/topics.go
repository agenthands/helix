package help

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"sync"
)

// Topic holds the name and embedded markdown content for a get_tool_help topic.
type Topic struct {
	Name    string
	Content string
}

// TopicRegistry is a thread-safe registry of named documentation topics.
// Topics are loaded at daemon start from the embedded docs FS and returned
// verbatim when the get_tool_help handler receives a `topic` argument.
type TopicRegistry struct {
	mu     sync.RWMutex
	topics map[string]*Topic
}

// NewTopicRegistry returns an empty TopicRegistry.
func NewTopicRegistry() *TopicRegistry {
	return &TopicRegistry{topics: make(map[string]*Topic)}
}

// Register adds or replaces a topic in the registry.
func (r *TopicRegistry) Register(name string, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.topics[name] = &Topic{Name: name, Content: content}
}

// Get returns the topic with the given name and true if found, or nil and false if not.
// Dispatch is exact-match only (D-24 + RESEARCH OI-04 — prefix dispatch deferred).
func (r *TopicRegistry) Get(name string) (*Topic, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.topics[name]
	return t, ok
}

// Names returns the sorted list of all registered topic names.
func (r *TopicRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.topics))
	for n := range r.topics {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// LoadDefaults populates the 6 Phase 66 D-24 topics from the embedded docs FS.
// The mapping is: topic name → path within the embed.FS.
func (r *TopicRegistry) LoadDefaults(fsys embed.FS) error {
	mapping := map[string]string{
		"guardrails":                       "docs/guardrails.md",
		"dod":                              "docs/dod.md",
		"workflow:rename":                  "docs/workflow_rename.md",
		"workflow:delete":                  "docs/workflow_delete.md",
		"workflow:large-edit":              "docs/workflow_large_edit.md",
		"workflow:security-sensitive-edit": "docs/workflow_security_sensitive_edit.md",
	}
	for topic, path := range mapping {
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("topic %q load %q: %w", topic, path, err)
		}
		r.Register(topic, string(b))
	}
	return nil
}

// defaultTopics is the package-level singleton topic registry, loaded at init time
// from the embedded docs FS. The get_tool_help handler uses this for topic dispatch.
var defaultTopics = NewTopicRegistry()

func init() {
	if err := defaultTopics.LoadDefaults(EmbeddedTopicDocs); err != nil {
		// This can only fail if the embedded FS is missing expected files,
		// which is a build-time error caught by go:embed. Panic at init to
		// surface it immediately rather than silently serving empty topics.
		panic("help: failed to load default topics from embedded FS: " + err.Error())
	}
}
