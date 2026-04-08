package mcp

import "time"

// ModeTransition records a single mode switch for audit purposes (D-07).
type ModeTransition struct {
	From      string
	To        string
	Timestamp time.Time
}

// SessionInfo tracks Serena-specific session state layered on top of MCP SDK sessions.
// Per DMN-06: session keyed by MCP session + dirty buffer overlay + mode/capability profile.
type SessionInfo struct {
	SessionID    string
	WorkspaceKey string   // hash of workspace.WorkspaceKey
	Mode         string   // current operational mode (D-07)
	Profile      string   // active profile name
	AllowedTools []string // tools available in current mode/profile (nil = all)

	// ModeHistory records all mode transitions for auditability (D-07).
	ModeHistory []ModeTransition
}

// RecordModeTransition appends a transition to the mode history and updates
// the current mode.
func (s *SessionInfo) RecordModeTransition(from, to string) {
	s.ModeHistory = append(s.ModeHistory, ModeTransition{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
	})
	s.Mode = to
}
