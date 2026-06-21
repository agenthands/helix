//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// profileDefaultMode mirrors each profile YAML's default_mode field. Used by
// TestProfile_Contract_Golden so the golden filename matches the effective mode
// at session startup.
var profileDefaultMode = map[string]string{
	"claude-code":   "edit",
	"codex":         "edit",
	"ide-assistant": "read",
	"ci-bot":        "review",
	"full":          "edit",
}

// profileModePairs enumerates every (profile, mode) combination whose mode is
// reachable per the profile's allowed_mode_transitions graph. Derived from the
// YAMLs at planning time; drift in YAML reachability must be reflected here.
var profileModePairs = []struct{ profile, mode string }{
	// claude-code: read/edit/review/admin
	{"claude-code", "read"},
	{"claude-code", "edit"},
	{"claude-code", "review"},
	{"claude-code", "admin"},
	// codex: read/edit/review/admin
	{"codex", "read"},
	{"codex", "edit"},
	{"codex", "review"},
	{"codex", "admin"},
	// ide-assistant: read/edit/review/admin
	{"ide-assistant", "read"},
	{"ide-assistant", "edit"},
	{"ide-assistant", "review"},
	{"ide-assistant", "admin"},
	// ci-bot: read/review/admin (no edit per YAML)
	{"ci-bot", "read"},
	{"ci-bot", "review"},
	{"ci-bot", "admin"},
	// full: all four
	{"full", "read"},
	{"full", "edit"},
	{"full", "review"},
	{"full", "admin"},
}

// TestMode_Contract_Golden (ADV-02): for each valid (profile, mode) combination,
// the visible tool list matches a checked-in golden file.
func TestMode_Contract_Golden(t *testing.T) {
	for _, tc := range profileModePairs {
		tc := tc // D-07
		t.Run(tc.profile+"_"+tc.mode, func(t *testing.T) {
			td := StartTestDaemon(t, Options{
				SkipLS:  true,
				Profile: tc.profile,
				Mode:    tc.mode,
			})
			tools := listSessionTools(t, td.Session)
			assertGoldenTools(t, tc.profile+"."+tc.mode, tools)
		})
	}
}

// TestMode_SwitchRefreshesToolList verifies Assumption A2 + Pitfall 5:
// after switch_mode, a fresh tools/list reflects the new mode. If this fails,
// the MCP SDK is caching list results client-side and our contract tests would
// give false greens after any mode switch.
func TestMode_SwitchRefreshesToolList(t *testing.T) {
	td := StartTestDaemon(t, Options{
		SkipLS:  true,
		Profile: "full",
		Mode:    "admin",
	})
	adminTools := listSessionTools(t, td.Session)

	// Switch to read mode via the tool.
	callTool(t, td.Session, "switch_mode", map[string]any{"target_mode": "read"})
	readTools := listSessionTools(t, td.Session)

	assert.NotEqual(t, adminTools, readTools,
		"tools/list did not change after switch_mode admin->read; "+
			"suspect MCP client cache (A1) or middleware not re-filtering")
	assert.Less(t, len(readTools), len(adminTools),
		"read mode should expose fewer tools than admin")
}

// TestProfile_ExcludedToolNotInvocable verifies the EoP threat (T-08-03):
// a tool missing from tools/list must ALSO error when invoked by name.
// Otherwise the filter is listing-only, not access control.
func TestProfile_ExcludedToolNotInvocable(t *testing.T) {
	// ci-bot excludes replace_symbol_body per its YAML exclude_tools list.
	td := StartTestDaemon(t, Options{
		SkipLS:  true,
		Profile: "ci-bot",
	})
	tools := listSessionTools(t, td.Session)
	// Confirm replace_symbol_body is NOT in the list (precondition sanity check).
	for _, name := range tools {
		if name == "replace_symbol_body" {
			t.Fatalf("precondition failed: ci-bot should not expose replace_symbol_body, got: %v", tools)
		}
	}
	// Try to invoke it anyway; the Phase 91 SEC-01 ProfileEnforcementMiddleware
	// refuses it at the tools/call boundary with a typed PermissionDenied error —
	// stronger than the pre-enforcement IsError-result behavior, since the handler
	// is never reached.
	err := callToolExpectProtocolError(t, td.Session, "replace_symbol_body", map[string]any{
		"name_path":     "Foo",
		"relative_path": "x.go",
		"body":          "func Foo() {}",
	})
	assert.Contains(t, err.Error(), "permission_denied", "excluded tool must be refused with a typed PermissionDenied error")
}
