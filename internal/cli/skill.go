package cli

import _ "embed"

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
