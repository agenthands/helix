package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// codexAGENTSMaxBytes is the Codex project_doc_max_bytes cap (32 KiB) applied to
// the whole AGENTS.md file. The appended Helix block is trimmed to a pointer (never
// the user content) to stay under it. (STACK.md A2; developers.openai.com/codex.)
const codexAGENTSMaxBytes = 32 * 1024

// codexAgentsPath resolves the Codex AGENTS.md destination: project root
// (<ProjectDir>/AGENTS.md) or, when global, ~/.codex/AGENTS.md. (STACK.md A2.)
func codexAgentsPath(projectDir string, global bool) string {
	if global {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".codex", "AGENTS.md")
		}
	}
	return filepath.Join(projectDir, "AGENTS.md")
}

// codexHooksPath resolves the Codex hooks.json destination: project
// <ProjectDir>/.codex/hooks.json or, when global, ~/.codex/hooks.json. Codex reads
// both project and home hooks; setup writes the scope the user selected. (STACK.md
// A1; developers.openai.com/codex/hooks.)
func codexHooksPath(projectDir string, global bool) string {
	if global {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".codex", "hooks.json")
		}
	}
	return filepath.Join(projectDir, ".codex", "hooks.json")
}

// geminiInstructionPath resolves the Gemini GEMINI.md destination: project root
// (<ProjectDir>/GEMINI.md) or, when global, ~/.gemini/GEMINI.md. (STACK.md A2;
// geminicli.com.) Gemini documents no hard project-doc cap, so the writer uses no
// byte cap for it.
func geminiInstructionPath(projectDir string, global bool) string {
	if global {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".gemini", "GEMINI.md")
		}
	}
	return filepath.Join(projectDir, "GEMINI.md")
}

// genericInstructionPath resolves the generic cross-tool instruction file:
// <ProjectDir>/AGENTS.md (the emerging agents.md standard). The generic client has
// no hook surface.
func genericInstructionPath(projectDir string) string {
	return filepath.Join(projectDir, "AGENTS.md")
}

// writeCodexHooks writes a Codex PreToolUse hooks.json whose command invokes
// `<binaryPath> nudge` — the SAME runNudge engine Claude uses (one engine, two
// runtimes; AGENT-03). It mirrors helixHookConfig's PreToolUse entry shape and is
// built via map[string]any + encoding/json (NEVER string-concatenated JSON,
// T-34-01). The hook is advisory-only: it emits the shared additionalContext
// envelope at exit 0 and NEVER a permissionDecision:"deny" default (locked
// anti-feature). The write is atomic (temp+rename), path-contained, and
// byte-stable on re-run.
//
// The config shape pins the documented Codex hooks convention (STACK.md A1):
// {"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command",
// "command":["<bin>","nudge"]}]}]}}. The exact live byte-shape is gated by the
// Task 2 human-verify checkpoint; the command-invokes-nudge + advisory-only
// invariants asserted here are the parts Helix controls.
func writeCodexHooks(path, binaryPath string) error {
	// Path containment (Security V12): refuse a ".." traversal destination.
	if strings.Contains(filepath.ToSlash(path), "/../") {
		return fmt.Errorf("refusing hooks.json write: %q contains a path traversal escape", path)
	}

	config := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": []any{binaryPath, "nudge"},
							"timeout": 5,
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling codex hooks config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating codex hooks directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp codex hooks file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming codex hooks file: %w", err)
	}
	return nil
}

// Sentinel markers bracketing the Helix-managed block inside a per-agent
// instruction file (AGENTS.md / GEMINI.md / generic). They are HTML comments so
// they are invisible in rendered markdown, and unique enough to strip on re-run
// (the idempotent filter-then-rewrite, mirroring mergeHooksIntoSettings' use of
// the "helix_managed": true marker for hook entries).
const (
	helixBlockBegin = "<!-- helix:begin (managed by `helix setup`) -->"
	helixBlockEnd   = "<!-- helix:end -->"
)

