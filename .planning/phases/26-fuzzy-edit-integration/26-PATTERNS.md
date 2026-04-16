# Phase 26: Fuzzy Edit Integration - Pattern Map

**Mapped:** 2026-04-16
**Files analyzed:** 9 (new/modified)
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/kernel/fileops/fuzzy_edit.go` | service | file-I/O + transform | `internal/kernel/fileops/replace.go` | exact |
| `internal/kernel/fileops/tools.go` (modify) | controller | request-response | self (registerReplaceInFile) | exact |
| `internal/kernel/fileops/replace.go` (modify) | service | file-I/O + transform | self | exact |
| `internal/kernel/fileops/skill.go` (modify) | config | registration | self | exact |
| `internal/kernel/edit/tools.go` (modify) | controller | request-response | self (registerReplaceBody) | exact |
| `internal/kernel/edit/replace.go` (modify) | service | file-I/O + transform | self | exact |
| `internal/kernel/edit/skill.go` (modify) | config | registration | self | exact |
| `internal/kernel/fileops/fuzzy_edit_test.go` | test | unit | `internal/fuzzy/fuzzy_test.go` | role-match |
| `internal/kernel/edit/replace_fuzzy_test.go` | test | unit | `internal/kernel/edit/edit_test.go` | role-match |

## Pattern Assignments

### `internal/kernel/fileops/fuzzy_edit.go` (NEW - service, file-I/O + transform)

**Analog:** `internal/kernel/fileops/replace.go`

**Imports pattern** (lines 1-8):
```go
package fileops

import (
	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/fuzzy"
)
```

**Core pattern** (replace.go lines 14-56 -- read file, transform, atomic write):
```go
func ReplaceInFile(root, path, pattern, replacement string, isRegex bool) (int, error) {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return 0, err
	}

	// Read existing content
	content, err := ReadFile(root, path)
	if err != nil {
		return 0, err
	}

	// ... transform logic ...

	// Use atomic write via OverwriteFile
	_ = absPath // path already validated
	if err := OverwriteFile(root, path, newContent); err != nil {
		return 0, serr.Wrap(serr.Internal, "writing replacement", err)
	}

	return count, nil
}
```

**New FuzzyEdit function should follow this exact pattern:** ValidatePath -> ReadFile -> fuzzy.Match -> OverwriteFile, returning `(*fuzzy.Result, error)`.

**Error handling pattern** (replace.go lines 16-18, 22-24):
```go
absPath, err := ValidatePath(root, path)
if err != nil {
	return 0, err  // serr typed error from ValidatePath
}
```

---

### `internal/kernel/fileops/tools.go` (MODIFY - add FuzzyEditArgs + registerFuzzyEdit)

**Analog:** self, `registerReplaceInFile` (lines 250-276)

**Args struct pattern** (lines 49-54):
```go
type ReplaceInFileArgs struct {
	Path        string `json:"path" jsonschema:"File path (relative to workspace root)"`
	Pattern     string `json:"pattern" jsonschema:"Pattern to search for (literal or regex)"`
	Replacement string `json:"replacement" jsonschema:"Replacement string"`
	IsRegex     bool   `json:"is_regex,omitempty" jsonschema:"Treat pattern as regex (default false)"`
}
```

**Tool registration pattern** (lines 250-276):
```go
func registerReplaceInFile(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "replace_in_file",
		Description: "Replace all occurrences of a pattern in a file (literal or regex)",
	}, kernel.WrapToolSpan(tracer, "replace_in_file", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceInFileArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("replace_in_file").Error()), nil, nil
		}
		if args.Pattern == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: pattern").
				WithTool("replace_in_file").Error()), nil, nil
		}

		count, err := ReplaceInFile(root, args.Path, args.Pattern, args.Replacement, args.IsRegex)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		return textResult(fmt.Sprintf("%d replacement(s) made in %s", count, args.Path)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "replace_in_file", Description: "Replace all occurrences of a pattern in a file (literal or regex)"})
}
```

**RegisterTools addition pattern** (line 67 -- add `registerFuzzyEdit` call):
```go
func RegisterTools(server *mcp.SerenaMCPServer, workspaceRoot func() string, tracer trace.Tracer) {
	registerReadFile(server, workspaceRoot, tracer)
	// ... existing ...
	registerReplaceInFile(server, workspaceRoot, tracer)
	registerFuzzyEdit(server, workspaceRoot, tracer)  // NEW
}
```

**Fuzzy fallback wiring in registerReplaceInFile** -- after `ReplaceInFile` returns count==0 and `!args.IsRegex`, call `fuzzy.Match` directly in the handler (as recommended by RESEARCH.md to avoid changing `ReplaceInFile` return type).

---

### `internal/kernel/fileops/replace.go` (MODIFY - no changes to ReplaceInFile signature)

Per RESEARCH.md recommendation: fuzzy fallback for `replace_in_file` is handled entirely in the **tool handler** (`registerReplaceInFile` in tools.go), not in the `ReplaceInFile` function itself. This avoids changing the tested `(int, error)` signature. The `replace.go` file may not need modification at all -- the fuzzy fallback logic lives in tools.go.

---

### `internal/kernel/fileops/skill.go` (MODIFY - add fuzzy_edit to Tools list)

**Analog:** self (lines 29-38)

**Pattern** -- add one entry to the `Tools()` slice:
```go
func (s *FileOpsSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		// ... existing 6 tools ...
		{Name: "fuzzy_edit", Description: "Fuzzy-match and replace text in a file using 4-strategy cascade"},
	}
}
```

Also update Description string (line 21) and comment (line 29) to say "7 file operation tools".

---

### `internal/kernel/edit/tools.go` (MODIFY - add SearchBody to ReplaceBodyArgs, update registerReplaceBody handler)

**Analog:** self, `registerReplaceBody` (lines 140-186)

**ReplaceBodyArgs modification** (lines 21-25 -- add SearchBody field):
```go
type ReplaceBodyArgs struct {
	Path       string `json:"path" jsonschema:"File path"`
	SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol whose body to replace"`
	NewBody    string `json:"new_body" jsonschema:"New body content to replace with"`
	SearchBody string `json:"search_body,omitempty" jsonschema:"Optional search text to fuzzy-match within body before replacing"`
}
```

**Handler modification point** (lines 178-183 -- after `ReplaceBodyWithPlan`):
When `args.SearchBody` is provided, the handler must pass it through to `ReplaceBodyWithPlan` (or a new variant). The response format changes to include strategy/score when fuzzy was used:
```go
// Current:
text := fmt.Sprintf("Replaced body of %q in %s", args.SymbolName, args.Path)
// With fuzzy, when SearchBody was used:
text := fmt.Sprintf("Replaced body of %q in %s\nmatch_strategy: %s\nsimilarity_score: %.2f",
	args.SymbolName, args.Path, result.Strategy, result.Score)
