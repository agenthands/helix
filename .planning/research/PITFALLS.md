# Pitfalls Research

**Domain:** Context Intelligence (RepoMap) & Resilient Editing (Fuzzy Search/Replace) for Code Intelligence Platform
**Researched:** 2026-04-15
**Confidence:** HIGH (grounded in aider issues, Serena codebase analysis, SQLite concurrency literature)

## Critical Pitfalls

### Pitfall 1: Identifier Collision Destroys PageRank Quality on Large Codebases

**What goes wrong:**
PageRank ranks symbols by how many other symbols reference them. But the graph is built on identifier name matching -- tree-sitter finds `getName` in file A, sees `getName` referenced in file B, draws an edge. In large codebases, common method names like `getName`, `toString`, `Close`, `String`, `Error` appear in hundreds of types. The graph connects them all, creating false hubs. Symbols with common names AND no outbound edges (leaf classes) absorb enormous PageRank and dominate top results despite being irrelevant. Aider issue #2341 documented this exactly: editing Cassandra, "approximately all of the top ranked definitions are artifacts of identifier collision."

**Why it happens:**
Tree-sitter extracts identifiers as flat strings without type/scope qualification. `Foo.getName` and `Bar.getName` both emit the tag `getName`. The graph treats them as the same node, or creates spurious edges between files that share nothing beyond a common method name. This is inherent to tree-sitter -- it lacks semantic type resolution.

**How to avoid:**
1. Use qualified identifiers: `StructName.MethodName` not just `MethodName`. Tree-sitter can extract the parent node's name field alongside the method -- Serena's `extractNodeName` in `treesitter.go` already does this for body extraction. Apply the same pattern to tag extraction.
2. Filter high-frequency identifiers. If an identifier appears in >N% of files (configurable, start at 5%), exclude it from graph edges -- treat it as a "stop word." Aider issue #2342 proposed this exact approach.
3. Use LSP-enriched edges when the worker pool has a warm session. LSP `textDocument/references` gives precise, type-aware edges. Build the graph from tree-sitter fast, then upgrade edges opportunistically from LSP.
4. Personalize PageRank toward the files the agent is working on, so common-name hubs get diluted by task relevance.

**Warning signs:**
- Top-ranked symbols in RepoMap output are generic methods (getters, interface methods) rather than domain-specific entry points.
- RepoMap output is nearly identical regardless of what task context is provided.
- Benchmark: run RepoMap against Serena's own codebase and verify top results are sensible for a given query.

**Phase to address:**
Graph construction / tag extraction phase. Must be correct before any ranking happens. Build qualification into tag extraction from day one -- retrofitting is expensive because cached graph data becomes invalid.

---

### Pitfall 2: Fuzzy Matching Silently Applies Edit to Wrong Location

**What goes wrong:**
An LLM produces a search/replace block where the search text has minor whitespace or formatting drift from the actual file. The fuzzy matcher finds a "close enough" match -- but it's the wrong occurrence. The edit silently corrupts a different function, different branch of a conditional, or different method overload. The agent reports success. The user discovers the bug hours later, potentially after further edits have stacked on top.

This is the single highest-severity pitfall because it violates Serena's core value of being rock-solid. A tool that silently corrupts code is worse than a tool that refuses to apply an edit.

**Why it happens:**
1. Multiple similar code blocks exist in the same file (duplicate patterns, overloaded methods, similar test cases).
2. The fuzzy matcher uses a similarity threshold without requiring uniqueness -- it returns the first match above threshold, not the only match.
3. Aider's `str.replace()` bug (issue #3883, April 2025) replaced ALL occurrences instead of the first. Even replacing the first is wrong when the intended target was the second.
4. LLMs produce minimal context in search blocks, making disambiguation impossible.
5. Serena's existing `fileops/replace.go` uses `strings.ReplaceAll` -- it already replaces all occurrences. This must not carry over to fuzzy matching.

