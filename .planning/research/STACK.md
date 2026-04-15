# Technology Stack

**Project:** Serena v1.6 -- Context Intelligence & Resilient Editing
**Researched:** 2026-04-15

## Recommended Stack Additions

### Graph Ranking (PageRank)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **No library -- implement in-house (~150 LOC)** | N/A | Personalized PageRank for symbol importance ranking | Gonum's `graph/network.PageRank` (v0.17.0) lacks personalized PageRank (no personalization vector parameter). The aider reference implementation requires personalization to bias ranking toward files the agent is actively working with. Third-party alternatives (alixaxel/pagerank, dcadenas/pagerank) also lack personalization and are unmaintained (last updated 2020). The max-planck-innovation-competition/pagerank has personalized PageRank but is a niche academic project with minimal adoption. A power-iteration PageRank with personalization vector is ~150 lines of Go with no external dependencies -- simpler than pulling in gonum's entire graph module for a function we'd need to fork anyway. |

**Implementation notes:**
- The core algorithm is power iteration over a sparse adjacency matrix: `r = d * M * r + (1-d) * p` where `p` is the personalization vector
- Use `map[string]map[string]float64` for the weighted directed graph (files are nodes, def/ref relationships are edges) -- no need for gonum's interface-heavy graph types
- The aider reference (borrow/aider/aider/repomap.py lines 460-530) uses networkx MultiDiGraph with personalization dict and weight parameter
- Edge weights encode: identifier quality heuristics (camelCase/snake_case bonus, underscore-prefix penalty, high-fan-out penalty) multiplied by sqrt(ref_count), boosted 50x for files in active context
- Convergence: iterate until L2 norm of rank delta < tolerance (1e-6), typically 20-40 iterations for codebases up to 100K files

**Why NOT gonum:**
- `gonum.org/v1/gonum/graph/network.PageRank(g graph.Directed, damp, tol float64) map[int64]float64` -- no personalization parameter (verified via source at github.com/gonum/gonum/blob/master/graph/network/page.go)
- Would need gonum's graph.Directed interface, simple.WeightedDirectedGraph, and int64 node IDs -- all overhead for what is a ~150 LOC algorithm operating on string-keyed maps
- gonum pulls in matrix/linear algebra packages that are irrelevant here

### Token Counting / Budgeting

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **tiktoken-go/tokenizer** | v0.7.0+ | Accurate BPE token counting for context budgeting | Pure Go, no CGO, embeds vocabularies (~4MB compiled). Supports cl100k_base and o200k_base encodings. API: `enc.Count(text)` returns token count. Claude's tokenizer shares ~70% vocabulary overlap with cl100k_base, making it a reasonable proxy for budget estimation. |

**Integration notes:**
- The existing `computeTokenBudget` in `internal/profile/skill.go` uses `len(text) / 4` as a crude estimate -- this is fine for tool schema budgets but too imprecise for RepoMap context selection where we're fitting ranked symbols into a token window
- Use `tiktoken-go/tokenizer` with `Cl100kBase` encoding for the RepoMap context budgeting tool only -- it provides ~85-90% accuracy vs Anthropic's actual tokenizer for English code
- For the RepoMap overview tool (structural map), keep the simple `len/4` heuristic -- exact counts don't matter when the output is a fixed-format tree
- The aider reference uses sampling-based token counting (sample 1% of lines, extrapolate) for performance on large texts -- replicate this pattern: use exact counting for texts < 1KB, sampled counting for larger texts
- Binary size impact: ~4MB for embedded vocabularies -- acceptable for a server binary

**Why tiktoken-go/tokenizer over pkoukk/tiktoken-go:**
- tiktoken-go/tokenizer embeds vocabularies at compile time (no runtime downloads, no cache directory, no network dependency)
- pkoukk/tiktoken-go downloads dictionaries at runtime -- unacceptable for a persistent daemon that may run in airgapped environments
- Both support the same encodings; tiktoken-go/tokenizer has cleaner API (`Count` method vs manual encode-and-count)

**Why NOT Anthropic's API-based counting:**
- Requires network call to Anthropic API -- adds latency and external dependency
- Token counting is for budget estimation, not billing -- ~10% variance is acceptable
- The daemon serves multiple agent types (Claude Code, Codex, IDE assistants) -- need a universal estimate, not provider-specific

