# Phase 33: FallbackExtractor Wiring & Cache Persistence - Research

**Researched:** 2026-04-20
**Domain:** Go / LSP integration / SQLite caching
**Confidence:** HIGH

## Summary

This phase wires the existing `FallbackExtractor` (already implemented and tested in `internal/repomap/fallback.go`) into the production `walkAndExtract` pipeline in `internal/skill/repomap/skill.go`. The FallbackExtractor provides LSP-based `textDocument/documentSymbol` tag extraction for languages that lack tree-sitter grammars. Additionally, the phase must verify that the SQLite `TagCache` survives daemon restarts.

All building blocks exist: `FallbackExtractor` with 5 passing tests, `TagCache` with WAL mode and a persistence test (`TestTagCache_Persistence`), `SymbolRequester` interface matching `WorkerLease.Request` signature, and the `GrammarRegistry.SupportsLanguage()` method for grammar detection. The work is primarily integration wiring -- connecting existing components in `walkAndExtract` and verifying the cache persistence test covers the daemon restart scenario.

**Primary recommendation:** Modify `walkAndExtract` to check `GrammarRegistry.SupportsLanguage(lang)` and, when false, attempt LSP fallback extraction via `FallbackExtractor` using a `WorkerLease` from `lspool.Pool`. Add a `SetFallbackDeps` method to `RepoMapSkill` following the existing `SetEnrichFn` post-init wiring pattern.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-33-01:** Fallback trigger uses registry-based check (`GrammarRegistry.SupportsLanguage`), not reactive error handling
- **D-33-02:** Missing LSP = log debug + return empty tags, no user-facing errors
- **D-33-03:** Cache persistence verified via unit test (SQLite reopen), not daemon lifecycle test
- **D-33-04:** SymbolRequester satisfied by WorkerLease adapter wrapping lspool lease

### Claude's Discretion
- Implementation details of registry lookup (method name, caching of check result)
- Whether WorkerLease adapter is a named type or closure, and how lease acquisition/release is managed within the walk loop

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RMAP-02 | Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction | FallbackExtractor exists with 5 tests; walkAndExtract needs wiring to call it when GrammarRegistry lacks the language; WorkerLease satisfies SymbolRequester interface |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Grammar detection | Skill layer (RepoMapSkill) | treesitter.GrammarRegistry | Registry lookup determines extraction path |
| LSP fallback extraction | repomap.FallbackExtractor | lspool.WorkerLease | FallbackExtractor calls documentSymbol via SymbolRequester |
| Lease acquisition | Daemon wiring (post-init) | lspool.Pool | Pool.AcquireLease provides per-language LS workers |
| Tag caching | repomap.TagCache | SQLite (modernc.org/sqlite) | WAL mode ensures persistence across restarts |
| Walk orchestration | skill/repomap/skill.go | -- | walkAndExtract is the single integration point |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| modernc.org/sqlite | (already in go.mod) | CGO-free SQLite for TagCache | Already used; WAL mode provides crash-safe persistence [VERIFIED: codebase] |
| go-tree-sitter | (already in go.mod) | Grammar registry and tag extraction | Already used for 23 languages [VERIFIED: codebase] |

### Supporting
No new dependencies needed. All components exist in the codebase.

## Architecture Patterns

### System Architecture Diagram

```
walkAndExtract(root)
    |
    v
filepath.WalkDir -->  LangFromExt(path) --> lang
    |
    v
cache.GetOrExtract(path, extractFn)
    |
    v
extractFn:
    |
    +-- registry.SupportsLanguage(lang)?
    |       |
    |       YES --> extractor.Extract(source, path, lang)  [tree-sitter path]
    |       |
    |       NO  --> fallbackExtractor != nil && leaseAcquireFn != nil?
    |                   |
    |                   YES --> acquire lease --> fallback.Extract(ctx, lease, path, uri)
    |                   |                            |
    |                   |                            v
    |                   |                       textDocument/documentSymbol --> def-only tags
    |                   |
    |                   NO --> return nil, nil  [silent skip, debug log]
    |
    v
tags stored in TagCache (SQLite WAL)
    |
    (daemon restart)
    |
    v
NewTagCache(same dbPath) --> tags still present (SQLite persistence)
```

### Recommended Project Structure

No new files needed. Changes touch:
```
internal/skill/repomap/
    skill.go              # walkAndExtract modification, new SetFallbackDeps/fields
internal/repomap/
    cache_test.go         # TestTagCache_Persistence already exists (verify sufficiency)
    fallback.go           # No changes needed (already complete)
```

