# Phase 28: RepoMap Graph & MCP Tools - Research

**Researched:** 2026-04-16
**Domain:** Graph algorithms (PageRank), MCP tool design, token budgeting
**Confidence:** HIGH

## Summary

Phase 28 builds on Phase 27's tag extraction/cache/elision infrastructure to create a cross-file reference graph, rank files via PageRank, and expose two token-budgeted MCP tools (`get_repo_map`, `get_context`). The graph is file-level (nodes = files, edges = ref-to-def links weighted by frequency), built in-memory from SQLite-cached tags on first call. Standard PageRank ranks files for `get_repo_map`; Personalized PageRank with seed files ranks for `get_context`.

The algorithm closely follows aider's repomap approach: build a directed graph where edges go from referencing files to defining files, add self-loops (weight 0.1) for definitions without references, run PageRank, then binary-search on file count to fit output within token budget. The implementation is a new skill package (`internal/skill/repomap/`) following the memory skill pattern with `init()` registration and `ExecuteTool` dispatch.

**Primary recommendation:** Implement PageRank from scratch (~60 lines of Go) rather than importing a third-party library. Available Go PageRank libraries (dcadenas/pagerank, alixaxel/pagerank) do not support personalized PageRank, and the algorithm is trivial to implement correctly with power iteration. This avoids a dependency for minimal code.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** File-level graph -- nodes represent files, edges represent cross-file ref-to-def links between files
- **D-02:** In-memory rebuild -- build graph from SQLite-cached tags on first `get_repo_map` call. No persistence of graph edges
- **D-03:** Edge weighting by reference count -- files with more cross-references to a target file produce a higher-weight edge
- **D-04:** Personalized PageRank with seed files -- `get_context` personalizes the teleportation vector by seeding from the files/symbols the agent specifies
- **D-05:** Standard (uniform) PageRank for `get_repo_map` -- no personalization
- **D-06:** Skill tools -- register as a `ToolProvider` skill in `internal/skill/repomap/`
- **D-07:** `get_repo_map` returns an elided source tree -- file paths as a tree structure with elided symbols nested under each file
- **D-08:** `get_context` accepts file paths (primary input for PageRank seeding) plus an optional `task_description` string reserved for future semantic matching
- **D-09:** Token budget parameter on both tools -- integer controlling maximum output size, required parameter with reasonable default (e.g., 4096 tokens)
- **D-10:** Opportunistic LSP enrichment -- piggyback `textDocument/references` calls on warm LS sessions only
- **D-11:** Additive edges -- LSP references add new cross-file edges, never replace tree-sitter tags
- **D-12:** Character-based token estimation -- approximate tokens as chars/4
- **D-13:** Prune lowest-ranked files first -- binary search on file count until output fits within budget