### Fuzzy Text Matching

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **sergi/go-diff** | v1.4.0 | Fuzzy matching and patching for resilient edits | Go port of Google's diff-match-patch. Provides `MatchMain` (Bitap fuzzy matching), `DiffMain` (diff computation), and `PatchApply` (fuzzy patch application). Configurable `MatchThreshold`, `MatchDistance`, `MatchMaxBits`. Used by 2,554 Go packages. MIT licensed. |

**Integration notes:**
- The aider reference (borrow/aider/aider/coders/search_replace.py) uses diff_match_patch for fuzzy search/replace with two strategies:
  1. Tight matching (`MatchThreshold=0.95`, `MatchDistance=500`) with relative-indent remapping
  2. Loose matching (`MatchThreshold=0.5`, `MatchDistance=100000`) as fallback
- For Serena's fuzzy edit fallback: implement a 3-tier strategy:
  1. **Exact match** -- `strings.Contains` on original content (current behavior)
  2. **Whitespace-normalized match** -- strip/normalize whitespace, match, map back to original offsets (custom, ~80 LOC)
  3. **Fuzzy match** -- `go-diff/diffmatchpatch.MatchMain` with configurable threshold
- The whitespace normalization layer (tier 2) handles the most common LLM drift pattern (incorrect indentation) without needing the full diff-match-patch machinery
- For the standalone fuzzy edit MCP tool: expose the strategy used in the response so agents know confidence level
- `PatchApply` returns `(string, []bool)` -- the bool slice indicates which hunks applied successfully, critical for reporting partial application

**Whitespace normalization (implement in-house, ~80 LOC):**
- Collapse runs of spaces/tabs to single space
- Strip trailing whitespace per line
- Normalize line endings to `\n`
- Build offset mapping from normalized positions back to original positions
- This handles 70-80% of LLM output drift (indentation changes, trailing whitespace) before needing fuzzy matching

### SQLite Tags Cache

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **modernc.org/sqlite** (reuse existing) | v1.48.1 | Cache tree-sitter extracted symbol tags with mtime invalidation | Already a direct dependency. Same WAL mode + busy_timeout pattern proven in `internal/memory/index.go`. CGO-free. No new dependency needed. |

**Integration notes:**
- The aider reference uses `diskcache.Cache` (SQLite-backed key-value store) for tags caching with mtime-based invalidation
- Implement a dedicated tags cache in the RepoMap package using the same `database/sql` + `modernc.org/sqlite` pattern as `internal/memory/`
- Schema:

```sql
CREATE TABLE IF NOT EXISTS tags (
    file_path TEXT NOT NULL,
    mtime     REAL NOT NULL,
    name      TEXT NOT NULL,
    kind      TEXT NOT NULL,  -- 'def' or 'ref'
    line      INTEGER NOT NULL,
    PRIMARY KEY (file_path, name, kind, line)
);
CREATE INDEX IF NOT EXISTS idx_tags_file ON tags(file_path);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
```

- On cache lookup: compare stored mtime vs current file mtime; on mismatch, re-parse with tree-sitter and upsert
- Batch inserts within a transaction for performance (DELETE WHERE file_path=? then INSERT batch)
- Store cache in `.serena/cache/tags.db` (project-scoped) -- aligned with existing config directory convention
- WAL mode is essential for concurrent reads during map generation while background tag updates proceed

### Tree-Sitter Tag Extraction (reuse existing)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **go-tree-sitter** (reuse existing) | v0.25.0 | Extract definition and reference tags from source files | Already used in `internal/kernel/edit/` for body extraction. Same parser creation pattern. Need to add tree-sitter query files (.scm) for tag extraction -- these are different from the body extraction queries. |

