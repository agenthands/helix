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
	callToolFn = func(_ context.Context, _ string, _ string, _ *slog.Logger, _ string, name string, args map[string]any) (*mcpsdk.CallToolResult, error) {
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
	callToolFn = func(_ context.Context, _ string, _ string, _ *slog.Logger, _ string, _ string, _ map[string]any) (*mcpsdk.CallToolResult, error) {
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

// TestVerbSpecsForDocs_CountMatchesAuthority asserts the doc accessor returns one
// entry per verb, sorted by verb key, with the same count as VerbToolNames()
// (the frozen 50-verb registry authority).
func TestVerbSpecsForDocs_CountMatchesAuthority(t *testing.T) {
	docs := VerbSpecsForDocs()
	names := VerbToolNames()
	if len(docs) != len(names) {
		t.Fatalf("VerbSpecsForDocs() len = %d, VerbToolNames() len = %d; must match (same authority)", len(docs), len(names))
	}
	if len(docs) != len(verbSpecs) {
		t.Fatalf("VerbSpecsForDocs() len = %d, len(verbSpecs) = %d; must be one entry per verb", len(docs), len(verbSpecs))
	}
	// Sorted by the kebab verb key.
	for i := 1; i < len(docs); i++ {
		if docs[i-1].Verb >= docs[i].Verb {
			t.Fatalf("VerbSpecsForDocs() not sorted by verb key at %d: %q >= %q", i, docs[i-1].Verb, docs[i].Verb)
		}
	}
}

// TestVerbSpecsForDocs_FieldsMirrorCatalog asserts each doc entry faithfully
// mirrors its underlying verbSpec (toolName, groupID, short, and the per-flag
// view with a stable lowercase kind token).
func TestVerbSpecsForDocs_FieldsMirrorCatalog(t *testing.T) {
	wantKind := map[flagKind]string{
		flagString:      "string",
		flagInt:         "int",
		flagBool:        "bool",
		flagStringSlice: "string-slice",
		flagJSON:        "json",
	}
	for _, d := range VerbSpecsForDocs() {
		spec, ok := verbSpecs[d.Verb]
		if !ok {
			t.Fatalf("doc entry %q has no matching verbSpec", d.Verb)
		}
		if d.ToolName != spec.toolName {
			t.Errorf("verb %q: ToolName = %q, want %q", d.Verb, d.ToolName, spec.toolName)
		}
		if d.GroupID != spec.groupID {
			t.Errorf("verb %q: GroupID = %q, want %q", d.Verb, d.GroupID, spec.groupID)
		}
		if d.Short != spec.short {
			t.Errorf("verb %q: Short = %q, want %q", d.Verb, d.Short, spec.short)
		}
		if len(d.Flags) != len(spec.flags) {
			t.Fatalf("verb %q: Flags len = %d, want %d", d.Verb, len(d.Flags), len(spec.flags))
		}
		for i, f := range spec.flags {
			df := d.Flags[i]
			if df.Name != f.name || df.ToolArg != f.toolArg || df.Required != f.required || df.Help != f.help {
				t.Errorf("verb %q flag %d: doc view %+v does not mirror %+v", d.Verb, i, df, f)
			}
			if df.Kind != wantKind[f.kind] {
				t.Errorf("verb %q flag %d: Kind = %q, want %q", d.Verb, i, df.Kind, wantKind[f.kind])
			}
		}
	}
}

// TestVerbSpecsForDocs_ReadOnly proves the accessor returns a fresh deep copy:
// mutating a returned flag slice (or its elements) does not affect verbSpecs on a
// subsequent read. Mirrors VerbToolNames's fresh-copy discipline.
func TestVerbSpecsForDocs_ReadOnly(t *testing.T) {
	// Pick a verb known to carry flags.
	const verb = "analyze-blast-radius"
	first := VerbSpecsForDocs()
	var target *VerbDoc
	for i := range first {
		if first[i].Verb == verb {
			target = &first[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("verb %q not present in VerbSpecsForDocs()", verb)
	}
	if len(target.Flags) == 0 {
		t.Fatalf("verb %q unexpectedly has no flags to mutate", verb)
	}
	origName := target.Flags[0].Name
	// Mutate the returned copy aggressively.
	target.Flags[0].Name = "MUTATED"
	target.Flags = target.Flags[:0]

	second := VerbSpecsForDocs()
	for _, d := range second {
		if d.Verb != verb {
			continue
		}
		if len(d.Flags) == 0 {
			t.Fatalf("mutation truncated the underlying flag slice for %q", verb)
		}
		if d.Flags[0].Name != origName {
			t.Errorf("mutation leaked into verbSpecs: flag[0].Name = %q, want %q", d.Flags[0].Name, origName)
		}
		return
	}
	t.Fatalf("verb %q missing on re-read", verb)
}
