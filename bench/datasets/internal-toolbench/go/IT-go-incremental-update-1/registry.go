// Package incrementalupdate is the Phase 78 ToolBench Go store-ON fixture
// (internal-toolbench/go/IT-go-incremental-update-1, capability=incremental_update).
//
// This is the ONE store-ON cell in the corpus: its task.json capability flips
// StoreOptIn=true (semantic_index.enabled=true + WithWorkingDir per-cell, D-01/
// D-02/D-03), so the daemon opens a per-cell .helix/semantic.duckdb store and the
// Phase 70 live overlay is active.
//
// The fixture models the incremental_update workflow: an agent edits a source
// symbol, then calls refresh_semantic_graph{wait_for_lsp:true} to drive the
// overlay-drain so the live semantic graph reflects the edit, then issues a
// semantic query whose result must reflect the post-edit state.
//
// Grading is carried by go test (GoRunner.RunTests). The scenario:
//
//   - registeredCapability() DELIBERATELY returns the WRONG token ("legacy")
//     pre-edit. The scripted agent's replace_in_file flips it to "incremental".
//   - Lookup(capabilityToken) resolves a handler from the registry keyed on that
//     token. Pre-edit the token "legacy" is not a registered key, so Lookup
//     returns the zeroHandler (Apply -> 0) and registry_test.go FAILS.
//   - Post-edit registeredCapability() returns "incremental", Lookup resolves the
//     real incrementalHandler (Apply -> input+1), and the test PASSES.
//
// The refresh_semantic_graph call is the capability's named tool (D-05): after
// the edit, the agent drives the overlay-drain so a follow-up semantic query
// (get_semantic_context anchored on the edited symbol) reflects the new wiring.
package incrementalupdate

// Handler applies an incremental update to a counter value.
type Handler interface {
	// Apply returns the next value for n. The zero handler returns 0 (no-op);
	// the incremental handler returns n+1.
	Apply(n int) int
}

// zeroHandler is the registry's default for an unknown capability token: it
// applies no update (returns 0), which is what makes the pre-edit state fail.
type zeroHandler struct{}

func (zeroHandler) Apply(int) int { return 0 }

// incrementalHandler applies a +1 increment — the correct behavior the test
// pins once the capability token is wired to it.
type incrementalHandler struct{}

func (incrementalHandler) Apply(n int) int { return n + 1 }

// registry maps capability tokens to their handlers. Only "incremental" is
// registered; any other token (e.g. the pre-edit "legacy") falls through to the
// zero handler.
var registry = map[string]Handler{
	"incremental": incrementalHandler{},
}

// registeredCapability returns the capability token this module registers its
// behavior under.
//
// DELIBERATELY WRONG pre-edit: it returns "legacy", which is NOT a key in
// registry, so Lookup falls back to the zero handler and registry_test.go fails.
// The scripted agent's replace_in_file edit flips "legacy" to "incremental";
// after refresh_semantic_graph drives the overlay-drain, the semantic graph and
// the compiled behavior both reflect the "incremental" wiring and the test passes.
func registeredCapability() string {
	return "legacy"
}

// Lookup resolves the Handler for token, falling back to the zero handler when
// the token is not a registered capability.
func Lookup(token string) Handler {
	if h, ok := registry[token]; ok {
		return h
	}
	return zeroHandler{}
}

// Update applies the registered capability's handler to n. It is the public
// entry point the test exercises: it resolves the handler via the token returned
// by registeredCapability(), so it only increments once that token is wired to
// the incremental handler.
func Update(n int) int {
	return Lookup(registeredCapability()).Apply(n)
}
