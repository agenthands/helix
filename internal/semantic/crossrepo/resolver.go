// Package crossrepo provides the pure resolution logic behind CROSS_* edges:
// classifying an import as internal vs external (cross-repo) and resolving an
// external import to a known sibling repository by module path.
//
// This is the federation resolver. It is intentionally pure (no I/O, no store
// dependency): the daemon-side caller supplies the set of known sibling repos
// (module path → repoID) and the importing repo's own module path; the
// resolver decides, per import, whether it is internal, external-unresolved,
// or external-resolved to a sibling repo. The emit layer turns those
// decisions into CROSS_IMPORTS / CROSS_CALLS edges.
//
// Per-language conventions (matched against the import-source string each
// provider emits — verified against extract/<lang>/queries.scm):
//   - go:        "domain/owner/repo/..." — external iff first segment is a host.
//   - python:    "pkg.sub" / ".relative" — internal iff top package == repo's.
//   - ts/js:     "./x","../x","@/x" internal; bare ("react") external.
//   - rust:      "crate::","self::","super::" internal; else external.
//   - java/kotlin: "java.","javax.","kotlin." stdlib internal; else external.
//   - c_sharp:   "System..." BCL internal; else external.
//   - php:       namespaced "Foo\\Bar" — internal iff under repo's namespace.
//   - ruby:      "."-prefixed (require_relative) internal; else external.
//   - c/cpp:     #include headers — treated internal (no module system).
package crossrepo

import "strings"

// Relation classifies an import relative to the importing repository.
type Relation int

const (
	// Internal — the import is within the importing repo's own module/package.
	Internal Relation = iota
	// External — the import is a cross-repo / third-party dependency.
	External
)

// ClassifyImport determines whether importSource is internal to the repo
// (same module/package) or external, per language convention. repoModulePath
// is the importing repo's own module path where the language uses one.
func ClassifyImport(repoModulePath, lang, importSource string) Relation {
	src := strings.TrimSpace(importSource)
	if src == "" {
		return Internal
	}
	switch lang {
	case "go":
		return classifyGo(repoModulePath, src)
	case "python":
		return classifyPython(repoModulePath, src)
	case "typescript", "javascript":
		return classifyJS(src)
	case "rust":
		return classifyRust(src)
	case "java", "kotlin":
		return classifyJVM(repoModulePath, src)
	case "c_sharp":
		return classifyCSharp(repoModulePath, src)
	case "php":
		return classifyPHP(repoModulePath, src)
	case "ruby":
		return classifyRuby(src)
	case "c", "cpp":
		// C/C++ #include is a header file inclusion, not a module dependency —
		// there is no module path to resolve, so it is never cross-repo.
		return Internal
	default:
		if strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../") {
			return Internal
		}
		return External
	}
}

// ModulePathPrefix extracts the module-root prefix of an import source — the
// key used to match an import to a sibling repo. Returns "" for relative /
// unparseable / module-less sources.
func ModulePathPrefix(lang, importSource string) string {
	src := strings.TrimSpace(importSource)
	switch lang {
	case "go":
		return goModuleRoot(src)
	case "python":
		if src == "" || strings.HasPrefix(src, ".") {
			return ""
		}
		if i := strings.IndexByte(src, '.'); i >= 0 {
			return src[:i]
		}
		return src
	case "typescript", "javascript":
		return jsModuleRoot(src)
	case "rust":
		if strings.HasPrefix(src, "crate::") || strings.HasPrefix(src, "self::") || strings.HasPrefix(src, "super::") {
			return ""
		}
		if i := strings.Index(src, "::"); i >= 0 {
			return src[:i]
		}
		return src
	case "java", "kotlin":
		return firstNDotSegments(src, 2) // e.g. "com.example" (Maven groupId-ish)
	case "c_sharp":
		return firstNDotSegments(src, 2)
	case "php":
		return firstNSegments(src, "\\", 2)
	case "ruby":
		if strings.HasPrefix(src, ".") {
			return ""
		}
		if i := strings.IndexByte(src, '/'); i >= 0 {
			return src[:i]
		}
		return src
	}
	return ""
}

