# Phase 97 — Deferred Items

## DEFER-97-01 — `sed -i` / `cat` Bash commands do not fire a nudge advisory today (classifier gate, not steerMessage)

**Discovered during:** Plan 97-02, ADOPT-01b golden authoring (Feature ADOPT-01b).

**Finding:** `bashSteerMessage` (nudge.go:188-204) has dedicated branches mapping `cat` →
`helix read-file` and `sed -i` → `helix replace-in-file`. However, `runNudge`
(nudge.go:105) only reaches `steerMessage`/`bashSteerMessage` when
`isGrepReadTool(toolName, input)` returns true, and `isGrepReadTool`'s Bash arm
(nudge.go:434-438) matches ONLY commands containing `grep` / `find` / `rg` / `ag`.
It does **not** match `sed` or `cat`. Therefore a `Bash` invocation of
`sed -i 's/a/b/' file.go` or `cat file.go` produces **no advisory** today — the
`cat`/`sed` branches in `bashSteerMessage` are effectively dead for Bash callers.

(The `cat`-equivalent shape DOES fire via the **Read tool** path — `steerMessage`'s
`case "Read"` emits `helix read-file` — and the `grep`-equivalent via the **Grep
tool** path. Only the *Bash-spelled* `cat`/`sed` shapes are silent.)

**Why deferred (NOT fixed here):** Phase 97's plan explicitly scopes classifier
broadening OUT ("Do NOT broaden the classifier (that is STEER-01, Phase 98) —
assert only the current mapping." — 97-02-PLAN.md Feature ADOPT-01b
`<implementation>`). Widening `isGrepReadTool` to admit `sed`/`cat` would be the
forbidden STEER-01 change. The ADOPT-01b golden therefore asserts the **live
current mapping** (the shapes that genuinely fire) and the empty-bucket floor is
honored against real emitting shapes — it does NOT assert non-existent
sed/cat-Bash behavior (that would be a vacuous/false golden, the exact anti-pattern
this phase exists to kill).

**Routing:** STEER-01, Phase 98 — when broadening the nudge classifier, decide
whether `sed`/`cat` Bash commands should fire (and wire `isGrepReadTool` to admit
them so the `bashSteerMessage` `cat`/`sed` branches become live). The
`TestNudgeShapeGolden` `sed-bash-silent` / `cat-bash-silent` sub-cases in
`nudge_test.go` document the current silence and will need updating then.
