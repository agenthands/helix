//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProfile_Contract_Golden (ADV-01 / SEC-02): each of the 5 profiles exposes
// exactly its declared tool subset at its default_mode — now asserted against the
// CLI VERB surface (the set of `helix <verb>`s visible+invokable under the
// profile) rather than the raw MCP tools/list surface. The oracle is re-pointed
// to cliVerbSurfaceTools; the golden FILES under testdata/profiles/ are reused
// UNCHANGED (the per-profile allowed set is the contract). Per D-03, we do NOT
// read YAML here — the oracle is the checked-in file, so YAML drift surfaces as a
// git diff. Do NOT run -update: a mismatch means the generated catalog or the
// profile resolution drifted; fix the source, never the golden.
func TestProfile_Contract_Golden(t *testing.T) {
	profiles := []string{"claude-code", "codex", "ide-assistant", "ci-bot", "full"}
	for _, p := range profiles {
		p := p // D-07
		t.Run(p, func(t *testing.T) {
			td := StartTestDaemon(t, Options{
				SkipLS:  true,
				Profile: p,
				// Mode left empty — rely on profile's default_mode.
			})
			tools := cliVerbSurfaceTools(t, td)
			defMode := profileDefaultMode[p]
			assertGoldenTools(t, p+"."+defMode, tools)
		})
	}
}

// TestProfile_CLI_Surface_Refusal (SEC-02, threat T-91-10) proves the BOTH-sides
// contract for the CLI verb surface: a verb whose underlying tool is OUTSIDE the
// active profile must be both
//
//	(i)  hidden  — absent from cliVerbSurfaceTools (the CLI catalog does not
//	     surface it for that profile), AND
//	(ii) refused — a direct tools/call for that tool is denied SERVER-SIDE by the
//	     91-02 ProfileEnforcementMiddleware (returns a typed serr.PermissionDenied
//	     error, NOT a silent success).
//
// CLI-side hiding alone is UX; the daemon refusal is the security boundary — the
// oracle asserts BOTH (91-RESEARCH Anti-Patterns: "ship BOTH"). The destructive
// probe tool is replace_symbol_body, an edit verb absent from read/review-mode
// goldens (e.g. ci-bot.review, ide-assistant.read). Profiles whose default-mode
// golden DOES include it (edit-mode profiles) are skipped for the probe — there
// the tool is legitimately in-surface.
func TestProfile_CLI_Surface_Refusal(t *testing.T) {
	const probeTool = "replace_symbol_body"
	profiles := []string{"claude-code", "codex", "ide-assistant", "ci-bot", "full"}
	for _, p := range profiles {
		p := p // D-07
		t.Run(p, func(t *testing.T) {
			td := StartTestDaemon(t, Options{
				SkipLS:  true,
				Profile: p,
				// Mode left empty — rely on profile's default_mode.
			})

			surface := cliVerbSurfaceTools(t, td)
			inSurface := false
			for _, name := range surface {
				if name == probeTool {
					inSurface = true
					break
				}
			}

			if inSurface {
				// The probe tool is legitimately part of this profile's default-mode
				// surface (an edit-mode profile); the hidden-AND-refused contract
				// does not apply to an in-profile tool. Nothing to prove here.
				t.Skipf("%s is in %s default-mode surface; refusal probe N/A", probeTool, p)
			}

			// (i) hidden: confirmed absent from the CLI verb surface above.
			assert.NotContains(t, surface, probeTool,
				"%s must be hidden from the %s CLI verb surface", probeTool, p)

			// (ii) refused: a direct server-side tools/call must be denied. The
			// 91-02 enforcement returns the deny as a Go error (not an IsError
			// result), so errors.Is(err, serr.ErrPermissionDenied) round-trips.
			_, err := td.Session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: probeTool,
				Arguments: map[string]any{
					"name_path":     "Foo",
					"relative_path": "x.go",
					"body":          "func Foo() {}",
				},
			})
			require.Error(t, err,
				"out-of-profile tool %s must be refused server-side under profile %s, "+
					"not silently succeed (SEC-02 hidden-AND-refused)", probeTool, p)
		})
	}
}