// agentInstructionBody returns the terse Helix steering block written into a
// per-agent instruction file. It is the "use X, not Y" decision matrix (sourced
// from SKILL.md's table) plus a pointer to the full reference / `helix
// get-tool-help`. It is deliberately terse so a Codex AGENTS.md stays well under
// the 32-KiB project_doc cap — the full 32-KiB reference is NEVER inlined (that
// would blow the cap when combined with user content); the agent is pointed at it
// instead.
func agentInstructionBody() string {
	return strings.TrimSpace(`
## Helix CLI — use the symbolic verb, not grep/sed/cat

This project ships **Helix** — a code-intelligence CLI. Prefer ` + "`helix <verb>`" + ` over
grep/sed/cat/find/ls/Read: the verbs parse the AST and (for first-class languages)
consult the LSP, returning real definitions, callers, and type-resolved references
instead of string matches.

| Question | Use this | Not this |
|---|---|---|
| Where is symbol ` + "`X`" + ` defined? | ` + "`helix go-to-definition`" + ` | ` + "`grep \"X\"`" + ` |
| Find a symbol by name | ` + "`helix search-symbols`" + ` | ` + "`grep \"func X\"`" + ` |
| Who calls / references ` + "`Y`" + `? | ` + "`helix find-references`" + ` / ` + "`helix get-call-hierarchy`" + ` | ` + "`grep -r \"Y(\"`" + ` |
| Implementations of an interface | ` + "`helix find-implementations`" + ` | ` + "`grep \"implements\"`" + ` |
| Type hierarchy (super/sub) | ` + "`helix get-type-hierarchy`" + ` | read + reason |
| A file's shape (outline) | ` + "`helix get-symbol-overview`" + ` | ` + "`cat file`" + ` |
| Impact of changing a symbol | ` + "`helix analyze-blast-radius`" + ` | recursive grep |
| Read a file (or a range) | ` + "`helix read-file`" + ` | ` + "`cat`" + ` / ` + "`sed -n`" + ` |
| Find-and-replace in a file | ` + "`helix replace-in-file`" + ` | ` + "`sed -i`" + ` |
| Replace a function body | ` + "`helix replace-symbol-body`" + ` | ` + "`sed -i`" + ` |
| Rename a symbol across files | ` + "`helix rename-symbol`" + ` | ` + "`sed -i`" + ` |
| Content search across code | ` + "`helix search-in-files`" + ` | ` + "`grep -r`" + ` |
| Find files by glob | ` + "`helix find-files`" + ` | ` + "`find . -name`" + ` |
| Ranked structural repo overview | ` + "`helix get-repo-map`" + ` | ` + "`tree`" + ` / many ` + "`cat`" + ` |

Run ` + "`helix get-tool-help`" + ` for the full per-verb argument reference (or read the
installed ` + "`reference.md`" + ` if your runtime bundles it).`)
}

// agentInstructionPointer returns the minimal fallback block used when the full
// terse body would push the file past its byte cap. It preserves the single most
// important instruction (prefer helix; where to get the full reference) without
// the inline matrix, so even a near-cap user AGENTS.md gets steering without
// trimming a single byte of user content.
func agentInstructionPointer() string {
	return strings.TrimSpace(`
## Helix CLI — use the symbolic verb, not grep/sed/cat

This project ships **Helix**, a code-intelligence CLI. Prefer ` + "`helix <verb>`" + ` over
grep/sed/cat/find/ls/Read. Run ` + "`helix get-tool-help`" + ` for the full verb reference.`)
}

