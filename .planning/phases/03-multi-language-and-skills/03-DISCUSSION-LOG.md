# Phase 3: Multi-Language and Skills - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-08
**Phase:** 03-multi-language-and-skills
**Areas discussed:** Language registry, Memory system, Skill pack interface

---

## Language Registry

**User's choice:** Embedded Go registry, YAML override/extend. Prefer existing LS install, managed download as fallback.

**Notes:** "Ship an embedded registry. Let YAML override or extend entries. Do not make the product depend on an external catalog file at boot." Install strategy: "prefer existing install first, then managed download as fallback — not blind auto-download first."

---

## Memory System

**User's choice:** Markdown + SQLite index (rebuildable). Directory-based scoping with topic metadata.

**Notes:** "Canonical storage: Markdown files. Derived index: lightweight SQLite by default. Loss model: if the index is corrupted or missing, rebuild it from Markdown files." Index contents: name, topic, path, scope, headings, tags, modified time, hash, summary, FTS body search. On-disk contract: `.serena/memories/**/*.md` = canonical, `.serena/index/memories.db` = disposable.

---

## Skill Pack Interface

**User's choice:** Config-driven + Go. Skill = reusable capability package (not just tool bundle).

**Notes:** "Go owns behavior; YAML owns selection and prompting." Two interfaces: Skill/ToolProvider in Go, ContextSpec/ModeSpec in YAML. Tool = atomic MCP operation. Skill = capability package (tool bundle, workflow, or both). "Every tool is protocol-facing; not every skill needs to be."

---

## Claude's Discretion

- Specific LS binary names/URLs for 40+ languages
- SQLite schema details
- Onboarding/handoff workflow specifics

## Deferred Ideas

None.
