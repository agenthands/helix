package mcp

import (
	"log/slog"
	"sync"
)

// ToolDef holds a tool definition before registration with the MCP SDK server.
type ToolDef struct {
	Name        string
	Description string
	// RegisterFn is called to register this tool with the MCP SDK server.
	// This is a callback because the SDK's AddTool is generic and requires type params.
	RegisterFn func(server interface{}) error
}

// ToolRegistry manages tools that can be dynamically added/removed (MCP-07).
type ToolRegistry struct {
	mu     sync.RWMutex
	tools  map[string]*ToolDef
	logger *slog.Logger
}

// NewToolRegistry creates a new registry.
func NewToolRegistry(logger *slog.Logger) *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]*ToolDef),
		logger: logger,
	}
}

// Register adds a tool definition to the registry.
func (r *ToolRegistry) Register(def *ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[def.Name] = def
	r.logger.Info("tool registered", "name", def.Name)
}

// Unregister removes a tool by name.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	r.logger.Info("tool unregistered", "name", name)
}

// Names returns all registered tool names.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
