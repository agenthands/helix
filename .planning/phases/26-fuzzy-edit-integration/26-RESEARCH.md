# Phase 26: Fuzzy Edit Integration - Research

**Researched:** 2026-04-16
**Domain:** MCP tool integration, fuzzy matching fallback wiring, Go
**Confidence:** HIGH

## Summary

Phase 26 integrates the Phase 25 fuzzy engine (`internal/fuzzy/`) into three touchpoints: (1) a new standalone `fuzzy_edit` MCP tool in `internal/kernel/fileops/`, (2) a fuzzy fallback in `replace_symbol_body` when exact body match fails, and (3) a fuzzy fallback in `replace_in_file` (note: requirements say `replace_content` but the Go tool is `replace_in_file`) when literal string match returns zero hits. No changes to the fuzzy engine itself are needed.

The codebase has clear, well-established patterns for all three integration points. Tool registration follows the `mcpsdk.AddTool` + `kernel.WrapToolSpan` + `server.Registry().Register` pattern. The fuzzy engine's `Match(source, search, opts)` API is pure (no I/O), returning `*Result` with `Strategy`, `Score`, `StartByte`, `EndByte`, `MatchedText`, and `ReplacementText`. All three integrations follow the same pattern: read file content, call `fuzzy.Match()`, use `Result.ReplacementText` to substitute, write via existing atomic write utilities.

**Primary recommendation:** Wire `fuzzy.Match()` into existing tool handlers with minimal code changes. The engine is designed for exactly this integration pattern -- callers read, match, decide, write.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Auto-fallback -- when exact matching fails in `replace_symbol_body` or `replace_in_file`, automatically try fuzzy matching. No agent-side opt-in needed. Report `match_strategy` in response so agents know fuzzy was used.
- **D-02:** `replace_symbol_body` gets a new optional `search_body` parameter. When provided AND tree-sitter body extraction succeeds, fuzzy-match `search_body` within the extracted body region, then apply `new_body` as replacement. When `search_body` is absent, behavior is unchanged (full body replace via tree-sitter range).
- **D-03:** `replace_in_file` fuzzy fallback activates ONLY when `is_regex=false` and literal string match returns 0 results. Regex failures stay as-is.
- **D-04:** `fuzzy_edit` parameters: `path` (string, required), `search` (string, required), `replacement` (string, required), `allow_ellipsis` (bool, default true).
- **D-05:** Tool lives in `internal/kernel/fileops/` alongside `replace_in_file`. It operates on raw file content without symbol awareness.
- **D-06:** Tool reads file, calls `fuzzy.Match()`, writes result if match succeeds. Atomic write via existing `OverwriteFile`. Reports strategy/score in response.
- **D-07:** All three tools include `match_strategy` and `similarity_score` as structured fields in the JSON response when fuzzy matching is used.
- **D-08:** When exact matching succeeds (no fuzzy fallback needed), existing response format is preserved.