// Resolve maps an external import source to a known sibling repository.
// known maps a sibling repo's module-root path → repoID. It returns the
// matching repoID (longest-prefix match wins) and true if the import falls
// under a known sibling repo; ("", false) otherwise.
func Resolve(lang, importSource string, known map[string]string) (repoID string, ok bool) {
	prefix := ModulePathPrefix(lang, importSource)
	if prefix == "" || len(known) == 0 {
		return "", false
	}
	best := ""
	bestID := ""
	for mod, id := range known {
		if mod == prefix || strings.HasPrefix(prefix, mod+"/") || strings.HasPrefix(prefix, mod+".") || strings.HasPrefix(prefix, mod+"\\") || strings.HasPrefix(prefix, mod+"::") {
			if len(mod) > len(best) {
				best = mod
				bestID = id
			}
		}
	}
	if best == "" {
		return "", false
	}
	return bestID, true
}

// ── per-language classifiers ──

func classifyGo(repoModulePath, src string) Relation {
	if repoModulePath != "" && (src == repoModulePath || strings.HasPrefix(src, repoModulePath+"/")) {
		return Internal
	}
	first := src
	if i := strings.IndexByte(src, '/'); i >= 0 {
		first = src[:i]
	}
	if strings.Contains(first, ".") {
		return External
	}
	return Internal
}

func classifyPython(repoModulePath, src string) Relation {
	if strings.HasPrefix(src, ".") {
		return Internal
	}
	top := src
	if i := strings.IndexByte(src, '.'); i >= 0 {
		top = src[:i]
	}
	if repoModulePath != "" && top == repoModulePath {
		return Internal
	}
	return External
}

func classifyJS(src string) Relation {
	if strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../") || strings.HasPrefix(src, "@/") {
		return Internal
	}
	return External
}

func classifyRust(src string) Relation {
	if strings.HasPrefix(src, "crate::") || strings.HasPrefix(src, "self::") || strings.HasPrefix(src, "super::") {
		return Internal
	}
	return External // std and third-party crates are external
}

// classifyJVM covers Java + Kotlin.
func classifyJVM(repoModulePath, src string) Relation {
	if repoModulePath != "" && (src == repoModulePath || strings.HasPrefix(src, repoModulePath+".")) {
		return Internal
	}
	for _, std := range []string{"java.", "javax.", "kotlin.", "kotlinx."} {
		if strings.HasPrefix(src, std) {
			return Internal
		}
	}
	return External
}

func classifyCSharp(repoModulePath, src string) Relation {
	if repoModulePath != "" && (src == repoModulePath || strings.HasPrefix(src, repoModulePath+".")) {
		return Internal
	}
	if strings.HasPrefix(src, "System") {
		return Internal
	}
	return External
}

func classifyPHP(repoModulePath, src string) Relation {
	if repoModulePath != "" && (src == repoModulePath || strings.HasPrefix(src, repoModulePath+"\\")) {
		return Internal
	}
	return External
}

func classifyRuby(src string) Relation {
	if strings.HasPrefix(src, ".") {
		return Internal // require_relative
	}
	return External
}

// ── prefix helpers ──

func goModuleRoot(src string) string {
	parts := strings.Split(src, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return src
}

func jsModuleRoot(src string) string {
	if strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../") || strings.HasPrefix(src, "@/") {
		return ""
	}
	if strings.HasPrefix(src, "@") {
		parts := strings.SplitN(src, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return src
	}
	if i := strings.IndexByte(src, '/'); i >= 0 {
		return src[:i]
	}
	return src
}

// firstNDotSegments returns the first n dot-separated segments of s (e.g.
// "com.example.foo" with n=2 → "com.example"). Returns s whole if it has
// fewer than n segments.
func firstNDotSegments(s string, n int) string {
	return firstNSegments(s, ".", n)
}

// firstNSegments returns the first n segments of s split on sep.
func firstNSegments(s, sep string, n int) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, sep)
	if len(parts) <= n {
		return s
	}
	return strings.Join(parts[:n], sep)
}