```

---

### `internal/kernel/edit/replace.go` (MODIFY - add fuzzy fallback in ReplaceBodyWithPlan)

**Analog:** self (lines 26-69)

**Key integration point** (lines 37-46 -- after tree-sitter body extraction):
```go
// Try tree-sitter body extraction first (D-14).
if extractor != nil && extractor.SupportsLanguage(lang) {
	startByte, endByte, err = extractor.ExtractBody(source, lang, plan.SymbolName, plan.Range)
	if err != nil {
		// Fall back to full symbol range (D-15).
		startByte, endByte = rangeToByteOffsets(source, plan.Range)
	}
} else {
	// No tree-sitter grammar: fall back to full symbol range (D-15).
	startByte, endByte = rangeToByteOffsets(source, plan.Range)
}
```

**Fuzzy match insertion point** -- after body extraction succeeds (startByte/endByte determined), if a `searchBody` parameter is provided, fuzzy-match within `source[startByte:endByte]`:
```go
bodyText := string(source[startByte:endByte])
fResult, fErr := fuzzy.Match(bodyText, searchBody, fuzzy.Options{
	Replacement:   plan.NewContent,
	AllowEllipsis: false,
})
if fErr != nil {
	return nil, fErr
}
// Translate body-relative offsets to absolute file offsets
absStart := int(startByte) + fResult.StartByte
absEnd := int(startByte) + fResult.EndByte
```

**Byte replacement pattern** (lines 53-57):
```go
var result []byte
result = append(result, source[:startByte]...)
result = append(result, []byte(plan.NewContent)...)
result = append(result, source[endByte:]...)
```

**Atomic write + LS notification pattern** (lines 59-68):
```go
if err := os.WriteFile(filePath, result, 0644); err != nil {
	return serr.Wrap(serr.Internal, "write file", err).WithDetail(filePath)
}
if err := notifyDidChange(ctx, lease, plan.URI, string(result)); err != nil {
	return serr.Wrap(serr.Internal, "didChange notification", err)
}
```

**Signature change consideration:** `ReplaceBodyWithPlan` currently returns `error`. To report strategy/score (D-07), it needs to return a richer type (e.g., `(*FuzzyMatchInfo, error)`) or accept the searchBody as a parameter and return match metadata. The handler in tools.go consumes this to format the response.

---

### `internal/kernel/edit/skill.go` (MODIFY - no tool count change needed)

The `replace_symbol_body` tool name stays the same -- it just gains a new optional parameter. No changes needed to the `Tools()` list. Update Description comment if desired.

---

### `internal/kernel/fileops/fuzzy_edit_test.go` (NEW - test)

**Analog:** `internal/fuzzy/fuzzy_test.go` (test structure) + `internal/kernel/fileops/fileops_test.go` (package conventions)

**Test imports pattern** (fuzzy_test.go lines 1-10):
```go
package fuzzy

