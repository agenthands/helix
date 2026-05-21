// Phase 64 P64-08 Task 2: profile-filter coverage for the four semantic
// MCP tools.
//
// Drives the same resolution path as InstallMiddleware /
// ProfileFilterMiddleware uses — skill.ResolveTools(skillNames, includes,
// excludes) — for every (profile, mode) combination in the embedded
// matrix. Asserts:
//
//   - All 5 profiles surface the four semantic tools when in `review` mode
//     (D-14: symmetric matrix).
//   - `read` mode excludes index_semantic_graph (review+/admin tier per
//     SPEC §30.2) for every profile.
//   - `edit` mode also excludes index_semantic_graph (mode is review+).
//   - Full-profile + every mode visibility table is correct end-to-end.
//
// Closes CONTEXT.md acceptance test #5 + #7 (TOOL-05).

package semantic

import (
	"sort"
	"testing"

	"github.com/agenthands/helix/internal/profile"
	"github.com/agenthands/helix/internal/skill"
)

// semanticToolNames is the four-tool surface this skill exposes via Tools().
var semanticToolNames = []string{
	"index_semantic_graph",
	"refresh_semantic_graph",
	"get_semantic_graph_status",
	"get_semantic_context",
}

// readPlusOnlyTools is the subset visible in read+ tier sessions (read +
// edit modes): index_semantic_graph is review+/admin per SPEC §30.2 and
// is excluded by mode YAML — see internal/profile/modes/{read,edit}.yaml.
var readPlusOnlyTools = []string{
	"refresh_semantic_graph",
	"get_semantic_graph_status",
	"get_semantic_context",
}

// resolveForProfileMode mirrors daemon.go's resolveAllowedToolsForMode
// helper without dragging the daemon into the import graph. The merge order
// (skills ∪ skills, tools ∪ tools, exclude_tools ∪ exclude_tools) and the
// final ResolveTools call are byte-for-byte identical so any drift between
// production and test surfaces is impossible.
func resolveForProfileMode(t *testing.T, store *profile.ProfileStore, profileName, modeName string) []string {
	t.Helper()

	prof, ok := store.Profile(profileName)
	if !ok {
		t.Fatalf("profile %q not found", profileName)
	}
	mode, ok := store.Mode(modeName)
	if !ok {
		t.Fatalf("mode %q not found", modeName)
	}

	skillSet := make(map[string]bool)
	for _, sk := range prof.Skills {
		skillSet[sk] = true
	}
	for _, sk := range mode.Skills {
		skillSet[sk] = true
	}
	skillNames := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skillNames = append(skillNames, sk)
	}

	includeTools := append([]string{}, prof.Tools...)
	includeTools = append(includeTools, mode.Tools...)

	excludeTools := append([]string{}, prof.ExcludeTools...)
	excludeTools = append(excludeTools, mode.ExcludeTools...)

	resolved := skill.ResolveTools(skillNames, includeTools, excludeTools)
	names := make([]string, len(resolved))
	for i, td := range resolved {
		names[i] = td.Name
	}
	sort.Strings(names)
	return names
}

// hasAll asserts every name in want appears in got.
func hasAll(t *testing.T, where string, got, want []string) {
	t.Helper()
	have := make(map[string]bool, len(got))
	for _, n := range got {
		have[n] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("%s: missing tool %q (got: %v)", where, w, got)
		}
	}
}

// hasNone asserts no name in unwanted appears in got.
func hasNone(t *testing.T, where string, got, unwanted []string) {
	t.Helper()
	have := make(map[string]bool, len(got))
	for _, n := range got {
		have[n] = true
	}
	for _, u := range unwanted {
		if have[u] {
			t.Errorf("%s: unwanted tool %q present (got: %v)", where, u, got)
		}
	}
}

// TestProfileFilter_FullProfile_AllModes_AllFourToolsVisible asserts the
// full profile surfaces the four tools in admin/review modes and the three
// read+ tools (no index) in read/edit modes.
func TestProfileFilter_FullProfile_AllModes_AllFourToolsVisible(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	cases := []struct {
		mode    string
		wantAll bool // when true, all 4 semantic tools must be visible
	}{
		{mode: "admin", wantAll: true},
		{mode: "review", wantAll: true},
		{mode: "edit", wantAll: false},
		{mode: "read", wantAll: false},
	}
	for _, tc := range cases {
		got := resolveForProfileMode(t, store, "full", tc.mode)
		if tc.wantAll {
			hasAll(t, "full/"+tc.mode, got, semanticToolNames)
		} else {
			hasAll(t, "full/"+tc.mode, got, readPlusOnlyTools)
			hasNone(t, "full/"+tc.mode, got, []string{"index_semantic_graph"})
		}
	}
}

