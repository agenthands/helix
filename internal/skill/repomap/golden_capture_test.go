// golden_capture_test.go — single-shot helper used to (re)generate the
// Phase 65 INTEG-01 byte-identical goldens at
// testdata/goldens/index_disabled_{repo_map,context}.txt.
//
// This is NOT a regression test. It runs only when the env var
// HELIX_GOLDEN_CAPTURE=1 is set; otherwise it is a no-op. The companion
// regression test is TestEnvelope_IndexDisabledIsTreeSitter (skill_integration_test.go),
// which reads the captured goldens and asserts byte-identical equality.
//
// Phase 65 INTEG-01 contract — internal/repomap engine source MUST NOT
// change. If this capture diverges after engine modification, the
// offending change violates INTEG-01 and the test below will fail.

package repomap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCaptureIndexDisabledGoldens regenerates the byte-identical goldens.
// Run with: HELIX_GOLDEN_CAPTURE=1 go test ./internal/skill/repomap/... -run TestCaptureIndexDisabledGoldens
func TestCaptureIndexDisabledGoldens(t *testing.T) {
	if os.Getenv("HELIX_GOLDEN_CAPTURE") != "1" {
		t.Skip("HELIX_GOLDEN_CAPTURE not set; skipping golden capture")
	}

	// Produce an absolute path resolved from the worktree root, NOT a
	// relative path — the test is invoked with go test's cwd set to the
	// package directory, but goldens live two levels up from there.
	pkgDir, err := os.Getwd()
	require.NoError(t, err)
	goldensDir := filepath.Join(pkgDir, "testdata", "goldens")
	require.NoError(t, os.MkdirAll(goldensDir, 0o755))

	// get_repo_map golden
	{
		s, _ := newIntegrationSkill(t)
		// Config gate omitted → ChooseSource emits SourceTreeSitter (D-04 / Pitfall §3).
		result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
			"token_budget": float64(4096),
		})
		require.NoError(t, err)
		var env struct {
			Source string `json:"source"`
			Tree   string `json:"tree"`
		}
		require.NoError(t, json.Unmarshal([]byte(result), &env))
		require.Equal(t, "tree_sitter", env.Source)
		require.NoError(t, os.WriteFile(
			filepath.Join(goldensDir, "index_disabled_repo_map.txt"),
			[]byte(env.Tree), 0o644))
	}

	// get_context golden
	{
		s, dir := newIntegrationSkill(t)
		result, err := s.ExecuteTool("get_context", map[string]interface{}{
			"files":        []interface{}{filepath.Join(dir, "server.go")},
			"token_budget": float64(4096),
		})
		require.NoError(t, err)
		var env struct {
			Source string `json:"source"`
			Tree   string `json:"tree"`
		}
		require.NoError(t, json.Unmarshal([]byte(result), &env))
		require.Equal(t, "tree_sitter", env.Source)
		require.NoError(t, os.WriteFile(
			filepath.Join(goldensDir, "index_disabled_context.txt"),
			[]byte(env.Tree), 0o644))
	}

	t.Logf("goldens written under %s", goldensDir)
}
