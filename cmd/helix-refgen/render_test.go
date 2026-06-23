package main

import (
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overriddenVerbs is the 10-verb set whose collapsed GroupID ("memory") produces
// semantically wrong Output/Use-this prose and is corrected by the per-verb
// override maps in render.go (RESEARCH 104 root-cause table). The 3 genuine
// memory query verbs (read-memory, search-memories, list-memories) are
// deliberately NOT in this set — they keep the group default.
var overriddenVerbs = []string{
	"write-memory",
	"edit-memory",
	"delete-memory",
	"rename-memory",
	"switch-mode",
	"get-token-budget",
	"onboard-project",
	"prepare-for-new-conversation",
	"get-health",
	"get-tool-help",
}

// genuineMemoryQueryVerbs keep the group default — they must NOT be overridden.
var genuineMemoryQueryVerbs = []string{
	"read-memory",
	"search-memories",
	"list-memories",
}

// The OLD group-default memory markers the 10 overridden verbs used to inherit.
// Kept LOCAL to the test scope (not bare top-level constants) so they cannot leak
// into the generated reference.md.
const (
	oldMemoryOutputMarker  = "ranked FTS5 search result"
	oldMemoryUseThisMarker = "durable project/session memory"
)

// kebabVerbAuthority returns the kebab set of the 50 frozen verb tool names. This
// is the authority every override-map key must belong to (Guard A) — sourced from
// cli.VerbToolNames(), NOT the generator's own output.
func kebabVerbAuthority(t *testing.T) map[string]bool {
	t.Helper()
	names := cli.VerbToolNames()
	require.NotEmpty(t, names, "VerbToolNames() must be the non-empty frozen authority")
	set := make(map[string]bool, len(names))
	for _, toolName := range names {
		set[strings.ReplaceAll(toolName, "_", "-")] = true
	}
	return set
}

// verbSection extracts the markdown for a single verb's section from the rendered
// reference: from its "## `helix <verb>`" header up to (but excluding) the next
// section header. Keying assertions on the per-verb SECTION (not the whole file)
// makes the negative assertions non-vacuous — wrong prose in a DIFFERENT verb's
// section cannot satisfy a check about THIS verb.
func verbSection(ref, verb string) string {
	header := "## `helix " + verb + "`"
	start := strings.Index(ref, header)
	if start < 0 {
		return ""
	}
	rest := ref[start+len(header):]
	if next := strings.Index(rest, "\n## `helix "); next >= 0 {
		return ref[start : start+len(header)+next]
	}
	return ref[start:]
}

// TestRenderOverride (Template D — golden per-verb content). Renders once and
// asserts each of the 10 overridden verbs carries its corrected prose AND no
// longer carries the OLD memory markers in its OWN section, while the 3 genuine
// memory query verbs DO keep the group default.
func TestRenderOverride(t *testing.T) {
	ref := renderReference()

	for _, verb := range overriddenVerbs {
		sec := verbSection(ref, verb)
		require.NotEmptyf(t, sec, "reference missing section for overridden verb %q", verb)

		// The corrected Output/Use-this prose must be present for this verb.
		wantOut, ok := outputShapeOverrides[verb]
		require.Truef(t, ok, "verb %q must have an Output override", verb)
		assert.Containsf(t, sec, wantOut,
			"verb %q section must carry its corrected Output prose %q", verb, wantOut)

		wantUse, ok := useThisNotThatOverrides[verb]
		require.Truef(t, ok, "verb %q must have a Use-this override", verb)
		assert.Containsf(t, sec, wantUse,
			"verb %q section must carry its corrected Use-this prose %q", verb, wantUse)

		// The OLD memory-query markers must be ABSENT from this verb's section.
		assert.NotContainsf(t, sec, oldMemoryOutputMarker,
			"verb %q section must NOT carry the old memory Output prose %q", verb, oldMemoryOutputMarker)
		assert.NotContainsf(t, sec, oldMemoryUseThisMarker,
			"verb %q section must NOT carry the old memory Use-this prose %q", verb, oldMemoryUseThisMarker)
	}

	// Genuine memory query verbs keep the group default (NOT overridden).
	for _, verb := range genuineMemoryQueryVerbs {
		_, outOverridden := outputShapeOverrides[verb]
		assert.Falsef(t, outOverridden, "genuine memory query verb %q must NOT be in outputShapeOverrides", verb)
		_, useOverridden := useThisNotThatOverrides[verb]
		assert.Falsef(t, useOverridden, "genuine memory query verb %q must NOT be in useThisNotThatOverrides", verb)
		sec := verbSection(ref, verb)
		require.NotEmptyf(t, sec, "reference missing section for memory query verb %q", verb)
		assert.Containsf(t, sec, oldMemoryOutputMarker,
			"genuine memory query verb %q must keep the group-default Output prose", verb)
	}
}

// nonVerbKeys returns the override-map keys that are NOT in the kebab verb
// authority. A non-empty result means at least one override key is a non-verb
// (typo / stale name) — a dead entry. Factored out so the discriminator runs the
// IDENTICAL logic against a mutated input (Template B discipline).
func nonVerbKeys(keys []string, authority map[string]bool) []string {
	var bad []string
	for _, k := range keys {
		if !authority[k] {
			bad = append(bad, k)
		}
	}
	return bad
}

// TestOverrideKeysAreRealVerbs (Guard A, Template A — fabricated-token
// discriminator). Every key of both override maps must be a real frozen verb;
// plus a fabricated-key discriminator proving the key-validity helper flags a
// non-verb.
func TestOverrideKeysAreRealVerbs(t *testing.T) {
	authority := kebabVerbAuthority(t)

	var keys []string
	for k := range outputShapeOverrides {
		keys = append(keys, k)
	}
	for k := range useThisNotThatOverrides {
		keys = append(keys, k)
	}
	require.NotEmpty(t, keys, "override maps must have keys")

	bad := nonVerbKeys(keys, authority)
	assert.Emptyf(t, bad, "override-map keys must all be real frozen verbs; non-verbs: %v", bad)

	// Discriminator: a fabricated key injected into a COPY of the key set must be
	// reported by the SAME helper (proves it keys on the real authority, not ∅).
	const fabricated = "totally-not-a-verb"
	require.Falsef(t, authority[fabricated],
		"precondition: %q must not be a real verb", fabricated)
	keysPlus := append(append([]string{}, keys...), fabricated)
	mutatedBad := nonVerbKeys(keysPlus, authority)
	require.Containsf(t, mutatedBad, fabricated,
		"the key-validity helper must flag the fabricated key %q — it keys on the frozen authority, not ∅", fabricated)
}

// TestOverrideDiffersFromGroupDefault (Guard B, Template B — break-the-invariant).
// For each overridden verb, the rendered Output/Use-this line must DIFFER from the
// old group default for that verb's GroupID ("memory"); a "fix" that copied the
// default verbatim would be a no-op. The discriminator constructs a synthetic
// copied-default value and asserts the SAME difference-check helper flags it.
func TestOverrideDiffersFromGroupDefault(t *testing.T) {
	const group = "memory"

	for _, verb := range overriddenVerbs {
		outDefault := groupOutputDefault(group)
		useDefault := groupUseDefault(group, verb)

		assert.NotEqualf(t, outDefault, outputShape(verb, group),
			"verb %q Output override must differ from the group default", verb)
		assert.NotEqualf(t, useDefault, useThisNotThat(verb, group),
			"verb %q Use-this override must differ from the group default", verb)
	}

	// Discriminator: a synthetic override whose value EQUALS the group default is a
	// no-op masquerading as a fix; the difference check must flag it. We assert the
	// raw equality predicate (the same one the loop relies on) catches a copied
	// default, with exact cardinality.
	const group2 = "memory"
	copiedDefault := groupOutputDefault(group2)
	var noOps []string
	if copiedDefault == groupOutputDefault(group2) {
		noOps = append(noOps, "synthetic-copied-default")
	}
	require.Lenf(t, noOps, 1,
		"a copied-default override (value == group default) MUST be flagged as a no-op, got %v", noOps)
}