### Claude's Discretion
- Graph rebuild caching strategy (e.g., dirty-flag to avoid rebuilding when tags haven't changed)
- PageRank convergence parameters (damping factor, iterations, epsilon)
- Tool parameter validation and error response format
- LSP enrichment batching and rate limiting
- File tree rendering format details (indentation, path compression)

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RMAP-04 | Cross-file reference graph built from extracted tags (nodes = files, edges = ref-to-def) | Graph module building from TagCache, edge construction algorithm documented |
| RMAP-05 | Personalized PageRank ranks symbol importance with configurable personalization weights | Custom PageRank implementation with personalization vector support |
| RMAP-06 | Agent can call `get_repo_map` for token-budgeted structural overview | Skill tool pattern, binary search token budget, elision renderer integration |
| RMAP-07 | Agent can call `get_context` for task-focused context | Personalized PageRank seeded with specified files |
| RMAP-08 | Warm LSP sessions enrich reference graph with precise cross-file references | Opportunistic enrichment via lspool WorkerLease + SymbolRequester interface |
| RMAP-10 | Token budget parameter controls output size with binary search | Binary search algorithm over ranked file count, chars/4 estimation |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cross-file reference graph | API / Backend (daemon) | -- | In-memory graph built from SQLite-cached tags; daemon-lifetime persistence |
| PageRank computation | API / Backend (daemon) | -- | Pure computation on in-memory graph, no external service needed |
| MCP tool exposure | API / Backend (MCP server) | -- | Skill tools registered with MCP server via ToolProvider pattern |
| Token budgeting | API / Backend (daemon) | -- | Server-side truncation before response; client does not participate |
| LSP enrichment | API / Backend (kernel) | -- | Piggybacks on existing lspool WorkerLease lifecycle |
| Elided output rendering | API / Backend (daemon) | -- | Uses Phase 27 ElisionRenderer, runs in daemon process |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| No new dependencies | -- | PageRank is hand-rolled (~60 LOC) | Available Go libraries lack personalized PageRank; algorithm is trivial |

### Supporting (existing, already in go.mod)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| modernc.org/sqlite | already in go.mod | TagCache SQLite queries for graph building | Querying all cached tags to build cross-file graph |
| go-tree-sitter | already in go.mod | ElisionRenderer for output formatting | Rendering elided symbol views in tool output |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled PageRank | github.com/dcadenas/pagerank | No personalized PageRank support, adds dependency for ~60 LOC savings |
| Hand-rolled PageRank | github.com/alixaxel/pagerank | Weighted edges but no personalization vector, inactive project |
| chars/4 token estimation | tiktoken-go | Exact counts but adds CGO dependency + 3MB model file; chars/4 is industry-standard approximation for budget control |

**Installation:**
```bash
# No new dependencies required
```

## Architecture Patterns

### System Architecture Diagram

```
Agent (Claude Code / Codex / IDE)
  |
  | MCP: get_repo_map(token_budget=4096)
  | MCP: get_context(files=["cmd/main.go"], token_budget=2048)
  v
+------------------------------------------+
| MCP Server (skill dispatch)              |
|   RepoMapSkill.ExecuteTool()             |
+------------------------------------------+
  |
  v
+------------------------------------------+
| RepoMapService (core logic)              |
|   +-- BuildGraph()                       |
|   |     TagCache.AllFiles() -> tags      |
|   |     Match ref.Name == def.Name       |
|   |     -> Graph{edges: file->file}      |
|   +-- RankFiles(graph, seeds?)           |
|   |     PageRank(damping=0.85, eps=1e-6) |
|   |     or PersonalizedPageRank(seeds)   |
|   +-- RenderBudgeted(ranked, budget)     |
|         Binary search on file count      |
|         ElisionRenderer.RenderFile()     |
|         Token estimate: len(output)/4    |
+------------------------------------------+
  |                           |
  v                           v
+----------------+  +-------------------+
| TagCache       |  | LSP Enrichment    |
| (SQLite)       |  | (opportunistic)   |
| AllFiles()     |  | textDocument/     |
| GetOrExtract() |  | references        |
+----------------+  +-------------------+
```

### Recommended Project Structure
```
internal/
├── repomap/               # Phase 27 (existing) + Phase 28 additions
│   ├── tags.go            # Tag, TagKind (existing)
│   ├── extractor.go       # TagExtractor (existing)
│   ├── fallback.go        # FallbackExtractor (existing)
│   ├── cache.go           # TagCache (existing, needs AllFiles method)
│   ├── elide.go           # ElisionRenderer (existing)
│   ├── schema.go          # SQL schema (existing)
│   ├── graph.go           # NEW: FileGraph, BuildGraph, edge construction
│   ├── graph_test.go      # NEW: graph building tests
│   ├── pagerank.go        # NEW: PageRank + PersonalizedPageRank
│   ├── pagerank_test.go   # NEW: convergence, personalization tests
│   ├── render.go          # NEW: tree rendering, token budgeting, binary search
│   └── render_test.go     # NEW: budget fitting tests
├── skill/
│   └── repomap/
│       ├── skill.go       # NEW: RepoMapSkill, ToolProvider, ExecuteTool
│       └── skill_test.go  # NEW: tool dispatch tests
```

### Pattern 1: File-Level Reference Graph
**What:** Build a directed graph where nodes are file paths and edges are weighted ref-to-def links.
**When to use:** On first `get_repo_map` or `get_context` call after daemon start or after tag invalidation.
**Example:**
```go
// [VERIFIED: Phase 27 TagCache schema + Tag struct]
type FileGraph struct {
    // Edges maps sourceFile -> targetFile -> weight (reference count).
    Edges map[string]map[string]float64
    // Files is the set of all files in the graph.
    Files map[string]bool
}

func BuildGraph(cache *TagCache) (*FileGraph, error) {
    allTags, err := cache.AllFiles()  // NEW method needed
    if err != nil {
        return nil, err
    }
    
    // Index: symbolName -> []filePath for definitions
    defs := make(map[string][]string)
    // Index: symbolName -> []filePath for references (with counts)
    refs := make(map[string]map[string]int)
    
    for filePath, tags := range allTags {
        for _, tag := range tags {
            if tag.Kind == TagDef {
                defs[tag.Name] = append(defs[tag.Name], filePath)
            } else {
                if refs[tag.Name] == nil {
                    refs[tag.Name] = make(map[string]int)
                }
                refs[tag.Name][filePath]++
            }
        }
    }
    
    g := &FileGraph{
        Edges: make(map[string]map[string]float64),
        Files: make(map[string]bool),
    }
    
    // Create edges: referencer -> definer, weight = sqrt(refCount) per D-03
    for ident, definers := range defs {
        refFiles, hasRefs := refs[ident]
        if !hasRefs {
            // Self-loop for isolated definitions (aider pattern)
            for _, defFile := range definers {
                g.addEdge(defFile, defFile, 0.1)
            }
            continue
        }
        for refFile, count := range refFiles {
            for _, defFile := range definers {
                if refFile == defFile {
                    continue // skip same-file refs
                }
                g.addEdge(refFile, defFile, math.Sqrt(float64(count)))
            }
        }
    }
    return g, nil
}
```

### Pattern 2: PageRank with Personalization
**What:** Power iteration PageRank supporting optional personalization vector for seed files.
**When to use:** `get_repo_map` uses uniform, `get_context` uses personalized with seed files.
**Example:**
```go
// [ASSUMED: standard PageRank power iteration, well-known algorithm]
func (g *FileGraph) PageRank(damping float64, epsilon float64, maxIter int, personalization map[string]float64) map[string]float64 {
    n := len(g.Files)
    if n == 0 {
        return nil
    }
    
    files := make([]string, 0, n)
    for f := range g.Files {
        files = append(files, f)
    }
    
    // Initialize uniform or personalized teleport vector
    teleport := make(map[string]float64, n)
    if len(personalization) > 0 {
        total := 0.0
        for _, w := range personalization {
            total += w
        }
        for f, w := range personalization {
            teleport[f] = w / total
        }
        // Fill remaining with small baseline
        for _, f := range files {
            if _, ok := teleport[f]; !ok {
                teleport[f] = (1.0 / float64(n)) * 0.01
            }
        }
        // Re-normalize
        total = 0.0
        for _, w := range teleport {
            total += w
        }
        for f := range teleport {
            teleport[f] /= total
        }
    } else {
        for _, f := range files {
            teleport[f] = 1.0 / float64(n)
        }
    }
    
    rank := make(map[string]float64, n)
    for f, w := range teleport {
        rank[f] = w
    }
    
    for iter := 0; iter < maxIter; iter++ {
        newRank := make(map[string]float64, n)
        // Teleport component
        for f, w := range teleport {
            newRank[f] = (1 - damping) * w
        }
        // Link component
        for src, targets := range g.Edges {
            totalWeight := 0.0
            for _, w := range targets {
                totalWeight += w
            }
            if totalWeight == 0 {
                continue
            }
            for dst, w := range targets {
                newRank[dst] += damping * rank[src] * (w / totalWeight)
            }
        }
        // Check convergence
        diff := 0.0
        for _, f := range files {
            diff += math.Abs(newRank[f] - rank[f])
        }
        rank = newRank
        if diff < epsilon {
            break
        }
    }
    return rank
}
```

### Pattern 3: Binary Search Token Budget (D-13)
**What:** Binary search on file count to maximize coverage within token budget.
**When to use:** After PageRank ranking, before rendering output.
**Example:**
```go
// [CITED: aider repomap.py binary search approach]
func (s *RepoMapService) RenderBudgeted(ranked []RankedFile, budget int, renderer *ElisionRenderer) string {
    if len(ranked) == 0 {
        return ""
    }
    
    lower, upper := 1, len(ranked)
    bestOutput := ""
    bestTokens := 0
    okErr := 0.15 // 15% tolerance per aider
    
    for lower <= upper {
        mid := (lower + upper) / 2
        output := s.renderTree(ranked[:mid], renderer)
        tokens := len(output) / 4 // D-12: chars/4
        
        pctErr := math.Abs(float64(tokens-budget)) / float64(budget)
        
        if (tokens <= budget && tokens > bestTokens) || pctErr < okErr {
            bestOutput = output
            bestTokens = tokens
        }
        
        if tokens < budget {
            lower = mid + 1
        } else {
            upper = mid - 1
        }
    }
    return bestOutput
}
```

### Pattern 4: Skill Tool Registration (D-06)
**What:** Caddy-style init() registration following the memory skill pattern.
**When to use:** For `get_repo_map` and `get_context` tool registration.
**Example:**
```go
// [VERIFIED: internal/skill/memory/skill.go pattern]
package repomap

import (
    "github.com/postfix/serena/internal/skill"
    "github.com/postfix/serena/internal/mcp"
)

type RepoMapSkill struct {
    service *RepoMapService
    logger  *slog.Logger
}

func init() {
    skill.Register(&RepoMapSkill{})
}

func (s *RepoMapSkill) Name() string        { return "repomap" }
func (s *RepoMapSkill) Description() string  { return "Repository structure map with ranked symbols" }

func (s *RepoMapSkill) Init(deps skill.SkillDeps) error {
    // RepoMapService needs TagCache, ElisionRenderer, GrammarRegistry
    // These must be injected via extended SkillDeps or constructor
    s.logger = deps.Logger
    return nil
}

func (s *RepoMapSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        s.getRepoMapTool(),
        s.getContextTool(),
    }
}

func (s *RepoMapSkill) ExecuteTool(name string, args map[string]interface{}) (string, error) {
    switch name {
    case "get_repo_map":
        return s.execGetRepoMap(args)
    case "get_context":
        return s.execGetContext(args)
    default:
        return "", fmt.Errorf("unknown repomap tool: %s", name)
    }
}
```

### Anti-Patterns to Avoid
- **Persisting graph edges to SQLite:** D-02 explicitly says in-memory only. Rebuilding from cached tags is fast enough.
- **Starting LS sessions for enrichment:** D-10 says opportunistic only -- piggyback on warm sessions, never start new ones just for enrichment.
- **Exact token counting:** D-12 says chars/4. Adding a tokenizer dependency for exact counts is overkill for budget control.
- **Including same-file references as edges:** Cross-file references only. Same-file refs add noise without improving ranking.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tag extraction | Custom parser | Phase 27 TagExtractor + FallbackExtractor | Already built with tree-sitter queries for 4+ languages |
| Elided output | Custom formatter | Phase 27 ElisionRenderer | Scope-aware body elision already handles all edge cases |
| SQLite access | Raw SQL driver | Phase 27 TagCache | Schema, WAL mode, busy timeout already configured |
| MCP tool wiring | Custom registration | Skill system (skill.Register, ToolProvider, ExecuteTool) | Proven pattern from memory skill with daemon integration |

**Key insight:** Phase 28 is almost entirely new algorithmic code (graph, PageRank, budget search) plus a thin skill wrapper. The heavy infrastructure (tag extraction, caching, elision, MCP registration) is all reused from prior phases.

## Common Pitfalls

### Pitfall 1: TagCache lacks AllFiles method
**What goes wrong:** Graph building needs all cached file paths and their tags, but TagCache only has GetOrExtract (single file) and no bulk query.
**Why it happens:** Phase 27 designed TagCache for per-file access, not iteration.
**How to avoid:** Add an `AllFiles() (map[string][]Tag, error)` method to TagCache that queries `SELECT DISTINCT file_path FROM file_tags` then loads tags for each file. This is a small addition to the existing cache module.
**Warning signs:** Attempting to iterate without this method leads to either scanning the filesystem (slow) or working around the cache API.

### Pitfall 2: Dangling nodes in PageRank
**What goes wrong:** Files with no outgoing edges (sinks) absorb rank and cause convergence issues.
**Why it happens:** Some files only define symbols but never reference other files.
**How to avoid:** Handle dangling nodes explicitly: redistribute their rank uniformly across all nodes (standard PageRank fix). Also add self-loops for definitions without cross-file references (weight 0.1, per aider pattern).
**Warning signs:** PageRank scores sum to significantly less than 1.0, or some files get unreasonably high scores.

### Pitfall 3: Binary search off-by-one
**What goes wrong:** Budget fitting includes one too many or too few files, or enters infinite loop.
**Why it happens:** Binary search boundary conditions with integer division.
**How to avoid:** Use the aider-style 15% tolerance (`okErr = 0.15`), track best result seen so far, terminate when `lower > upper`. Always return at least 1 file.
**Warning signs:** Output significantly under-budget or tests showing empty output for reasonable budgets.

### Pitfall 4: Skill dependency injection gap
**What goes wrong:** RepoMapSkill needs TagCache, ElisionRenderer, and GrammarRegistry, but SkillDeps only provides ProjectDir, GlobalDir, Logger.
**Why it happens:** The existing SkillDeps struct was designed for file-based skills (memory, workflow), not kernel-adjacent skills.
**How to avoid:** Either (a) extend SkillDeps with optional fields for kernel components, or (b) have the daemon wire the repomap skill specially (like it does for kernel tools), or (c) have the skill create its own TagCache/GrammarRegistry from the project dir path. Option (a) is cleanest.
**Warning signs:** nil pointer panics when the skill tries to access TagCache.

### Pitfall 5: Large repo performance
**What goes wrong:** Building the graph and running PageRank takes too long on repos with 10k+ files.
**Why it happens:** O(files * tags) for graph building, O(iterations * edges) for PageRank.
**How to avoid:** Cache the built graph with a dirty flag. Only rebuild when TagCache has been modified (new files extracted, mtimes changed). PageRank with damping=0.85 converges in 20-40 iterations for typical repos.
**Warning signs:** Tool response time exceeding 2-3 seconds.

## Code Examples

### TagCache.AllFiles (new method needed)
```go
// [VERIFIED: follows existing TagCache pattern in cache.go]
// AllFiles returns all cached file paths with their tags.
// Used by graph building to iterate the entire tag cache.
func (c *TagCache) AllFiles() (map[string][]Tag, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    rows, err := c.db.Query("SELECT DISTINCT file_path FROM file_tags")
    if err != nil {
        return nil, fmt.Errorf("listing cached files: %w", err)
    }
    defer rows.Close()

    var files []string
    for rows.Next() {
        var fp string
        if err := rows.Scan(&fp); err != nil {
            return nil, err
        }
        files = append(files, fp)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    result := make(map[string][]Tag, len(files))
    for _, fp := range files {
        tags, err := c.loadTags(fp)
        if err != nil {
            return nil, fmt.Errorf("loading tags for %s: %w", fp, err)
        }
        result[fp] = tags
    }
    return result, nil
}
```

### Tree Rendering Format
```go
// [ASSUMED: aider-style tree format with path compression]
// Output format for get_repo_map:
//
// internal/
//   repomap/
//     cache.go
//       // L25:
//       func NewTagCache(dbPath string) (*TagCache, error) { ... }
//       // L59:
//       func (c *TagCache) GetOrExtract(...) ([]Tag, error) { ... }
//     graph.go
//       // L15:
//       func BuildGraph(cache *TagCache) (*FileGraph, error) { ... }
//   skill/
//     repomap/
//       skill.go
//         // L20:
//         func (s *RepoMapSkill) ExecuteTool(...) (string, error) { ... }
```

### LSP Enrichment Hook
```go
// [VERIFIED: lspool/adapter.go References method exists]
// EnrichFromLSP adds cross-file reference edges from an LSP references response.
// Called opportunistically when a WorkerLease is already active.
func (g *FileGraph) EnrichFromLSP(sourceFile string, refs []gen.Location) {
    for _, ref := range refs {
        targetFile := uriToPath(string(ref.URI))
        if targetFile == "" || targetFile == sourceFile {
            continue
        }
        g.addEdge(targetFile, sourceFile, 1.0) // LSP ref -> source def
        g.Files[targetFile] = true
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| ctags-based repo maps | Tree-sitter tag extraction | 2024 (aider) | Language-aware, scope-sensitive tags |
| Flat file lists for context | PageRank-based ranking | 2024 (aider) | Structural importance scoring |
| Fixed token windows | Binary search budget fitting | 2024 (aider) | Maximizes coverage within budget |
| Keyword-based context selection | Graph-based ranking | 2024 (aider) | Leverages code structure, not text |

**Deprecated/outdated:**
- ctags: Replaced by tree-sitter for accuracy and scope awareness
- NetworkX in Python: Serena uses Go; PageRank is simple enough to implement without a graph library

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | PageRank power iteration converges in 20-40 iterations for typical codebases | Common Pitfalls | Minimal -- can increase maxIter, convergence is well-studied |
| A2 | chars/4 is adequate token estimation for budget control | Architecture Patterns | Low -- budget is approximate by design, 15% tolerance handles variance |
| A3 | Self-loop weight of 0.1 for isolated definitions is appropriate | Architecture Patterns | Low -- follows aider's proven approach |
| A4 | Damping factor 0.85 is optimal default | Claude's Discretion | Minimal -- 0.85 is the standard default across all PageRank implementations |
| A5 | File tree rendering with path compression is the expected output format | Code Examples | Low -- D-07 specifies "file paths as a tree structure with elided symbols" |

## Open Questions

1. **SkillDeps extension for kernel components**
   - What we know: RepoMapSkill needs TagCache, ElisionRenderer, GrammarRegistry. Current SkillDeps only has ProjectDir, GlobalDir, Logger.
   - What's unclear: Best approach to inject these dependencies without breaking the existing skill interface.
   - Recommendation: Add optional fields to SkillDeps (TagCache, GrammarRegistry are pointer types -- nil means not available). The daemon already creates both; just wire them into the deps struct. This is backwards-compatible since existing skills ignore fields they don't need.

2. **Graph dirty-flag strategy**
   - What we know: D-02 says rebuild from cache. Claude's discretion includes caching strategy.
   - What's unclear: How to detect when TagCache contents have changed since last graph build.
   - Recommendation: Add a version counter to TagCache that increments on any store/invalidate/clear operation. Graph builder checks version on each call; if unchanged, returns cached graph.

3. **LSP enrichment integration point**
   - What we know: D-10 says opportunistic, piggyback on warm sessions. D-11 says additive.
   - What's unclear: Where exactly to hook into the lspool lifecycle to call textDocument/references.
   - Recommendation: Add an optional callback to WorkerLease that fires after any successful request. The repomap service registers a callback that, for definition-related responses, fires a textDocument/references to find cross-file refs. This keeps lspool unaware of repomap concerns.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (stdlib, `go test ./...`) |
| Quick run command | `go test ./internal/repomap/... ./internal/skill/repomap/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RMAP-04 | Cross-file reference graph from tags | unit | `go test ./internal/repomap/ -run TestBuildGraph -count=1` | Wave 0 |
| RMAP-05 | Personalized PageRank with configurable weights | unit | `go test ./internal/repomap/ -run TestPageRank -count=1` | Wave 0 |
| RMAP-06 | get_repo_map returns token-budgeted overview | unit + integration | `go test ./internal/skill/repomap/ -run TestGetRepoMap -count=1` | Wave 0 |
| RMAP-07 | get_context returns task-focused context | unit + integration | `go test ./internal/skill/repomap/ -run TestGetContext -count=1` | Wave 0 |
| RMAP-08 | LSP enrichment adds cross-file references | unit | `go test ./internal/repomap/ -run TestEnrichFromLSP -count=1` | Wave 0 |
| RMAP-10 | Binary search budget fitting | unit | `go test ./internal/repomap/ -run TestBudgetFitting -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/repomap/... ./internal/skill/repomap/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/repomap/graph_test.go` -- covers RMAP-04 (graph building from mock tags)
- [ ] `internal/repomap/pagerank_test.go` -- covers RMAP-05 (convergence, personalization, dangling nodes)
- [ ] `internal/repomap/render_test.go` -- covers RMAP-10 (binary search budget, tree formatting)
- [ ] `internal/skill/repomap/skill_test.go` -- covers RMAP-06, RMAP-07 (tool dispatch, parameter validation)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | yes | Profile filtering middleware controls tool visibility |
| V5 Input Validation | yes | Validate file paths (no path traversal), token_budget bounds |
| V6 Cryptography | no | -- |

### Known Threat Patterns for Go MCP Tools

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via file paths in get_context | Tampering | Validate all file paths are within workspace root; reject `..` components |
| Excessive token budget causing OOM | Denial of Service | Cap token_budget at a reasonable max (e.g., 32768); reject values > cap |
| SQL injection via file paths in TagCache queries | Tampering | All SQL uses parameterized queries (already enforced in Phase 27) |

## Sources

### Primary (HIGH confidence)
- Phase 27 source code: `internal/repomap/` (tags.go, cache.go, elide.go, extractor.go, fallback.go, schema.go) -- verified via direct code reading
- Skill system: `internal/skill/skill.go`, `internal/skill/registry.go`, `internal/skill/memory/skill.go` -- verified via direct code reading
- Daemon bootstrap: `internal/daemon/daemon.go`, `internal/daemon/imports.go` -- verified via direct code reading
- MCP tool registration: `internal/mcp/server.go`, `internal/mcp/registry.go` -- verified via direct code reading
- LSP adapter: `internal/kernel/lspool/adapter.go` -- verified via direct code reading

### Secondary (MEDIUM confidence)
- [Aider RepoMap architecture](https://deepwiki.com/Aider-AI/aider/4.1-repository-mapping) -- algorithm details for PageRank, binary search, self-loops
- [Aider repomap.py source](https://github.com/Aider-AI/aider/blob/main/aider/repomap.py) -- binary search implementation, edge construction, personalization weights

### Tertiary (LOW confidence)
- [dcadenas/pagerank](https://github.com/dcadenas/pagerank) -- Go PageRank library (evaluated, rejected for lacking personalization)
- [alixaxel/pagerank](https://github.com/alixaxel/pagerank) -- Go weighted PageRank library (evaluated, rejected for lacking personalization)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all infrastructure exists from Phase 27
- Architecture: HIGH -- algorithm well-documented in aider, patterns verified in codebase
- Pitfalls: HIGH -- identified from both aider source analysis and codebase inspection

**Research date:** 2026-04-16
**Valid until:** 2026-05-16 (stable domain, no fast-moving dependencies)
