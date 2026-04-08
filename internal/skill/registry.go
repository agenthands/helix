package skill

import (
	"fmt"
	"sort"
	"sync"
)

var (
	globalMu sync.RWMutex
	skills   = make(map[string]Skill)
)

// Register adds a skill to the global registry.
// Intended to be called from init() in skill packages (Caddy-style registration).
// Duplicate names overwrite silently (last wins).
func Register(s Skill) {
	globalMu.Lock()
	defer globalMu.Unlock()
	skills[s.Name()] = s
}

// Get returns a skill by name and whether it was found.
func Get(name string) (Skill, bool) {
	globalMu.RLock()
	defer globalMu.RUnlock()
	s, ok := skills[name]
	return s, ok
}

// All returns all registered skills sorted by name.
func All() []Skill {
	globalMu.RLock()
	defer globalMu.RUnlock()
	result := make([]Skill, 0, len(skills))
	for _, s := range skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// ToolProviders returns all registered skills that implement ToolProvider,
// sorted by name.
func ToolProviders() []ToolProvider {
	globalMu.RLock()
	defer globalMu.RUnlock()
	var result []ToolProvider
	for _, s := range skills {
		if tp, ok := s.(ToolProvider); ok {
			result = append(result, tp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// WorkflowProviders returns all registered skills that implement WorkflowProvider,
// sorted by name.
func WorkflowProviders() []WorkflowProvider {
	globalMu.RLock()
	defer globalMu.RUnlock()
	var result []WorkflowProvider
	for _, s := range skills {
		if wp, ok := s.(WorkflowProvider); ok {
			result = append(result, wp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// InitAll calls Init on every registered skill with the provided dependencies.
// Returns the first error encountered, or nil if all succeed.
func InitAll(deps SkillDeps) error {
	globalMu.RLock()
	defer globalMu.RUnlock()
	for _, s := range skills {
		if err := s.Init(deps); err != nil {
			return fmt.Errorf("skill %q init failed: %w", s.Name(), err)
		}
	}
	return nil
}

// Reset clears all registrations. Intended for testing only.
func Reset() {
	globalMu.Lock()
	defer globalMu.Unlock()
	skills = make(map[string]Skill)
}
