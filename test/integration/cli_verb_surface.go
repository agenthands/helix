//go:build integration

package integration_test

import (
	"sort"
	"testing"

	"github.com/agenthands/helix/internal/cli"
)

// cliVerbSurfaceTools returns the SORTED list of tool names the CLI verb surface
// exposes for the daemon's active profile session — the SEC-02 oracle re-pointed
// from the MCP tools/list surface to the CLI verb surface.
//
// It is computed as the INTERSECTION of:
//
//	(a) the generated verb catalog's tool names, read via the 91-01-exported
//	    read-only accessor cli.VerbToolNames() (this plan adds NO code to
//	    internal/cli; it only consumes the accessor across the package boundary),
//	    and
//	(b) the tools the profile session actually allows, read from the
//	    authoritative server-side view listSessionTools(tb, td.Session) (MCP
//	    tools/list still reflects the profile's AllowedTools via
//	    ProfileFilterMiddleware).
//
// Two invariants are asserted so the surface cannot silently drift:
//
//   - every profile-allowed tool has a corresponding generated verb (so the CLI
//     verb surface == the profile surface; no profile tool is unreachable from
//     the CLI), and
//   - the returned surface contains no tool outside the profile's allowed set
//     (the catalog is filtered down to exactly the profile's tools).
//
// Because every profile-allowed tool has exactly one verb (91-01 parity) and the
// profile's allowed set is a subset of the verb catalog, the intersection is
// byte-identical to the profile's allowed tool set — so the result compares
// cleanly against the existing per-profile golden file.
//
// Core protocol tools (activate_project, ping, echo) are NOT verbs and NOT in
// any profile golden; they are absent from cli.VerbToolNames() (the helix-cligen
// intersection drops them) and so are naturally excluded by the intersection.
func cliVerbSurfaceTools(tb testing.TB, td *TestDaemon) []string {
	tb.Helper()

	// (a) generated verb catalog tool names (sorted, fresh copy per accessor
	// contract). Build a membership set for the intersection.
	catalog := cli.VerbToolNames()
	inCatalog := make(map[string]bool, len(catalog))
	for _, name := range catalog {
		inCatalog[name] = true
	}

	// (b) the profile session's authoritative allowed set.
	allowed := listSessionTools(tb, td.Session)

	surface := make([]string, 0, len(allowed))
	for _, name := range allowed {
		// Core protocol tools (activate_project, ping, echo) are registered
		// outside the profile/skill system and are not profile-gated agent
		// verbs; they are absent from the verb catalog, so the catalog
		// membership check skips them without special-casing here.
		if !inCatalog[name] {
			// A profile-allowed tool with no corresponding verb means the CLI
			// surface cannot reach a tool the profile permits — a parity break.
			// Core tools are the only legitimate non-verb members; everything
			// else must have a verb.
			if alwaysAllowedCoreToolNames[name] {
				continue
			}
			tb.Fatalf("profile-allowed tool %q has no corresponding CLI verb "+
				"(cli.VerbToolNames() catalog drift vs profile surface)", name)
		}
		surface = append(surface, name)
	}

	sort.Strings(surface)
	return surface
}

// alwaysAllowedCoreToolNames mirrors internal/mcp.alwaysAllowedCoreTools — the
// protocol-substrate tools registered outside the profile/skill system that are
// never in any profile's AllowedTools whitelist nor in the verb catalog. Kept as
// a local set so cli_verb_surface.go does not depend on internal/mcp internals;
// it only documents the legitimate non-verb members of a profile surface.
var alwaysAllowedCoreToolNames = map[string]bool{
	"ping":             true,
	"echo":             true,
	"activate_project": true,
}
