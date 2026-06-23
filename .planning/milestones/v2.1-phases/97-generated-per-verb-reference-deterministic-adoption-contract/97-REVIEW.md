---
phase: 97-generated-per-verb-reference-deterministic-adoption-contract
reviewed: 2026-06-22T17:05:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - cmd/helix-refgen/main.go
  - cmd/helix-refgen/render.go
  - cmd/helix-refgen/main_test.go
  - internal/cli/verb.go
  - internal/cli/verb_test.go
  - internal/cli/skill.go
  - internal/cli/skill_test.go
  - internal/cli/setup_test.go
  - internal/cli/reference_contract_test.go
  - internal/cli/nudge_test.go
  - Makefile
  - .github/workflows/go-test.yml
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 97: Code Review Report

**Reviewed:** 2026-06-22T17:05:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Phase 97 adds the `cmd/helix-refgen` generator that renders a committed per-verb
`reference.md` from the live tool registry, a `--check` drift gate (REF-03) wired
into the Makefile and `go-test.yml`, the `cli.VerbSpecsForDocs()` / `VerbDoc` /
`FlagDoc` doc-facing seam, the `embed.FS` switch in `internal/cli/skill.go` that
ships the SKILL.md + reference.md bundle, and a set of contract/golden tests.

The implementation is solid on its load-bearing claims, all of which I verified
empirically:

- **`--check` drift gate is correct.** A missing file in `--check` mode hard-fails
  (exit 1 via `log.Fatalf` from `referenceStale`'s surfaced read error — NOT a
  false "up to date"); a stale file exits 1; an in-sync file exits 0. Verified by
  running the generator against a missing path, the live tree, and a temp round-trip.
- **Blank-import parity is faithful to docgen.** `cmd/helix-refgen/main.go` imports
  the same provider set the daemon registers, including `internal/skill/semantic`,
  which is present in both the generator and `internal/daemon/imports.go`. Counts
  reconcile: 50 `verbSpecs`, 50 reference sections, `VerbToolNames()` == 50.
- **`withinSkillRoot` path containment holds.** I exercised the lexical-root +
  cleaned-path `filepath.Rel` guard against exact-root, deeper, `..`-escape,
  nested-pair, and no-pair inputs; the `..`-escape case is correctly rejected and
  the install/uninstall loops derive destination names ONLY from the embedded FS.
- **`EmbeddedSkillBody()` is SKILL.md-only.** `embeddedSkillBytes()` reads exactly
  `skills/helix/SKILL.md`; `TestEmbeddedSkillBody` proves no reference.md banner
  bleed.
- **`VerbSpecsForDocs()` fresh-copy discipline is intact.** Each `VerbDoc` carries a
  freshly-allocated `[]FlagDoc`; the read-only test mutates a returned slice and
  proves the catalog is unaffected.