### Claude's Discretion
- Error message formatting for fuzzy fallback failures
- Whether `fuzzy_edit` response includes the matched text region for agent inspection
- Integration test structure and fixture design

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FUZZ-04 | Agent can use standalone `fuzzy_edit` MCP tool for raw text fuzzy matching independent of symbol boundaries | New `registerFuzzyEdit` in `internal/kernel/fileops/tools.go`, new `FuzzyEditArgs` struct, `fuzzy.Match()` integration, `OverwriteFile` for atomic write |
| FUZZ-05 | `replace_symbol_body` falls back to fuzzy matching within tree-sitter-located body when exact match fails | Add `search_body` param to `ReplaceBodyArgs`, modify `ReplaceBodyWithPlan` to fuzzy-match within extracted body region |
| FUZZ-06 | `replace_in_file` falls back to fuzzy matching when exact/regex match fails | Modify `ReplaceInFile` to call `fuzzy.Match()` when `isRegex=false` and `strings.Count==0` |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| fuzzy_edit standalone tool | API / Backend (fileops) | -- | Pure file operation, no symbol/LSP awareness needed |
| replace_symbol_body fallback | API / Backend (edit) | -- | Operates within tree-sitter-extracted body region, same tier as existing tool |
| replace_in_file fallback | API / Backend (fileops) | -- | Extends existing literal replace path, same tier |
| Profile registration | API / Backend (profile) | -- | Tool visibility controlled by profile/mode YAML |
| MCP response formatting | API / Backend (tool handlers) | -- | Strategy/score fields added at handler boundary |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/fuzzy` | local | 4-strategy cascade matcher | Phase 25 output, designed for this integration |
| `internal/kernel/fileops` | local | File operations, atomic write | Existing tool home for `fuzzy_edit` |
| `internal/kernel/edit` | local | Symbol editing with tree-sitter | Existing tool home for FUZZ-05 |
| `internal/errors` (serr) | local | Typed errors | Project convention for all error paths |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go-tree-sitter` | existing | Body extraction | Already used by `ReplaceBodyWithPlan`, no new dependency |
| `mcpsdk` (MCP Go SDK) | existing | Tool registration | `mcpsdk.AddTool` pattern for `fuzzy_edit` |

No new external dependencies required. [VERIFIED: codebase inspection]

## Architecture Patterns

### System Architecture Diagram

```
Agent (MCP client)
  |
  v
MCP Tool Router (daemon)
  |
  +---> fuzzy_edit --------> fileops.ReadFile -> fuzzy.Match() -> fileops.OverwriteFile
  |                                                                     |
  +---> replace_in_file ---> fileops.ReplaceInFile                      |
  |       (literal match fails, is_regex=false)                         |
  |         \---> fuzzy.Match() fallback -> OverwriteFile               |
  |                                                                     |
  +---> replace_symbol_body -> PlanEdit (LSP) -> ReplaceBodyWithPlan    |
          (search_body provided, exact match fails within body)         |
            \---> fuzzy.Match(bodyRegion, search_body) -> byte replace  |
                                                                        |
  All paths: strategy/score in response <-------------------------------+
```

### Recommended Project Structure (changes only)
```
internal/kernel/
  fileops/
    tools.go          # Add FuzzyEditArgs, registerFuzzyEdit
    replace.go        # Modify ReplaceInFile for fuzzy fallback (or new func)
    fuzzy_edit.go     # NEW: FuzzyEdit function (read + match + write)
    fuzzy_edit_test.go # NEW: Unit tests for FuzzyEdit
    skill.go          # Add fuzzy_edit to Tools() list (7 tools now)
  edit/
    tools.go          # Add SearchBody to ReplaceBodyArgs, update handler
    replace.go        # Add fuzzy fallback in ReplaceBodyWithPlan
    replace_test.go   # NEW or extend: tests for fuzzy fallback path
```

### Pattern 1: Standalone fuzzy_edit Tool

**What:** New MCP tool that reads a file, runs `fuzzy.Match()`, writes result via `OverwriteFile`.
**When to use:** Agent wants to do arbitrary text replacement without symbol context.
**Example:**
```go
// Source: codebase pattern from registerReplaceInFile in fileops/tools.go
type FuzzyEditArgs struct {
    Path          string `json:"path" jsonschema:"File path (relative to workspace root)"`
    Search        string `json:"search" jsonschema:"Text to search for (fuzzy matched)"`
    Replacement   string `json:"replacement" jsonschema:"Replacement text"`
    AllowEllipsis bool   `json:"allow_ellipsis,omitempty" jsonschema:"Allow ... ellipsis segmentation (default true)"`
}

func FuzzyEdit(root, path, search, replacement string, allowEllipsis bool) (*fuzzy.Result, error) {
    content, err := ReadFile(root, path)
    if err != nil {
        return nil, err
    }
    result, err := fuzzy.Match(content, search, fuzzy.Options{
        Replacement:   replacement,
        AllowEllipsis: allowEllipsis,
    })
    if err != nil {
        return nil, err
    }
    // Apply replacement
    newContent := content[:result.StartByte] + result.ReplacementText + content[result.EndByte:]
    if err := OverwriteFile(root, path, newContent); err != nil {
        return nil, serr.Wrap(serr.Internal, "writing fuzzy edit", err)
    }
    return result, nil
}
```
[VERIFIED: codebase patterns from fileops/replace.go and fuzzy/match.go]

