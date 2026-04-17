# Phase 30: RepoMap Pipeline Wiring - Research

**Researched:** 2026-04-17
**Domain:** Go integration wiring -- connecting existing components into a working data pipeline
**Confidence:** HIGH

## Summary

Phase 30 is purely an integration/wiring phase. All components exist and pass unit tests individually: TagExtractor, FallbackExtractor, TagCache, FileGraph, PageRank, EnrichFromLSP, TreeRenderer, ElisionRenderer, and the RepoMapSkill MCP tools. The problem is that no production code instantiates the extractors, walks the workspace to populate the cache, calls EnrichFromLSP, or delegates to TreeRenderer. The skill uses inline rendering copies instead of the shared TreeRenderer, and the rootDir is derived from os.Getwd() rather than the activated workspace path.

The fix is surgical: modify `RepoMapSkill.Init()` and the tool execution methods to wire the pipeline. No new packages, no new dependencies, no architectural changes. The main challenges are (1) getting the workspace root into the skill (SkillDeps.ProjectDir is `~/.serena/default-project`, not the workspace), (2) walking the workspace to populate TagCache via TagExtractor/FallbackExtractor, and (3) replacing inline rendering with TreeRenderer.

**Primary recommendation:** Wire the existing components in RepoMapSkill with a lazy workspace-walk triggered on first tool call, delegate rendering to TreeRenderer, and hook EnrichFromLSP into ensureGraph when a kernel reference is available.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RMAP-04 | Cross-file reference graph built from extracted tags | Graph building exists in `repomap.BuildGraph()` -- needs cache populated with real workspace data via TagExtractor walk |
| RMAP-05 | Personalized PageRank ranks symbol importance | PageRank exists in `FileGraph.PageRank()` / `RankFiles()` -- works once graph has data |
| RMAP-06 | get_repo_map returns token-budgeted ranked overview | Tool dispatch works; fix = populate cache + delegate to TreeRenderer.RenderBudgeted |
| RMAP-07 | get_context returns task-focused context | Same as RMAP-06 but with personalization vector from seed files |
| RMAP-08 | Warm LSP sessions enrich reference graph | `EnrichFromLSP()` exists and tested -- needs production call site with kernel pool access |
| RMAP-10 | Token budget controls output size via binary search | TreeRenderer.RenderBudgeted has 15% tolerance binary search; skill's inline copy lacks it -- replace inline with TreeRenderer |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Workspace file walking | RepoMapSkill (skill layer) | -- | Skill owns cache population timing and workspace root resolution |
| Tag extraction (tree-sitter) | TagExtractor (repomap pkg) | -- | Already built, just needs instantiation |
| Tag extraction (LSP fallback) | FallbackExtractor (repomap pkg) | Kernel worker pool | Needs LSP session for documentSymbol requests |
| Cache population | TagCache (repomap pkg) | -- | GetOrExtract drives lazy extraction per file |
| Graph building | FileGraph.BuildGraph (repomap pkg) | -- | Reads from populated cache, already wired via ensureGraph |
| LSP enrichment | FileGraph.EnrichFromLSP (repomap pkg) | Kernel worker pool | Needs workspace key + active lease for textDocument/references |
| Ranked rendering | TreeRenderer (repomap pkg) | -- | Must replace skill.go inline renderTree/renderBudgeted |
| Token budgeting | TreeRenderer.RenderBudgeted | -- | 15% tolerance binary search (vs skill's 0% tolerance) |

## Standard Stack

No new dependencies needed. All components exist in the codebase.

### Core (already in codebase)
| Component | Package | Purpose |
|-----------|---------|---------|
| TagExtractor | `internal/repomap/extractor.go` | Tree-sitter .scm tag extraction |
| FallbackExtractor | `internal/repomap/fallback.go` | LSP documentSymbol fallback |
| TagCache | `internal/repomap/cache.go` | SQLite cache with mtime invalidation |
| FileGraph | `internal/repomap/graph.go` | Cross-file reference graph + EnrichFromLSP |
| PageRank | `internal/repomap/pagerank.go` | Personalized PageRank ranking |
| TreeRenderer | `internal/repomap/render.go` | Tree-structured token-budgeted rendering |
| ElisionRenderer | `internal/repomap/elide.go` | Scope-aware body elision |
| GrammarRegistry | `internal/treesitter/registry.go` | Shared tree-sitter grammar instances |
| RepoMapSkill | `internal/skill/repomap/skill.go` | MCP tool dispatch (get_repo_map, get_context) |

## Architecture Patterns

### System Architecture Diagram

```
[MCP Client] --tool call--> [RepoMapSkill.ExecuteTool]
                                    |
                          [ensureCache] -- lazy, first call
                                |
                    [filepath.WalkDir(workspaceRoot)]
                                |
                    +-----------+-----------+
                    |                       |
            [langFromExt]           [langFromExt == ""]
            supported lang          unsupported lang
                    |                       |
        [TagExtractor.Extract]    [FallbackExtractor.Extract]
            (tree-sitter)           (LSP documentSymbol)
                    |                       |
                    +-------+-------+-------+
                            |
                   [TagCache.GetOrExtract]
                            |
                   [ensureGraph] -- version-gated rebuild
                            |
                   [BuildGraph(cache)]
                            |
               [EnrichFromLSP] -- opportunistic, if warm LSP
                            |
                   [graph.RankFiles(damping, personalization)]
                            |
               [TreeRenderer.RenderBudgeted(ranked, budget)]
                            |
                   [MCP tool response]
```

### Pattern 1: Lazy Cache Population
**What:** Walk workspace and populate TagCache on first tool call, not during Init(). [VERIFIED: codebase analysis]
**When to use:** When workspace root is unknown at Init() time (SkillDeps.ProjectDir is ~/.serena/default-project, not workspace root).
**Why:** The workspace root is only known after `activate_project` is called. RepoMapSkill.Init() runs during daemon startup before any workspace is activated.

```go
// ensureCache lazily walks the workspace and populates the tag cache.
func (s *RepoMapSkill) ensureCache() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.cachePopulated {
        return nil
    }
    root := s.resolveRoot()
    err := s.walkAndExtract(root)
    if err != nil {
        return err
    }
    s.cachePopulated = true
    return nil
}
```

### Pattern 2: Workspace Root Resolution
**What:** Derive workspace root from os.Getwd() (current fallback) or accept it as a parameter. [VERIFIED: daemon.go:286-302]
**Critical insight:** The daemon's `SetActivateCallback` updates `activeWSKey.RepoRoot` when `activate_project` is called. However, this value is NOT propagated to the RepoMapSkill. The skill falls back to `os.Getwd()` via `resolveRoot()`.

**Options (Claude's discretion):**
1. **Add WorkspaceRoot to SkillDeps** -- requires modifying SkillDeps struct and daemon wiring
2. **Add a SetWorkspaceRoot method on RepoMapSkill** -- daemon calls it in SetActivateCallback
3. **Accept workspace root as tool parameter** -- agent passes it explicitly
4. **Keep os.Getwd() fallback** -- works because daemon cwd is typically the workspace

**Recommended:** Option 2 (SetWorkspaceRoot callback) -- minimal SkillDeps change, daemon already has the hook point in SetActivateCallback. This is consistent with the existing pattern where profile skill has SetSessionProvider.

### Pattern 3: Replace Inline Rendering with TreeRenderer
**What:** Delete `renderBudgeted` and `renderTree` from skill.go, replace with TreeRenderer delegation. [VERIFIED: render.go vs skill.go analysis]
**Why:** Two divergent implementations:
- `skill.go:renderBudgeted` -- no 15% tolerance, no min-1-file guarantee
- `render.go:RenderBudgeted` -- has 15% tolerance (`okErr := 0.15`), guarantees at least 1 file
- `skill.go:renderTree` -- flat indented list, sorts by path
- `render.go:renderTree` -- proper directory tree structure with nesting

### Pattern 4: Duplicate langFromExt Consolidation
**What:** Two `langFromExt` functions exist with different signatures and coverage. [VERIFIED: grep]
- `skill.go:langFromExt(ext string)` -- takes extension (e.g., ".go"), supports 6 languages
- `render.go:langFromExt(path string)` -- takes full path, calls filepath.Ext(), supports 11 languages

**Fix:** Delete from skill.go, use the render.go version (or move to a shared location in the repomap package).

### Anti-Patterns to Avoid
- **Eager workspace walking in Init():** SkillDeps.ProjectDir is NOT the workspace root. Walking ~/.serena/default-project would find nothing useful. Walk must be lazy, triggered after workspace activation. [VERIFIED: daemon.go:209]
- **Creating a second GrammarRegistry in RepoMapSkill:** The daemon already creates one at line 191. RepoMapSkill.Init() creates another at line 71. Share the daemon's instance or accept the duplication (GrammarRegistry is stateless, ~5 allocations). [VERIFIED: daemon.go:191, skill.go:71]
- **Blocking on FallbackExtractor for every file:** FallbackExtractor needs an LSP session. Only use it for files where TagExtractor returns "no query for language". Don't block the walk on LSP availability. [VERIFIED: fallback.go needs SymbolRequester]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token-budgeted rendering | Inline binary search in skill.go | `TreeRenderer.RenderBudgeted` | Already has 15% tolerance, min-1-file, proper tree structure |
| File tree rendering | Flat indented list in skill.go | `TreeRenderer.renderTree` | Proper directory nesting, sorted dirs-first |
| Tag extraction | Manual regex/string parsing | `TagExtractor.Extract` + `FallbackExtractor.Extract` | Tree-sitter queries are precise, LSP fallback handles edge cases |
| Directory skip list | Custom skip logic | Reuse `skipDirs` pattern from `fileops/find.go` | .git, node_modules, __pycache__, .serena already defined |

## Common Pitfalls

### Pitfall 1: SkillDeps.ProjectDir is NOT workspace root
**What goes wrong:** Using `deps.ProjectDir` to walk for source files yields nothing -- it points to `~/.serena/default-project/`.
**Why it happens:** SkillDeps is set once at daemon startup. The workspace root is only known after `activate_project`.
**How to avoid:** Use lazy resolution via `resolveRoot()` or a SetWorkspaceRoot callback triggered from daemon's activate callback.
**Warning signs:** "No files found in repository" despite active workspace. [VERIFIED: daemon.go:209]

### Pitfall 2: FallbackExtractor requires active LSP session
**What goes wrong:** Calling `FallbackExtractor.Extract()` without a warm LSP session blocks or fails.
**Why it happens:** FallbackExtractor.Extract takes a `SymbolRequester` which is a WorkerLease from lspool. No lease = no extraction.
**How to avoid:** Use FallbackExtractor only opportunistically. If no LSP session is available for a language, skip fallback extraction for those files. Tree-sitter covers Go/Python/TypeScript/Rust; fallback is for other languages.
**Warning signs:** Timeouts during workspace walk. [VERIFIED: fallback.go:30]

### Pitfall 3: GrammarRegistry duplication
**What goes wrong:** Two separate GrammarRegistry instances waste memory and compile queries twice.
**Why it happens:** daemon.go:191 creates one for BodyExtractor; skill.go:71 creates another for ElisionRenderer.
**How to avoid:** Accept the duplication (it's small -- 5 language pointers) OR pass the daemon's registry through SkillDeps. Pragmatic choice: accept duplication for now, it's not a correctness issue. [VERIFIED: daemon.go:191, skill.go:71]

### Pitfall 4: Walking large repositories is slow
**What goes wrong:** Walking a repo with 100K+ files and extracting tags for each blocks the first tool call for seconds/minutes.
**Why it happens:** `filepath.WalkDir` + tree-sitter parse for every file is O(n * parse_time).
**How to avoid:** (1) TagCache mtime invalidation means second call is fast. (2) Skip non-source files (check extension before extraction). (3) Consider capping walk at a reasonable file count. (4) Walk concurrently with a worker pool for extraction. [ASSUMED]

### Pitfall 5: Absolute vs relative paths in TagCache
**What goes wrong:** TagCache stores absolute paths but tool output should show relative paths.
**Why it happens:** `filepath.WalkDir` yields absolute paths; `renderTree` needs relative paths for display.
**How to avoid:** Store absolute paths in cache (for mtime stat), convert to relative in rendering. TreeRenderer already does this via `filepath.Rel(r.rootDir, rf.Path)`. [VERIFIED: render.go:97-99]

### Pitfall 6: EnrichFromLSP needs kernel access
**What goes wrong:** RepoMapSkill has no reference to the kernel or worker pool.
**Why it happens:** Skills receive SkillDeps which doesn't include kernel reference.
**How to avoid:** Either (a) add an optional KernelPool to SkillDeps, (b) add a SetKernel method on RepoMapSkill that daemon calls, or (c) make EnrichFromLSP a post-Init callback. Option (b) is consistent with the SetSessionProvider pattern. [VERIFIED: skill.go SkillDeps struct]

## Code Examples

### Workspace Walk and Cache Population
```go
// walkAndExtract walks the workspace root and populates TagCache using extractors.
// Source: pattern derived from fileops/find.go:35 + repomap/cache.go:57
func (s *RepoMapSkill) walkAndExtract(root string) error {
    return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            if d != nil && d.IsDir() && skipDirs[d.Name()] {
                return filepath.SkipDir
            }
            return nil
        }
        lang := repomap.LangFromExt(path) // use the render.go version
        if lang == "" {
            return nil // skip unsupported files
        }
        _, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
            if s.extractor != nil {
                if _, ok := s.extractor.HasLanguage(lang); ok {
                    return s.extractor.Extract(source, path, lang)
                }
            }
            // Could try FallbackExtractor here if LSP available
            return nil, nil
        })
        return extractErr // or log and continue
    })
}
```

### TreeRenderer Delegation
```go
// Before (skill.go inline):
output := s.renderBudgeted(ranked, budget)

// After (delegate to TreeRenderer):
output := s.renderer.RenderBudgeted(ranked, budget)
```

### EnrichFromLSP Call Site
```go
// In ensureGraph, after BuildGraph:
// Source: pattern from graph.go:126
if s.pool != nil {
    // Opportunistic: try to enrich from warm LSP sessions
    for file := range s.cache.AllFiles() {
        // Only if there's a warm lease available (non-blocking)
        // This is additive -- graph already has tree-sitter edges
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline renderBudgeted (no tolerance) | TreeRenderer.RenderBudgeted (15% tolerance) | Phase 28 | More files included near budget boundary |
| Flat indented file list | Directory tree with nesting | Phase 28 | Better readability for agents |
| Empty cache (no extraction) | Lazy walk + TagExtractor | Phase 30 (this) | Tools actually return data |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Walking large repos may need file count cap or concurrency | Pitfall 4 | First-call latency could be unacceptable for very large repos |
| A2 | Accept GrammarRegistry duplication rather than passing through SkillDeps | Pitfall 3 | Minor memory waste, no correctness risk |
| A3 | os.Getwd() as fallback workspace root works in daemon context | Pattern 2 | Wrong root if daemon cwd differs from workspace |

## Open Questions

1. **How should workspace root reach RepoMapSkill?**
   - What we know: SkillDeps.ProjectDir is ~/.serena/default-project, not workspace root. Daemon updates activeWSKey in SetActivateCallback.
   - What's unclear: Whether to modify SkillDeps, add a callback, or rely on os.Getwd()
   - Recommendation: Add SetWorkspaceRoot method, call from daemon's activate callback (matches SetSessionProvider pattern)

2. **Should FallbackExtractor be wired in Phase 30 or deferred?**
   - What we know: FallbackExtractor needs a SymbolRequester (LSP WorkerLease). Getting a lease requires kernel pool + workspace key.
   - What's unclear: Whether the added complexity of LSP lease management in the walk is worth it when tree-sitter covers the 5 primary languages
   - Recommendation: Wire it optionally -- attempt fallback only for files with unsupported tree-sitter languages AND when an LSP pool reference is available. Don't block on it.

3. **Should we invalidate/repopulate cache on workspace switch?**
   - What we know: TagCache is per-skill instance with a single dbPath. If workspace changes, old cache has irrelevant files.
   - What's unclear: Whether multi-workspace is a realistic scenario in v1.6
   - Recommendation: Clear cache on workspace switch (SetWorkspaceRoot), let mtime invalidation handle individual file changes within a workspace

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.10.0 |
| Config file | None (standard go test) |
| Quick run command | `go test ./internal/skill/repomap/ -v -count=1` |
| Full suite command | `go test ./internal/repomap/... ./internal/skill/repomap/... -v -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RMAP-04 | Graph built from extracted tags in cache | integration | `go test ./internal/skill/repomap/ -run TestPipelineWalk -v` | Wave 0 |
| RMAP-05 | PageRank produces ranked output from real data | integration | `go test ./internal/skill/repomap/ -run TestPipelineRanked -v` | Wave 0 |
| RMAP-06 | get_repo_map returns ranked symbols | integration | `go test ./internal/skill/repomap/ -run TestGetRepoMap_WithWorkspace -v` | Wave 0 |
| RMAP-07 | get_context returns task-relevant symbols | integration | `go test ./internal/skill/repomap/ -run TestGetContext_WithWorkspace -v` | Wave 0 |
| RMAP-08 | EnrichFromLSP called when LSP warm | unit | `go test ./internal/skill/repomap/ -run TestEnrichFromLSP -v` | Wave 0 |
| RMAP-10 | TreeRenderer.RenderBudgeted used with tolerance | unit | `go test ./internal/skill/repomap/ -run TestRenderBudgeted_Tolerance -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/skill/repomap/ -v -count=1`
- **Per wave merge:** `go test ./internal/repomap/... ./internal/skill/repomap/... -v -count=1 && go vet ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Integration tests with real workspace walk + TagExtractor + cache population
- [ ] Test that TreeRenderer.RenderBudgeted is used (not inline copy)
- [ ] Test that workspace root is correctly resolved (not hardcoded)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | yes | Path traversal check already exists (skill.go:165-168) |
| V5 Input Validation | yes | Token budget capping (skill.go:357-377), file array validation |
| V6 Cryptography | no | -- |

### Known Threat Patterns for RepoMap

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via files parameter | Tampering | Already implemented: `strings.Contains(fp, "..")` check |
| Token budget overflow | DoS | Already implemented: cap at 32768, min 64 |
| Large repo walk DoS | DoS | Consider max file count cap during walk |
| Symlink escape during walk | Information Disclosure | `filepath.WalkDir` follows symlinks by default -- consider `d.Type()&fs.ModeSymlink` check |

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/skill/repomap/skill.go` -- current RepoMapSkill implementation
- Codebase analysis: `internal/repomap/extractor.go` -- TagExtractor interface and Extract method
- Codebase analysis: `internal/repomap/fallback.go` -- FallbackExtractor and SymbolRequester interface
- Codebase analysis: `internal/repomap/cache.go` -- TagCache with GetOrExtract, AllFiles, Version
- Codebase analysis: `internal/repomap/graph.go` -- FileGraph, BuildGraph, EnrichFromLSP
- Codebase analysis: `internal/repomap/render.go` -- TreeRenderer with RenderBudgeted (15% tolerance)
- Codebase analysis: `internal/repomap/pagerank.go` -- PageRank with personalization support
- Codebase analysis: `internal/daemon/daemon.go` -- Daemon bootstrap, skill init, activate callback
- Codebase analysis: `internal/skill/skill.go` -- SkillDeps structure
- Codebase analysis: `internal/treesitter/registry.go` -- GrammarRegistry (shared, stateless)
- `.planning/v1.6-MILESTONE-AUDIT.md` -- Gap analysis identifying all broken paths

### Secondary (MEDIUM confidence)
- Test execution: `go test ./internal/skill/repomap/` -- all 9 tests pass (verified 2026-04-17)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all components exist in codebase, verified by reading source
- Architecture: HIGH -- integration pattern is clear from codebase analysis
- Pitfalls: HIGH -- all identified from verified code paths and interface signatures

**Research date:** 2026-04-17
**Valid until:** 2026-05-01 (stable -- internal integration, no external dependencies)