### Pattern 1: Post-Init Wiring via Setter (existing pattern)

**What:** The daemon wires dependencies into skills after `skill.InitAll()` because the kernel/pool isn't available during Init().
**When to use:** When a skill needs kernel/pool access that isn't in `SkillDeps`.
**Example:**
```go
// Source: internal/daemon/daemon.go:274-285, internal/skill/repomap/skill.go:103-109
// Existing pattern: SetEnrichFn wires LSP enrichment after kernel creation.
// New: SetFallbackDeps wires grammar registry + lease acquire function.

// In RepoMapSkill:
type FallbackDeps struct {
    Registry    *treesitter.GrammarRegistry
    AcquireFn   func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error)
    Extractor   *repomap.FallbackExtractor
}

func (s *RepoMapSkill) SetFallbackDeps(deps FallbackDeps) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.fallbackDeps = &deps
}
```
[VERIFIED: codebase -- SetEnrichFn pattern at skill.go:103-109]

### Pattern 2: Lease-per-Language in Walk Loop

**What:** Each file in the walk may need an LSP connection for a different language. The lease must be acquired per-language, not once for the entire walk.
**When to use:** When walking files across multiple languages that need LSP fallback.
**Example:**
```go
// In walkAndExtract extractFn, for fallback path:
// 1. Build WorkspaceKey with the file's language
// 2. AcquireLease from pool
// 3. Use lease as SymbolRequester
// 4. Release lease after extraction

// The AcquireFn closure captures kernel + wsKey.RepoRoot from daemon context:
acquireFn := func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error) {
    wsKey := workspace.WorkspaceKey{RepoRoot: rootDir, Language: lang}
    lease, err := k.Pool().AcquireLease(ctx, "fallback-extract-"+lang, wsKey, false)
    if err != nil {
        return nil, nil, err
    }
    releaseFn := func() { k.Pool().ReleaseLease("fallback-extract-" + lang) }
    return lease, releaseFn, nil
}
```
[VERIFIED: codebase -- enrichRepoMapFromLSP at daemon.go:546-553 uses same AcquireLease/ReleaseLease pattern]

### Anti-Patterns to Avoid
- **Holding a single lease for entire walk:** Different files need different language servers; one lease serves one language. Acquire/release per-language (or batch per-language group).
- **Modifying SkillDeps struct:** Per Phase 28 decision, RepoMapSkill creates own dependencies. Use setter-based post-init wiring.
- **Returning errors from walkAndExtract for missing LS:** Per D-33-02, missing LS is debug log + empty tags, never an error.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Grammar detection | Custom file extension check | `GrammarRegistry.SupportsLanguage(lang)` | Already exists, thread-safe, maintained with grammar additions [VERIFIED: registry.go:100-106] |
| LSP documentSymbol | Custom LSP client | `FallbackExtractor.Extract()` via `WorkerLease` | Already implemented and tested with 5 cases [VERIFIED: fallback.go + fallback_test.go] |
| Cache persistence | Custom file serialization | `TagCache` with SQLite WAL | Already implemented, tested, handles mtime invalidation [VERIFIED: cache.go + cache_test.go] |

## Common Pitfalls

### Pitfall 1: WorkerLease Request Signature Mismatch
**What goes wrong:** `FallbackExtractor.Extract` expects `SymbolRequester` with `Request(ctx, method, params, result)` but `WorkerLease.Request` has the same signature -- they match.
**Why it happens:** Could be a concern if signatures diverge in future.
**How to avoid:** `WorkerLease` already satisfies `SymbolRequester` interface directly -- no adapter needed. Verify with `var _ repomap.SymbolRequester = (*lspool.WorkerLease)(nil)` compile-time check.
**Warning signs:** Compilation error on interface assertion.

### Pitfall 2: Context Propagation in Walk
**What goes wrong:** `walkAndExtract` currently takes no `context.Context`, but `FallbackExtractor.Extract` and `Pool.AcquireLease` both require one.
**Why it happens:** The original walk only did synchronous tree-sitter parsing (no I/O), so no context was needed.
**How to avoid:** Add `context.Context` parameter to `walkAndExtract` or use `context.Background()` (less ideal). The caller `ensureCache` should propagate context.
**Warning signs:** Using `context.Background()` loses cancellation from tool execution timeout.