### Pattern 2: replace_in_file Fuzzy Fallback (FUZZ-06)

**What:** When `isRegex=false` and `strings.Count(content, pattern)==0`, try `fuzzy.Match()` before returning zero replacements.
**When to use:** Automatic fallback path in existing tool.
**Example:**
```go
// In ReplaceInFile, after literal match returns count==0 and isRegex==false:
if !isRegex && count == 0 {
    result, err := fuzzy.Match(content, pattern, fuzzy.Options{
        Replacement:   replacement,
        AllowEllipsis: false, // replace_in_file is literal-oriented
    })
    if err != nil {
        // Fuzzy also failed -- return 0 replacements (or the error)
        return 0, nil
    }
    newContent := content[:result.StartByte] + result.ReplacementText + content[result.EndByte:]
    if err := OverwriteFile(root, path, newContent); err != nil {
        return 0, serr.Wrap(serr.Internal, "writing replacement", err)
    }
    // Return 1 replacement + result metadata for handler to format response
    return 1, nil // handler needs Result for strategy/score reporting
}
```
**Design note:** `ReplaceInFile` currently returns `(int, error)`. To report strategy/score (D-07), the function signature likely needs to return a richer result type, or the handler must call fuzzy.Match directly. Recommend creating a `ReplaceInFileResult` struct or having the handler call `fuzzy.Match` in the tool handler itself (keeping `ReplaceInFile` for the exact-match path).
[VERIFIED: codebase analysis of fileops/replace.go]

### Pattern 3: replace_symbol_body Fuzzy Fallback (FUZZ-05)

**What:** Add optional `search_body` param to `ReplaceBodyArgs`. When provided, after tree-sitter extracts the body region, fuzzy-match `search_body` within that region and replace.
**When to use:** Agent provides `search_body` AND exact match fails within the body.
**Example flow:**
```
1. PlanEdit locates symbol via LSP (unchanged)
2. ExtractBody gets body byte range via tree-sitter (unchanged)
3. NEW: If search_body is provided:
   a. Extract body text: source[startByte:endByte]
   b. fuzzy.Match(bodyText, searchBody, opts{Replacement: newBody})
   c. If match succeeds: calculate absolute offsets = startByte + result.StartByte
   d. Replace source[absStart:absEnd] with result.ReplacementText
4. If search_body is absent: existing behavior (full body replace)
```
**Key insight:** The fuzzy match runs against the body sub-string, but byte offsets in the write must be translated back to absolute file positions by adding the body's `startByte` offset.
[VERIFIED: codebase analysis of edit/replace.go]

### Anti-Patterns to Avoid
- **Modifying fuzzy engine for integration:** The engine is pure and complete. All integration happens in tool handlers and business logic functions.
- **Returning fuzzy Result through existing return types:** `ReplaceInFile` returns `(int, error)` which cannot carry strategy/score. Don't force it; create new return types or handle in the tool handler layer.
- **Fuzzy fallback on regex patterns:** D-03 explicitly forbids this. Guard with `!isRegex` check.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Text fuzzy matching | Custom diff/match logic | `fuzzy.Match()` | Phase 25 engine handles cascade, ambiguity, indentation reflow |
| Atomic file write | `os.WriteFile` directly | `fileops.OverwriteFile` | Temp file + rename prevents partial writes |
| Path validation | Manual path joining | `fileops.ValidatePath` | Handles symlinks, traversal attacks, workspace containment |
| File reading with size limits | Raw `os.ReadFile` | `fileops.ReadFile` | 10MB limit, existence checks, directory rejection |

## Common Pitfalls

