# Phase 105: SKILL.md Decision-Matrix Rewrite - Pattern Map

**Mapped:** 2026-06-24
**Files analyzed:** 2 (1 content asset modified, 1 test file extended)
**Analogs found:** 2 / 2

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `internal/cli/skills/helix/SKILL.md` | config (hand-authored steering asset) | transform (static content, embedded → installed) | itself (in-place rewrite of `## Decision matrix` section) | exact (self) |
| `internal/cli/skill_test.go` | test | transform (parse static asset → assert invariants) | `internal/cli/reference_contract_test.go` (anti-vacuity discriminator + revert-and-fail) | role+pattern exact |

Both files already exist. This is a content rewrite of one section in SKILL.md plus three NEW anti-vacuity tests appended to the existing `skill_test.go`. No new files, no new packages, no new Go deps (`git diff go.mod` must stay empty).

## Pattern Assignments

### `internal/cli/skills/helix/SKILL.md` (config asset, in-place section rewrite)

**Analog:** itself — the `## Decision matrix` table is rewritten in place; the frontmatter, prose, and token-note comment are preserved byte-for-byte except where a fill is required.

**Load-bearing anchors that MUST NOT move:**
- `## Decision matrix` heading at line 35 — the `StripDecisionMatrix` strip anchor (see scorecard.go:101). It is currently the LAST `## ` heading; the strip runs heading→EOF. Do NOT add any `## ` heading after the table (would shorten the strip and risk `TestSabotageNonNoop`). Group by capability with a lead column INSIDE the single table.
- Frontmatter `description` block (lines 3-11, ≈599 bytes) — capped at 1,536 bytes by `TestSkillDescriptionCap`/`TestSkillIdleCostBound`. Do NOT lengthen it while editing the body; the body is unbounded.
- Token-note HTML comment (lines 81-93) — must keep a real digit and the literal "Token note" / "SKILL-04" (`TestSkillTokenNoteFilled`, `TestSkillTokenNotePresent`). Stays after the matrix as a `<!-- -->` comment (NOT a `## ` heading), so it does not break the strip.

**Current matrix structure** (lines 37-75): columns `Question | Use this | Not this`, 36 data rows, all 50 frozen verbs cited as `` `helix <kebab>` ``.

**The 6 `—` placeholder rows to fill** (SKILL-02): lines 68, 69, 71, 72, 73, 75.

**The 5 QUERY/ACTION-mixed rows to split** (SKILL-01): lines 68, 70, 71, 72, 73 → produces +7 data rows (36→43).

**Verb-citation form to preserve** (every verb must stay backtick-fenced as `` `helix <kebab-verb>` `` so the drift gate at `skill_test.go:174` captures it):
```markdown
| Where is symbol `X` defined? | `helix go-to-definition --path --line --column` | `grep "X"` |
```

**Recommended grouping (lead `Capability` column, NO new `## ` heading, `†` for graph-prereq):**
```markdown
## Decision matrix

Grouped by capability. Rows marked † require `helix index-semantic-graph` first.

| Capability | Question | Use this | Not this |
|---|---|---|---|
| Navigation | Where is symbol `X` defined? | `helix go-to-definition` | `grep "X"` |
| ...
| Semantic graph | Symbols related to a symbol † | `helix find-related-symbols` | recursive grep |
```

**"Not this" fill source:** CLAUDE.md "Helix CLI tool routing" table (canonical). For verbs absent from CLAUDE.md, use the SKILL-ISSUE.md "after" draft named fallbacks. `get-tool-help` has no shell equivalent — use a named human-fallback (`guess` / `infer by reading`), never `—`.

---

### `internal/cli/skill_test.go` (test, append 3 anti-vacuity tests + 1 parse helper)

**Analog:** `internal/cli/reference_contract_test.go` — same package (`cli`), same testify imports, same "confirm-and-seal + revert-and-fail" anti-vacuity architecture the milestone demands.

**Existing imports to reuse** (`skill_test.go:1-9`): `os`, `path/filepath`, `regexp`, `strings`, `testing`. The reference_contract analog adds `github.com/stretchr/testify/assert` + `require` (already vendored, used across the package) — the new tests may import them.

