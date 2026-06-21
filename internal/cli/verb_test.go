package cli

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// findRootVerb returns the named generated verb subcommand attached to the root
// command, failing the test if it is not registered.
func findRootVerb(t *testing.T, name string) (*cobra.Command, verbSpec) {
	t.Helper()
	root := NewRootCommand()
	for _, c := range root.Commands() {
		if c.Name() == name {
			return c, verbSpecs[name]
		}
	}
	t.Fatalf("generated verb %q not found on root command", name)
	return nil, verbSpec{}
}

// Test 1: flag→args mapping (no daemon). Parsing typed flags produces the
// expected Arguments map with the correct Go value types, asserted WITHOUT
// dialing a daemon (buildVerbArgs is isolated from the network call).
func TestVerb_FlagToArgsMapping(t *testing.T) {
	sub, spec := findRootVerb(t, "go-to-definition")

	if err := sub.Flags().Set("path", "src/main.go"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := sub.Flags().Set("line", "10"); err != nil {
		t.Fatalf("set line: %v", err)
	}
	if err := sub.Flags().Set("column", "5"); err != nil {
		t.Fatalf("set column: %v", err)
	}

	args, err := buildVerbArgs(sub, spec)
	if err != nil {
		t.Fatalf("buildVerbArgs: %v", err)
	}
	if got, ok := args["path"].(string); !ok || got != "src/main.go" {
		t.Fatalf("path arg = %v (%T), want string src/main.go", args["path"], args["path"])
	}
	if got, ok := args["line"].(int); !ok || got != 10 {
		t.Fatalf("line arg = %v (%T), want int 10", args["line"], args["line"])
	}
	if got, ok := args["column"].(int); !ok || got != 5 {
		t.Fatalf("column arg = %v (%T), want int 5", args["column"], args["column"])
	}
}

// Test 2: tool-name resolution. The verb name maps to the intended underlying
// tool Name passed in CallToolParams.
func TestVerb_ToolNameResolution(t *testing.T) {
	spec, ok := verbSpecs["go-to-definition"]
	if !ok {
		t.Fatalf("verb go-to-definition not registered")
	}
	if spec.toolName != "go_to_definition" {
		t.Fatalf("toolName = %q, want go_to_definition", spec.toolName)
	}

	var gotName string
	var gotArgs map[string]any
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, name string, args map[string]any) (*mcpsdk.CallToolResult, error) {
		gotName = name
		gotArgs = args
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}}}, nil
	}
	defer func() { callToolFn = restore }()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"go-to-definition", "--path=x.go", "--line=1", "--column=1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotName != spec.toolName {
		t.Fatalf("tool Name = %q, want %q", gotName, spec.toolName)
	}
	if gotArgs["path"] != "x.go" {
		t.Fatalf("path arg not forwarded: %v", gotArgs["path"])
	}
}

// Test 3: missing-required arg errors BEFORE dialing. A required flag left unset
// returns an error before any ConnectOrStartDaemon / callToolFn call.
func TestVerb_MissingRequiredArgErrorsBeforeDial(t *testing.T) {
	dialed := false
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, _ string, _ map[string]any) (*mcpsdk.CallToolResult, error) {
		dialed = true
		return nil, errors.New("should not be reached")
	}
	defer func() { callToolFn = restore }()

	cmd := NewRootCommand()
	// Omit the required --path flag.
	cmd.SetArgs([]string{"go-to-definition", "--line=1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected an error for missing required flag, got nil")
	}
	if dialed {
		t.Fatalf("callToolFn was reached despite a missing required flag — validation must run before dialing")
	}
}
