package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// routeHandlerLink pairs a synthesized Route symbol's NodeID with the
// best-effort handler name extracted at the route-registration call site.
// factsFromExtracted resolves the handler name against the batch symbol index
// to emit a HANDLES edge once all symbols are known.
type routeHandlerLink struct {
	handlerName string
	routeNodeID uint64
}

// This file holds the pure name/file heuristics behind the TESTS edge kind
// emitted by factsFromExtracted, plus the repo-module detector and the
// in-memory multi-repo registry. They are intentionally side-effect-free (the
// registry aside) so they unit-test without spinning up the extraction
// pipeline.
//
// TESTS is a name+file heuristic edge: a test symbol is linked to the
// production symbol it most likely exercises by stripping the test prefix/
// suffix and looking up the result across the extraction batch. Confidence is
// low (0.30) — a name match, not a resolved reference. Coverage is per
// language test convention (Go/Python/TS-JS/Java/Kotlin/C#/Rust/PHP/Ruby;
// C/C++ use macros and are not name-detectable).

// isTestFile reports whether path is a test file for the given language, by
// filename/directory convention. path may be absolute or repo-relative.
func isTestFile(lang, path string) bool {
	base := path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		base = path[i+1:]
	}
	inTestsDir := strings.HasPrefix(path, "tests/") || strings.Contains(path, "/tests/")
	lowerBase := strings.ToLower(base)
	switch lang {
	case "go":
		return strings.HasSuffix(base, "_test.go")
	case "python":
		return strings.HasSuffix(base, "_test.py") ||
			(strings.HasSuffix(base, ".py") && strings.HasPrefix(base, "test_"))
	case "typescript", "javascript":
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
	case "java":
		return hasTestClassSuffix(base, ".java")
	case "kotlin":
		return hasTestClassSuffix(base, ".kt")
	case "c_sharp":
		return hasTestClassSuffix(base, ".cs")
	case "rust":
		return inTestsDir || strings.HasSuffix(base, "_test.rs") || strings.HasPrefix(base, "test_")
	case "php":
		return strings.HasSuffix(base, "Test.php")
	case "ruby":
		return strings.HasSuffix(base, "_test.rb") || strings.HasSuffix(base, "_spec.rb")
	case "c", "cpp":
		return strings.Contains(lowerBase, "test")
	}
	return false
}

// hasTestClassSuffix matches *Test / *Tests / *TestCase filenames, e.g.
// UserServiceTest.java, FooTests.cs.
func hasTestClassSuffix(base, ext string) bool {
	for _, s := range []string{"Test", "Tests", "TestCase"} {
		if strings.HasSuffix(base, s+ext) {
			return true
		}
	}
	return false
}

// isTestSymbol reports whether (lang, name, file) denotes a test symbol.
// Requires BOTH a test-file context (where the language uses one) AND a
// test-naming signal on the symbol itself, to avoid false positives.
func isTestSymbol(lang, name, file string) bool {
	if name == "" {
		return false
	}
	switch lang {
	case "go":
		if !strings.HasSuffix(file, "_test.go") {
			return false
		}
		for _, p := range []string{"Test", "Benchmark", "Example", "Fuzz"} {
			if strings.HasPrefix(name, p) && len(name) > len(p) {
				return true
			}
		}
		return false
	case "python":
		if !isTestFile(lang, file) {
			return false
		}
		return strings.HasPrefix(name, "test_") || strings.HasPrefix(name, "Test")
	case "typescript", "javascript":
		if !isTestFile(lang, file) {
			return false
		}
		return strings.HasSuffix(name, "Spec") || strings.HasSuffix(name, "Test")
	case "java", "kotlin", "c_sharp":
		// JUnit/NUnit/xUnit: a test class (Test/Tests/TestCase suffix) or a
		// test* method. @Test annotations aren't visible from the name alone.
		if !isTestFile(lang, file) {
			return false
		}
		return hasTestClassSuffixName(name) || strings.HasPrefix(name, "test")
	case "rust":
		// #[test] isn't visible from the name; rely on test_ prefix or a
		// tests/ integration-test file (every fn there is a test).
		return strings.HasPrefix(name, "test_") || (isTestFile(lang, file) && name != "")
	case "php":
		if !isTestFile(lang, file) {
			return false
		}
		return strings.HasPrefix(name, "test") || strings.HasSuffix(name, "Test")
	case "ruby":
		if !isTestFile(lang, file) {
			return false
		}
		return strings.HasPrefix(name, "test_")
	case "c", "cpp":
		// Google Test / Catch2 tests are macros (TEST(...), TEST_CASE) and
		// aren't symbol defs; detect test fixtures + test-named helpers instead.
		if !isTestFile(lang, file) {
			return false
		}
		return hasTestClassSuffixName(name) || strings.HasPrefix(name, "test_")
	}
	return false
}

// hasTestClassSuffixName matches class names ending Test/Tests/TestCase.
func hasTestClassSuffixName(name string) bool {
	for _, s := range []string{"Test", "Tests", "TestCase"} {
		if strings.HasSuffix(name, s) && len(name) > len(s) {
			return true
		}
	}
	return false
}

// stripTestClassSuffix removes a trailing Test/Tests/TestCase from name.
func stripTestClassSuffix(name string) string {
	for _, s := range []string{"TestCase", "Tests", "Test"} {
		if strings.HasSuffix(name, s) && len(name) > len(s) {
			return name[:len(name)-len(s)]
		}
	}
	return name
}

