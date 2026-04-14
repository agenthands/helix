package kernel

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/langregistry"
	"github.com/postfix/serena/internal/workspace"
)

// WorkspaceRuntime extends Phase 1 workspace with LS pool integration.
// Per D-01: daemon owns the pool, WorkspaceRuntime coordinates language detection
// and document version tracking.
type WorkspaceRuntime struct {
	key         workspace.WorkspaceKey
	pool        *lspool.Pool
	langReg     *langregistry.Registry
	docVersions sync.Map // map[string]*atomic.Int32 -- per Pitfall 5: workspace owns version counters
	languages   []string // detected languages
	mu          sync.RWMutex
}

// NewWorkspaceRuntime creates a new workspace runtime for the given key.
func NewWorkspaceRuntime(key workspace.WorkspaceKey, pool *lspool.Pool, langReg *langregistry.Registry) *WorkspaceRuntime {
	return &WorkspaceRuntime{
		key:     key,
		pool:    pool,
		langReg: langReg,
	}
}

// Key returns the workspace key.
func (w *WorkspaceRuntime) Key() workspace.WorkspaceKey {
	return w.key
}

// DetectLanguages scans the root path for language marker files (per WRK-02).
// Detects: Go (go.mod), Python (pyproject.toml, setup.py), TypeScript/JavaScript (tsconfig.json,
// package.json, jsconfig.json), Rust (Cargo.toml), C++ (compile_commands.json, CMakeLists.txt),
// Swift (Package.swift), Zig (build.zig).
func (w *WorkspaceRuntime) DetectLanguages(rootPath string) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var langs []string

	markers := []struct {
		files    []string
		language string
	}{
		{files: []string{"go.mod"}, language: "go"},
		{files: []string{"pyproject.toml", "setup.py", "setup.cfg"}, language: "python"},
		{files: []string{"tsconfig.json", "package.json", "jsconfig.json"}, language: "typescript"},
		{files: []string{"Cargo.toml"}, language: "rust"},
		{files: []string{"pom.xml", "build.gradle", "build.gradle.kts", ".classpath"}, language: "java"},
		{files: []string{"compile_commands.json", "CMakeLists.txt", ".clangd"}, language: "cpp"},
		{files: []string{"Package.swift"}, language: "swift"},
		{files: []string{"build.zig", "build.zig.zon"}, language: "zig"},
		{files: []string{"composer.json", "composer.lock"}, language: "php"},
	}

	for _, m := range markers {
		for _, f := range m.files {
			if _, err := os.Stat(filepath.Join(rootPath, f)); err == nil {
				langs = append(langs, m.language)
				break
			}
		}
	}

	// Fallback: if no marker files matched and we have a language registry,
	// scan top-level files by extension to detect languages like Markdown
	// that have no project marker files.
	if len(langs) == 0 && w.langReg != nil {
		seen := make(map[string]bool)
		entries, _ := os.ReadDir(rootPath)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext == "" || seen[ext] {
				continue
			}
			seen[ext] = true
			for _, le := range w.langReg.ByExtension(ext) {
				if !seen[le.Language] {
					seen[le.Language] = true
					langs = append(langs, le.Language)
				}
			}
		}
	}

	w.languages = langs
	return langs
}

// Languages returns the detected languages.
func (w *WorkspaceRuntime) Languages() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, len(w.languages))
	copy(out, w.languages)
	return out
}

// NextDocVersion returns the next monotonically increasing version for a document URI.
// Per Pitfall 5: workspace owns version counters, not sessions.
func (w *WorkspaceRuntime) NextDocVersion(uri string) int32 {
	actual, _ := w.docVersions.LoadOrStore(uri, &atomic.Int32{})
	counter := actual.(*atomic.Int32)
	return counter.Add(1)
}

// AcquireSession acquires a worker lease from the pool for this workspace.
// The language is determined from the workspace key, falling back to the first
// detected language if the key's Language field is empty.
func (w *WorkspaceRuntime) AcquireSession(ctx context.Context, sessionID string, dirty bool) (*lspool.WorkerLease, error) {
	w.mu.RLock()
	lang := ""
	if len(w.languages) > 0 {
		lang = w.languages[0]
	}
	w.mu.RUnlock()

	key := w.key
	if key.Language == "" && lang != "" {
		key.Language = lang
	}
	return w.pool.AcquireLease(ctx, sessionID, key, dirty)
}
