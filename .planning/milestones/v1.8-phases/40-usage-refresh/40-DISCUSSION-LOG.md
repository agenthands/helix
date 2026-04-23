# Phase 40: USAGE Refresh - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 40-usage-refresh
**Areas discussed:** Feature documentation placement, Documentation depth per feature, Tutorial updates, Troubleshooting format

---

## Feature Documentation Placement

| Option | Description | Selected |
|--------|-------------|----------|
| New 'Feature Guide' section | Add dedicated section after Quick Tutorials for feature-by-feature docs | ✓ |
| Integrate into existing tutorials | Weave features into existing tutorials + add new tutorials | |
| Hybrid: feature section + tutorial updates | Both a Feature Guide section and updated tutorials | |

**User's choice:** New 'Feature Guide' section
**Notes:** Keeps tutorials as workflow guides, Feature Guide as reference. Placed between Quick Tutorials and Profiles and Modes.

---

## Documentation Depth per Feature

| Option | Description | Selected |
|--------|-------------|----------|
| Concept + usage example | 1-2 paragraphs + concrete tool call example per feature | ✓ |
| Brief one-liners | One paragraph per feature, no examples | |
| Deep reference docs | Full docs with all parameters, multiple examples, edge cases | |

**User's choice:** Concept + usage example
**Notes:** Enough to discover and use, not exhaustive internals.

---

## Tutorial Updates

| Option | Description | Selected |
|--------|-------------|----------|
| Update Tutorial 1 only | Update Onboarding to use `serena setup`, leave others unchanged | ✓ |
| Update all + add new | Update Tutorials 1-2 + add RepoMap tutorial | |
| Leave tutorials as-is | No tutorial changes, Feature Guide handles everything | |

**User's choice:** Update Tutorial 1 only
**Notes:** Only the onboarding tutorial needs updating (setup CLI replaces manual JSON config). Refactoring and code review workflows haven't changed.

---

## Troubleshooting Format

| Option | Description | Selected |
|--------|-------------|----------|
| Same format as existing | Follow Symptom/Cause/Fix pattern, include specific workarounds | ✓ |
| Brief notes | Shorter symptom + one-line fix entries | |
| FAQ style | Q&A format instead of structured entries | |

**User's choice:** Same format as existing (Symptom/Cause/Fix)
**Notes:** Consistent with rust-analyzer, circuit breaker, and other existing troubleshooting entries.

---

## Claude's Discretion

- Exact wording within Feature Guide subsections
- Feature ordering within the Feature Guide
- Tree-sitter grammar coverage placement
- Troubleshooting workaround detail level

## Deferred Ideas

None — discussion stayed within phase scope.
