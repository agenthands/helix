package mcp

import (
	"log/slog"
	"sync"
)

// ToolDef holds a tool definition before registration with the MCP SDK server.
type ToolDef struct {
	Name             string
	Description      string
	BriefDescription string // short description for tools/list (under 100 tokens, DESC-01)
	HelpText         string // usage examples and patterns for get_tool_help (DESC-02)
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

// BriefDescriptions returns a map of tool name to BriefDescription for all
// tools that have a non-empty BriefDescription (DESC-01).
func (r *ToolRegistry) BriefDescriptions() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descs := make(map[string]string, len(r.tools))
	for name, def := range r.tools {
		if def.BriefDescription != "" {
			descs[name] = def.BriefDescription
		}
	}
	return descs
}

// Get returns the ToolDef for a tool by name, or nil if not found.
func (r *ToolRegistry) Get(name string) *ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}