// writeAgentInstructions appends or refreshes the Helix-managed block in the
// per-agent instruction file at path, idempotently and atomically:
//
//   - read the existing file (empty if absent)
//   - strip any prior Helix block (everything between the sentinels, inclusive),
//     so a re-run never duplicates the block (filter-then-rewrite, mirroring
//     mergeHooksIntoSettings)
//   - append a fresh sentinel-delimited block built from body, preserving all
//     surviving user content verbatim
//   - if the result exceeds maxBytes (e.g. 32*1024 for a Codex AGENTS.md), trim
//     the APPENDED block down to a minimal pointer — NEVER the user content; if
//     even the pointer cannot fit, return an error rather than truncating user
//     bytes
//   - write atomically (temp + rename, mirroring saveSessionStats) so a
//     concurrent setup never observes a partial file
//
// maxBytes <= 0 means "no cap" (used for runtimes like Gemini that document no
// hard project-doc size limit). The destination is path-contained: a path whose
// directory escapes via ".." is refused before any write (Security V12).
func writeAgentInstructions(path, body string, maxBytes int) error {
	// Path containment (Security V12): refuse a destination whose path contains a
	// ".." traversal segment. Derive the declared root from the LEXICAL (uncleaned)
	// parent directory and require the CLEANED full path to resolve at-or-under it
	// with no ".." escape — the same posture as withinSkillRoot. A path like
	// "<dir>/sub/../../escaped/AGENTS.md" declares lexical parent
	// "<dir>/sub/../../escaped" but the cleaned path leaves that declared root, so
	// the guard rejects it before any write.
	lexicalDir := filepath.Dir(path)
	if !containedIn(filepath.Clean(lexicalDir), filepath.Clean(path)) ||
		strings.Contains(filepath.ToSlash(path), "/../") {
		return fmt.Errorf("refusing instruction-file write: %q contains a path traversal escape", path)
	}
	dir := filepath.Clean(lexicalDir)

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading instruction file %s: %w", path, err)
	}

	user := stripHelixBlock(existing)
	user = ensureTrailingNewline(user)

	build := func(blockBody string) string {
		block := helixBlockBegin + "\n" + strings.TrimRight(blockBody, "\n") + "\n" + helixBlockEnd + "\n"
		return user + block
	}

	content := build(body)
	if maxBytes > 0 && len(content) > maxBytes {
		// Over cap: trim the APPENDED block to a minimal pointer, never the user
		// content.
		content = build(agentInstructionPointer())
		if len(content) > maxBytes {
			// Even the minimal pointer does not fit alongside the user content.
			// Refuse rather than truncate user bytes.
			return fmt.Errorf(
				"cannot append Helix block to %s within %d bytes without truncating user content (user content is %d bytes)",
				path, maxBytes, len(user),
			)
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating instruction-file directory: %w", err)
	}

	// Atomic write: temp + rename (mirror saveSessionStats) so a concurrent setup
	// never observes a partial file, and no .tmp sibling is left on success.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing temp instruction file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming instruction file: %w", err)
	}
	return nil
}

// stripHelixBlock removes every Helix-managed block (the bytes from
// helixBlockBegin to helixBlockEnd, inclusive) from content, returning only the
// surrounding user content. It is the idempotency primitive: stripping before
// re-appending guarantees exactly one block on re-run. It tolerates a missing end
// sentinel (a malformed prior write) by stripping from the begin marker to end of
// file, so a corrupted block is repaired rather than duplicated.
func stripHelixBlock(content string) string {
	for {
		start := strings.Index(content, helixBlockBegin)
		if start < 0 {
			return content
		}
		// Find the end sentinel after the begin marker.
		rest := content[start+len(helixBlockBegin):]
		endRel := strings.Index(rest, helixBlockEnd)
		if endRel < 0 {
			// No closing sentinel: drop from the begin marker to end of file.
			return content[:start]
		}
		end := start + len(helixBlockBegin) + endRel + len(helixBlockEnd)
		// Also consume a single trailing newline immediately after the block so
		// re-runs do not accumulate blank lines.
		if end < len(content) && content[end] == '\n' {
			end++
		}
		content = content[:start] + content[end:]
	}
}

// ensureTrailingNewline returns s with exactly one trailing newline, or "" when s
// is empty (an empty user file should not gain a leading blank line before the
// block). It guarantees the appended block starts on its own line.
func ensureTrailingNewline(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimRight(s, "\n") + "\n"
}

// removeAgentInstructions strips the Helix-managed block from the instruction file
// at path, preserving all user content, and writes it back atomically. A missing
// file is a no-op (idempotent uninstall). If the file contained no Helix block it
// is left untouched. Path containment is enforced as in writeAgentInstructions.
func removeAgentInstructions(path string) error {
	if strings.Contains(filepath.ToSlash(path), "/../") {
		return fmt.Errorf("refusing instruction-file edit: %q contains a path traversal escape", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading instruction file %s: %w", path, err)
	}
	existing := string(data)
	stripped := stripHelixBlock(existing)
	if stripped == existing {
		return nil // no Helix block present — nothing to do.
	}
	stripped = strings.TrimRight(stripped, "\n")
	if stripped != "" {
		stripped += "\n"
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(stripped), 0644); err != nil {
		return fmt.Errorf("writing temp instruction file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming instruction file: %w", err)
	}
	return nil
}
