package mcp

import (
	"encoding/json"

	serr "github.com/postfix/serena/internal/errors"
)

// Domain sentinel errors -- re-exported from internal/errors for backward compatibility.
// Callers should migrate to serr.ErrXxx in Phase 23.
var (
	ErrSessionExpired    = serr.New(serr.Timeout, "session expired")
	ErrWorkspaceNotReady = serr.ErrNoWorkspace
	ErrToolNotAvailable  = serr.ErrUnsupported
	ErrProjectNotFound   = serr.ErrNotFound
	ErrLSCrashed         = serr.New(serr.Internal, "language server crashed")
)

// ErrorDetail provides structured error info for MCP responses.
// Deprecated: will be superseded by serr.Error in Phase 23.
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
