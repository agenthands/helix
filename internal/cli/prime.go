package cli

// primingMatrixMaxBytes bounds the SessionStart priming text (STEER-02). It mirrors
// the SKILL-04 idle-cost discipline (the skill description is capped at 1,536 chars);
// the priming matrix is allowed a slightly larger, still-terse budget. The matrix is
// a compiled constant, so this cap is asserted at test time, never exceeded at
// runtime. Keep the matrix well under this bound — it is injected into the agent's
// session context once per session and must NOT regress the idle-cost win by
// inlining the 32 KB reference or the full SKILL body.
const primingMatrixMaxBytes = 2048

// primingMatrix is the terse "use X not Y" decision matrix emitted once per session
// by the SessionStart hook (via `helix activate` stdout). It is the SINGLE source of
// truth for the priming text — a compiled package constant that cannot drift at
// runtime and cannot fail the session.
//
// The rows are a curated subset of internal/cli/skills/helix/SKILL.md's
// "## Decision matrix" (the canonical use-X-not-Y table). They steer the agent's
// most common grep/cat/sed/find reflexes toward the specific frozen `helix` verbs.
// Every `helix <verb>` cited here MUST be a real frozen verb in VerbToolNames();
// TestSessionStartPriming asserts this (kebab→snake), so a future verb rename
// surfaces as a test failure instead of a silently-stale matrix. Do NOT inline the
// full SKILL body or the generated reference.md here — that would regress the
// idle-cost bound (98-RESEARCH Pitfall 5). Run `helix get-tool-help` for the full
// per-verb reference.
const primingMatrix = `Helix is active. Prefer the verbs below over grep/cat/sed/find — they parse the AST and (first-class langs) the LSP, returning real symbols not string matches:
- Find where a symbol is defined → helix go-to-definition (not grep)
- Find a symbol by name → helix search-symbols (not grep "func X")
- Who calls/references Y → helix find-references / helix get-call-hierarchy (not grep -r)
- Implementations of an interface → helix find-implementations
- Type hierarchy (super/sub) → helix get-type-hierarchy
- Type/signature at a position → helix get-hover-info
- A file's outline → helix get-symbol-overview (not cat file)
- Impact of changing a symbol → helix analyze-blast-radius
- Content search across code → helix search-in-files (not grep -r)
- Find files by glob → helix find-files (not find -name)
- Read a file or a range → helix read-file (not cat / sed -n)
- Find-and-replace in a file → helix replace-in-file (not sed -i)
- Replace a function body → helix replace-symbol-body (not sed -i)
- Rename a symbol across files → helix rename-symbol (not sed -i)
- Edit tolerant of LLM drift → helix fuzzy-edit (not sed -i)
- Ranked structural repo overview → helix get-repo-map
- Context around a symbol/task → helix get-context
- Errors/warnings in a file → helix get-diagnostics
Run helix get-tool-help for the full verb reference.
`

// sessionPrimingText returns the terse SessionStart priming matrix (STEER-02). It is
// a PURE function — it returns the compiled primingMatrix constant with no daemon
// dial and no file I/O, so it can never fail the session (fail-open). The caller
// (runActivate) emits it to stdout best-effort after the activation status line; a
// write failure must not change the command's return value.
func sessionPrimingText() string { return primingMatrix }
