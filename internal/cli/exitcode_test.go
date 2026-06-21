package cli

import (
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
)

// TestParseKind_WireForms confirms parseKind recognizes the "kind: message"
// wire form (serr.Error.Error()) and the runVerb-wrapped "calling <tool>:
// <kind>: <msg>" form, scanning for any of the 9 documented kinds.
func TestParseKind_WireForms(t *testing.T) {
	cases := []struct {
		msg  string
		want serr.Kind
		ok   bool
	}{
		{"permission_denied: replace_symbol_body is not available in profile", serr.PermissionDenied, true},
		{"not_found: symbol X", serr.NotFound, true},
		{"calling go_to_definition: permission_denied: blocked", serr.PermissionDenied, true},
		{"invalid_args: bad input", serr.InvalidArgs, true},
		{"some random error with no kind", "", false},
		{"malformed: not a real kind", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := parseKind(tc.msg)
		if ok != tc.ok {
			t.Errorf("parseKind(%q) ok=%v, want %v", tc.msg, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("parseKind(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

// TestExitCodeForKind_FrozenContract pins the frozen exit-code numbering
// (RESEARCH Open Questions (RESOLVED) Q4). These codes are the published
// contract Phase 93 SKILL.md cites — the numbering must not drift.
func TestExitCodeForKind_FrozenContract(t *testing.T) {
	cases := []struct {
		kind serr.Kind
		code int
	}{
		{serr.InvalidArgs, 2},
		{serr.NoWorkspace, 3},
		{serr.NotFound, 4},
		{serr.PermissionDenied, 5},
		{serr.Unsupported, 6},
		{serr.Timeout, 7},
		{serr.CircuitOpen, 8},
		{serr.GuardrailViolation, 9},
		{serr.Internal, 70},
	}
	for _, tc := range cases {
		if got := exitCodeForKind(tc.kind); got != tc.code {
			t.Errorf("exitCodeForKind(%q) = %d, want %d", tc.kind, got, tc.code)
		}
	}
}

// TestExitCodeForKind_UnknownFallback confirms an unrecognized/empty kind maps
// to the generic non-zero fallback (1), never 0.
func TestExitCodeForKind_UnknownFallback(t *testing.T) {
	if got := exitCodeForKind(""); got != 1 {
		t.Errorf("exitCodeForKind(empty) = %d, want 1", got)
	}
	if got := exitCodeForKind(serr.Kind("not_a_kind")); got != 1 {
		t.Errorf("exitCodeForKind(unknown) = %d, want 1", got)
	}
}

// TestExitCodeForKind_DistinctNonZero confirms every one of the 9 kinds maps to
// a distinct, non-zero exit code.
func TestExitCodeForKind_DistinctNonZero(t *testing.T) {
	kinds := []serr.Kind{
		serr.NotFound, serr.InvalidArgs, serr.NoWorkspace, serr.Unsupported,
		serr.Internal, serr.CircuitOpen, serr.Timeout, serr.PermissionDenied,
		serr.GuardrailViolation,
	}
	seen := map[int]serr.Kind{}
	for _, k := range kinds {
		code := exitCodeForKind(k)
		if code == 0 {
			t.Errorf("exitCodeForKind(%q) = 0, want non-zero", k)
		}
		if prev, dup := seen[code]; dup {
			t.Errorf("exit code %d collides: %q and %q", code, prev, k)
		}
		seen[code] = k
	}
}

// TestStderrPrefixForKind confirms the stderr prefix is the Kind value verbatim
// (the stable token the agent branches on).
func TestStderrPrefixForKind(t *testing.T) {
	if got := stderrPrefixForKind(serr.PermissionDenied); got != "permission_denied" {
		t.Errorf("stderrPrefixForKind(PermissionDenied) = %q, want %q", got, "permission_denied")
	}
	if got := stderrPrefixForKind(serr.NotFound); got != "not_found" {
		t.Errorf("stderrPrefixForKind(NotFound) = %q, want %q", got, "not_found")
	}
}
