package mcp

import (
	"sync"
	"time"
)

// ModeTransition records a single mode switch for audit purposes (D-07).
type ModeTransition struct {
	From      string
	To        string
	Timestamp time.Time
}

// SessionInfo tracks Serena-specific session state layered on top of MCP SDK sessions.
// Per DMN-06: session keyed by MCP session + dirty buffer overlay + mode/capability profile.
//
// Thread-safety: All fields are guarded by mu. Callers MUST use the accessor methods
// (Snapshot, SetAllowedTools, RecordModeTransition) rather than touching fields
// directly, because concurrent MCP tool calls and tools/list requests can race on
// Mode / AllowedTools (threat T-08-08: Tampering / EoP on session state under
// concurrent switch_mode + tool invocation). Direct field access is retained only
// for single-writer initialization in the daemon bootstrap before the session is
// exposed to MCP handlers.
type SessionInfo struct {
	mu sync.RWMutex

	SessionID    string
	WorkspaceKey string   // hash of workspace.WorkspaceKey
	Mode         string   // current operational mode (D-07) — read/write via accessors after bootstrap
	Profile      string   // active profile name
	AllowedTools []string // tools available in current mode/profile (nil = all)

	// ModeHistory records all mode transitions for auditability (D-07).
	ModeHistory []ModeTransition
}

// SessionSnapshot is an immutable point-in-time view of a SessionInfo suitable
// for use by readers (middleware, tool handlers) that need a consistent read of
// multiple related fields without holding the session lock across further work.
type SessionSnapshot struct {
	SessionID    string
	WorkspaceKey string
	Mode         string
	Profile      string
	AllowedTools []string // defensive copy; safe to iterate without locking
}

// Snapshot returns a defensively-copied snapshot of the session state. It is
// the preferred read path for code outside of this package.
func (s *SessionInfo) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var allowed []string
	if s.AllowedTools != nil {
		allowed = make([]string, len(s.AllowedTools))
		copy(allowed, s.AllowedTools)
	}
	return SessionSnapshot{
		SessionID:    s.SessionID,
		WorkspaceKey: s.WorkspaceKey,
		Mode:         s.Mode,
		Profile:      s.Profile,
		AllowedTools: allowed,
	}
}

// SetAllowedTools replaces the session's tool whitelist under the write lock.
// The provided slice is copied so callers may safely mutate it afterwards.
func (s *SessionInfo) SetAllowedTools(tools []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tools == nil {
		s.AllowedTools = nil
		return
	}
	copied := make([]string, len(tools))
	copy(copied, tools)
	s.AllowedTools = copied
}

// RecordModeTransition appends a transition to the mode history and updates
// the current mode atomically.
func (s *SessionInfo) RecordModeTransition(from, to string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ModeHistory = append(s.ModeHistory, ModeTransition{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
	})
	s.Mode = to
}