// TestProfileFilter_AllProfiles_SemanticToolsListed asserts every profile
// surfaces all four semantic tools in `review` mode (D-14: symmetric matrix
// — every profile gets every semantic tool).
func TestProfileFilter_AllProfiles_SemanticToolsListed(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	for _, p := range []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"} {
		got := resolveForProfileMode(t, store, p, "review")
		hasAll(t, p+"/review", got, semanticToolNames)
	}
}

// TestProfileFilter_ReadMode_IndexExcluded asserts every profile in `read`
// mode emits exactly the three read+ semantic tools (no
// index_semantic_graph).
func TestProfileFilter_ReadMode_IndexExcluded(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	for _, p := range []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"} {
		got := resolveForProfileMode(t, store, p, "read")
		hasAll(t, p+"/read", got, readPlusOnlyTools)
		hasNone(t, p+"/read", got, []string{"index_semantic_graph"})
	}
}

// TestProfileFilter_EditMode_IndexExcluded asserts every profile in `edit`
// mode emits exactly the three read+ semantic tools (no
// index_semantic_graph). edit is also a read+ tier per SPEC §30.2.
func TestProfileFilter_EditMode_IndexExcluded(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	for _, p := range []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"} {
		got := resolveForProfileMode(t, store, p, "edit")
		hasAll(t, p+"/edit", got, readPlusOnlyTools)
		hasNone(t, p+"/edit", got, []string{"index_semantic_graph"})
	}
}

// ----- Phase 73-02: P1 tool golden tests (5×4 profile×mode matrix) -----

// p1ToolNames is the full set of 6 P1 MCP tools added in Phase 73-01.
// 5 are read+ (visible in all 4 modes); 1 is review+ (visible in review/admin
// only). Mirrors the semanticToolNames pattern above.
var p1ToolNames = []string{
	"explain_symbol_deep",
	"find_related_symbols",
	"validate_graph_edge",
	"get_cluster_map",
	"explain_cluster",
	"get_change_impact_graph",
}

// p1ReadPlusOnlyTools is the subset of P1 tools visible in ALL 4 modes (read+
// tier). These 5 tools must be present for every (profile, mode) cell.
var p1ReadPlusOnlyTools = []string{
	"explain_symbol_deep",
	"find_related_symbols",
	"validate_graph_edge",
	"get_cluster_map",
	"explain_cluster",
}

// p1ReviewPlusOnlyTools is the review+ P1 tool: visible in review and admin
// only, absent from read and edit modes per
// internal/profile/modes/{read,edit}.yaml exclude_tools.
var p1ReviewPlusOnlyTools = []string{
	"get_change_impact_graph",
}

// TestProfileFilter_P1ReadPlusTools_AllProfilesAllModes asserts the five
// read+ P1 tools are present for every (profile, mode) cell in the 5×4
// matrix. No mode or profile may suppress these tools.
func TestProfileFilter_P1ReadPlusTools_AllProfilesAllModes(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	profiles := []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"}
	modes := []string{"read", "edit", "review", "admin"}

	for _, p := range profiles {
		for _, m := range modes {
			p, m := p, m
			t.Run(p+"/"+m, func(t *testing.T) {
				got := resolveForProfileMode(t, store, p, m)
				hasAll(t, p+"/"+m, got, p1ReadPlusOnlyTools)
			})
		}
	}
}

// TestProfileFilter_P1ChangeImpact_ReviewPlusGating asserts that
// get_change_impact_graph is absent from read and edit modes (review+ tier)
// and present in review and admin modes — for every profile. Mirrors the
// existing TestProfileFilter_ReadMode_IndexExcluded pattern (lines 164-175).
func TestProfileFilter_P1ChangeImpact_ReviewPlusGating(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	profiles := []string{"full", "claude-code", "codex", "ide-assistant", "ci-bot"}

	for _, p := range profiles {
		p := p
		t.Run(p+"/read", func(t *testing.T) {
			got := resolveForProfileMode(t, store, p, "read")
			hasNone(t, p+"/read", got, p1ReviewPlusOnlyTools)
		})
		t.Run(p+"/edit", func(t *testing.T) {
			got := resolveForProfileMode(t, store, p, "edit")
			hasNone(t, p+"/edit", got, p1ReviewPlusOnlyTools)
		})
		t.Run(p+"/review", func(t *testing.T) {
			got := resolveForProfileMode(t, store, p, "review")
			hasAll(t, p+"/review", got, p1ReviewPlusOnlyTools)
		})
		t.Run(p+"/admin", func(t *testing.T) {
			got := resolveForProfileMode(t, store, p, "admin")
			hasAll(t, p+"/admin", got, p1ReviewPlusOnlyTools)
		})
	}
}