**How to avoid:**
1. **Uniqueness check is mandatory.** If the fuzzy matcher finds >1 candidate above threshold, refuse the edit and return all candidates with line numbers. Never silently pick one.
2. **Return match metadata.** Every fuzzy edit result must report: match line range, similarity score, number of candidates found, which strategy matched (exact/whitespace-normalized/fuzzy). The agent can then decide.
3. **Require minimum context.** If search text is fewer than 3 lines, require either a line number hint or a symbol name anchor. Single-line fuzzy matches are the most dangerous.
4. **Layered matching with strict preference.** Try exact first, then whitespace-normalized, then fuzzy. If exact matches, never fall through to fuzzy. Report which layer matched.
5. **Diff preview in tool response.** Return the before/after diff snippet so the agent (and user) can verify.

**Warning signs:**
- Tests pass but code behavior changes unexpectedly after fuzzy edits.
- Fuzzy edit tool reports "1 replacement made" but the change is in an unexpected function.
- Similarity scores cluster tightly (e.g., three candidates at 0.92, 0.91, 0.90) -- threshold cannot disambiguate.

**Phase to address:**
Fuzzy edit implementation phase. Uniqueness check and match metadata must be in the initial design. Integration tests must include "ambiguous match" scenarios from day one.

---

### Pitfall 3: Cache Invalidation Race Between Graph Cache, File System, and LSP State

**What goes wrong:**
RepoMap caches the parsed symbol graph (tags per file) in SQLite, keyed by file path + mtime. An agent edits a file via Serena's edit tools, which writes the file and notifies the LS via `didChange`. But the RepoMap cache still holds stale tags for that file. If another tool call requests RepoMap context before the cache invalidates, the graph contains phantom symbols (deleted functions still ranked) or misses new symbols. Worse: the LS might have a different view of the file than what's on disk if `didChange` and the file write race.

**Why it happens:**
Three independent state sources: filesystem (mtime), SQLite tag cache, and LS in-memory buffers. Serena already has this pattern with the memory system (`fsnotify` watcher + SQLite index), but RepoMap adds a fourth: the in-memory graph structure derived from cached tags. Each has its own invalidation timing.

The existing `edit/replace.go` writes to disk then calls `notifyDidChange` -- but nothing invalidates a hypothetical RepoMap cache. The memory system's `watcher.go` uses fsnotify for external changes but not for changes made by Serena itself.

**How to avoid:**
1. **Single invalidation bus.** Create a file-change notification channel that all Serena edit operations publish to. RepoMap cache, memory index, and LS notifications all subscribe. This fires synchronously after the write, before returning success to the agent.
2. **Lazy invalidation, not eager rebuild.** Mark cached tags as stale (delete from cache or bump a generation counter). Rebuild tags on next read. Do not re-parse on every edit -- agents often make rapid sequential edits.
3. **mtime is not reliable enough.** Filesystem mtime granularity is 1 second on many systems. Two rapid writes within the same second produce the same mtime, so cache thinks file hasn't changed. Use content hash -- Serena already does `contentHash` in `memory/index.go`. Apply the same pattern to RepoMap tag cache.
4. **Version the graph.** Every graph query returns a generation counter. If the graph has been invalidated since the agent started its workflow, report it.

**Warning signs:**
- RepoMap shows symbols that were recently deleted or renamed.
- RepoMap misses symbols that were just added.
- Flaky tests where RepoMap results depend on timing.

**Phase to address:**
Cache infrastructure phase (before or alongside graph construction). The invalidation bus design must be settled before building the cache.

---

### Pitfall 4: Token Budget Estimation Diverges Across Models

**What goes wrong:**
RepoMap's context selection tool must fit output within a token budget. But different LLMs use different tokenizers: Claude uses its own BPE, GPT-4 uses cl100k_base/o200k_base, open-source models use various SentencePiece variants. If Serena estimates tokens using one tokenizer but the consuming model uses another, the output either wastes 15-20% of available context (over-cautious) or exceeds the limit and gets truncated (under-cautious). Truncation mid-symbol-definition corrupts the context.

