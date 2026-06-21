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
	NotFound    Kind = "not_found"
	InvalidArgs Kind = "invalid_args"
	NoWorkspace Kind = "no_workspace"
	// Unsupported marks an operation the runtime cannot perform.
	//
	// Phase 76 ABLATE-07 convention (D-06): tools disabled by a kernel
	// subsystem-disable flag (e.g. DisableStructuredEditSubsystem) reuse this
	// kind rather than introducing a new one, and standardize a greppable
	// message prefix of "subsystem_disabled: " so ablation-disabled tool
	// errors can be located via `grep "subsystem_disabled:"`. Example:
	//
	//	serr.New(serr.Unsupported,
	//	  "subsystem_disabled: replace_symbol_body requires the structured-edit subsystem; use replace_in_file")
	Unsupported        Kind = "unsupported"
	Internal           Kind = "internal"
	CircuitOpen        Kind = "circuit_open"
	Timeout            Kind = "timeout"
	PermissionDenied   Kind = "permission_denied"
	GuardrailViolation Kind = "guardrail_violation" // Phase 66 GUARD-04/05
)

// Sentinel errors for use with errors.Is. Each sentinel carries only a Kind;
// Error.Is() compares Kind values, so errors.Is(err, ErrNotFound) matches
// any *Error with Kind == NotFound.
var (
	ErrNotFound           = &Error{Kind: NotFound}
	ErrInvalidArgs        = &Error{Kind: InvalidArgs}
	ErrNoWorkspace        = &Error{Kind: NoWorkspace}
	ErrUnsupported        = &Error{Kind: Unsupported}
	ErrInternal           = &Error{Kind: Internal}
	ErrCircuitOpen        = &Error{Kind: CircuitOpen}
	ErrTimeout            = &Error{Kind: Timeout}
	ErrPermissionDenied   = &Error{Kind: PermissionDenied}
	ErrGuardrailViolation = &Error{Kind: GuardrailViolation} // Phase 66 GUARD-04/05
)

// --- Phase 66 P01: GuardrailViolation typed extension ---

// ReceiptClassRef is a string alias for a ReceiptClass value, used in
// GuardrailViolationDetail to avoid a cyclic import with internal/guardrails.
type ReceiptClassRef = string

// SeeAlsoRef is a structured pointer to a remediation tool invocation.
// Every guardrail violation includes a see_also list pointing to
// get_tool_help with a relevant workflow topic (D-24).
type SeeAlsoRef struct {
	Tool string            `json:"tool"`
	Args map[string]string `json:"args,omitempty"`
}

// GuardrailViolationDetail carries structured fields for a guardrail violation error.
// Recoverable via errors.As(err, **GuardrailViolationDetail) when Kind == "guardrail_violation".
type GuardrailViolationDetail struct {
	Rule             string            `json:"rule"`
	Message          string            `json:"message"`
	RequiredReceipts []ReceiptClassRef `json:"required_receipts,omitempty"`
	SuggestedTools   []string          `json:"suggested_tools,omitempty"`
	SeeAlso          []SeeAlsoRef      `json:"see_also,omitempty"`
}

// guardrailError wraps *Error and carries the typed GuardrailViolationDetail payload.
// errors.Is matches via *Error.Is (Kind comparison); errors.As unwraps to
// *GuardrailViolationDetail via the As method.
//
// Note: *Error is embedded but we must promote Error() to satisfy the error
// interface — embedding *Error promotes the Error() method from the named type.
// However, since the embedded field is named "Error" (same as the method), Go
// resolves this ambiguity by requiring an explicit method forwarding.
type guardrailError struct {
	inner     *Error
	violation GuardrailViolationDetail
}

// Error satisfies the error interface.
func (e *guardrailError) Error() string { return e.inner.Error() }

// Is delegates to the inner *Error for Kind-based matching.
func (e *guardrailError) Is(target error) bool { return e.inner.Is(target) }

// Unwrap returns the inner *Error for errors.Is chain traversal.
func (e *guardrailError) Unwrap() error { return e.inner }

// As implements errors.As for *GuardrailViolationDetail recovery.
func (e *guardrailError) As(target interface{}) bool {
	if t, ok := target.(**GuardrailViolationDetail); ok {
		*t = &e.violation
		return true
	}
	return false
}

// NewGuardrailViolation constructs a guardrail violation error.
// errors.Is(err, ErrGuardrailViolation) returns true; AsGuardrailViolation
// extracts *GuardrailViolationDetail from the typed wrapper.
func NewGuardrailViolation(rule, message string, required []ReceiptClassRef, suggested []string, seeAlso []SeeAlsoRef) error {
	return &guardrailError{
		inner: &Error{
			Kind:    GuardrailViolation,
			Message: message,
		},
		violation: GuardrailViolationDetail{
			Rule:             rule,
			Message:          message,
			RequiredReceipts: required,
			SuggestedTools:   suggested,
			SeeAlso:          seeAlso,
		},
	}
}

// AsGuardrailViolation extracts the *GuardrailViolationDetail from an error
// produced by NewGuardrailViolation, walking the error chain.
// Returns nil if err is not a guardrail violation.
func AsGuardrailViolation(err error) *GuardrailViolationDetail {
	var ge *guardrailError
	// Walk the chain manually: errors.As with *guardrailError target.
	for err != nil {
		if e, ok := err.(*guardrailError); ok {
			ge = e
			break
		}
		// Unwrap one level.
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	if ge == nil {
		return nil
	}
	return &ge.violation
}
