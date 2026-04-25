# Phase 49: bug-grammar-registry-consolidation - Pattern Map

**Mapped:** 2026-04-25
**Files analyzed:** 2 modified files (no new files)
**Analogs found:** 2 / 2 (in-file precedents — same files, neighboring methods)

## Scope Recap

This is a consolidation/DI cleanup phase, not a feature build. There are no new files. Two existing files are modified to collapse three `treesitter.NewGrammarRegistry()` calls down to one canonical instantiation at daemon bootstrap, propagated by DI:

1. `internal/skill/repomap/skill.go` — add `SetRegistry`, gut registry-dependent setup out of `Init`, drop `FallbackDeps.Registry` field, read registry from skill state in fallback path.
2. `internal/daemon/daemon.go` — wire `grammarRegistry` (already constructed at line 196) into RepoMapSkill via the new setter; delete the second `treesitter.NewGrammarRegistry()` at line 295.

## File Classification

| File | Role | Data Flow | Closest Analog | Match Quality |
|------|------|-----------|----------------|---------------|
| `internal/skill/repomap/skill.go` | skill (ToolProvider) | request-response (post-init wiring) | `SetEnrichFn` / `SetFallbackDeps` in same file | **exact** (in-file precedent) |
| `internal/daemon/daemon.go` | bootstrap orchestrator | event-driven (init sequence) | step 12b/12c block in same file | **exact** (in-file precedent) |

Match quality is "exact" because the patterns to copy live in the very files being edited — this is the cleanest possible analog signal.

## Pattern Assignments

### `internal/skill/repomap/skill.go` — Add `SetRegistry`, slim `Init`, drop `FallbackDeps.Registry`

**Analog:** `SetEnrichFn` (lines 115-121) and `SetFallbackDeps` (lines 123-129) in the same file.

#### Pattern 1 — Post-init setter shape (verbatim template for new `SetRegistry`)

`internal/skill/repomap/skill.go:115-129`:

```go
// SetEnrichFn sets the optional LSP enrichment callback.
// Called by the daemon after kernel creation to enable cross-file LSP references.
func (s *RepoMapSkill) SetEnrichFn(fn func(graph *repomap.FileGraph)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enrichFn = fn
}

// SetFallbackDeps sets the fallback extraction dependencies for languages without tree-sitter grammars.
// Called by the daemon after kernel creation to enable LSP documentSymbol fallback (D-33-01).
func (s *RepoMapSkill) SetFallbackDeps(deps *FallbackDeps) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fallbackDeps = deps
}
```

**Apply to new `SetRegistry`:** identical mutex-guarded shape. Per D-03 the new setter additionally performs the previously-eager construction of `s.elider` and `s.extractor` (lazy init under the lock). Suggested shape:

```go
// SetRegistry injects the canonical GrammarRegistry constructed at daemon bootstrap.
// Called by the daemon after kernel creation. Idempotent and nil-safe.
// Per D-03, this is also the point where registry-dependent renderers/extractors are built.
func (s *RepoMapSkill) SetRegistry(registry *treesitter.GrammarRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if registry == nil || s.registry != nil {
		return // nil-guard + idempotency, mirroring defensive style of surrounding setters
	}
	s.registry = registry
	s.elider = repomap.NewElisionRenderer(registry)
	if extractor, err := repomap.NewTagExtractor(registry); err == nil {
		s.extractor = extractor
	} else {
		s.logger.Warn("tag extractor creation failed, tree-sitter extraction disabled", "error", err)
	}
}
```

The exact name (`SetRegistry` vs `SetGrammarRegistry`) is Claude's discretion per CONTEXT.md. `SetRegistry` is consistent with the terse `SetEnrichFn` / `SetFallbackDeps` neighbors.

#### Pattern 2 — `Init` slim-down (delete registry-dependent block)

`internal/skill/repomap/skill.go:67-97` — current `Init`:

```go
func (s *RepoMapSkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}

	// Create tag cache from project dir.
	dbPath := filepath.Join(deps.ProjectDir, "index", "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	if err != nil {
		return serr.Wrap(serr.Internal, "creating tag cache for repomap skill", err)
	}
	s.cache = cache

	// Create grammar registry and elision renderer for output formatting.
	registry := treesitter.NewGrammarRegistry()   // <-- DELETE (D-03)
	s.registry = registry                          // <-- DELETE
	s.elider = repomap.NewElisionRenderer(registry) // <-- MOVE to SetRegistry

	// Create tag extractor for tree-sitter-based tag extraction.
	extractor, err := repomap.NewTagExtractor(registry) // <-- MOVE to SetRegistry
	if err != nil {
		s.logger.Warn("tag extractor creation failed, tree-sitter extraction disabled", "error", err)
	} else {
		s.extractor = extractor
	}

	return nil
}
```

After consolidation, `Init` ends at `s.cache = cache; return nil`. Everything from line 83 onward moves into `SetRegistry`. The `treesitter` import remains (still used by the field type).

#### Pattern 3 — `FallbackDeps` field removal

`internal/skill/repomap/skill.go:47-53` — current struct:

```go
// FallbackDeps holds dependencies for LSP-based fallback tag extraction.
// Wired by the daemon after kernel creation via SetFallbackDeps.
type FallbackDeps struct {
	Registry  *treesitter.GrammarRegistry  // <-- DELETE per D-04
	AcquireFn func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error)
	Extractor *repomap.FallbackExtractor
}
```

Per D-04, `Registry` is removed. The fallback path at lines 379-394 already references the registry only indirectly through `s.fallbackDeps.AcquireFn` / `s.fallbackDeps.Extractor` — actually it never reads `s.fallbackDeps.Registry` at all. So no fallback-path code rewrite is needed beyond the struct field deletion. Confirm with a `grep -n "fallbackDeps.Registry" internal/` during planning.