### Pitfall 3: Language Name Mapping Between LangFromExt and LangRegistry
**What goes wrong:** `LangFromExt` returns names like "c_sharp", "cpp", "hcl" -- these must match what `langregistry.Registry.Get()` expects, which uses the embedded YAML language names.
**Why it happens:** Two separate name mappings (LangFromExt for repomap, langregistry for LS resolution) may use different strings for the same language.
**How to avoid:** The AcquireFn should use the langregistry's expected name. If names differ, a mapping is needed. However, the enrichment path in daemon.go uses a single wsKey language, suggesting the primary language is already resolved at workspace activation time.
**Warning signs:** AcquireLease returning "no worker" for languages that have LS support.

### Pitfall 4: Lease Session ID Collisions
**What goes wrong:** Using a static session ID like "fallback-extract" for multiple concurrent fallback extractions could cause lease conflicts.
**Why it happens:** `AcquireLease` uses sessionID as a map key in `p.leases`.
**How to avoid:** Use unique session IDs per language or per-file: `fmt.Sprintf("fallback-%s-%d", lang, time.Now().UnixNano())` or at minimum `"fallback-" + lang`.
**Warning signs:** "lease already exists" errors during walk.

## Code Examples

### walkAndExtract with Fallback (target state)
```go
// Source: synthesis from existing codebase patterns
func (s *RepoMapSkill) walkAndExtract(ctx context.Context, root string) error {
    return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil
        }
        if d.IsDir() && skipDirs[d.Name()] {
            return filepath.SkipDir
        }
        if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
            return nil
        }

        lang := repomap.LangFromExt(path)
        if lang == "" {
            return nil
        }

        _, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
            // Primary path: tree-sitter extraction
            if s.extractor != nil && s.registry.SupportsLanguage(lang) {
                source, readErr := os.ReadFile(path)
                if readErr != nil {
                    return nil, readErr
                }
                tags, tagErr := s.extractor.Extract(source, path, lang)
                if tagErr != nil {
                    s.logger.Debug("tag extraction skipped", "path", path, "lang", lang, "error", tagErr)
                    return nil, nil
                }
                return tags, nil
            }

            // Fallback path: LSP documentSymbol (D-33-01)
            if s.fallbackDeps != nil && s.fallbackDeps.AcquireFn != nil {
                requester, release, acqErr := s.fallbackDeps.AcquireFn(ctx, lang)
                if acqErr != nil {
                    s.logger.Debug("fallback extraction skipped: no LS", "path", path, "lang", lang, "error", acqErr)
                    return nil, nil // D-33-02: silent skip
                }
                defer release()
                uri := "file://" + path
                tags, fbErr := s.fallbackDeps.Extractor.Extract(ctx, requester, path, uri)
                if fbErr != nil {
                    s.logger.Debug("fallback extraction failed", "path", path, "lang", lang, "error", fbErr)
                    return nil, nil
                }
                return tags, nil
            }

            // No extractor available
            return nil, nil
        })
        if extractErr != nil {
            s.logger.Debug("cache population skipped", "path", path, "error", extractErr)
        }
        return nil
    })
}
```

### Cache Persistence Test (already exists)
```go
// Source: internal/repomap/cache_test.go:207-244
// TestTagCache_Persistence already covers D-33-03:
// 1. Creates TagCache, inserts tags via GetOrExtract
// 2. Closes TagCache
// 3. Reopens new TagCache at same path
// 4. Verifies tags present without calling extractFn again
// This test ALREADY PASSES -- no new test code needed for persistence.
```

