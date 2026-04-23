# Phase 45: Cross-link & Manual-Config Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 45-polish-crosslinks
**Areas discussed:** README manual-config scope, Cross-link scope, F-06 rust-analyzer currency, Verification approach

---

## Area selection

| Option | Description | Selected |
|--------|-------------|----------|
| README manual-config scope | F-02 | ✓ |
| Cross-link scope | F-12 | ✓ |
| F-06 rust-analyzer currency | criterion #3 | ✓ |
| Verification approach | how to prove success criteria | ✓ |

---

## README manual-config scope

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit pointer only | Keep 3 JSONs, replace vague line 134 with explicit sentence naming other 7 clients | ✓ |
| Add serena setup clients | Expand README to cover 7 setup-CLI clients | |
| Full 9-client coverage | Mirror INSTALL.md in README | |

**User's choice:** Explicit pointer only (Recommended).
**Notes:** Matches F-02's literal recommendation; keeps README light.

## README pointer form

| Option | Description | Selected |
|--------|-------------|----------|
| One sentence naming clients | Single line pointing to INSTALL.md with the 7 other clients named | ✓ |
| Short bullet list + link | Bulleted list of other clients with INSTALL anchor | |

**User's choice:** One sentence naming clients (Recommended).

---

## Cross-link scope

| Option | Description | Selected |
|--------|-------------|----------|
| Required two only | README→CHANGELOG + USAGE→INSTALL | ✓ |
| Required + CONTRIBUTING back-links | Also add CONTRIBUTING→USAGE/README/INSTALL | |

**User's choice:** Required two only (Recommended).
**Notes:** CONTRIBUTING back-links flagged as deferred.

## README→CHANGELOG placement

| Option | Description | Selected |
|--------|-------------|----------|
| End of README | Footer / docs-index area | ✓ |
| Top badges / intro | Near the top | |
| You decide | Claude picks | |

**User's choice:** End of README (Recommended).

## USAGE→INSTALL placement

| Option | Description | Selected |
|--------|-------------|----------|
| Prerequisites / Setup section | Early, natural reading order | ✓ |
| Troubleshooting section | From troubleshooting intro | |
| You decide | Claude picks | |

**User's choice:** Prerequisites / Setup section (Recommended).

---

## F-06 rust-analyzer currency

| Option | Description | Selected |
|--------|-------------|----------|
| Text review only | Read USAGE.md:531–537, confirm v1.90 reference and Symptom/Cause/Workaround still match | ✓ |
| Live re-test | Spin up Rust workspace and retry rename_symbol | |

**User's choice:** Text review only (Recommended).
**Notes:** F-06 already marked resolved in re-run; this is a freshness sanity check only.

---

## Verification approach

| Option | Description | Selected |
|--------|-------------|----------|
| Targeted grep + link checks | Deterministic grep/diff against edited files | ✓ |
| Full v1.8 integration-check re-run | Re-run workflow and flip F-02/F-12 status | |
| Both | Grep during execution + integration-check as final gate | |

**User's choice:** Targeted grep + link checks (Recommended).

---

## Claude's Discretion

- Exact sentence wording for D-02, D-04, D-05 (Claude picks phrasing preserving locked decisions).
- Section anchor targets in INSTALL.md (Claude verifies actual anchor).
- Whether CHANGELOG link points to file root or latest release heading.

## Deferred Ideas

- CONTRIBUTING → USAGE/README/INSTALL back-links (F-12 optional part).
- Expanding README manual-config to 7 or 9 clients.
- Full v1.8 integration-check re-run after edits.
- Live rust-analyzer rename re-test.
