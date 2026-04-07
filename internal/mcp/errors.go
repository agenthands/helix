package mcp

import (
	"encoding/json"
	"errors"
)

// Domain sentinel errors (D-19) -- use errors.Is/As pattern.
var (
	ErrSessionExpired    = errors.New("session expired")
	ErrWorkspaceNotReady = errors.New("workspace not ready")
	ErrToolNotAvailable  = errors.New("tool not available in current mode")
	ErrProjectNotFound   = errors.New("project not found at specified path")
	ErrLSCrashed         = errors.New("language server crashed")
)

// ErrorDetail provides structured error info for MCP responses (D-18).
type ErrorDetail struct {
	Code       string `json:"code"`
	Cause      string `json:"cause"`
	Suggestion string `json:"suggestion,omitempty"`
}

// MarshalJSON returns the JSON encoding of ErrorDetail.
func (e ErrorDetail) MarshalJSON() ([]byte, error) {
	type Alias ErrorDetail
	return json.Marshal((*Alias)(&e))
}
