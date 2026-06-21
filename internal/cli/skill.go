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