**Integration notes:**
- The existing `BodyExtractor` in `internal/kernel/edit/treesitter.go` demonstrates the parser lifecycle pattern: create parser, set language, parse source, walk AST, close
- For tag extraction, use tree-sitter's query API (`tree_sitter.NewQuery`, `QueryCursor`) with `.scm` query files that define `@name.definition.*` and `@name.reference.*` captures
- The aider reference uses `tags.scm` query files per language (from the tree-sitter-languages pack)
- Tag query files for Go, Python, TypeScript, Rust are available in tree-sitter grammar repos -- embed as Go string constants or as embedded files via `//go:embed`
- Fallback for languages without tree-sitter queries: use Pygments-style lexer tokenization (identify identifiers by token type) -- this is what aider does for unsupported languages
- For languages without tree-sitter grammars: naive identifier extraction via regex (`\b[A-Za-z_]\w+\b`) filtered through a stop-word list -- good enough for PageRank edges

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Graph ranking | In-house PageRank (~150 LOC) | gonum/v1/gonum/graph/network | No personalization vector support; heavy dependency for a single function |
| Graph ranking | In-house PageRank | alixaxel/pagerank | Unmaintained (2020), no personalization, uint32 node IDs (we need strings) |
| Graph ranking | In-house PageRank | max-planck-innovation-competition/pagerank | Niche academic project, minimal adoption, Gauss-Seidel method (power iteration is simpler to reason about) |
| Token counting | tiktoken-go/tokenizer | pkoukk/tiktoken-go | Downloads vocabularies at runtime; runtime network dependency unacceptable for daemon |
| Token counting | tiktoken-go/tokenizer | len(text)/4 heuristic | Too imprecise for context budgeting (off by 20-40% on code with many short identifiers) |
| Token counting | tiktoken-go/tokenizer | Anthropic API counting | Network dependency, latency, provider-specific |
| Fuzzy matching | sergi/go-diff | agnivade/levenshtein | Edit distance only, no patch application or fuzzy search-in-text |
| Fuzzy matching | sergi/go-diff | hbollon/go-edlib | String similarity metrics only, no patch/apply workflow |
| Tags cache | modernc.org/sqlite (existing) | bbolt/bolt | Already have SQLite; adding another embedded DB increases complexity |
| Tags cache | modernc.org/sqlite (existing) | File-based JSON cache | No concurrent access safety, no indexing for tag lookups |

## What NOT to Add

| Library | Reason |
|---------|--------|
| gonum (any package) | Overkill -- only need PageRank, and their implementation lacks personalization |
| networkx Go ports | None exist with equivalent quality; in-house is cleaner |
| Vector/embedding libraries | Out of scope per PROJECT.md ("Augment Context Engine does this better") |
| go-git | Out of scope per PROJECT.md ("GitHub MCP Server handles git") |
| Additional tree-sitter grammar bindings beyond Go/Python/TypeScript/Rust | Add only when specific language demand arises; start with the 4 already compiled in |

## Installation

```bash
# New dependencies (2 packages)
go get github.com/tiktoken-go/tokenizer@latest
go get github.com/sergi/go-diff@v1.4.0

# Existing dependencies (no changes needed)
# modernc.org/sqlite v1.48.1 -- already in go.mod
# go-tree-sitter v0.25.0 -- already in go.mod
# tree-sitter-{go,python,rust,typescript} -- already in go.mod
```

## Dependency Impact

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Direct dependencies | 18 | 20 | +2 |
| Binary size estimate | ~45MB | ~49MB | +4MB (tiktoken vocabularies) |
| CGO required | No | No | No change |
| New transitive deps | 0 | ~1-2 | Minimal (both are leaf packages) |

## Sources

- [gonum graph/network PageRank docs](https://pkg.go.dev/gonum.org/v1/gonum/graph/network) -- verified no personalization parameter (HIGH confidence)
- [gonum PageRank source](https://github.com/gonum/gonum/blob/master/graph/network/page.go) -- confirmed via WebFetch (HIGH confidence)
- [tiktoken-go/tokenizer](https://pkg.go.dev/github.com/tiktoken-go/tokenizer) -- v0.7.0, pure Go, embedded vocabs (HIGH confidence)
- [pkoukk/tiktoken-go](https://github.com/pkoukk/tiktoken-go) -- v0.1.8, runtime downloads (HIGH confidence)
- [sergi/go-diff](https://pkg.go.dev/github.com/sergi/go-diff/diffmatchpatch) -- v1.4.0, 2554 importers (HIGH confidence)
- [Anthropic token counting docs](https://platform.claude.com/docs/en/build-with-claude/token-counting) -- Claude uses BPE with ~70% cl100k overlap (MEDIUM confidence)
- [aider repomap.py reference](borrow/aider/aider/repomap.py) -- PageRank with personalization, SQLite tag cache (HIGH confidence, local code)
- [aider search_replace.py reference](borrow/aider/aider/coders/search_replace.py) -- diff-match-patch fuzzy matching strategies (HIGH confidence, local code)
- [alixaxel/pagerank](https://pkg.go.dev/github.com/alixaxel/pagerank) -- weighted but no personalization, last updated 2020 (HIGH confidence)
