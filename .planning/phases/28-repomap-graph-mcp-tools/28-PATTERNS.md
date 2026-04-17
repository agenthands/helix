# Phase 28: RepoMap Graph & MCP Tools - Pattern Map

**Mapped:** 2026-04-16
**Files analyzed:** 10 (7 new, 3 modified)
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/repomap/graph.go` | service | transform | `internal/repomap/cache.go` | role-match |
| `internal/repomap/graph_test.go` | test | transform | `internal/repomap/cache_test.go` | exact |
| `internal/repomap/pagerank.go` | utility | batch | (no analog -- pure algorithm) | none |
| `internal/repomap/pagerank_test.go` | test | batch | `internal/repomap/cache_test.go` | role-match |
| `internal/repomap/render.go` | service | transform | `internal/repomap/elide.go` | exact |
| `internal/repomap/render_test.go` | test | transform | `internal/repomap/elide_test.go` | exact |
| `internal/skill/repomap/skill.go` | controller | request-response | `internal/skill/memory/skill.go` | exact |
| `internal/skill/repomap/skill_test.go` | test | request-response | `internal/repomap/cache_test.go` | role-match |
| `internal/repomap/cache.go` (modify) | service | CRUD | -- (self) | -- |
| `internal/daemon/imports.go` (modify) | config | -- | -- (self) | -- |

## Pattern Assignments

### `internal/skill/repomap/skill.go` (controller, request-response)

**Analog:** `internal/skill/memory/skill.go`

**Imports pattern** (lines 1-16):
```go
package repomap

import (
	"fmt"
	"log/slog"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/skill"
)
```

**init() registration pattern** (lines 24-26):
```go
func init() {
	skill.Register(&RepoMapSkill{})
}
```

**Skill interface pattern** (lines 18-33):
```go
type MemorySkill struct {
	store  *memory.MemoryStore
	logger *slog.Logger
}

func (s *MemorySkill) Name() string { return "memory" }
func (s *MemorySkill) Description() string { return "Project and global memory persistence" }
```

**Init with deps pattern** (lines 35-51):
```go
func (s *MemorySkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}
	// ... create underlying service from deps paths ...
	return nil
}
```

**Tools() return pattern** (lines 54-64):
```go
func (s *MemorySkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		s.writeMemoryTool(),
		s.readMemoryTool(),
		// ...
	}
}
```

**ToolDef construction pattern** (lines 67-77):
```go
func (s *MemorySkill) writeMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "write_memory",
		Description: "Write information about this project...",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}
```

**ExecuteTool dispatch pattern** (lines 151-170):
```go
func (s *MemorySkill) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "write_memory":
		return s.execWrite(args)
	// ...
	default:
		return "", serr.New(serr.InvalidArgs, "unknown memory tool").WithTool(name)
	}
}
```

**Arg extraction + error pattern** (lines 172-185):
```go
func (s *MemorySkill) execWrite(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("write_memory")
	}
	// ...
	if err := s.store.Write(name, content); err != nil {
		return "", serr.Wrap(serr.Internal, "write operation failed", err).WithTool("write_memory")
	}
	return fmt.Sprintf("Memory %q written successfully.", name), nil
}
```

---

### `internal/repomap/graph.go` (service, transform)

**Analog:** `internal/repomap/cache.go` (same package, struct + methods pattern)

**Package and imports pattern** (cache.go lines 1-11):
```go
package repomap

import (
	"fmt"
	"math"
	"sync"
)
```

**Struct with mutex pattern** (cache.go lines 17-20):
```go
type TagCache struct {
	db *sql.DB
	mu sync.Mutex
}
```
Apply as: `FileGraph` struct with `sync.Mutex` for concurrent access, `Edges` and `Files` maps.

**Method on struct with mutex pattern** (cache.go lines 59-102):
```go
func (c *TagCache) GetOrExtract(filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
	// ...
	c.mu.Lock()
	// ... check/compute ...
	c.mu.Unlock()
	// ...
}
```

---

### `internal/repomap/pagerank.go` (utility, batch)

**No direct analog.** Pure algorithm file (~60 LOC). Follow the same package conventions:

**File structure pattern** (from tags.go, elide.go):
```go
package repomap

// Package-level doc comment explaining the algorithm.
// Exported function signature with clear parameter docs.
```

Follow existing `repomap` package conventions: exported types at top, exported functions next, unexported helpers at bottom.

---

### `internal/repomap/render.go` (service, transform)

**Analog:** `internal/repomap/elide.go`

**Imports pattern** (elide.go lines 1-12):
```go
package repomap

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)
```

**Renderer struct pattern** (elide.go lines 18-25):
```go
type ElisionRenderer struct {
	registry *treesitter.GrammarRegistry
}

