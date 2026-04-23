# Phase 39: README Rewrite - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 39-README Rewrite
**Areas discussed:** Product identity & tone, Structure & sections, Content depth, New capabilities coverage

---

## Product Identity & Tone

### Tagline

| Option | Description | Selected |
|--------|-------------|----------|
| Keep current tagline | "Serena is the IDE for your coding agent." — concise, memorable | |
| Emphasize LSP/MCP gateway | "Universal LSP gateway for coding agents" — more technical | |
| Emphasize code intelligence | "Code intelligence platform for MCP" — focuses on what it provides | |

**User's choice:** Keep current tagline AND Emphasize code intelligence (combined — tagline stays, subtitle adds code intelligence positioning)
**Notes:** User initially selected "Keep + LSP/MCP gateway", then revisited and changed to "Keep + code intelligence"

### Tone

| Option | Description | Selected |
|--------|-------------|----------|
| Developer-technical | Terse, code-heavy, minimal marketing language | |
| Product-aware technical | Developer audience with clear value props and benefit statements | ✓ |
| Capability showcase | Lead with demos, examples, before/after | |

**User's choice:** Product-aware technical

### Legacy Note

| Option | Description | Selected |
|--------|-------------|----------|
| Footer one-liner | Single line at the bottom: "Originally inspired by Python Serena." | ✓ |
| Acknowledgements section | Fold into existing Acknowledgements section | |
| No mention in README | Handle legacy framing in CHANGELOG or CONTRIBUTING instead | |

**User's choice:** Footer one-liner

---

## Structure & Sections

### Quick Start Position

| Option | Description | Selected |
|--------|-------------|----------|
| Move up after hero | Hero -> Quick start -> Features/advantages | |
| Move up after advantages | Hero -> How it works -> Advantages -> Quick start | ✓ |
| Keep at bottom | Current position after language/tool tables | |

**User's choice:** Move up after advantages

### Production & Observability

| Option | Description | Selected |
|--------|-------------|----------|
| Keep full section | Signals production-readiness right in README | ✓ |
| Brief mention + link | 2-3 line summary with link to USAGE.md | |
| Remove from README | Production ops details belong in USAGE.md | |

**User's choice:** Keep full section

### Table of Contents

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add TOC | Helps navigation in 250+ line README | ✓ |
| No TOC needed | GitHub auto-generates one in sidebar | |

**User's choice:** Yes, add TOC

---

## Content Depth

### Language & Tool Tables

| Option | Description | Selected |
|--------|-------------|----------|
| Keep both inline | Full tables stay in README with auto-gen markers | ✓ |
| Collapse in details tags | Wrap in `<details>` for expandability | |
| Summary + link | Top 10 inline, link to full tables elsewhere | |

**User's choice:** Keep both inline

### Architecture Detail

| Option | Description | Selected |
|--------|-------------|----------|
| Current level (brief) | 4-layer diagram + 2 paragraphs | ✓ |
| Expand with more detail | Add subsections for each layer (~30 lines) | |
| Minimal + link | Keep diagram only, link to design doc | |

**User's choice:** Current level (brief)

### Quick Start Content

| Option | Description | Selected |
|--------|-------------|----------|
| Setup CLI first (recommended) | `serena setup` as primary, manual configs collapsed/linked | ✓ |
| Both equally | Show setup and manual JSON side by side | |
| Keep manual configs | Current approach with explicit JSON blocks | |

**User's choice:** Setup CLI first

---

## New Capabilities Coverage

### RepoMap Presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated subsection | A 'Code Intelligence' or 'RepoMap' section | ✓ |
| Fold into features table | Just ensure tools table has good descriptions | |
| Mention in advantages | Add a row to the advantages table | |

**User's choice:** Dedicated subsection

### v1.7 Features

| Option | Description | Selected |
|--------|-------------|----------|
| Highlights section | A 'Key Features' section listing standout capabilities | ✓ |
| Weave into existing sections | Mention each in the most relevant existing section | |
| You decide | Claude picks best placement per feature | |

**User's choice:** Highlights section

---

## Claude's Discretion

- Exact subtitle wording
- Highlights section naming and placement
- Manual config presentation in quick start
- RepoMap subsection positioning relative to other sections

## Deferred Ideas

None — discussion stayed within phase scope.
