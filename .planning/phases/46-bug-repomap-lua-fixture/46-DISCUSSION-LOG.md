# Phase 46: bug-repomap-lua-fixture - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 46-bug-repomap-lua-fixture
**Areas discussed:** Testdata exclusion policy, Fix strategy preference, Regression test location, Scope of the fix

---

## Testdata exclusion policy

**Question:** How should get_repo_map treat known testdata paths (testdata/, legacy/test/resources/, */test/resources/*, fixtures/)?

| Option | Description | Selected |
|--------|-------------|----------|
| Exclude by default, flag to include | Drop these paths from tag extraction entirely unless an explicit include_testdata flag is passed. Simple, predictable; risks hiding legitimate testdata-like dirs in other repos. | |
| De-rank, don't exclude | Keep tags but apply a ranking penalty so testdata never dominates. Preserves discoverability; more subtle behavior. | |
| Fix ranking only, no path heuristics | Don't special-case paths at all — fix the underlying PageRank/extractor bug so testdata naturally ranks low when there are more-connected sources. Purist fix. | ✓ |
| You decide after RCA | Defer — let researcher surface RCA first, then pick the minimum intervention that fixes the symptom without new heuristics. | |

**User's choice:** Fix ranking only, no path heuristics
**Notes:** Path filters explicitly rejected — would mask the underlying bug.

---

## Fix strategy preference

**Question:** Do you have a leading hypothesis the researcher should prioritize investigating first?

| Option | Description | Selected |
|--------|-------------|----------|
| No prior — researcher picks order | Let RCA follow the evidence; all four candidates (PageRank, extractor, workspace root, elision) start equal. | ✓ |
| PageRank starvation | Suspect Go sources have low PageRank because cross-file references aren't being captured. | |
| Elision / rendering bug | Suspect tag extraction is fine but the token-budget elider picks the single deepest file. | |
| Workspace root / walk scope | Suspect the walker is not treating internal/ as part of workspace, or legacy/ is mis-prioritized. | |

**User's choice:** No prior — researcher picks order
**Notes:** Researcher investigates all four candidates on equal footing.

---

## Regression test location

**Question:** Where should the regression test land?

| Option | Description | Selected |
|--------|-------------|----------|
| internal/repomap/ unit test w/ synthetic fixture | Minimal polyglot fixture tree; assert ranked output contains ≥1 Go symbol. Fast, hermetic. | |
| test/oracle/ integration test vs Serena repo | Assert against the real Serena workspace. Most realistic but brittle. | |
| Both — unit for invariant, oracle smoke | Unit enforces the invariant on a controlled fixture; oracle pins the actual symptom. | ✓ |

**User's choice:** Both — unit for invariant, oracle smoke
**Notes:** Unit test is non-negotiable; oracle test may be downgraded if brittle in practice.

---

## Scope of the fix

**Question:** How broad should the fix be within this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Narrow — just this symptom | Fix the specific Lua-fixture-dominance bug; defer all adjacent cleanup. | |
| Fix + adjacent low-risk cleanup | If RCA reveals a small adjacent bug, fix it too if low-risk and inside M sizing. | |
| You decide after RCA | Let researcher/planner recommend scope based on RCA findings; user reviews before execute. | ✓ |

**User's choice:** You decide after RCA
**Notes:** Default narrow; planner proposes scope expansion only if justified by RCA, user approves.

---

## Claude's Discretion

- Exact synthetic fixture shape for the unit test
- Format/location of RCA documentation in the phase review
- Whether to refactor repomap internals touched directly during the fix

## Deferred Ideas

- Path-based testdata exclusion (explicitly rejected as a fix mechanism)
- Broader repomap ranking overhauls / cross-language scoring changes
- Cross-file reference capture improvements beyond what's needed for this symptom
