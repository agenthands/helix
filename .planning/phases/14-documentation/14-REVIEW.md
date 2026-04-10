---
phase: 14-documentation
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - CHANGELOG.md
  - Makefile
  - README.md
  - USAGE.md
  - cmd/docgen/main.go
  - cmd/docgen/main_test.go
  - internal/langregistry/registry.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Reviewed the documentation generation tooling (`cmd/docgen`), the language registry (`internal/langregistry/registry.go`), the Makefile, and the output documentation files (README.md, USAGE.md, CHANGELOG.md). The `cmd/docgen` tool is well-structured with good test coverage including idempotency checks. Two logic issues were found in the Go source: a missing marker ordering validation in `replaceSection` that can produce corrupted output, and an inconsistent deep-merge strategy in the registry override logic. The documentation files and Makefile are clean.

## Warnings

### WR-01: replaceSection does not validate marker ordering

**File:** `cmd/docgen/main.go:85-91`
**Issue:** `replaceSection` finds `beginTag` and `endTag` independently via `strings.Index`. If the end marker appears before the begin marker in the content (e.g., due to a malformed README), the function produces silently corrupted output instead of returning an error. The slice `content[:begin]` would include the end marker, and `content[endOfEndTag:]` would start before the begin position.
**Fix:**
```go
func replaceSection(content, beginMarker, endMarker, newContent string) (string, error) {
	beginTag := "<!-- " + beginMarker + " -->"
	endTag := "<!-- " + endMarker + " -->"
	begin := strings.Index(content, beginTag)
	end := strings.Index(content, endTag)
	if begin == -1 || end == -1 {
		return "", fmt.Errorf("markers not found: %s / %s", beginMarker, endMarker)
	}
	if end <= begin {
		return "", fmt.Errorf("end marker %q appears before begin marker %q", endMarker, beginMarker)
	}
	endOfEndTag := end + len(endTag)
	return content[:begin] + beginTag + "\n" + newContent + "\n" + endTag + content[endOfEndTag:], nil
}
```

### WR-02: Inconsistent deep-merge strategy for YAML override maps

**File:** `internal/langregistry/registry.go:139-141` vs `internal/langregistry/registry.go:171-176`
**Issue:** `mergeOverride` performs key-by-key deep merge for `InitOptions` (lines 139-141: iterates keys and sets individually), but `mergeInstallOverride` performs wholesale replacement for `URLs` and `SHA256` maps (lines 171-176: assigns entire map). This means a YAML override that specifies only `urls: { "linux-amd64": "..." }` will silently drop all other platform URLs from the base entry. This inconsistency could cause language server install failures on platforms not listed in the override.
**Fix:**
```go
func mergeInstallOverride(base *InstallInfo, ov *yamlInstallInfo) {
	if ov.Type != nil {
		base.Type = *ov.Type
	}
	if ov.Package != nil {
		base.Package = *ov.Package
	}
	if ov.Version != nil {
		base.Version = *ov.Version
	}
	if ov.URLs != nil {
		if base.URLs == nil {
			base.URLs = make(map[string]string)
		}
		for k, v := range ov.URLs {
			base.URLs[k] = v
		}
	}
	if ov.SHA256 != nil {
		if base.SHA256 == nil {
			base.SHA256 = make(map[string]string)
		}
		for k, v := range ov.SHA256 {
			base.SHA256[k] = v
		}
	}
}
```

## Info

### IN-01: Deprecated strings.Title usage acknowledged but not resolved

**File:** `cmd/docgen/main.go:150-151`
**Issue:** `strings.Title` is deprecated since Go 1.18. The `//nolint:staticcheck` suppression is present and the comment notes it is fine for ASCII keys. This is acceptable for the current use case (language registry keys are ASCII-only) but will trigger warnings if the nolint directive is ever removed or if a non-ASCII language key is added.
**Fix:** Consider replacing with `cases.Title(language.Und).String()` from `golang.org/x/text/cases` when convenient, or document the ASCII-only constraint on language keys.

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