**Why it happens:**
Serena has `get_token_budget` in profiles, but that reports budget limits -- it doesn't provide a tokenizer. There is no universal token counter. The profile system knows which agent type is connected (claude-code, codex, ide-assistant), but this doesn't map to a specific tokenizer. Anthropic's official tokenizer requires an API call; tiktoken only works for OpenAI models; estimates using wrong tokenizer are 5-15% off.

**How to avoid:**
1. **Use a character-based heuristic, not a tokenizer.** For code, 1 token approximately equals 4 characters across all major tokenizers (within 10-15% accuracy). This is good enough for budget allocation.
2. **Build with a safety margin.** Target 85-90% of the declared budget. The 10-15% cushion absorbs tokenizer variance.
3. **Make the estimator pluggable.** `TokenEstimator` interface with a default `CharBasedEstimator` (chars/4). If a specific model's tokenizer becomes available in Go, swap it in. Do not build a Go port of tiktoken as a prerequisite.
4. **Report estimated token count in output.** Let the agent know "this output is ~2,400 estimated tokens."

**Warning signs:**
- Agents frequently report context window overflow when using RepoMap output.
- RepoMap consistently returns much less content than the budget allows.
- Token estimate accuracy varies dramatically between agent profiles.

**Phase to address:**
Context selection phase. The estimator must exist before the selection algorithm, because the algorithm makes budget decisions during tree construction.

---

### Pitfall 5: Graph Construction Blows Memory/CPU on Large Repos

**What goes wrong:**
Building a full symbol graph requires parsing every file with tree-sitter, extracting all definitions and references, and constructing an adjacency structure. For a repo with 10,000+ files (e.g., Kubernetes at ~25K Go files), this means: parsing every file, storing O(files x symbols_per_file) nodes, storing O(references) edges. Naive implementation loads everything into memory, parses sequentially, and blocks the first RepoMap call for 30+ seconds.

**Why it happens:**
Developers test against small repos (Serena itself is ~35K LOC, manageable). The scaling cliff appears at 10K+ files where memory exceeds hundreds of MB and parse time exceeds user tolerance. Aider caches tags in SQLite via diskcache but still reports slow initial scans on large repos.

**How to avoid:**
1. **Incremental, file-at-a-time construction.** Parse files lazily on first access. Build the graph progressively. Never require a full-repo parse before returning results.
2. **SQLite-backed tag storage from day one.** Store parsed tags in SQLite (path, symbol, kind, line, references). This survives daemon restarts and avoids re-parsing unchanged files. Serena already uses SQLite for memory FTS5 -- reuse the same pattern.
3. **Background warming.** After initial on-demand parsing, start a background goroutine to parse remaining files. Use the lspool's existing pressure-aware patterns -- if memory pressure is high, pause warming.
4. **Cap the graph.** For repos with >50K files, only include files matching language registry entries. Skip vendored, generated, test fixture files. Use `.gitignore` patterns. Provide `.serena/repomap.yml` for include/exclude overrides.
5. **Benchmark against a 10K+ file repo early.** Don't wait until release to discover scaling issues.

**Warning signs:**
- RepoMap tool call latency exceeds 5 seconds on repos >5K files.
- Daemon RSS jumps by >200MB during graph construction.
- Background parsing saturates CPU and degrades concurrent LS operations.

**Phase to address:**
Graph construction phase. The storage backend (SQLite vs. in-memory) decision must be made upfront because it determines the entire caching and invalidation architecture.

---

### Pitfall 6: SQLite Locking Contention Between Memory Index, RepoMap Cache, and Concurrent Tool Calls

**What goes wrong:**
Serena's daemon handles concurrent MCP tool calls. The existing memory system uses SQLite with WAL mode and a Go `sync.Mutex` around every operation (see `memory/index.go` lines 31-32). Adding a RepoMap tag cache to the same or a separate SQLite database introduces more write contention. If a RepoMap background warming goroutine is writing tags while an agent searches memories AND another agent requests RepoMap context, the mutex serializes everything. With `busy_timeout=5000`, a blocked write waits up to 5 seconds before failing.