### Pitfall 1: Tool Name Mismatch (replace_content vs replace_in_file)
**What goes wrong:** Requirements and profiles reference `replace_content` but the Go tool is `replace_in_file`.
**Why it happens:** Legacy Python name persists in requirements docs and profile YAML exclusion lists.
**How to avoid:** Always use `replace_in_file` in Go code. The profile YAMLs already exclude `replace_content` (a no-op since no tool with that name exists); no changes needed there.
**Warning signs:** Grep for `replace_content` in Go source -- should return zero hits outside profile YAMLs.

### Pitfall 2: Byte Offset Translation in replace_symbol_body
**What goes wrong:** Fuzzy match returns offsets relative to the body sub-string, but file write needs absolute offsets.
**Why it happens:** `fuzzy.Match()` operates on the string you pass it. If you pass the body region, offsets are body-relative.
**How to avoid:** Add `startByte` (from tree-sitter extraction) to `result.StartByte` and `result.EndByte` when calculating the replacement range in the full file.
**Warning signs:** Replacement appears at wrong position in file, or file corruption.

### Pitfall 3: Response Format Backward Compatibility (D-08)
**What goes wrong:** Adding `match_strategy`/`similarity_score` to all responses breaks agents that parse exact-match responses.
**Why it happens:** Eagerly adding fields to every response path.
**How to avoid:** Only include strategy/score fields when fuzzy matching was actually used. When exact matching succeeds (the happy path), preserve the existing text-only response format.
**Warning signs:** Agents that parse `replace_symbol_body` responses start seeing unexpected JSON fields.

### Pitfall 4: AllowEllipsis Default for fuzzy_edit vs replace_in_file
**What goes wrong:** Using `AllowEllipsis=true` in `replace_in_file` fallback when users provide literal `...` in their search pattern.
**Why it happens:** `fuzzy_edit` defaults to `AllowEllipsis=true` (D-04) but `replace_in_file` is literal-oriented.
**How to avoid:** `replace_in_file` fallback should use `AllowEllipsis: false`. Only `fuzzy_edit` uses the user-specified value (defaulting true).
**Warning signs:** `replace_in_file` incorrectly segments patterns containing literal `...`.

### Pitfall 5: ReplaceInFile Return Type Limitation
**What goes wrong:** Cannot report `match_strategy`/`similarity_score` because `ReplaceInFile` returns `(int, error)`.
**Why it happens:** The function was designed for multi-occurrence literal/regex replace, not single fuzzy match.
**How to avoid:** Either (a) create a richer return type `ReplaceInFileResult` with optional strategy/score fields, or (b) handle fuzzy fallback entirely in the tool handler (call `fuzzy.Match` directly when `ReplaceInFile` returns count=0).
**Warning signs:** Strategy/score fields missing from response despite fuzzy match being used.

## Code Examples

### fuzzy_edit Tool Registration
```go
// Source: pattern from fileops/tools.go registerReplaceInFile
func registerFuzzyEdit(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "fuzzy_edit",
        Description: "Fuzzy-match and replace text in a file using 4-strategy cascade (exact, whitespace-normalized, indentation-flexible)",
    }, kernel.WrapToolSpan(tracer, "fuzzy_edit", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FuzzyEditArgs) (*mcpsdk.CallToolResult, any, error) {
        root := rootFn()
        if root == "" {
            return noWorkspaceError(), nil, nil
        }
        // validate args...
        result, err := FuzzyEdit(root, args.Path, args.Search, args.Replacement, args.AllowEllipsis)
        if err != nil {
            return errorResult(err.Error()), nil, nil
        }
        text := fmt.Sprintf("Fuzzy edit applied to %s\nmatch_strategy: %s\nsimilarity_score: %.2f",
            args.Path, result.Strategy, result.Score)
        return textResult(text), nil, nil
    }))
    server.Registry().Register(&mcp.ToolDef{
        Name: "fuzzy_edit",
        Description: "Fuzzy-match and replace text in a file using 4-strategy cascade",
    })
}
```
[VERIFIED: follows exact pattern from fileops/tools.go lines 250-276]

