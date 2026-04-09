//go:build integration

package integration_test

import "testing"

// TestProfile_Contract_Golden (ADV-01): each of the 5 profiles exposes exactly its
// declared tool subset at its default_mode. Source of truth = checked-in golden
// file under testdata/profiles/. Per D-03, we do NOT read YAML here — the oracle
// is the checked-in file, so YAML drift surfaces as a git diff.
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
			tools := listSessionTools(t, td.Session)
			defMode := profileDefaultMode[p]
			assertGoldenTools(t, p+"."+defMode, tools)
		})
	}
}
