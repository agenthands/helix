package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// embeddedSkillMD is the Claude Code Agent Skill that teaches an agent the
// frozen Phase 92 `helix` kebab verbs and the terse
// `relpath:line:col<TAB>payload` output via a `| Question | Use this | Not this |`
// decision table. It is compiled into the binary (string form, not embed.FS,
// since SKILL.md is a single file — analog: internal/eval/judge/client.go) and
// written verbatim to disk by installSkill (helix setup). The asset is the
// on-demand replacement for the preloaded MCP tools/list schema blob: the agent
// learns the verbs from a ≤1,536-char description instead of a full-schema
// preload tax.
//
//go:embed skills/helix/SKILL.md
var embeddedSkillMD string

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
	fm, ok := skillFrontmatter(embeddedSkillMD)
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

// installSkill writes the embedded SKILL.md verbatim to <targetDir>/SKILL.md,
// atomically (temp file + rename, mirroring nudge.go saveSessionStats so a
// concurrent setup never observes a partial/corrupt file) and idempotently
// (re-running is byte-stable). It creates targetDir (MkdirAll 0755) if absent.
//
// Containment (T-93-01, Security V12): installSkill refuses any targetDir that is
// not within the per-client skills root, i.e. that does not carry the
// "skills/helix" path-segment sequence that skillTargetDir produces. A crafted
// client/project dir whose ".." components escape that root collapses under
// filepath.Clean and no longer carries the sequence, so it is rejected before
// any write — the path-traversal analog of render.go readSnippetLine's
// filepath.Rel + ".."-prefix guard.
func installSkill(targetDir string) error {
	if !withinSkillRoot(targetDir) {
		return fmt.Errorf("refusing skill install: target %q is outside the skills/helix root", targetDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating skill directory: %w", err)
	}

	// Write embeddedSkillMD with a single trailing newline (policy matching
	// saveSessionStats), only adding one if absent.
	data := []byte(embeddedSkillMD)
	if !strings.HasSuffix(embeddedSkillMD, "\n") {
		data = append(data, '\n')
	}

	dst := filepath.Join(targetDir, "SKILL.md")
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp skill file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("renaming skill file: %w", err)
	}
	return nil
}

// withinSkillRoot reports whether the cleaned targetDir carries the consecutive
// path segments "skills" then "helix" — the root every skillTargetDir output
// produces. A traversal-crafted path whose ".." components escape that root
// collapses under filepath.Clean and no longer carries the sequence, so it is
// rejected. A legitimate nested target (skills/helix/<sub>...) still carries it.
func withinSkillRoot(targetDir string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(targetDir))
	segs := strings.Split(cleaned, "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "skills" && segs[i+1] == "helix" {
			return true
		}
	}
	return false
}
