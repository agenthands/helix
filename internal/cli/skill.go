package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// embeddedSkillFS is the Claude Code Agent Skill BUNDLE: the terse SKILL.md
// (idle tier — teaches the frozen `helix` kebab verbs via a decision table) plus
// the generated reference.md (on-demand tier — the full per-verb reference). The
// whole skills/helix tree is compiled into the binary and shipped to disk by
// installSkill (helix setup), each file written atomically. SKILL.md is the
// on-demand replacement for the preloaded MCP tools/list schema blob: the agent
// learns the verbs from a ≤1,536-char description instead of a full-schema preload
// tax. reference.md is loaded only when the agent reaches for it (progressive
// disclosure) and is therefore EXEMPT from the SKILL-04 idle-cost cap.
//
//go:embed skills/helix/*
var embeddedSkillFS embed.FS

// embeddedSkillBytes returns the verbatim embedded SKILL.md content (ONLY
// SKILL.md, never the bundle). It is the SKILL.md-only accessor every former
// embeddedSkillMD consumer reads — the idle-cost description parser, the body
// accessor, and the in-package tests — so reference.md never bleeds into the idle
// tier (97-RESEARCH.md Pitfall 4).
func embeddedSkillBytes() string {
	b, err := embeddedSkillFS.ReadFile("skills/helix/SKILL.md")
	if err != nil {
		// The file is embedded at build time; a read failure means the embed
		// directive lost SKILL.md, which is a build-time programming error.
		panic("embedded skills/helix/SKILL.md missing: " + err.Error())
	}
	return string(b)
}

// EmbeddedSkillBody returns the verbatim embedded SKILL.md content. It is the
// single source of truth for the helix Agent Skill so external callers (e.g. the
// test/oracle/llm behavioral oracle for TEST-03) can load the skill body into a
// system prompt without duplicating the asset. It returns SKILL.md bytes ONLY (no
// reference.md bleed). The bytes are identical to what installSkill writes to disk
// (modulo a normalizing trailing newline).
func EmbeddedSkillBody() string { return embeddedSkillBytes() }

// skillDescription returns the SKILL.md frontmatter `description` value (joined
// with the optional `when_to_use` value when present). It is the idle-skill-cost
// payload: the only text Claude Code keeps in context until the skill triggers
// (the body below the frontmatter loads on use). SKILL-04 asserts the byte length
// of this string against the Claude Code ≤1,536-char listing cap as a
// dependency-free idle-cost upper bound.
//
// It uses only stdlib string handling — no YAML dependency is added, so the
// zero-dep invariant (`git diff go.mod` empty) holds. The parser is a minimal
// `---`-fence split that understands single-line values and `>-`/`>`/`|`/`|-`
// block scalars (the form the description uses).
func skillDescription() (string, error) {
	fm, ok := skillFrontmatter(embeddedSkillBytes())
	if !ok {
		return "", fmt.Errorf("SKILL.md has no leading --- frontmatter block")
	}
	desc, ok := skillFrontmatterValue(fm, "description")
	if !ok || strings.TrimSpace(desc) == "" {
		return "", fmt.Errorf("SKILL.md frontmatter missing non-empty 'description'")
	}
	if wtu, ok := skillFrontmatterValue(fm, "when_to_use"); ok {
		desc = strings.TrimSpace(desc + " " + wtu)
	}
	return desc, nil
}

// skillFrontmatter splits the leading `---`...`---` YAML block out of the
// embedded SKILL.md, returning the raw frontmatter text and ok=false if the file
// does not open with a `---` fence followed by a closing `---`. Minimal splitter,
// no YAML dependency.
func skillFrontmatter(md string) (string, bool) {
	if !strings.HasPrefix(md, "---\n") && !strings.HasPrefix(md, "---\r\n") {
		return "", false
	}
	rest := md
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	}
	lines := strings.Split(rest, "\n")
	var fm []string
	for _, ln := range lines {
		if strings.TrimRight(ln, "\r") == "---" {
			return strings.Join(fm, "\n"), true
		}
		fm = append(fm, ln)
	}
	return "", false
}

// skillFrontmatterValue extracts a single-line `key:` value or a YAML block
// scalar (`>-` / `>` / `|` / `|-`) from the frontmatter text, concatenating the
// indented continuation lines for block scalars. Returns the trimmed value and ok.
func skillFrontmatterValue(fm, key string) (string, bool) {
	lines := strings.Split(fm, "\n")
	prefix := key + ":"
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if val == ">-" || val == ">" || val == "|" || val == "|-" {
			var parts []string
			for _, cont := range lines[i+1:] {
				if strings.TrimSpace(cont) == "" {
					parts = append(parts, "")
					continue
				}
				if cont[0] != ' ' && cont[0] != '\t' {
					break
				}
				parts = append(parts, strings.TrimSpace(cont))
			}
			return strings.TrimSpace(strings.Join(parts, " ")), true
		}
		return val, true
	}
	return "", false
}

// skillTargetDir resolves the per-client skills directory for the helix skill:
// <claudeDir>/skills/helix (the Claude Code layout — project
// .claude/skills/<name>/SKILL.md or personal ~/.claude/skills/<name>/SKILL.md).
// The output always carries the "skills/helix" segment sequence that
// installSkill's containment check relies on.
func skillTargetDir(claudeDir string) string {
	return filepath.Join(claudeDir, "skills", "helix")
}

