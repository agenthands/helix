package errors

import (
	"encoding/json"
	"fmt"
)

// Error is the structured error type for all Serena MCP tools.
// It carries a Kind for programmatic matching, a human-readable Message,
// an optional Tool name, an optional Detail string, and an unexported
// cause for error chain traversal.
//
// CRITICAL: Functions returning the error interface must never assign a
// *Error nil pointer to a variable and return it. Always return nil directly.
// A typed nil *Error satisfies error as non-nil, causing false positives.
type Error struct {
	Kind    Kind   `json:"kind"`
	Message string `json:"message"`
	Tool    string `json:"tool,omitempty"`
	Detail  string `json:"detail,omitempty"`
	cause   error
}

// New creates a typed error with the given Kind and message.
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// Wrap creates a typed error that wraps a cause error, preserving the
// error chain for errors.Is and errors.As traversal.
func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, cause: cause}
}

// WithTool sets the tool name on the error and returns the same *Error
// for builder-style chaining.
func (e *Error) WithTool(tool string) *Error {
	e.Tool = tool
	return e
}

// WithDetail sets the detail string on the error and returns the same
// *Error for builder-style chaining.
func (e *Error) WithDetail(detail string) *Error {
	e.Detail = detail
	return e
}

// Error returns a human-readable string: "kind: message" or
// "kind: message (detail)" when Detail is non-empty.
func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap returns the underlying cause error for error chain traversal.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is enables errors.Is matching by Kind. When the target is an *Error,
// it matches if both errors share the same Kind value. This allows
// sentinel-based matching: errors.Is(err, serr.ErrNotFound).
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Kind == t.Kind
	}
	return false
}

// MarshalJSON returns the JSON encoding of the Error. Uses a type alias
// to prevent infinite recursion through the json.Marshaler interface.
func (e *Error) MarshalJSON() ([]byte, error) {
	type alias Error
	return json.Marshal((*alias)(e))
}
