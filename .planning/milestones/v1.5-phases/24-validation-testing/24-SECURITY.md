---
phase: 24
slug: validation-testing
status: secured
threats_open: 0
asvs_level: 1
created: 2026-04-15
---

# Phase 24 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Agent -> MCP tool handler | Untrusted tool arguments from AI agents cross into tool execution | String arguments (path, query, symbol_name, etc.) |
| Golden file content -> test assertions | Golden files are committed test artifacts; tampering causes false passes | Error text snapshots |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-24-01 | Tampering | Kernel tool handlers | mitigate | Validate empty-string required fields at handler entry before any work begins — 33 checks across 4 files | closed |
| T-24-02 | Tampering | verify_edit handler | mitigate | Added `wsKey.RepoRoot == ""` workspace guard at edit/tools.go:378 before path validation | closed |
| T-24-03 | Tampering | fileops tools | mitigate | Added empty-path check before ValidatePath at fileops/tools.go:99,130,152,259 | closed |
| T-24-04 | Information Disclosure | Error messages | accept | Error messages contain field names but no sensitive data — field names already visible in MCP tool schemas | closed |
| T-24-05 | Repudiation | Golden file updates | accept | Golden files committed to git; diffs visible in code review; GOLDEN_UPDATE=1 requires explicit opt-in | closed |
| T-24-06 | Tampering | extractKind helper | mitigate | Validates against 7 known Kind constant strings; returns empty for unknown formats to prevent false-positive matches | closed |

*Status: open · closed*

---

## Accepted Risks

| Risk ID | Description | Justification | Owner |
|---------|-------------|---------------|-------|
| T-24-04 | Error messages expose field names (path, query, symbol_name) | Field names are documented in MCP tool schemas already visible to agents — no incremental disclosure | Phase 24 |
| T-24-05 | Golden file updates could mask error format regressions | Git history provides full audit trail; GOLDEN_UPDATE=1 is explicit opt-in | Phase 24 |

---

## Audit Trail

### Security Audit 2026-04-15

| Metric | Count |
|--------|-------|
| Threats found | 6 |
| Closed | 6 |
| Open | 0 |

**Evidence:**
- T-24-01: `grep -c 'serr.New(serr.InvalidArgs' internal/kernel/*/tools.go` → symbols:9, edit:14, diag:3, fileops:7
- T-24-02: `grep 'wsKey.RepoRoot == ""' internal/kernel/edit/tools.go` → line 378
- T-24-03: `grep 'args.Path == ""' internal/kernel/fileops/tools.go` → lines 99, 130, 152, 259
- T-24-06: `grep 'case "not_found"' test/integration/errors_test.go` → validates against 7 known constants