**(1) The drift cross-check the planner extends** — `TestSkillVerbMembershipDrift` (`skill_test.go:179-202`). Every backtick-fenced `` `helix <kebab>` `` in the body must map kebab→snake into `VerbToolNames()`. KEEP THIS GREEN — the new prerequisite `†` legend should cite `` `helix index-semantic-graph` `` so it is also a real verb citation. The classification-completeness assertion in the NEW tests should iterate `VerbToolNames()` so the QUERY/ACTION map cannot drift:
```go
var helixVerbRe = regexp.MustCompile("`helix ([a-z][a-z0-9-]+)")

func TestSkillVerbMembershipDrift(t *testing.T) {
	catalog := make(map[string]bool)
	for _, n := range VerbToolNames() {
		catalog[n] = true
	}
	matches := helixVerbRe.FindAllStringSubmatch(embeddedSkillBytes(), -1)
	if len(matches) == 0 {
		t.Fatal("no `helix <verb>` citations found in SKILL.md decision table")
	}
	seen := make(map[string]bool)
	for _, m := range matches {
		kebab := m[1]
		if seen[kebab] { continue }
		seen[kebab] = true
		snake := strings.ReplaceAll(kebab, "-", "_")
		if !catalog[snake] {
			t.Errorf("SKILL.md cites `helix %s` (tool %q) which is NOT in cli.VerbToolNames()", kebab, snake)
		}
	}
}
```

**(2) The anchor + size-cap guards to PRESERVE (do not break):**

`StripDecisionMatrix` + `TestSabotageNonNoop` (`test/oracle/adopt/scorecard.go:101-122`, `scorecard_test.go:116-125`) — proves the `## Decision matrix` anchor is present and the strip is non-trivial:
```go
const decisionMatrixHeading = "## Decision matrix" // load-bearing literal

func StripDecisionMatrix(skill string) string {
	i := strings.Index(skill, decisionMatrixHeading)
	if i < 0 { return skill }
	rest := skill[i+len(decisionMatrixHeading):]
	j := strings.Index(rest, "\n## ")
	if j < 0 { return skill[:i] } // matrix is last section → strip to EOF
	return skill[:i] + rest[j:]
}

func TestSabotageNonNoop(t *testing.T) {
	body := cli.EmbeddedSkillBody()
	stripped := StripDecisionMatrix(body)
	require.Lessf(t, len(stripped), len(body), "...anchor moved...")
}
```
INVARIANT: keep `## Decision matrix` byte-exact AND keep it the terminal `## ` section (the token-note is a `<!-- -->` comment, not a heading, so it does not register). Adding `## Prerequisites` after the table makes `j >= 0` short-circuit and shortens the strip.

SKILL-04 size cap — `TestSkillDescriptionCap` (`skill_test.go:105-122`) and `TestSkillIdleCostBound` (`skill_test.go:141-150`):
```go
const cap1536 = 1536
total := len(desc)
if wtu, ok := frontmatterValue(fm, "when_to_use"); ok { total += len(wtu) }
if total > cap1536 {
	t.Errorf("description(+when_to_use) = %d bytes, exceeds Claude Code listing cap %d", total, cap1536)
}
```
The cap is on the frontmatter `description` ONLY (≈937 bytes headroom). +7 body rows do not touch it.

**(3) The break-the-invariant / anti-vacuity pattern to MIRROR** — from `reference_contract_test.go`. Three structural moves the new SKILL-01/02/03 tests must replicate:

- *Factored pure helper* run identically on real bytes and on a synthetic mutation (`referenceMissingVerbs`, lines 37-46). The new tests need a small in-package `parseMatrixRows(body string) []row` helper (locate `## Decision matrix`, take `|`-leading lines, split/trim — stdlib only) plus a per-criterion checker (e.g. `matrixRowMixesQueryAction`, `matrixRowEmptyNotThis`, `matrixRowMissingGraphPrereq`).