### Daemon Wiring (post-init pattern)
```go
// Source: synthesis from daemon.go:274-285 pattern
// In daemon.go, after kernel creation:
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    registry := treesitter.NewGrammarRegistry() // or share from skill init
    rs.SetFallbackDeps(repomapSkill.FallbackDeps{
        Registry:  registry,
        Extractor: repomap.NewFallbackExtractor(),
        AcquireFn: func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error) {
            wsKey := workspace.WorkspaceKey{RepoRoot: activeWSKey.RepoRoot, Language: lang}
            sessionID := fmt.Sprintf("fallback-%s", lang)
            lease, err := k.Pool().AcquireLease(ctx, sessionID, wsKey, false)
            if err != nil {
                return nil, nil, err
            }
            return lease, func() { k.Pool().ReleaseLease(sessionID) }, nil
        },
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Tree-sitter only (no fallback) | Tree-sitter + LSP fallback | Phase 33 | Languages without grammars get def-only tags |
| No grammar availability check | `GrammarRegistry.SupportsLanguage()` | Phase 31 (added method) | Deterministic fallback trigger |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `WorkerLease` directly satisfies `SymbolRequester` interface without adapter | Pitfall 1 | Need to write a thin adapter wrapper |
| A2 | `LangFromExt` language names match what `lspool` expects in `WorkspaceKey.Language` | Pitfall 3 | Need a name mapping layer |
| A3 | `walkAndExtract` can be changed to accept `context.Context` without breaking callers | Pitfall 2 | Need to trace all callers and update signatures |

## Open Questions

1. **Language name mapping between LangFromExt and WorkspaceKey.Language**
   - What we know: `LangFromExt` returns "go", "python", "c_sharp", etc. `WorkspaceKey.Language` is set during workspace activation.
   - What's unclear: Whether `langregistry.Registry.Get()` accepts the same names as `LangFromExt` returns.
   - Recommendation: Verify at implementation time; if names differ, add a mapping in the AcquireFn closure. Low risk since currently all 23 languages have tree-sitter grammars, so fallback only fires for future languages.

2. **GrammarRegistry sharing between Init and fallback**
   - What we know: `RepoMapSkill.Init()` creates a `GrammarRegistry` locally and passes it to `NewTagExtractor` and `NewElisionRenderer`.
   - What's unclear: Whether to store the registry on the skill struct for reuse in the fallback check, or pass it via `SetFallbackDeps`.
   - Recommendation: Store on skill struct during Init (cleanest), then reference in `walkAndExtract` for the `SupportsLanguage` check. Alternatively pass via `SetFallbackDeps`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (already configured) |
| Config file | None needed (standard `go test`) |
| Quick run command | `go test ./internal/repomap/... ./internal/skill/repomap/... -run "Fallback\|Persistence" -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RMAP-02 | FallbackExtractor called for non-tree-sitter languages | unit | `go test ./internal/skill/repomap/... -run TestWalkFallback -count=1` | No -- Wave 0 |
| RMAP-02 | SymbolRequester satisfied by WorkerLease | unit (compile check) | `go vet ./internal/repomap/...` | No -- Wave 0 |
| RMAP-03 (verify) | Cache persistence across TagCache close/reopen | unit | `go test ./internal/repomap/... -run TestTagCache_Persistence -count=1` | Yes -- already passes |

### Sampling Rate
- **Per task commit:** `go test ./internal/repomap/... ./internal/skill/repomap/... -count=1 && go vet ./...`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/skill/repomap/skill_test.go` -- test for walkAndExtract fallback path with mock registry + mock SymbolRequester
- [ ] Compile-time interface assertion: `var _ repomap.SymbolRequester = (*lspool.WorkerLease)(nil)`

## Security Domain

No new security concerns. FallbackExtractor uses existing LSP communication channel (lspool), TagCache uses parameterized SQL queries (per T-27-05), and no new external inputs are introduced.

## Sources

### Primary (HIGH confidence)
- `internal/repomap/fallback.go` -- FallbackExtractor implementation, SymbolRequester interface [VERIFIED: codebase]
- `internal/repomap/fallback_test.go` -- 5 test cases [VERIFIED: codebase]
- `internal/repomap/cache.go` -- TagCache with WAL mode [VERIFIED: codebase]
- `internal/repomap/cache_test.go` -- TestTagCache_Persistence (lines 207-244) [VERIFIED: codebase]
- `internal/skill/repomap/skill.go` -- walkAndExtract (lines 288-331), SetEnrichFn pattern [VERIFIED: codebase]
- `internal/kernel/lspool/lease.go` -- WorkerLease.Request signature [VERIFIED: codebase]
- `internal/kernel/lspool/pool.go` -- AcquireLease/ReleaseLease [VERIFIED: codebase]
- `internal/treesitter/registry.go` -- GrammarRegistry.SupportsLanguage [VERIFIED: codebase]
- `internal/daemon/daemon.go` -- Post-init wiring pattern (lines 274-285), enrichRepoMapFromLSP (lines 546-553) [VERIFIED: codebase]
- `internal/workspace/key.go` -- WorkspaceKey with Language field [VERIFIED: codebase]

### Secondary (MEDIUM confidence)
None needed -- all findings verified from codebase.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all components exist
- Architecture: HIGH -- wiring follows established SetEnrichFn pattern exactly
- Pitfalls: HIGH -- identified from actual codebase patterns (context propagation, session IDs, name mapping)

**Research date:** 2026-04-20
**Valid until:** 2026-05-20 (stable -- internal codebase patterns)
