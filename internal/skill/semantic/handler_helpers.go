package semantic

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
)

// errorResult wraps msg in a CallToolResult with IsError=true. The MCP SDK
// surfaces this to the caller as a structured tool error envelope; the raw
// underlying error is logged via slog.Warn at the call site, never embedded
// in the response (T-64-04-05).
//
// This file is OWNED by P64-04 (W1) and consumed read-only by W2 plans
// (64-05/06/07). W2 plans add new helpers in their own *_helpers.go files
// to preserve exclusive file ownership.
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}

// jsonResult marshals v to JSON and wraps it in a TextContent CallToolResult.
// On marshal error, returns an errorResult — JSON marshal of envelope.go
// types should never fail in production (all fields are basic Go types), so
// the error path is a defensive guard for future envelope changes.
func jsonResult(v any) *mcpsdk.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("internal: failed to marshal response: %v", err))
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(b)},
		},
	}
}

// validatePaths rejects path-traversal attempts and absolute paths outside the
// workspace root. Mirrors internal/skill/repomap/skill.go:311-319 (T-28-05).
//
// Empty input is permitted (paths is optional on every Phase 64 tool). The
// root parameter is the workspace's RepoRoot string; when root is empty,
// absolute-path containment is not enforced (test path / pre-activation).
func validatePaths(paths []string, root string) error {
	for _, p := range paths {
		if strings.Contains(p, "..") {
			return serr.New(serr.InvalidArgs,
				fmt.Sprintf("path %q contains '..' (path traversal not allowed)", p))
		}
		if filepath.IsAbs(p) {
			if root == "" || !strings.HasPrefix(p, root) {
				return serr.New(serr.InvalidArgs,
					fmt.Sprintf("path %q is outside workspace root", p))
			}
		}
	}
	return nil
}
