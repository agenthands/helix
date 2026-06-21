package cli

import (
	"context"
	"log/slog"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	// Blank imports trigger skill.Register() via init() — the SAME set the
	// generator (cmd/helix-cligen) and the daemon enumerate. The parity test
	// counts the live registry BY NAME against the generated verbSpecs; if this
	// set drifts from the generator's, the count assertion fails.
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/health"
	_ "github.com/agenthands/helix/internal/kernel/help"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/repomap"
	_ "github.com/agenthands/helix/internal/skill/semantic"
	_ "github.com/agenthands/helix/internal/skill/workflow"

	"github.com/agenthands/helix/internal/skill"
)

// liveRegistryNameSet builds the unique live-registry tool name set by ranging
// skill.ToolProviders() exactly as the generator does. Duplicate names (e.g.
// analyze_blast_radius listed by two providers) collapse to one entry.
func liveRegistryNameSet() map[string]bool {
	set := make(map[string]bool)
	for _, tp := range skill.ToolProviders() {
		for _, t := range tp.Tools() {
			set[t.Name] = true
		}
	}
	return set
}

// verbSpecToolNameSet returns the set of underlying tool names in the generated
// verbSpecs catalog (one per verb).
func verbSpecToolNameSet() map[string]bool {
	set := make(map[string]bool)
	for _, spec := range verbSpecs {
		set[spec.toolName] = true
	}
	return set
}

// TestVerbsParity_CountByName is the VERB-01 contract: the generated verb count
// equals the live-registry tool count, enumerated BY NAME (never hardcoded 53).
func TestVerbsParity_CountByName(t *testing.T) {
	live := liveRegistryNameSet()
	gen := verbSpecToolNameSet()

	if len(gen) != len(live) {
		t.Errorf("verb count mismatch: len(verbSpecs)=%d, live registry=%d", len(gen), len(live))
	}

	// Set-equality with an explicit diff for actionable failures.
	for name := range live {
		if !gen[name] {
			t.Errorf("live tool %q has no generated verb (regenerate: go run ./cmd/helix-cligen)", name)
		}
	}
	for name := range gen {
		if !live[name] {
			t.Errorf("generated verb tool %q is not in the live registry (stale verbs_gen.go)", name)
		}
	}
}

// TestVerbToolNames_SortedSetEqual is the VerbToolNames() accessor contract: it
// returns a sorted, duplicate-free slice whose element set equals the
// verbSpec.toolName values (the seam 91-03 consumes).
func TestVerbToolNames_SortedSetEqual(t *testing.T) {
	got := VerbToolNames()

	// Sorted ascending.
	if !sort.StringsAreSorted(got) {
		t.Errorf("VerbToolNames() is not sorted ascending: %v", got)
	}

	// No duplicates.
	seen := make(map[string]bool)
	for _, n := range got {
		if seen[n] {
			t.Errorf("VerbToolNames() contains duplicate %q", n)
		}
		seen[n] = true
	}

	// Set-equal to verbSpecs toolNames.
	want := verbSpecToolNameSet()
	if len(seen) != len(want) {
		t.Errorf("VerbToolNames() len=%d, verbSpecs toolNames=%d", len(seen), len(want))
	}
	for n := range want {
		if !seen[n] {
			t.Errorf("VerbToolNames() missing %q", n)
		}
	}
}

// TestVerbToolNames_ReadOnly asserts mutating the returned slice does not affect
// internal state (a fresh copy is returned each call).
func TestVerbToolNames_ReadOnly(t *testing.T) {
	a := VerbToolNames()
	if len(a) == 0 {
		t.Fatalf("VerbToolNames() returned empty")
	}
	orig := a[0]
	a[0] = "MUTATED"
	b := VerbToolNames()
	if b[0] != orig {
		t.Errorf("mutating returned slice leaked into internal state: b[0]=%q want %q", b[0], orig)
	}
}

// TestVerbs_NoEmptyFlags is the VERB-04 no-empty contract: every verb has >= 1
// flag OR is on the documented zero-arg allowlist (tools registered via the
// dynamic AddSkillTool map[string]any handler, plus genuine no-arg tools).
func TestVerbs_NoEmptyFlags(t *testing.T) {
	// Documented zero-arg / dynamic-handler allowlist. These tools register via
	// AddSkillTool (map[string]any) or are catalog-only (profile), so no *Args
	// struct exists for the AST scan to derive flags from — empty flags are
	// expected, not a scan miss. (Plus get_semantic_graph_status, a genuine
	// no-arg typed tool.)
	allow := map[string]bool{
		"write_memory":                 true,
		"read_memory":                  true,
		"list_memories":                true,
		"search_memories":              true,
		"rename_memory":                true,
		"edit_memory":                  true,
		"delete_memory":                true,
		"get_repo_map":                 true,
		"get_context":                  true,
		"onboard_project":              true,
		"prepare_for_new_conversation": true,
		"switch_mode":                  true,
		"get_token_budget":             true,
		"get_semantic_graph_status":    true,
	}
	for verb, spec := range verbSpecs {
		if len(spec.flags) == 0 && !allow[spec.toolName] {
			t.Errorf("verb %q (tool %q) has zero flags and is not on the allowlist — likely a scan miss", verb, spec.toolName)
		}
	}
}

// TestVerbs_RequiredBeforeDial is the VERB-03 pre-dial contract: invoking a
// generated verb with a required flag MISSING returns the required-flag error
// WITHOUT reaching the network seam.
func TestVerbs_RequiredBeforeDial(t *testing.T) {
	dialed := false
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, _ string, _ map[string]any) (*mcpsdk.CallToolResult, error) {
		dialed = true
		return nil, nil
	}
	defer func() { callToolFn = restore }()

	cmd := NewRootCommand()
	// replace-symbol-body requires --path/--symbol-name/--new-body; omit all.
	cmd.SetArgs([]string{"replace-symbol-body"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected required-flag error, got nil")
	}
	if dialed {
		t.Fatalf("callToolFn reached despite a missing required flag — validation must run before dialing")
	}
}

// TestVerbs_FlattenedOnRoot is the VERB-03 flatten contract: a generated verb is
// a DIRECT child of the root command under its capability group (not under a
// `call` parent).
func TestVerbs_FlattenedOnRoot(t *testing.T) {
	root := NewRootCommand()
	var found *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "go-to-definition" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatalf("go-to-definition is not a direct child of root (flatten failed)")
	}
	if found.GroupID != groupNavigation {
		t.Errorf("go-to-definition GroupID = %q, want %q", found.GroupID, groupNavigation)
	}
	// And there must be NO `call` parent command.
	for _, c := range root.Commands() {
		if c.Name() == "call" {
			t.Errorf("legacy `call` parent still present; verbs must flatten onto root")
		}
	}
}
