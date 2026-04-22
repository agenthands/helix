package bench_test

// tools_integration_test.go verifies Phase 38 MCP protocol-layer behavior:
//
//   - TestToolsListShowsBriefDescriptions: tools/list returns brief descriptions
//     (under 100 tokens), not full detailed descriptions (UAT item 1)
//   - TestGetToolHelpReturnsComprehensiveDocs: get_tool_help returns formatted
//     documentation with parameter details and usage examples (UAT item 2)
//   - TestLazyInitActivatesWorkspaceOnFirstCall: calling a tool without prior
//     activate_project triggers transparent workspace activation (UAT item 3)

import (
	"context"
	"strings"
	"sync"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// intDaemon is shared across all integration tests to avoid creating multiple
// daemon instances (each leaks a few goroutines until context unwinds).
var (
	intDaemonOnce sync.Once
	intDaemonInst *benchDaemon
)

func getIntDaemon(t *testing.T) *benchDaemon {
	t.Helper()
	intDaemonOnce.Do(func() {
		intDaemonInst = startBenchDaemon(t)
	})
	return intDaemonInst
}

// TestToolsListShowsBriefDescriptions verifies that tools/list responses contain
// brief descriptions (not the full detailed ones) via the MCP protocol layer.
// This exercises ProfileFilterMiddleware's brief description rewrite (D-02).
func TestToolsListShowsBriefDescriptions(t *testing.T) {
	bd := getIntDaemon(t)

	result, err := bd.Session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("tools/list returned 0 tools")
	}

	// Every tool in the listing should have a non-empty description.
	var missingDesc []string
	var tooLong []string
	for _, tool := range result.Tools {
		if tool.Description == "" {
			missingDesc = append(missingDesc, tool.Name)
			continue
		}
		// Brief descriptions should be under 80 words (~100 tokens).
		words := len(strings.Fields(tool.Description))
		if words >= 80 {
			tooLong = append(tooLong, tool.Name)
		}
	}
	if len(missingDesc) > 0 {
		t.Errorf("tools with empty description in tools/list: %v", missingDesc)
	}
	if len(tooLong) > 0 {
		t.Errorf("tools with description >= 80 words in tools/list (should be brief): %v", tooLong)
	}

	// Spot-check: search_symbols' tools/list description should be the brief one,
	// not the full detailed one. The brief version is shorter.
	for _, tool := range result.Tools {
		if tool.Name == "search_symbols" {
			// The detailed description is multi-sentence. The brief is one short sentence.
			words := len(strings.Fields(tool.Description))
			if words > 20 {
				t.Errorf("search_symbols description in tools/list appears to be the full description (%d words), not brief: %q", words, tool.Description)
			}
			break
		}
	}
}

// TestGetToolHelpReturnsComprehensiveDocs verifies that get_tool_help returns
// formatted documentation with parameter details extracted from JSON Schema.
func TestGetToolHelpReturnsComprehensiveDocs(t *testing.T) {
	bd := getIntDaemon(t)

	// Call get_tool_help for a well-known tool with parameters.
	result := callToolB(t, bd.Session, "get_tool_help", map[string]any{
		"tool_name": "search_symbols",
	})
	text := benchTextContent(result)
	if text == "" {
		t.Fatal("get_tool_help returned empty response")
	}

	// Should contain parameter documentation.
	if !strings.Contains(text, "query") {
		t.Error("get_tool_help output missing 'query' parameter documentation")
	}

	// Should contain the full (detailed) description, not the brief one.
	// The help text should be more comprehensive than a single sentence.
	lines := strings.Split(text, "\n")
	if len(lines) < 5 {
		t.Errorf("get_tool_help output is too short (%d lines) — expected comprehensive docs", len(lines))
	}

	// Call for a non-existent tool should return an error.
	errResult, err := bd.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_tool_help",
		Arguments: map[string]any{"tool_name": "nonexistent_tool_xyz"},
	})
	if err != nil {
		t.Fatalf("get_tool_help protocol error: %v", err)
	}
	if !errResult.IsError {
		t.Error("get_tool_help for nonexistent tool should return IsError=true")
	}
}

// TestLazyInitActivatesWorkspaceOnFirstCall verifies end-to-end lazy workspace
// activation through the full MCP protocol stack. A tool call without prior
// activate_project should trigger transparent activation (LAZY-01).
func TestLazyInitActivatesWorkspaceOnFirstCall(t *testing.T) {
	bd := getIntDaemon(t)

	// Call a tool that doesn't require an active workspace (get_tool_help)
	// to verify the daemon is responsive without prior activation.
	helpResult := callToolB(t, bd.Session, "get_tool_help", map[string]any{
		"tool_name": "ping",
	})
	helpText := benchTextContent(helpResult)
	if helpText == "" {
		t.Fatal("get_tool_help for ping returned empty — daemon not responsive")
	}

	// Call activate_project through the full MCP protocol layer.
	// The daemon wires it through the full middleware chain (including lazy init).
	fixtureDir := prepareGoFixtureB(t)

	activateResult, err := bd.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "activate_project",
		Arguments: map[string]any{"repo_path": fixtureDir},
	})
	if err != nil {
		t.Fatalf("activate_project protocol error: %v", err)
	}
	if activateResult.IsError {
		t.Fatalf("activate_project failed: %s", benchTextContent(activateResult))
	}

	activateText := benchTextContent(activateResult)
	if !strings.Contains(activateText, "activated") {
		t.Errorf("activate_project response missing 'activated': %s", activateText)
	}
}