**Why it happens:**
SQLite WAL allows concurrent readers with one writer, but the existing `Index.mu sync.Mutex` in `memory/index.go` serializes ALL operations (reads and writes) behind a single lock -- this is safe but removes SQLite's natural read concurrency. Adding more SQLite consumers amplifies the problem.

**How to avoid:**
1. **Use separate SQLite databases.** Memory index and RepoMap tag cache should be in different `.db` files. SQLite's locking is per-database-file; separate files eliminate cross-feature write contention entirely.
2. **Use `sync.RWMutex` instead of `sync.Mutex`.** The existing memory index uses a plain Mutex, serializing reads behind writes. An RWMutex allows concurrent reads (which is what SQLite WAL supports natively). Apply this to both the existing memory index and the new RepoMap cache.
3. **Keep write transactions short.** Batch tag upserts into transactions of 100-500 rows, not one transaction per file or one giant transaction for the whole repo. Commit between batches to release the write lock.
4. **Use `BEGIN IMMEDIATE` for writes.** This acquires the write lock at transaction start rather than at first write statement, preventing SQLITE_BUSY surprises mid-transaction.

**Warning signs:**
- "database is locked" errors in logs during concurrent tool calls.
- Tool call latency spikes when RepoMap is warming in the background.
- Memory search operations slow down after RepoMap feature is added.

**Phase to address:**
Infrastructure/cache phase. The database separation decision must happen before implementing the tag cache. Upgrading the existing memory index Mutex to RWMutex should be a preparatory step.

---

### Pitfall 7: Tree-Sitter Query Maintenance Burden for 52 Languages

**What goes wrong:**
Serena currently has tree-sitter grammars for 4 languages (Go, Python, TypeScript, Rust) used for body extraction in edit tools (see `internal/kernel/edit/queries/`). RepoMap needs tree-sitter parsing for tag extraction across all 52 supported languages. Writing and maintaining `.scm` query files (or programmatic AST walks) for each language's declaration/reference patterns is enormous ongoing work. Each language has different node type names (`function_declaration` in Go, `function_definition` in Python, `function_item` in Rust), different field names, and different AST structures.

**Why it happens:**
Tree-sitter grammars are maintained by different communities with different conventions. There's no standard "declaration" node type. Each grammar has its own field naming, and these change between versions. Aider handles this by supporting 130+ languages through tree-sitter but with varying quality.

**How to avoid:**
1. **Two-tier approach.** Tier 1 (Go, Python, TypeScript, Rust, Java, C/C++) gets curated `.scm` queries with full definition/reference extraction. Tier 2 (remaining 46 languages) gets a generic fallback: extract only top-level named nodes using tree-sitter's `named_children` iterator without language-specific queries.
2. **Leverage existing aider/tree-sitter-tags queries.** Aider has tag queries for many languages, BSD-licensed. Port the top 10-15 and use generic fallback for the rest.
3. **Pin tree-sitter grammar versions in `go.mod`.** When upgrading a grammar, run tag extraction tests to catch broken queries before release.
4. **Abstract the tag extraction interface.** `TagExtractor` interface with `ExtractTags(source []byte, lang string) ([]Tag, error)`. Implementations can be tree-sitter, LSP-based, or regex-based. This decouples RepoMap from tree-sitter specifics and allows graceful degradation.

**Warning signs:**
- RepoMap returns empty or nonsensical results for specific languages.
- Grammar upgrade PRs break tag extraction without obvious test failures.
- Engineers avoid adding new language support because query maintenance is too high.