- **The contract tests are genuinely non-vacuous.** `reference_contract_test.go`
  sources completeness from `VerbToolNames()` (the registry authority, not the
  generator's own output), anchors the 50-verb count, and carries a mandatory
  revert-and-fail proof. `nudge_test.go`'s golden keys on the SPECIFIC emitted
  `helix <verb>` token (not a weak `"helix"` substring) and includes a wrong-verb
  revert proof. No tautology found.

All Phase-97 tests pass (`go test ./cmd/helix-refgen/... ./internal/cli/...`).

The findings below are quality/robustness defects, not correctness blockers. The
documented DEFER-97-01 (Bash `sed`/`cat` not steered, deferred to Phase 98) is an
intentional, recorded deferral and is explicitly NOT flagged.

## Warnings

### WR-01: `firstSentence` truncates on abbreviations ("e.g. ", "i.e. "), producing a misleading synopsis

**File:** `cmd/helix-refgen/render.go:99-105`

**Issue:** `firstSentence` splits on the first `". "` occurrence within 160 chars.
This treats any abbreviation that ends in a period-space (`e.g. `, `i.e. `, `vs. `,
`No. `) as a sentence boundary, silently truncating the synopsis mid-clause. I
confirmed the defect directly: `firstSentence("Find symbol, e.g. Foo, across the
repo. More text.")` returns `"Find symbol, e.g."` — a synopsis that reads as if the
verb's description simply stops at the abbreviation.

This is currently **latent**: I enumerated every rendered synopsis in the committed
`reference.md` and none of the 50 live descriptions hit this case today. But the
synopsis is sourced from each tool's free-text `Description`, which authors edit
independently of this generator. The first description that adds an "e.g." in its
opening clause will ship a truncated, misleading reference section, and because the
output stays byte-deterministic the `--check` gate will happily bless it. Classifying
WARNING (latent correctness defect in generated agent-facing docs), not BLOCKER,
because no live input triggers it yet.

**Fix:** Guard the boundary against a trailing single-token abbreviation, e.g. reject
a match where the token immediately preceding the `.` is a known abbreviation or is a
single lowercase letter / two-letter `x.y` form:

```go
func firstSentence(desc string) string {
	desc = strings.TrimSpace(desc)
	idx := strings.Index(desc, ". ")
	if idx <= 0 || idx >= 160 {
		return desc
	}
	// Reject common abbreviations that end in "."+space (e.g., i.e., vs.).
	head := desc[:idx+1]
	for _, abbr := range []string{"e.g.", "i.e.", "vs.", "etc.", "cf.", "No."} {
		if strings.HasSuffix(head, abbr) {
			// Look for the NEXT sentence boundary instead.
			if next := strings.Index(desc[idx+2:], ". "); next >= 0 && idx+2+next < 160 {
				return desc[:idx+2+next+1]
			}
			return desc
		}
	}
	return head
}
```

### WR-02: `installSkill` leaves a partially-written bundle (and an orphaned `.tmp`) if a mid-loop write/rename fails

**File:** `internal/cli/skill.go:164-188`

**Issue:** The per-file install loop writes-temp-then-renames each bundle file in
sequence with no rollback. If the bundle grows to N files and the write or rename of
file _k_ fails, files `0..k-1` have already been atomically renamed into place while
files `k..N-1` are absent, so `targetDir` is left in a torn state (e.g. a fresh
`reference.md` next to a stale `SKILL.md`, or vice-versa). Separately, on an
`os.Rename` failure the loop returns before removing `dst + ".tmp"`, orphaning a
`<name>.tmp` sibling next to the real file. The function's own doc comment promises
"a concurrent setup never observes a partial/corrupt file," which holds per-file but
NOT for the bundle as a whole.

This is robustness/atomicity, not a security or path-traversal issue (the destination
names are still embed-derived and contained), so WARNING. It matters because `helix
setup` is the install surface agents depend on; a torn bundle ships a SKILL.md that
references a reference.md that was never written.

**Fix:** On any error in the loop, best-effort remove the just-written `.tmp` before
returning; and consider staging all temp files first, then renaming them only after
every write succeeded, so a failure aborts before any file is swapped into place:

```go
for _, e := range entries {
	// ... build data, dst, tmp ...
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp skill file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp) // do not orphan the temp on failure
		return fmt.Errorf("renaming skill file: %w", err)
	}
}
```

(A two-pass write-all-temps / rename-all approach further closes the torn-bundle
window; the snippet above at minimum fixes the orphaned-`.tmp` leak.)

## Info

### IN-01: JSON / required-arg worked examples render non-runnable placeholders

**File:** `cmd/helix-refgen/render.go:138-167` (`placeholder`)

**Issue:** For `flagJSON` flags the generator emits `--seed-json='[]'`, but the
flag's own help text states the seed requires `symbol_id OR (file_path AND
symbol_name)`. An empty `[]` would fail at the daemon, so the worked example for
`explain-symbol-deep`, `find-related-symbols`, `get-change-impact-graph`, and
`validate-graph-edge` is illustrative-but-non-functional. The spec explicitly accepts
"placeholder tokens... illustrative without being hand-authored," so this is INFO, not
a defect — but an agent that copy-pastes the example verbatim gets a runtime error.
Consider a minimally-valid JSON object placeholder (e.g.
`'{"file_path":"internal/foo.go","symbol_name":"Foo"}'`) keyed off the `-json` flags.

### IN-02: `firstSentence`'s `idx < 160` fall-through can emit a multi-sentence synopsis

**File:** `cmd/helix-refgen/render.go:101`

**Issue:** When the first `". "` boundary lands at or beyond column 160, the function
returns the ENTIRE description rather than a first sentence. The result is still
byte-deterministic (so `--check` is unaffected), but the "terse synopsis" contract is
silently violated for any long-lead description. No live description triggers this
today. INFO — cosmetic determinism-preserving edge, surfaced for awareness alongside
WR-01.

### IN-03: `containedIn` separator-prefix check is platform-correct but worth a test pin

**File:** `internal/cli/skill.go:230-238`

**Issue:** `containedIn` rejects an escape by checking `rel != ".."` and
`!strings.HasPrefix(rel, ".."+string(filepath.Separator))`. This is correct on the
target platform, but the guard's correctness depends on `filepath.Rel` always
emitting OS-native separators — there is no explicit unit test pinning the
`".."`-prefix rejection for a `rel` that is exactly `".."` versus `"../x"`. The
existing `TestInstallSkillContainment` exercises the happy escape path through
`installSkill`, but a direct `containedIn`/`withinSkillRoot` table test (exact-root,
deeper, `..`-escape, nested-pair, no-pair) would lock the guard against future
refactors. I verified all five cases pass today; this is a test-coverage suggestion,
not a defect. INFO.

---

_Reviewed: 2026-06-22T17:05:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