### replace_symbol_body with search_body Param
```go
// Updated ReplaceBodyArgs
type ReplaceBodyArgs struct {
    Path       string `json:"path" jsonschema:"File path"`
    SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol whose body to replace"`
    NewBody    string `json:"new_body" jsonschema:"New body content to replace with"`
    SearchBody string `json:"search_body,omitempty" jsonschema:"Optional search text to fuzzy-match within body before replacing"`
}
```
[VERIFIED: follows existing struct patterns in edit/tools.go]

### Fuzzy Fallback in replace_in_file Handler
```go
// In the tool handler, after ReplaceInFile returns count==0:
count, err := ReplaceInFile(root, args.Path, args.Pattern, args.Replacement, args.IsRegex)
if err != nil {
    return errorResult(err.Error()), nil, nil
}
if count == 0 && !args.IsRegex {
    // Fuzzy fallback (D-03: only when is_regex=false)
    content, readErr := ReadFile(root, args.Path)
    if readErr != nil {
        return textResult("0 replacement(s) made in " + args.Path), nil, nil
    }
    fResult, fErr := fuzzy.Match(content, args.Pattern, fuzzy.Options{
        Replacement:   args.Replacement,
        AllowEllipsis: false,
    })
    if fErr != nil {
        return errorResult(fErr.Error()), nil, nil
    }
    newContent := content[:fResult.StartByte] + fResult.ReplacementText + content[fResult.EndByte:]
    if wErr := OverwriteFile(root, args.Path, newContent); wErr != nil {
        return errorResult(wErr.Error()), nil, nil
    }
    text := fmt.Sprintf("1 replacement made in %s (fuzzy)\nmatch_strategy: %s\nsimilarity_score: %.2f",
        args.Path, fResult.Strategy, fResult.Score)
    return textResult(text), nil, nil
}
```
[VERIFIED: pattern derived from fileops/tools.go and fuzzy/match.go APIs]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Exact-only body replace | Fuzzy fallback with search_body | Phase 26 | Agents can tolerate minor whitespace/indent differences |
| Exact/regex-only file replace | Fuzzy fallback for literals | Phase 26 | Fewer "0 replacements" failures for agents |
| No standalone fuzzy tool | `fuzzy_edit` MCP tool | Phase 26 | Agents can fuzzy-edit without symbol context |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `AllowEllipsis` should default to `false` for `replace_in_file` fallback | Pitfall 4 | Literal `...` in search patterns could be misinterpreted as segment delimiters |
| A2 | Profile YAMLs need `fuzzy_edit` added to appropriate skill tool lists | Architecture Patterns | Tool won't appear in profile-filtered tool sets |

## Open Questions

1. **replace_in_file fuzzy fallback: handler-level or function-level?**
   - What we know: `ReplaceInFile` returns `(int, error)` which can't carry strategy/score. D-07 requires strategy/score in response.
   - What's unclear: Whether to refactor `ReplaceInFile` signature or handle fuzzy in the tool handler.
   - Recommendation: Handle fuzzy fallback in the tool handler (simpler, avoids signature change of tested function). The handler already has access to `ReadFile` and `OverwriteFile`.

2. **replace_symbol_body: should search_body fuzzy also support AllowEllipsis?**
   - What we know: D-02 says fuzzy-match `search_body` within body region. D-04 gives `fuzzy_edit` an `allow_ellipsis` param.
   - What's unclear: Whether `replace_symbol_body` should also expose `allow_ellipsis`.
   - Recommendation: No -- keep `replace_symbol_body` simple. Agents wanting ellipsis use `fuzzy_edit` instead. Pass `AllowEllipsis: false` in the fallback.

3. **Profile YAML updates: which profiles get fuzzy_edit?**
   - What we know: `fuzzy_edit` is a fileops tool. `file-ops` skill is in `full` profile and `edit` mode. Claude-code and codex exclude `replace_content` (legacy name).
   - What's unclear: Whether claude-code/codex profiles should include `fuzzy_edit` since they have `symbol-editing` skill (and `replace_symbol_body` gets fuzzy fallback automatically).
   - Recommendation: Add `fuzzy_edit` to `file-ops` skill tool list (it's a fileops tool). Claude-code/codex profiles that include `symbol-editing` will get the FUZZ-05 fallback automatically. `fuzzy_edit` availability depends on whether `file-ops` skill is in their skill list. Currently claude-code and codex don't include `file-ops`, so `fuzzy_edit` won't be available to them (which is correct -- they handle file ops natively).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify |
| Config file | none (go test convention) |
| Quick run command | `go test ./internal/kernel/fileops/... ./internal/kernel/edit/... -run Fuzzy -v` |
| Full suite command | `go test ./internal/kernel/fileops/... ./internal/kernel/edit/... ./internal/fuzzy/... -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FUZZ-04 | fuzzy_edit reads file, matches, writes, reports strategy | unit | `go test ./internal/kernel/fileops/... -run TestFuzzyEdit -v` | Wave 0 |
| FUZZ-05 | replace_symbol_body falls back to fuzzy when search_body provided and exact fails | unit | `go test ./internal/kernel/edit/... -run TestReplaceBodyFuzzyFallback -v` | Wave 0 |
| FUZZ-06 | replace_in_file falls back to fuzzy when literal match fails | unit | `go test ./internal/kernel/fileops/... -run TestReplaceInFileFuzzyFallback -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/kernel/fileops/... ./internal/kernel/edit/... -v`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go vet ./... && go test ./... -count=1`

### Wave 0 Gaps
- [ ] `internal/kernel/fileops/fuzzy_edit_test.go` -- covers FUZZ-04
- [ ] `internal/kernel/edit/replace_fuzzy_test.go` -- covers FUZZ-05
- [ ] `internal/kernel/fileops/replace_fuzzy_test.go` -- covers FUZZ-06

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task**
- **Language:** Go, single binary
- **Tool registration:** `RegisterTools()` with typed args + `mcpsdk.AddTool`
- **Error handling:** `serr` typed errors (`serr.InvalidArgs`, `serr.Internal`, etc.)
- **Tracing:** `kernel.WrapToolSpan` for all tool handlers
- **Skill registration:** `skill.Register()` in `init()`, tools listed in skill's `Tools()` method
- **Atomic writes:** Use `OverwriteFile` (temp file + rename)
- **Build verification:** `go build ./cmd/serena`

## Sources

### Primary (HIGH confidence)
- `internal/fuzzy/types.go` -- Strategy, Options, Result API surface [VERIFIED: direct file read]
- `internal/fuzzy/match.go` -- Match() entry point, cascade logic [VERIFIED: direct file read]
- `internal/kernel/edit/replace.go` -- ReplaceBodyWithPlan implementation [VERIFIED: direct file read]
- `internal/kernel/edit/tools.go` -- ReplaceBodyArgs, registerReplaceBody [VERIFIED: direct file read]
- `internal/kernel/fileops/replace.go` -- ReplaceInFile implementation [VERIFIED: direct file read]
- `internal/kernel/fileops/tools.go` -- Tool registration pattern [VERIFIED: direct file read]
- `internal/kernel/fileops/skill.go` -- FileOpsSkill tool list [VERIFIED: direct file read]
- `internal/kernel/edit/skill.go` -- SymbolEditingSkill tool list [VERIFIED: direct file read]
- `internal/profile/profiles/*.yaml` -- Profile tool inclusion/exclusion [VERIFIED: direct file read]
- `internal/daemon/daemon.go` -- Daemon wiring of RegisterTools [VERIFIED: grep]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries are local/existing, no new dependencies
- Architecture: HIGH -- integration points are well-understood from code inspection
- Pitfalls: HIGH -- identified from direct code analysis of existing patterns

**Research date:** 2026-04-16
**Valid until:** 2026-05-16 (stable internal codebase, no external dependency changes)