**Phase to address:**
Tag extraction phase. The two-tier decision and interface design must be made before implementing individual language queries.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| In-memory graph only (no SQLite cache) | Simpler implementation, faster iteration | Re-parse on every daemon restart, O(seconds) cold start on large repos | Never for production; acceptable for prototype/proof-of-concept only |
| Single tokenizer (chars/4 heuristic) | No external dependencies, works for all models | 10-15% inaccuracy, occasional over/under-budget | Acceptable permanently with safety margin -- over-engineering tokenizer matching is worse |
| Unqualified identifiers in graph | Simpler tag extraction, fewer tree-sitter queries | PageRank quality degrades on large codebases (aider #2341) | Never -- qualification is cheap at extraction time, expensive to retrofit |
| Same SQLite DB for memory + repomap | One fewer file to manage | Write contention under concurrent load | Only if RepoMap writes are infrequent (lazy invalidation); separate DBs are safer |
| Skip fuzzy matching uniqueness check | Faster implementation, higher match rate | Silent wrong-location edits, trust erosion | Never -- this is the single most dangerous shortcut |
| Replace-all semantics in fuzzy edit | Simpler code, matches existing fileops/replace.go pattern | Replaces every occurrence when only one was intended (aider #3883) | Never for fuzzy matching -- only for explicit regex-based replace_in_file |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| RepoMap + lspool worker lease | Holding a worker lease during graph construction (blocks pool for other tools) | Use tree-sitter for initial graph (no LS needed). Only acquire lease for optional LSP enrichment, release immediately after each query. |
| Fuzzy edit + didChange notification | Sending didChange with content from before the fuzzy match resolved, or sending stale content | Read file, apply fuzzy edit, write file, THEN send didChange with final file content. Never send intermediate states. |
| RepoMap cache + memory fsnotify watcher | Both watching the same directories, duplicating filesystem events, creating thundering herd on batch edits | Single watcher (or single event bus) that dispatches to both cache invalidation and memory index. Debounce: coalesce events within 100ms window. |
| PageRank + personalization vector | Personalization vector sums to 0 (all queried files excluded from repo) or contains files not in graph | Validate personalization vector: if queried files aren't in graph, fall back to uniform distribution. Log a warning. |
| Token budget + RepoMap output formatting | Counting tokens of raw data but outputting formatted markdown with headers, separators, indentation | Count tokens of the formatted output, not the raw symbol data. Format first, measure second. Or reserve a fixed overhead (200 tokens) for formatting chrome. |
| Fuzzy edit + existing replace_symbol_body | Adding fuzzy fallback to replace_symbol_body without considering tree-sitter body extraction failure | Fuzzy matching should only activate AFTER tree-sitter + LSP have both failed to locate the target. It's a last resort, not a first attempt. The existing tree-sitter path is more precise. |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full-repo tree-sitter parse on first RepoMap call | 10-30s hang on first tool call, agent timeout | Incremental/lazy parsing with background warming | >5K files |
| PageRank convergence iteration without cap | CPU spike, 100% core utilization for seconds | Cap iterations at 100 (standard). Use power iteration with tolerance 1e-6. Code graphs typically converge in 20-40 iterations. | >50K nodes |
| Fuzzy matching with Levenshtein on large files | O(n*m) per comparison, seconds per match on 10K-line files | Use line-based chunking first (find candidate regions by line hash or trigram), then run edit distance only on candidates | >5K lines per file |
| Rebuilding full graph on every file change | CPU saturation during rapid edit sequences | Invalidate only the changed file's tags. Defer PageRank recomputation until next query (lazy). | Any batch edit (>3 files) |
| Storing full file content in tag cache | SQLite DB grows to hundreds of MB | Store only tags (name, kind, line, references), not file content. File content lives on disk. | >10K files |
| Creating new tree-sitter parser per file parse | GC pressure from allocating/freeing parser objects at scale | Pool tree-sitter parsers per language, reuse across file parses. Serena already pools LS workers -- apply same pattern. | >1K files parsed in a warming batch |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Fuzzy edit matching across file boundaries | Agent sends search text; fuzzy matcher finds match in a different file than intended and edits it | Fuzzy edit must require explicit file path. Never search across files unless explicitly designed for that. |
| RepoMap exposing files outside workspace | Graph includes files from parent directories, vendored dependencies, or symlink targets outside workspace root | Validate all paths against workspace root before including in graph. Use the existing `ValidatePath` from fileops. |
| Tag cache poisoned by crafted file content | A file with crafted identifier names causes SQL injection into tag cache, or causes tree-sitter to consume excessive memory | Use parameterized queries (already done in memory/index.go). Set tree-sitter parse timeout. Limit identifier length in tag extraction. |
| Fuzzy edit applied to file with uncommitted changes from another agent | Two concurrent agents editing same file; fuzzy match is based on stale pre-read content | Read file content immediately before matching, not from cache. Use the same atomic-write pattern as existing edit tools. |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Fuzzy edit silently succeeds with low confidence | User trusts tool output, doesn't review, code is wrong | Always report match confidence. If score < 0.95, include a warning: "Low confidence match at line X. Please review." |
| RepoMap returns wall of text without structure | Agent spends tokens parsing unstructured output, misses key symbols | Return structured output: ranked list with file path, symbol name, kind, line number, relevance score. Consistent formatting the agent can parse. |
| "No match found" without guidance | Agent retries with same input, wastes turns | On no-match, return: closest candidate (with score), actual content around expected location, suggestion ("Did you mean line 42-47?") |
| RepoMap budget exhausted by a few large files | Important small files with key interfaces get crowded out by large implementation files | Rank by symbol importance, not file size. Include at least the signature (not body) of every top-ranked symbol, then fill remaining budget with bodies of the most important ones. |
| Fuzzy edit returns opaque "match failed" | Agent cannot diagnose why the match failed or how to fix the search text | Return: (a) the best candidate with its score, (b) the character-level diff between search text and best candidate, (c) whether whitespace normalization would have matched |

## "Looks Done But Isn't" Checklist

- [ ] **Fuzzy edit:** Often missing uniqueness enforcement -- verify that ambiguous matches are rejected, not silently resolved
- [ ] **Fuzzy edit:** Often missing the "which strategy matched" metadata in tool response -- verify exact vs. whitespace-normalized vs. fuzzy is reported
- [ ] **Fuzzy edit:** Often missing integration with existing `replace_symbol_body` / `replace_content` -- verify fallback chain works end-to-end through the existing edit tools, not just the standalone fuzzy tool
- [ ] **RepoMap:** Often missing personalization -- verify that task context actually changes rankings, not just filters results
- [ ] **RepoMap:** Often missing incremental invalidation -- verify that editing a file actually updates the next RepoMap call's output
- [ ] **Token budget:** Often missing the formatting overhead -- verify token count includes markdown/structure chrome, not just raw content
- [ ] **Graph construction:** Often missing .gitignore filtering -- verify vendored/generated files are excluded
- [ ] **Cache:** Often missing content-hash-based invalidation -- verify rapid same-second edits don't produce stale results
- [ ] **Integration:** Often missing the "RepoMap cache is stale after Serena's own edits" scenario -- verify internal edit -> cache invalidation -> fresh RepoMap round-trip
- [ ] **Fuzzy edit in edit tools:** Often missing the "tree-sitter succeeded but LSP range is stale" case -- verify fallback order is tree-sitter -> LSP range -> fuzzy, not tree-sitter -> fuzzy (skipping LSP)

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Wrong-location fuzzy edit applied | MEDIUM | Git diff shows the damage. Revert file, apply edit with explicit line range. If caught immediately: clean undo. If discovered after stacking edits: painful manual merge. |
| PageRank quality is bad (identifier collision) | HIGH | Requires changing tag extraction format (add qualification), invalidating entire cache, re-parsing all files. If graph schema changed: SQLite migration. |
| SQLite locking causes tool timeouts | LOW | Split databases. Change Mutex to RWMutex. Both are backward-compatible changes. |
| Token budget overflow truncates context | LOW | Reduce budget target to 80%. Adjust heuristic multiplier. No data loss, just suboptimal context. |
| Graph construction OOM on large repo | MEDIUM | Add file count cap, exclude patterns. Requires config surface. May need to re-architect storage if in-memory-only. |
| Tree-sitter query breaks on grammar update | LOW | Pin grammar version, revert update, fix query. No data corruption, just degraded tag quality for that language. |
| Cache invalidation race produces stale RepoMap | LOW | Force cache rebuild via admin endpoint or tool. Add content-hash check. No data loss -- stale results are transient. |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Identifier collision (P1) | Graph construction / tag extraction | Benchmark: run against a 10K+ file repo, verify top-10 ranked symbols are domain-relevant, not generic getters |
| Silent wrong-location edit (P2) | Fuzzy edit implementation | Test: file with 3 similar functions, fuzzy match returns error not silent pick |
| Cache invalidation race (P3) | Cache infrastructure (before graph) | Test: edit file via Serena tool, immediately call RepoMap, verify new symbol appears |
| Token budget divergence (P4) | Context selection implementation | Test: measure actual token consumption (via API response) vs. estimated, verify <15% error |
| Graph memory/CPU (P5) | Graph construction | Benchmark: 10K-file repo, verify <500MB RSS, <5s first-call latency |
| SQLite locking (P6) | Infrastructure / preparatory | Load test: 10 concurrent tool calls mixing memory search + RepoMap, verify no SQLITE_BUSY |
| Tree-sitter 52-language burden (P7) | Tag extraction design | Ship with tier-1 (6 langs) curated + tier-2 generic fallback, verify both tiers return non-empty results |

## Sources

- [Aider RepoMap identifier uniqueness issue #2341](https://github.com/Aider-AI/aider/issues/2341) -- HIGH confidence, direct bug report with reproduction
- [Aider RepoMap rank distribution issue #2342](https://github.com/Aider-AI/aider/issues/2342) -- HIGH confidence, companion fix proposal
- [Aider search/replace replacing all matches #3883](https://github.com/Aider-AI/aider/issues/3883) -- HIGH confidence, confirmed bug
- [Aider search/replace partial line matching #1294](https://github.com/paul-gauthier/aider/issues/1294) -- MEDIUM confidence
- [Aider diff-fenced format long line failures #4716](https://github.com/aider-ai/aider/issues/4716) -- MEDIUM confidence
- [Code Surgery: How AI Assistants Make Precise Edits (Fabian Hertwig)](https://fabianhertwig.com/blog/coding-assistants-file-edits/) -- HIGH confidence, comparative analysis
- [Aider RepoMap documentation](https://aider.chat/docs/repomap.html) -- HIGH confidence
- [Aider tree-sitter repomap blog post](https://aider.chat/2023/10/22/repomap.html) -- HIGH confidence
- [Repository Mapping System (DeepWiki)](https://deepwiki.com/Aider-AI/aider/4.1-repository-mapping) -- MEDIUM confidence
- [SQLite concurrent writes and locking](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/) -- HIGH confidence
- [Token counting across models (Propel 2025)](https://www.propelcode.ai/blog/token-counting-tiktoken-anthropic-gemini-guide-2025) -- MEDIUM confidence
- [Counting Claude tokens without a tokenizer](https://blog.gopenai.com/counting-claude-tokens-without-a-tokenizer-e767f2b6e632) -- MEDIUM confidence
- [Cursor silent code reversion bug (2026)](https://vibecoding.app/blog/cursor-problems-2026) -- LOW confidence (third-party reporting)
- Serena codebase analysis: `internal/kernel/edit/treesitter.go`, `internal/kernel/edit/replace.go`, `internal/memory/index.go`, `internal/kernel/fileops/replace.go`, `internal/kernel/edit/queries/` -- HIGH confidence, direct code review

---
*Pitfalls research for: Context Intelligence & Resilient Editing (v1.6)*
*Researched: 2026-04-15*