- *Confirm-and-seal discriminator* — fabricated input proves the gate keys on real authority, not `∅ ⊇ ∅` (`TestReferenceContractDiscriminatesAbsentVerb`, lines 85-98):
```go
func TestReferenceContractDiscriminatesAbsentVerb(t *testing.T) {
	ref := readEmbeddedReference(t)
	const fabricated = "totally-not-a-verb"
	require.NotContains(t, ref, referenceSectionHeader(fabricated),
		"precondition: the fabricated verb must have no section in reference.md")
	authorityPlus := append(append([]string{}, VerbToolNames()...), fabricated)
	missing := referenceMissingVerbs(ref, authorityPlus)
	require.Contains(t, missing, fabricated,
		"...the gate keys on the real frozen authority, not ∅ ⊇ ∅")
}
```
The NEW negative arms mirror this exactly: feed the SAME checker a hand-built table containing a known mixed row (`| q | `+"`helix read-memory`"+` / `+"`helix write-memory`"+` | ... |`) and assert it is REJECTED; feed a synthetic `| q | v | — |` row and assert rejection; feed a synthetic graph-reader row missing the `†` marker and assert rejection.

- *Revert-and-fail proof* — mutate the real intact bytes by one element and assert the gate goes RED (`TestReferenceCompletenessRevertFails`, lines 106-128):
```go
const droppedVerb = "search-symbols"
mutated := strings.ReplaceAll(ref, referenceSectionHeader(droppedVerb), "## `helix REDACTED`")
missing := referenceMissingVerbs(mutated, authority)
require.Contains(t, missing, droppedVerb, "...MUST turn the completeness gate RED...")
require.Len(t, missing, 1, "...exactly that one verb...")
```

**NEW tests to add** (Wave 0, all in `skill_test.go`, stdlib + testify):
| Test | Asserts (positive arm on real SKILL.md) | Negative arm (anti-vacuity) |
|------|------------------------------------------|------------------------------|
| `TestSkillMatrixNoQueryActionMix` | no real row's "Use this" cell mixes a QUERY and an ACTION verb (classified via a §D map keyed to `VerbToolNames()`) | a synthetic mixed row is REJECTED |
| `TestSkillMatrixNoEmptyNotThis` | no real row's last cell is `—`/empty | a synthetic `| q | v | — |` row is REJECTED |
| `TestSkillMatrixGraphPrereq` | each of the 8 graph-reader verbs' row carries the `index-semantic-graph` marker/legend | a synthetic graph-reader row missing the marker is REJECTED |
| (map-completeness) | every frozen verb in `VerbToolNames()` is classified exactly once in the QUERY/ACTION map | — |

## Shared Patterns

### Authority single-source-of-truth
**Source:** `cli.VerbToolNames()` (50 frozen verbs, `verbs_gen.go`)
**Apply to:** all three new matrix tests + the existing drift test
Never hardcode a verb list in a test (`reference_contract_test.go:54-59` anchors `len==50`). Key the QUERY/ACTION classification map and the graph-reader set against `VerbToolNames()` so they cannot fall behind a future verb add.

### Anti-vacuity (confirm-and-seal + revert-and-fail)
**Source:** `internal/cli/reference_contract_test.go:76-128`
**Apply to:** every new SKILL-01/02/03 test
Each gate gets BOTH a positive arm on the real embedded bytes AND a negative arm that feeds a fabricated mixed/placeholder/missing-marker row and asserts rejection — proving the gate bites (the milestone anti-vacuity requirement).

### Embedded-asset read seam
**Source:** `embeddedSkillBytes()` / `EmbeddedSkillBody()` (used at `skill_test.go:75,120,185`) and `embeddedSkillFS.ReadFile(...)` (`reference_contract_test.go:17`)
**Apply to:** the new matrix-parse helper — read the SAME bytes `installSkill` ships, so the gate covers the artifact the agent actually receives.

### Frontmatter splitter (zero-dep)
**Source:** `frontmatterBlock` / `frontmatterValue` (`skill_test.go:16-70`)
**Apply to:** any new check that must read the description without a YAML dep (keeps `git diff go.mod` empty).

## No Analog Found

None — both files exist and have direct analogs.

## Metadata

**Analog search scope:** `internal/cli/`, `test/oracle/adopt/`
**Files scanned:** 4 (SKILL.md, skill_test.go, scorecard.go, scorecard_test.go, reference_contract_test.go)
**Pattern extraction date:** 2026-06-24
