# Phase 33: FallbackExtractor Wiring & Cache Persistence - Pattern Map

**Mapped:** 2026-04-20
**Files analyzed:** 3 modified files, 1 new test
**Analogs found:** 4 / 4

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/skill/repomap/skill.go` (modify: walkAndExtract + new fields/setter) | skill | transform | `internal/skill/repomap/skill.go` (SetEnrichFn pattern) | exact |
| `internal/daemon/daemon.go` (modify: post-init wiring) | config/wiring | request-response | `internal/daemon/daemon.go:273-285` (SetEnrichFn wiring) | exact |
| `internal/skill/repomap/skill_test.go` (modify: add fallback walk test) | test | CRUD | `internal/skill/repomap/skill_test.go` (newTestSkill pattern) | exact |
| `internal/repomap/cache_test.go` (verify existing persistence test) | test | CRUD | `internal/repomap/cache_test.go:207-244` (TestTagCache_Persistence) | exact |

## Pattern Assignments

### `internal/skill/repomap/skill.go` — Struct Fields + Setter (skill, transform)

**Analog:** Same file, existing `SetEnrichFn` pattern

**Struct field pattern** (lines 30-42):
```go
type RepoMapSkill struct {
	cache          *repomap.TagCache
	extractor      *repomap.TagExtractor
	renderer       *repomap.TreeRenderer // nil until ensureCache sets rootDir
	elider         *repomap.ElisionRenderer
	graph          *repomap.FileGraph
	graphVer       int64 // TagCache version when graph was last built
	mu             sync.Mutex
	logger         *slog.Logger
	rootDir        string // workspace root, resolved lazily via os.Getwd if empty
	cachePopulated bool
	enrichFn       func(graph *repomap.FileGraph) // optional LSP enrichment callback
}
```
New fields to add follow the same optional-callback pattern as `enrichFn`. Add a `registry` field (from Init) and a `fallbackDeps` pointer-to-struct field.

**Post-init setter pattern** (lines 103-109):
```go
// SetEnrichFn sets the optional LSP enrichment callback.
// Called by the daemon after kernel creation to enable cross-file LSP references.
func (s *RepoMapSkill) SetEnrichFn(fn func(graph *repomap.FileGraph)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enrichFn = fn
}
```
New `SetFallbackDeps` method must follow identical mutex-guarded setter pattern.

**Init registry preservation** (lines 72-74):
```go
// Create grammar registry and elision renderer for output formatting.
registry := treesitter.NewGrammarRegistry()
s.elider = repomap.NewElisionRenderer(registry)
```
Store `registry` on the struct so `walkAndExtract` can call `registry.SupportsLanguage(lang)`.

### `internal/skill/repomap/skill.go` — walkAndExtract Modification (skill, transform)

**Analog:** Same file, current `walkAndExtract` (lines 288-331)

**Current signature and extract function** (lines 288-325):
```go
func (s *RepoMapSkill) walkAndExtract(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// ... dir/symlink skip logic ...
		lang := repomap.LangFromExt(path)
		if lang == "" {
			return nil // skip unsupported file types
		}
		_, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
			if s.extractor == nil {
				return nil, nil // no tree-sitter extractor available
			}
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, readErr
			}
			tags, tagErr := s.extractor.Extract(source, path, lang)
			if tagErr != nil {
				// Log and skip: unsupported language for this extractor is not fatal.
				s.logger.Debug("tag extraction skipped", "path", path, "lang", lang, "error", tagErr)
				return nil, nil
			}
			return tags, nil
		})
		if extractErr != nil {
			s.logger.Debug("cache population skipped", "path", path, "error", extractErr)
		}
		return nil // never abort walk on individual file errors
	})
}
```

**Key modification points:**
1. Add `ctx context.Context` parameter (needed by FallbackExtractor.Extract and AcquireLease)
2. Inside the extract closure, after tree-sitter path: add registry check + fallback path
3. Follow the existing debug-log-and-skip error pattern (line 321)

**Caller to update** (`ensureCache`, line 276):
```go
if err := s.walkAndExtract(root); err != nil {
```
Must add context propagation. The caller `ensureCache` is called from tool handlers which have context.

### `internal/daemon/daemon.go` — Post-Init Wiring (config/wiring, request-response)

**Analog:** Same file, SetEnrichFn wiring block (lines 273-285)

**Existing wiring pattern** (lines 273-285):
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
```

New `SetFallbackDeps` call goes immediately after this block, following the same guard pattern (`if rs := ... != nil`).

**AcquireLease/ReleaseLease pattern** (lines 546-553):
```go
func enrichRepoMapFromLSP(ctx context.Context, k *kernel.Kernel, wsKey workspace.WorkspaceKey, g *repomapPkg.FileGraph, cache *repomapPkg.TagCache, logger *slog.Logger) {
	sessionID := "enrich-repomap"
	lease, err := k.Pool().AcquireLease(ctx, sessionID, wsKey, false)
	if err != nil {
		logger.Debug("LSP enrichment skipped: no warm session", "error", err)
		return
	}
	defer k.Pool().ReleaseLease(sessionID)
```

The AcquireFn closure for fallback must follow this exact AcquireLease/ReleaseLease pair pattern but with per-language session IDs and WorkspaceKey.

### `internal/skill/repomap/skill_test.go` — Fallback Walk Test (test, CRUD)

**Analog:** Same file, `newTestSkill` helper (lines 19-35)

**Test helper pattern** (lines 19-35):
```go
func newTestSkill(t *testing.T, populate bool) *RepoMapSkill {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })
	registry := treesitter.NewGrammarRegistry()
	elider := repomap.NewElisionRenderer(registry)
	s := &RepoMapSkill{
		cache:  cache,
		elider: elider,
		logger: slog.Default(),
	}
	// ...
}
```

New test should:
1. Create skill via `newTestSkill` (or similar direct struct construction)
2. Set `fallbackDeps` with a mock `SymbolRequester` and a `FallbackExtractor`
3. Use a mock/fake registry that returns `false` for `SupportsLanguage` to trigger the fallback path
4. Write a file with a non-tree-sitter extension to the temp dir
5. Call `walkAndExtract` and verify tags were extracted via the fallback

**Mock SymbolRequester pattern** from `internal/repomap/fallback_test.go`:
```go
// SymbolRequester interface:
type SymbolRequester interface {
	Request(ctx context.Context, method string, params, result interface{}) error
}
```

### `internal/repomap/cache_test.go` — Verify Existing Test (test, CRUD)

**Analog:** Same file, `TestTagCache_Persistence` (lines 207-244)

```go
func TestTagCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	filePath := writeTempFile(t, dir, "hello.go", "package main\n\nfunc Hello() {}\n")
	var calls atomic.Int32
	extractFn := func() ([]Tag, error) {
		calls.Add(1)
		return []Tag{
			{Name: "Hello", Kind: TagDef, File: filePath, Line: 2, Column: 5, StartByte: 20, EndByte: 50},
			{Name: "Run", Kind: TagRef, File: filePath, Line: 3, Column: 1, StartByte: 55, EndByte: 65},
		}, nil
	}
	cache1, err := NewTagCache(dbPath)
	require.NoError(t, err)
	tags1, err := cache1.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags1, 2)
	require.NoError(t, cache1.Close())
	cache2, err := NewTagCache(dbPath)
	require.NoError(t, err)
	defer cache2.Close()
	tags2, err := cache2.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, tags2, 2)
	assert.Equal(t, int32(1), calls.Load(), "extractFn should be called only once (tags persisted across restart)")
}
```

This test already exists and passes. D-33-03 is satisfied. No modification needed.

---

## Shared Patterns

### Debug-Log-and-Skip Error Handling
**Source:** `internal/skill/repomap/skill.go` lines 320-322
**Apply to:** All fallback extraction error paths in `walkAndExtract`
```go
s.logger.Debug("tag extraction skipped", "path", path, "lang", lang, "error", tagErr)
return nil, nil
```
Per D-33-02: missing LS and fallback failures use this same pattern -- debug log, return nil/nil, never abort the walk.

### Mutex-Guarded Setter for Post-Init Wiring
**Source:** `internal/skill/repomap/skill.go` lines 103-109
**Apply to:** New `SetFallbackDeps` method
```go
func (s *RepoMapSkill) SetEnrichFn(fn func(graph *repomap.FileGraph)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enrichFn = fn
}
```

### AcquireLease/ReleaseLease Pair
**Source:** `internal/daemon/daemon.go` lines 547-553
**Apply to:** Fallback AcquireFn closure in daemon wiring
```go
lease, err := k.Pool().AcquireLease(ctx, sessionID, wsKey, false)
if err != nil {
	logger.Debug("LSP enrichment skipped: no warm session", "error", err)
	return
}
defer k.Pool().ReleaseLease(sessionID)
```

### GetRepoMapSkill Guard Pattern
**Source:** `internal/skill/repomap/skill.go` lines 111-123 and `internal/daemon/daemon.go` line 274
**Apply to:** Daemon wiring for SetFallbackDeps
```go
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	// wire dependencies
}
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | All files have exact analogs in the existing codebase |

## Metadata

**Analog search scope:** `internal/skill/repomap/`, `internal/repomap/`, `internal/daemon/`, `internal/kernel/lspool/`, `internal/treesitter/`
**Files scanned:** 10
**Pattern extraction date:** 2026-04-20