// installSkill writes the embedded skill BUNDLE (SKILL.md + reference.md) to
// targetDir, each file atomically (temp file + rename, mirroring nudge.go
// saveSessionStats so a concurrent setup never observes a partial/corrupt file)
// and idempotently (re-running is byte-stable). It creates targetDir (MkdirAll
// 0755) if absent.
//
// Containment (T-93-01, Security V12): installSkill refuses any targetDir that is
// not within the per-client skills root — i.e. a targetDir that does not resolve
// (via filepath.Rel) to a path at or under its own ".../skills/helix" root, with
// no ".." escape. A crafted client/project dir whose ".." components escape that
// root is rejected before any write — the path-traversal analog of render.go
// readSnippetLine's filepath.Rel + ".."-prefix guard. The per-file write loop
// derives each destination name ONLY from the embedded FS (never user input), so
// the multi-file bundle keeps the same path-traversal posture as the single-file
// install (T-97-01).
func installSkill(targetDir string) error {
	if !withinSkillRoot(targetDir) {
		return fmt.Errorf("refusing skill install: target %q is outside the skills/helix root", targetDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating skill directory: %w", err)
	}

	entries, err := embeddedSkillFS.ReadDir("skills/helix")
	if err != nil {
		return fmt.Errorf("reading embedded skill dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := embeddedSkillFS.ReadFile("skills/helix/" + e.Name())
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", e.Name(), err)
		}
		// Single trailing newline (policy matching saveSessionStats), only adding
		// one if absent.
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		// Entry name comes ONLY from the embedded FS (never user-controlled), so
		// the join writes strictly within targetDir (T-97-01).
		dst := filepath.Join(targetDir, e.Name())
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			return fmt.Errorf("writing temp skill file: %w", err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			return fmt.Errorf("renaming skill file: %w", err)
		}
	}
	return nil
}

// withinSkillRoot reports whether targetDir is contained within a per-client
// "skills/helix" root, using a real filepath.Rel containment check rather than a
// segment-substring scan (WR-93-02).
//
// It derives the DECLARED root from the LEXICAL (pre-Clean) path — the prefix up
// to and including the first "skills/helix" segment pair — then requires the
// CLEANED path to resolve at-or-under that declared root (filepath.Rel yields "."
// or a non-".."-escaping subpath). Taking the root from the lexical path and
// comparing the cleaned path is what makes ".." escapes detectable: a target like
// "<root>/skills/helix/../../../etc" declares root "<root>/skills/helix" but
// cleans to a path OUTSIDE it, so Rel returns a "..\"-prefixed result and the
// guard rejects it — the path-traversal analog of render.go readSnippetLine's
// filepath.Rel + ".."-prefix guard. A path that carries no "skills/helix" pair at
// all is rejected outright.
func withinSkillRoot(targetDir string) bool {
	root, ok := lexicalSkillRoot(targetDir)
	if !ok {
		return false
	}
	return containedIn(filepath.Clean(root), filepath.Clean(targetDir))
}

// lexicalSkillRoot returns the ".../skills/helix" root prefix of the LEXICAL
// (uncleaned) path — up to and including the first "skills" then "helix" segment
// pair — and ok=false if the path does not carry that consecutive segment pair.
// Using the uncleaned path is deliberate: it makes a later ".." escape detectable
// when the cleaned path is compared against this declared root.
func lexicalSkillRoot(targetDir string) (string, bool) {
	slashed := filepath.ToSlash(targetDir)
	segs := strings.Split(slashed, "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "skills" && segs[i+1] == "helix" {
			root := strings.Join(segs[:i+2], "/")
			return filepath.FromSlash(root), true
		}
	}
	return "", false
}

// containedIn reports whether target resolves at-or-under root with no ".."
// escape, using filepath.Rel (the same pattern render.go uses for snippet paths).
func containedIn(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// uninstallSkill removes every embedded bundle file (SKILL.md + reference.md) from
// targetDir and, if it then becomes empty, the targetDir itself (WR-93-01). It is
// best-effort and path-contained: it refuses any targetDir outside the per-client
// skills/helix root (same guard as installSkill), and a missing file/dir is a
// no-op (not an error) so --uninstall is idempotent. Only the empty skills/helix
// dir is pruned; parent dirs (skills, .claude) are left intact since they may hold
// other content. Entry names come ONLY from the embedded FS (never user input).
func uninstallSkill(targetDir string) error {
	if !withinSkillRoot(targetDir) {
		return fmt.Errorf("refusing skill uninstall: target %q is outside the skills/helix root", targetDir)
	}

	entries, err := embeddedSkillFS.ReadDir("skills/helix")
	if err != nil {
		return fmt.Errorf("reading embedded skill dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dst := filepath.Join(targetDir, e.Name())
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing skill file: %w", err)
		}
	}

	// Prune the now-(possibly-)empty skills/helix dir. os.Remove only succeeds on
	// an empty directory, so a non-empty dir is left intact; a missing dir is a
	// no-op. Either non-emptiness or absence is fine — swallow both.
	_ = os.Remove(targetDir)
	return nil
}
