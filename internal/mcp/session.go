package mcp

// SessionInfo tracks Serena-specific session state layered on top of MCP SDK sessions.
// Per DMN-06: session keyed by MCP session + dirty buffer overlay + mode/capability profile.
type SessionInfo struct {
	SessionID    string
	WorkspaceKey string   // hash of workspace.WorkspaceKey
	Mode         string   // current operational mode
	Profile      string   // capability profile
	AllowedTools []string // tools available in current mode/profile (nil = all)
}
