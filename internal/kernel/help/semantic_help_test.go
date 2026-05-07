// Phase 64 P64-08 Task 2: get_tool_help coverage for the four semantic
// MCP tools.
//
// Closes CONTEXT.md acceptance test #5 (TOOL-05): get_tool_help returns
// parameter docs for each of the four Phase 64 tools. The test stays in
// internal/kernel/help/ so the help-package import surface and
// ExtractParamDocs / FormatHelp behavior are exercised against the same
// schema-generation path the production daemon uses.
//
// Schema generation: the MCP go-sdk infers the JSON schema from a typed-
// args struct via google/jsonschema-go's jsonschema.ForType (see
// modelcontextprotocol/go-sdk/mcp/server.go:436). The test rebuilds the
// schema via jsonschema.For[T]() for each tool's args struct so the
// extraction path is byte-for-byte identical to what the running daemon
// would feed into ExtractParamDocs.

package help_test

import (
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/agenthands/helix/internal/kernel/help"
	"github.com/agenthands/helix/internal/skill/semantic"
)

// schemaFor returns the JSON-schema Go object for a typed-args struct,
// using the same generator the MCP server uses at registration time.
func schemaFor[T any](t *testing.T) any {
	t.Helper()
	s, err := jsonschema.For[T](nil)
	if err != nil {
		t.Fatalf("jsonschema.For: %v", err)
	}
	return s
}

// hasParam asserts the docs slice contains a ParamDoc entry with name == n.
func hasParam(t *testing.T, where string, docs []help.ParamDoc, n string) {
	t.Helper()
	for _, d := range docs {
		if d.Name == n {
			return
		}
	}
	t.Errorf("%s: missing parameter %q in docs %+v", where, n, docs)
}

// TestGetToolHelp_IndexSemanticGraph_ReturnsParameterDocs asserts the
// index_semantic_graph args struct surfaces all 3 params (mode,
// max_duration_ms, paths) via ExtractParamDocs.
func TestGetToolHelp_IndexSemanticGraph_ReturnsParameterDocs(t *testing.T) {
	docs := help.ExtractParamDocs(schemaFor[semantic.IndexSemanticGraphArgs](t))
	if len(docs) == 0 {
		t.Fatalf("ExtractParamDocs returned empty docs")
	}
	for _, want := range []string{"mode", "max_duration_ms", "paths"} {
		hasParam(t, "index_semantic_graph", docs, want)
	}

	// Smoke-check FormatHelp produces a non-empty rendering with all
	// params + the const helpText together.
	rendered := help.FormatHelp(
		"index_semantic_graph",
		"Build or refresh a committed semantic snapshot.",
		docs,
		"## Mode Tier\nreview+\n",
	)
	for _, want := range []string{"index_semantic_graph", "mode", "max_duration_ms", "paths", "review+"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("FormatHelp output missing %q; got:\n%s", want, rendered)
		}
	}
}

// TestGetToolHelp_RefreshSemanticGraph_ReturnsParameterDocs asserts
// refresh_semantic_graph surfaces all 3 params (paths, wait_for_lsp,
// max_wait_ms).
func TestGetToolHelp_RefreshSemanticGraph_ReturnsParameterDocs(t *testing.T) {
	docs := help.ExtractParamDocs(schemaFor[semantic.RefreshSemanticGraphArgs](t))
	if len(docs) == 0 {
		t.Fatalf("ExtractParamDocs returned empty docs")
	}
	for _, want := range []string{"paths", "wait_for_lsp", "max_wait_ms"} {
		hasParam(t, "refresh_semantic_graph", docs, want)
	}
}

// TestGetToolHelp_GetSemanticGraphStatus_ReturnsHelpText asserts
// get_semantic_graph_status takes no args; ExtractParamDocs returns nil
// (or empty), and FormatHelp still produces a non-empty rendering driven
// by description + helpText.
func TestGetToolHelp_GetSemanticGraphStatus_ReturnsHelpText(t *testing.T) {
	docs := help.ExtractParamDocs(schemaFor[semantic.GetSemanticGraphStatusArgs](t))
	// Empty struct should produce no params (or nil docs).
	if len(docs) != 0 {
		t.Errorf("expected 0 params for get_semantic_graph_status; got %d (%+v)", len(docs), docs)
	}

	rendered := help.FormatHelp(
		"get_semantic_graph_status",
		"Return semantic graph status (read+).",
		docs,
		"## Mode Tier\nread+\n",
	)
	for _, want := range []string{"get_semantic_graph_status", "read+"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("FormatHelp output missing %q; got:\n%s", want, rendered)
		}
	}
}

// TestGetToolHelp_GetSemanticContext_ReturnsParameterDocs asserts
// get_semantic_context surfaces its full param surface (task, files,
// symbols, max_tokens, freshness_mode).
func TestGetToolHelp_GetSemanticContext_ReturnsParameterDocs(t *testing.T) {
	docs := help.ExtractParamDocs(schemaFor[semantic.GetSemanticContextArgs](t))
	if len(docs) == 0 {
		t.Fatalf("ExtractParamDocs returned empty docs")
	}
	for _, want := range []string{"task", "files", "symbols", "max_tokens", "freshness_mode"} {
		hasParam(t, "get_semantic_context", docs, want)
	}
}
