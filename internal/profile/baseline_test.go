package profile

import (
	"testing"

	"github.com/agenthands/helix/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaselineProfileLoads asserts that the "baseline" profile is present in the
// embedded profile registry. RED: fails before baseline.yaml is added.
func TestBaselineProfileLoads(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	_, ok := store.Profile("baseline")
	require.True(t, ok, "baseline profile should exist in the registry")
}

// TestBaselineExposesZeroHelixTools verifies Assumption A1:
// a profile with empty skills and empty tools produces zero exposed tools
// when the ProfileFilterMiddleware allow-list is applied.
//
// The filter mechanism in ProfileFilterMiddleware is:
//   - if snap.AllowedTools != nil → filter to that set
//   - an empty (non-nil) AllowedTools slice → zero tools exposed
//
// ResolveTools(nil skills, nil tools, nil exclude) returns []  (empty, non-nil),
// which when passed to SetAllowedTools becomes an empty non-nil slice — exactly
// the zero-tool outcome. This test mirrors that logic inline so it runs without
// the full MCP server stack.
func TestBaselineExposesZeroHelixTools(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	p, ok := store.Profile("baseline")
	require.True(t, ok, "baseline profile must be loaded")

	// Simulate what the daemon does at session start: call ResolveTools with the
	// profile's skills + tools + exclude_tools.
	resolved := skill.ResolveTools(p.Skills, p.Tools, p.ExcludeTools)

	// The resolved slice is the allow-list that SetAllowedTools stores.
	// ProfileFilterMiddleware skips filtering only when AllowedTools == nil.
	// An empty slice causes it to filter to zero tools.
	assert.Empty(t, resolved,
		"baseline profile must resolve to zero tools (Assumption A1); "+
			"if this fails, enable disable_all_tools mechanism in profile.go+middleware.go")
}

// TestBaselineGuardrailsOff asserts that the baseline profile declares guardrails off.
// Eval comparison tasks need an unrestricted baseline with no guardrail noise.
func TestBaselineGuardrailsOff(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	p, ok := store.Profile("baseline")
	require.True(t, ok, "baseline profile must be loaded")

	assert.Equal(t, "off", p.Guardrails.Enforcement,
		"baseline profile must set guardrails.enforcement: off")
}

// TestBaselineDescriptionMentionsEval asserts that the baseline profile description
// contains the substring "eval" so reviewers cannot accidentally repurpose the profile.
func TestBaselineDescriptionMentionsEval(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	p, ok := store.Profile("baseline")
	require.True(t, ok, "baseline profile must be loaded")

	assert.Contains(t, p.Description, "eval",
		"baseline profile description must mention 'eval' to prevent accidental repurposing")
}
