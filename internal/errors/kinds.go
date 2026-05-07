// Package errors defines a typed error taxonomy for Helix's MCP tools.
//
// Import convention: use alias "serr" to avoid shadowing stdlib errors.
//
//	import serr "github.com/agenthands/helix/internal/errors"
package errors

// Kind classifies errors for programmatic matching by agents.
// Callers check Kind via errors.Is(err, serr.ErrNotFound) rather than
// string matching on error messages.
type Kind string

const (
	NotFound         Kind = "not_found"
	InvalidArgs      Kind = "invalid_args"
	NoWorkspace      Kind = "no_workspace"
	Unsupported      Kind = "unsupported"
	Internal         Kind = "internal"
	CircuitOpen      Kind = "circuit_open"
	Timeout          Kind = "timeout"
	PermissionDenied Kind = "permission_denied"
)

// Sentinel errors for use with errors.Is. Each sentinel carries only a Kind;
// Error.Is() compares Kind values, so errors.Is(err, ErrNotFound) matches
// any *Error with Kind == NotFound.
var (
	ErrNotFound         = &Error{Kind: NotFound}
	ErrInvalidArgs      = &Error{Kind: InvalidArgs}
	ErrNoWorkspace      = &Error{Kind: NoWorkspace}
	ErrUnsupported      = &Error{Kind: Unsupported}
	ErrInternal         = &Error{Kind: Internal}
	ErrCircuitOpen      = &Error{Kind: CircuitOpen}
	ErrTimeout          = &Error{Kind: Timeout}
	ErrPermissionDenied = &Error{Kind: PermissionDenied}
)
