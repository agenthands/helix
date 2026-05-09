package guardrails

import (
	"errors"
	"testing"
)

func TestParseEnforcementLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    EnforcementLevel
		wantErr bool
	}{
		{"off", LevelOff, false},
		{"warn", LevelWarn, false},
		{"enforce", LevelEnforce, false},
		{"require_force", LevelRequireForce, false},
		{"", "", true},
		{"ENFORCE", "", true},
		{"block", "", true},
		{"require-force", "", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseEnforcementLevel(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseEnforcementLevel(%q) returned nil error; want error", tc.input)
				}
				var e *ErrInvalidEnforcementLevel
				if !errors.As(err, &e) {
					t.Errorf("ParseEnforcementLevel(%q) error type = %T; want *ErrInvalidEnforcementLevel", tc.input, err)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseEnforcementLevel(%q) returned error %v; want nil", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseEnforcementLevel(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveGuardrailEnforcement(t *testing.T) {
	t.Parallel()

	set := func(l EnforcementLevel) EnforcementLayer { return EnforcementLayer{Level: l, Set: true} }
	unset := func() EnforcementLayer { return EnforcementLayer{} }

	tests := []struct {
		name    string
		cli     EnforcementLayer
		perTool EnforcementLayer
		perRule EnforcementLayer
		profile EnforcementLayer
		global  EnforcementLayer
		want    EnforcementLevel
	}{
		{
			name:   "all unset returns default warn",
			cli:    unset(), perTool: unset(), perRule: unset(), profile: unset(), global: unset(),
			want: LevelWarn,
		},
		{
			name:   "global=enforce all else unset",
			cli:    unset(), perTool: unset(), perRule: unset(), profile: unset(), global: set(LevelEnforce),
			want: LevelEnforce,
		},
		{
			name:   "cli wins over global",
			cli:    set(LevelEnforce), perTool: unset(), perRule: unset(), profile: unset(), global: set(LevelWarn),
			want: LevelEnforce,
		},
		{
			name:   "per-rule wins over profile and global",
			cli:    unset(), perTool: unset(), perRule: set(LevelWarn), profile: set(LevelEnforce), global: set(LevelEnforce),
			want: LevelWarn,
		},
		{
			name:   "per-tool wins over per-rule",
			cli:    unset(), perTool: set(LevelOff), perRule: set(LevelEnforce), profile: unset(), global: unset(),
			want: LevelOff,
		},
		{
			name:   "profile wins when cli/per-tool/per-rule unset",
			cli:    unset(), perTool: unset(), perRule: unset(), profile: set(LevelRequireForce), global: set(LevelWarn),
			want: LevelRequireForce,
		},
		{
			name:   "D-20 fixture: global=enforce, per-rule G-001=warn, per-tool unset, profile=warn, cli unset => warn",
			cli:    unset(), perTool: unset(), perRule: set(LevelWarn), profile: set(LevelWarn), global: set(LevelEnforce),
			want: LevelWarn, // per-rule wins (first set layer)
		},
		{
			name:   "D-20 fixture: global=warn, cli=enforce => enforce",
			cli:    set(LevelEnforce), perTool: unset(), perRule: unset(), profile: unset(), global: set(LevelWarn),
			want: LevelEnforce,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveGuardrailEnforcement(tc.cli, tc.perTool, tc.perRule, tc.profile, tc.global)
			if got != tc.want {
				t.Errorf("ResolveGuardrailEnforcement() = %q; want %q", got, tc.want)
			}
		})
	}
}

// TestResolveGuardrailEnforcement_HighestPrecedenceWinsNotMostRestrictive verifies
// that the resolver does NOT use most-restrictive-wins logic. This test would
// fail if the implementation compared level values instead of layer precedence.
func TestResolveGuardrailEnforcement_HighestPrecedenceWinsNotMostRestrictive(t *testing.T) {
	t.Parallel()

	// If most-restrictive-wins: result would be enforce (more restrictive than warn).
	// If highest-precedence-set-wins: per-rule=warn wins over global=enforce.
	got := ResolveGuardrailEnforcement(
		EnforcementLayer{},                            // cli unset
		EnforcementLayer{},                            // per-tool unset
		EnforcementLayer{Level: LevelWarn, Set: true}, // per-rule = warn
		EnforcementLayer{},                            // profile unset
		EnforcementLayer{Level: LevelEnforce, Set: true}, // global = enforce
	)

	if got == LevelEnforce {
		t.Errorf("ResolveGuardrailEnforcement used most-restrictive-wins logic (returned enforce); "+
			"expected highest-precedence-set-wins (should return warn because per-rule=warn is set and has higher precedence than global=enforce)")
	}
	if got != LevelWarn {
		t.Errorf("got %q; want %q", got, LevelWarn)
	}
}
