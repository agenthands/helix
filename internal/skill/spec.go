package skill

import (
	"fmt"
	"os"

	"github.com/agenthands/helix/internal/mcp"
	"gopkg.in/yaml.v3"
)

// ContextSpec defines a tool context loaded from YAML (D-12).
// Controls which skills and tools are active in a given environment.
type ContextSpec struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Skills       []string `yaml:"skills"`        // skill names to activate
	Tools        []string `yaml:"tools"`         // individual tool names to include
	ExcludeTools []string `yaml:"exclude_tools"` // tools to exclude from active skills
}

// ModeSpec defines an operational mode loaded from YAML (D-12).
// Controls behavior patterns within a context.
type ModeSpec struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Skills       []string          `yaml:"skills"`
	Tools        []string          `yaml:"tools"`
	ExcludeTools []string          `yaml:"exclude_tools"`
	Prompts      map[string]string `yaml:"prompts"` // prompt overrides per tool
}

// contextSpecFile is a wrapper for loading a YAML file containing multiple contexts.
type contextSpecFile struct {
	Contexts []ContextSpec `yaml:"contexts"`
}

// modeSpecFile is a wrapper for loading a YAML file containing multiple modes.
type modeSpecFile struct {
	Modes []ModeSpec `yaml:"modes"`
}

// LoadContextSpecs loads context definitions from a YAML file.
func LoadContextSpecs(path string) ([]ContextSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading context specs from %s: %w", path, err)
	}
	var file contextSpecFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing context specs from %s: %w", path, err)
	}
	return file.Contexts, nil
}

// LoadModeSpecs loads mode definitions from a YAML file.
func LoadModeSpecs(path string) ([]ModeSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading mode specs from %s: %w", path, err)
	}
	var file modeSpecFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing mode specs from %s: %w", path, err)
	}
	return file.Modes, nil
}

// ResolveTools takes skill names, include/exclude lists, and the global registry,
// and returns the list of ToolDefs that should be active.
// It gathers all tools from named skills (via ToolProviders), adds includeTools
// by name from the registry, and removes excludeTools.
func ResolveTools(skillNames []string, includeTools []string, excludeTools []string) []*mcp.ToolDef {
	// Build set of desired skill names for fast lookup.
	wantSkills := make(map[string]bool, len(skillNames))
	for _, name := range skillNames {
		wantSkills[name] = true
	}

	// Collect tools from matching ToolProviders.
	toolMap := make(map[string]*mcp.ToolDef)
	for _, tp := range ToolProviders() {
		if wantSkills[tp.Name()] {
			for _, t := range tp.Tools() {
				toolMap[t.Name] = t
			}
		}
	}

	// Add explicitly included tools (from any ToolProvider, regardless of skill filter).
	if len(includeTools) > 0 {
		includeSet := make(map[string]bool, len(includeTools))
		for _, name := range includeTools {
			includeSet[name] = true
		}
		for _, tp := range ToolProviders() {
			for _, t := range tp.Tools() {
				if includeSet[t.Name] {
					toolMap[t.Name] = t
				}
			}
		}
	}

	// Remove excluded tools.
	for _, name := range excludeTools {
		delete(toolMap, name)
	}

	// Collect into sorted slice.
	result := make([]*mcp.ToolDef, 0, len(toolMap))
	for _, t := range toolMap {
		result = append(result, t)
	}
	return result
}
