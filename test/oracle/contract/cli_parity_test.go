// Package contract_test's typed-args -> cobra-flags parity oracle (TEST-02).
//
// This file is DELIBERATELY UNTAGGED (no //go:build constraint) so it runs in the
// DEFAULT `go test ./...` suite — it is the replacement for the old MCP
// inputSchema/outputSchema meta-validation (schema_test.go), which was tagged
// behind integration/llm and required a live MCP runner. The contract this file
// guards is the v2.0 CLI-first one: the GENERATED verb surface stays in lockstep
// with the LIVE tool registry, i.e. every live tool has a generated verb (the
// "typed args -> cobra flags" recovery cmd/helix-cligen performs is complete and
// non-stale). It does NOT start a daemon, dial a socket, or touch a language
// server — it reads two pure in-process catalogs and asserts set-equality.
//
// Why this replaces the schema meta-validation: in v2.0 the agent-facing surface
// is the CLI verbs, not the MCP inputSchema. The load-bearing contract is "the
// verb catalog == the tool registry" (so no tool is unreachable from the CLI and
// no verb is orphaned), which the by-NAME parity below enforces against the same
// live providers the daemon and the generator enumerate.

package contract_test

import (
	"sort"
	"testing"

	// Blank imports trigger skill.Register() via init() — the SAME provider set
	// the generator (cmd/helix-cligen) and the daemon enumerate. If this set
	// drifts from the generator's, the by-NAME parity assertion below fails.
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

	"github.com/agenthands/helix/internal/cli"
	"github.com/agenthands/helix/internal/skill"
)

// liveRegistryToolNames returns the sorted, deduped set of tool names the live
// skill registry exposes — the SAME enumeration the generator and daemon perform
// (skill.ToolProviders() ranged by Tools().Name). Duplicate names across
// providers collapse to one entry.
func liveRegistryToolNames() []string {
	set := make(map[string]bool)
	for _, tp := range skill.ToolProviders() {
		for _, tool := range tp.Tools() {
			set[tool.Name] = true
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TestCLIParity_VerbsMatchRegistry is the typed-args -> cobra-flags parity
// contract (TEST-02): the generated verb catalog (cli.VerbToolNames()) must be
// set-equal to the live tool registry. Every live tool has a generated verb (no
// CLI-unreachable tool) and every generated verb maps to a live tool (no orphan
// verb / stale verbs_gen.go). This is the default-suite replacement for the MCP
// schema meta-validation.
func TestCLIParity_VerbsMatchRegistry(t *testing.T) {
	verbTools := cli.VerbToolNames() // sorted set of underlying tool names per verb
	liveTools := liveRegistryToolNames()

	verbSet := make(map[string]bool, len(verbTools))
	for _, n := range verbTools {
		verbSet[n] = true
	}
	liveSet := make(map[string]bool, len(liveTools))
	for _, n := range liveTools {
		liveSet[n] = true
	}

	if len(verbSet) != len(liveSet) {
		t.Errorf("verb/registry count mismatch: VerbToolNames()=%d unique, live registry=%d unique",
			len(verbSet), len(liveSet))
	}

	// Every live tool must have a generated verb (else it is CLI-unreachable).
	for _, name := range liveTools {
		if !verbSet[name] {
			t.Errorf("live tool %q has NO generated verb (CLI-unreachable; regenerate: go run ./cmd/helix-cligen)", name)
		}
	}
	// Every generated verb must map to a live tool (else verbs_gen.go is stale).
	for _, name := range verbTools {
		if !liveSet[name] {
			t.Errorf("generated verb tool %q is NOT in the live registry (stale verbs_gen.go)", name)
		}
	}
}

// TestCLIParity_VerbToolNamesSorted is a thin guard on the read-only seam the
// parity above consumes: VerbToolNames() returns a sorted, duplicate-free slice.
// (The deeper accessor contract is owned by internal/cli's own verbs_gen_test;
// this asserts only the surface the out-of-package oracle depends on.)
func TestCLIParity_VerbToolNamesSorted(t *testing.T) {
	got := cli.VerbToolNames()
	if len(got) == 0 {
		t.Fatal("cli.VerbToolNames() returned empty — the verb catalog seam is broken")
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("cli.VerbToolNames() is not sorted ascending: %v", got)
	}
	seen := make(map[string]bool, len(got))
	for _, n := range got {
		if seen[n] {
			t.Errorf("cli.VerbToolNames() contains duplicate %q", n)
		}
		seen[n] = true
	}
}