#### Pattern 4 — Existing tree-sitter dispatch (unchanged, but read it before editing)

`internal/skill/repomap/skill.go:362-398` — fallback dispatch reads `s.registry` directly via `s.registry.SupportsLanguage(lang)` (line 364). After the change `s.registry` is set by `SetRegistry`, not `Init`. `LazyInitMiddleware` ensures no tool fires before post-init wiring completes (per CLAUDE.md middleware order and D-03), so this remains safe. **No code change required here.**

---

### `internal/daemon/daemon.go` — Wire shared registry, delete second instantiation

**Analog:** existing step 12b/12c wiring block (lines 278-307) in the same file.

#### Pattern 1 — Canonical construction (already exists, verify only)

`internal/daemon/daemon.go:193-197`:

```go
// 6. Create diagnostic store and body extractor.
// NOTE: step numbering preserved from original New() for git-blame continuity.
diagStore := diag.NewDiagnosticStore()
grammarRegistry := treesitter.NewGrammarRegistry()
bodyExtractor := edit.NewBodyExtractor(grammarRegistry)
```

Per D-06, this stays exactly as-is. `bodyExtractor` already proves the DI pattern works for kernel consumers.

#### Pattern 2 — Existing post-init setter calls (template for new `SetRegistry` call)

`internal/daemon/daemon.go:278-307` — existing 12b/12c block:

```go
// 12b. Wire repomap skill LSP enrichment callback (RMAP-08).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	tagCache := rs.Cache()
	rs.SetEnrichFn(func(g *repomapPkg.FileGraph) {
		wsKey := activeWSKey
		if wsKey.RepoRoot == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		enrichRepoMapFromLSP(ctx, k, wsKey, g, tagCache, logger)
	})
}

// 12c. Wire repomap skill fallback extraction for non-tree-sitter languages (RMAP-02).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	rs.SetFallbackDeps(&repomapSkill.FallbackDeps{
		Registry:  treesitter.NewGrammarRegistry(),  // <-- DELETE per D-05
		Extractor: repomapPkg.NewFallbackExtractor(),
		AcquireFn: func(ctx context.Context, lang string) (repomapPkg.SymbolRequester, func(), error) {
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

**Two surgical edits land here:**

1. **Insert** a new wiring block (call it 12a or fold into 12b — Claude's discretion per CONTEXT.md) that calls `rs.SetRegistry(grammarRegistry)`. Per D-03 it must run **before** the first tool call but ordering relative to `SetEnrichFn` / `SetFallbackDeps` is discretionary; the natural choice is *first* in the post-init block so that subsequent setters operate on a registry-equipped skill.

   Suggested shape (mirrors 12b precedent):

   ```go
   // 12a. Wire shared GrammarRegistry into repomap skill (BUG-04, D-01/D-02/D-03).
   if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
       rs.SetRegistry(grammarRegistry)
   }
   ```

2. **Delete** line 295 (`Registry: treesitter.NewGrammarRegistry(),`) per D-05. The `FallbackDeps` literal then has only `Extractor` and `AcquireFn`, matching the slimmed struct from D-04.

#### Pattern 3 — Verification

Per D-10, after the change:

```bash
grep -rn "NewGrammarRegistry(" --include="*.go" | grep -v "_test.go"
```

must return exactly one hit at `internal/daemon/daemon.go:196`. Planner should include this as the acceptance check on the production-side plan.

## Shared Patterns

### Idempotent post-init setter (mutex + once-only)
**Source:** `internal/skill/repomap/skill.go:115-129` (`SetEnrichFn`, `SetFallbackDeps`)
**Apply to:** the new `SetRegistry`
- Lock `s.mu` for the full setter body.
- No `sync.Once` — setters rely on daemon calling them at most once during post-init. Defensive nil-guard plus "already set" early-return is the project's convention here.

### `GetRepoMapSkill()` lookup gate before wiring
**Source:** `internal/daemon/daemon.go:279, 293` (both 12b and 12c open with `if rs := repomapSkill.GetRepoMapSkill(); rs != nil`)
**Apply to:** the new 12a `SetRegistry` call — same `if rs != nil` guard. This keeps daemon bootstrap tolerant of the skill not being registered (matches the existing degraded-mode philosophy in step 9).

### DI for kernel consumers of `*GrammarRegistry`
**Source:** `internal/daemon/daemon.go:197` (`bodyExtractor := edit.NewBodyExtractor(grammarRegistry)`)
**Apply to:** RepoMapSkill via the new setter. Same registry pointer flows to both `bodyExtractor` (constructor injection) and `RepoMapSkill` (setter injection). Single source of truth at line 196.

## No Analog Found

None. Every required pattern has an in-file precedent in the two files being modified.

## Files Not Touched (Verify Only)

| File | Why It's Listed | What to Check |
|------|-----------------|---------------|
| `internal/treesitter/registry.go` | Constructor stays exported per D-09 | `NewGrammarRegistry` signature/visibility unchanged |
| `internal/kernel/edit/` (BodyExtractor) | DI pattern already correct per D-06 | No edits; constructor still receives `grammarRegistry` from daemon |
| Test files (~22 sites) | Independent fixtures per D-07 | No changes — they intentionally construct their own registries |

## Metadata

**Analog search scope:** `internal/skill/repomap/`, `internal/daemon/`, `internal/treesitter/`
**Files scanned:** 4 (CONTEXT.md, skill.go, daemon.go bootstrap region, registry.go header)
**Pattern extraction date:** 2026-04-25
**Strategy:** in-file precedents only — no broader codebase search needed because the consolidation pattern (post-init setter mirroring two existing post-init setters) is fully specified by neighbors.