func NewElisionRenderer(registry *treesitter.GrammarRegistry) *ElisionRenderer {
	return &ElisionRenderer{registry: registry}
}
```

**RenderFile method signature** (elide.go lines 69-93) -- the render.go will call this:
```go
func (r *ElisionRenderer) RenderFile(source []byte, lang string, tags []Tag) string {
	// ... filtering, sorting, tree-sitter rendering ...
}
```

**bytes.Buffer output assembly** (elide.go lines 134-163):
```go
var buf bytes.Buffer
for _, tag := range defs {
	// ...
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	fmt.Fprintf(&buf, "// L%d:\n%s", tag.Line+1, text)
}
return buf.String()
```

---

### `internal/repomap/graph_test.go` (test, transform)

**Analog:** `internal/repomap/cache_test.go`

**Test file structure** (cache_test.go lines 1-11):
```go
package repomap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**Helper function pattern** (cache_test.go lines 13-21):
```go
func newTestCache(t *testing.T) *TagCache {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tags.db")
	cache, err := NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })
	return cache
}
```

**Test function pattern** (cache_test.go lines 44-69):
```go
func TestTagCache_GetOrExtract_CacheMiss(t *testing.T) {
	cache := newTestCache(t)
	// ... setup ...
	result, err := cache.GetOrExtract(filePath, extractFn)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "Hello", result[0].Name)
}
```

Test naming convention: `TestType_Method_Scenario` (e.g., `TestFileGraph_BuildGraph_CrossFileEdges`).

---

### `internal/repomap/cache.go` (modify -- add AllFiles method)

**Existing method pattern** (cache.go lines 129-151 -- `loadTags`):
```go
func (c *TagCache) loadTags(filePath string) ([]Tag, error) {
	rows, err := c.db.Query(
		"SELECT name, kind, line, col, start_byte, end_byte FROM file_tags WHERE file_path = ? ORDER BY line, col",
		filePath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		// ... Scan ...
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
```

New `AllFiles()` method should follow this exact SQL+scan pattern, adding `c.mu.Lock()/Unlock()` like public methods do.

---

### `internal/daemon/imports.go` (modify -- add blank import)

**Pattern** (imports.go lines 1-12):
```go
package daemon

import (
	// Blank imports trigger skill.Register() via init() (Caddy-style).
	_ "github.com/postfix/serena/internal/kernel/diag"
	_ "github.com/postfix/serena/internal/kernel/edit"
	_ "github.com/postfix/serena/internal/kernel/fileops"
	_ "github.com/postfix/serena/internal/kernel/symbols"
	_ "github.com/postfix/serena/internal/profile"
	_ "github.com/postfix/serena/internal/skill/memory"
	_ "github.com/postfix/serena/internal/skill/workflow"
)
```

Add: `_ "github.com/postfix/serena/internal/skill/repomap"` in alphabetical order.

## Shared Patterns

### Error Handling
**Source:** `internal/errors/errors.go` + `internal/skill/memory/skill.go`
**Apply to:** `internal/skill/repomap/skill.go` (all exec methods)
```go
import serr "github.com/postfix/serena/internal/errors"

// Argument validation:
serr.New(serr.InvalidArgs, "'token_budget' parameter is required").WithTool("get_repo_map")

// Wrapping internal errors:
serr.Wrap(serr.Internal, "building reference graph", err).WithTool("get_repo_map")
```

Available error kinds: `NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`.

### Skill Registration
**Source:** `internal/skill/skill.go` lines 11-19, `internal/skill/registry.go` lines 17-21
**Apply to:** `internal/skill/repomap/skill.go`

SkillDeps struct currently provides:
```go
type SkillDeps struct {
	ProjectDir string
	GlobalDir  string
	Logger     *slog.Logger
}
```

RepoMapSkill needs TagCache, ElisionRenderer, GrammarRegistry -- these must be injected. Options: extend SkillDeps with optional pointer fields, or have the skill construct its own from ProjectDir.

### Test Framework
**Source:** `internal/repomap/cache_test.go`
**Apply to:** All test files
- Use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require`
- Use `t.Helper()` in helper functions
- Use `t.TempDir()` for temp directories
- Use `t.Cleanup()` for resource cleanup
- Name tests: `TestType_Method_Scenario`

### Package Convention
**Source:** All `internal/repomap/*.go` files
**Apply to:** `graph.go`, `pagerank.go`, `render.go`
- Package comment on first file (tags.go already has it)
- Exported types and constructors at top
- Public methods in the middle
- Private helpers at bottom
- `sync.Mutex` for shared mutable state

### LSP Enrichment Interface
**Source:** `internal/repomap/fallback.go` lines 13-15
**Apply to:** `internal/repomap/graph.go` (enrichment method)
```go
type SymbolRequester interface {
	Request(ctx context.Context, method string, params, result interface{}) error
}
```

LSAdapter.References method (lspool/adapter.go lines 43-53) returns `[]gen.Location` -- graph enrichment should consume this type.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/repomap/pagerank.go` | utility | batch | Pure algorithm (power iteration PageRank); no similar computational files exist in the codebase. Use RESEARCH.md Pattern 2 code example as the starting template. |

## Metadata

**Analog search scope:** `internal/repomap/`, `internal/skill/`, `internal/errors/`, `internal/mcp/`, `internal/kernel/lspool/`, `internal/daemon/`
**Files scanned:** 15
**Pattern extraction date:** 2026-04-16