// testTargetCandidates returns production-symbol name candidates for a test
// symbol by stripping the language-specific test prefix/suffix. Most-specific
// first. Returns nil if name is not a recognized test symbol for lang.
func testTargetCandidates(lang, name string) []string {
	switch lang {
	case "go":
		for _, p := range []string{"Test", "Benchmark", "Example", "Fuzz"} {
			if strings.HasPrefix(name, p) && len(name) > len(p) {
				rest := name[len(p):]
				cands := []string{rest}
				if i := strings.IndexByte(rest, '_'); i > 0 {
					cands = append(cands, rest[:i])
				}
				return cands
			}
		}
	case "python":
		switch {
		case strings.HasPrefix(name, "test_") && len(name) > len("test_"):
			return []string{name[len("test_"):]}
		case strings.HasPrefix(name, "Test") && len(name) > len("Test"):
			return []string{name[len("Test"):]}
		}
	case "typescript", "javascript":
		switch {
		case strings.HasSuffix(name, "Spec") && len(name) > len("Spec"):
			return []string{name[:len(name)-len("Spec")]}
		case strings.HasSuffix(name, "Test") && len(name) > len("Test"):
			return []string{name[:len(name)-len("Test")]}
		}
	case "java", "kotlin", "c_sharp":
		if hasTestClassSuffixName(name) {
			return []string{stripTestClassSuffix(name)}
		}
		if strings.HasPrefix(name, "test") && len(name) > len("test") {
			return []string{name[len("test"):]} // testCreateUser -> CreateUser
		}
	case "rust":
		if strings.HasPrefix(name, "test_") && len(name) > len("test_") {
			return []string{name[len("test_"):]}
		}
	case "php":
		switch {
		case strings.HasSuffix(name, "Test") && len(name) > len("Test"):
			return []string{name[:len(name)-len("Test")]}
		case strings.HasPrefix(name, "test") && len(name) > len("test"):
			return []string{name[len("test"):]}
		}
	case "ruby":
		if strings.HasPrefix(name, "test_") && len(name) > len("test_") {
			return []string{name[len("test_"):]}
		}
	case "c", "cpp":
		if hasTestClassSuffixName(name) {
			return []string{stripTestClassSuffix(name)}
		}
		if strings.HasPrefix(name, "test_") && len(name) > len("test_") {
			return []string{name[len("test_"):]}
		}
	}
	return nil
}

// importLocalName returns the receiver-binding name a member-call would use
// for an import, per language (Go last "/"-segment, Java/C#/Kotlin/Python last
// "."-segment, Rust last "::"-segment, PHP last "\\"-segment). Used by the
// CROSS_CALLS emit to match a call's receiver (pkg.Method / Bar.method) to the
// external module it was imported from. Returns "" when not applicable.
func importLocalName(lang, source string) string {
	switch lang {
	case "go":
		if i := strings.LastIndexByte(source, '/'); i >= 0 {
			return source[i+1:]
		}
		return source
	case "python", "java", "kotlin", "c_sharp":
		if i := strings.LastIndexByte(source, '.'); i >= 0 {
			return source[i+1:]
		}
		return source
	case "rust":
		if i := strings.LastIndex(source, "::"); i >= 0 {
			return source[i+2:]
		}
		return source
	case "php":
		if i := strings.LastIndexByte(source, '\\'); i >= 0 {
			return source[i+1:]
		}
		return source
	}
	return ""
}
// detectRepoModulePath returns the repo's own module/package path used to
// classify imports as internal vs cross-repo. It reads go.mod (Go) first,
// then falls back to the Python top-level package (a root subdir with
// __init__.py). Returns "" when neither is found — cross-repo classification
// then falls back to per-language syntax heuristics (relative imports,
// crate/self/super, System/java.* stdlib, etc.).
func detectRepoModulePath(root string) string {
	if root == "" {
		return ""
	}
	if m := moduleFromGoMod(root); m != "" {
		return m
	}
	return pythonTopPackage(root)
}

func moduleFromGoMod(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "module\t") {
			return strings.TrimSpace(line[len("module"):])
		}
	}
	return ""
}

// pythonTopPackage returns the name of the single importable top-level Python
// package (a root subdir containing __init__.py), or "" if there is none or
// more than one (ambiguous).
func pythonTopPackage(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var pkg string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "__init__.py")); err == nil {
			if pkg != "" {
				return "" // ambiguous: multiple top-level packages
			}
			pkg = e.Name()
		}
	}
	return pkg
}

// multiRepoRegistry tracks repos indexed in this daemon session, keyed by
// repoID, so CROSS_IMPORTS edges can resolve external modules to known sibling
// repos via crossrepo.Resolve (federation). In-memory: a daemon restart
// re-registers repos as they re-index; persisting to the store is a follow-up.
type multiRepoRegistry struct {
	mu    sync.RWMutex
	repos map[string]repoInfo // repoID → info
}

type repoInfo struct {
	modulePath string
	rootPath   string
}

func newMultiRepoRegistry() *multiRepoRegistry {
	return &multiRepoRegistry{repos: make(map[string]repoInfo)}
}

// Register records (or refreshes) a repo's module path + root. Safe for
// concurrent buildFn invocations across distinct workspaces.
func (r *multiRepoRegistry) Register(repoID, modulePath, rootPath string) {
	if r == nil || repoID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repos[repoID] = repoInfo{modulePath: modulePath, rootPath: rootPath}
}

// KnownRepos returns modulePath → repoID for every registered repo except
// selfRepoID, skipping repos with no module path. The result feeds
// crossrepo.Resolve to upgrade CROSS_IMPORTS from module-level to
// resolved-to-sibling-repo.
func (r *multiRepoRegistry) KnownRepos(selfRepoID string) map[string]string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.repos))
	for id, info := range r.repos {
		if id == selfRepoID || info.modulePath == "" {
			continue
		}
		out[info.modulePath] = id
	}
	return out
}