import (
	"testing"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**File-based test pattern** (fileops_test.go lines 37-49 -- use t.TempDir):
```go
func TestValidatePathAcceptsValidPath(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644)

	resolved, err := ValidatePath(root, "test.txt")
	// ...
}
```

**Test should:** Create temp dir, write fixture file, call `FuzzyEdit(root, path, search, replacement, allowEllipsis)`, verify file contents were changed, verify returned `Result` has expected Strategy/Score.

---

### `internal/kernel/edit/replace_fuzzy_test.go` (NEW - test)

**Analog:** `internal/kernel/edit/edit_test.go` (lines 1-9 package and imports)

**Package and imports pattern:**
```go
package edit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**Table-driven test pattern** (edit_test.go lines 11-56):
```go
func TestRangeToByteOffsets(t *testing.T) {
	tests := []struct {
		name      string
		// ... fields ...
	}{
		{ name: "case 1", /* ... */ },
		{ name: "case 2", /* ... */ },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ... assertions ...
		})
	}
}
```

---

## Shared Patterns

### Tool Registration
**Source:** `internal/kernel/fileops/tools.go` lines 250-276
**Apply to:** `registerFuzzyEdit` (new), `registerReplaceInFile` (modify for fallback)

Three-step registration:
1. `mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name, Description}, kernel.WrapToolSpan(tracer, name, handler))`
2. Inside handler: `rootFn()` check -> arg validation with `serr.New(serr.InvalidArgs, ...)` -> business logic call -> `textResult()` or `errorResult()`
3. `server.Registry().Register(&mcp.ToolDef{Name, Description})`

### Error Handling
**Source:** `internal/kernel/fileops/replace.go` lines 16-18, 51-53
**Apply to:** All new/modified files

```go
// Wrap errors with serr typed codes
if err := OverwriteFile(root, path, newContent); err != nil {
	return 0, serr.Wrap(serr.Internal, "writing replacement", err)
}
```

### Response Formatting (D-07/D-08)
**Source:** `internal/kernel/fileops/tools.go` lines 69-84
**Apply to:** `registerFuzzyEdit`, `registerReplaceInFile` (fuzzy path), `registerReplaceBody` (fuzzy path)

When exact match succeeds: use existing `textResult()` format (no strategy/score).
When fuzzy match used: include `match_strategy` and `similarity_score` in text response:
```go
text := fmt.Sprintf("Fuzzy edit applied to %s\nmatch_strategy: %s\nsimilarity_score: %.2f",
	path, result.Strategy, result.Score)
return textResult(text), nil, nil
```

### Fuzzy Engine Invocation
**Source:** `internal/fuzzy/match.go` line 24
**Apply to:** `fuzzy_edit.go`, `tools.go` (replace_in_file fallback), `replace.go` (replace_symbol_body fallback)

```go
result, err := fuzzy.Match(content, search, fuzzy.Options{
	Replacement:   replacement,
	AllowEllipsis: allowEllipsis,  // true for fuzzy_edit, false for fallback paths
})
if err != nil {
	return nil, err  // serr.InvalidArgs from engine
}
newContent := content[:result.StartByte] + result.ReplacementText + content[result.EndByte:]
```

### Skill Tools() List
**Source:** `internal/kernel/fileops/skill.go` lines 29-38
**Apply to:** `skill.go` (add `fuzzy_edit` entry)

```go
{Name: "fuzzy_edit", Description: "Fuzzy-match and replace text in a file using 4-strategy cascade"},
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | All files have direct analogs in the same package |

## Metadata

**Analog search scope:** `internal/kernel/fileops/`, `internal/kernel/edit/`, `internal/fuzzy/`
**Files scanned:** 15
**Pattern extraction date:** 2026-04-16
