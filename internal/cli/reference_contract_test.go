package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readEmbeddedReference returns the embedded skills/helix/reference.md bytes (the
// on-demand per-verb reference tier shipped in the skill bundle). It reads from
// the same embed.FS installSkill ships to disk, so the contract gates the
// artifact the agent actually receives.
func readEmbeddedReference(t *testing.T) string {
	t.Helper()
	b, err := embeddedSkillFS.ReadFile("skills/helix/reference.md")
	require.NoError(t, err, "embedded skills/helix/reference.md must exist (Plan 01 artifact)")
	return string(b)
}

// referenceSectionHeader is the exact markdown section header the generator emits
// for a verb: "## `helix <kebab-verb>`". Keying the completeness gate on the
// per-verb SECTION (not a bare substring) makes it non-vacuous: a verb mentioned
// only in another verb's "use this, not that" prose does NOT count as covered, and
// deleting a verb's own section turns the gate RED even if its kebab token survives
// elsewhere in the file.
func referenceSectionHeader(kebabVerb string) string {
	return "## `helix " + kebabVerb + "`"
}

// referenceMissingVerbs is the completeness check, factored out so the revert-and-
// fail sibling can run the IDENTICAL logic against a mutated copy. authority is the
// 50 frozen tool names (VerbToolNames()) — the single source of truth. It returns
// the kebab verbs whose section header is absent from refBytes. A non-empty result
// means the reference does not cover every frozen verb.
func referenceMissingVerbs(refBytes string, authority []string) []string {
	var missing []string
	for _, toolName := range authority {
		verb := strings.ReplaceAll(toolName, "_", "-")
		if !strings.Contains(refBytes, referenceSectionHeader(verb)) {
			missing = append(missing, verb)
		}
	}
	return missing
}

// TestReferenceCoversEveryVerb is the ADOPT-01a completeness gate. The authority is
// cli.VerbToolNames() (the 51 frozen verbs) — NOT refgen's own output (97-RESEARCH
// Pitfall 2: never source completeness from the generator that produced the file,
// that is a set-compared-to-itself tautology that can never go RED). Every frozen
// verb must have its own section in the embedded reference.md.
func TestReferenceCoversEveryVerb(t *testing.T) {
	authority := VerbToolNames()

	// Anchor the authority size: a future verb addition without a reference regen
	// must trip this gate (and REF-03's --check). 51 frozen verbs.
	require.Equal(t, 51, len(authority),
		"VerbToolNames() is the completeness authority; expected the 51 frozen verbs, got %d", len(authority))

	ref := readEmbeddedReference(t)
	missing := referenceMissingVerbs(ref, authority)
	assert.Emptyf(t, missing,
		"reference.md is missing a section for these frozen verbs: %v — regenerate with `go run ./cmd/helix-refgen`",
		missing)

	// Per-verb assertion (never an empty-iteration pass): every verb individually
	// present, with a message naming the offender.
	for _, toolName := range authority {
		verb := strings.ReplaceAll(toolName, "_", "-")
		assert.Containsf(t, ref, referenceSectionHeader(verb),
			"reference.md missing section for verb %q", verb)
	}
}

// TestReferenceContractDiscriminatesAbsentVerb is the explicit anti-vacuity
// discriminator (BUNDLE-02, SC-3): it proves referenceMissingVerbs keys on the
// real frozen VerbToolNames() authority and reports a verb that is genuinely
// absent from the reference. Appending a fabricated token that has no section in
// reference.md and asserting it is reported missing seals the gate against an
// "∅ ⊇ ∅" vacuous pass through the Phase 104/105 reference/skill churn. It is a
// confirm-and-seal anchor (expected green today); its value is that it would go
// RED if a future refactor made the membership check stop reporting genuinely
// absent names.
func TestReferenceContractDiscriminatesAbsentVerb(t *testing.T) {
	ref := readEmbeddedReference(t)
	// A fabricated token that is NOT a real frozen verb and carries no markdown
	// that could accidentally match a "## `helix ...`" section header.
	const fabricated = "totally-not-a-verb"
	require.NotContains(t, ref, referenceSectionHeader(fabricated),
		"precondition: the fabricated verb must have no section in reference.md")

	authorityPlus := append(append([]string{}, VerbToolNames()...), fabricated)
	missing := referenceMissingVerbs(ref, authorityPlus)
	require.Contains(t, missing, fabricated,
		"referenceMissingVerbs must report a genuinely-absent verb %q — the gate keys on the real frozen authority, not ∅ ⊇ ∅",
		fabricated)
}

// TestReferenceCompletenessRevertFails is the MANDATORY revert-and-fail proof
// (97-RESEARCH Anti-Vacuity Architecture, T-97-06). It takes the real embedded
// reference.md, strips ONE verb's section header in-memory, and asserts the SAME
// completeness helper now reports that verb as missing. This proves the gate is
// sourced from the VerbToolNames() registry and CAN go RED when a verb's coverage
// is dropped — it is not a tautology.
func TestReferenceCompletenessRevertFails(t *testing.T) {
	authority := VerbToolNames()
	ref := readEmbeddedReference(t)

	// Sanity: the intact reference passes (no missing verbs) — the baseline the
	// revert mutates away from.
	require.Empty(t, referenceMissingVerbs(ref, authority),
		"baseline reference.md must be complete before the revert mutation")

	const droppedVerb = "search-symbols"
	require.Contains(t, ref, referenceSectionHeader(droppedVerb),
		"precondition: %q section must exist to be dropped", droppedVerb)

	// Mutate: remove the verb's section header so the section is no longer present.
	mutated := strings.ReplaceAll(ref, referenceSectionHeader(droppedVerb), "## `helix REDACTED`")

	missing := referenceMissingVerbs(mutated, authority)
	require.Contains(t, missing, droppedVerb,
		"deleting verb %q's section MUST turn the completeness gate RED (report it missing), got missing=%v",
		droppedVerb, missing)
	require.Len(t, missing, 1,
		"dropping exactly one verb must surface exactly that one verb as missing, got %v", missing)
}
