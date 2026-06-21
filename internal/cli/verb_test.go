package cli

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// findSubcommand returns the named subcommand of the verb spine, failing the
// test if it is not registered.
func findSubcommand(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range root.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("subcommand %q not found under verb spine", name)
	return nil
}

// Test 1: flag→args mapping (no daemon). Parsing typed flags produces the
// expected Arguments map with the correct Go value types, asserted WITHOUT
// dialing a daemon (buildVerbArgs is isolated from the network call).
func TestVerb_FlagToArgsMapping(t *testing.T) {
	cmd := newVerbCommand()
	// Locate the representative subcommand spine carries.
	sub := findSubcommand(t, cmd, representativeVerb)

	if err := sub.Flags().Set("query", "needle"); err != nil {
		t.Fatalf("set query: %v", err)
	}
	if err := sub.Flags().Set("max-results", "7"); err != nil {
		t.Fatalf("set max-results: %v", err)
	}
	if err := sub.Flags().Set("context-lines", "2"); err != nil {
		t.Fatalf("set context-lines: %v", err)
	}

	args, err := buildVerbArgs(sub, verbSpecs[representativeVerb])
	if err != nil {
		t.Fatalf("buildVerbArgs: %v", err)
	}

	// Flags map to the underlying tool's schema keys (toolArg indirection):
	// --query -> pattern, --max-results -> max_results,
	// --context-lines -> context_lines.
	if got, ok := args["pattern"].(string); !ok || got != "needle" {
		t.Fatalf("pattern arg = %v (%T), want string needle", args["pattern"], args["pattern"])
	}
	if got, ok := args["max_results"].(int); !ok || got != 7 {
		t.Fatalf("max_results arg = %v (%T), want int 7", args["max_results"], args["max_results"])
	}
	if got, ok := args["context_lines"].(int); !ok || got != 2 {
		t.Fatalf("context_lines arg = %v (%T), want int 2", args["context_lines"], args["context_lines"])
	}
}

// Test 2: tool-name resolution. The representative verb name maps to the
// intended underlying tool Name passed in CallToolParams.
func TestVerb_ToolNameResolution(t *testing.T) {
	spec, ok := verbSpecs[representativeVerb]
	if !ok {
		t.Fatalf("representative verb %q not registered", representativeVerb)
	}
	if spec.toolName == "" {
		t.Fatalf("representative verb %q has empty toolName", representativeVerb)
	}

	// End-to-end through the seam: invoking the command must pass spec.toolName
	// as the tool Name to the one-shot helper.
	var gotName string
	var gotArgs map[string]any
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, name string, args map[string]any) (*mcpsdk.CallToolResult, error) {
		gotName = name
		gotArgs = args
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}}}, nil
	}
	defer func() { callToolFn = restore }()

	cmd := newVerbCommand()
	cmd.SetArgs([]string{representativeVerb, "--query=x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotName != spec.toolName {
		t.Fatalf("tool Name = %q, want %q", gotName, spec.toolName)
	}
	// --query maps to the tool's `pattern` argument (toolArg indirection).
	if gotArgs["pattern"] != "x" {
		t.Fatalf("pattern arg not forwarded: %v", gotArgs["pattern"])
	}
}

// Test 3: missing-required arg errors BEFORE dialing. A required flag left
// unset returns an error before any ConnectOrStartDaemon / callToolFn call.
func TestVerb_MissingRequiredArgErrorsBeforeDial(t *testing.T) {
	dialed := false
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, _ string, _ map[string]any) (*mcpsdk.CallToolResult, error) {
		dialed = true
		return nil, errors.New("should not be reached")
	}
	defer func() { callToolFn = restore }()

	cmd := newVerbCommand()
	// Omit the required --query flag (pass only an optional flag).
	cmd.SetArgs([]string{representativeVerb, "--max-results=5"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected an error for missing required flag, got nil")
	}
	if dialed {
		t.Fatalf("callToolFn was reached despite a missing required flag — validation must run before dialing")
	}
}
