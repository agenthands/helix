//go:build integration || llm || llmjudge

// TODO(phase-20): Add timeout, circuit_open, and unsupported error category goldens
// once Phase 20 provides runtime stress infrastructure to trigger these deterministically.
// Currently covered: no_workspace, not_found, invalid_args.

// Error category contract tests (CONT-03).
//
// Each deterministic error category has a golden file capturing the stable error
// response structure. Error responses are verified to:
//   - Return IsError=true
//   - Not contain misleading success payloads
//   - Be deterministic (identical across repeated calls)

package contract_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// errorCase describes one error category test.
type errorCase struct {
	category       string         // error category name
	tool           string         // tool to trigger the error
	args           map[string]any // arguments that trigger this category
	needsWorkspace bool           // whether to activate a workspace first
}

// errorCategories returns the deterministic error categories with their trigger cases.
func errorCategories() []errorCase {
	return []errorCase{
		// no_workspace: call a workspace-requiring tool without activating workspace.
		{
			category:       "no_workspace",
			tool:           "read_file",
			args:           map[string]any{"path": "main.go"},
			needsWorkspace: false,
		},
		// not_found: call a file tool on a nonexistent file in an active workspace.
		{
			category:       "not_found",
			tool:           "read_file",
			args:           map[string]any{"path": "nonexistent_file_xyz.go"},
			needsWorkspace: true,
		},
		// invalid_args: call a tool with missing required arguments.
		{
			category:       "invalid_args",
			tool:           "search_symbols",
			args:           map[string]any{},
			needsWorkspace: false,
		},
	}
}

// errorGoldenDir returns the directory for error category golden files (D-06).
func errorGoldenDir() string {
	return filepath.Join(harness.ProjectRoot(), "test", "oracle", "contract", "testdata", "golden", "errors")
}

// successPhrases are strings that should never appear in error responses.
var successPhrases = []string{"Success", "completed successfully", "Done"}

// TestError_CategoryContracts verifies each deterministic error category
// returns IsError=true and captures the error response as a golden file (CONT-03).
func TestError_CategoryContracts(t *testing.T) {
	gDir := errorGoldenDir()

	// Start a runner without workspace for no_workspace and invalid_args cases.
	noWSRunner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	// Start a runner with workspace for not_found cases.
	fixtureDir := harness.PrepareFixture(t, "go")
	wsRunner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	for _, tc := range errorCategories() {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			session := noWSRunner.Session
			wsDir := ""
			if tc.needsWorkspace {
				session = wsRunner.Session
				wsDir = fixtureDir
			}

			result := harness.CallToolExpectError(t, session, tc.tool, tc.args)
			text := harness.TextContent(result)
			normalized := normalizeResponse(text, wsDir)

			goldenPath := filepath.Join(gDir, tc.category+".golden")
			harness.AssertGolden(t, goldenPath, []byte(normalized))
		})
	}
}

// TestError_NoMisleadingSuccessPayload verifies that error responses do not
// contain success-like phrases that could mislead clients (CONT-03, T-19-08).
func TestError_NoMisleadingSuccessPayload(t *testing.T) {
	// Start a runner without workspace for no_workspace and invalid_args cases.
	noWSRunner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	// Start a runner with workspace for not_found cases.
	fixtureDir := harness.PrepareFixture(t, "go")
	wsRunner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	for _, tc := range errorCategories() {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			session := noWSRunner.Session
			if tc.needsWorkspace {
				session = wsRunner.Session
			}

			result := harness.CallToolExpectError(t, session, tc.tool, tc.args)
			require.True(t, result.IsError, "expected IsError=true for category %s", tc.category)

			text := harness.TextContent(result)
			for _, phrase := range successPhrases {
				assert.False(t, strings.Contains(text, phrase),
					"error response for category %s contains misleading success phrase %q: %s",
					tc.category, phrase, text)
			}
		})
	}
}

// TestError_StableErrorStructure verifies that error responses are deterministic:
// calling the same tool with the same (bad) arguments twice produces identical
// normalized error text (CONT-03, T-19-09).
func TestError_StableErrorStructure(t *testing.T) {
	// Start a runner without workspace for no_workspace and invalid_args cases.
	noWSRunner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	// Start a runner with workspace for not_found cases.
	fixtureDir := harness.PrepareFixture(t, "go")
	wsRunner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	for _, tc := range errorCategories() {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			session := noWSRunner.Session
			wsDir := ""
			if tc.needsWorkspace {
				session = wsRunner.Session
				wsDir = fixtureDir
			}

			// First invocation.
			result1 := harness.CallToolExpectError(t, session, tc.tool, tc.args)
			text1 := normalizeResponse(harness.TextContent(result1), wsDir)

			// Second invocation.
			result2 := harness.CallToolExpectError(t, session, tc.tool, tc.args)
			text2 := normalizeResponse(harness.TextContent(result2), wsDir)

			assert.Equal(t, text1, text2,
				"error response for category %s is not deterministic", tc.category)
		})
	}
}
