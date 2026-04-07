package workspace

import (
	"fmt"
	"sync"
)

// SessionState holds per-session state (per DMN-06).
// Keyed by MCP session ID + dirty buffer overlay + mode/capability profile.
type SessionState struct {
	SessionID    string
	WorkspaceKey WorkspaceKey
	Mode         string // current mode (planning, editing, etc.)
	Profile      string // capability profile name
	// DirtyBuffers will be populated in Phase 2 for buffer overlay support
}

// WorkspaceState holds per-workspace state (per DMN-05).
type WorkspaceState struct {
	Key    WorkspaceKey
	Status string // "initializing", "ready", "error"
	// LSWorkers will be added in Phase 2
}

// Registry manages active workspaces and sessions (per DMN-01, WRK-01).
type Registry struct {
	mu         sync.RWMutex
	workspaces map[string]*WorkspaceState // keyed by WorkspaceKey.Hash()
	sessions   map[string]*SessionState   // keyed by session ID
}

// NewRegistry creates a new workspace registry.
func NewRegistry() *Registry {
	return &Registry{
		workspaces: make(map[string]*WorkspaceState),
		sessions:   make(map[string]*SessionState),
	}
}

// ActivateWorkspace registers or retrieves a workspace (per WRK-01).
func (r *Registry) ActivateWorkspace(key WorkspaceKey) (*WorkspaceState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hash := key.Hash()
	if ws, ok := r.workspaces[hash]; ok {
		return ws, nil
	}

	ws := &WorkspaceState{
		Key:    key,
		Status: "initializing",
	}
	r.workspaces[hash] = ws
	// Status transitions to "ready" asynchronously (Phase 2: after LS init)
	ws.Status = "ready"
	return ws, nil
}

// GetWorkspace returns a workspace by key, or error if not found.
func (r *Registry) GetWorkspace(key WorkspaceKey) (*WorkspaceState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hash := key.Hash()
	ws, ok := r.workspaces[hash]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", key)
	}
	return ws, nil
}

// RegisterSession creates a new session (per DMN-06).
func (r *Registry) RegisterSession(sessionID string, wsKey WorkspaceKey, mode, profile string) *SessionState {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := &SessionState{
		SessionID:    sessionID,
		WorkspaceKey: wsKey,
		Mode:         mode,
		Profile:      profile,
	}
	r.sessions[sessionID] = s
	return s
}

// RemoveSession removes a session (per DMN-02: daemon survives disconnect).
func (r *Registry) RemoveSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}

// WorkspaceCount returns the number of active workspaces.
func (r *Registry) WorkspaceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workspaces)
}

// SessionCount returns the number of active sessions.
func (r *Registry) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
