package cli

import (
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// exitcode.go maps the typed error taxonomy (internal/errors.Kind) to the CLI's
// process exit codes and stable stderr prefixes. These codes are the PUBLISHED
// CONTRACT that Phase 93 SKILL.md cites, so the numbering is frozen here
// (RESEARCH Open Questions (RESOLVED) Q4): an agent branches on the exit code
// and the stderr prefix, never on prose. Do not renumber without bumping the
// documented contract.

// knownKinds is the fixed set of the 9 documented serr.Kind values. parseKind
// scans for ONLY these — arbitrary "word:" prefixes are rejected to defend
// against error-kind spoofing (T-92-02: a daemon message body containing a
// "kind:"-looking token must not be mistaken for a real typed kind).
var knownKinds = []serr.Kind{
	serr.NotFound,
	serr.InvalidArgs,
	serr.NoWorkspace,
	serr.Unsupported,
	serr.Internal,
	serr.CircuitOpen,
	serr.Timeout,
	serr.PermissionDenied,
	serr.GuardrailViolation,
}

// exitCodeByKind freezes the kind→code numbering. The 9 codes are distinct and
// non-zero; an unrecognized kind falls back to 1 (generic non-zero) via
// exitCodeForKind.
var exitCodeByKind = map[serr.Kind]int{
	serr.InvalidArgs:        2,
	serr.NoWorkspace:        3,
	serr.NotFound:           4,
	serr.PermissionDenied:   5,
	serr.Unsupported:        6,
	serr.Timeout:            7,
	serr.CircuitOpen:        8,
	serr.GuardrailViolation: 9,
	serr.Internal:           70,
}

// parseKind scans an error message for any of the 9 documented serr.Kind
// values appearing as a "<kind>:" token, returning the recognized kind. It
// handles both wire forms: the bare "kind: message" rendered by
// serr.Error.Error(), and the runVerb-wrapped "calling <tool>: <kind>: <msg>"
// form (verb.go:214). The serr wire form is "<kind>: <message> [(detail)]"
// (errors.go:51-56), and the ONLY prefix runVerb ever prepends is the non-kind
// "calling <tool>: " wrapper. The TRUE kind is therefore always the LEFTMOST
// recognized "<kind>:" token; selecting it lets the genuine leading kind win
// over any incidental "<kind>:"-looking token that appears later in the message
// BODY or detail.
//
// Security (T-92-02 kind-spoofing guard): a message body that quotes another
// kind's token (e.g. `invalid_args: value must be one of ...; got
// "permission_denied:x"`) is exactly the spoof vector. Leftmost-wins makes the
// genuine leading kind win over the spoofed body token; the prior rightmost
// scan let the body win. A kind also matches ONLY when it is one of the 9 enum
// values followed immediately by a colon — an arbitrary "word:" prefix such as
// "malformed:" never matches.
func parseKind(msg string) (serr.Kind, bool) {
	bestIdx := -1
	var best serr.Kind
	for _, k := range knownKinds {
		tok := string(k) + ":"
		i := strings.Index(msg, tok)
		if i < 0 {
			continue
		}
		// Guard against a longer kind name ending in a shorter one: require the
		// match to start at a word boundary (start of string or a non-identifier
		// char before it) so "not_a_not_found:" doesn't false-match "not_found:".
		if i != 0 && isKindNameByte(msg[i-1]) {
			continue
		}
		// Select the leftmost recognized kind token: the genuine kind always
		// leads (after the optional non-kind "calling <tool>: " wrapper, which
		// contains no kind token), so an earlier match is the real kind.
		if bestIdx < 0 || i < bestIdx {
			bestIdx = i
			best = k
		}
	}
	if bestIdx < 0 {
		return "", false
	}
	return best, true
}

// isKindNameByte reports whether b can appear inside a serr.Kind identifier
// (lowercase letters and underscores). Used to enforce a left word boundary in
// parseKind so a kind token is not matched as a suffix of a longer word.
func isKindNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || b == '_'
}

// exitCodeForKind returns the frozen process exit code for a kind, or 1 (the
// generic non-zero fallback) for an unrecognized/empty kind. It never returns 0
// — a typed error always exits non-zero.
func exitCodeForKind(kind serr.Kind) int {
	if code, ok := exitCodeByKind[kind]; ok {
		return code
	}
	return 1
}

// stderrPrefixForKind returns the stable stderr prefix for a kind: the Kind
// value verbatim (e.g. "permission_denied"). This is the token the agent
// branches on, so it is intentionally identical to the enum string.
func stderrPrefixForKind(kind serr.Kind) string {
	return string(kind)
}

// ExitCodeForError is the exported entrypoint cmd/helix/main.go uses to map a
// CLI error to its process exit code. It parses the typed `<kind>:` token from
// the error message (parseKind) and returns the frozen per-kind code
// (exitCodeForKind); an error with no recognized kind falls through to 1, the
// generic non-zero code. A nil error returns 0. The numbering is the published
// contract (see exitcode.go header).
func ExitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	kind, ok := parseKind(err.Error())
	if !ok {
		return 1
	}
	return exitCodeForKind(kind)
}
